// Package access defines authenticated principals and resource authorization.
// HTTP headers select a resource; only server-issued grants authorize it.
package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type Partition string

const (
	PublicEvidence      Partition = "PUBLIC_EVIDENCE"
	ExerciseInput       Partition = "EXERCISE_INPUT"
	ExerciseObservation Partition = "EXERCISE_OBSERVATION"
	ExerciseTruth       Partition = "EXERCISE_TRUTH"
)

type Resource struct {
	WorkspaceID   string    `json:"workspaceId"`
	EnvironmentID string    `json:"environmentId"`
	Partition     Partition `json:"partition"`
	RunID         string    `json:"runId,omitempty"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,160}$`)

func (r Resource) Validate() error {
	if !identifier.MatchString(r.WorkspaceID) || !identifier.MatchString(r.EnvironmentID) {
		return apperr.Validation("invalid workspace or environment")
	}
	switch r.Partition {
	case PublicEvidence:
		if r.RunID != "" {
			return apperr.Validation("public evidence cannot be run-bound")
		}
	case ExerciseInput:
	case ExerciseObservation, ExerciseTruth:
		if r.RunID == "" {
			return apperr.Validation("exercise output requires a run")
		}
	default:
		return apperr.Validation("unknown data partition")
	}
	if r.RunID != "" && !identifier.MatchString(r.RunID) {
		return apperr.Validation("invalid run")
	}
	return nil
}

func (r Resource) ID() string {
	return r.WorkspaceID + "/" + r.EnvironmentID + "/" + string(r.Partition) + "/" + r.RunID
}
func LegacyResource() Resource { return Resource{"default", "local", ExerciseObservation, "legacy"} }

type Grant struct {
	Resource    Resource `json:"resource"`
	Permissions []string `json:"permissions"`
	SourceIDs   []string `json:"sourceIds,omitempty"`
}

type Principal struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// HumanID is the registered person, resolved by authentication. Different
	// principals/credentials belonging to that person do not establish separation.
	HumanID      string    `json:"-"`
	CredentialID string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
	Grants       []Grant   `json:"-"`
}

type Denial struct{ PrincipalID, Permission, ScopeID, Reason string }
type Auditor func(context.Context, Denial)
type state struct {
	principal  Principal
	resource   Resource
	audit      Auditor
	projection bool
	create     func(context.Context, Resource) error
	activate   func(context.Context, Resource) error
	refresh    func(context.Context) (context.Context, error)
}
type key struct{}

func Bind(ctx context.Context, p Principal, r Resource, audit Auditor) (context.Context, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if p.ID == "" || (p.Kind != "human" && p.Kind != "service") {
		return nil, apperr.Unauthorized("authenticated principal required")
	}
	if !p.ExpiresAt.IsZero() && !time.Now().Before(p.ExpiresAt) {
		return nil, apperr.Unauthorized("credential expired")
	}
	st := state{principal: p, resource: r, audit: audit}
	st.principal.Grants = cloneGrants(p.Grants)
	if !st.matchesResource() {
		st.deny(ctx, "scope.select", "resource grant required")
		return nil, apperr.Forbidden("resource is not authorized")
	}
	if r.Partition == ExerciseTruth {
		granted := false
		for _, g := range st.matching() {
			for _, permission := range g.Permissions {
				if permission == "sim.truth.read" {
					granted = true
				}
			}
		}
		if !granted {
			st.deny(ctx, "sim.truth.read", "truth grant required")
			return nil, apperr.Forbidden("truth is not authorized")
		}
	}
	return context.WithValue(ctx, key{}, st), nil
}

func From(ctx context.Context) (Principal, Resource, bool) {
	st, ok := ctx.Value(key{}).(state)
	p := st.principal
	p.Grants = cloneGrants(p.Grants)
	return p, st.resource, ok
}

func cloneGrants(grants []Grant) []Grant {
	result := make([]Grant, len(grants))
	for i, g := range grants {
		result[i] = g
		result[i].Permissions = append([]string(nil), g.Permissions...)
		result[i].SourceIDs = append([]string(nil), g.SourceIDs...)
	}
	return result
}

func Refresh(ctx context.Context) (context.Context, error) {
	st, ok := ctx.Value(key{}).(state)
	if !ok {
		return nil, apperr.Unauthorized("resource context required")
	}
	if st.refresh != nil {
		fresh, err := st.refresh(ctx)
		if err != nil {
			return nil, err
		}
		// A freshly bound identity must still support the next live check. A
		// multi-stage read/worker must not silently fall back to cached grants
		// after its first refresh returns a new context.
		if next, ok := fresh.Value(key{}).(state); ok && next.refresh == nil {
			next.refresh = st.refresh
			fresh = context.WithValue(fresh, key{}, next)
		}
		return fresh, nil
	}
	if err := Require(ctx, "operational.read"); err != nil {
		return nil, err
	}
	return ctx, nil
}

