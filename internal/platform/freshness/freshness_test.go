package freshness

import (
	"testing"
	"time"
)

func at(seconds int) *time.Time {
	value := time.Date(2026, 10, 13, 12, 0, seconds, 0, time.UTC)
	return &value
}

func TestForAssetAgeBands(t *testing.T) {
	policy := Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute}
	now := time.Date(2026, 10, 13, 12, 1, 0, 0, time.UTC)

	cases := []struct {
		name   string
		signal AssetSignal
		want   State
	}{
		{name: "fresh report is live", signal: AssetSignal{LastSeenAt: at(59), ConnectionState: "connected"}, want: StateLive},
		{name: "degraded link is still live", signal: AssetSignal{LastSeenAt: at(50), ConnectionState: "degraded"}, want: StateLive},
		{name: "age past delayed threshold", signal: AssetSignal{LastSeenAt: at(0), ConnectionState: "connected"}, want: StateDelayed},
		{name: "age past stale threshold", signal: AssetSignal{LastSeenAt: at(-290), ConnectionState: "connected"}, want: StateStale},
		{name: "age past unavailable threshold", signal: AssetSignal{LastSeenAt: at(-1801), ConnectionState: "connected"}, want: StateUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := policy.ForAsset(tc.signal, now); got != tc.want {
				t.Fatalf("ForAsset() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestForAssetDeclarationWinsOverAge(t *testing.T) {
	policy := Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute}
	now := time.Date(2026, 10, 13, 12, 1, 0, 0, time.UTC)
	fresh := at(59)

	cases := []struct {
		name   string
		signal AssetSignal
		want   State
	}{
		{name: "disconnected link beats fresh telemetry", signal: AssetSignal{LastSeenAt: fresh, ConnectionState: "disconnected"}, want: StateUnavailable},
		{name: "offline status beats fresh telemetry", signal: AssetSignal{LastSeenAt: fresh, ConnectionState: "connected", Status: "offline"}, want: StateUnavailable},
		{name: "unavailable status beats fresh telemetry", signal: AssetSignal{LastSeenAt: fresh, ConnectionState: "connected", Status: "unavailable"}, want: StateUnavailable},
		{name: "maintenance is not unavailable", signal: AssetSignal{LastSeenAt: fresh, ConnectionState: "connected", Status: "maintenance"}, want: StateLive},
		{name: "unknown connection state is judged on age", signal: AssetSignal{LastSeenAt: fresh, ConnectionState: "unknown"}, want: StateLive},
		{name: "case and padding are tolerated", signal: AssetSignal{LastSeenAt: fresh, ConnectionState: " Connected ", Status: " Assigned "}, want: StateLive},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := policy.ForAsset(tc.signal, now); got != tc.want {
				t.Fatalf("ForAsset() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestForAssetNeverReportedIsUnknownNotStale(t *testing.T) {
	policy := DefaultPolicy()
	got := policy.ForAsset(AssetSignal{ConnectionState: "connected"}, time.Now().UTC())
	if got != StateUnknown {
		t.Fatalf("ForAsset() = %q, want %q", got, StateUnknown)
	}
}

func TestForTrack(t *testing.T) {
	policy := Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute}
	now := time.Date(2026, 10, 13, 12, 1, 0, 0, time.UTC)

	cases := []struct {
		name   string
		signal TrackSignal
		want   State
	}{
		{name: "fresh observation", signal: TrackSignal{LastSeenAt: *at(59), Status: "active"}, want: StateLive},
		{name: "aging track", signal: TrackSignal{LastSeenAt: *at(0), Status: "active"}, want: StateDelayed},
		{name: "old track", signal: TrackSignal{LastSeenAt: *at(-400), Status: "active"}, want: StateStale},
		{name: "very old track", signal: TrackSignal{LastSeenAt: *at(-2000), Status: "active"}, want: StateUnavailable},
		{name: "lost track is unavailable not stale", signal: TrackSignal{LastSeenAt: *at(59), Status: "lost"}, want: StateUnavailable},
		{name: "closed track is judged on age", signal: TrackSignal{LastSeenAt: *at(59), Status: "closed"}, want: StateLive},
		{name: "zero last seen is unknown", signal: TrackSignal{Status: "active"}, want: StateUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := policy.ForTrack(tc.signal, now); got != tc.want {
				t.Fatalf("ForTrack() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFutureTimestampDegradesRatherThanTrusts(t *testing.T) {
	policy := DefaultPolicy()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	future := now.Add(90 * time.Second)

	if got := policy.ForAsset(AssetSignal{LastSeenAt: &future, ConnectionState: "connected"}, now); got != StateDelayed {
		t.Fatalf("future asset timestamp = %q, want %q", got, StateDelayed)
	}
	if got := policy.ForTrack(TrackSignal{LastSeenAt: future, Status: "active"}, now); got != StateDelayed {
		t.Fatalf("future track timestamp = %q, want %q", got, StateDelayed)
	}
}

func TestNormalizedRepairsUnorderedPolicy(t *testing.T) {
	cases := []struct {
		name   string
		policy Policy
		want   Policy
	}{
		{
			name:   "missing stale threshold takes the default and keeps a valid delayed",
			policy: Policy{DelayedAfter: time.Minute},
			want:   Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
		},
		{
			// An entirely unset policy must match the documented defaults
			// exactly, so an in-process harness and an explicit deployment
			// classify freshness identically.
			name:   "entirely unset policy takes the documented defaults",
			policy: Policy{},
			want:   DefaultPolicy(),
		},
		{
			name:   "missing delayed threshold derives one below stale",
			policy: Policy{StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
			want:   Policy{DelayedAfter: 75 * time.Second, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
		},
		{
			name:   "delayed equal to stale is pulled below it",
			policy: Policy{DelayedAfter: 5 * time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
			want:   Policy{DelayedAfter: 75 * time.Second, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
		},
		{
			name:   "unavailable below stale is widened",
			policy: Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 2 * time.Minute},
			want:   Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
		},
		{
			name:   "already ordered policy is untouched",
			policy: Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
			want:   Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.Normalized(); got != tc.want {
				t.Fatalf("Normalized() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestEveryStateIsReachableAndDescribed(t *testing.T) {
	policy := Policy{DelayedAfter: time.Minute, StaleAfter: 5 * time.Minute, UnavailableAfter: 30 * time.Minute}
	now := time.Date(2026, 10, 13, 12, 1, 0, 0, time.UTC)

	reached := map[State]bool{}
	for _, signal := range []AssetSignal{
		{LastSeenAt: at(59), ConnectionState: "connected"},
		{LastSeenAt: at(0), ConnectionState: "connected"},
		{LastSeenAt: at(-400), ConnectionState: "connected"},
		{LastSeenAt: at(-2000), ConnectionState: "connected"},
		{ConnectionState: "connected"},
	} {
		reached[policy.ForAsset(signal, now)] = true
	}
	reached[policy.ForAsset(AssetSignal{LastSeenAt: at(59), ConnectionState: "disconnected"}, now)] = true

	for _, state := range AllStates() {
		if !reached[state] {
			t.Errorf("state %q is declared but not reachable from any asset signal", state)
		}
		if !state.Valid() {
			t.Errorf("state %q is declared but not valid", state)
		}
		if state.Label() == "" || state.Label() == "Unknown state" {
			t.Errorf("state %q has no operator-facing label", state)
		}
	}
}

func TestTrustworthy(t *testing.T) {
	trustworthy := map[State]bool{
		StateLive: true, StateDelayed: true,
		StateStale: false, StateUnavailable: false, StateUnknown: false,
	}
	for state, want := range trustworthy {
		if got := state.Trustworthy(); got != want {
			t.Errorf("%q.Trustworthy() = %v, want %v", state, got, want)
		}
	}
}

func TestAge(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	if Age(nil, now) != nil {
		t.Fatal("Age(nil) should be nil for an entity that never reported")
	}
	seen := now.Add(-90 * time.Second)
	age := Age(&seen, now)
	if age == nil || *age != 90*time.Second {
		t.Fatalf("Age() = %v, want 90s", age)
	}
}
