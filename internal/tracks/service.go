package tracks

import (
	"context"
	"log/slog"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// ObservationProcessor marks observations as processed once they are attached
// to a track. Implemented by the observations module.
type ObservationProcessor interface {
	MarkProcessed(ctx context.Context, observationID string) error
}

// Service correlates observations into tracks. Correlation is deliberately
// simple: a scenario-supplied hint identifies an existing track, every other
// observation starts its own track.
type Service struct {
	repo      Repository
	processor ObservationProcessor
	bus       *events.Dispatcher
	now       func() time.Time
}

// NewService wires a track service and subscribes it to observation events.
func NewService(repo Repository, processor ObservationProcessor, bus *events.Dispatcher) *Service {
	s := &Service{
		repo:      repo,
		processor: processor,
		bus:       bus,
		now:       func() time.Time { return time.Now().UTC() },
	}
	bus.Subscribe(events.TopicObservationReceived, func(ctx context.Context, ev events.Event) {
		if received, ok := ev.(events.ObservationReceived); ok {
			s.HandleObservationReceived(ctx, received)
		}
	})
	return s
}

// HandleObservationReceived correlates one observation into a track. It is
// exported so tests and scenario tooling can drive correlation directly.
func (s *Service) HandleObservationReceived(ctx context.Context, ev events.ObservationReceived) {
	if ev.Duplicate {
		return
	}
	now := s.now()
	update := ObservationUpdate{
		ObservationID: ev.ObservationID,
		SourceID:      ev.SourceID,
		ObservedAt:    ev.ObservedAt,
		Position:      ev.Position,
		TrackHint:     ev.TrackHint,
	}

	track, err := s.correlate(ctx, update, now)
	if err != nil {
		slog.Default().Error("correlate observation", "error", err, "observation_id", ev.ObservationID)
		return
	}

	attached, err := s.repo.AttachObservation(ctx, track.ID, update.ObservationID)
	if err != nil {
		slog.Default().Error("attach observation", "error", err,
			"observation_id", update.ObservationID, "track_id", track.ID)
		return
	}
	if !attached {
		return
	}

	// Stale evidence stays attached but must not move the track backwards.
	if update.ObservedAt.Before(track.LastSeenAt) {
		s.markProcessed(ctx, update)
		return
	}

	var speed, heading *float64
	if update.Position != nil {
		speed, heading = s.motion(track, update)
		if err := s.repo.UpsertState(ctx, StateUpdate{
			TrackID:  track.ID,
			Position: update.Position,
			Speed:    speed,
			Heading:  heading,
		}); err != nil {
			slog.Default().Error("upsert track state", "error", err, "track_id", track.ID)
			return
		}
		if err := s.repo.AddHistoryEntry(ctx, HistoryPoint{
			ID:         ids.New("trh"),
			TrackID:    track.ID,
			ObservedAt: update.ObservedAt,
			Position:   update.Position,
			Speed:      speed,
			Heading:    heading,
		}); err != nil {
			slog.Default().Error("add track history", "error", err, "track_id", track.ID)
		}
	}

	if err := s.repo.Touch(ctx, track.ID, update.ObservedAt); err != nil {
		slog.Default().Error("touch track", "error", err, "track_id", track.ID)
	}

	s.markProcessed(ctx, update)

	s.bus.Publish(ctx, events.TrackUpdated{
		At:         now,
		TrackID:    track.ID,
		Position:   update.Position,
		Speed:      speed,
		Heading:    heading,
		ObservedAt: update.ObservedAt,
		LastSeenAt: update.ObservedAt,
	})
}

// Get returns a track by id.
func (s *Service) Get(ctx context.Context, id string) (Track, error) {
	return s.repo.Get(ctx, id)
}

// FindByExternalRef resolves a scenario/operator supplied track reference to
// the internal track identity.
func (s *Service) FindByExternalRef(ctx context.Context, ref string) (Track, error) {
	return s.repo.FindByExternalRef(ctx, ref)
}

// List returns tracks newest-seen first.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Track, int, error) {
	return s.repo.List(ctx, clampLimit(limit), max(offset, 0))
}

// History returns a track's state history oldest first.
func (s *Service) History(ctx context.Context, id string, limit int) ([]HistoryPoint, error) {
	return s.repo.ListHistory(ctx, id, clampLimit(limit))
}

// Close marks a terminal track state and publishes track.closed.
func (s *Service) Close(ctx context.Context, id string) (Track, error) {
	closed, err := s.repo.Close(ctx, id)
	if err != nil {
		return Track{}, err
	}
	s.bus.Publish(ctx, events.TrackClosed{At: s.now(), TrackID: closed.ID})
	return closed, nil
}

func (s *Service) correlate(ctx context.Context, update ObservationUpdate, now time.Time) (Track, error) {
	if update.TrackHint == "" {
		return s.createTrack(ctx, update, now)
	}
	track, err := s.repo.FindByExternalRef(ctx, update.TrackHint)
	if err == nil {
		return track, nil
	}
	if !apperr.Is(err, apperr.CodeNotFound) {
		return Track{}, err
	}
	return s.createTrack(ctx, update, now)
}

func (s *Service) createTrack(ctx context.Context, update ObservationUpdate, now time.Time) (Track, error) {
	track := Track{
		ID:          ids.New("trk"),
		ExternalRef: update.TrackHint,
		Status:      StatusActive,
		FirstSeenAt: update.ObservedAt,
		LastSeenAt:  update.ObservedAt,
		Metadata:    map[string]any{"sourceId": update.SourceID},
	}
	created, err := s.repo.Create(ctx, track)
	if err != nil {
		return Track{}, err
	}
	s.bus.Publish(ctx, events.TrackCreated{
		At:          now,
		TrackID:     created.ID,
		ExternalRef: created.ExternalRef,
		Position:    update.Position,
		ObservedAt:  update.ObservedAt,
	})
	return created, nil
}

// motion derives speed and heading from the previous projected state. It
// returns nils when the track has no previous position or time did not
// advance.
func (s *Service) motion(track Track, update ObservationUpdate) (*float64, *float64) {
	if track.Position == nil || update.Position == nil {
		return nil, nil
	}
	seconds := update.ObservedAt.Sub(track.LastSeenAt).Seconds()
	if seconds <= 0 {
		return nil, nil
	}
	speed := geo.DistanceMeters(*track.Position, *update.Position) / seconds
	heading := geo.BearingDegrees(*track.Position, *update.Position)
	return &speed, &heading
}

func (s *Service) markProcessed(ctx context.Context, update ObservationUpdate) {
	if err := s.processor.MarkProcessed(ctx, update.ObservationID); err != nil {
		slog.Default().Error("mark observation processed", "error", err,
			"observation_id", update.ObservationID)
	}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
