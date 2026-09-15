// Package commands models instructions issued toward controlled assets. Every
// command carries an explicit, auditable lifecycle from issue to outcome.
package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// State is a command lifecycle state.
type State string

const (
	StateCreated      State = "CREATED"
	StateQueued       State = "QUEUED"
	StateSent         State = "SENT"
	StateAcknowledged State = "ACKNOWLEDGED"
	StateCompleted    State = "COMPLETED"
	StateRejected     State = "REJECTED"
	StateFailed       State = "FAILED"
	StateTimedOut     State = "TIMED_OUT"
	StateCancelled    State = "CANCELLED"
)

// Command is an instruction toward an asset.
type Command struct {
	ID             string
	AssetID        string
	MissionID      string
	IncidentID     string
	Type           string
	Payload        map[string]any
	State          State
	CreatedBy      string
	CorrelationID  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	QueuedAt       *time.Time
	SentAt         *time.Time
	AcknowledgedAt *time.Time
	CompletedAt    *time.Time
	FailureReason  string
}

// IssueInput is the application input for issuing a command.
type IssueInput struct {
	AssetID       string
	MissionID     string
	IncidentID    string
	Type          string
	Payload       map[string]any
	Actor         string
	CorrelationID string
}

// AssetRegistry verifies that command assets exist without importing the
// assets module.
type AssetRegistry interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// Normalize trims input fields and applies defaults.
func (in *IssueInput) Normalize() {
	in.AssetID = strings.TrimSpace(in.AssetID)
	in.MissionID = strings.TrimSpace(in.MissionID)
	in.IncidentID = strings.TrimSpace(in.IncidentID)
	in.Type = strings.TrimSpace(in.Type)
	in.Actor = strings.TrimSpace(in.Actor)
	in.CorrelationID = strings.TrimSpace(in.CorrelationID)
	if in.Payload == nil {
		in.Payload = map[string]any{}
	}
}

// Validate checks issue input against domain rules.
func (in IssueInput) Validate() error {
	if in.AssetID == "" {
		return apperr.Validation("assetId is required")
	}
	if in.Type == "" {
		return apperr.Validation("command type is required")
	}
	return nil
}

// CanTransition reports whether a command may move from one state to another.
func CanTransition(from, to State) bool {
	switch from {
	case StateCreated:
		return to == StateQueued || to == StateSent || to == StateCancelled
	case StateQueued:
		return to == StateSent || to == StateCancelled
	case StateSent:
		return to == StateAcknowledged || to == StateRejected || to == StateFailed ||
			to == StateTimedOut || to == StateCancelled
	case StateAcknowledged:
		return to == StateCompleted || to == StateFailed || to == StateCancelled
	}
	return false
}

// IsTerminal reports whether a command state has no outgoing transitions.
func IsTerminal(s State) bool {
	switch s {
	case StateCompleted, StateRejected, StateFailed, StateTimedOut, StateCancelled:
		return true
	}
	return false
}

func validState(s State) bool {
	switch s {
	case StateCreated, StateQueued, StateSent, StateAcknowledged, StateCompleted,
		StateRejected, StateFailed, StateTimedOut, StateCancelled:
		return true
	}
	return false
}

func invalidTransition(from, to State) error {
	return apperr.Conflict(fmt.Sprintf("invalid command transition %s -> %s", from, to))
}
