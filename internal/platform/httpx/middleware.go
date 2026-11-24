package httpx

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/go-chi/chi/v5"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	operatorIDKey
	operatorRoleKey
	operatorNameKey
	traceIDKey
)

// RequestIDHeader is the header carrying the request correlation id.
const RequestIDHeader = "X-Request-ID"

// OperatorHeader carries a legacy acting operator id. It is honored only when
// bearer authentication is explicitly disabled for a local harness.
const OperatorHeader = "X-Operator-ID"

// AuthorizationHeader is the standard bearer token header.
const AuthorizationHeader = "Authorization"

// TraceHeader carries the platform's request trace/correlation id.
const TraceHeader = "X-Trace-ID"

// Identity is the authenticated operator available to handlers and audit
// attribution. Role is resolved from the operator record, never from a
// client-supplied header.
type Identity struct {
	ID   string
	Name string
	Role string
}

// OperatorLookup resolves a configured token's subject to its current
// operator record.
type OperatorLookup func(contextT, string) (Identity, error)

// AuthOptions configures the first-party bearer-token authenticator.
type AuthOptions struct {
	Resolve         func(contextT, *http.Request) (contextT, Identity, error)
	Required        bool
	Tokens          map[string]string // token -> operator id
	DefaultOperator string
	Lookup          OperatorLookup
	OnFailure       func()
}

// IsPublicRequest identifies the small set of requests that must be served
// before an operator identity exists. The authorization middleware uses the
// same predicate as the authenticator so public probes and CORS preflight do
// not accidentally become role-protected.
func IsPublicRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	return r.Method == http.MethodOptions || r.URL.Path == "/health" || r.URL.Path == "/metrics"
}

// RequestMetrics is implemented by the platform metrics registry. Keeping a
// tiny interface here prevents HTTP middleware from depending on its concrete
// exporter.
type RequestMetrics interface {
	ObserveRequest(method, path string, status, bytes int, duration time.Duration)
}

// RequestID ensures every request has a correlation id available in context
// and echoed to the client.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = ids.New("req")
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := withRequestID(r.Context(), id)
		traceID := strings.TrimSpace(r.Header.Get(TraceHeader))
		if traceID == "" {
			traceID = id
		}
		w.Header().Set(TraceHeader, traceID)
		ctx = withTraceID(ctx, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OperatorIdentity resolves the acting operator for audit attribution.
func OperatorIdentity(defaultOperator string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			operator := strings.TrimSpace(r.Header.Get(OperatorHeader))
			if operator == "" {
				operator = defaultOperator
			}
			ctx := withOperatorID(r.Context(), operator)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authenticate resolves a bearer token to an operator. WebSocket clients use
// the access_token query parameter because browser WebSocket constructors do
// not permit arbitrary request headers; it is accepted only during an upgrade.
func Authenticate(options AuthOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsPublicRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			if options.Resolve != nil {
				ctx, identity, err := options.Resolve(r.Context(), r)
				if err != nil {
					if options.OnFailure != nil {
						options.OnFailure()
					}
					Error(w, err)
					return
				}
				ctx = withOperatorID(ctx, identity.ID)
				ctx = withOperatorRole(ctx, identity.Role)
				ctx = withOperatorName(ctx, identity.Name)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			operatorID, ok := bearerSubject(r, options.Tokens)
			if !ok && !options.Required {
				operatorID = strings.TrimSpace(r.Header.Get(OperatorHeader))
				if operatorID == "" {
					operatorID = options.DefaultOperator
				}
				ok = operatorID != ""
			}
			if !ok {
				if options.OnFailure != nil {
					options.OnFailure()
				}
				w.Header().Set("WWW-Authenticate", `Bearer realm="c4isr"`)
				Error(w, apperr.Unauthorized("a valid bearer token is required"))
				return
			}

			identity := Identity{ID: operatorID, Role: "operator"}
			if options.Lookup != nil {
				resolved, err := options.Lookup(r.Context(), operatorID)
				if err != nil || resolved.ID == "" {
					if options.OnFailure != nil {
						options.OnFailure()
					}
					w.Header().Set("WWW-Authenticate", `Bearer realm="c4isr"`)
					Error(w, apperr.Unauthorized("operator identity is not registered"))
					return
				}
				identity = resolved
			}

			ctx := withOperatorID(r.Context(), identity.ID)
			ctx = withOperatorRole(ctx, identity.Role)
			ctx = withOperatorName(ctx, identity.Name)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerSubject(r *http.Request, tokens map[string]string) (string, bool) {
	raw := strings.TrimSpace(r.Header.Get(AuthorizationHeader))
	if raw == "" && strings.Contains(strings.ToLower(r.Header.Get("Upgrade")), "websocket") {
		raw = strings.TrimSpace(RealtimeCredential(r))
	} else {
		parts := strings.Fields(raw)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			raw = parts[1]
		} else {
			raw = ""
		}
	}
	if raw == "" {
		return "", false
	}
	for token, operatorID := range tokens {
		if subtle.ConstantTimeCompare([]byte(token), []byte(raw)) == 1 {
			return operatorID, true
		}
	}
	return "", false
}

// Authorization applies route-level RBAC after authentication. It maps the
// stable API surface to permissions instead of leaking authorization checks
// into individual domain handlers.
func Authorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsPublicRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if GetOperatorID(r.Context()) == "" {
			Error(w, apperr.Unauthorized("operator authentication is required"))
			return
		}
		policy := PolicyFor(r.Method, r.URL.Path)
		if _, resource, scoped := access.From(r.Context()); scoped {
			if resource.Partition != access.PublicEvidence && resource.Partition != access.ExerciseObservation && r.URL.Path != "/api/v1/auth/me" && !privilegedSimulationRoute(resource, r.Method, r.URL.Path, policy) {
				access.Deny(r.Context(), "cop.subscribe", "ordinary transport partition denied")
				Error(w, apperr.Forbidden("partition requires a separate privileged transport"))
				return
			}
			if !policy.Known {
				Error(w, apperr.Forbidden("unknown action"))
				return
			}
			if policy.Permission != "" {
				if err := access.Require(r.Context(), policy.Permission); err != nil {
					Error(w, err)
					return
				}
			}
			next.ServeHTTP(w, r)
			return
		}
		if policy.Known && (policy.Permission == "" || Allows(GetOperatorRole(r.Context()), policy.Permission)) {
			next.ServeHTTP(w, r)
			return
		}
		Error(w, apperr.Forbidden("operator role is not permitted to perform this action"))
	})
}