func WithRefresh(ctx context.Context, refresh func(context.Context) (context.Context, error)) context.Context {
	st := ctx.Value(key{}).(state)
	st.refresh = refresh
	return context.WithValue(ctx, key{}, st)
}

// RefreshWithContext keeps the authenticated session's refresh operation, but
// bounds its database work by the current delivery context.
func RefreshWithContext(auth, work context.Context) (context.Context, error) {
	st, ok := auth.Value(key{}).(state)
	if !ok {
		return nil, apperr.Unauthorized("resource context required")
	}
	return Refresh(context.WithValue(work, key{}, st))
}

func ResourceFrom(ctx context.Context) (Resource, error) {
	_, r, ok := From(ctx)
	if !ok {
		return Resource{}, apperr.Unauthorized("resource context required")
	}
	return r, nil
}

func (s state) matching() []Grant {
	var out []Grant
	for _, g := range s.principal.Grants {
		r := g.Resource
		if r.WorkspaceID == s.resource.WorkspaceID && r.EnvironmentID == s.resource.EnvironmentID && r.Partition == s.resource.Partition && (r.RunID == s.resource.RunID || r.RunID == "*") {
			out = append(out, g)
		}
	}
	return out
}
func (s state) matchesResource() bool { return len(s.matching()) > 0 }
func (s state) deny(ctx context.Context, perm, reason string) {
	if s.audit != nil {
		s.audit(ctx, Denial{s.principal.ID, perm, s.resource.ID(), reason})
	}
}

func Require(ctx context.Context, permission string) error {
	st, ok := ctx.Value(key{}).(state)
	if !ok {
		return apperr.Unauthorized("authenticated resource context required")
	}
	if strings.HasPrefix(permission, "assets.") || strings.HasPrefix(permission, "telemetry.") || strings.HasPrefix(permission, "missions.") || strings.HasPrefix(permission, "commands.") || permission == "scenarios.control" {
		if st.resource.Partition != ExerciseObservation {
			st.deny(ctx, permission, "exercise operation required")
			return apperr.Forbidden("operation requires an exercise")
		}
	}
	if !st.principal.ExpiresAt.IsZero() && !time.Now().Before(st.principal.ExpiresAt) {
		st.deny(ctx, permission, "credential expired")
		return apperr.Unauthorized("credential expired")
	}
	if st.projection && KnownPermission(permission) && permission != "platform.maintenance" && permission != "sim.evaluation.read" && st.resource.Partition != ExerciseTruth && !strings.HasPrefix(permission, "admin.") && !strings.HasPrefix(permission, "sim.truth.") {
		return nil
	}
	for _, g := range st.matching() {
		for _, p := range g.Permissions {
			if p == permission {
				return nil
			}
		}
	}
	st.deny(ctx, permission, "permission not granted")
	return apperr.Forbidden("action is not authorized")
}

// Allows checks a capability without recording a denial for every filtered
// event. Explicit subscription failures are audited at the transport boundary.
func Allows(ctx context.Context, permission string) bool {
	st, ok := ctx.Value(key{}).(state)
	if !ok {
		return false
	}
	st.audit = nil
	return Require(context.WithValue(ctx, key{}, st), permission) == nil
}

func RequireSource(ctx context.Context, id string) error {
	st, ok := ctx.Value(key{}).(state)
	if !ok {
		return apperr.Unauthorized("resource context required")
	}
	if st.projection {
		return nil
	}
	for _, g := range st.matching() {
		if len(g.SourceIDs) == 0 {
			return nil
		}
		for _, source := range g.SourceIDs {
			if source == id {
				return nil
			}
		}
	}
	st.deny(ctx, "source.use", "source not granted")
	return apperr.Forbidden("source is not authorized")
}

// SourceIDs is the combined allowlist enforced by source row policies. A projection is
// trusted domain work in the same resource, never a new caller grant.
func SourceIDs(ctx context.Context) []string {
	st, ok := ctx.Value(key{}).(state)
	if !ok || st.projection {
		return nil
	}
	var ids []string
	for _, g := range st.matching() {
		if len(g.SourceIDs) == 0 {
			return nil
		}
		ids = append(ids, g.SourceIDs...)
	}
	return ids
}

// Projection is only used by the synchronous domain dispatcher after an
// authorized service has accepted input. It cannot change ownership or read truth.
func Projection(ctx context.Context) context.Context {
	st, ok := ctx.Value(key{}).(state)
	if !ok {
		return ctx
	}
	st.projection = true
	return context.WithValue(ctx, key{}, st)
}

