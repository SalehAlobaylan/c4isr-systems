// Package runctx carries scenario-run provenance through normal application
// services without coupling those services to the scenario package.
package runctx

import (
	"context"
	"strings"
)

type contextKey struct{}

// Scope identifies the isolated resource namespace that produced an event.
type Scope struct {
	RunID             string
	ResourceNamespace string
}

func (s Scope) ActorID() string {
	return "scenario:" + strings.TrimSpace(s.RunID)
}

// WithScope attaches scenario provenance to a request or background action.
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, contextKey{}, scope)
}

// ScopeFrom returns scenario provenance when the current operation belongs to
// a scenario run.
func ScopeFrom(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(contextKey{}).(Scope)
	return scope, ok && scope.RunID != ""
}