// RoutePolicy is the authorization contract for a known API route. A route
// may be known without an additional permission (currently only auth/me).
type RoutePolicy struct {
	Known      bool
	Permission string
}

// PolicyFor maps API methods and paths to stable permission identifiers. It
// intentionally returns unknown for paths and methods outside the mounted API
// surface so authorization fails closed before a handler can be reached.
func PolicyFor(method, rawPath string) RoutePolicy {
	if rawPath != "/api/v1" && !strings.HasPrefix(rawPath, "/api/v1/") {
		return RoutePolicy{}
	}
	path := strings.TrimPrefix(rawPath, "/api/v1")
	if path == "" {
		return RoutePolicy{}
	}
	if strings.Contains(path, "//") {
		return RoutePolicy{}
	}
	path = strings.TrimSuffix(path, "/")

	parts := pathSegments(path)
	if (path == "/imagery/capture-proposal" || path == "/imagery/comparison-check") && method == http.MethodPost {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if len(parts) == 4 && parts[0] == "analyst-scope" && parts[1] == "reads" && parts[3] == "records" && method == http.MethodGet {
		return RoutePolicy{Known: true, Permission: "observations.read"}
	}
	if len(parts) == 4 && parts[0] == "analyst-scope" && parts[1] == "reads" && parts[3] == "release" && method == http.MethodPost {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if path == "/catalog/reads" && method == http.MethodPost {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if method == http.MethodDelete && len(parts) == 3 && parts[0] == "catalog" && parts[1] == "reads" {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if permission := geographicEnvironmentPermission(method, parts); permission != "" {
		return RoutePolicy{Known: true, Permission: permission}
	}
	if len(parts) == 4 && parts[0] == "sources" && parts[2] == "scheduler" && parts[3] == "run" && method == http.MethodPost {
		return RoutePolicy{Known: true, Permission: "connectors.execute"}
	}
	if len(parts) == 3 && parts[0] == "sources" && parts[2] == "acquisition-metrics" && method == http.MethodGet {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if len(parts) >= 1 && parts[0] == "acquisitions" {
		if method == http.MethodGet && len(parts) <= 2 {
			return RoutePolicy{Known: true, Permission: "acquisitions.read"}
		}
		if method == http.MethodPost && (len(parts) == 1 || len(parts) == 3 && parts[2] == "cancel") {
			return RoutePolicy{Known: true, Permission: "acquisitions.request"}
		}
	}
	if path == "/coverage/query" && method == http.MethodPost {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if path == "/coverage/adapters" && method == http.MethodGet {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if path == "/coverage/inventory" && method == http.MethodGet {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if len(parts) >= 3 && parts[0] == "sources" && parts[2] == "profiles" {
		if method == http.MethodGet && len(parts) == 3 {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
		if method == http.MethodPost && (len(parts) == 4 || len(parts) == 5 && parts[4] == "execution") {
			return RoutePolicy{Known: true, Permission: "sources.manage"}
		}
		if method == http.MethodPost && len(parts) == 5 && parts[4] == "connector-runs" {
			return RoutePolicy{Known: true, Permission: "connectors.execute"}
		}
	}
	if len(parts) > 0 && parts[0] == "geography" {
		if method == http.MethodPost && len(parts) == 2 && parts[1] == "place-handoffs" {
			return RoutePolicy{Known: true, Permission: "context.export"}
		}
		if method == http.MethodGet && len(parts) == 2 && (parts[1] == "availability" || parts[1] == "releases") {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
		if len(parts) == 2 && parts[1] == "reference-requests" {
			if method == http.MethodGet {
				return RoutePolicy{Known: true, Permission: "sources.manage"}
			}
			if method == http.MethodPost {
				return RoutePolicy{Known: true, Permission: "geography.request"}
			}
		}
		if method == http.MethodPost && len(parts) == 4 && parts[1] == "reference-requests" && parts[3] == "resolve" {
			return RoutePolicy{Known: true, Permission: "sources.manage"}
		}
	}
	if len(parts) > 0 && parts[0] == "delivery-jobs" {
		if method == http.MethodGet && len(parts) == 2 {
			return RoutePolicy{Known: true, Permission: "outputs.read"}
		}
		if method == http.MethodPost && len(parts) == 3 && (parts[2] == "cancel" || parts[2] == "retry") {
			return RoutePolicy{Known: true, Permission: "outputs.create"}
		}
	}
	if len(parts) > 0 && parts[0] == "outputs" {
		if method == http.MethodGet && (len(parts) == 2 || len(parts) == 3 && (parts[2] == "content" || parts[2] == "bundle" || parts[2] == "attestation")) {
			return RoutePolicy{Known: true, Permission: "outputs.read"}
		}
		if len(parts) == 3 && (method == http.MethodPut && parts[2] == "content" || method == http.MethodPost && parts[2] == "expire") {
			return RoutePolicy{Known: true, Permission: "outputs.create"}
		}
	}
	if len(parts) > 0 && parts[0] == "investigations" {
		if len(parts) == 3 && parts[2] == "delivery-jobs" {
			if method == http.MethodGet {
				return RoutePolicy{Known: true, Permission: "outputs.read"}
			}
			if method == http.MethodPost {
				return RoutePolicy{Known: true, Permission: "outputs.create"}
			}
		}

		if len(parts) == 3 && method == http.MethodGet && (parts[2] == "history" || parts[2] == "reviews") {
			return RoutePolicy{Known: true, Permission: "investigations.read"}
		}
		if len(parts) == 3 && method == http.MethodPost && parts[2] == "review" {
			return RoutePolicy{Known: true, Permission: "investigations.read"}
		}
		if len(parts) == 3 && method == http.MethodPost && parts[2] == "outputs" {
			return RoutePolicy{Known: true, Permission: "outputs.create"}
		}

		if method == http.MethodGet && len(parts) <= 2 {
			return RoutePolicy{Known: true, Permission: "investigations.read"}
		}
		if method == http.MethodPost && len(parts) == 1 {
			return RoutePolicy{Known: true, Permission: "investigations.create"}
		}
		if method == http.MethodPost && len(parts) == 3 && parts[2] == "revisions" {
			return RoutePolicy{Known: true, Permission: "investigations.edit"}
		}
		if method == http.MethodPost && len(parts) == 3 && parts[2] == "members" {
			return RoutePolicy{Known: true, Permission: "investigations.manage"}
		}
	}
	if len(parts) >= 2 && parts[0] == "areas" {
		if method == http.MethodPost && len(parts) == 2 && (parts[1] == "resolve" || parts[1] == "revisions") || method == http.MethodGet && len(parts) == 3 && parts[1] == "revisions" {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
	}
	if len(parts) >= 2 && parts[0] == "analyst-scope" {
		if method == http.MethodPost && len(parts) == 2 && (parts[1] == "resolve" || parts[1] == "freeze" || parts[1] == "read") || method == http.MethodGet && len(parts) == 3 && (parts[1] == "reads" || parts[1] == "snapshots") {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
	}
	if len(parts) == 3 && parts[0] == "geography" && (parts[1] == "releases" || parts[1] == "profiles") && method == http.MethodGet {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if len(parts) == 2 && parts[0] == "geography" {
		if method == http.MethodGet && parts[1] == "regions" {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
		if method == http.MethodPost && (parts[1] == "editions" || parts[1] == "releases" || parts[1] == "profiles") {
			return RoutePolicy{Known: true, Permission: "sources.manage"}
		}
	}
	if len(parts) > 0 && parts[0] == "map" {
		// Each feature family enforces its own read permission inside the handler.
		if len(parts) == 2 && parts[1] == "features" && method == http.MethodGet {
			return RoutePolicy{Known: true}
		}
		if len(parts) == 2 && (parts[1] == "measure" || parts[1] == "surrounding-area") && method == http.MethodPost {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
		if len(parts) == 5 && parts[1] == "reference" && strings.HasSuffix(parts[4], ".pbf") && method == http.MethodGet {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
	}
	if len(parts) >= 2 && parts[0] == "exercise-view" && method == http.MethodGet && (len(parts) == 2 || len(parts) == 3 && parts[2] == "content") {
		return RoutePolicy{Known: true, Permission: "observations.read"}
	}
	if len(parts) >= 2 && parts[0] == "imagery" && parts[1] == "captures" {
		if method == http.MethodGet && (len(parts) == 2 || len(parts) == 3) {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
		if method == http.MethodPost && ((len(parts) == 3 && parts[2] == "claim") || (len(parts) == 4 && parts[3] == "progress")) {
			return RoutePolicy{Known: true, Permission: "imagery.capture.execute"}
		}
		if method == http.MethodPost && (len(parts) == 2 || (len(parts) == 4 && (parts[3] == "retry" || parts[3] == "cancel"))) {
			return RoutePolicy{Known: true, Permission: "imagery.jobs.submit"}
		}
		return RoutePolicy{}
	}
	if len(parts) == 1 && parts[0] == "reference-features" && method == http.MethodGet {
		return RoutePolicy{Known: true, Permission: "sources.read"}
	}
	if len(parts) == 2 && parts[0] == "simulation-runtime" && method == http.MethodGet {
		if parts[1] == "queue" {
			return RoutePolicy{Known: true, Permission: "sim.runtime.execute"}
		}
		if parts[1] == "capabilities" {
			return RoutePolicy{Known: true, Permission: "sim.runtime.control"}
		}
	}
	if len(parts) == 2 && parts[0] == "platform" && (method == http.MethodGet && parts[1] == "readiness" || method == http.MethodPost && parts[1] == "worker-heartbeats") {
		if method == http.MethodPost {
			return RoutePolicy{Known: true}
		}
		return RoutePolicy{Known: true, Permission: "operational.read"}
	}
	if len(parts) > 0 && parts[0] == "simulation-recordings" {
		if method == http.MethodGet && len(parts) == 2 {
			return RoutePolicy{Known: true, Permission: "sim.recordings.read"}
		}
		if len(parts) == 3 {
			if method == http.MethodPost && parts[2] == "archive" {
				return RoutePolicy{Known: true, Permission: "sim.recordings.manage"}
			}
			if method == http.MethodPost && parts[2] == "release" {
				return RoutePolicy{Known: true, Permission: "sim.evaluation.release"}
			}
			if method == http.MethodGet {
				switch parts[2] {
				case "events", "frame", "checkpoint", "report":
					return RoutePolicy{Known: true, Permission: "sim.recordings.read"}
				case "evaluation", "spatial":
					return RoutePolicy{Known: true, Permission: "sim.evaluation.read"}
				}
			}
		}
		return RoutePolicy{}
	}
	if len(parts) > 0 && parts[0] == "simulation-studio" {
		if len(parts) == 2 && parts[1] == "context-packages" && method == http.MethodGet || len(parts) == 2 && (parts[1] == "scenes" || parts[1] == "sensor-presets") && (method == http.MethodGet || method == http.MethodPost) || len(parts) == 3 && (parts[1] == "scenes" || parts[1] == "sensor-presets") && method == http.MethodGet {
			return RoutePolicy{Known: true, Permission: "sim.scenarios.author"}
		}
		return RoutePolicy{}
	}
	if len(parts) > 1 && parts[0] == "simulation-runtime" && parts[1] == "runs" {
		if method == http.MethodGet && (len(parts) == 2 || len(parts) == 3) || method == http.MethodPost && len(parts) == 2 {
			return RoutePolicy{Known: true, Permission: "sim.runtime.control"}
		}
		if len(parts) == 4 {
			if method == http.MethodGet && parts[3] == "truth" {
				return RoutePolicy{Known: true, Permission: "sim.truth.read"}
			}
			if method == http.MethodPost {
				switch parts[3] {
				case "pause", "resume", "stop":
					return RoutePolicy{Known: true, Permission: "sim.runtime.control"}
				case "attach", "heartbeat":
					return RoutePolicy{Known: true, Permission: "sim.runtime.execute"}
				}
			}
		}
		return RoutePolicy{}
	}
	if len(parts) > 0 && parts[0] == "simulation-assets" {
		if len(parts) >= 2 {
			family := parts[1]
			known := false
			if method == http.MethodGet {
				known = (len(parts) == 2 && (family == "uploads" || family == "models" || family == "templates" || family == "environments")) || (len(parts) == 3 && (family == "uploads" || family == "models" || family == "templates" || family == "environments")) || (len(parts) == 4 && (family == "uploads" && (parts[3] == "content" || parts[3] == "attempts") || family == "models" && parts[3] == "manifest"))
			}
			if method == http.MethodPost {
				known = len(parts) == 2 && (family == "uploads" || family == "templates" || family == "environments")
			}
			if method == http.MethodPut {
				known = len(parts) == 4 && family == "uploads" && parts[3] == "content"
			}
			if known {
				return RoutePolicy{Known: true, Permission: "sim.scenarios.author"}
			}
		}
		return RoutePolicy{}
	}
	if len(parts) > 0 && parts[0] == "artifacts" {
		if method == http.MethodGet && (len(parts) == 2 || (len(parts) == 3 && parts[2] == "content")) {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
		if (method == http.MethodPost && len(parts) == 1) || (method == http.MethodPut && len(parts) == 3 && parts[2] == "content") || (method == http.MethodDelete && len(parts) == 2) {
			return RoutePolicy{Known: true, Permission: "sources.manage"}
		}
		return RoutePolicy{}
	}
	if len(parts) > 0 && parts[0] == "imagery" {
		if method == http.MethodGet && len(parts) >= 2 && (((parts[1] == "scenes" || parts[1] == "sensor-presets") && (len(parts) == 2 || len(parts) == 3)) || (parts[1] == "jobs" && (len(parts) == 2 || len(parts) == 3 || (len(parts) == 4 && (parts[3] == "attempts" || parts[3] == "result"))))) {
			return RoutePolicy{Known: true, Permission: "sources.read"}
		}
		if method == http.MethodPost && ((len(parts) == 2 && parts[1] == "jobs") || (len(parts) == 4 && parts[1] == "jobs" && (parts[3] == "cancel" || parts[3] == "retry"))) {
			return RoutePolicy{Known: true, Permission: "imagery.jobs.submit"}
		}
		return RoutePolicy{}
	}
	if len(parts) > 0 && parts[0] == "assessments" {
		if method == http.MethodGet && ((len(parts) == 2 && (parts[1] == "intelligence" || parts[1] == "reports")) || ((len(parts) == 3 || len(parts) == 4) && parts[2] == "revisions")) {
			return RoutePolicy{Known: true, Permission: "assessments.read"}
		}
		if method == http.MethodPost && ((len(parts) == 2 && (parts[1] == "imagery-proposals" || parts[1] == "report-proposals")) || (len(parts) == 3 && (parts[2] == "revisions" || parts[2] == "submit"))) {
			return RoutePolicy{Known: true, Permission: "assessments.create"}
		}
		if method == http.MethodPost && len(parts) == 3 && parts[2] == "review" {
			return RoutePolicy{Known: true, Permission: "assessments.review"}
		}
	}
	if path == "/content-purpose" {
		return RoutePolicy{Known: method == http.MethodGet || method == http.MethodPost, Permission: "sources.manage"}
	}
	if path == "/auth/me" {
		return RoutePolicy{Known: method == http.MethodGet}
	}
	if path == "/realtime" {
		if method == http.MethodGet {
			return RoutePolicy{Known: true, Permission: "operational.read"}
		}
		return RoutePolicy{}
	}

	if method == http.MethodGet {
		segments := pathSegments(path)
		if len(segments) == 0 {
			return RoutePolicy{}
		}
		switch segments[0] {
		case "audit":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "audit.read"}
			}
		case "geospatial":
			if validGeospatialGetPath(segments) {
				return RoutePolicy{Known: true, Permission: "geospatial.read"}
			}
		case "operators":
			if len(segments) == 1 || len(segments) == 2 {
				return RoutePolicy{Known: true, Permission: "operators.read"}
			}
		case "picture":
			// Replay is a read of recorded evidence, never a new fact. It
			// carries the same permission as the live picture so scrubbing
			// cannot reveal anything an operator could not already see.
			if validPictureGetPath(segments) {
				return RoutePolicy{Known: true, Permission: "operational.read"}
			}
		case "scenarios":
			if len(segments) == 4 && segments[1] == "runs" && segments[3] == "synthetic-messages" {
				return RoutePolicy{Known: true, Permission: "observations.read"}
			}
			if len(segments) == 4 && segments[1] == "runs" && segments[3] == "version" {
				return RoutePolicy{Known: true, Permission: "sim.scenarios.author"}
			}
			if validScenarioGetPath(segments) {
				return RoutePolicy{Known: true, Permission: "scenarios.read"}
			}
		case "assets":
			if len(segments) == 1 || len(segments) == 2 {
				return RoutePolicy{Known: true, Permission: "assets.read"}
			}
			if len(segments) == 3 && segments[2] == "telemetry" {
				return RoutePolicy{Known: true, Permission: "telemetry.read"}
			}
		case "public-context":
			if len(segments) >= 2 && segments[1] == "records" && (len(segments) == 2 || len(segments) == 3 || len(segments) == 4 && segments[3] == "history") {
				return RoutePolicy{Known: true, Permission: "sources.read"}
			}
		case "external-tracks":
			if len(segments) == 1 || len(segments) == 2 || (len(segments) == 3 && (segments[2] == "associations" || segments[2] == "history")) {
				return RoutePolicy{Known: true, Permission: "tracks.read"}
			}
		case "tracks":
			if len(segments) == 1 || len(segments) == 2 {
				return RoutePolicy{Known: true, Permission: "tracks.read"}
			}
			if len(segments) == 3 && (segments[2] == "history" || segments[2] == "classifications") {
				return RoutePolicy{Known: true, Permission: "tracks.read"}
			}
		case "context-packages":
			if method == http.MethodGet && len(segments) == 3 && segments[2] == "geographic" || method == http.MethodPost && len(segments) == 2 && segments[1] == "geographic" {
				return RoutePolicy{Known: true, Permission: "context.export"}
			}
			if method == http.MethodPost && len(segments) == 2 && segments[1] == "preview" {
				return RoutePolicy{Known: true, Permission: "context.export"}
			}
			if (method == http.MethodGet && (len(segments) == 1 || len(segments) == 2 || (len(segments) == 3 && segments[2] == "bundle"))) || (method == http.MethodPost && len(segments) == 1) {
				return RoutePolicy{Known: true, Permission: "context.export"}
			}
		case "reference-datasets":
			if len(segments) == 1 || len(segments) == 2 || (len(segments) == 3 && (segments[2] == "features" || segments[2] == "artifact")) {
				return RoutePolicy{Known: true, Permission: "sources.read"}
			}
		case "public-events":
			if method == http.MethodGet && (len(segments) == 1 || len(segments) == 2 || (len(segments) == 3 && segments[2] == "revisions")) {
				return RoutePolicy{Known: true, Permission: "observations.read"}
			}
		case "sources":
			if len(segments) == 1 || len(segments) == 2 || (len(segments) == 3 && (segments[2] == "admission" || segments[2] == "activation" || segments[2] == "health" || segments[2] == "evidence" || segments[2] == "connector-runs")) || (len(segments) == 4 && (segments[2] == "evidence" || segments[2] == "admissions")) || (len(segments) == 5 && segments[2] == "evidence" && segments[4] == "original") {
				return RoutePolicy{Known: true, Permission: "sources.read"}
			}
		case "observations", "geofences", "alerts", "incidents", "missions", "commands", "assessments":
			if len(segments) == 1 || len(segments) == 2 {
				return RoutePolicy{Known: true, Permission: segments[0] + ".read"}
			}
		case "classifications":
			if len(segments) == 2 {
				return RoutePolicy{Known: true, Permission: "classifications.read"}
			}
		}
		return RoutePolicy{}
	}

	if method == http.MethodPost {
		segments := pathSegments(path)
		if len(segments) == 0 {
			return RoutePolicy{}
		}
		switch segments[0] {
		case "context-packages":
			if method == http.MethodGet && len(segments) == 3 && segments[2] == "geographic" || method == http.MethodPost && len(segments) == 2 && segments[1] == "geographic" {
				return RoutePolicy{Known: true, Permission: "context.export"}
			}
			if method == http.MethodPost && len(segments) == 2 && segments[1] == "preview" {
				return RoutePolicy{Known: true, Permission: "context.export"}
			}
			if (method == http.MethodGet && (len(segments) == 1 || len(segments) == 2 || (len(segments) == 3 && segments[2] == "bundle"))) || (method == http.MethodPost && len(segments) == 1) {
				return RoutePolicy{Known: true, Permission: "context.export"}
			}
		case "reference-datasets":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "sources.manage"}
			}
		case "incidents":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "incidents.create"}
			}
			if len(segments) == 3 && (segments[2] == "status" || segments[2] == "relations") {
				return RoutePolicy{Known: true, Permission: "incidents.update"}
			}
		case "missions":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "missions.create"}
			}
			if (len(segments) == 3 && (segments[2] == "status" || segments[2] == "assets" || segments[2] == "tasks")) ||
				(len(segments) == 5 && segments[2] == "tasks" && segments[4] == "status") {
				return RoutePolicy{Known: true, Permission: "missions.update"}
			}
		case "commands":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "commands.issue"}
			}
			if len(segments) == 3 && segments[2] == "transition" {
				return RoutePolicy{Known: true, Permission: "commands.transition"}
			}
		case "assessments":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "assessments.create"}
			}
		case "classifications":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "classifications.create"}
			}
		case "observations":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "observations.ingest"}
			}
		case "telemetry":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "telemetry.ingest"}
			}
		case "alerts":
			if len(segments) == 3 && segments[2] == "acknowledge" {
				return RoutePolicy{Known: true, Permission: "alerts.acknowledge"}
			}
			if len(segments) == 3 && segments[2] == "resolve" {
				return RoutePolicy{Known: true, Permission: "alerts.resolve"}
			}
		case "sources":
			if (len(segments) == 3 && segments[2] == "connector-runs") || (len(segments) == 4 && segments[2] == "publications" && segments[3] == "retry") {
				return RoutePolicy{Known: true, Permission: "connectors.execute"}
			}
			if len(segments) == 3 && segments[2] == "admission" {
				return RoutePolicy{Known: true, Permission: "sources.manage"}
			}
			if len(segments) == 1 || (len(segments) == 3 && segments[2] == "status") {
				return RoutePolicy{Known: true, Permission: "sources.manage"}
			}
		case "assets":
			if len(segments) == 1 || (len(segments) == 3 && segments[2] == "status") {
				return RoutePolicy{Known: true, Permission: "assets.manage"}
			}
		case "geofences":
			if len(segments) == 1 || (len(segments) == 3 && segments[2] == "active") {
				return RoutePolicy{Known: true, Permission: "geofences.manage"}
			}
		case "operators":
			if len(segments) == 1 {
				return RoutePolicy{Known: true, Permission: "admin.manage"}
			}
		case "scenarios":
			if validScenarioPostPath(segments) {
				return RoutePolicy{Known: true, Permission: "scenarios.control"}
			}
		}
	}

	segments := pathSegments(path)
	if method == http.MethodPatch && len(segments) == 2 && segments[0] == "incidents" {
		return RoutePolicy{Known: true, Permission: "incidents.update"}
	}
	return RoutePolicy{}
}

