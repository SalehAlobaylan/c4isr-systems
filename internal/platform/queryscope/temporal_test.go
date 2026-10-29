package queryscope

import (
	"testing"
	"time"
)

func TestEvaluateSharedClocksAndIntervals(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	v, e := Evaluate(Temporal{Mode: "monitoring", Timezone: "Asia/Riyadh"}, now)
	if e != nil || !v.Interval.From.Equal(now.Add(-24*time.Hour)) || !v.Interval.To.Equal(now) || v.Bounds != "[from,to)" || v.RollingSeconds != 0 {
		t.Fatalf("rolling evaluation: %+v %v", v, e)
	}
	fixed := v.Temporal
	again, e := Evaluate(fixed, now.Add(time.Hour))
	if e != nil || !again.Interval.To.Equal(now) {
		t.Fatal("fixed evaluation rolled", again, e)
	}
	at := now.Add(-time.Minute)
	for _, in := range []Temporal{{Mode: "historical"}, {Mode: "monitoring", RevisionView: "known_at", AsOf: &at}, {Mode: "historical", Interval: &Interval{now, now}}, {Mode: "monitoring", RollingSeconds: 30}, {Mode: "monitoring", Timezone: "Not/AZone"}, {Mode: "historical", Interval: v.Interval, RevisionView: "effective_at"}} {
		if _, e := Evaluate(in, now); e == nil {
			t.Fatalf("invalid period accepted: %+v", in)
		}
	}
	historical, e := Evaluate(Temporal{Mode: "historical", Interval: v.Interval, RevisionView: "known_at", AsOf: &at, Timezone: "America/New_York"}, now)
	if e != nil {
		t.Fatal(e)
	}
	if Capability("earthquakes", historical).State != "supported" || Capability("weather", historical).State != "unsupported" || Capability("imagery", historical).State != "unsupported" {
		t.Fatal("family history support lost")
	}
	if v.Interval.From.Year() != 2025 || v.Interval.To.Year() != 2026 {
		t.Fatal("year boundary changed")
	}
	legacy, e := Evaluate(Temporal{Mode: "historical", Interval: v.Interval, RevisionView: "latest", LegacyEndInclusive: true}, now)
	if e != nil || !legacy.Interval.To.Equal(now.Add(time.Microsecond)) || legacy.LegacyEndInclusive {
		t.Fatal("legacy inclusive adapter", legacy, e)
	}
	instant, e := Evaluate(Temporal{Mode: "historical", Interval: &Interval{now, now}, LegacyEndInclusive: true}, now)
	if e != nil || !instant.Interval.To.Equal(now.Add(time.Microsecond)) {
		t.Fatal("legacy single timestamp interval", instant, e)
	}
	longest, e := Evaluate(Temporal{Mode: "historical", Interval: &Interval{now.Add(-366 * 24 * time.Hour), now}, LegacyEndInclusive: true}, now)
	if e != nil {
		t.Fatal("legacy maximum interval", e)
	}
	if _, e = Evaluate(longest.Temporal, now); e != nil {
		t.Fatal("adapted maximum interval could not be resolved again", e)
	}
	// Offsets identify instants; preference affects display, never evidence time.
	offset, _ := time.Parse(time.RFC3339, "2026-01-01T02:59:00+03:00")
	h, e := Evaluate(Temporal{Mode: "historical", Interval: &Interval{offset, now}, RevisionView: "latest", Timezone: "UTC"}, now)
	if e != nil || !h.Interval.From.Equal(now.Add(-time.Minute)) {
		t.Fatal("offset changed query instant", h, e)
	}
}
