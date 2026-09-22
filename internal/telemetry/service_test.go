package telemetry

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

type fakeRepository struct {
	created      []Sample
	createErr    error
	snapshot     StateSnapshot
	snapshots    []StateSnapshot
	snapshotErr  error
	applied      []StateUpdate
	expected     []*time.Time
	applyResults []bool
	applyResult  *bool
	applyErr     error
	staleIDs     []string
}

func (f *fakeRepository) Create(_ context.Context, sample Sample) (Sample, error) {
	if f.createErr != nil {
		return Sample{}, f.createErr
	}
	f.created = append(f.created, sample)
	return sample, nil
}

func (f *fakeRepository) StateSnapshot(_ context.Context, assetID string) (StateSnapshot, error) {
	if f.snapshotErr != nil {
		return StateSnapshot{}, f.snapshotErr
	}
	snapshot := f.snapshot
	if len(f.snapshots) > 0 {
		snapshot = f.snapshots[0]
		f.snapshots = f.snapshots[1:]
	}
	snapshot.AssetID = assetID
	return snapshot, nil
}

func (f *fakeRepository) ApplyState(_ context.Context, update StateUpdate, expected *time.Time) (bool, error) {
	if f.applyErr != nil {
		return false, f.applyErr
	}
	f.applied = append(f.applied, update)
	if expected != nil {
		copy := *expected
		f.expected = append(f.expected, &copy)
	} else {
		f.expected = append(f.expected, nil)
	}
	if len(f.applyResults) > 0 {
		result := f.applyResults[0]
		f.applyResults = f.applyResults[1:]
		return result, nil
	}
	if f.applyResult != nil {
		return *f.applyResult, nil
	}
	return true, nil
}

func (f *fakeRepository) ListStaleAssetIDs(context.Context, time.Time) ([]string, error) {
	return f.staleIDs, nil
}

func (f *fakeRepository) ListByAsset(_ context.Context, _ string, _, _ int) ([]Sample, int, error) {
	return nil, 0, nil
}

type fakeRegistry struct {
	exists bool
	err    error
}

func (f fakeRegistry) Exists(_ context.Context, _ string) (bool, error) {
	return f.exists, f.err
}

type eventLog struct {
	events []events.Event
}

func (l *eventLog) record(_ context.Context, ev events.Event) {
	l.events = append(l.events, ev)
}

func (l *eventLog) count(topic string) int {
	n := 0
	for _, ev := range l.events {
		if ev.Topic() == topic {
			n++
		}
	}
	return n
}

func newTestService(repo Repository, registry AssetRegistry) (*Service, *eventLog) {
	log := &eventLog{}
	bus := events.NewDispatcher(slog.New(slog.DiscardHandler))
	bus.Subscribe(events.TopicAssetPositionUpdated, log.record)
	bus.Subscribe(events.TopicAssetConnectionChanged, log.record)
	bus.Subscribe(events.TopicTelemetryReceived, log.record)
	return NewService(repo, registry, bus), log
}

func testInput(observedAt time.Time) CreateInput {
	return CreateInput{
		MessageID:  "msg_1",
		AssetID:    "ast_1",
		SourceID:   "src_1",
		ObservedAt: observedAt,
		Position:   &geo.Point{Lat: 10, Lng: 20},
	}
}

func TestIngestRejectsUnknownAsset(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeRepository{}
	svc, log := newTestService(repo, fakeRegistry{exists: false})

	_, err := svc.Ingest(context.Background(), testInput(now.Add(-time.Minute)))
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created %d samples, want 0", len(repo.created))
	}
	if len(log.events) != 0 {
		t.Fatalf("published %d events, want 0", len(log.events))
	}
}

