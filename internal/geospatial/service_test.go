package geospatial

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/runctx"
)

type setStateCall struct {
	geofenceID string
	trackID    string
	inside     bool
}

type fakeRepository struct {
	containing      []Geofence
	containingErr   error
	containingCalls int
	states          map[string]bool
	statesErr       error
	setStateCalls   []setStateCall
	setStateErr     error
	createErr       error
	getResult       Geofence
	getErr          error
}

func (f *fakeRepository) Create(_ context.Context, geofence Geofence) (Geofence, error) {
	if f.createErr != nil {
		return Geofence{}, f.createErr
	}
	return geofence, nil
}

func (f *fakeRepository) Get(context.Context, string) (Geofence, error) {
	if f.getErr != nil {
		return Geofence{}, f.getErr
	}
	return f.getResult, nil
}

func (f *fakeRepository) List(context.Context, int, int) ([]Geofence, int, error) {
	return nil, 0, nil
}

func (f *fakeRepository) SetActive(context.Context, string, bool) (Geofence, error) {
	return Geofence{}, nil
}

func (f *fakeRepository) ContainingPoint(context.Context, geo.Point) ([]Geofence, error) {
	f.containingCalls++
	return f.containing, f.containingErr
}

func (f *fakeRepository) StateFor(context.Context, string, string) (*bool, error) {
	return nil, nil
}

func (f *fakeRepository) SetState(_ context.Context, geofenceID, trackID string, inside bool) error {
	f.setStateCalls = append(f.setStateCalls, setStateCall{geofenceID: geofenceID, trackID: trackID, inside: inside})
	return f.setStateErr
}

func (f *fakeRepository) StatesForTrack(context.Context, string) (map[string]bool, error) {
	return f.states, f.statesErr
}

func (f *fakeRepository) AssetsWithinRadius(context.Context, geo.Point, float64, int) ([]AssetDistance, error) {
	return nil, nil
}

func (f *fakeRepository) NearestAssets(context.Context, geo.Point, int) ([]AssetDistance, error) {
	return nil, nil
}

type eventRecorder struct {
	breaches []events.GeofenceBreached
	exits    []events.GeofenceExited
	created  []events.GeofenceCreated
}

func newTestService(repo Repository) (*Service, *eventRecorder) {
	bus := events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := &eventRecorder{}
	bus.Subscribe(events.TopicGeofenceBreached, func(_ context.Context, ev events.Event) {
		recorder.breaches = append(recorder.breaches, ev.(events.GeofenceBreached))
	})
	bus.Subscribe(events.TopicGeofenceExited, func(_ context.Context, ev events.Event) {
		recorder.exits = append(recorder.exits, ev.(events.GeofenceExited))
	})
	bus.Subscribe(events.TopicGeofenceCreated, func(_ context.Context, ev events.Event) {
		recorder.created = append(recorder.created, ev.(events.GeofenceCreated))
	})
	return NewService(repo, bus), recorder
}

func TestHandleTrackUpdatedPublishesBreachOnEntry(t *testing.T) {
	geofence := Geofence{ID: "geo_1", Name: "Restricted Zone", Type: TypeRestricted, Severity: SeverityHigh}
	repo := &fakeRepository{containing: []Geofence{geofence}, states: map[string]bool{}}
	svc, recorder := newTestService(repo)
	position := geo.Point{Lat: 10, Lng: 20}

	svc.HandleTrackUpdated(context.Background(), events.TrackUpdated{TrackID: "trk_1", Position: &position})

	if len(recorder.breaches) != 1 {
		t.Fatalf("expected 1 breach event, got %d", len(recorder.breaches))
	}
	breach := recorder.breaches[0]
	if breach.GeofenceID != "geo_1" || breach.TrackID != "trk_1" {
		t.Fatalf("unexpected breach identity: %+v", breach)
	}
	if breach.GeofenceType != "restricted" || breach.Severity != "high" {
		t.Fatalf("unexpected breach classification: %+v", breach)
	}
	if breach.Position != position {
		t.Fatalf("expected position %+v, got %+v", position, breach.Position)
	}
	if len(recorder.exits) != 0 {
		t.Fatalf("expected no exit events, got %d", len(recorder.exits))
	}
	want := []setStateCall{{geofenceID: "geo_1", trackID: "trk_1", inside: true}}
	if len(repo.setStateCalls) != 1 || repo.setStateCalls[0] != want[0] {
		t.Fatalf("unexpected state calls: %+v", repo.setStateCalls)
	}
}