// PermissionFor maps API methods and paths to stable permission identifiers.
// It is retained as a small compatibility helper for callers that only need
// the permission; authorization itself also checks RoutePolicy.Known.
func PermissionFor(method, rawPath string) string {
	return PolicyFor(method, rawPath).Permission
}

func pathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func validScenarioGetPath(segments []string) bool {
	if len(segments) == 1 || len(segments) == 2 {
		return true
	}
	return len(segments) == 3 && segments[1] == "runs" || len(segments) == 4 && segments[1] == "runs" && (segments[3] == "events" || segments[3] == "manifest" || segments[3] == "version")
}

func validScenarioPostPath(segments []string) bool {
	if len(segments) == 3 && segments[2] == "start" {
		return true
	}
	if len(segments) == 4 && segments[1] == "definitions" && segments[3] == "start" {
		return true
	}
	return len(segments) == 4 && segments[1] == "runs" && (segments[3] == "pause" || segments[3] == "resume" || segments[3] == "stop" || segments[3] == "restart" || segments[3] == "speed")
}

func validGeospatialGetPath(segments []string) bool {
	return len(segments) == 2 && (segments[1] == "geofences-containing" ||
		segments[1] == "assets-within" || segments[1] == "nearest-assets")
}

// validPictureGetPath accepts the snapshot itself and the scrubber's window.
// Both are read-only reconstructions of recorded evidence.
func validPictureGetPath(segments []string) bool {
	if len(segments) == 1 {
		return true
	}
	return len(segments) == 2 && segments[1] == "window"
}

