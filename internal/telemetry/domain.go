// Package telemetry ingests normalized telemetry samples, preserves raw
// history, and projects the latest accepted state onto each asset.
package telemetry

import (
	"errors"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

const maxClockSkew = 5 * time.Minute

// ErrDuplicateMessage reports a repeated client-supplied message id. The
// original sample is preserved; the retry is acknowledged without side effects.
var ErrDuplicateMessage = errors.New("duplicate telemetry message id")

// Sample is one normalized telemetry record.
type Sample struct {
	ID              string
	MessageID       string
	AssetID         string
	SourceID        string
	ObservedAt      time.Time
	ReceivedAt      time.Time
	Position        *geo.Point
	Speed           *float64
	Heading         *float64
	Health          string
	ConnectionState string
	Payload         map[string]any
	CreatedAt       time.Time
}

// CreateInput is the application input for telemetry ingestion.
type CreateInput struct {
	// ID is an optional client-supplied idempotency key for the sample itself.
	// MessageID is the transport-level deduplication key.
	ID              string
	MessageID       string
	AssetID         string
	SourceID        string
	ObservedAt      time.Time
	ReceivedAt      time.Time
	Position        *geo.Point
	Speed           *float64
	Heading         *float64
	Health          string
	ConnectionState string
	Payload         map[string]any
}

// Normalize trims identifiers and applies defaults.
func (in *CreateInput) Normalize() {
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		in.ID = ids.New("tel")
	}
	in.MessageID = strings.TrimSpace(in.MessageID)
	in.AssetID = strings.TrimSpace(in.AssetID)
	in.SourceID = strings.TrimSpace(in.SourceID)
	in.Health = strings.TrimSpace(in.Health)
	in.ConnectionState = strings.TrimSpace(in.ConnectionState)
	if in.ReceivedAt.IsZero() {
		in.ReceivedAt = time.Now().UTC()
	}
	if in.Payload == nil {
		in.Payload = map[string]any{}
	}
	if !in.ObservedAt.IsZero() {
		in.ObservedAt = in.ObservedAt.UTC()
	}
}

// Validate checks the sample against domain rules at time now. A sample may
// arrive late, but not from the future beyond a small clock skew allowance.
func (in CreateInput) Validate(now time.Time) error {
	if in.AssetID == "" {
		return apperr.Validation("assetId is required")
	}
	if in.ObservedAt.IsZero() {
		return apperr.Validation("observedAt is required")
	}
	if in.ObservedAt.After(now.Add(maxClockSkew)) {
		return apperr.Validation("observedAt is too far in the future")
	}
	if in.Position != nil && !in.Position.Valid() {
		return apperr.Validation("position must be a valid WGS84 coordinate")
	}
	if in.ConnectionState != "" && !validConnectionState(in.ConnectionState) {
		return apperr.Validation("connectionState must be one of connected, degraded, disconnected, unknown")
	}
	return nil
}

// StateSnapshot is the current projected state of an asset.
type StateSnapshot struct {
	AssetID         string
	Position        *geo.Point
	ConnectionState string
	LastSeenAt      *time.Time
}

// StateUpdate is a state projection change derived from a sample.
type StateUpdate struct {
	AssetID         string
	Position        *geo.Point
	Speed           *float64
	Heading         *float64
	Health          string
	ConnectionState string
	ObservedAt      time.Time
}

func validConnectionState(state string) bool {
	switch state {
	case "connected", "degraded", "disconnected", "unknown":
		return true
	}
	return false
}
