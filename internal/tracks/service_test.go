package tracks

import (
	"context"
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

type fakeRepository struct {
	tracks   map[string]Track
	states   map[string]StateUpdate
	history  []HistoryPoint
	attached map[string]string
	touched  map[string]time.Time
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		tracks:   map[string]Track{},
		states:   map[string]StateUpdate{},
		attached: map[string]string{},
		touched:  map[string]time.Time{},
	}
}

func (f *fakeRepository) Create(_ context.Context, track Track) (Track, error) {
	f.tracks[track.ID] = track
	return track, nil
}

func (f *fakeRepository) Get(_ context.Context, id string) (Track, error) {
	track, ok := f.tracks[id]
	if !ok {
		return Track{}, apperr.NotFound("track", id)
	}
	return track, nil
}

func (f *fakeRepository) FindByExternalRef(_ context.Context, ref string) (Track, error) {
	for _, track := range f.tracks {
		if track.ExternalRef == ref {
			return track, nil
		}
	}
	return Track{}, apperr.NotFound("track", ref)
}

func (f *fakeRepository) List(_ context.Context, _, _ int) ([]Track, int, error) {
	out := make([]Track, 0, len(f.tracks))
	for _, track := range f.tracks {
		out = append(out, track)
	}
	return out, len(out), nil
}

func (f *fakeRepository) Touch(_ context.Context, id string, observedAt time.Time) error {
	track, ok := f.tracks[id]
	if !ok {
		return apperr.NotFound("track", id)
	}
	if observedAt.After(track.LastSeenAt) {
		track.LastSeenAt = observedAt
	}
	f.tracks[id] = track
	f.touched[id] = observedAt
	return nil
}

func (f *fakeRepository) Close(_ context.Context, id string) (Track, error) {
	track, ok := f.tracks[id]
	if !ok {
		return Track{}, apperr.NotFound("track", id)
	}
	track.Status = StatusClosed
	f.tracks[id] = track
	return track, nil
}

func (f *fakeRepository) UpsertState(_ context.Context, update StateUpdate) error {
	f.states[update.TrackID] = update
	track := f.tracks[update.TrackID]
	if update.Position != nil {
		track.Position = update.Position
	}
	if update.Speed != nil {
		track.Speed = update.Speed
	}
	if update.Heading != nil {
		track.Heading = update.Heading
	}
	f.tracks[update.TrackID] = track
	return nil
}

func (f *fakeRepository) AttachObservation(_ context.Context, trackID, observationID string) (bool, error) {
	if _, ok := f.attached[observationID]; ok {
		return false, nil
	}
	f.attached[observationID] = trackID
	return true, nil
}

func (f *fakeRepository) AddHistoryEntry(_ context.Context, entry HistoryPoint) error {
	f.history = append(f.history, entry)
	return nil
}

func (f *fakeRepository) ListHistory(_ context.Context, trackID string, _ int) ([]HistoryPoint, error) {
	out := make([]HistoryPoint, 0, len(f.history))
	for _, entry := range f.history {
		if entry.TrackID == trackID {
			out = append(out, entry)
		}
	}
	return out, nil
}

