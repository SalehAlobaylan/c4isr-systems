package commands

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/observability"
	"go.opentelemetry.io/otel/attribute"
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
	if scopeErr := access.Require(ctx, "commands.issue"); scopeErr != nil {
		return Command{}, scopeErr
	}

	in.Normalize()
	owner := in.Actor
	if p, _, scoped := access.From(ctx); scoped {
		owner = p.ID
	}
	if in.RequestKey != "" {
		if len(in.RequestKey) > 160 {
			return Command{}, apperr.Validation("command request key exceeds 160 characters")
		}
		if finder, ok := s.repo.(interface {
			FindRequest(context.Context, string, string) (Command, error)
		}); ok {
			old, e := finder.FindRequest(ctx, owner, in.RequestKey)
			if e == nil {
				if !sameRequest(old, in) {
					return Command{}, apperr.Conflict("command request key reused with different task")
				}
				return old, nil
			}
			if !apperr.Is(e, apperr.CodeNotFound) {
				return Command{}, e
			}
		}
	}
	spanCtx, span := observability.StartSpan(ctx, "c4isr.command.issue",
		attribute.String("asset.id", in.AssetID),
		attribute.String("mission.id", in.MissionID),
		attribute.String("command.type", in.Type),
	)
	ctx = spanCtx
	var spanErr error
	defer func() { observability.EndSpan(span, spanErr) }()

	if err := in.Validate(); err != nil {
		spanErr = err
		return Command{}, err
	}
	exists, err := s.assets.Exists(ctx, in.AssetID)
	if err != nil {
		spanErr = err
		return Command{}, err
	}
	if !exists {
		err := apperr.Validation("unknown asset: " + in.AssetID)
		spanErr = err
		return Command{}, err
	}

	command := Command{
		RequestOwner:  owner,
		RequestKey:    in.RequestKey,
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
		spanErr = err
		return Command{}, err
	}
	if created.ID != command.ID {
		return created, nil
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
	if scopeErr := access.Require(ctx, "commands.transition"); scopeErr != nil {
		return Command{}, scopeErr
	}

	spanCtx, span := observability.StartSpan(ctx, "c4isr.command.transition",
		attribute.String("command.id", id),
		attribute.String("command.state", string(to)),
	)
	ctx = spanCtx
	var spanErr error
	defer func() { observability.EndSpan(span, spanErr) }()

	if native, ok := s.repo.(interface {
		Native(context.Context) (bool, error)
	}); ok {
		enabled, e := native.Native(ctx)
		if e != nil {
			return Command{}, e
		}
		p, _, _ := access.From(ctx)
		if enabled && p.ID != "internal:native-exercise-projection" && to != StateCancelled {
			return Command{}, apperr.Forbidden("native execution outcomes are runtime-owned")
		}
	}
	if !validState(to) {
		err := apperr.Validation("command state must be one of CREATED, QUEUED, SENT, ACKNOWLEDGED, COMPLETED, REJECTED, FAILED, TIMED_OUT, CANCELLED")
		spanErr = err
		return Command{}, err
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		spanErr = err
		return Command{}, err
	}
	if !CanTransition(current.State, to) {
		err := invalidTransition(current.State, to)
		spanErr = err
		return Command{}, err
	}
	updated, err := s.repo.Transition(ctx, id, current.State, to, reason)
	if err != nil {
		spanErr = err
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
	if scopeErr := access.Require(ctx, "commands.transition"); scopeErr != nil {
		return Command{}, scopeErr
	}

	return s.Transition(ctx, id, StateAcknowledged, "", actor)
}

// Complete marks an acknowledged command as completed.
func (s *Service) Complete(ctx context.Context, id, actor string) (Command, error) {
	if scopeErr := access.Require(ctx, "commands.transition"); scopeErr != nil {
		return Command{}, scopeErr
	}

	return s.Transition(ctx, id, StateCompleted, "", actor)
}

// Reject records that an asset rejected a sent command.
func (s *Service) Reject(ctx context.Context, id, reason, actor string) (Command, error) {
	if scopeErr := access.Require(ctx, "commands.transition"); scopeErr != nil {
		return Command{}, scopeErr
	}

	return s.Transition(ctx, id, StateRejected, reasonOr(reason, "rejected by asset"), actor)
}

// Fail records that command execution failed.
func (s *Service) Fail(ctx context.Context, id, reason, actor string) (Command, error) {
	if scopeErr := access.Require(ctx, "commands.transition"); scopeErr != nil {
		return Command{}, scopeErr
	}

	return s.Transition(ctx, id, StateFailed, reasonOr(reason, "command execution failed"), actor)
}

// Timeout records that a command timed out awaiting a response.
func (s *Service) Timeout(ctx context.Context, id, reason, actor string) (Command, error) {
	if scopeErr := access.Require(ctx, "commands.transition"); scopeErr != nil {
		return Command{}, scopeErr
	}

	return s.Transition(ctx, id, StateTimedOut, reasonOr(reason, "command timed out"), actor)
}

// Cancel cancels a command before it reaches a terminal outcome.
func (s *Service) Cancel(ctx context.Context, id, actor string) (Command, error) {
	if scopeErr := access.Require(ctx, "commands.transition"); scopeErr != nil {
		return Command{}, scopeErr
	}

	return s.Transition(ctx, id, StateCancelled, "", actor)
}

// Get returns a command by id.
func (s *Service) Get(ctx context.Context, id string) (Command, error) {
	if scopeErr := access.Require(ctx, "commands.read"); scopeErr != nil {
		return Command{}, scopeErr
	}

	return s.repo.Get(ctx, id)
}

// List returns commands newest first.
func (s *Service) List(ctx context.Context, filter ListFilter, limit, offset int) ([]Command, int, error) {
	if scopeErr := access.Require(ctx, "commands.read"); scopeErr != nil {
		return nil, 0, scopeErr
	}

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

func sameRequest(c Command, in IssueInput) bool {
	a, _ := json.Marshal(c.Payload)
	b, _ := json.Marshal(in.Payload)
	return c.AssetID == in.AssetID && c.MissionID == in.MissionID && c.IncidentID == in.IncidentID && c.Type == in.Type && string(a) == string(b)
}
