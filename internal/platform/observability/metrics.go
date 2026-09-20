// Package observability contains the small, dependency-light observability
// surface used by the modular monolith. It exposes Prometheus-compatible
// metrics while keeping domain packages independent from a metrics vendor.
package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics is an in-process metrics registry. The first milestone deliberately
// keeps this in memory: it is sufficient for local diagnosis and can later be
// replaced by an OpenTelemetry meter exporter without changing callers.
type Metrics struct {
	startedAt time.Time

	requestCount sync.Map // requestKey -> *counter
	requestBytes sync.Map // requestKey -> *counter
	eventCount   sync.Map // topic -> *counter
	valueCount   sync.Map // metric name -> *counter
	requestNanos atomic.Uint64
	requestTotal atomic.Uint64
}

type counter struct{ value atomic.Uint64 }

// New creates a metrics registry.
func New() *Metrics { return &Metrics{startedAt: time.Now().UTC()} }

// ObserveRequest records a completed HTTP request. Path is supplied by the
// HTTP adapter and remains deliberately low-cardinality in this application.
func (m *Metrics) ObserveRequest(method, path string, status int, bytes int, duration time.Duration) {
	if m == nil {
		return
	}
	method = boundedMethod(method)
	route := boundedRoute(path)
	if status == http.StatusNotFound {
		route = "unmatched"
	}
	key := fmt.Sprintf("%s|%s|%d", method, route, status)
	inc(&m.requestCount, key)
	add(&m.requestBytes, key, uint64(maxInt(bytes, 0)))
	m.requestTotal.Add(1)
	m.requestNanos.Add(uint64(maxInt64(duration.Nanoseconds(), 0)))
}

func boundedMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead,
		http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

// ObserveEvent records a published domain event and useful aggregate counters.
func (m *Metrics) ObserveEvent(topic string) {
	if m == nil {
		return
	}
	inc(&m.eventCount, topic)
	switch topic {
	case "observation.received":
		inc(&m.valueCount, "observations_received_total")
	case "telemetry.received":
		inc(&m.valueCount, "telemetry_received_total")
	case "alert.created":
		inc(&m.valueCount, "alerts_created_total")
	case "command.status.changed":
		inc(&m.valueCount, "command_transitions_total")
	}
}

// ObserveAuthFailure increments the authentication failure counter.
func (m *Metrics) ObserveAuthFailure() {
	if m != nil {
		inc(&m.valueCount, "auth_failures_total")
	}
}

// Handler returns the Prometheus text exposition endpoint.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if m == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = fmt.Fprint(w, "# HELP c4isr_uptime_seconds Seconds since the server metrics registry was created.\n")
		_, _ = fmt.Fprint(w, "# TYPE c4isr_uptime_seconds gauge\n")
		_, _ = fmt.Fprintf(w, "c4isr_uptime_seconds %.3f\n", time.Since(m.startedAt).Seconds())

		writeCounterMap(w, "c4isr_http_requests_total", "Completed HTTP requests.", "method", "path", "status", &m.requestCount, func(key string) []string {
			parts := strings.SplitN(key, "|", 3)
			if len(parts) != 3 {
				return []string{"unknown", "unknown", "0"}
			}
			return parts
		})
		writeCounterMap(w, "c4isr_http_response_bytes_total", "Response bytes written by completed HTTP requests.", "method", "path", "status", &m.requestBytes, func(key string) []string {
			parts := strings.SplitN(key, "|", 3)
			if len(parts) != 3 {
				return []string{"unknown", "unknown", "0"}
			}
			return parts
		})
		writeCounterMap(w, "c4isr_domain_events_total", "Published domain events.", "topic", "", "", &m.eventCount, func(key string) []string {
			return []string{key}
		})
		writeCounterMap(w, "c4isr_platform_total", "Platform aggregate counters.", "metric", "", "", &m.valueCount, func(key string) []string {
			return []string{key}
		})

		_, _ = fmt.Fprint(w, "# HELP c4isr_http_request_duration_seconds_sum Sum of completed request durations.\n")
		_, _ = fmt.Fprint(w, "# TYPE c4isr_http_request_duration_seconds_sum counter\n")
		_, _ = fmt.Fprintf(w, "c4isr_http_request_duration_seconds_sum %.9f\n", float64(m.requestNanos.Load())/float64(time.Second))
		_, _ = fmt.Fprint(w, "# HELP c4isr_http_request_duration_seconds_count Number of completed request durations.\n")
		_, _ = fmt.Fprint(w, "# TYPE c4isr_http_request_duration_seconds_count counter\n")
		_, _ = fmt.Fprintf(w, "c4isr_http_request_duration_seconds_count %d\n", m.requestTotal.Load())
	}
}

func boundedRoute(path string) string {
	path = strings.TrimSpace(path)
	if path == "/health" || path == "/metrics" {
		return path
	}
	if path == "unmatched" {
		return path
	}
	if !strings.HasPrefix(path, "/api/v1/") {
		return "unmatched"
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	if len(parts) == 0 || !knownMetricRoot(parts[0]) {
		return "unmatched"
	}
	// The composed HTTP middleware passes chi's already-bounded route template
	// here. Direct callers do not have that routing context, so even a string
	// containing braces must collapse to a known root rather than becoming an
	// attacker-controlled metric label.
	return "/api/v1/" + parts[0]
}

func knownMetricRoot(root string) bool {
	switch root {
	case "auth", "sources", "observations", "assets", "telemetry", "tracks", "classifications", "geofences", "geospatial", "alerts", "incidents", "missions", "commands", "assessments", "audit", "operators", "scenarios", "realtime":
		return true
	default:
		return false
	}
}

func inc(store *sync.Map, key string) {
	add(store, key, 1)
}

func add(store *sync.Map, key string, value uint64) {
	actual, _ := store.LoadOrStore(key, &counter{})
	actual.(*counter).value.Add(value)
}

func writeCounterMap(w http.ResponseWriter, metric, help, labelA, labelB, labelC string, store *sync.Map, labels func(string) []string) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", metric, help, metric)
	entries := make(map[string]uint64)
	store.Range(func(key, value any) bool {
		entries[key.(string)] = value.(*counter).value.Load()
		return true
	})
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values := labels(key)
		labelNames := make([]string, 0, 3)
		labelValues := make([]string, 0, 3)
		for i, name := range []string{labelA, labelB, labelC} {
			if name == "" || i >= len(values) {
				continue
			}
			labelNames = append(labelNames, name)
			labelValues = append(labelValues, values[i])
		}
		if len(labelNames) == 0 {
			_, _ = fmt.Fprintf(w, "%s %d\n", metric, entries[key])
			continue
		}
		_, _ = fmt.Fprintf(w, "%s{%s} %d\n", metric, formatLabels(labelNames, labelValues), entries[key])
	}
}

func formatLabels(names, values []string) string {
	parts := make([]string, 0, len(names))
	for i := range names {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, names[i], strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(values[i])))
	}
	return strings.Join(parts, ",")
}

func maxInt(value, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}

func maxInt64(value, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}
