// Package picture reconstructs the operational picture as it was at a point in
// time.
//
// The product's real advantage is deterministic replay: most competing systems
// cannot reproduce an incident exactly. That advantage only exists if the
// historical picture is derived from the same evidence the live picture was
// built from, so this package reads track_history and asset_telemetry - the
// append-oriented records - and never the current-state projections, which have
// already moved on.
//
// Two properties are deliberate and are what make replay trustworthy:
//
//   - A reconstructed entity is one that existed at the requested time. A track
//     with no history at or before the cut is omitted rather than shown at a
//     stale position, because showing it would invent a presence.
//   - Freshness is re-derived at the requested time. Replaying "T+23s" must not
//     paint a breach-age track in today's colours, or the operator is looking at
//     a picture that mixes two moments.
package picture

import (
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// Track is one observed entity as it was at the requested time.
type Track struct {
	ID          string
	ExternalRef string
	Status      string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	ObservedAt  time.Time
	Position    *geo.Point
	Speed       *float64
	Heading     *float64
	Freshness   freshness.State
}

// Asset is one controlled entity as it was at the requested time.
type Asset struct {
	ID              string
	Name            string
	Type            string
	Status          string
	ObservedAt      time.Time
	Position        *geo.Point
	Speed           *float64
	Heading         *float64
	ConnectionState string
	Health          string
	Freshness       freshness.State
}

// Geofence is a zone, unchanged by replay but included so the reconstructed
// picture is complete enough to judge a breach against.
type Geofence struct {
	ID       string
	Name     string
	Type     string
	Severity string
	Active   bool
	GeoJSON  string
}

// Alert is an alert that had already been raised at the requested time.
type Alert struct {
	ID              string
	Type            string
	Severity        string
	State           string
	Title           string
	Message         string
	SourceReference map[string]any
	TrackID         string
	AssetID         string
	GeofenceID      string
	CreatedAt       time.Time
}

// Window is the span of evidence a scrubber can traverse. The control is
// bounded by it, so the operator is never offered a moment the system cannot
// actually reconstruct.
type Window struct {
	Earliest time.Time
	Latest   time.Time
}

// Empty reports whether any evidence exists at all.
func (w Window) Empty() bool {
	return w.Earliest.IsZero() || w.Latest.IsZero() || w.Latest.Before(w.Earliest)
}

// Snapshot is the reconstructed picture at one instant.
type Snapshot struct {
	// At is the instant the picture represents.
	At time.Time
	// VirtualTimeMs is the scenario clock at that instant, when the evidence
	// came from a scenario run. Replay is usually discussed in scenario time
	// ("what did T+23s look like?"), so it is reported alongside wall clock.
	VirtualTimeMs *int64
	// Live reports whether this instant is the present moment. A scrubbed
	// picture is always historical; the interface says so rather than letting
	// the operator assume they are looking at now.
	Live            bool
	Tracks          []Track
	Assets          []Asset
	Alerts          []Alert
	Geofences       []Geofence
	Window          Window
	TruncatedAlerts bool
}

// Clamp returns the requested instant bounded to the available evidence. A cut
// outside the window cannot be reconstructed, and returning an invented picture
// would be worse than returning the nearest real one.
func (w Window) Clamp(at time.Time) time.Time {
	if w.Empty() {
		return at
	}
	if at.Before(w.Earliest) {
		return w.Earliest
	}
	if at.After(w.Latest) {
		return w.Latest
	}
	return at
}
