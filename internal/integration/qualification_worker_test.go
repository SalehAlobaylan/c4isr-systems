package integration

// The planned worker restart is a real restart of a worker process that this
// workload owns. The child process is a re-execution of this test binary; it
// talks only to the disposable isolated server and database created for the
// run. No shared installation, service or container is contacted, stopped or
// mutated by the restart.
import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	qualificationWorkerMarker   = "QUALIFICATION-WORKER "
	qualificationWorkerEnv      = "C4ISR_QUALIFICATION_WORKER"
	qualificationWorkerURL      = "C4ISR_QUALIFICATION_WORKER_URL"
	qualificationWorkerToken    = "C4ISR_QUALIFICATION_WORKER_TOKEN"
	qualificationWorkerSource   = "C4ISR_QUALIFICATION_WORKER_SOURCE"
	qualificationWorkerID       = "C4ISR_QUALIFICATION_WORKER_ID"
	qualificationWorkerDeadline = "C4ISR_QUALIFICATION_WORKER_DEADLINE"
)

// TestMapFirstQualificationWorkerProcess is the restarted worker itself. It
// is only started by the qualification harness, performs scheduled retrievals
// against the isolated server, and reports every attempt as one marked line.
func TestMapFirstQualificationWorkerProcess(t *testing.T) {
	if os.Getenv(qualificationWorkerEnv) != "1" {
		t.Skip("qualification worker process; started only by the qualification harness")
	}
	base := os.Getenv(qualificationWorkerURL)
	token := os.Getenv(qualificationWorkerToken)
	source := os.Getenv(qualificationWorkerSource)
	if base == "" || token == "" || source == "" {
		t.Fatal("qualification worker configuration is incomplete")
	}
	deadline := time.Now().Add(15 * time.Minute)
	if raw := os.Getenv(qualificationWorkerDeadline); raw != "" {
		if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
			deadline = time.Unix(seconds, 0)
		}
	}
	client := &http.Client{Timeout: 15 * time.Second}
	for time.Now().Before(deadline) {
		started := time.Now()
		status := 0
		request, err := http.NewRequest(http.MethodPost, base+"/api/v1/sources/"+source+"/scheduler/run", nil)
		if err == nil {
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("Content-Type", "application/json")
			if response, e := client.Do(request); e == nil {
				status = response.StatusCode
				_, _ = io.Copy(io.Discard, response.Body)
				response.Body.Close()
			}
		}
		action, _ := json.Marshal(map[string]any{"kind": "worker_action",
			"id": os.Getenv(qualificationWorkerID), "pid": os.Getpid(),
			"at": time.Now().UTC().Format(time.RFC3339Nano), "status": status,
			"success":   status >= 200 && status < 300,
			"elapsedMs": float64(time.Since(started).Microseconds()) / 1000})
		fmt.Printf("%s%s\n", qualificationWorkerMarker, action)
		time.Sleep(time.Second)
	}
}

// syncBuffer collects a child process error stream without blocking it.
type syncBuffer struct {
	mu      sync.Mutex
	content []byte
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.content = append(b.content, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.content)
}

// qualificationWorkerProcess is one owned worker generation.
type qualificationWorkerProcess struct {
	id      string
	pid     int
	command *exec.Cmd
	actions chan map[string]any
	closed  chan struct{}
	stderr  *syncBuffer
}

func startQualificationWorker(base, token, source, id string, deadline time.Time) (*qualificationWorkerProcess, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("qualification worker executable: %w", err)
	}
	command := exec.Command(executable, "-test.run=^TestMapFirstQualificationWorkerProcess$", "-test.timeout=20m")
	command.Env = append(os.Environ(),
		qualificationWorkerEnv+"=1",
		qualificationWorkerURL+"="+base,
		qualificationWorkerToken+"="+token,
		qualificationWorkerSource+"="+source,
		qualificationWorkerID+"="+id,
		qualificationWorkerDeadline+"="+strconv.FormatInt(deadline.Unix(), 10))
	stderr := &syncBuffer{}
	command.Stderr = stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("qualification worker stdout: %w", err)
	}
	if err = command.Start(); err != nil {
		return nil, fmt.Errorf("qualification worker start: %w", err)
	}
	worker := &qualificationWorkerProcess{id: id, pid: command.Process.Pid, command: command,
		actions: make(chan map[string]any, 64), closed: make(chan struct{}), stderr: stderr}
	go func() {
		defer close(worker.closed)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, qualificationWorkerMarker) {
				continue
			}
			action := map[string]any{}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, qualificationWorkerMarker)), &action) == nil {
				select {
				case worker.actions <- action:
				default:
				}
			}
		}
	}()
	return worker, nil
}

// stop ends this owned worker generation. No process outside this run's own
// child is selected, named or signalled.
func (w *qualificationWorkerProcess) stop() {
	if w == nil || w.command == nil || w.command.Process == nil {
		return
	}
	_ = w.command.Process.Kill()
	select {
	case <-w.closed:
	case <-time.After(10 * time.Second):
	}
	_ = w.command.Wait()
}

// success waits for one completed scheduled retrieval by this generation.
func (w *qualificationWorkerProcess) success(timeout time.Duration) (map[string]any, bool) {
	deadline := time.After(timeout)
	for {
		select {
		case action := <-w.actions:
			if ok, _ := action["success"].(bool); ok {
				return action, true
			}
		case <-deadline:
			return nil, false
		}
	}
}

