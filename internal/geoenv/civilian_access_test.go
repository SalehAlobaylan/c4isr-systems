package geoenv

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"
)

func ownedAccessExercise(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/simulation-inputs/riyadh-access-exercise.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func atAccessTime(t *testing.T, raw []byte, clock string) civilianAccessReplay {
	t.Helper()
	cutoff, err := time.Parse(time.RFC3339, "2026-11-21T"+clock+"Z")
	if err != nil {
		t.Fatal(err)
	}
	result, err := replayCivilianAccess(raw, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCivilianReplayPreservesCutoffAndSourceMeaning(t *testing.T) {
	raw := ownedAccessExercise(t)
	before := atAccessTime(t, raw, "06:08:00")
	if len(before.Reports) != 3 || len(before.Associations) != 3 ||
		before.Associations[0].Decision != "suggested" || len(before.Inspections) != 0 || before.Reports[0].TicketClosedAt != nil ||
		len(before.Assignments) != 1 || before.Assignments[0].ArrivedAt != nil {
		t.Fatalf("future evidence appeared in the queue: %+v", before)
	}
	planned := atAccessTime(t, raw, "06:45:00")
	if planned.AssociationChecks.ReviewedPairs != 3 || planned.AssociationChecks.FalseCandidates != 2 ||
		planned.AssociationChecks.CorrectCandidates != 1 {
		t.Fatalf("the district rule needs separate reporting of false suggestions: %+v", planned.AssociationChecks)
	}
	zone := planned.Destinations[0]
	if zone.Observed.VerifiedDistanceM == nil || *zone.Observed.VerifiedDistanceM != 230 ||
		!slices.Equal(zone.Observed.EdgeIDs, []string{"a-detour", "detour-b"}) ||
		zone.WithPlannedWork.Outcome != "unreachable-in-graph" || planned.Edges[0].State != "closed" ||
		planned.Reports[0].TicketClosedAt == nil || planned.Associations[0].Decision != "linked" ||
		planned.Associations[1].Decision != "separate" {
		t.Fatalf("wrong detour, ticket or review: %+v", planned)
	}
	if planned.Reports[3].PlaceLabel != "" {
		t.Fatal("unlocated report was assigned a place")
	}
	afterCancel := atAccessTime(t, raw, "06:55:00")
	if afterCancel.Works[0].Status != "cancelled" || afterCancel.Restoration[0].CompletedAt != nil || afterCancel.Edges[0].State != "closed" ||
		afterCancel.Destinations[0].WithPlannedWork.Outcome != "reachable" {
		t.Fatalf("cancelled works must leave observed closure independent: %+v", afterCancel)
	}
	reopened := atAccessTime(t, raw, "07:05:00")
	if reopened.Restoration[0].CompletedAt == nil || reopened.Restoration[0].Currency != "SAR" ||
		reopened.Restoration[0].PricingYear != 2026 || reopened.Restoration[1].CompletedAt != nil {
		t.Fatalf("wrong facility task availability or price basis: %+v", reopened.Restoration)
	}
	if reopened.Edges[0].State != "open" || reopened.Edges[1].State != "closed" ||
		*reopened.Destinations[0].Observed.VerifiedDistanceM != 120 ||
		reopened.Destinations[1].Observed.Outcome != "reachable" ||
		reopened.Destinations[2].Observed.Outcome != "unknown" ||
		reopened.Destinations[3].Observed.Outcome != "outside-known-network" {
		t.Fatalf("wrong directional or entrance restoration: %+v", reopened)
	}
	a, b, c := reopened.Destinations[0], reopened.Destinations[1], reopened.Destinations[2]
	if *a.KnownPeople != 90 || *b.KnownPeople != 30 || c.UnknownCells != 1 || *c.KnownPeople != 0 ||
		*reopened.UnallocatedPeople != 10 {
		t.Fatalf("population not conserved or no-data confused with zero: %+v", reopened.Destinations)
	}
}

func TestCivilianReplayHandlesTurnRestrictionAndDisputedAccess(t *testing.T) {
	var exercise accessExercise
	if err := json.Unmarshal(ownedAccessExercise(t), &exercise); err != nil {
		t.Fatal(err)
	}
	exercise.TurnRestrictions = []accessTurn{{FromEdge: "a-b", ToEdge: "to-west"}}
	conflicting := exercise.Inspections[4]
	conflicting.ID, conflicting.Access, conflicting.Reviewer = "other-inspection", "closed", "independent-review"
	exercise.Inspections = append(exercise.Inspections, conflicting)
	raw, err := json.Marshal(exercise)
	if err != nil {
		t.Fatal(err)
	}
	disputed := atAccessTime(t, raw, "07:05:00")
	if disputed.Edges[0].State != "disputed" || len(disputed.Edges[0].EvidenceIDs) != 2 ||
		*disputed.Destinations[0].Observed.VerifiedDistanceM != 230 {
		t.Fatalf("disputed edge must stay unresolved: %+v", disputed)
	}
	exercise.Inspections = exercise.Inspections[:len(exercise.Inspections)-1]
	raw, err = json.Marshal(exercise)
	if err != nil {
		t.Fatal(err)
	}
	restricted := atAccessTime(t, raw, "07:05:00").Destinations[1].Observed
	if restricted.VerifiedDistanceM == nil || *restricted.VerifiedDistanceM != 237 ||
		!slices.Equal(restricted.EdgeIDs, []string{"a-detour", "detour-b", "to-west"}) {
		t.Fatalf("prohibited turn must leave the longer route available: %+v", restricted)
	}
}

func TestCivilianReplaySourceBackedPilot(t *testing.T) {
	path := os.Getenv("EMAD_TEST_RIYADH_ACCESS_PILOT")
	if path == "" {
		t.Skip("provider-backed local pilot not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before := atAccessTime(t, raw, "06:45:00")
	after := atAccessTime(t, raw, "07:05:00")
	if before.Kind != "authored-exercise" || before.Destinations[0].Observed.Outcome != "unreachable-in-graph" ||
		after.Destinations[0].Observed.Outcome != "reachable" ||
		after.Destinations[0].KnownPeople == nil || len(after.Reports) != 2 ||
		after.RetainedManifestSHA256 == "" || after.Edges[0].RetainedOSMFeatureID == "" ||
		after.Works[0].EdgeID != "" || after.Destinations[1].Observed.Outcome != "outside-known-network" {
		t.Fatalf("source preparation conflated independent claims or lost native identity: %+v", after)
	}
}
