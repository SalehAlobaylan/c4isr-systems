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

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
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
	databaseUp   atomic.Int64 // -1 means no health probe has completed yet.

	observationsReceived atomic.Uint64
	observationsRejected atomic.Uint64
	telemetryReceived    atomic.Uint64
	alertsGenerated      atomic.Uint64
	staleEntities        atomic.Int64
	staleMu              sync.Mutex
	staleAssets          map[string]struct{}
	commandIssuedMu      sync.Mutex
	commandIssuedAt      map[string]time.Time // command id -> issue time
	latencies            map[string]*latencyMetric
}

type counter struct{ value atomic.Uint64 }

type latencyMetric struct {
	nanos atomic.Uint64
	count atomic.Uint64
}

const (
	latencyObservationIngest = "observation_ingest"
	latencyTelemetryIngest   = "telemetry_ingest"
	latencyTrackUpdate       = "track_update"
	latencyGeofence          = "geofence_evaluation"
	latencyWebSocket         = "websocket_publish"
	latencyCommandAck        = "command_acknowledgment"
	latencyDatabase          = "database_query"
	commandIssuedAtMax       = 4096
	commandIssuedAtTTL       = 24 * time.Hour
)

// New creates a metrics registry.
func New() *Metrics {
	m := &Metrics{
		startedAt:       time.Now().UTC(),
		staleAssets:     make(map[string]struct{}),
		commandIssuedAt: make(map[string]time.Time),
		latencies: map[string]*latencyMetric{
			latencyObservationIngest: {},
			latencyTelemetryIngest:   {},
			latencyTrackUpdate:       {},
			latencyGeofence:          {},
			latencyWebSocket:         {},
			latencyCommandAck:        {},
			latencyDatabase:          {},
		},
	}
	m.databaseUp.Store(-1)
	return m
}

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
// It remains topic-only for callers that do not need event-specific latency
// and state observations.
func (m *Metrics) ObserveEvent(topic string) {
	if m == nil {
		return
	}
	m.observeTopic(topic)
}

