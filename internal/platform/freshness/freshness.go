// Package freshness classifies how much an operator should trust what is on
// the picture right now.
//
// Freshness is deliberately separated from epistemic uncertainty. Freshness is
// a question about time ("is this still current?"), while classification
// confidence answers "how sure are we what this is?". PRODUCT.md §25 requires
// the interface to distinguish live, delayed, stale, unavailable, and
// uncertain; this package owns the first four and leaves the fifth to the
// classification and assessment modules.
//
// The policy is a pure function of signals already present on every read, so a
// stale entity is never a stored value that can drift out of date. Thresholds
// are configuration, not constants buried in the interface, and the server is
// the only place they are evaluated: the browser renders what the API decided.
package freshness

import (
	"strings"
	"time"
)

// State is the operator-facing freshness classification of an entity.
type State string

const (
	// StateLive means the entity reported within the delayed threshold.
	StateLive State = "live"
	// StateDelayed means the entity is aging past expectations but the
	// picture is still within a plausible window.
	StateDelayed State = "delayed"
	// StateStale means the entity has not reported for longer than an
	// operator can reasonably assume its position still holds.
	StateStale State = "stale"
	// StateUnavailable means contact is lost by declaration (disconnected,
	// offline, or lost) or has been lost long enough that continuing to
	// render a position is misleading.
	StateUnavailable State = "unavailable"
	// StateUnknown means the entity exists but has never reported, so no
	// age can be computed. This is distinct from stale: nothing is known,
	// rather than something being known to be old.
	StateUnknown State = "unknown"
)

// AllStates lists every state in increasing order of trust. The UI legend and
// the API contract both derive from this list so they cannot drift.
func AllStates() []State {
	return []State{StateLive, StateDelayed, StateStale, StateUnavailable, StateUnknown}
}

// Valid reports whether s is one of the known states.
func (s State) Valid() bool {
	switch s {
	case StateLive, StateDelayed, StateStale, StateUnavailable, StateUnknown:
		return true
	}
	return false
}

// Label is short operator-facing text. It is used for badges and map legends
// where the state name alone is too technical.
func (s State) Label() string {
	switch s {
	case StateLive:
		return "Live"
	case StateDelayed:
		return "Delayed"
	case StateStale:
		return "Stale"
	case StateUnavailable:
		return "Unavailable"
	case StateUnknown:
		return "No data"
	default:
		return "Unknown state"
	}
}

// Trustworthy reports whether the entity's last known state can be treated as
// current. Anything other than live or delayed requires the operator to be
// told before acting on the picture.
func (s State) Trustworthy() bool {
	return s == StateLive || s == StateDelayed
}

// Policy holds the age thresholds that separate the states. A zero or
// unsorted Policy is normalized before use so a misconfigured deployment
// degrades predictably instead of producing unreachable states.
type Policy struct {
	DelayedAfter     time.Duration
	StaleAfter       time.Duration
	UnavailableAfter time.Duration
}

// DefaultPolicy matches the historical C4ISR_TELEMETRY_STALE_AFTER default so
// existing deployments keep their current behaviour and gain the finer bands
// around it.
func DefaultPolicy() Policy {
	return Policy{
		DelayedAfter:     time.Minute,
		StaleAfter:       5 * time.Minute,
		UnavailableAfter: 30 * time.Minute,
	}
}

// Normalized returns a policy with strictly increasing, non-zero thresholds.
// Ordering is repaired by clamping rather than rejecting, because freshness
// should never be the reason a server refuses to start.
func (p Policy) Normalized() Policy {
	// An entirely unset policy - an in-process harness that declared no
	// thresholds - must produce exactly the documented defaults. Repairing one
	// field at a time would derive a different delayed band from the stale one
	// and quietly disagree with a deployment that set the defaults explicitly.
	if p.DelayedAfter <= 0 && p.StaleAfter <= 0 && p.UnavailableAfter <= 0 {
		return DefaultPolicy()
	}
	if p.StaleAfter <= 0 {
		p.StaleAfter = DefaultPolicy().StaleAfter
	}
	if p.DelayedAfter <= 0 || p.DelayedAfter >= p.StaleAfter {
		p.DelayedAfter = p.StaleAfter / 4
	}
	if p.DelayedAfter <= 0 {
		p.DelayedAfter = time.Second
	}
	if p.UnavailableAfter <= p.StaleAfter {
		p.UnavailableAfter = p.StaleAfter * 6
	}
	return p
}

// AssetSignal is everything known about a controlled asset's freshness.
type AssetSignal struct {
	// LastSeenAt is the observation time of the newest telemetry applied to
	// the asset. Nil means the asset has never reported.
	LastSeenAt *time.Time
	// ConnectionState is the reported link state (connected, degraded,
	// disconnected, unknown).
	ConnectionState string
	// Status is the administrative availability (available, assigned,
	// unavailable, offline, maintenance).
	Status string
}

// TrackSignal is everything known about an observed entity's freshness.
// Tracks are never administratively unavailable, so only a lost track is.
type TrackSignal struct {
	LastSeenAt time.Time
	Status     string
}

// ForAsset classifies a controlled asset.
//
// Declaration wins over age: an operator who was told the link is down must not
// be shown a confident position. Age then widens the gap between "recent" and
// "we have effectively lost this".
func (p Policy) ForAsset(signal AssetSignal, now time.Time) State {
	switch normalize(signal.Status) {
	case "unavailable", "offline":
		return StateUnavailable
	}
	if normalize(signal.ConnectionState) == "disconnected" {
		return StateUnavailable
	}
	if signal.LastSeenAt == nil {
		return StateUnknown
	}
	return p.forAge(now.Sub(*signal.LastSeenAt))
}

// ForTrack classifies an observed entity. A lost track is unavailable rather
// than stale: the distinction is that we have declared the contact lost, not
// merely gone quiet.
func (p Policy) ForTrack(signal TrackSignal, now time.Time) State {
	if normalize(signal.Status) == "lost" {
		return StateUnavailable
	}
	if signal.LastSeenAt.IsZero() {
		return StateUnknown
	}
	return p.forAge(now.Sub(signal.LastSeenAt))
}

func (p Policy) forAge(age time.Duration) State {
	policy := p.Normalized()
	// A future-dated timestamp means a clock disagreement rather than
	// excellent freshness. Treating it as live would be the one lie the
	// operator cannot detect, so it degrades to delayed instead.
	if age < 0 {
		return StateDelayed
	}
	switch {
	case age >= policy.UnavailableAfter:
		return StateUnavailable
	case age >= policy.StaleAfter:
		return StateStale
	case age >= policy.DelayedAfter:
		return StateDelayed
	default:
		return StateLive
	}
}

// Age returns the time since an entity was last seen, or nil when it has never
// reported. Callers use it for "last seen 4m ago" text alongside the state.
func Age(lastSeenAt *time.Time, now time.Time) *time.Duration {
	if lastSeenAt == nil {
		return nil
	}
	age := now.Sub(*lastSeenAt)
	return &age
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