// Internal creates explicitly scoped background authority, without truth access.
func Internal(ctx context.Context, r Resource, purpose string) context.Context {
	if r.Partition == ExerciseTruth {
		return context.WithValue(ctx, key{}, struct{}{})
	}
	p := Principal{ID: "internal:" + purpose, Kind: "service", Grants: []Grant{{Resource: r}}}
	var audit Auditor
	if old, ok := ctx.Value(key{}).(state); ok {
		audit = old.audit
	}
	bound, err := Bind(ctx, p, r, audit)
	if err != nil {
		return context.WithValue(ctx, key{}, struct{}{})
	}
	return Projection(bound)
}

func Rebind(ctx context.Context, r Resource) (context.Context, error) {
	st, ok := ctx.Value(key{}).(state)
	if !ok {
		return nil, apperr.Unauthorized("resource context required")
	}
	bound, err := Bind(ctx, st.principal, r, st.audit)
	if err != nil {
		return nil, err
	}
	return withLifecycle(bound, st.create, st.activate), nil
}

func withLifecycle(ctx context.Context, create, activate func(context.Context, Resource) error) context.Context {
	st := ctx.Value(key{}).(state)
	st.create = create
	st.activate = activate
	return context.WithValue(ctx, key{}, st)
}

func NewRun(ctx context.Context, id string) (context.Context, error) {
	if err := Require(ctx, "scenarios.control"); err != nil {
		return nil, err
	}
	r, err := ResourceFrom(ctx)
	if err != nil {
		return nil, err
	}
	if r.Partition != ExerciseObservation {
		return nil, apperr.Forbidden("runs require an exercise resource")
	}
	r.RunID = id
	bound, err := Rebind(ctx, r)
	if err != nil {
		return nil, err
	}
	st := bound.Value(key{}).(state)
	if st.create != nil {
		if err = st.create(ctx, r); err != nil {
			return nil, err
		}
	}
	return bound, nil
}

func Activate(ctx context.Context) error {
	st, ok := ctx.Value(key{}).(state)
	if ok && st.activate != nil {
		return st.activate(ctx, st.resource)
	}
	return nil
}

func Bootstrap(ctx context.Context) context.Context {
	r := LegacyResource()
	p := Principal{ID: "internal:bootstrap", Kind: "service", Grants: []Grant{{Resource: r, Permissions: []string{"admin.manage", "operators.read", "platform.maintenance"}}}}
	bound, _ := Bind(ctx, p, r, nil)
	return bound
}

func Deny(ctx context.Context, permission, reason string) {
	if st, ok := ctx.Value(key{}).(state); ok {
		st.deny(ctx, permission, reason)
	}
}

var Permissions = []string{
	"imagery.capture.execute",
	"sim.recordings.read", "sim.recordings.manage", "sim.recordings.execute", "sim.evaluation.release", "exercise.ingest", "sim.truth.record", "sim.assets.import", "sim.runtime.control", "sim.runtime.execute",
	"connectors.execute", "assessments.review", "context.export", "imagery.jobs.submit", "imagery.jobs.execute",
	"observations.process", "tracks.project", "alerts.project", "geofences.evaluate", "audit.append",
	"operational.read", "sources.read", "sources.manage", "observations.read", "observations.ingest", "assets.read", "assets.manage", "telemetry.read", "telemetry.ingest",
	"tracks.read", "classifications.read", "classifications.create", "geofences.read", "geofences.manage", "geospatial.read", "alerts.read", "alerts.acknowledge", "alerts.resolve",
	"incidents.read", "incidents.create", "incidents.update", "missions.read", "missions.create", "missions.update", "commands.read", "commands.issue", "commands.transition",
	"assessments.read", "assessments.create", "audit.read", "scenarios.read", "scenarios.control", "sim.scenarios.author", "sim.truth.read", "sim.truth.grant", "sim.evaluation.read", "operators.read", "admin.manage", "platform.maintenance",
}

func KnownPermission(permission string) bool {
	for _, p := range Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

func EffectivePermissions(ctx context.Context) []string {
	st, ok := ctx.Value(key{}).(state)
	if !ok {
		return []string{}
	}
	seen := map[string]bool{}
	result := []string{}
	for _, g := range st.matching() {
		for _, p := range g.Permissions {
			if !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}

// ResourceID scopes caller-supplied identifiers while preserving legacy and
// existing run-qualified IDs. Returned IDs remain the canonical API identifiers.
func ResourceID(ctx context.Context, kind, id string) string {
	_, r, ok := From(ctx)
	id = strings.TrimSpace(id)
	if !ok || id == "" || r == LegacyResource() || (r.RunID != "" && strings.HasPrefix(id, r.RunID+"__"+kind+"__")) {
		return id
	}
	h := sha256.Sum256([]byte(r.ID()))
	prefix := "scoped_" + hex.EncodeToString(h[:12]) + "__" + kind + "__"
	if strings.HasPrefix(id, prefix) {
		return id
	}
	return prefix + id
}
