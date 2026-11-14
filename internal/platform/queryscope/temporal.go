// Package queryscope defines clock semantics independently of query domains.
package queryscope

import (
	"context"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type Interval struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// MaxInterval includes one PostgreSQL timestamp quantum for imported inclusive
// legacy boundaries, so the resulting fixed scope can be resolved again.
const MaxInterval = 366*24*time.Hour + time.Microsecond

type Temporal struct {
	LegacyEndInclusive bool       `json:"legacyEndInclusive,omitempty"`
	Mode               string     `json:"mode"`
	Interval           *Interval  `json:"interval,omitempty"`
	RollingSeconds     int        `json:"rollingSeconds,omitempty"`
	RevisionView       string     `json:"revisionView"`
	AsOf               *time.Time `json:"asOf,omitempty"`
	Timezone           string     `json:"timezone"`
	BackgroundImagery  bool       `json:"backgroundImagery"`
}
type Evaluated struct {
	Temporal
	EvaluatedAt time.Time `json:"evaluatedAt"`
	Bounds      string    `json:"bounds"`
	Frozen      bool      `json:"frozen"`
}

func Evaluate(in Temporal, now time.Time) (Evaluated, error) {
	bad := func() (Evaluated, error) {
		return Evaluated{}, apperr.Validation("select monitoring with a bounded rolling window or a fixed historical interval, an explicit revision interpretation and IANA timezone; intervals are start-inclusive/end-exclusive")
	}
	now = now.UTC()
	if in.Mode == "" {
		in.Mode = "monitoring"
	}
	if in.RevisionView == "" {
		in.RevisionView = "latest"
	}
	if in.Timezone == "" {
		in.Timezone = "UTC"
	}
	if _, e := time.LoadLocation(in.Timezone); e != nil || len(in.Timezone) > 100 || !strings.Contains(in.Timezone, "/") && in.Timezone != "UTC" && in.Timezone != "GMT" {
		return bad()
	}
	if in.Mode != "monitoring" && in.Mode != "historical" || in.RevisionView != "latest" && in.RevisionView != "known_at" && in.RevisionView != "effective_at" {
		return bad()
	}
	if (in.RevisionView == "latest") != (in.AsOf == nil) || in.AsOf != nil && (in.AsOf.IsZero() || in.AsOf.After(now.Add(5*time.Minute))) {
		return bad()
	}
	if in.Mode == "monitoring" && (in.RevisionView != "latest" || in.AsOf != nil) {
		return bad()
	}
	if in.Interval != nil {
		v := *in.Interval
		v.From = v.From.UTC()
		v.To = v.To.UTC()
		maxInterval := MaxInterval
		if in.LegacyEndInclusive {
			maxInterval -= time.Microsecond
		}
		if in.RollingSeconds != 0 || v.From.IsZero() || v.To.Before(v.From) || v.To.Equal(v.From) && !in.LegacyEndInclusive || v.To.After(now.Add(5*time.Minute)) || v.To.Sub(v.From) > maxInterval {
			return bad()
		}
		if in.LegacyEndInclusive {
			// Legacy query parameters are encoded by pgx at microsecond
			// precision. Advance to the next representable timestamp so the
			// formerly inclusive endpoint survives the exclusive SQL adapter.
			v.To = v.To.Truncate(time.Microsecond).Add(time.Microsecond)
			in.LegacyEndInclusive = false
		}
		in.Interval = &v
	} else {
		if in.LegacyEndInclusive {
			return bad()
		}
		if in.Mode != "monitoring" {
			return bad()
		}
		if in.RollingSeconds == 0 {
			in.RollingSeconds = 86400
		}
		if in.RollingSeconds < 60 || in.RollingSeconds > 366*86400 {
			return bad()
		}
		in.Interval = &Interval{now.Add(-time.Duration(in.RollingSeconds) * time.Second), now}
	}
	// Preserve the preset separately in callers. An evaluated interval never rolls.
	in.RollingSeconds = 0
	if in.AsOf != nil {
		v := in.AsOf.UTC()
		in.AsOf = &v
	}
	return Evaluated{in, now, "[from,to)", true}, nil
}

type Support struct {
	State  string `json:"state"`
	Clock  string `json:"clock"`
	Reason string `json:"reason,omitempty"`
}

func Capability(family string, t Evaluated) Support {
	clock := "observation"
	if family == "imagery" || family == "scenes" || family == "captures" || family == "findings" {
		clock = "acquisition"
	}
	if family == "coverage" {
		clock = "admission_policy"
	}
	switch family {
	case "events", "earthquakes", "thermal":
		return Support{"supported", clock, ""}
	case "bulletins":
		if t.RevisionView == "effective_at" {
			return Support{"unsupported", clock, "Effective-time reconstruction is unavailable for provider reports."}
		}
		if t.Mode == "historical" || t.RevisionView == "known_at" || t.Interval.To.Before(t.EvaluatedAt.Add(-5*time.Minute)) {
			return Support{"supported", clock, "Historical results include only providers with an admitted historical report adapter. Gaps are unknown."}
		}
	case "weather", "aircraft", "vessels":
		if t.Mode == "historical" || t.RevisionView != "latest" || t.Interval.To.Before(t.EvaluatedAt.Add(-5*time.Minute)) {
			return Support{"unsupported", clock, "This family retains current context rather than a complete historical reconstruction. Exact authorized retained records remain readable."}
		}
	case "imagery", "scenes", "captures", "findings", "coverage":
		if t.RevisionView != "latest" {
			return Support{"unsupported", clock, "This family's catalog does not implement known-at/effective-at reconstruction. Select exact retained revisions separately."}
		}
	default:
		return Support{"unsupported", clock, "Unknown query family."}
	}
	return Support{"supported", clock, ""}
}

type key struct{}

type Resolution struct{ Geometry, Kind, ScopeHash, AuthorizationRevision string }

func Catalog(ctx context.Context, family string) (string, *time.Time, *time.Time, bool) {
	t, ok := From(ctx)
	if !ok {
		return "", nil, nil, false
	}
	if Capability(family, t).State == "unsupported" {
		return "", nil, nil, true
	}
	a, _ := Area(ctx)
	return a.Geometry, &t.Interval.From, &t.Interval.To, false
}
func ListMetadata(ctx context.Context, family string, body map[string]any) map[string]any {
	Metadata(ctx, body)
	if t, ok := From(ctx); ok {
		s := Capability(family, t)
		body["support"] = s
		if s.State == "unsupported" {
			body["state"] = "historical_unsupported"
		}
	}
	return body
}

type resolutionKey struct{}

func WithResolution(ctx context.Context, r Resolution) context.Context {
	return context.WithValue(ctx, resolutionKey{}, r)
}
func Area(ctx context.Context) (Resolution, bool) {
	v, ok := ctx.Value(resolutionKey{}).(Resolution)
	return v, ok
}
func Metadata(ctx context.Context, body map[string]any) map[string]any {
	if r, ok := Area(ctx); ok {
		body["scopeHash"] = r.ScopeHash
		if t, ok := From(ctx); ok {
			body["evaluatedAt"] = t.EvaluatedAt
			body["temporal"] = t
		}
	}
	return body
}

func With(ctx context.Context, t Evaluated) context.Context { return context.WithValue(ctx, key{}, t) }
func From(ctx context.Context) (Evaluated, bool)            { v, ok := ctx.Value(key{}).(Evaluated); return v, ok }
