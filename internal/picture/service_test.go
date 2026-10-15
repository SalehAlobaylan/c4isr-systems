package picture

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

type fakeRepository struct {
	window    Window
	tracks    []Track
	assets    []Asset
	alerts    []Alert
	geofences []Geofence

	windowErr error
	tracksErr error
	assetsErr error
	alertsErr error

	tracksAt time.Time
	assetsAt time.Time
	scope    string
}

func (f *fakeRepository) Window(context.Context, string) (Window, error) {
	return f.window, f.windowErr
}

func (f *fakeRepository) TracksAt(_ context.Context, at time.Time, scope string) ([]Track, error) {
	f.tracksAt = at
	f.scope = scope
	return f.tracks, f.tracksErr
}

func (f *fakeRepository) AssetsAt(_ context.Context, at time.Time, _ string) ([]Asset, error) {
	f.assetsAt = at
	return f.assets, f.assetsErr
}

func (f *fakeRepository) AlertsAt(context.Context, time.Time, int) ([]Alert, error) {
	return f.alerts, f.alertsErr
}

func (f *fakeRepository) GeofencesAt(context.Context, int) ([]Geofence, error) {
	return f.geofences, nil
}

func testPolicy() freshness.Policy {
	return freshness.Policy{
		DelayedAfter:     time.Minute,
		StaleAfter:       5 * time.Minute,
		UnavailableAfter: 30 * time.Minute,
	}
}

var base = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func TestSnapshotReconstructsFromEvidenceNotCurrentState(t *testing.T) {
	repo := &fakeRepository{
		window: Window{Earliest: base, Latest: base.Add(time.Minute)},
		tracks: []Track{{ID: "trk_1", Status: "active", ObservedAt: base.Add(20 * time.Second)}},
		assets: []Asset{{ID: "ast_1", Status: "available", ObservedAt: base.Add(20 * time.Second)}},
	}
	svc := NewService(repo, testPolicy())
	svc.now = func() time.Time { return base.Add(5 * time.Minute) }

	snapshot, err := svc.Snapshot(context.Background(), Request{At: base.Add(30 * time.Second)})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if !snapshot.At.Equal(base.Add(30 * time.Second)) {
		t.Errorf("At = %v, want %v", snapshot.At, base.Add(30*time.Second))
	}
	if !repo.tracksAt.Equal(base.Add(30 * time.Second)) {
		t.Errorf("TracksAt queried at %v, want the requested instant", repo.tracksAt)
	}
	if !repo.assetsAt.Equal(base.Add(30 * time.Second)) {
		t.Errorf("AssetsAt queried at %v, want the requested instant", repo.assetsAt)
	}
}

func TestFreshnessIsDerivedFromTheRequestedInstant(t *testing.T) {
	repo := &fakeRepository{
		window: Window{Earliest: base, Latest: base.Add(time.Hour)},
		tracks: []Track{{ID: "trk_1", Status: "active", ObservedAt: base.Add(20 * time.Second)}},
	}
	svc := NewService(repo, testPolicy())
	svc.now = func() time.Time { return base.Add(time.Hour) }

	// Replaying to T+30s: the observation is 10s old *at that moment*, so the
	// track was live then even though it is an hour stale now. Computing
	// freshness against the wall clock would repaint history.
	snapshot, err := svc.Snapshot(context.Background(), Request{At: base.Add(30 * time.Second)})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if got := snapshot.Tracks[0].Freshness; got != freshness.StateLive {
		t.Fatalf("freshness at the replayed instant = %q, want %q", got, freshness.StateLive)
	}

	// The same track, judged now, is a different question and gets a different
	// answer.
	now, err := svc.Snapshot(context.Background(), Request{Live: true})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if !now.Live {
		t.Error("an explicit live request must report the picture as live")
	}
	if got := now.Tracks[0].Freshness; got == freshness.StateLive {
		t.Fatalf("freshness now = %q, want a non-live band", got)
	}
}

func TestClampsAnInstantOutsideTheEvidenceWindow(t *testing.T) {
	repo := &fakeRepository{
		window: Window{Earliest: base.Add(time.Minute), Latest: base.Add(2 * time.Minute)},
	}
	svc := NewService(repo, testPolicy())
	svc.now = func() time.Time { return base.Add(time.Hour) }

	// Before the window: the earliest moment that actually exists.
	before, err := svc.Snapshot(context.Background(), Request{At: base})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if !before.At.Equal(base.Add(time.Minute)) {
		t.Errorf("At = %v, want the window start", before.At)
	}

	// After the window: the latest.
	after, err := svc.Snapshot(context.Background(), Request{At: base.Add(10 * time.Hour)})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if !after.At.Equal(base.Add(2 * time.Minute)) {
		t.Errorf("At = %v, want the window end", after.At)
	}

	// A clamped instant is historical, not live.
	if after.Live {
		t.Error("a clamped instant must not be reported as live")
	}
}