func (f *fakeRepository) CountObservations(_ context.Context, trackID string) (int, error) {
	count := 0
	for _, attachedTo := range f.attached {
		if attachedTo == trackID {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepository) CountObservationsBySource(_ context.Context, _ string) ([]SourceCount, error) {
	return []SourceCount{}, nil
}

var _ Repository = (*fakeRepository)(nil)

type fakeProcessor struct {
	processed []string
	err       error
}

func (p *fakeProcessor) MarkProcessed(_ context.Context, observationID string) error {
	if p.err != nil {
		return p.err
	}
	p.processed = append(p.processed, observationID)
	return nil
}

func testDispatcher() *events.Dispatcher {
	return events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHandleObservationReceivedCreatesTrackForHint(t *testing.T) {
	bus := testDispatcher()
	var created []events.TrackCreated
	bus.Subscribe(events.TopicTrackCreated, func(_ context.Context, ev events.Event) {
		if e, ok := ev.(events.TrackCreated); ok {
			created = append(created, e)
		}
	})

	repo := newFakeRepository()
	processor := &fakeProcessor{}
	svc := NewService(repo, processor, bus)

	observedAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	position := &geo.Point{Lat: 36.2, Lng: 36.1}
	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		At:            observedAt,
		ObservationID: "obs_1",
		SourceID:      "src_1",
		ObservedAt:    observedAt,
		Position:      position,
		TrackHint:     "TRK-HINT-1",
	})

	if len(repo.tracks) != 1 {
		t.Fatalf("tracks = %d, want 1", len(repo.tracks))
	}
	var track Track
	for _, candidate := range repo.tracks {
		track = candidate
	}
	if track.ExternalRef != "TRK-HINT-1" {
		t.Errorf("externalRef = %q, want TRK-HINT-1", track.ExternalRef)
	}
	if track.Status != StatusActive {
		t.Errorf("status = %q, want active", track.Status)
	}
	if track.Metadata["sourceId"] != "src_1" {
		t.Errorf("metadata sourceId = %v, want src_1", track.Metadata["sourceId"])
	}
	if !track.FirstSeenAt.Equal(observedAt) || !track.LastSeenAt.Equal(observedAt) {
		t.Errorf("seen window = %v..%v, want %v", track.FirstSeenAt, track.LastSeenAt, observedAt)
	}
	if !strings.HasPrefix(track.ID, "trk_") {
		t.Errorf("id = %q, want trk_ prefix", track.ID)
	}

	if len(created) != 1 {
		t.Fatalf("track.created events = %d, want 1", len(created))
	}
	if created[0].TrackID != track.ID || created[0].ExternalRef != "TRK-HINT-1" {
		t.Errorf("track.created = %+v, want track %s", created[0], track.ID)
	}
	if created[0].Position == nil || *created[0].Position != *position {
		t.Errorf("track.created position = %v, want %v", created[0].Position, position)
	}

	if repo.attached["obs_1"] != track.ID {
		t.Errorf("obs_1 attached to %q, want %q", repo.attached["obs_1"], track.ID)
	}
	state, ok := repo.states[track.ID]
	if !ok || state.Position == nil || *state.Position != *position {
		t.Fatalf("state = %+v, want position %v", state, position)
	}
	if len(repo.history) != 1 {
		t.Fatalf("history entries = %d, want 1", len(repo.history))
	}
	if !strings.HasPrefix(repo.history[0].ID, "trh_") || !repo.history[0].ObservedAt.Equal(observedAt) {
		t.Errorf("history[0] = %+v, want trh_ id at %v", repo.history[0], observedAt)
	}
	if _, ok := repo.touched[track.ID]; !ok {
		t.Error("track was not touched")
	}
	if len(processor.processed) != 1 || processor.processed[0] != "obs_1" {
		t.Errorf("processed = %v, want [obs_1]", processor.processed)
	}
}

func TestHandleObservationReceivedWithoutHintCreatesOwnTrack(t *testing.T) {
	bus := testDispatcher()
	repo := newFakeRepository()
	processor := &fakeProcessor{}
	svc := NewService(repo, processor, bus)

	observedAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	for i, observationID := range []string{"obs_1", "obs_2"} {
		svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
			ObservationID: observationID,
			SourceID:      "src_1",
			ObservedAt:    observedAt.Add(time.Duration(i) * time.Minute),
			Position:      &geo.Point{Lat: 36, Lng: 36},
		})
	}

	if len(repo.tracks) != 2 {
		t.Fatalf("tracks = %d, want 2 (unhinted observations never correlate)", len(repo.tracks))
	}
	for _, track := range repo.tracks {
		if track.ExternalRef != "" {
			t.Errorf("externalRef = %q, want empty", track.ExternalRef)
		}
	}
}

func TestHandleObservationReceivedStaleDoesNotRegressState(t *testing.T) {
	bus := testDispatcher()
	var updated []events.TrackUpdated
	bus.Subscribe(events.TopicTrackUpdated, func(_ context.Context, ev events.Event) {
		if e, ok := ev.(events.TrackUpdated); ok {
			updated = append(updated, e)
		}
	})

	repo := newFakeRepository()
	lastSeen := time.Date(2026, 3, 1, 10, 5, 0, 0, time.UTC)
	newest := &geo.Point{Lat: 37, Lng: 37}
	repo.tracks["trk_existing"] = Track{
		ID:          "trk_existing",
		ExternalRef: "TRK-HINT-1",
		Status:      StatusActive,
		FirstSeenAt: lastSeen.Add(-5 * time.Minute),
		LastSeenAt:  lastSeen,
		Position:    newest,
		Metadata:    map[string]any{},
	}
	processor := &fakeProcessor{}
	svc := NewService(repo, processor, bus)

	staleAt := lastSeen.Add(-time.Minute)
	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		At:            staleAt,
		ObservationID: "obs_stale",
		SourceID:      "src_1",
		ObservedAt:    staleAt,
		Position:      &geo.Point{Lat: 30, Lng: 30},
		TrackHint:     "TRK-HINT-1",
	})

	if repo.attached["obs_stale"] != "trk_existing" {
		t.Fatalf("stale evidence attached to %q, want trk_existing", repo.attached["obs_stale"])
	}
	if len(repo.states) != 0 {
		t.Errorf("stale observation updated state: %+v", repo.states)
	}
	if len(repo.history) != 0 {
		t.Errorf("stale observation added history: %+v", repo.history)
	}
	if len(repo.touched) != 0 {
		t.Errorf("stale observation touched track: %+v", repo.touched)
	}
	if len(updated) != 0 {
		t.Errorf("stale observation published %d track.updated events", len(updated))
	}
	if len(processor.processed) != 1 || processor.processed[0] != "obs_stale" {
		t.Errorf("processed = %v, want [obs_stale]", processor.processed)
	}
	stored := repo.tracks["trk_existing"]
	if !stored.LastSeenAt.Equal(lastSeen) {
		t.Errorf("lastSeenAt = %v, want %v", stored.LastSeenAt, lastSeen)
	}
	if stored.Position == nil || *stored.Position != *newest {
		t.Errorf("position = %v, want %v", stored.Position, newest)
	}
}

