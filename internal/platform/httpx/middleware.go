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

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
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
		raw = strings.TrimSpace(r.URL.Query().Get("access_token"))
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
		case "scenarios":
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
		case "tracks":
			if len(segments) == 1 || len(segments) == 2 {
				return RoutePolicy{Known: true, Permission: "tracks.read"}
			}
			if len(segments) == 3 && (segments[2] == "history" || segments[2] == "classifications") {
				return RoutePolicy{Known: true, Permission: "tracks.read"}
			}
		case "sources", "observations", "geofences", "alerts", "incidents", "missions", "commands", "assessments":
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
	return len(segments) == 3 && segments[1] == "runs" || len(segments) == 4 && segments[1] == "runs" && segments[3] == "events"
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
		return read || permission == "assessments.create" || permission == "classifications.create"
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
		"assessments.create", "classifications.create", "operators.read":
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
		"assessments.create", "classifications.create", "scenarios.control", "admin.manage",
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
				"path", r.URL.Path,
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
					logger.Error("panic recovered",
						"panic", p,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
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
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+RequestIDHeader+", "+OperatorHeader+", "+TraceHeader)
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
