package publicingest

import (
	"context"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type oauthTransport func(*http.Request) (*http.Response, error)

func (f oauthTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOpenSkyOAuthFixedEndpointCacheAndExpiry(t *testing.T) {
	calls := 0
	f := NewHTTPFetcher()
	location := Location{oauthID: "unit-client", oauthSecret: "unit-secret-value"}
	f.client.Transport = oauthTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != OpenSkyTokenURL || r.Method != "POST" {
			t.Fatal("credential egress changed")
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "grant_type=client_credentials") || !strings.Contains(string(raw), "client_secret=unit-secret-value") {
			t.Fatal("missing native OAuth form")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"unit-bearer-token-value","expires_in":1800,"token_type":"Bearer"}`)), Header: make(http.Header)}, nil
	})
	for i := 0; i < 2; i++ {
		value, err := f.openSkyToken(context.Background(), location)
		if err != nil || value != "unit-bearer-token-value" {
			t.Fatal("OAuth failed", err)
		}
	}
	if calls != 1 {
		t.Fatal("unnecessary token request")
	}
	for key, value := range f.oauthTokens {
		value.expires = time.Now().Add(30 * time.Second)
		f.oauthTokens[key] = value
	}
	if _, err := f.openSkyToken(context.Background(), location); err != nil || calls != 2 {
		t.Fatal("token was not refreshed before expiry", err)
	}
}

func TestGatedProvidersLoadWithoutSecretsAndNeverFetch(t *testing.T) {
	t.Setenv("EMAD_TEST_MISSING_PROVIDER_KEY", "")
	resource := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.PublicEvidence}
	entries := []Location{
		{ID: "firms", SourceID: "fires", Resource: resource, URL: FIRMSAreaURL, Adapter: FIRMSAdapter, AuthTokenEnv: "EMAD_TEST_MISSING_PROVIDER_KEY", MaxRequestsPerDay: 1, GateState: "NEEDS_CREDENTIALS"},
		{ID: "ais", SourceID: "vessels", Resource: resource, URL: AISURL, Adapter: AISAdapter, AuthTokenEnv: "EMAD_TEST_MISSING_PROVIDER_KEY", MaxRequestsPerDay: 1, GateState: "NEEDS_PERMISSION"},
	}
	registry, err := NewRegistry(entries)
	if err != nil {
		t.Fatal(err)
	}
	fetcher := NewHTTPFetcher()
	fetcher.client.Transport = oauthTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("gated provider attempted network access")
		return nil, nil
	})
	for _, location := range registry {
		_, _, err := fetcher.Fetch(context.Background(), location, FetchRequest{})
		var retrieval *RetrievalError
		if !errors.As(err, &retrieval) {
			t.Fatalf("gated provider was not rejected: %v", err)
		}
	}
	entries[0].GateState = ""
	if _, err := NewRegistry(entries); err == nil {
		t.Fatal("activated provider accepted a missing key")
	}
}