func resourcePermission(path, action string) string {
	first := strings.TrimPrefix(path, "/")
	if index := strings.IndexByte(first, '/'); index >= 0 {
		first = first[:index]
	}
	if first == "" {
		return "operational." + action
	}
	return first + "." + action
}

// Allows implements the product's initial role/permission matrix.
func Allows(role, permission string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "administrator" {
		return true
	}
	if permission == "operators.read" {
		return role == "supervisor"
	}
	read := strings.HasSuffix(permission, ".read") || permission == "operational.read" || permission == "geospatial.read"
	if role == "analyst" {
		return read || permission == "assessments.create" || permission == "classifications.create" || permission == "imagery.jobs.submit"
	}
	if role != "operator" && role != "supervisor" {
		return false
	}
	if read {
		return true
	}
	switch permission {
	case "alerts.acknowledge", "alerts.resolve",
		"incidents.create", "incidents.update",
		"missions.create", "missions.update",
		"commands.issue", "commands.transition",
		"scenarios.control":
		return true
	case "observations.ingest", "telemetry.ingest",
		"sources.manage", "assets.manage", "geofences.manage",
		"assessments.create", "classifications.create", "operators.read", "imagery.jobs.submit", "assessments.review", "context.export":
		return role == "supervisor"
	default:
		return false
	}
}

