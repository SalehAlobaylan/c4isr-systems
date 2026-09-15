package commands

import (
	"context"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// DefaultLimit and MaxLimit bound list queries.
const (
	DefaultLimit = 100
	MaxLimit     = 500
)

// Service implements command issue and lifecycle workflows.
type Service struct {
	repo   Repository
	assets AssetRegistry
	bus    *events.Dispatcher
}

// NewService wires a command service.
func NewService(repo Repository, assets AssetRegistry, bus *events.Dispatcher) *Service {
	return &Service{repo: repo, assets: assets, bus: bus}
}

// Issue validates, persists, and publishes a command handed to transport.
func (s *Service) Issue(ctx context.Context, in IssueInput) (Command, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return Command{}, err
	}
	exists, err := s.assets.Exists(ctx, in.AssetID)
	if err != nil {
		return Command{}, err
	}
	if !exists {
		return Command{}, apperr.Validation("unknown asset: " + in.AssetID)
	}

	command := Command{
		ID:            ids.New("cmd"),
		AssetID:       in.AssetID,
		MissionID:     in.MissionID,
		IncidentID:    in.IncidentID,
		Type:          in.Type,
		Payload:       in.Payload,
		State:         StateSent,
		CreatedBy:     in.Actor,
		CorrelationID: in.CorrelationID,
	}
	created, err := s.repo.Create(ctx, command)
	if err != nil {
		return Command{}, err
	}
	s.bus.Publish(ctx, events.CommandIssued{
		At:        time.Now().UTC(),
		CommandID: created.ID,
		AssetID:   created.AssetID,
		MissionID: created.MissionID,
		Type:      created.Type,
		Actor:     in.Actor,
	})
	return created, nil
}

// Transition moves a command to a new lifecycle state and publishes
// command.status.changed.
func (s *Service) Transition(ctx context.Context, id string, to State, reason, actor string) (Command, error) {
	if !validState(to) {
		return Command{}, apperr.Validation("command state must be one of CREATED, QUEUED, SENT, ACKNOWLEDGED, COMPLETED, REJECTED, FAILED, TIMED_OUT, CANCELLED")
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return Command{}, err
	}
	if !CanTransition(current.State, to) {
		return Command{}, invalidTransition(current.State, to)
	}
	updated, err := s.repo.Transition(ctx, id, to, reason)
	if err != nil {
		return Command{}, err
	}
	s.bus.Publish(ctx, events.CommandStatusChanged{
		At:            time.Now().UTC(),
		CommandID:     updated.ID,
		AssetID:       updated.AssetID,
		State:         string(to),
		FailureReason: reason,
		Actor:         actor,
	})
	return updated, nil
}

// Acknowledge marks a sent command as acknowledged.
func (s *Service) Acknowledge(ctx context.Context, id, actor string) (Command, error) {
	return s.Transition(ctx, id, StateAcknowledged, "", actor)
}

// Complete marks an acknowledged command as completed.
func (s *Service) Complete(ctx context.Context, id, actor string) (Command, error) {
	return s.Transition(ctx, id, StateCompleted, "", actor)
}

// Reject records that an asset rejected a sent command.
func (s *Service) Reject(ctx context.Context, id, reason, actor string) (Command, error) {
	return s.Transition(ctx, id, StateRejected, reasonOr(reason, "rejected by asset"), actor)
}

// Fail records that command execution failed.
func (s *Service) Fail(ctx context.Context, id, reason, actor string) (Command, error) {
	return s.Transition(ctx, id, StateFailed, reasonOr(reason, "command execution failed"), actor)
}

// Timeout records that a command timed out awaiting a response.
func (s *Service) Timeout(ctx context.Context, id, reason, actor string) (Command, error) {
	return s.Transition(ctx, id, StateTimedOut, reasonOr(reason, "command timed out"), actor)
}

// Cancel cancels a command before it reaches a terminal outcome.
func (s *Service) Cancel(ctx context.Context, id, actor string) (Command, error) {
	return s.Transition(ctx, id, StateCancelled, "", actor)
}

// Get returns a command by id.
func (s *Service) Get(ctx context.Context, id string) (Command, error) {
	return s.repo.Get(ctx, id)
}

// List returns commands newest first.
func (s *Service) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Command, int, error) {
	return s.repo.List(ctx, filter, clampLimit(limit), max(offset, 0))
}

func reasonOr(reason, fallback string) string {
	if strings.TrimSpace(reason) == "" {
		return fallback
	}
	return reason
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
