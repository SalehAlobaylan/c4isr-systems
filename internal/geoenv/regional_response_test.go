package geoenv

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
)

func TestRegionalCoastKeepsReceptionVisibleBeforePortWatch(t *testing.T) {
	day := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	receptionAt, portAt := day.AddDate(0, 0, 1), day.AddDate(0, 0, 2)
	count := 0
	coast := regionalCoast{Port: "Fixture port", PortWatchID: "port24", Coverage: []regionalReception{{
		SourceID: "reception", Day: day.Format(time.DateOnly), AvailableAt: receptionAt,
		WindowMinutes: 60, UsableMinutes: 45, PortCalls: &count, PortSourceID: "portwatch", PortAvailableAt: &portAt,
	}}}
	sources := map[string]bool{"reception": true, "portwatch": true}
	before, err := coast.review(receptionAt, "", sources)
	if err != nil || len(before.Coverage) != 1 || before.Coverage[0].UsableMinutes != 45 || before.Coverage[0].PortCalls != nil ||
		before.Coverage[0].PortSourceID != "" || before.Coverage[0].PortAvailableAt != nil {
		t.Fatalf("only reception evidence should be visible before PortWatch: %+v %v", before, err)
	}
	after, err := coast.review(portAt, "", sources)
	if err != nil || len(after.Coverage) != 1 || after.Coverage[0].PortCalls == nil ||
		*after.Coverage[0].PortCalls != 0 || after.Coverage[0].PortSourceID != "portwatch" {
		t.Fatalf("the recorded zero and its distinct provenance should appear on time: %+v %v", after, err)
	}
}

func TestRegionalResponseAccessAllocationAndCapacity(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/simulation-inputs/regional-response-exercise.json")
	if err != nil {
		t.Fatal(err)
	}
	read := func(minute int, slots *int) regionalResponse {
		t.Helper()
		out, err := replayRegionalResponse(raw, time.Date(2026, 11, 20, 8, minute, 0, 0, time.UTC), "", slots)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := read(18, nil)
	closed := read(40, nil)
	opened := read(55, nil)
	const reach = "reachable"
	if before.Service.Zones[1].Current.Outcome != reach || closed.Service.Zones[1].Reference.Outcome != reach ||
		closed.Service.Zones[1].Current.Outcome != "unreachable-in-graph" || opened.Service.Zones[1].Current.Outcome != reach ||
		closed.Service.Zones[0].Current.Outcome != reach || closed.Service.Zones[2].Current.Outcome != "outside-known-network" {
		t.Fatalf("only the ridge connection should change: %+v", closed.Service.Zones)
	}
	steady, peak, ridge := closed.Service.Workloads[0], closed.Service.Workloads[1], closed.Service.Workloads[2]
	if steady.Arrivals != 6 || peak.Arrivals != 6 || ridge.Arrivals != 6 || steady.Reachable != 3 ||
		peak.Disconnected != 2 || ridge.Disconnected != 4 || ridge.AccessUnknown != 1 || opened.Service.Workloads[0].Reachable != 5 ||
		steady.Reference == nil || steady.Reference.Reachable != 5 || steady.Reference.Disconnected != 0 || steady.Reference.Queues["none"].Waiting != 5 {
		t.Fatalf("unexpected allocation or access: %+v", closed.Service.Workloads)
	}
	if *steady.Queues["standard"].MedianWait >= *peak.Queues["standard"].MedianWait ||
		steady.Queues["none"].Waiting != 3 || steady.Queues["unknown"] != nil {
		t.Fatalf("expected peak pressure, known zero and unknown capacity: %+v", steady.Queues)
	}
	two := 2
	changed := read(40, &two)
	if *changed.Checks[0].Residual != 12 || len(changed.Indicators) != len(closed.Indicators) ||
		*changed.Indicators[3].Value != 367 || changed.Service.Workloads[1].Queues["analyst-override"].Completed != 3 {
		t.Fatalf("staffing assumptions must leave published references intact: %+v", changed)
	}
	if len(closed.Coast.Coverage) != 2 || closed.Coast.Coverage[0].UsableMinutes != 45 || *closed.Coast.Coverage[0].Duplicates != 1 ||
		closed.Coast.Coverage[1].UniqueReports != nil || closed.Coast.Notices[0].Revision != 1 ||
		opened.Coast.Notices[1].Status != "cancelled" || opened.Coast.Notices[0].ServiceImpact != "unknown" ||
		opened.Coast.Depth.Samples[2].ElevationM != nil {
		t.Fatalf("unexpected coverage, revision or sample status: %+v", opened.Coast)
	}
	if _, err := ReadRegionalResponse(raw, opened.Cutoff, strings.Repeat("0", 64), "", nil); err == nil {
		t.Fatal("incorrect pin was accepted")
	}
	if _, err := ReadRegionalResponse(raw, opened.Cutoff, artifacts.Hash(raw), "", nil); err != nil {
		t.Fatal(err)
	}
}

func TestRegionalResponseRejectsDifferentGrainAndUnpinnedCoast(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/simulation-inputs/regional-response-exercise.json")
	if err != nil {
		t.Fatal(err)
	}
	var input regionalExercise
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 11, 20, 8, 55, 0, 0, time.UTC)
	input.Coast.RetainedManifestSHA256 = strings.Repeat("a", 64)
	modified, _ := json.Marshal(input)
	if _, err := replayRegionalResponse(modified, cutoff, "", nil); err == nil || !strings.Contains(err.Error(), "pinned") {
		t.Fatalf("coast accepted with a different release: %v", err)
	}
	input.Indicators[1].RegionCode = "different-geography"
	modified, _ = json.Marshal(input)
	if _, err := replayRegionalResponse(modified, cutoff, input.Coast.RetainedManifestSHA256, nil); err == nil || !strings.Contains(err.Error(), "grain") {
		t.Fatalf("incompatible indicator grains accepted: %v", err)
	}
}