// PermissionsForRole is the single permission matrix used by /auth/me. It is
// generated from Allows so the session contract cannot drift from middleware.
func PermissionsForRole(role string) []string {
	known := []string{
		"operational.read",
		"sources.read", "observations.read", "assets.read", "telemetry.read", "tracks.read",
		"classifications.read", "geofences.read", "alerts.read", "incidents.read",
		"missions.read", "commands.read", "assessments.read", "geospatial.read",
		"audit.read", "scenarios.read", "operators.read",
		"sources.manage", "assets.manage", "geofences.manage", "observations.ingest", "telemetry.ingest",
		"alerts.acknowledge", "alerts.resolve", "incidents.create", "incidents.update",
		"missions.create", "missions.update", "commands.issue", "commands.transition",
		"assessments.create", "classifications.create", "scenarios.control", "admin.manage", "imagery.jobs.submit", "assessments.review", "context.export",
	}
	permissions := make([]string, 0, len(known))
	for _, permission := range known {
		if Allows(role, permission) {
			permissions = append(permissions, permission)
		}
	}
	return permissions
}

// WithRequestID stores the request id in context.
func WithRequestID(ctx contextT, id string) contextT { return withRequestID(ctx, id) }

// WithOperatorID stores the operator id in context.
func WithOperatorID(ctx contextT, id string) contextT { return withOperatorID(ctx, id) }

