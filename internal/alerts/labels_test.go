package alerts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/classifications"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
)

type fakeClassifier struct {
	latest   *classifications.Classification
	err      error
	trackIDs []string
}

func (f *fakeClassifier) LatestForTrack(_ context.Context, trackID string) (*classifications.Classification, error) {
	f.trackIDs = append(f.trackIDs, trackID)
	if f.err != nil {
		return nil, f.err
	}
	return f.latest, nil
}

func confidence(value float64) *float64 { return &value }

func TestBreachTitleNamesTheSubjectNotTheIdentifier(t *testing.T) {
	classifier := &fakeClassifier{latest: &classifications.Classification{
		TrackID:    "trk_01m2z50fzhyrcdb3832nxcpycv",
		Label:      "vehicle",
		Confidence: confidence(0.91),
	}}

	subject := DescribeTrack(context.Background(), "trk_01m2z50fzhyrcdb3832nxcpycv", classifier)
	title := subject.BreachTitle("Restricted Zone A")

	if strings.Contains(title, "trk_01m2z50fzhyrcdb3832nxcpycv") {
		t.Fatalf("title leaks a track identifier: %q", title)
	}
	if want := "Vehicle entered Restricted Zone A"; title != want {
		t.Fatalf("title = %q, want %q", title, want)
	}
	if !strings.Contains(subject.BreachMessage("restricted", "Restricted Zone A"), "Restricted Zone A") {
		t.Fatalf("message should name the geofence: %q", subject.BreachMessage("restricted", "Restricted Zone A"))
	}
}

func TestBreachTitleHedgesLowConfidence(t *testing.T) {
	classifier := &fakeClassifier{latest: &classifications.Classification{
		Label:      "vehicle",
		Confidence: confidence(0.4),
	}}

	subject := DescribeTrack(context.Background(), "trk_1", classifier)
	if subject.Confirmed {
		t.Fatal("a 0.4 confidence classification must not read as confirmed")
	}
	if want := "Suspected vehicle entered Restricted Zone A"; subject.BreachTitle("Restricted Zone A") != want {
		t.Fatalf("title = %q, want %q", subject.BreachTitle("Restricted Zone A"), want)
	}
}

func TestDescribeTrackWithoutConfidenceDoesNotClaimCertainty(t *testing.T) {
	classifier := &fakeClassifier{latest: &classifications.Classification{Label: "drone"}}

	subject := DescribeTrack(context.Background(), "trk_1", classifier)
	if subject.Label != "drone" {
		t.Fatalf("label = %q, want drone", subject.Label)
	}
	if subject.Confirmed {
		t.Fatal("a classification with no confidence must be hedged")
	}
}

func TestDescribeTrackWithoutClassificationStaysHonest(t *testing.T) {
	cases := []struct {
		name       string
		classifier ClassifierLabeler
	}{
		{name: "no classifier wired", classifier: nil},
		{name: "no classification recorded", classifier: &fakeClassifier{}},
		{name: "classification has a blank label", classifier: &fakeClassifier{latest: &classifications.Classification{Label: "   "}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			subject := DescribeTrack(context.Background(), "trk_1", tc.classifier)
			if subject.Label != "Unidentified contact" {
				t.Fatalf("label = %q, want %q", subject.Label, "Unidentified contact")
			}
			if subject.Confirmed {
				t.Fatal("an unclassified track must never be reported as confirmed")
			}
			if title := subject.BreachTitle("Restricted Zone A"); strings.Contains(title, "trk_") {
				t.Fatalf("title leaks an identifier: %q", title)
			}
		})
	}
}

func TestDescribeTrackSurvivesClassifierFailure(t *testing.T) {
	// A degraded classification read must not raise, and must not invent a
	// name: an operator who trusts a label acts on it.
	classifier := &fakeClassifier{err: errors.New("database unavailable")}

	subject := DescribeTrack(context.Background(), "trk_1", classifier)
	if subject.Label != "Unidentified contact" || subject.Confirmed {
		t.Fatalf("subject = %+v, want unidentified and unconfirmed", subject)
	}
}