func TestHandleTrackUpdatedRefreshesStateWhenAlreadyInside(t *testing.T) {
	geofence := Geofence{ID: "geo_1", Name: "Restricted Zone", Type: TypeRestricted, Severity: SeverityHigh}
	repo := &fakeRepository{containing: []Geofence{geofence}, states: map[string]bool{"geo_1": true}}
	svc, recorder := newTestService(repo)
	position := geo.Point{Lat: 10, Lng: 20}

	svc.HandleTrackUpdated(context.Background(), events.TrackUpdated{TrackID: "trk_1", Position: &position})

	if len(recorder.breaches) != 0 {
		t.Fatalf("expected no breach events, got %d", len(recorder.breaches))
	}
	want := []setStateCall{{geofenceID: "geo_1", trackID: "trk_1", inside: true}}
	if len(repo.setStateCalls) != 1 || repo.setStateCalls[0] != want[0] {
		t.Fatalf("expected a refresh state call, got %+v", repo.setStateCalls)
	}
}

func TestHandleTrackUpdatedPublishesExitWhenLeaving(t *testing.T) {
	repo := &fakeRepository{states: map[string]bool{"geo_1": true}}
	svc, recorder := newTestService(repo)
	position := geo.Point{Lat: 10, Lng: 20}

	svc.HandleTrackUpdated(context.Background(), events.TrackUpdated{TrackID: "trk_1", Position: &position})

	if len(recorder.exits) != 1 {
		t.Fatalf("expected 1 exit event, got %d", len(recorder.exits))
	}
	exit := recorder.exits[0]
	if exit.GeofenceID != "geo_1" || exit.TrackID != "trk_1" || exit.Position != position {
		t.Fatalf("unexpected exit event: %+v", exit)
	}
	want := []setStateCall{{geofenceID: "geo_1", trackID: "trk_1", inside: false}}
	if len(repo.setStateCalls) != 1 || repo.setStateCalls[0] != want[0] {
		t.Fatalf("unexpected state calls: %+v", repo.setStateCalls)
	}
}

func TestHandleTrackUpdatedIgnoresTrackWithoutPosition(t *testing.T) {
	repo := &fakeRepository{containing: []Geofence{{ID: "geo_1"}}}
	svc, recorder := newTestService(repo)

	svc.HandleTrackUpdated(context.Background(), events.TrackUpdated{TrackID: "trk_1"})

	if repo.containingCalls != 0 || len(repo.setStateCalls) != 0 {
		t.Fatal("expected no repository calls without a position")
	}
	if len(recorder.breaches) != 0 || len(recorder.exits) != 0 {
		t.Fatal("expected no events without a position")
	}
}

func TestHandleTrackUpdatedIgnoresPersistedOutsideState(t *testing.T) {
	repo := &fakeRepository{states: map[string]bool{"geo_1": false}}
	svc, recorder := newTestService(repo)
	position := geo.Point{Lat: 10, Lng: 20}

	svc.HandleTrackUpdated(context.Background(), events.TrackUpdated{TrackID: "trk_1", Position: &position})

	if len(repo.setStateCalls) != 0 {
		t.Fatalf("expected no state calls, got %+v", repo.setStateCalls)
	}
	if len(recorder.breaches) != 0 || len(recorder.exits) != 0 {
		t.Fatal("expected no events for an outside state")
	}
}

func TestHandleTrackUpdatedScopesScenarioGeofences(t *testing.T) {
	scope := runctx.Scope{RunID: "run-1", ResourceNamespace: "run-1__"}
	owned := Geofence{ID: "run-1__geofence__zone", Name: "Scenario Zone"}
	repo := &fakeRepository{
		containing: []Geofence{owned, {ID: "legacy-zone", Name: "Legacy Zone"}},
		states: map[string]bool{
			owned.ID:      false,
			"legacy-zone": false,
		},
	}
	svc, recorder := newTestService(repo)
	position := geo.Point{Lat: 10, Lng: 20}

	svc.HandleTrackUpdated(runctx.WithScope(context.Background(), scope), events.TrackUpdated{
		TrackID:  "trk_1",
		Position: &position,
	})

	if len(recorder.breaches) != 1 || recorder.breaches[0].GeofenceID != owned.ID {
		t.Fatalf("scenario track evaluated unrelated geofences: %+v", recorder.breaches)
	}
	if len(repo.setStateCalls) != 1 || repo.setStateCalls[0].geofenceID != owned.ID {
		t.Fatalf("scenario track persisted unrelated geofence state: %+v", repo.setStateCalls)
	}
}