// WithTraceID stores a trace/correlation id in context.
func WithTraceID(ctx contextT, id string) contextT { return withTraceID(ctx, id) }

// GetRequestID returns the request correlation id, if any.
func GetRequestID(ctx contextT) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// GetOperatorID returns the acting operator id, if any.
func GetOperatorID(ctx contextT) string {
	if v, ok := ctx.Value(operatorIDKey).(string); ok {
		return v
	}
	return ""
}

// GetOperatorRole returns the authenticated operator role.
func GetOperatorRole(ctx contextT) string {
	if v, ok := ctx.Value(operatorRoleKey).(string); ok {
		return v
	}
	return ""
}

// GetOperatorName returns the authenticated operator display name.
func GetOperatorName(ctx contextT) string {
	if v, ok := ctx.Value(operatorNameKey).(string); ok {
		return v
	}
	return ""
}

// GetTraceID returns the request trace/correlation id.
func GetTraceID(ctx contextT) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// Logger logs completed requests with method, path, status and duration.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return LoggerWithMetrics(logger, nil)
}

// LoggerWithMetrics logs completed requests and optionally records them in the
// metrics registry.
func LoggerWithMetrics(logger *slog.Logger, metrics RequestMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if metrics != nil {
				metrics.ObserveRequest(r.Method, metricRoute(r), rec.status, rec.bytes, time.Since(start))
			}

			logger.Info("http request",
				"method", r.Method,
				"path", metricRoute(r),
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", float64(time.Since(start).Microseconds())/1000,
				"request_id", GetRequestID(r.Context()),
				"trace_id", GetTraceID(r.Context()),
				"operator_id", GetOperatorID(r.Context()),
				"operator_role", GetOperatorRole(r.Context()),
			)
		})
	}
}

