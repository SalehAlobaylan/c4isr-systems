package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

func TestFreshnessMonitorKeepsIndependentRunBaselines(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeRepository{}
	monitor, log := newTestMonitor(t, repo, nil, now)
	for _, run := range []string{"a", "b", "a", "b"} {
		r := access.Resource{WorkspaceID: "exercise", EnvironmentID: "lab", Partition: access.ExerciseObservation, RunID: run}
		ctx := access.Internal(context.Background(), r, "freshness-test")
		repo.freshnessSignals = []FreshnessSignal{{AssetID: run, Status: "available", ConnectionState: "connected", LastSeenAt: &now}}
		if err := monitor.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
	}
	transitions := freshnessEvents(log)
	if len(transitions) != 2 || transitions[0].EntityID != "a" || transitions[1].EntityID != "b" {
		t.Fatalf("switching runs emitted disappearance or repeated transitions: %+v", transitions)
	}
}