func TestHandleObservationReceivedAttachIsIdempotent(t *testing.T) {
	bus := testDispatcher()
	var updated []events.TrackUpdated
	bus.Subscribe(events.TopicTrackUpdated, func(_ context.Context, ev events.Event) {
		if e, ok := ev.(events.TrackUpdated); ok {
			updated = append(updated, e)
		}
	})

	repo := newFakeRepository()
	lastSeen := time.Date(2026, 3, 1, 10, 5, 0, 0, time.UTC)
	repo.tracks["trk_existing"] = Track{
		ID:          "trk_existing",
		ExternalRef: "TRK-HINT-1",
		Status:      StatusActive,
		FirstSeenAt: lastSeen.Add(-5 * time.Minute),
		LastSeenAt:  lastSeen,
		Position:    &geo.Point{Lat: 37, Lng: 37},
		Metadata:    map[string]any{},
	}
	repo.attached["obs_dup"] = "trk_existing"
	processor := &fakeProcessor{}
	svc := NewService(repo, processor, bus)

	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		At:            lastSeen.Add(time.Minute),
		ObservationID: "obs_dup",
		SourceID:      "src_1",
		ObservedAt:    lastSeen.Add(time.Minute),
		Position:      &geo.Point{Lat: 38, Lng: 38},
		TrackHint:     "TRK-HINT-1",
	})

	if len(repo.states) != 0 {
		t.Errorf("re-attachment updated state: %+v", repo.states)
	}
	if len(repo.history) != 0 {
		t.Errorf("re-attachment added history: %+v", repo.history)
	}
	if len(repo.touched) != 0 {
		t.Errorf("re-attachment touched track: %+v", repo.touched)
	}
	if len(updated) != 0 {
		t.Errorf("re-attachment published %d track.updated events", len(updated))
	}
	if len(processor.processed) != 0 {
		t.Errorf("re-attachment marked processed: %v", processor.processed)
	}
}

func TestHandleObservationReceivedComputesSpeedAndHeading(t *testing.T) {
	bus := testDispatcher()
	repo := newFakeRepository()
	processor := &fakeProcessor{}
	svc := NewService(repo, processor, bus)

	lastSeen := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	repo.tracks["trk_existing"] = Track{
		ID:          "trk_existing",
		ExternalRef: "TRK-HINT-1",
		Status:      StatusActive,
		FirstSeenAt: lastSeen,
		LastSeenAt:  lastSeen,
		Position:    &geo.Point{Lat: 0, Lng: 0},
		Metadata:    map[string]any{},
	}

	observedAt := lastSeen.Add(10 * time.Second)
	svc.HandleObservationReceived(context.Background(), events.ObservationReceived{
		ObservationID: "obs_2",
		SourceID:      "src_1",
		ObservedAt:    observedAt,
		Position:      &geo.Point{Lat: 0, Lng: 0.001},
		TrackHint:     "TRK-HINT-1",
	})

	state, ok := repo.states["trk_existing"]
	if !ok {
		t.Fatal("state was not updated")
	}
	if state.Speed == nil || math.Abs(*state.Speed-11.13) > 0.1 {
		t.Errorf("speed = %v, want ~11.13 m/s", state.Speed)
	}
	if state.Heading == nil || math.Abs(*state.Heading-90) > 0.1 {
		t.Errorf("heading = %v, want ~90", state.Heading)
	}
	if len(repo.history) != 1 {
		t.Fatalf("history entries = %d, want 1", len(repo.history))
	}
	if repo.history[0].Speed == nil || repo.history[0].Heading == nil {
		t.Errorf("history entry missing motion: %+v", repo.history[0])
	}
}
