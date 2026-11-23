package geoenv

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
)

func TestAirportNoticesKeepScheduledAndActiveIdentitiesSeparate(t *testing.T) {
	cutoff := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	active := airportNotice{ID: "roadwork:scheduled", DeliveryID: "active", SourceID: "fixture",
		Publisher: "Fixture", Title: "Civil road work", Kind: "service-notice", Status: "active", Revision: 1,
		PublishedAt: cutoff.Add(-time.Hour), AvailableAt: cutoff, EffectiveAt: cutoff}
	future := active
	future.ID, future.DeliveryID, future.EffectiveAt = "roadwork", "future", cutoff.Add(time.Hour)
	notices, err := airportNoticesAt([]airportNotice{active, future}, map[string]bool{"fixture": true}, cutoff)
	if err != nil || len(notices) != 2 || notices[0].ID != active.ID || notices[0].Status != "active" ||
		notices[1].ID != future.ID || notices[1].Status != "scheduled" {
		t.Fatalf("distinct notice IDs and scheduling must be preserved: %+v %v", notices, err)
	}
}

func TestAerialReviewUsesOnlyAvailableEvidence(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/simulation-inputs/civilian-aerial-exercise.json")
	if err != nil {
		t.Fatal(err)
	}
	replay := func(hour, minute int) aerialReview {
		t.Helper()
		cutoff := time.Date(2026, 11, 18, hour, minute, 0, 0, time.UTC)
		out, err := reviewAerialExercise(raw, cutoff)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	early := replay(9, 25)
	if early.ImageStatus != "needs-another-observation" || early.Before != nil || early.After != nil ||
		len(early.Candidates) != 4 || early.Candidates[1].CatalogCoverage != "partial-page" ||
		early.Candidates[1].State != "assets-unavailable" || early.Candidates[2].State != "usable-area-below-threshold" ||
		early.Candidates[3].State != "insufficient-resolution" || len(early.Briefing) != 1 || len(early.Energy) != 0 {
		t.Fatalf("unexpected early evidence: %+v", early)
	}
	if len(early.Notices) != 3 || early.Notices[0].Status != "active" || len(early.Notices[0].DeliveryIDs) != 2 ||
		early.Notices[1].ID != "edition-1" || early.Notices[2].Status != "scheduled" {
		t.Fatalf("early editions or replicated notices: %+v", early.Notices)
	}
	if len(early.Traffic) != 2 || early.Traffic[0].CivilCount == nil || *early.Traffic[0].CivilCount != 0 || early.Traffic[1].CivilCount != nil {
		t.Fatalf("known zero differs from missing coverage: %+v", early.Traffic)
	}
	late := replay(10, 5)
	if late.ImageStatus != "paired" || late.Before.ID != "before" || late.After.ID != "after" || len(late.Briefing) != 2 || len(late.Energy) != 1 {
		t.Fatalf("unexpected late evidence: %+v", late)
	}
	withPair := replay(9, 42)
	if withPair.After == nil || len(withPair.After.Assets) != 2 || len(late.After.Assets) != 3 || late.After.Assets[2].Role != "flood-overlay" {
		t.Fatal("optional overlay must appear only at its own availability time")
	}
	if len(late.Assessments) != 3 || !late.Assessments[0].SharesScene || late.Assessments[1].Affected != nil ||
		late.Assessments[2].Version != 2 || *late.Assessments[2].Affected || len(late.Assessments[2].SourceImageTimes) != 1 {
		t.Fatalf("assessment provenance or cutoff: %+v", late.Assessments)
	}
	if len(late.Notices) != 2 || late.Notices[0].Status != "cancelled" || late.Notices[1].ID != "edition-2" {
		t.Fatalf("cancellation and edition boundary: %+v", late.Notices)
	}
	if *late.Survey != (surveyReview{Assignments: 3, Aborted: 1, Required: 4, Covered: 3, Duplicate: 1}) {
		t.Fatalf("survey totals: %+v", late.Survey)
	}
	if _, err := ReadAerialReview(raw, late.Cutoff, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong package pin was accepted")
	}
	if _, err := ReadAerialReview(raw, late.Cutoff, artifacts.Hash(raw)); err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Replace(raw, []byte("iVBOR"), []byte("mVBOR"), 1)
	if _, err := reviewAerialExercise(corrupt, late.Cutoff); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("modified asset accepted: %v", err)
	}
}
