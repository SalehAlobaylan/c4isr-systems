package integration

// This is a repeatable isolated operational sample, not the full MAP-6 dataset
// or a substitute for observed analysts. Longer wall-clock runs use the same
// fixed synthetic provider budgets and never touch the local installation.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

func TestMapFirstQualificationConcurrentSample(t *testing.T) {
	if os.Getenv("C4ISR_QUALIFICATION") != "1" {
		t.Skip("opt-in isolated workload")
	}
	duration := 5 * time.Second
	if value := os.Getenv("C4ISR_QUALIFICATION_DURATION"); value != "" {
		var err error
		duration, err = time.ParseDuration(value)
		if err != nil || duration < time.Second || duration > 25*time.Hour {
			t.Fatal("duration must be 1s–25h")
		}
	}
	dir := os.Getenv("C4ISR_TEST_EVIDENCE_DIR")
	if dir == "" {
		t.Fatal("evidence directory required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "workload.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	var mu sync.Mutex
	emit := func(v any) {
		mu.Lock()
		defer mu.Unlock()
		if err := enc.Encode(v); err != nil {
			t.Error("write workload evidence", err)
		}
	}
	f := newPublicFixture(t)
	f.admission.Policy.MaxRequestsPerDay = 10000
	f.admission.Policy.MinIntervalSeconds = 1
	loc := f.registry["fixture-location"]
	loc.MaxRequestsPerDay = 10000
	f.registry["fixture-location"] = loc
	f.admit(t)
	// Renew credentials before the measured interval, including a real 24h run.
	store := access.NewStore(f.h.pool)
	admin := access.Bootstrap(context.Background())
	issue := func(id string) string {
		_, token, e := store.IssueCredential(admin, id, f.resource, time.Now().Add(48*time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		return token
	}
	f.manager = issue("source-reviewer")
	f.worker = issue("connector-fixture")
	for i := 1; i < 10; i++ {
		key := fmt.Sprintf("qualification-%02d", i)
		input := f.admission
		input.AnalystQuestion = "Isolated acceptance schedule"
		input.Rights.ID = "acceptance-terms-" + key
		p := acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+f.source+"/profiles/"+key, publicingest.ProfileInput{Admission: input}, 201)
		acquisitionCall(t, f, f.manager, "POST", "/api/v1/sources/"+f.source+"/profiles/"+key+"/execution", publicingest.ExecutionGrant{ProfileRevisionID: "rpr_" + p["id"].(string), PrincipalID: "connector-fixture", Enabled: true}, 200)
	}
	// One thousand explicitly classified retained-history objects in ten areas.
	raw := `{"method":"acceptance-only-history","aoi":{"west":46,"south":24,"east":47,"north":25}}`
	_, err = f.h.pool.Exec(context.Background(), `INSERT INTO imagery_capture_jobs(id,scope_id,source_id,input,status,created_by) SELECT 'qualification-history-'||i,$1,$2,$3::jsonb||jsonb_build_object('acceptanceArea',i%10),'CANCELLED','source-reviewer' FROM generate_series(1,1000)i`, f.resource.ID(), f.source, raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.h.pool.Exec(context.Background(), `INSERT INTO content_purpose_revisions(id,scope_id,entity_kind,entity_id,source_id,version,purpose,reason,receipt_reference,actor_id) SELECT 'qualification-purpose-'||i,$1,'capture','qualification-history-'||i,$2,1,'acceptance','Isolated synthetic workload','qualification-profile-v1','source-reviewer' FROM generate_series(1,1000)i`, f.resource.ID(), f.source)
	if err != nil {
		t.Fatal(err)
	}
	tokens := []string{}
	for i := 0; i < 5; i++ {
		human := fmt.Sprintf("qualification-human-%d", i)
		principal := fmt.Sprintf("qualification-analyst-%d", i)
		_, err = f.h.pool.Exec(context.Background(), `INSERT INTO operators(id,name,role)VALUES($1,'Synthetic acceptance identity','operator')`, human)
		if err != nil {
			t.Fatal(err)
		}
		err = store.CreateHuman(admin, principal, human, access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "sources.manage", "observations.read"}})
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, issue(principal))
	}
	emit(map[string]any{"kind": "profile", "schema": "emad.workload-sample.v1", "startedAt": time.Now().UTC(), "durationSeconds": duration.Seconds(), "analysts": 5, "scheduledProfiles": 10, "historyRows": 1000, "areas": 10, "reportRevisions": 0, "detailedReferenceFeatures": 0, "weatherMovingRecords": 0, "synthetic": true, "scope": "Reduced isolated operational sample; full scale and browser latency are separate gates", "goVersion": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "providerDailyLimit": 10000, "profileIntervalSeconds": 1})
	client := &http.Client{Timeout: 5 * time.Second}
	var failures int
	request := func(token, operation, method, path string, body any, cold bool) map[string]any {
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, f.h.server.URL+path, bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		started := time.Now()
		response, e := client.Do(req)
		status, size := 0, 0
		value := map[string]any{}
		correlation := ""
		if e == nil {
			status = response.StatusCode
			correlation = response.Header.Get("X-Request-ID")
			b, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
			response.Body.Close()
			size = len(b)
			e = readErr
			if e == nil {
				e = json.Unmarshal(b, &value)
			}
		}
		ok := e == nil && status >= 200 && status < 300
		if operation == "catalog_search" && value["total"] != float64(1000) {
			ok = false
		}
		if operation == "scope_resolve" {
			if hash, yes := value["scopeHash"].(string); !yes || hash == "" {
				ok = false
			}
		}
		if code, exists := value["failureCode"]; exists && code != "" && code != nil {
			ok = false
		}
		timedOut := false
		if e != nil {
			if timeout, yes := e.(interface{ Timeout() bool }); yes {
				timedOut = timeout.Timeout()
			}
		}
		mu.Lock()
		if !ok {
			failures++
		}
		mu.Unlock()
		emit(map[string]any{"kind": "request", "operation": operation, "cold": cold, "elapsedMs": float64(time.Since(started).Microseconds()) / 1000, "success": ok, "timeout": timedOut, "status": status, "bytes": size, "correlationId": correlation})
		return value
	}
	// All ten schedules are actually claimed through the scheduler, respecting
	// trusted account pacing; no fixture clock/cost reset occurs during the run.
	for i := 0; i < 10; i++ {
		request(f.worker, "scheduled_retrieval", "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, true)
		time.Sleep(1100 * time.Millisecond)
	}
	var attempted int
	if err = f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM retrieval_profile_state WHERE last_attempt_at IS NOT NULL`).Scan(&attempted); err != nil || attempted != 10 {
		t.Fatalf("ten distinct schedules did not run: %d %v", attempted, err)
	}
	start := time.Now()
	end := start.Add(duration)
	var wg sync.WaitGroup
	for _, token := range tokens {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			snapshot := ""
			cold := true
			for time.Now().Before(end) {
				v := request(token, "catalog_search", "POST", "/api/v1/catalog/reads?content_view=all", map[string]any{"kind": "captures", "limit": 100, "replaceSnapshotId": snapshot}, cold)
				if id, ok := v["snapshotId"].(string); ok {
					snapshot = id
				}
				request(token, "scope_resolve", "POST", "/api/v1/analyst-scope/resolve", caseInput().Scope, cold)
				cold = false
				time.Sleep(time.Second)
			}
		}(token)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		next := start.Add(12 * time.Second)
		for time.Now().Before(end) {
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			var dbBytes, queued int64
			var oldestQueue, freshnessAge *float64
			e := f.h.pool.QueryRow(context.Background(), `SELECT pg_database_size(current_database()),(SELECT count(*) FROM delivery_jobs WHERE state IN('QUEUED','RUNNING')),(SELECT max(EXTRACT(EPOCH FROM clock_timestamp()-created_at))::float8 FROM delivery_jobs WHERE state IN('QUEUED','RUNNING')),(SELECT max(EXTRACT(EPOCH FROM clock_timestamp()-last_success_at))::float8 FROM retrieval_profile_state)`).Scan(&dbBytes, &queued, &oldestQueue, &freshnessAge)
			emit(map[string]any{"kind": "telemetry", "elapsedSeconds": time.Since(start).Seconds(), "heapBytes": mem.HeapAlloc, "dbBytes": dbBytes, "deliveryQueue": queued, "oldestDeliveryQueueSeconds": oldestQueue, "oldestRetrievalSuccessAgeSeconds": freshnessAge, "goroutines": runtime.NumGoroutine(), "success": e == nil})
			if time.Now().After(next) {
				request(f.worker, "scheduled_retrieval", "POST", "/api/v1/sources/"+f.source+"/scheduler/run", nil, false)
				next = next.Add(12 * time.Second)
			}
			time.Sleep(time.Second)
		}
	}()
	wg.Wait()
	emit(map[string]any{"kind": "end", "completed": true, "elapsedSeconds": time.Since(start).Seconds(), "failures": failures, "workerRestart": "NOT_RUN", "fullWorkload": "NOT_RUN", "observedHumans": "NOT_RUN"})
	if failures != 0 {
		t.Fatalf("workload retained %d failures (see workload.jsonl)", failures)
	}
}
