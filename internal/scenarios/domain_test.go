package scenarios

import (
	"math"
	"strings"
	"testing"
)

func TestValidatePhase18Actions(t *testing.T) {
	validPosition := Position{Lat: 24.71, Lng: 46.67}
	scenario := Scenario{
		Name:    "phase18",
		Sources: []SourceSpec{{ID: "radar"}},
		Assets:  []AssetSpec{{ID: "asset-1"}},
		Tracks:  []TrackSpec{{ID: "track-1", Start: &validPosition}},
		Events: []EventSpec{
			{Action: ActionObserve, Source: "radar", Track: "track-1", Fault: "duplicate"},
			{Action: ActionAssessment, SubjectType: "track", SubjectID: "track-1", AssessmentType: "identity", Conclusion: "unknown"},
			{Action: ActionAssetStatus, Asset: "asset-1", Status: "unavailable"},
			{Action: ActionIncident, IncidentRef: "incident-1", IncidentTitle: "Review"},
		},
	}
	if err := scenario.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsUnknownFaultAndBadAssetStatus(t *testing.T) {
	base := Scenario{
		Name:    "phase18-invalid",
		Assets:  []AssetSpec{{ID: "asset-1"}},
		Tracks:  []TrackSpec{{ID: "track-1"}},
		Sources: []SourceSpec{{ID: "source-1"}},
	}
	for name, test := range map[string]struct {
		event EventSpec
		want  string
	}{
		"fault": {
			event: EventSpec{Action: ActionObserve, Source: "source-1", Track: "track-1", Fault: "not-a-fault"},
			want:  "unknown fault",
		},
		"asset status": {
			event: EventSpec{Action: ActionAssetStatus, Asset: "asset-1", Status: "destroyed"},
			want:  "asset status",
		},
	} {
		base.Events = []EventSpec{test.event}
		err := base.Validate()
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s Validate() error = %v", name, err)
		}
	}
}

func TestValidateRejectsMalformedCoordinatesAndTimelineRanges(t *testing.T) {
	validPosition := Position{Lat: 24.71, Lng: 46.67}
	base := Scenario{
		Name:    "invalid-edge-cases",
		Sources: []SourceSpec{{ID: "source-1"}},
		Tracks:  []TrackSpec{{ID: "track-1", Start: &validPosition}},
		Assets:  []AssetSpec{{ID: "asset-1"}},
	}
	cases := []struct {
		name  string
		event EventSpec
		want  string
	}{
		{
			name:  "invalid waypoint",
			event: EventSpec{Action: ActionMove, Track: "track-1", Waypoints: []Position{{Lat: 91, Lng: 0}}},
			want:  "waypoint",
		},
		{
			name:  "until before after",
			event: EventSpec{Action: ActionObserve, Source: "source-1", Track: "track-1", After: "10s", Until: "5s"},
			want:  "until",
		},
		{
			name:  "confidence outside range",
			event: EventSpec{Action: ActionClassify, Track: "track-1", Label: "unknown", Confidence: ptrFloat64(1.1)},
			want:  "confidence",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			candidate.Events = []EventSpec{tc.event}
			err := candidate.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}

	candidate := base
	candidate.CommandSimulation = &CommandSimSpec{Behavior: "normal", AcknowledgeAfter: "not-a-duration"}
	if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), "acknowledge_after") {
		t.Fatalf("Validate() error = %v, want command simulation duration error", err)
	}

	candidate.CommandSimulation = &CommandSimSpec{Behavior: "normal", AcknowledgeAfter: "5s", CompleteAfter: "1s"}
	if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), "complete_after") {
		t.Fatalf("Validate() error = %v, want command ordering error", err)
	}

	candidate.CommandSimulation = nil
	candidate.Events = []EventSpec{{Action: ActionObserve, Source: "source-1", Track: "track-1", JitterM: math.NaN()}}
	if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), "jitter_m") {
		t.Fatalf("Validate() error = %v, want finite jitter error", err)
	}
}

func TestValidateRejectsAmbiguousAndMalformedActionReferences(t *testing.T) {
	base := Scenario{
		Name:    "action-references",
		Sources: []SourceSpec{{ID: "source-1"}},
		Assets:  []AssetSpec{{ID: "asset-1"}},
		Tracks:  []TrackSpec{{ID: "track-1"}},
	}
	cases := []struct {
		name  string
		event EventSpec
		want  string
	}{
		{
			name:  "move has two targets",
			event: EventSpec{Action: ActionMove, Asset: "asset-1", Track: "track-1", Waypoints: []Position{{Lat: 24.7, Lng: 46.6}}},
			want:  "exactly one",
		},
		{
			name:  "assessment evidence missing id",
			event: EventSpec{Action: ActionAssessment, SubjectType: "track", SubjectID: "track-1", AssessmentType: "identity", Conclusion: "unknown", Evidence: []EvidenceSpec{{Type: "track"}}},
			want:  "requires an id",
		},
		{
			name:  "incident status has no stable reference",
			event: EventSpec{Action: ActionIncident, IncidentStatus: "INVESTIGATING"},
			want:  "requires incident_ref",
		},
		{
			name:  "incident reference is whitespace",
			event: EventSpec{Action: ActionIncident, IncidentRef: "  "},
			want:  "requires incident_ref",
		},
		{
			name:  "command type is whitespace",
			event: EventSpec{Action: ActionIssueCommand, Asset: "asset-1", Type: "  "},
			want:  "requires a type",
		},
		{
			name:  "assessment subject is whitespace",
			event: EventSpec{Action: ActionAssessment, SubjectType: "track", SubjectID: "  ", AssessmentType: "identity", Conclusion: "unknown"},
			want:  "requires subject_type and subject_id",
		},
		{
			name:  "assessment conclusion is whitespace",
			event: EventSpec{Action: ActionAssessment, SubjectType: "track", SubjectID: "track-1", AssessmentType: "identity", Conclusion: "  "},
			want:  "requires assessment_type and conclusion",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			candidate.Events = []EventSpec{tc.event}
			err := candidate.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateRejectsNamespaceCollidingLogicalIDs(t *testing.T) {
	validPolygon := []Position{{Lat: 24.70, Lng: 46.60}, {Lat: 24.71, Lng: 46.60}, {Lat: 24.70, Lng: 46.61}}
	cases := []struct {
		name     string
		scenario Scenario
		want     string
	}{
		{
			name: "surrounding whitespace",
			scenario: Scenario{
				Name:   "whitespace-id",
				Assets: []AssetSpec{{ID: "asset-1"}, {ID: " asset-1 "}},
			},
			want: "surrounding whitespace",
		},
		{
			name: "duplicate geofence",
			scenario: Scenario{
				Name:      "duplicate-geofence",
				Geofences: []GeofenceSpec{{ID: "restricted", Polygon: validPolygon}, {ID: "restricted", Polygon: validPolygon}},
			},
			want: "duplicate geofence",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.scenario.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func ptrFloat64(value float64) *float64 { return &value }
