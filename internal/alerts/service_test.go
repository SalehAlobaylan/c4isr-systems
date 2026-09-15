package alerts

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

type fakeRepository struct {
	alerts        map[string]Alert
	createdCalls  []Alert
	createErr     error
	unresolved    *Alert
	unresolvedErr error
	listResult    []Alert
	listTotal     int
	listErr       error
	listFilter    ListFilter
	listLimit     int
	listOffset    int
	incidentSet   [2]string
	incidentErr   error
}

func (f *fakeRepository) Create(_ context.Context, alert Alert) (Alert, error) {
	if f.createErr != nil {
		return Alert{}, f.createErr
	}
	if f.alerts == nil {
		f.alerts = map[string]Alert{}
	}
	f.alerts[alert.ID] = alert
	f.createdCalls = append(f.createdCalls, alert)
	return alert, nil
}

func (f *fakeRepository) Get(_ context.Context, id string) (Alert, error) {
	alert, ok := f.alerts[id]
	if !ok {
		return Alert{}, nil
	}
	return alert, nil
}

func (f *fakeRepository) List(_ context.Context, filter ListFilter, limit, offset int) ([]Alert, int, error) {
	f.listFilter = filter
	f.listLimit = limit
	f.listOffset = offset
	return f.listResult, f.listTotal, f.listErr
}

func (f *fakeRepository) Acknowledge(_ context.Context, id, operator string) (Alert, error) {
	alert, ok := f.alerts[id]
	if !ok {
		return Alert{}, nil
	}
	if alert.State != StateActive {
		return Alert{}, apperr.Conflict("alert is not active")
	}
	now := time.Now().UTC()
	alert.State = StateAcknowledged
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = operator
	f.alerts[id] = alert
	return alert, nil
}

func (f *fakeRepository) Resolve(_ context.Context, id, operator string) (Alert, error) {
	alert, ok := f.alerts[id]
	if !ok {
		return Alert{}, nil
	}
	if alert.State == StateResolved {
		return Alert{}, apperr.Conflict("alert is already resolved")
	}
	now := time.Now().UTC()
	alert.State = StateResolved
	alert.ResolvedAt = &now
	alert.ResolvedBy = operator
	f.alerts[id] = alert
	return alert, nil
}

func (f *fakeRepository) SetIncident(_ context.Context, alertID, incidentID string) error {
	f.incidentSet = [2]string{alertID, incidentID}
	return f.incidentErr
}

func (f *fakeRepository) FindUnresolvedForGeofenceTrack(context.Context, string, string) (*Alert, error) {
	return f.unresolved, f.unresolvedErr
}

type alertRecorder struct {
	created      []events.AlertCreated
	acknowledged []events.AlertAcknowledged
	resolved     []events.AlertResolved
}

func newTestService(repo Repository) (*Service, *alertRecorder) {
	bus := events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := &alertRecorder{}
	bus.Subscribe(events.TopicAlertCreated, func(_ context.Context, ev events.Event) {
		recorder.created = append(recorder.created, ev.(events.AlertCreated))
	})
	bus.Subscribe(events.TopicAlertAcknowledged, func(_ context.Context, ev events.Event) {
		recorder.acknowledged = append(recorder.acknowledged, ev.(events.AlertAcknowledged))
	})
	bus.Subscribe(events.TopicAlertResolved, func(_ context.Context, ev events.Event) {
		recorder.resolved = append(recorder.resolved, ev.(events.AlertResolved))
	})
	return NewService(repo, bus), recorder
}

func TestHandleGeofenceBreachedCreatesAlert(t *testing.T) {
	fixedNow := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	svc, recorder := newTestService(repo)
	svc.now = func() time.Time { return fixedNow }

	svc.HandleGeofenceBreached(context.Background(), events.GeofenceBreached{
		At:           fixedNow,
		GeofenceID:   "geo_1",
		GeofenceName: "No-Fly Zone",
		GeofenceType: "exclusion",
		Severity:     "critical",
		TrackID:      "trk_1",
		Position:     geo.Point{Lat: 10, Lng: 20},
	})

	if len(repo.createdCalls) != 1 {
		t.Fatalf("expected 1 created alert, got %d", len(repo.createdCalls))
	}
	alert := repo.createdCalls[0]
	if !strings.HasPrefix(alert.ID, "alr_") {
		t.Fatalf("expected alr id, got %q", alert.ID)
	}
	if alert.Type != "geofence.breach" || alert.State != StateActive {
		t.Fatalf("unexpected alert identity: %+v", alert)
	}
	if alert.Severity != SeverityCritical {
		t.Fatalf("expected critical severity, got %q", alert.Severity)
	}
	if alert.TrackID != "trk_1" || alert.GeofenceID != "geo_1" {
		t.Fatalf("unexpected alert references: %+v", alert)
	}
	if alert.Title != "Track trk_1 entered No-Fly Zone" {
		t.Fatalf("unexpected title: %q", alert.Title)
	}
	if alert.Message != "Track trk_1 entered exclusion geofence No-Fly Zone" {
		t.Fatalf("unexpected message: %q", alert.Message)
	}
	if alert.SourceReference["rule"] != "geofence.breach" || alert.SourceReference["geofenceId"] != "geo_1" ||
		alert.SourceReference["geofenceName"] != "No-Fly Zone" || alert.SourceReference["geofenceType"] != "exclusion" ||
		alert.SourceReference["trackId"] != "trk_1" {
		t.Fatalf("unexpected source reference: %+v", alert.SourceReference)
	}

	if len(recorder.created) != 1 {
		t.Fatalf("expected 1 created event, got %d", len(recorder.created))
	}
	created := recorder.created[0]
	if created.AlertID != alert.ID || created.Severity != "critical" || created.TrackID != "trk_1" || created.GeofenceID != "geo_1" {
		t.Fatalf("unexpected created event: %+v", created)
	}
	if !created.At.Equal(fixedNow) {
		t.Fatalf("expected event time %v, got %v", fixedNow, created.At)
	}
}

