package incidents

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type fakeRepo struct {
	incidents map[string]Incident
	relations []string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{incidents: map[string]Incident{}}
}

func (f *fakeRepo) Create(_ context.Context, incident Incident) (Incident, error) {
	now := time.Now().UTC()
	incident.CreatedAt = now
	incident.UpdatedAt = now
	f.incidents[incident.ID] = incident
	return incident, nil
}

func (f *fakeRepo) CreateWithRelations(ctx context.Context, incident Incident, relations RelationIDs) (Incident, error) {
	created, err := f.Create(ctx, incident)
	if err != nil {
		return Incident{}, err
	}
	for _, id := range relations.AlertIDs {
		if err := f.AttachAlert(ctx, created.ID, id); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range relations.TrackIDs {
		if err := f.AttachTrack(ctx, created.ID, id); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range relations.AssetIDs {
		if err := f.AttachAsset(ctx, created.ID, id); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range relations.ObservationIDs {
		if err := f.AttachObservation(ctx, created.ID, id); err != nil {
			return Incident{}, err
		}
	}
	for _, id := range relations.AssessmentIDs {
		if err := f.AttachAssessment(ctx, created.ID, id); err != nil {
			return Incident{}, err
		}
	}
	return created, nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (Incident, error) {
	incident, ok := f.incidents[id]
	if !ok {
		return Incident{}, apperr.NotFound("incident", id)
	}
	return incident, nil
}

func (f *fakeRepo) GetDetail(ctx context.Context, id string) (Detail, error) {
	incident, err := f.Get(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Incident: incident}, nil
}

func (f *fakeRepo) List(_ context.Context, status string, limit, offset int) ([]Incident, int, error) {
	out := make([]Incident, 0, len(f.incidents))
	for _, incident := range f.incidents {
		if status != "" && string(incident.Status) != status {
			continue
		}
		out = append(out, incident)
	}
	return out, len(out), nil
}

func (f *fakeRepo) UpdateStatus(ctx context.Context, id string, status, expected Status) (Incident, error) {
	incident, err := f.Get(ctx, id)
	if err != nil {
		return Incident{}, err
	}
	if incident.Status != expected {
		return Incident{}, apperr.Conflict("incident status changed concurrently")
	}
	incident.Status = status
	f.incidents[id] = incident
	return incident, nil
}

func (f *fakeRepo) Update(_ context.Context, incident Incident) (Incident, error) {
	f.incidents[incident.ID] = incident
	return incident, nil
}

func (f *fakeRepo) AttachAlert(_ context.Context, incidentID, alertID string) error {
	f.relations = append(f.relations, "alert:"+incidentID+":"+alertID)
	return nil
}

func (f *fakeRepo) AttachTrack(_ context.Context, incidentID, trackID string) error {
	f.relations = append(f.relations, "track:"+incidentID+":"+trackID)
	return nil
}

func (f *fakeRepo) AttachAsset(_ context.Context, incidentID, assetID string) error {
	f.relations = append(f.relations, "asset:"+incidentID+":"+assetID)
	return nil
}

func (f *fakeRepo) AttachObservation(_ context.Context, incidentID, observationID string) error {
	f.relations = append(f.relations, "observation:"+incidentID+":"+observationID)
	return nil
}

func (f *fakeRepo) AttachAssessment(_ context.Context, incidentID, assessmentID string) error {
	f.relations = append(f.relations, "assessment:"+incidentID+":"+assessmentID)
	return nil
}

func testBus() *events.Dispatcher {
	return events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateValidatesRelationIDs(t *testing.T) {
	svc := NewService(newFakeRepo(), testBus())
	_, err := svc.Create(context.Background(), CreateInput{Title: "Fire", AlertIDs: []string{" "}})
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestUpdateStatusRejectsInvalidTransition(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, testBus())
	created, err := svc.Create(context.Background(), CreateInput{Title: "Fire"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateStatus(context.Background(), created.ID, StatusResolved, "op_1"); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("err = %v, want conflict error", err)
	}
}

func TestUpdateStatusPublishesIncidentUpdated(t *testing.T) {
	bus := testBus()
	var updates []events.IncidentUpdated
	bus.Subscribe(events.TopicIncidentUpdated, func(_ context.Context, ev events.Event) {
		updates = append(updates, ev.(events.IncidentUpdated))
	})
	svc := NewService(newFakeRepo(), bus)
	created, err := svc.Create(context.Background(), CreateInput{Title: "Fire"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateStatus(context.Background(), created.ID, StatusInvestigating, "op_7")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusInvestigating {
		t.Fatalf("status = %s, want %s", updated.Status, StatusInvestigating)
	}
	if len(updates) != 1 {
		t.Fatalf("got %d events, want 1", len(updates))
	}
	if updates[0].IncidentID != created.ID || updates[0].Status != string(StatusInvestigating) || updates[0].Actor != "op_7" {
		t.Fatalf("unexpected event: %+v", updates[0])
	}
}

func TestCreateAttachesRelationsAndPublishes(t *testing.T) {
	bus := testBus()
	var created []events.IncidentCreated
	bus.Subscribe(events.TopicIncidentCreated, func(_ context.Context, ev events.Event) {
		created = append(created, ev.(events.IncidentCreated))
	})
	repo := newFakeRepo()
	svc := NewService(repo, bus)
	incident, err := svc.Create(context.Background(), CreateInput{
		Title:    "Fire",
		Priority: PriorityHigh,
		AlertIDs: []string{"alt_1"},
		TrackIDs: []string{"trk_1"},
		AssetIDs: []string{"ast_1"},
		Actor:    "op_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if incident.Status != StatusOpen || incident.AssignedOperator != "op_1" {
		t.Fatalf("unexpected incident: %+v", incident)
	}
	if len(repo.relations) != 3 {
		t.Fatalf("got %d relations, want 3", len(repo.relations))
	}
	if len(created) != 1 || created[0].IncidentID != incident.ID || created[0].Actor != "op_1" {
		t.Fatalf("unexpected events: %+v", created)
	}
}
