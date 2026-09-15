// Package pgconv converts between generated database types and domain values,
// keeping pgtype imports out of domain packages.
package pgconv

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// JSONB marshals a value for a jsonb column, falling back to an empty object.
func JSONB(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

// JSONBArray marshals a value for a jsonb column that is an array.
func JSONBArray(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil || raw == nil {
		return []byte("[]")
	}
	return raw
}

// Map decodes a jsonb column into a map, returning an empty map for null.
func Map(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

// Slice decodes a jsonb column into a slice of values.
func Slice(raw []byte) []any {
	var out []any
	if len(raw) == 0 {
		return []any{}
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

// TS wraps a time for a timestamptz parameter.
func TS(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// Time unwraps a timestamptz result, returning the zero time when null.
func Time(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

// TimePtr unwraps a nullable timestamptz result.
func TimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// TextPtr returns a pointer for a non-empty string.
func TextPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Deref returns the value behind p or the zero value.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
