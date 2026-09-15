package httpx

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	operatorIDKey
)

// RequestIDHeader is the header carrying the request correlation id.
const RequestIDHeader = "X-Request-ID"

// OperatorHeader carries the acting operator id. Until Phase 17 (RBAC) the
// server falls back to a configured default operator, but any operator action
// is still attributed in the audit log.
const OperatorHeader = "X-Operator-ID"

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

// WithRequestID stores the request id in context.
func WithRequestID(ctx contextT, id string) contextT { return withRequestID(ctx, id) }

// WithOperatorID stores the operator id in context.
func WithOperatorID(ctx contextT, id string) contextT { return withOperatorID(ctx, id) }

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

// Logger logs completed requests with method, path, status and duration.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", float64(time.Since(start).Microseconds())/1000,
				"request_id", GetRequestID(r.Context()),
				"operator_id", GetOperatorID(r.Context()),
			)
		})
	}
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
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+RequestIDHeader+", "+OperatorHeader)
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
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush implements http.Flusher so streaming responses keep working.
func (r *statusRecorder) Flush() {
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