func (m *Metrics) observeTopic(topic string) {
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

// ObserveDomainEvent records counters and the dimensions that require the
// concrete event payload, such as stale-entity state and command latency.
func (m *Metrics) ObserveDomainEvent(ev events.Event) {
	if m == nil || ev == nil {
		return
	}
	m.observeTopic(ev.Topic())
	switch e := ev.(type) {
	case events.ObservationReceived:
		m.observationsReceived.Add(1)
		m.ObserveLatency(latencyObservationIngest, e.At.Sub(e.ReceivedAt))
	case events.ObservationRejected:
		m.observationsRejected.Add(1)
	case events.TelemetryReceived:
		m.telemetryReceived.Add(1)
		m.ObserveLatency(latencyTelemetryIngest, e.At.Sub(e.ObservedAt))
		// A stale sample is an out-of-order record, not evidence that the asset
		// has gone silent. Silence is reconciled from asset_state by the periodic
		// telemetry monitor; accepted newer samples clear that status here.
		if !e.Stale {
			m.ObserveStaleEntity(e.AssetID, false)
		}
	case events.TrackUpdated:
		m.ObserveLatency(latencyTrackUpdate, e.At.Sub(e.ObservedAt))
	case events.AlertCreated:
		m.alertsGenerated.Add(1)
	case events.CommandIssued:
		if e.CommandID != "" {
			m.rememberCommandIssued(e.CommandID, e.At)
		}
	case events.CommandStatusChanged:
		if e.State == "ACKNOWLEDGED" {
			if issuedAt, ok := m.lookupCommandIssued(e.CommandID, e.At); ok {
				m.ObserveLatency(latencyCommandAck, e.At.Sub(issuedAt))
			}
		}
		if e.State == "COMPLETED" || e.State == "REJECTED" || e.State == "FAILED" || e.State == "TIMED_OUT" || e.State == "CANCELLED" {
			m.forgetCommandIssued(e.CommandID)
		}
	}
}

func (m *Metrics) rememberCommandIssued(commandID string, issuedAt time.Time) {
	m.commandIssuedMu.Lock()
	defer m.commandIssuedMu.Unlock()
	if m.commandIssuedAt == nil {
		m.commandIssuedAt = make(map[string]time.Time)
	}
	m.pruneCommandIssuedLocked(issuedAt)
	if _, exists := m.commandIssuedAt[commandID]; !exists && len(m.commandIssuedAt) >= commandIssuedAtMax {
		var oldestID string
		var oldestAt time.Time
		for id, at := range m.commandIssuedAt {
			if oldestID == "" || at.Before(oldestAt) {
				oldestID, oldestAt = id, at
			}
		}
		if oldestID != "" {
			delete(m.commandIssuedAt, oldestID)
		}
	}
	m.commandIssuedAt[commandID] = issuedAt
}

func (m *Metrics) lookupCommandIssued(commandID string, reference time.Time) (time.Time, bool) {
	m.commandIssuedMu.Lock()
	defer m.commandIssuedMu.Unlock()
	m.pruneCommandIssuedLocked(reference)
	issuedAt, ok := m.commandIssuedAt[commandID]
	return issuedAt, ok
}

func (m *Metrics) forgetCommandIssued(commandID string) {
	m.commandIssuedMu.Lock()
	delete(m.commandIssuedAt, commandID)
	m.commandIssuedMu.Unlock()
}

func (m *Metrics) pruneCommandIssuedLocked(reference time.Time) {
	if reference.IsZero() {
		return
	}
	for commandID, issuedAt := range m.commandIssuedAt {
		if !reference.Before(issuedAt) && reference.Sub(issuedAt) > commandIssuedAtTTL {
			delete(m.commandIssuedAt, commandID)
		}
	}
}

// ObserveLatency records a low-cardinality duration aggregate. Unknown names
// are ignored so callers cannot create unbounded metric families.
func (m *Metrics) ObserveLatency(name string, duration time.Duration) {
	if m == nil || duration < 0 {
		return
	}
	metric := m.latencies[name]
	if metric == nil {
		return
	}
	metric.nanos.Add(uint64(duration.Nanoseconds()))
	metric.count.Add(1)
}

// ObserveGeofenceEvaluation records the time spent applying spatial rules.
func (m *Metrics) ObserveGeofenceEvaluation(duration time.Duration) {
	m.ObserveLatency(latencyGeofence, duration)
}

// ObserveWebSocketPublish records the time spent writing one event to a
// connected WebSocket client.
func (m *Metrics) ObserveWebSocketPublish(duration time.Duration) {
	m.ObserveLatency(latencyWebSocket, duration)
}

// ObserveDatabaseQuery records one PostgreSQL query or command duration.
func (m *Metrics) ObserveDatabaseQuery(duration time.Duration) {
	m.ObserveLatency(latencyDatabase, duration)
}

// ObserveStaleEntity maintains a bounded gauge of assets whose latest
// telemetry is stale. Asset identifiers are kept only in an internal set; no
// identifier is exposed as a metric label.
func (m *Metrics) ObserveStaleEntity(assetID string, stale bool) {
	if m == nil || strings.TrimSpace(assetID) == "" {
		return
	}
	assetID = strings.TrimSpace(assetID)
	m.staleMu.Lock()
	defer m.staleMu.Unlock()
	if m.staleAssets == nil {
		m.staleAssets = make(map[string]struct{})
	}
	if stale {
		if _, loaded := m.staleAssets[assetID]; !loaded {
			m.staleAssets[assetID] = struct{}{}
			m.staleEntities.Add(1)
		}
		return
	}
	if _, loaded := m.staleAssets[assetID]; loaded {
		delete(m.staleAssets, assetID)
		m.staleEntities.Add(-1)
	}
}

// ReplaceStaleEntities replaces the stale set from a database reconciliation.
// It also removes assets that recovered without producing a telemetry event,
// while the mutex keeps a scrape and a live telemetry update consistent.
func (m *Metrics) ReplaceStaleEntities(assetIDs []string) {
	if m == nil {
		return
	}
	next := make(map[string]struct{}, len(assetIDs))
	for _, assetID := range assetIDs {
		if assetID = strings.TrimSpace(assetID); assetID != "" {
			next[assetID] = struct{}{}
		}
	}
	m.staleMu.Lock()
	m.staleAssets = next
	m.staleEntities.Store(int64(len(next)))
	m.staleMu.Unlock()
}

// ObserveAuthFailure increments the authentication failure counter.
func (m *Metrics) ObserveAuthFailure() {
	if m != nil {
		inc(&m.valueCount, "auth_failures_total")
	}
}

// ObserveDatabaseHealth records the latest database readiness result. It is a
// gauge because a failed probe must remain visible until a successful probe
// clears it, even if no request is currently reaching the API.
func (m *Metrics) ObserveDatabaseHealth(up bool) {
	if m == nil {
		return
	}
	if up {
		m.databaseUp.Store(1)
		return
	}
	m.databaseUp.Store(0)
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
		if databaseUp := m.databaseUp.Load(); databaseUp >= 0 {
			_, _ = fmt.Fprint(w, "# HELP c4isr_database_up Whether the latest database health probe succeeded.\n")
			_, _ = fmt.Fprint(w, "# TYPE c4isr_database_up gauge\n")
			_, _ = fmt.Fprintf(w, "c4isr_database_up %d\n", databaseUp)
		}

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

		writeSimpleCounter(w, "c4isr_observations_received_total", "Accepted observations.", m.observationsReceived.Load())
		writeSimpleCounter(w, "c4isr_observations_rejected_total", "Rejected observations.", m.observationsRejected.Load())
		writeSimpleCounter(w, "c4isr_telemetry_received_total", "Accepted telemetry samples.", m.telemetryReceived.Load())
		writeSimpleCounter(w, "c4isr_alerts_generated_total", "Generated alerts.", m.alertsGenerated.Load())
		_, _ = fmt.Fprint(w, "# HELP c4isr_stale_entities Current number of assets with stale telemetry.\n")
		_, _ = fmt.Fprint(w, "# TYPE c4isr_stale_entities gauge\n")
		_, _ = fmt.Fprintf(w, "c4isr_stale_entities %d\n", maxInt64(m.staleEntities.Load(), 0))

		writeLatency(w, "c4isr_observation_ingest_duration_seconds", "Observation ingest latency.", m.latencies[latencyObservationIngest])
		writeLatency(w, "c4isr_telemetry_ingest_duration_seconds", "Telemetry ingest latency.", m.latencies[latencyTelemetryIngest])
		writeLatency(w, "c4isr_track_update_duration_seconds", "Track update latency.", m.latencies[latencyTrackUpdate])
		writeLatency(w, "c4isr_geofence_evaluation_duration_seconds", "Geofence evaluation duration.", m.latencies[latencyGeofence])
		writeLatency(w, "c4isr_websocket_publish_duration_seconds", "WebSocket publish duration.", m.latencies[latencyWebSocket])
		writeLatency(w, "c4isr_command_acknowledgment_duration_seconds", "Command acknowledgment latency.", m.latencies[latencyCommandAck])
		writeLatency(w, "c4isr_database_query_duration_seconds", "Database query duration.", m.latencies[latencyDatabase])

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
	if queryIndex := strings.IndexByte(path, '?'); queryIndex >= 0 {
		path = path[:queryIndex]
	}
	path = strings.TrimSpace(path)
	if path == "/health" || path == "/metrics" {
		return path
	}
	if path == "unmatched" {
		return path
	}
	if path == "/api/v1/auth/me" {
		return path
	}
	if !strings.HasPrefix(path, "/api/v1/") {
		return "unmatched"
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	if len(parts) == 0 || !knownMetricRoot(parts[0]) {
		return "unmatched"
	}
	root := parts[0]
	if len(parts) == 1 {
		return "/api/v1/" + root
	}

	// Route templates retain useful endpoint shape while replacing every
	// resource identity with a fixed placeholder. This supports both chi's
	// already-templated route and direct callers that provide a concrete id.
	if root == "geospatial" {
		if len(parts) == 2 && (parts[1] == "geofences-containing" || parts[1] == "assets-within" || parts[1] == "nearest-assets") {
			return "/api/v1/" + root + "/" + parts[1]
		}
		return "unmatched"
	}
	if root == "realtime" {
		return "unmatched"
	}
	if root == "scenarios" {
		return boundedScenarioRoute(parts)
	}
	if len(parts) == 2 {
		return "/api/v1/" + root + "/{id}"
	}
	if len(parts) == 3 && boundedResourceAction(root, parts[2]) {
		return "/api/v1/" + root + "/{id}/" + parts[2]
	}
	if root == "missions" && len(parts) == 4 && parts[2] == "tasks" {
		return "/api/v1/missions/{id}/tasks/{taskId}"
	}
	if root == "missions" && len(parts) == 5 && parts[2] == "tasks" && parts[4] == "status" {
		return "/api/v1/missions/{id}/tasks/{taskId}/status"
	}
	return "unmatched"
}

func boundedScenarioRoute(parts []string) string {
	if len(parts) == 2 {
		if parts[1] == "runs" {
			return "/api/v1/scenarios/runs"
		}
		return "/api/v1/scenarios/{name}"
	}
	if len(parts) == 3 && parts[1] == "runs" {
		if parts[2] == "" {
			return "unmatched"
		}
		return "/api/v1/scenarios/runs/{id}"
	}
	if len(parts) == 4 && parts[1] == "runs" {
		switch parts[3] {
		case "events", "pause", "resume", "stop", "restart", "speed":
			return "/api/v1/scenarios/runs/{id}/" + parts[3]
		}
	}
	if len(parts) == 4 && parts[1] == "definitions" && parts[3] == "start" {
		return "/api/v1/scenarios/definitions/{name}/start"
	}
	return "unmatched"
}

func boundedResourceAction(root, action string) bool {
	switch root {
	case "sources":
		return action == "status"
	case "assets":
		return action == "status" || action == "telemetry"
	case "tracks":
		return action == "history" || action == "classifications"
	case "geofences":
		return action == "active"
	case "alerts":
		return action == "acknowledge" || action == "resolve"
	case "incidents":
		return action == "status" || action == "relations"
	case "missions":
		return action == "status" || action == "assets" || action == "tasks"
	case "commands":
		return action == "transition"
	default:
		return false
	}
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

func writeSimpleCounter(w http.ResponseWriter, metric, help string, value uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", metric, help, metric, metric, value)
}

func writeLatency(w http.ResponseWriter, metric, help string, value *latencyMetric) {
	if value == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "# HELP %s Sum and count of %s\n# TYPE %s summary\n", metric, help, metric)
	_, _ = fmt.Fprintf(w, "%s_sum %.9f\n%s_count %d\n", metric, float64(value.nanos.Load())/float64(time.Second), metric, value.count.Load())
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
