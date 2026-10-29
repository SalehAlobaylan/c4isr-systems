package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Fixed operation labels exclude resource IDs, case prose, query strings and
// credentials. Status classes keep errors/timeouts in the same distribution.
func requestOperation(path string) string {
	path = strings.SplitN(path, "?", 2)[0]
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "v1" {
		return "other"
	}
	switch parts[2] {
	case "catalog":
		return "catalog"
	case "analyst-scope":
		return "map_scope"
	case "coverage":
		return "coverage"
	case "reference-datasets", "geography", "geographic-releases":
		return "geography"
	case "imagery":
		return "imagery"
	case "investigations":
		return "investigation"
	case "outputs":
		return "output"
	case "delivery-jobs":
		return "delivery"
	case "acquisitions":
		return "acquisition"
	case "sources":
		return "source"
	default:
		return "other"
	}
}

var requestBounds = [...]float64{.005, .01, .025, .05, .1, .25, .5, 1, 2, 3, 5, 10, 30, 60}

type operationHistogram struct {
	mu      sync.Mutex
	buckets [len(requestBounds)]uint64
	count   uint64
	nanos   uint64
}

func (m *Metrics) observeOperation(method, path string, status int, duration time.Duration) {
	class := "other"
	if status >= 100 && status <= 599 {
		class = fmt.Sprintf("%dxx", status/100)
	}
	key := boundedMethod(method) + "|" + requestOperation(path) + "|" + class
	actual, _ := m.operationLatency.LoadOrStore(key, &operationHistogram{})
	h := actual.(*operationHistogram)
	nanos := uint64(maxInt64(duration.Nanoseconds(), 0))
	seconds := float64(nanos) / float64(time.Second)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.nanos += nanos
	for i, bound := range requestBounds {
		if seconds <= bound {
			h.buckets[i]++
		}
	}
}

func (m *Metrics) writeOperationLatencies(w http.ResponseWriter) {
	const metric = "c4isr_http_operation_duration_seconds"
	fmt.Fprintf(w, "# HELP %s All completed requests by bounded operation, including failures.\n# TYPE %s histogram\n", metric, metric)
	entries := map[string]*operationHistogram{}
	keys := []string{}
	m.operationLatency.Range(func(key, value any) bool {
		entries[key.(string)] = value.(*operationHistogram)
		keys = append(keys, key.(string))
		return true
	})
	sort.Strings(keys)
	for _, key := range keys {
		h := entries[key]
		h.mu.Lock()
		buckets, count, nanos := h.buckets, h.count, h.nanos
		h.mu.Unlock()
		values := strings.Split(key, "|")
		names := []string{"method", "operation", "status_class"}
		labels := formatLabels(names, values)
		for i, bound := range requestBounds {
			fmt.Fprintf(w, "%s_bucket{%s,le=\"%g\"} %d\n", metric, labels, bound, buckets[i])
		}
		fmt.Fprintf(w, "%s_bucket{%s,le=\"+Inf\"} %d\n%s_count{%s} %d\n%s_sum{%s} %.9f\n", metric, labels, count, metric, labels, count, metric, labels, float64(nanos)/float64(time.Second))
	}
}