func TestBreachTitleFallsBackWhenGeofenceIsUnnamed(t *testing.T) {
	subject := Subject{Label: "Vehicle", Confirmed: true}
	for _, name := range []string{"", "   "} {
		title := subject.BreachTitle(name)
		if !strings.Contains(title, "a restricted zone") {
			t.Fatalf("title = %q, want a generic zone reference", title)
		}
		if strings.Contains(title, "  ") {
			t.Fatalf("title has a double space: %q", title)
		}
	}
	if got := subject.BreachMessage("", ""); !strings.Contains(got, "a restricted zone") {
		t.Fatalf("message = %q, want a generic zone reference", got)
	}
}

func TestHandleGeofenceBreachedStoresResolvedSubjectOnTheAlert(t *testing.T) {
	repo := &fakeRepository{}
	classifier := &fakeClassifier{latest: &classifications.Classification{
		Label:      "vehicle",
		Confidence: confidence(0.95),
	}}

	bus := events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := NewServiceWithClassifier(repo, bus, classifier)

	svc.HandleGeofenceBreached(context.Background(), events.GeofenceBreached{
		GeofenceID:   "gf_1",
		GeofenceName: "Restricted Zone A",
		GeofenceType: "restricted",
		Severity:     "high",
		TrackID:      "trk_01m2z50fzhyrcdb3832nxcpycv",
	})

	if len(repo.createdCalls) != 1 {
		t.Fatalf("created %d alerts, want 1", len(repo.createdCalls))
	}
	created := repo.createdCalls[0]

	if strings.Contains(created.Title, "trk_") {
		t.Errorf("alert title leaks a track identifier: %q", created.Title)
	}
	if strings.Contains(created.Message, "trk_") {
		t.Errorf("alert message leaks a track identifier: %q", created.Message)
	}
	if want := "Vehicle entered Restricted Zone A"; created.Title != want {
		t.Errorf("title = %q, want %q", created.Title, want)
	}
	// The resolved subject is recorded so provenance explains the wording, and
	// so the alert keeps explaining itself if the track is later reclassified.
	if got := created.SourceReference["subjectLabel"]; got != "vehicle" {
		t.Errorf("sourceReference subjectLabel = %v, want vehicle", got)
	}
	if got := created.SourceReference["subjectConfirmed"]; got != true {
		t.Errorf("sourceReference subjectConfirmed = %v, want true", got)
	}
	// The evidence link is preserved regardless of how the alert is worded.
	if created.TrackID != "trk_01m2z50fzhyrcdb3832nxcpycv" {
		t.Errorf("alert lost its track reference: %q", created.TrackID)
	}
	if len(classifier.trackIDs) != 1 || classifier.trackIDs[0] != created.TrackID {
		t.Errorf("classifier asked about %v, want [%s]", classifier.trackIDs, created.TrackID)
	}
}

func TestHandleGeofenceBreachedWithoutClassifierStillReadsAsProse(t *testing.T) {
	repo := &fakeRepository{}
	svc, _ := newTestService(repo)

	svc.HandleGeofenceBreached(context.Background(), events.GeofenceBreached{
		GeofenceID:   "gf_1",
		GeofenceName: "Restricted Zone A",
		GeofenceType: "restricted",
		Severity:     "high",
		TrackID:      "trk_01m2z50fzhyrcdb3832nxcpycv",
	})

	created := repo.createdCalls[0]
	if strings.Contains(created.Title, "trk_") {
		t.Errorf("alert title leaks a track identifier: %q", created.Title)
	}
	if want := "Suspected unidentified contact entered Restricted Zone A"; created.Title != want {
		t.Errorf("title = %q, want %q", created.Title, want)
	}
}
