package geoenv

import (
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
)

func TestSupportingSourcesKeepsRevisionsCoverageAndQueuesSeparate(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/simulation-inputs/supporting-evidence-exercise.json")
	if err != nil {
		t.Fatal(err)
	}
	review := func(hour, minute int) supportOutput {
		t.Helper()
		out, err := replaySupportingSources(raw, time.Date(2026, 11, 20, hour, minute, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	early, late := review(8, 10), review(10, 0)
	find := func(output supportOutput, id string) bool {
		return slices.ContainsFunc(output.Fields, func(f supportField) bool { return f.ID == id })
	}
	if find(early, "run-later") || find(early, "archive") || !find(early, "run-first") || !find(late, "run-later") || !find(late, "archive") {
		t.Fatal("forecast runs or archive appeared at the wrong cutoff")
	}
	if !slices.ContainsFunc(early.Events, func(e supportEvent) bool { return e.Kind == "image-opportunity" }) ||
		slices.ContainsFunc(early.Events, func(e supportEvent) bool { return e.Kind == "imagery" }) ||
		!slices.ContainsFunc(late.Events, func(e supportEvent) bool { return e.Kind == "imagery" }) {
		t.Fatal("possible opportunity must wait for an independently published scene")
	}
	if len(late.Events) != 18 || late.UniqueOrigins != 11 || countSupportEvents(late.Events, "flood-bulletin") != 4 || countSupportEvents(late.Events, "own-field-verdict") != 1 {
		t.Fatalf("event editions or source lineage: %d events, %d origins", len(late.Events), late.UniqueOrigins)
	}
	if len(late.Comparisons) != 5 || math.Abs(*late.Comparisons[0].Difference-.2) > 1e-9 ||
		late.Comparisons[1].Status != "unavailable" || late.Comparisons[2].Status != "unavailable" || late.Comparisons[3].Status != "scope-changed" {
		t.Fatalf("incorrect measurement or indicator comparison: %+v", late.Comparisons)
	}
	if early.Delivery[0].Pending != 0 || early.Delivery[1].Pending != 3 || early.Delivery[1].RecoveryStatus != "unresolved" ||
		late.Delivery[1].PeakBacklog != 3 || late.Delivery[1].Pending != 0 || *late.Delivery[1].RecoverySeconds != 180 ||
		*late.Delivery[0].P90LagSeconds != *late.Delivery[2].P90LagSeconds || early.Delivery[1].Receipts[2].ReceivedAt != nil ||
		late.Delivery[1].Receipts[2].AcknowledgedAt != nil {
		t.Fatalf("incorrect queue receipt, lag or recovery: %+v", late.Delivery)
	}
	if _, err := ReadSupportingSources(raw, late.Cutoff, strings.Repeat("a", 64)); err == nil {
		t.Fatal("accepted wrong file pin")
	}
	if _, err := ReadSupportingSources(raw, late.Cutoff, artifacts.Hash(raw)); err != nil {
		t.Fatal(err)
	}
}

func countSupportEvents(events []supportEvent, origin string) int {
	n := 0
	for _, event := range events {
		if event.Origin == origin {
			n++
		}
	}
	return n
}
