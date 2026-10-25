package scenarios

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/testsupport"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestInformationClockSeparatesExerciseFromPublicAndWallTime(t *testing.T) {
	repo := newMemoryScenarioRepository()
	epoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	repo.Create(testsupport.Context(), Run{ID: "clock-run", Status: StatusPaused, VirtualTimeMs: 1250, Frozen: FrozenRun{Clock: ClockConfig{Epoch: epoch}}})
	svc := NewService(repo, nil, EngineDeps{}, nil)
	r := access.LegacyResource()
	r.RunID = "clock-run"
	ctx, err := access.Rebind(testsupport.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	wall := time.Now().Add(24 * time.Hour)
	if got := svc.ClockNow(ctx, wall); !got.Equal(epoch.Add(1250 * time.Millisecond)) {
		t.Fatal("exercise followed wall clock", got)
	}
	public := access.Resource{WorkspaceID: "public", EnvironmentID: "test", Partition: access.PublicEvidence}
	p := access.Principal{ID: "public-reader", Kind: "human", Grants: []access.Grant{{Resource: public}}}
	ctx, err = access.Bind(context.Background(), p, public, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := svc.ClockNow(ctx, wall); !got.Equal(wall) {
		t.Fatal("public evidence followed exercise clock", got)
	}
}

func TestRestartUsesFrozenDefinitionAfterAuthorEditsAndDeletesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frozen.yaml")
	raw := []byte("scenario: frozen\nseed: 17\nevents:\n  - after: 1h\n    action: incident\n    incident_ref: original\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	repo := newMemoryScenarioRepository()
	svc := NewService(repo, NewLoader(dir), EngineDeps{}, nil)
	defer svc.Close(testsupport.Context())
	first, err := svc.Start(testsupport.Context(), "frozen", StartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("scenario: changed\nseed: 99\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Restart(testsupport.Context(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Frozen.VersionID != second.Frozen.VersionID || second.Seed != 17 || second.Frozen.RestartOf != first.ID || !second.Frozen.Clock.Epoch.Equal(first.Frozen.Clock.Epoch) {
		t.Fatalf("restart changed inputs: %+v %+v", first, second)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	third, err := svc.Restart(testsupport.Context(), second.ID)
	if err != nil {
		t.Fatal(err)
	}
	v, err := svc.Version(testsupport.Context(), third.ID)
	if err != nil || v.Definition.Events[0].IncidentRef != "original" {
		t.Fatalf("lost frozen authoring input: %+v %v", v, err)
	}
}
func TestPausedClockAndSpeedChangesPreserveSeededDeclaredOutput(t *testing.T) {
	type output struct {
		Sequence int
		Instant  time.Time
		Position geo.Point
	}
	run := func(changes bool) []output {
		repo := newMemoryScenarioRepository()
		id := "test"
		repo.Create(testsupport.Context(), Run{ID: id, Status: StatusRunning})
		e := newEngine(id, NewRunScope(id), &Scenario{}, 77, 1, EngineDeps{}, repo, nil)
		e.epoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		out := []output{}
		for i, at := range []int64{10, 10, 30, 50} {
			seq := i + 1
			e.eventStates[seq] = EventInspection{Sequence: seq, AtMs: at, Status: EventPending}
			e.static = append(e.static, scheduledAction{AtMs: at, Seq: seq, Name: "sample", Run: func(context.Context) error {
				out = append(out, output{seq, e.scenarioInstant(), e.jitter(geo.Point{Lat: 21, Lng: 39}, 5)})
				if changes {
					e.SetSpeed(20)
				}
				return nil
			}})
		}
		if err := e.persistStaticEvents(testsupport.Context()); err != nil {
			t.Fatal(err)
		}
		if changes {
			e.Pause()
			before := e.VirtualTimeMs()
			time.Sleep(20 * time.Millisecond)
			e.SetSpeed(5)
			time.Sleep(10 * time.Millisecond)
			if e.VirtualTimeMs() != before {
				t.Fatal("paused clock advanced")
			}
			e.Resume()
		}
		if err := e.run(testsupport.Context()); err != nil {
			t.Fatal(err)
		}
		last := e.VirtualTimeMs()
		time.Sleep(3 * time.Millisecond)
		if e.VirtualTimeMs() != last {
			t.Fatal("completed clock advanced")
		}
		return out
	}
	first, second := run(false), run(true)
	if !reflect.DeepEqual(first, second) {
		a, _ := json.Marshal(first)
		b, _ := json.Marshal(second)
		t.Fatalf("speed affected declared output:\n%s\n%s", a, b)
	}
}
