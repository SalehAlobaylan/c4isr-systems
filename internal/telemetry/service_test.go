package telemetry

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

type fakeRepository struct {
	created          []Sample
	createErr        error
	snapshot         StateSnapshot
	snapshots        []StateSnapshot
	snapshotErr      error
	applied          []StateUpdate
	expected         []*time.Time
	applyResults     []bool
	applyResult      *bool
	applyErr         error
	freshnessSignals []FreshnessSignal
	// trackFreshnessSignals mirrors freshnessSignals for tracks.
	trackFreshnessSignals []TrackFreshnessSignal
	trackFreshnessErr     error
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

func (f *fakeRepository) ListTrackFreshnessSignals(context.Context) ([]TrackFreshnessSignal, error) {
	return f.trackFreshnessSignals, f.trackFreshnessErr
}

func (f *fakeRepository) ListFreshnessSignals(context.Context) ([]FreshnessSignal, error) {
	return f.freshnessSignals, nil
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

func (l *eventLog) reset() {
	l.events = nil
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

type staleObserver struct{ stale []string }

// contains reports whether the gauge currently holds an entity.
func (o *staleObserver) contains(id string) bool {
	for _, held := range o.stale {
		if held == id {
			return true
		}
	}
	return false
}

func (o *staleObserver) ObserveStaleEntity(assetID string, stale bool) {
	if stale {
		o.stale = append(o.stale, assetID)
	}
}

// transitionObserver records every gauge update, including recovery, so a test
// can assert on the full sequence rather than only the stale entries.
type transitionObserver struct {
	calls []string
}

func (o *transitionObserver) ObserveStaleEntity(assetID string, stale bool) {
	state := "live"
	if stale {
		state = "stale"
	}
	o.calls = append(o.calls, assetID+":"+state)
}

func monitorTestPolicy() freshness.Policy {
	return freshness.Policy{
		DelayedAfter:     time.Minute,
		StaleAfter:       5 * time.Minute,
		UnavailableAfter: 30 * time.Minute,
	}
}

func newTestMonitor(t *testing.T, repo *fakeRepository, observer FreshnessObserver, now time.Time) (*FreshnessMonitor, *eventLog) {
	t.Helper()
	log := &eventLog{}
	bus := events.NewDispatcher(slog.New(slog.DiscardHandler))
	bus.Subscribe(events.TopicEntityFreshnessChanged, log.record)
	monitor := NewFreshnessMonitor(repo, bus, observer, monitorTestPolicy())
	monitor.now = func() time.Time { return now }
	return monitor, log
}

func freshnessEvents(log *eventLog) []events.EntityFreshnessChanged {
	var out []events.EntityFreshnessChanged
	for _, ev := range log.events {
		if changed, ok := ev.(events.EntityFreshnessChanged); ok {
			out = append(out, changed)
		}
	}
	return out
}

func TestFreshnessMonitorPublishesFirstClassification(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-20 * time.Second)
	quiet := now.Add(-12 * time.Minute)
	repo := &fakeRepository{freshnessSignals: []FreshnessSignal{
		{AssetID: "ast_fresh", Status: "available", ConnectionState: "connected", LastSeenAt: &fresh},
		{AssetID: "ast_quiet", Status: "available", ConnectionState: "connected", LastSeenAt: &quiet},
	}}

	monitor, log := newTestMonitor(t, repo, &staleObserver{}, now)
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	byID := map[string]events.EntityFreshnessChanged{}
	for _, ev := range freshnessEvents(log) {
		byID[ev.EntityID] = ev
	}
	if got := byID["ast_fresh"].Freshness; got != string(freshness.StateLive) {
		t.Errorf("ast_fresh freshness = %q, want live", got)
	}
	if got := byID["ast_quiet"].Freshness; got != string(freshness.StateStale) {
		t.Errorf("ast_quiet freshness = %q, want stale", got)
	}
}

func TestFreshnessMonitorReportsOnlyTransitions(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-20 * time.Second)
	signals := []FreshnessSignal{
		{AssetID: "ast_1", Status: "available", ConnectionState: "connected", LastSeenAt: &fresh},
		{AssetID: "ast_2", Status: "available", ConnectionState: "connected", LastSeenAt: &fresh},
	}
	repo := &fakeRepository{freshnessSignals: signals}
	monitor, log := newTestMonitor(t, repo, &staleObserver{}, now)

	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	first := len(freshnessEvents(log))
	if first != 2 {
		t.Fatalf("first tick published %d events, want 2", first)
	}

	log.reset()
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}
	if got := len(freshnessEvents(log)); got != 0 {
		t.Fatalf("unchanged tick published %d events, want 0", got)
	}
}

