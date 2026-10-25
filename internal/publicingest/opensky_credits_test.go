package publicingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOpenSkyRemainingCreditsAreReportedNotAssumed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	in := FetchRequest{Now: now, From: now.Add(-time.Minute), To: now, Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{MaxResponseBytes: 4096, MaxRecordsPerPage: 200, AOI: Bounds{West: 39, South: 21, East: 40, North: 22}}}}}
	for _, tc := range []struct {
		header string
		want   *int64
	}{
		{"3997", creditValue(3997)}, {"0", creditValue(0)}, {"", nil}, {"-1", nil}, {"4000 credits", nil}, {"99999999999999999999", nil},
	} {
		t.Run("remaining="+tc.header, func(t *testing.T) {
			fetcher := NewHTTPFetcher()
			fetcher.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				h := make(http.Header)
				h.Set("X-Rate-Limit-Remaining", tc.header)
				return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"time":%d,"states":null}`, now.Unix())))}, nil
			})
			_, meta, err := fetcher.Fetch(context.Background(), Location{Adapter: OpenSkyAdapter, URL: OpenSkyURL}, in)
			if err != nil {
				t.Fatal(err)
			}
			if (meta.CreditsRemaining == nil) != (tc.want == nil) || tc.want != nil && *meta.CreditsRemaining != *tc.want {
				t.Fatalf("unexpected provider balance: %v", meta.CreditsRemaining)
			}
		})
	}
}

func TestOpenSkyCreditExhaustionUsesProviderRetryHeader(t *testing.T) {
	now := time.Now().UTC()
	fetcher := NewHTTPFetcher()
	fetcher.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		h := make(http.Header)
		h.Set("X-Rate-Limit-Remaining", "0")
		h.Set("X-Rate-Limit-Retry-After-Seconds", "90")
		return &http.Response{StatusCode: 429, Header: h, Body: io.NopCloser(strings.NewReader("not retained"))}, nil
	})
	_, meta, err := fetcher.Fetch(context.Background(), Location{Adapter: OpenSkyAdapter, URL: OpenSkyURL}, FetchRequest{Now: now})
	var limited *RetrievalError
	if !errors.As(err, &limited) || limited.Code != "provider_rate_limit" || !limited.RetryAt.Equal(now.Add(90*time.Second)) || meta.CreditsRemaining == nil || *meta.CreditsRemaining != 0 {
		t.Fatal("provider credit exhaustion/retry was not retained correctly", err)
	}
}

func creditValue(value int64) *int64 { return &value }