func TestIngestDuplicatePublishesNothing(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeRepository{createErr: ErrDuplicateMessage}
	svc, log := newTestService(repo, fakeRegistry{exists: true})

	result, err := svc.Ingest(context.Background(), testInput(now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !result.Duplicate {
		t.Fatal("Duplicate = false, want true")
	}
	if result.Stale || result.StateUpdated {
		t.Fatalf("result = %+v, want only Duplicate set", result)
	}
	if len(repo.applied) != 0 {
		t.Fatalf("applied %d state updates, want 0", len(repo.applied))
	}
	if len(log.events) != 0 {
		t.Fatalf("published %d events, want 0", len(log.events))
	}
}

func TestIngestStaleSampleKeepsState(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	lastSeen := now.Add(-30 * time.Second)
	repo := &fakeRepository{snapshot: StateSnapshot{
		Position:        &geo.Point{Lat: 11, Lng: 21},
		ConnectionState: "connected",
		LastSeenAt:      &lastSeen,
	}}
	svc, log := newTestService(repo, fakeRegistry{exists: true})
	svc.now = func() time.Time { return now }

	result, err := svc.Ingest(context.Background(), testInput(now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !result.Stale {
		t.Fatal("Stale = false, want true")
	}
	if result.StateUpdated {
		t.Fatal("StateUpdated = true, want false")
	}
	if len(repo.applied) != 0 {
		t.Fatalf("applied %d state updates, want 0", len(repo.applied))
	}
	if len(log.events) != 1 {
		t.Fatalf("events = %v, want one telemetry.received", log.events)
	}
	received, ok := log.events[0].(events.TelemetryReceived)
	if !ok {
		t.Fatalf("event = %T, want telemetry.received", log.events[0])
	}
	if !received.Stale {
		t.Fatal("telemetry.received Stale = false, want true")
	}
}

func TestIngestOutOfOrderOlderSampleIsStale(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	lastSeen := now.Add(-time.Second)
	repo := &fakeRepository{snapshot: StateSnapshot{
		Position:        &geo.Point{Lat: 10, Lng: 20.01},
		ConnectionState: "connected",
		LastSeenAt:      &lastSeen,
	}}
	svc, log := newTestService(repo, fakeRegistry{exists: true})
	svc.now = func() time.Time { return now }

	older := testInput(now.Add(-5 * time.Minute))
	older.Position = &geo.Point{Lat: 10, Lng: 20}
	result, err := svc.Ingest(context.Background(), older)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !result.Stale || result.StateUpdated {
		t.Fatalf("result = %+v, want stale without state update", result)
	}
	if len(repo.applied) != 0 {
		t.Fatalf("applied %d state updates, want 0", len(repo.applied))
	}
	if log.count(events.TopicAssetPositionUpdated) != 0 {
		t.Fatal("published asset.position.updated for an out-of-order sample")
	}
	if log.count(events.TopicTelemetryReceived) != 1 {
		t.Fatalf("telemetry.received count = %d, want 1", log.count(events.TopicTelemetryReceived))
	}
}

func TestIngestNewerSampleAppliesStateAndPublishes(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	observedAt := now.Add(-10 * time.Second)
	lastSeen := observedAt.Add(-10 * time.Second)
	previous := geo.Point{Lat: 10, Lng: 20}
	next := geo.Point{Lat: 10, Lng: 20.001}

	repo := &fakeRepository{snapshot: StateSnapshot{
		Position:        &previous,
		ConnectionState: "unknown",
		LastSeenAt:      &lastSeen,
	}}
	svc, log := newTestService(repo, fakeRegistry{exists: true})
	svc.now = func() time.Time { return now }

	in := testInput(observedAt)
	in.Position = &next
	result, err := svc.Ingest(context.Background(), in)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if result.Stale {
		t.Fatal("Stale = true, want false")
	}
	if !result.StateUpdated {
		t.Fatal("StateUpdated = false, want true")
	}
	if len(repo.applied) != 1 {
		t.Fatalf("applied %d state updates, want 1", len(repo.applied))
	}

	update := repo.applied[0]
	if update.Position == nil || *update.Position != next {
		t.Fatalf("position = %v, want %v", update.Position, next)
	}
	if update.ConnectionState != "connected" {
		t.Fatalf("connection state = %q, want connected", update.ConnectionState)
	}
	if update.Speed == nil || update.Heading == nil {
		t.Fatalf("speed/heading = %v/%v, want derived values", update.Speed, update.Heading)
	}
	if *update.Speed <= 0 {
		t.Fatalf("speed = %v, want > 0", *update.Speed)
	}
	if *update.Heading < 85 || *update.Heading > 95 {
		t.Fatalf("heading = %v, want roughly east", *update.Heading)
	}

	if log.count(events.TopicAssetPositionUpdated) != 1 {
		t.Fatalf("asset.position.updated count = %d, want 1", log.count(events.TopicAssetPositionUpdated))
	}
	if log.count(events.TopicAssetConnectionChanged) != 1 {
		t.Fatalf("asset.connection.changed count = %d, want 1", log.count(events.TopicAssetConnectionChanged))
	}
	if log.count(events.TopicTelemetryReceived) != 1 {
		t.Fatalf("telemetry.received count = %d, want 1", log.count(events.TopicTelemetryReceived))
	}
}

func TestIngestConcurrentStateWinnerSuppressesProjectionEvents(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	lastSeen := now.Add(-time.Minute)
	applyResult := false
	repo := &fakeRepository{
		snapshot:    StateSnapshot{ConnectionState: "connected", LastSeenAt: &lastSeen},
		applyResult: &applyResult,
	}
	svc, log := newTestService(repo, fakeRegistry{exists: true})
	svc.now = func() time.Time { return now }

	result, err := svc.Ingest(context.Background(), testInput(now.Add(-time.Second)))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !result.Stale || result.StateUpdated {
		t.Fatalf("result = %+v, want stale without state update", result)
	}
	if log.count(events.TopicAssetPositionUpdated) != 0 || log.count(events.TopicAssetConnectionChanged) != 0 {
		t.Fatal("published an asset projection event after losing the compare-and-set")
	}
	if received, ok := log.events[len(log.events)-1].(events.TelemetryReceived); !ok || !received.Stale {
		t.Fatalf("last event = %#v, want stale telemetry.received", log.events[len(log.events)-1])
	}
}

func TestIngestRetriesKinematicsFromTheWinningPredecessor(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	t0 := now.Add(-30 * time.Second)
	t1 := now.Add(-20 * time.Second)
	t2 := now.Add(-10 * time.Second)
	p0 := geo.Point{Lat: 10, Lng: 20}
	p1 := geo.Point{Lat: 10, Lng: 20.001}
	p2 := geo.Point{Lat: 10, Lng: 20.002}
	last0 := t0
	last1 := t1
	repo := &fakeRepository{
		snapshots: []StateSnapshot{
			{Position: &p0, ConnectionState: "connected", LastSeenAt: &last0},
			{Position: &p1, ConnectionState: "connected", LastSeenAt: &last1},
		},
		applyResults: []bool{false, true},
	}
	svc, _ := newTestService(repo, fakeRegistry{exists: true})
	svc.now = func() time.Time { return now }
	in := testInput(t2)
	in.Position = &p2

	result, err := svc.Ingest(context.Background(), in)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !result.StateUpdated || result.Stale {
		t.Fatalf("result = %+v, want successful retry", result)
	}
	if len(repo.applied) != 2 {
		t.Fatalf("apply calls = %d, want 2", len(repo.applied))
	}
	if repo.applied[0].Speed == nil || repo.applied[1].Speed == nil {
		t.Fatalf("speeds = %v, want derived values", repo.applied)
	}
	want := geo.DistanceMeters(p1, p2) / t2.Sub(t1).Seconds()
	if *repo.applied[1].Speed != want {
		t.Fatalf("retry speed = %v, want %v from winning predecessor", *repo.applied[1].Speed, want)
	}
	if repo.expected[0] == nil || !repo.expected[0].Equal(t0) || repo.expected[1] == nil || !repo.expected[1].Equal(t1) {
		t.Fatalf("expected watermarks = %v, want [%v %v]", repo.expected, t0, t1)
	}
}

func TestReconcileStaleDelegatesToObserver(t *testing.T) {
	repo := &fakeRepository{staleIDs: []string{"ast_1", "ast_2"}}
	svc, _ := newTestService(repo, fakeRegistry{exists: true})
	observer := &staleObserver{}
	if err := svc.ReconcileStale(context.Background(), time.Now().UTC(), observer); err != nil {
		t.Fatalf("ReconcileStale() error = %v", err)
	}
	if len(observer.stale) != 2 || observer.stale[0] != "ast_1" || observer.stale[1] != "ast_2" {
		t.Fatalf("stale assets = %v, want [ast_1 ast_2]", observer.stale)
	}
}

type staleObserver struct{ stale []string }

func (o *staleObserver) ObserveStaleEntity(assetID string, stale bool) {
	if stale {
		o.stale = append(o.stale, assetID)
	}
}
