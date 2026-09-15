package audit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

type fakeRepository struct {
	appended []Entry
}

func (f *fakeRepository) Append(_ context.Context, entry Entry) (Entry, error) {
	f.appended = append(f.appended, entry)
	return entry, nil
}

func (f *fakeRepository) List(context.Context, ListFilter, int, int) ([]Entry, int, error) {
	return []Entry{}, 0, nil
}

func testService(repo Repository) (*Service, *events.Dispatcher) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := events.NewDispatcher(logger)
	return NewService(repo, bus), bus
}

func TestHandleMapsEventToEntry(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	position := geo.Point{Lat: 12.5, Lng: 45.25}

	tests := []struct {
		name        string
		event       events.Event
		actorType   ActorType
		actorID     string
		action      string
		subjectType string
		subjectID   string
	}{
		{
			name:        "source created is system",
			event:       events.SourceCreated{At: now, SourceID: "src_1", Name: "radar", Type: "sensor"},
			actorType:   ActorSystem,
			action:      events.TopicSourceCreated,
			subjectType: "source",
			subjectID:   "src_1",
		},
		{
			name:        "source updated is system",
			event:       events.SourceUpdated{At: now, SourceID: "src_1", Status: "degraded"},
			actorType:   ActorSystem,
			action:      events.TopicSourceUpdated,
			subjectType: "source",
			subjectID:   "src_1",
		},
		{
			name:        "observation received is system",
			event:       events.ObservationReceived{At: now, ObservationID: "obs_1", SourceID: "src_1", Position: &position},
			actorType:   ActorSystem,
			action:      events.TopicObservationReceived,
			subjectType: "observation",
			subjectID:   "obs_1",
		},
		{
			name:        "asset position updated is system",
			event:       events.AssetPositionUpdated{At: now, AssetID: "ast_1", Position: position},
			actorType:   ActorSystem,
			action:      events.TopicAssetPositionUpdated,
			subjectType: "asset",
			subjectID:   "ast_1",
		},
		{
			name:        "telemetry received is system",
			event:       events.TelemetryReceived{At: now, TelemetryID: "tel_1", AssetID: "ast_1", SourceID: "src_1"},
			actorType:   ActorSystem,
			action:      events.TopicTelemetryReceived,
			subjectType: "telemetry",
			subjectID:   "tel_1",
		},
		{
			name:        "track updated is system",
			event:       events.TrackUpdated{At: now, TrackID: "trk_1", Position: &position},
			actorType:   ActorSystem,
			action:      events.TopicTrackUpdated,
			subjectType: "track",
			subjectID:   "trk_1",
		},
		{
			name:        "classification created is system",
			event:       events.ClassificationCreated{At: now, ClassificationID: "cls_1", TrackID: "trk_1", Label: "vessel"},
			actorType:   ActorSystem,
			action:      events.TopicClassificationCreated,
			subjectType: "classification",
			subjectID:   "cls_1",
		},
		{
			name:        "geofence breached is system",
			event:       events.GeofenceBreached{At: now, GeofenceID: "gef_1", TrackID: "trk_1", Severity: "high", Position: position},
			actorType:   ActorSystem,
			action:      events.TopicGeofenceBreached,
			subjectType: "geofence",
			subjectID:   "gef_1",
		},
		{
			name:        "alert created is system",
			event:       events.AlertCreated{At: now, AlertID: "alr_1", Type: "geofence", Severity: "high", Title: "breach", TrackID: "trk_1"},
			actorType:   ActorSystem,
			action:      events.TopicAlertCreated,
			subjectType: "alert",
			subjectID:   "alr_1",
		},
		{
			name:        "alert acknowledged is operator",
			event:       events.AlertAcknowledged{At: now, AlertID: "alr_1", Operator: "opr_1"},
			actorType:   ActorOperator,
			actorID:     "opr_1",
			action:      events.TopicAlertAcknowledged,
			subjectType: "alert",
			subjectID:   "alr_1",
		},
		{
			name:        "alert resolved is operator",
			event:       events.AlertResolved{At: now, AlertID: "alr_1", Operator: "opr_2"},
			actorType:   ActorOperator,
			actorID:     "opr_2",
			action:      events.TopicAlertResolved,
			subjectType: "alert",
			subjectID:   "alr_1",
		},
		{
			name:        "incident created is operator",
			event:       events.IncidentCreated{At: now, IncidentID: "inc_1", Title: "border", Priority: "high", Status: "open", Actor: "opr_3"},
			actorType:   ActorOperator,
			actorID:     "opr_3",
			action:      events.TopicIncidentCreated,
			subjectType: "incident",
			subjectID:   "inc_1",
		},
		{
			name:        "incident updated without actor is system",
			event:       events.IncidentUpdated{At: now, IncidentID: "inc_1", Status: "closed"},
			actorType:   ActorSystem,
			action:      events.TopicIncidentUpdated,
			subjectType: "incident",
			subjectID:   "inc_1",
		},
		{
			name:        "mission created is operator",
			event:       events.MissionCreated{At: now, MissionID: "msn_1", Name: "patrol", Status: "planned", Actor: "opr_4"},
			actorType:   ActorOperator,
			actorID:     "opr_4",
			action:      events.TopicMissionCreated,
			subjectType: "mission",
			subjectID:   "msn_1",
		},
		{
			name:        "mission updated is operator",
			event:       events.MissionUpdated{At: now, MissionID: "msn_1", Status: "active", Actor: "opr_4"},
			actorType:   ActorOperator,
			actorID:     "opr_4",
			action:      events.TopicMissionUpdated,
			subjectType: "mission",
			subjectID:   "msn_1",
		},
		{
			name:        "command issued is operator",
			event:       events.CommandIssued{At: now, CommandID: "cmd_1", AssetID: "ast_1", Type: "MOVE", Actor: "opr_5"},
			actorType:   ActorOperator,
			actorID:     "opr_5",
			action:      events.TopicCommandIssued,
			subjectType: "command",
			subjectID:   "cmd_1",
		},
		{
			name:        "command status changed without actor is system",
			event:       events.CommandStatusChanged{At: now, CommandID: "cmd_1", AssetID: "ast_1", State: "ACKNOWLEDGED"},
			actorType:   ActorSystem,
			action:      events.TopicCommandStatusChanged,
			subjectType: "command",
			subjectID:   "cmd_1",
		},
		{
			name:        "assessment created is system",
			event:       events.AssessmentCreated{At: now, AssessmentID: "asm_1", SubjectType: "track", SubjectID: "trk_1", Type: "threat", Method: "AI"},
			actorType:   ActorSystem,
			action:      events.TopicAssessmentCreated,
			subjectType: "assessment",
			subjectID:   "asm_1",
		},
		{
			name:        "scenario run started is scenario",
			event:       events.ScenarioRunStarted{At: now, RunID: "run_1", ScenarioName: "demo", Seed: 42},
			actorType:   ActorScenario,
			actorID:     "run_1",
			action:      events.TopicScenarioRunStarted,
			subjectType: "scenario_run",
			subjectID:   "run_1",
		},
		{
			name:        "scenario run ended is scenario",
			event:       events.ScenarioRunEnded{At: now, RunID: "run_1", Status: "completed"},
			actorType:   ActorScenario,
			actorID:     "run_1",
			action:      events.TopicScenarioRunEnded,
			subjectType: "scenario_run",
			subjectID:   "run_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			_, bus := testService(repo)

			bus.Publish(context.Background(), tt.event)

			if len(repo.appended) != 1 {
				t.Fatalf("appended entries = %d, want 1", len(repo.appended))
			}
			entry := repo.appended[0]
			if entry.ID == "" || !strings.HasPrefix(entry.ID, "aud_") {
				t.Errorf("entry id = %q, want aud_ prefix", entry.ID)
			}
			if entry.OccurredAt.IsZero() {
				t.Error("entry occurredAt is zero")
			}
			if entry.ActorType != tt.actorType {
				t.Errorf("actor type = %q, want %q", entry.ActorType, tt.actorType)
			}
			if entry.ActorID != tt.actorID {
				t.Errorf("actor id = %q, want %q", entry.ActorID, tt.actorID)
			}
			if entry.Action != tt.action {
				t.Errorf("action = %q, want %q", entry.Action, tt.action)
			}
			if entry.SubjectType != tt.subjectType {
				t.Errorf("subject type = %q, want %q", entry.SubjectType, tt.subjectType)
			}
			if entry.SubjectID != tt.subjectID {
				t.Errorf("subject id = %q, want %q", entry.SubjectID, tt.subjectID)
			}
		})
	}
}