func TestHandleGeofenceBreachedSuppressesDuplicate(t *testing.T) {
	repo := &fakeRepository{unresolved: &Alert{ID: "alr_existing", State: StateAcknowledged}}
	svc, recorder := newTestService(repo)

	svc.HandleGeofenceBreached(context.Background(), events.GeofenceBreached{
		GeofenceID: "geo_1",
		TrackID:    "trk_1",
		Severity:   "high",
	})

	if len(repo.createdCalls) != 0 {
		t.Fatalf("expected duplicate suppression, got %d created alerts", len(repo.createdCalls))
	}
	if len(recorder.created) != 0 {
		t.Fatalf("expected no events, got %d", len(recorder.created))
	}
}

func TestHandleGeofenceBreachedDefaultsUnknownSeverity(t *testing.T) {
	repo := &fakeRepository{}
	svc, _ := newTestService(repo)

	svc.HandleGeofenceBreached(context.Background(), events.GeofenceBreached{
		GeofenceID: "geo_1",
		TrackID:    "trk_1",
		Severity:   "bogus",
	})

	if len(repo.createdCalls) != 1 || repo.createdCalls[0].Severity != SeverityMedium {
		t.Fatalf("expected medium severity fallback, got %+v", repo.createdCalls)
	}
}

func TestAlertLifecycle(t *testing.T) {
	repo := &fakeRepository{alerts: map[string]Alert{
		"alr_1": {ID: "alr_1", State: StateActive},
	}}
	svc, recorder := newTestService(repo)

	acknowledged, err := svc.Acknowledge(context.Background(), "alr_1", "operator-07")
	if err != nil {
		t.Fatalf("unexpected acknowledge error: %v", err)
	}
	if acknowledged.State != StateAcknowledged || acknowledged.AcknowledgedBy != "operator-07" {
		t.Fatalf("unexpected acknowledged alert: %+v", acknowledged)
	}
	if len(recorder.acknowledged) != 1 || recorder.acknowledged[0].Operator != "operator-07" {
		t.Fatalf("expected acknowledgement event, got %+v", recorder.acknowledged)
	}

	resolved, err := svc.Resolve(context.Background(), "alr_1", "operator-07")
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if resolved.State != StateResolved || resolved.ResolvedBy != "operator-07" {
		t.Fatalf("unexpected resolved alert: %+v", resolved)
	}
	if len(recorder.resolved) != 1 || recorder.resolved[0].Operator != "operator-07" {
		t.Fatalf("expected resolve event, got %+v", recorder.resolved)
	}

	if _, err := svc.Resolve(context.Background(), "alr_1", "operator-07"); err == nil {
		t.Fatal("expected conflict resolving a resolved alert")
	}
	if len(recorder.resolved) != 1 {
		t.Fatalf("expected no event for the rejected transition, got %d", len(recorder.resolved))
	}

	if _, err := svc.Acknowledge(context.Background(), "alr_1", "operator-07"); err == nil {
		t.Fatal("expected conflict acknowledging a resolved alert")
	}
	if len(recorder.acknowledged) != 1 {
		t.Fatalf("expected no event for the rejected transition, got %d", len(recorder.acknowledged))
	}
}

func TestListClampsLimitAndOffset(t *testing.T) {
	repo := &fakeRepository{listResult: []Alert{{ID: "alr_1"}}, listTotal: 1}
	svc, _ := newTestService(repo)
	filter := ListFilter{State: "ACTIVE", Severity: "high", TrackID: "trk_1", IncidentID: "inc_1"}

	items, total, err := svc.List(context.Background(), filter, 5000, -3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || total != 1 {
		t.Fatalf("unexpected result: items=%d total=%d", len(items), total)
	}
	if repo.listLimit != MaxLimit || repo.listOffset != 0 {
		t.Fatalf("expected limit=%d offset=0, got limit=%d offset=%d", MaxLimit, repo.listLimit, repo.listOffset)
	}
	if repo.listFilter != filter {
		t.Fatalf("expected filter %+v, got %+v", filter, repo.listFilter)
	}

	if _, _, err := svc.List(context.Background(), ListFilter{}, 0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listLimit != DefaultLimit {
		t.Fatalf("expected default limit %d, got %d", DefaultLimit, repo.listLimit)
	}
}

func TestSetIncidentDelegates(t *testing.T) {
	repo := &fakeRepository{}
	svc, _ := newTestService(repo)

	if err := svc.SetIncident(context.Background(), "alr_1", "inc_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.incidentSet != [2]string{"alr_1", "inc_1"} {
		t.Fatalf("unexpected incident link: %+v", repo.incidentSet)
	}
}
