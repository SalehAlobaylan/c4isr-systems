package telemetry

import (
	"context"
	"errors"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// AssetRegistry verifies that telemetry references a registered asset.
// Implemented by the assets module.
type AssetRegistry interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// Service implements telemetry ingestion and retrieval.
type Service struct {
	repo   Repository
	assets AssetRegistry
	bus    *events.Dispatcher
	now    func() time.Time
}

// NewService wires a telemetry service.
func NewService(repo Repository, assets AssetRegistry, bus *events.Dispatcher) *Service {
	return &Service{
		repo:   repo,
		assets: assets,
		bus:    bus,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// IngestResult reports how a sample was classified during ingestion.
type IngestResult struct {
	Sample       Sample
	Duplicate    bool
	Stale        bool
	StateUpdated bool
}

// Ingest validates, persists, and applies a telemetry sample. Duplicates are
// acknowledged without side effects. Stale samples are retained as history but
// never move the state projection backwards.
func (s *Service) Ingest(ctx context.Context, in CreateInput) (IngestResult, error) {
	in.Normalize()
	now := s.now()
	if err := in.Validate(now); err != nil {
		return IngestResult{}, err
	}

	exists, err := s.assets.Exists(ctx, in.AssetID)
	if err != nil {
		return IngestResult{}, err
	}
	if !exists {
		return IngestResult{}, apperr.Validation("unknown asset: " + in.AssetID)
	}

	sample := Sample{
		ID:              in.ID,
		MessageID:       in.MessageID,
		AssetID:         in.AssetID,
		SourceID:        in.SourceID,
		ObservedAt:      in.ObservedAt,
		ReceivedAt:      in.ReceivedAt,
		Position:        in.Position,
		Speed:           in.Speed,
		Heading:         in.Heading,
		Health:          in.Health,
		ConnectionState: in.ConnectionState,
		Payload:         in.Payload,
		CreatedAt:       now,
	}

	created, err := s.repo.Create(ctx, sample)
	if err != nil {
		if errors.Is(err, ErrDuplicateMessage) {
			return IngestResult{Duplicate: true}, nil
		}
		return IngestResult{}, err
	}

	snapshot, err := s.repo.StateSnapshot(ctx, created.AssetID)
	if err != nil {
		return IngestResult{}, err
	}

	stale := snapshot.LastSeenAt != nil && created.ObservedAt.Before(*snapshot.LastSeenAt)
	result := IngestResult{Sample: created, Stale: stale}

	if !stale {
		update := StateUpdate{
			AssetID:         created.AssetID,
			Position:        created.Position,
			Speed:           created.Speed,
			Heading:         created.Heading,
			Health:          created.Health,
			ConnectionState: created.ConnectionState,
			ObservedAt:      created.ObservedAt,
		}
		deriveKinematics(&update, created, snapshot)
		if update.ConnectionState == "" {
			update.ConnectionState = "connected"
		}

		if err := s.repo.ApplyState(ctx, update); err != nil {
			return IngestResult{}, err
		}

		if created.Position != nil {
			s.bus.Publish(ctx, events.AssetPositionUpdated{
				At:         now,
				AssetID:    created.AssetID,
				Position:   *created.Position,
				Speed:      update.Speed,
				Heading:    update.Heading,
				ObservedAt: created.ObservedAt,
			})
		}
		if snapshot.ConnectionState != update.ConnectionState {
			s.bus.Publish(ctx, events.AssetConnectionChanged{
				At:              now,
				AssetID:         created.AssetID,
				ConnectionState: update.ConnectionState,
				Health:          update.Health,
			})
		}
		result.StateUpdated = true
	}

	s.bus.Publish(ctx, events.TelemetryReceived{
		At:          now,
		TelemetryID: created.ID,
		AssetID:     created.AssetID,
		SourceID:    created.SourceID,
		ObservedAt:  created.ObservedAt,
		Stale:       stale,
		Duplicate:   false,
	})
	return result, nil
}

// ListByAsset returns telemetry for an asset newest first.
func (s *Service) ListByAsset(ctx context.Context, assetID string, limit, offset int) ([]Sample, int, error) {
	return s.repo.ListByAsset(ctx, assetID, clampLimit(limit), max(offset, 0))
}

func deriveKinematics(update *StateUpdate, sample Sample, snapshot StateSnapshot) {
	if update.Speed != nil && update.Heading != nil {
		return
	}
	if sample.Position == nil || snapshot.Position == nil || snapshot.LastSeenAt == nil {
		return
	}
	seconds := sample.ObservedAt.Sub(*snapshot.LastSeenAt).Seconds()
	if seconds <= 0 {
		return
	}
	if update.Speed == nil {
		speed := geo.DistanceMeters(*snapshot.Position, *sample.Position) / seconds
		update.Speed = &speed
	}
	if update.Heading == nil {
		heading := geo.BearingDegrees(*snapshot.Position, *sample.Position)
		update.Heading = &heading
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
