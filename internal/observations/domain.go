// Package observations implements the evidence layer: append-oriented
// statements made by sources about the world at particular times. Observations
// are preserved independently of any track, classification, or assessment
// derived from them.
package observations

import (
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

// maxClockSkew bounds how far in the future an observation may claim to have
// been made. Sensors with clock drift beyond this are rejected so that
// operational state cannot be polluted by bogus future timestamps.
const maxClockSkew = 5 * time.Minute

// Observation is a timestamped statement from a source.
type Observation struct {
	ID          string
	SourceID    string
	Type        string
	ObservedAt  time.Time
	ReceivedAt  time.Time
	ProcessedAt *time.Time
	Position    *geo.Point
	Payload     map[string]any
	Quality     map[string]any
	TrackHint   string
	CreatedAt   time.Time
}

// CreateInput is the application input for observation ingestion.
type CreateInput struct {
	// ID is an optional client-supplied idempotency key. Re-submitting the
	// same id returns the stored observation instead of creating a new one.
	ID         string
	SourceID   string
	Type       string
	ObservedAt time.Time
	ReceivedAt time.Time
	Position   *geo.Point
	Payload    map[string]any
	Quality    map[string]any
	TrackHint  string
}

// Normalize trims identifiers and applies defaults.
func (in *CreateInput) Normalize() {
	in.SourceID = trimmed(in.SourceID)
	in.Type = trimmed(in.Type)
	in.TrackHint = trimmed(in.TrackHint)
	if in.ID == "" {
		in.ID = ids.New("obs")
	}
	if in.Payload == nil {
		in.Payload = map[string]any{}
	}
	if in.ReceivedAt.IsZero() {
		in.ReceivedAt = time.Now().UTC()
	}
	if !in.ObservedAt.IsZero() {
		in.ObservedAt = in.ObservedAt.UTC()
	}
}

// Validate checks the observation against domain rules at time now.
func (in CreateInput) Validate(now time.Time) error {
	if in.SourceID == "" {
		return apperr.Validation("sourceId is required")
	}
	if in.Type == "" {
		return apperr.Validation("observation type is required")
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
	return nil
}

func trimmed(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