func TestFreshnessMonitorPublishesRecovery(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	quiet := now.Add(-12 * time.Minute)
	fresh := now.Add(-10 * time.Second)
	signals := []FreshnessSignal{
		{AssetID: "ast_1", Status: "available", ConnectionState: "connected", LastSeenAt: &quiet},
	}
	repo := &fakeRepository{freshnessSignals: signals}
	monitor, log := newTestMonitor(t, repo, &staleObserver{}, now)

	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if got := freshnessEvents(log)[0].Freshness; got != string(freshness.StateStale) {
		t.Fatalf("first tick freshness = %q, want stale", got)
	}

	// The asset resumes reporting. The previous stale-only query could not
	// observe this, which is why the monitor reads every asset each tick.
	repo.freshnessSignals = []FreshnessSignal{
		{AssetID: "ast_1", Status: "available", ConnectionState: "connected", LastSeenAt: &fresh},
	}
	log.reset()
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	events := freshnessEvents(log)
	if len(events) != 1 {
		t.Fatalf("recovery published %d events, want 1", len(events))
	}
	if events[0].Freshness != string(freshness.StateLive) || events[0].Previous != string(freshness.StateStale) {
		t.Fatalf("recovery event = %+v, want stale -> live", events[0])
	}
}

// A track that has gone quiet is the case an operator most needs to be told
// about, and it is the primary thing on a C4ISR picture. Counting only assets
// would leave a silent contact invisible to the only freshness metric.
func TestFreshnessMonitorClassifiesTracks(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		trackFreshnessSignals: []TrackFreshnessSignal{
			{TrackID: "trk_fresh", Status: "active", LastSeenAt: now.Add(-10 * time.Second)},
			{TrackID: "trk_quiet", Status: "active", LastSeenAt: now.Add(-12 * time.Minute)},
			{TrackID: "trk_lost", Status: "lost", LastSeenAt: now.Add(-5 * time.Second)},
			{TrackID: "trk_slow", Status: "active", LastSeenAt: now.Add(-2 * time.Minute)},
		},
	}
	observer := &staleObserver{}
	monitor, log := newTestMonitor(t, repo, observer, now)
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	byID := map[string]events.EntityFreshnessChanged{}
	for _, ev := range freshnessEvents(log) {
		byID[ev.EntityID] = ev
	}

	// A track that declared itself lost is unavailable, not merely stale: we
	// know contact was lost rather than inferring it from quietness.
	if got := byID["trk_lost"].Freshness; got != string(freshness.StateUnavailable) {
		t.Errorf("trk_lost freshness = %q, want unavailable", got)
	}
	if got := byID["trk_lost"].EntityType; got != EntityTypeTrack {
		t.Errorf("trk_lost entity type = %q, want track", got)
	}
	if got := byID["trk_quiet"].Freshness; got != string(freshness.StateStale) {
		t.Errorf("trk_quiet freshness = %q, want stale", got)
	}
	if got := byID["trk_fresh"].Freshness; got != string(freshness.StateLive) {
		t.Errorf("trk_fresh freshness = %q, want live", got)
	}
	// Delayed is still within a plausible window, so it must not reach the
	// gauge. Counting it would page an operator about a contact that is fine.
	if got := byID["trk_slow"].Freshness; got != string(freshness.StateDelayed) {
		t.Errorf("trk_slow freshness = %q, want delayed", got)
	}

	for _, id := range []string{"trk_quiet", "trk_lost"} {
		if !observer.contains("track:" + id) {
			t.Errorf("gauge does not contain track:%s, stale set = %v", id, observer.stale)
		}
	}
	for _, id := range []string{"trk_fresh", "trk_slow"} {
		if observer.contains("track:" + id) {
			t.Errorf("gauge contains track:%s but it is trustworthy", id)
		}
	}
}