// newQualificationWorkerSource provisions the isolated source, service
// identity and profile that the restarted worker performs work against. It is
// separate from the measured source so the restart cannot distort the
// measured request distribution.
func newQualificationWorkerSource(t *testing.T, f *publicFixture) (string, string, error) {
	t.Helper()
	const requestedID = "qualification-worker-feed"
	status, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources",
		map[string]any{"id": requestedID, "name": "Isolated qualification worker feed", "type": "external"}, nil)
	if status != 201 {
		return "", "", fmt.Errorf("source creation returned %d: %v", status, body)
	}
	// The server is authoritative for the identity it stored.
	sourceID, _ := body["id"].(string)
	if sourceID == "" {
		return "", "", fmt.Errorf("source creation returned no identity: %v", body)
	}
	store := access.NewStore(f.h.pool)
	admin := access.Bootstrap(context.Background())
	if err := store.CreateService(admin, "qualification-worker", "Qualification worker", f.resource,
		[]string{"operational.read", "sources.read", "connectors.execute"}, []string{sourceID}); err != nil {
		return "", "", fmt.Errorf("worker identity: %w", err)
	}
	_, token, err := store.IssueCredential(admin, "qualification-worker", f.resource, time.Now().Add(time.Hour))
	if err != nil {
		return "", "", fmt.Errorf("worker credential: %w", err)
	}
	locations, err := publicingest.NewRegistry([]publicingest.Location{{ID: "qualification-worker-location",
		SourceID: sourceID, Resource: f.resource, URL: "https://provider.example.invalid/worker",
		Adapter: "emad-json-page-v1", MaxRequestsPerDay: 10000}})
	if err != nil {
		return "", "", fmt.Errorf("worker location: %w", err)
	}
	for id, location := range locations {
		f.registry[id] = location
	}
	admission := f.admission
	admission.AnalystQuestion = "Isolated qualification worker restart rehearsal"
	admission.Rights.ID = "qualification-worker-rights"
	admission.ConnectorID = "qualification-worker-location"
	admission.ConnectorPrincipalID = "qualification-worker"
	status, body = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+sourceID+"/admission", admission, nil)
	if status != 201 {
		return "", "", fmt.Errorf("admission returned %d: %v", status, body)
	}
	profile := acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+sourceID+"/profiles/worker",
		publicingest.ProfileInput{Admission: admission}, 201)
	acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+sourceID+"/profiles/worker/execution",
		publicingest.ExecutionGrant{ProfileRevisionID: "rpr_" + profile["id"].(string),
			PrincipalID: "qualification-worker", Enabled: true}, 200)
	return sourceID, token, nil
}

// plannedWorkerRestart provisions the isolated worker source and runs one
// restart. A provisioning failure is recorded as NOT_RUN with its reason: a
// rehearsal that could not run is not evidence that restart is handled, and it
// must not abort the workload measurement it runs alongside.
func plannedWorkerRestart(t *testing.T, f *publicFixture) map[string]any {
	t.Helper()
	source, token, err := newQualificationWorkerSource(t, f)
	if err != nil {
		return map[string]any{"planned": 1, "performed": 0, "status": "NOT_RUN",
			"reason": "the isolated worker source could not be provisioned: " + err.Error()}
	}
	return runPlannedWorkerRestart(f.h.server.URL, token, source, time.Now().Add(10*time.Minute))
}

// runPlannedWorkerRestart stops one owned worker process and starts its
// replacement. The reported outcome is what actually happened.
func runPlannedWorkerRestart(base, token, source string, deadline time.Time) map[string]any {
	record := map[string]any{"planned": 1, "performed": 0, "status": "FAIL",
		"basis":     "a worker process owned by this workload was stopped and replaced; the replacement completed scheduled retrieval work",
		"isolation": "the worker child process talks only to the disposable isolated server and database of this run"}
	first, err := startQualificationWorker(base, token, source, "worker-1", deadline)
	if err != nil {
		record["status"] = "NOT_RUN"
		record["reason"] = "the worker process could not be started: " + err.Error()
		return record
	}
	startedAt := time.Now().UTC()
	action, ok := first.success(120 * time.Second)
	if !ok {
		first.stop()
		record["status"] = "FAIL"
		record["reason"] = "the first worker generation completed no successful scheduled retrieval"
		record["workerStderr"] = first.stderr.String()
		return record
	}
	firstRecord := map[string]any{"id": first.id, "pid": first.pid, "startedAt": startedAt.Format(time.RFC3339Nano),
		"firstSuccessAt": action["at"], "actions": 1}
	stoppedAt := time.Now().UTC()
	first.stop()
	second, err := startQualificationWorker(base, token, source, "worker-2", deadline)
	if err != nil {
		record["status"] = "FAIL"
		record["reason"] = "the replacement worker process could not be started: " + err.Error()
		record["firstWorker"] = firstRecord
		return record
	}
	restartedAt := time.Now().UTC()
	defer second.stop()
	replacement, ok := second.success(120 * time.Second)
	if !ok {
		record["status"] = "FAIL"
		record["reason"] = "the replacement worker generation completed no successful scheduled retrieval"
		record["firstWorker"] = firstRecord
		record["workerStderr"] = second.stderr.String()
		return record
	}
	record["performed"] = 1
	record["status"] = "PASS"
	record["startedAt"] = startedAt.Format(time.RFC3339Nano)
	record["stoppedAt"] = stoppedAt.Format(time.RFC3339Nano)
	record["restartedAt"] = restartedAt.Format(time.RFC3339Nano)
	record["firstWorker"] = firstRecord
	record["secondWorker"] = map[string]any{"id": second.id, "pid": second.pid,
		"restartedAt": restartedAt.Format(time.RFC3339Nano), "firstSuccessAt": replacement["at"]}
	return record
}

// processResidentBytes reports the resident set size of this isolated API
// process. It is the process working set, not container or cgroup memory.
func processResidentBytes() (uint64, error) {
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return 0, err
	}
	info, err := proc.MemoryInfo()
	if err != nil {
		return 0, err
	}
	return info.RSS, nil
}