func TestHandleTrackUpdatedScopesScenarioExitState(t *testing.T) {
	scope := runctx.Scope{RunID: "run-1", ResourceNamespace: "run-1__"}
	ownedID := "run-1__geofence__zone"
	repo := &fakeRepository{states: map[string]bool{
		ownedID:       true,
		"legacy-zone": true,
	}}
	svc, recorder := newTestService(repo)
	position := geo.Point{Lat: 10, Lng: 20}

	svc.HandleTrackUpdated(runctx.WithScope(context.Background(), scope), events.TrackUpdated{
		TrackID:  "trk_1",
		Position: &position,
	})

	if len(recorder.exits) != 1 || recorder.exits[0].GeofenceID != ownedID {
		t.Fatalf("scenario track emitted unrelated exits: %+v", recorder.exits)
	}
	if len(repo.setStateCalls) != 1 || repo.setStateCalls[0].geofenceID != ownedID {
		t.Fatalf("scenario track updated unrelated exit state: %+v", repo.setStateCalls)
	}
}

func TestCreatePublishesGeofenceCreated(t *testing.T) {
	repo := &fakeRepository{}
	svc, recorder := newTestService(repo)

	created, err := svc.Create(context.Background(), CreateInput{
		Name:     "Patrol Area",
		Type:     TypePatrol,
		Severity: SeverityLow,
		Polygon:  validPolygon(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == "" || created.GeoJSON == "" {
		t.Fatalf("expected persisted geofence with id and geojson, got %+v", created)
	}
	if len(recorder.created) != 1 {
		t.Fatalf("expected 1 created event, got %d", len(recorder.created))
	}
	if recorder.created[0].GeofenceID != created.ID || recorder.created[0].Name != "Patrol Area" || recorder.created[0].Type != "patrol" {
		t.Fatalf("unexpected created event: %+v", recorder.created[0])
	}
}

func TestEnsureReturnsExistingGeofence(t *testing.T) {
	existing := Geofence{ID: "geo_1", Name: "Existing Zone"}
	repo := &fakeRepository{getResult: existing}
	svc, recorder := newTestService(repo)

	geofence, created, err := svc.Ensure(context.Background(), CreateInput{
		ID:      "geo_1",
		Name:    "Zone",
		Type:    TypeRestricted,
		Polygon: validPolygon(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Fatal("expected Ensure to reuse the existing geofence")
	}
	if geofence.ID != "geo_1" {
		t.Fatalf("expected existing geofence, got %+v", geofence)
	}
	if len(recorder.created) != 0 {
		t.Fatalf("expected no created events, got %d", len(recorder.created))
	}
}

func TestEnsureCreatesMissingGeofence(t *testing.T) {
	repo := &fakeRepository{getErr: apperr.NotFound("geofence", "geo_1")}
	svc, recorder := newTestService(repo)

	geofence, created, err := svc.Ensure(context.Background(), CreateInput{
		ID:      "geo_1",
		Name:    "Zone",
		Type:    TypeRestricted,
		Polygon: validPolygon(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created || geofence.ID != "geo_1" {
		t.Fatalf("expected a new geofence, got created=%v geofence=%+v", created, geofence)
	}
	if len(recorder.created) != 1 {
		t.Fatalf("expected 1 created event, got %d", len(recorder.created))
	}
}

func TestAssetsWithinRadiusValidatesInput(t *testing.T) {
	svc, _ := newTestService(&fakeRepository{})

	if _, err := svc.AssetsWithinRadius(context.Background(), geo.Point{Lat: 0, Lng: 0}, 0, 0); err == nil {
		t.Fatal("expected radius validation error")
	}
	if _, err := svc.AssetsWithinRadius(context.Background(), geo.Point{Lat: 99, Lng: 0}, 100, 0); err == nil {
		t.Fatal("expected center validation error")
	}
}