// An asset and a track that share an id must not collapse into one another in
// the gauge. Ids are normally prefixed apart, but a deployment that does not
// prefix them would otherwise silently lose whichever entity was seen second.
func TestFreshnessMonitorSeparatesAssetAndTrackWithTheSameID(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-10 * time.Second)
	repo := &fakeRepository{
		freshnessSignals: []FreshnessSignal{
			{AssetID: "shared", Status: "available", ConnectionState: "connected", LastSeenAt: &fresh},
		},
		trackFreshnessSignals: []TrackFreshnessSignal{
			{TrackID: "shared", Status: "active", LastSeenAt: now.Add(-12 * time.Minute)},
		},
	}
	observer := &staleObserver{}
	monitor, _ := newTestMonitor(t, repo, observer, now)
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if observer.contains("asset:shared") {
		t.Error("a live asset was counted in the stale gauge")
	}
	if !observer.contains("track:shared") {
		t.Errorf("a stale track sharing an id with a live asset was not counted; stale set = %v", observer.stale)
	}
	if got := len(observer.stale); got != 1 {
		t.Fatalf("stale gauge holds %d entities, want exactly the stale track", got)
	}
}

// A restart empties the gauge and the monitor's memory of previous states. If
// the first tick after a restart did not report every entity, the gauge would
// read zero on a picture full of silent contacts until each one happened to
// transition - which is the failure the bulk-set reconciler used to paper over
// and which removing it must not reintroduce.
func TestFreshnessMonitorRebuildsTheGaugeAfterRestart(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		freshnessSignals: []FreshnessSignal{
			{AssetID: "ast_quiet", Status: "available", ConnectionState: "connected",
				LastSeenAt: ptr(now.Add(-12 * time.Minute))},
		},
		trackFreshnessSignals: []TrackFreshnessSignal{
			{TrackID: "trk_quiet", Status: "active", LastSeenAt: now.Add(-12 * time.Minute)},
		},
	}

	// A fresh monitor is exactly the post-restart condition: no memory, no
	// accumulated gauge, first tick is the only chance to report.
	observer := &staleObserver{}
	monitor, _ := newTestMonitor(t, repo, observer, now)
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if got := len(observer.stale); got != 2 {
		t.Fatalf("first tick after a restart rebuilt a gauge of %d, want 2; stale set = %v",
			got, observer.stale)
	}
}

func ptr(t time.Time) *time.Time { return &t }

func TestFreshnessMonitorClearsGaugeForRemovedAsset(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	quiet := now.Add(-40 * time.Minute)
	repo := &fakeRepository{freshnessSignals: []FreshnessSignal{
		{AssetID: "ast_gone", Status: "available", ConnectionState: "connected", LastSeenAt: &quiet},
	}}
	observer := &staleObserver{}
	monitor, _ := newTestMonitor(t, repo, observer, now)

	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if len(observer.stale) != 1 {
		t.Fatalf("stale gauge = %v, want one asset", observer.stale)
	}

	repo.freshnessSignals = nil
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}
	if got := monitor.Snapshot(); len(got) != 0 {
		t.Fatalf("snapshot after removal = %v, want empty", got)
	}
}

func TestFreshnessMonitorHandlesDeclaredLossWithoutAge(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-5 * time.Second)
	repo := &fakeRepository{freshnessSignals: []FreshnessSignal{
		{AssetID: "ast_1", Status: "available", ConnectionState: "connected", LastSeenAt: &fresh},
	}}
	observer := &transitionObserver{}
	monitor, log := newTestMonitor(t, repo, observer, now)
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	repo.freshnessSignals = []FreshnessSignal{
		{AssetID: "ast_1", Status: "offline", ConnectionState: "connected", LastSeenAt: &fresh},
	}
	if err := monitor.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	transitions := freshnessEvents(log)
	last := transitions[len(transitions)-1]
	if last.Freshness != string(freshness.StateUnavailable) {
		t.Fatalf("declared offline freshness = %q, want unavailable", last.Freshness)
	}
	// Keys are type-namespaced so an asset and a track sharing an id cannot
	// collapse into one another in the gauge.
	want := []string{"asset:ast_1:live", "asset:ast_1:stale"}
	if len(observer.calls) != len(want) {
		t.Fatalf("gauge updates = %v, want %v", observer.calls, want)
	}
	for i := range want {
		if observer.calls[i] != want[i] {
			t.Fatalf("gauge updates = %v, want %v", observer.calls, want)
		}
	}
}