func TestSnapshotWithoutARequestIsTheLivePicture(t *testing.T) {
	now := base.Add(time.Hour)
	repo := &fakeRepository{
		window: Window{Earliest: base, Latest: now},
		tracks: []Track{{ID: "trk_1", ObservedAt: now}},
	}
	svc := NewService(repo, testPolicy())
	svc.now = func() time.Time { return now }

	snapshot, err := svc.Snapshot(context.Background(), Request{})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if !snapshot.Live {
		t.Error("an unrequested snapshot is the live picture")
	}
	if !snapshot.At.Equal(now) {
		t.Errorf("At = %v, want now", snapshot.At)
	}
}

func TestSnapshotReturnsEmptyCollectionsNotNulls(t *testing.T) {
	repo := &fakeRepository{window: Window{Earliest: base, Latest: base.Add(time.Minute)}}
	svc := NewService(repo, testPolicy())
	svc.now = func() time.Time { return base.Add(time.Minute) }

	snapshot, err := svc.Snapshot(context.Background(), Request{})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	// A JSON null would render as a broken interface; an empty array does not.
	for name, collection := range map[string]any{
		"Tracks":    snapshot.Tracks,
		"Assets":    snapshot.Assets,
		"Alerts":    snapshot.Alerts,
		"Geofences": snapshot.Geofences,
	} {
		if collection == nil {
			t.Errorf("%s is nil; the contract promises an array", name)
		}
	}
}

func TestSnapshotPropagatesRepositoryFailure(t *testing.T) {
	svc := NewService(&fakeRepository{
		window:    Window{Earliest: base, Latest: base.Add(time.Minute)},
		tracksErr: errors.New("database unavailable"),
	}, testPolicy())

	if _, err := svc.Snapshot(context.Background(), Request{}); err == nil {
		t.Fatal("a failed reconstruction must not return an empty picture that looks real")
	}
}

func TestSnapshotScopeIsPassedThrough(t *testing.T) {
	repo := &fakeRepository{window: Window{Earliest: base, Latest: base.Add(time.Minute)}}
	svc := NewService(repo, testPolicy())
	svc.now = func() time.Time { return base.Add(time.Minute) }

	if _, err := svc.Snapshot(context.Background(), Request{ScopePrefix: "run_1__"}); err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if repo.scope != "run_1__" {
		t.Fatalf("scope = %q, want run_1__", repo.scope)
	}
}

func TestPositionSurvivesReconstruction(t *testing.T) {
	repo := &fakeRepository{
		window: Window{Earliest: base, Latest: base.Add(time.Minute)},
		tracks: []Track{{
			ID:         "trk_1",
			ObservedAt: base.Add(20 * time.Second),
			Position:   &geo.Point{Lat: 24.71, Lng: 46.68},
			Speed:      func() *float64 { v := 12.5; return &v }(),
		}},
	}
	svc := NewService(repo, testPolicy())
	svc.now = func() time.Time { return base.Add(time.Minute) }

	snapshot, err := svc.Snapshot(context.Background(), Request{At: base.Add(30 * time.Second)})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	track := snapshot.Tracks[0]
	if track.Position == nil {
		t.Fatal("reconstructed track lost its position")
	}
	if track.Position.Lat != 24.71 || track.Position.Lng != 46.68 {
		t.Fatalf("position = %+v, want the recorded one", track.Position)
	}
	if track.Speed == nil || *track.Speed != 12.5 {
		t.Fatalf("speed = %v, want 12.5", track.Speed)
	}
}

func TestWindowEmptyWhenNoEvidenceExists(t *testing.T) {
	cases := []struct {
		name   string
		window Window
		want   bool
	}{
		{name: "no evidence at all", window: Window{}, want: true},
		{name: "only a start", window: Window{Earliest: base}, want: true},
		{name: "only an end", window: Window{Latest: base}, want: true},
		{name: "inverted", window: Window{Earliest: base.Add(time.Minute), Latest: base}, want: true},
		{name: "a single instant", window: Window{Earliest: base, Latest: base}, want: false},
		{name: "a real span", window: Window{Earliest: base, Latest: base.Add(time.Minute)}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.window.Empty(); got != tc.want {
				t.Fatalf("Empty() = %v, want %v", got, tc.want)
			}
		})
	}
}