func metricRoute(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "unmatched"
	}
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	// A raw fallback would turn arbitrary 404 paths into metric labels. The
	// registry has a bounded compatibility normalizer for direct callers, but
	// composed HTTP requests must use the fixed unmatched label here.
	return "unmatched"
}

// Recoverer converts panics into 500 responses without killing the server.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					if p == http.ErrAbortHandler {
						panic(p)
					}
					logger.Error("panic recovered",
						"panic", "handler failure",
						"stack", string(debug.Stack()),
						"path", metricRoute(r),
						"request_id", GetRequestID(r.Context()),
					)
					JSON(w, http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
						Code:    "internal_error",
						Message: "internal error",
					}})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS allows the operator UI (and other configured origins) to call the API.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Range, If-Range, X-EMAD-Workspace-ID, X-EMAD-Environment-ID, X-EMAD-Partition, X-EMAD-Run-ID, "+RequestIDHeader+", "+OperatorHeader+", "+TraceHeader)
					w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Range, Accept-Ranges")
					w.Header().Set("Access-Control-Max-Age", "300")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush implements http.Flusher so streaming responses keep working.
func (r *statusRecorder) Flush() {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker so protocol upgrades (WebSocket) work
// through the logging middleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

// Unwrap exposes the wrapped writer to http.ResponseController.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Input/truth transports are opt-in per known route; ordinary COP APIs remain closed.
func privilegedSimulationRoute(resource access.Resource, method, path string, policy RoutePolicy) bool {
	if !policy.Known {
		return false
	}
	if resource.Partition == access.ExerciseTruth && strings.HasPrefix(path, "/api/v1/simulation-recordings/") {
		return method == http.MethodGet && (policy.Permission == "sim.recordings.read" || policy.Permission == "sim.evaluation.read") || method == http.MethodPost && policy.Permission == "sim.evaluation.release"
	}
	if resource.Partition == access.ExerciseInput && resource.RunID == "" {
		if path == "/api/v1/platform/readiness" && method == http.MethodGet || path == "/api/v1/platform/worker-heartbeats" && method == http.MethodPost {
			return true
		}
		return strings.HasPrefix(path, "/api/v1/simulation-assets/") || strings.HasPrefix(path, "/api/v1/simulation-studio/") || strings.HasPrefix(path, "/api/v1/simulation-runtime/") && policy.Permission != "sim.truth.read"
	}
	return resource.Partition == access.ExerciseTruth && method == http.MethodGet && strings.HasPrefix(path, "/api/v1/simulation-runtime/runs/") && policy.Permission == "sim.truth.read"
}
