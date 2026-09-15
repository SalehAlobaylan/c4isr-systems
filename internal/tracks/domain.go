// Package tracks implements the interpretation layer: tracks are operational
// interpretations built from observations. Observations remain primary
// evidence, and a track is something observed, never controlled.
package tracks

import (
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// Status describes a track's lifecycle state.
type Status string

const (
	StatusActive Status = "active"
	StatusLost   Status = "lost"
	StatusClosed Status = "closed"
)

// Track is an operational interpretation built from observations.
//
// ObservationCount and SourceIDs are read-model extras. Get populates them;
// list queries leave them zero to keep reads cheap.
type Track struct {
	ID               string
	ExternalRef      string
	Status           Status
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	Position         *geo.Point
	Speed            *float64
	Heading          *float64
	Metadata         map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ClosedAt         *time.Time
	ObservationCount int
	SourceIDs        []string
}

// HistoryPoint is one projected track state sample used for replay and path
// rendering.
type HistoryPoint struct {
	ID         string
	TrackID    string
	ObservedAt time.Time
	Position   *geo.Point
	Speed      *float64
	Heading    *float64
}

// StateUpdate is a projected change to a track's current state.
type StateUpdate struct {
	TrackID  string
	Position *geo.Point
	Speed    *float64
	Heading  *float64
}

// ObservationUpdate is the internal input to correlation. It carries the
// evidence fields a track needs without coupling the module to observation
// internals.
type ObservationUpdate struct {
	ObservationID string
	SourceID      string
	ObservedAt    time.Time
	Position      *geo.Point
	TrackHint     string
	Duplicate     bool
}

// SourceCount reports how many supporting observations came from one source.
type SourceCount struct {
	SourceID string
	Count    int
}