func TestHandleRecordsBreachPosition(t *testing.T) {
	repo := &fakeRepository{}
	_, bus := testService(repo)
	position := geo.Point{Lat: -33.5, Lng: 151.25}

	bus.Publish(context.Background(), events.GeofenceBreached{
		At:         time.Now().UTC(),
		GeofenceID: "gef_1",
		TrackID:    "trk_9",
		Severity:   "critical",
		Position:   position,
	})

	if len(repo.appended) != 1 {
		t.Fatalf("appended entries = %d, want 1", len(repo.appended))
	}
	entry := repo.appended[0]
	if entry.Data["trackId"] != "trk_9" {
		t.Errorf("data trackId = %v, want trk_9", entry.Data["trackId"])
	}
	got, ok := entry.Data["position"].(map[string]any)
	if !ok {
		t.Fatalf("data position = %#v, want map", entry.Data["position"])
	}
	if got["lat"] != position.Lat || got["lng"] != position.Lng {
		t.Errorf("data position = %#v, want %#v", got, position)
	}
}

func TestHandleSwallowsRepositoryErrors(t *testing.T) {
	repo := &failingRepository{}
	_, bus := testService(repo)

	bus.Publish(context.Background(), events.TrackClosed{At: time.Now().UTC(), TrackID: "trk_1"})

	if repo.calls != 1 {
		t.Fatalf("repo calls = %d, want 1", repo.calls)
	}
}

func TestAppendFillsIdentity(t *testing.T) {
	repo := &fakeRepository{}
	svc, _ := testService(repo)

	entry, err := svc.Append(context.Background(), Entry{
		ActorType:   ActorOperator,
		ActorID:     "opr_9",
		Action:      events.TopicAlertAcknowledged,
		SubjectType: "alert",
		SubjectID:   "alr_9",
	})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if !strings.HasPrefix(entry.ID, "aud_") {
		t.Fatalf("Append() id = %q, want aud_ prefix", entry.ID)
	}
	if entry.OccurredAt.IsZero() {
		t.Fatal("Append() occurredAt is zero")
	}
	if entry.Data == nil {
		t.Fatal("Append() data is nil, want empty map")
	}
}

type failingRepository struct {
	calls int
}

func (f *failingRepository) Append(_ context.Context, entry Entry) (Entry, error) {
	f.calls++
	return Entry{}, errors.New("test error")
}

func (f *failingRepository) List(context.Context, ListFilter, int, int) ([]Entry, int, error) {
	return nil, 0, errors.New("test error")
}
