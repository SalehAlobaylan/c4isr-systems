package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOperationHistogramIncludesFailuresAndBoundsPrivateLabels(t *testing.T) {
	m := New()
	m.ObserveRequest("POST", "/api/v1/catalog/reads?secret=never", 200, 1, 10*time.Millisecond)
	m.ObserveRequest("POST", "/api/v1/catalog/private-case-title", 504, 1, 5*time.Second)
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	for _, want := range []string{
		`c4isr_http_operation_duration_seconds_bucket{method="POST",operation="catalog",status_class="2xx",le="0.01"} 1`,
		`c4isr_http_operation_duration_seconds_bucket{method="POST",operation="catalog",status_class="5xx",le="5"} 1`,
		`c4isr_http_operation_duration_seconds_count{method="POST",operation="catalog",status_class="5xx"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(body, "secret=never") || strings.Contains(body, "private-case-title") {
		t.Fatal("private metric label")
	}
}
