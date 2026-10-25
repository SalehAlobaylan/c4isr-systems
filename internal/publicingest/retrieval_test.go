package publicingest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

func TestPublicAddressExcludesInternalAndRebindingTargets(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "0.0.0.0", "198.18.0.1", "192.0.2.1", "224.0.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "2001:db8::1", "64:ff9b::a00:1", "2002:a00:1::"} {
		if publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("unsafe address allowed: %s", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("public address denied: %s", raw)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if conn, err := safeDial(ctx, "tcp", "127.0.0.1:443"); err == nil {
		conn.Close()
		t.Fatal("loopback dial allowed")
	}
}

func TestRegistryHasExactBindingsAndServerSideCredentials(t *testing.T) {
	r := access.Resource{WorkspaceID: "public", EnvironmentID: "world", Partition: access.PublicEvidence}
	base := Location{ID: "fixture", SourceID: "source-1", Resource: r, URL: "https://provider.example/feed", Adapter: "emad-json-page-v1", MaxRequestsPerDay: 10}
	for _, raw := range []string{"http://provider.example/feed", "https://user:password@provider.example/feed", "https://provider.example/feed?key=secret", "https://provider.example:8443/feed", "https://provider.example/feed#secret"} {
		bad := base
		bad.URL = raw
		if _, err := NewRegistry([]Location{bad}); err == nil {
			t.Errorf("unsafe endpoint admitted: %s", raw)
		}
	}
	if _, err := NewRegistry([]Location{base, base}); err == nil {
		t.Fatal("duplicate source budget binding allowed")
	}
	private := base
	private.Resource.Partition = access.ExerciseTruth
	private.Resource.RunID = "run"
	if _, err := NewRegistry([]Location{private}); err == nil {
		t.Fatal("truth connector admitted")
	}
	t.Setenv("P1_FIXTURE_SECRET", "server-only-value")
	base.AuthTokenEnv = "P1_FIXTURE_SECRET"
	registry, err := NewRegistry([]Location{base})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(registry[base.ID])
	if strings.Contains(string(raw), "server-only-value") {
		t.Fatal("connector credential serialized")
	}
	location := registry[base.ID]
	for _, raw := range []string{"https://foreign.example/item", "https://provider.example/item?token=secret", "http://provider.example/item"} {
		if permittedReference(location, raw) {
			t.Fatalf("unvetted artifact reference: %s", raw)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBoundedHTTPRetrievalPreservesWindowAndDoesNotLeakSecrets(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-time.Hour)
	in := FetchRequest{Now: now, From: from, To: now, Checkpoint: "resume-token", Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{MaxResponseBytes: 4096, MaxRecordsPerPage: 10, AOI: Bounds{West: 46, South: 24, East: 47, North: 25}}}}}
	location := Location{ID: "fixture", URL: "https://provider.example/feed", token: "server-secret"}
	fetcher := NewHTTPFetcher()
	fetcher.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("checkpoint") != "resume-token" || r.URL.Query().Get("from") != from.Format(time.RFC3339Nano) || r.URL.Query().Get("to") != now.Format(time.RFC3339Nano) || r.URL.Query().Get("bbox") != "46,24,47,25" || r.Header.Get("Authorization") != "Bearer server-secret" {
			t.Fatal("retrieval contract changed")
		}
		page := Page{Records: []Record{{ProviderRecordID: "id", ProviderRevision: "r1", OriginalRecord: json.RawMessage(`{"x":1}`)}}, Checkpoint: "done", CoverageFrom: from, CoverageTo: now}
		body, _ := json.Marshal(page)
		return &http.Response{StatusCode: 200, Header: http.Header{"Etag": {`"etag"`}}, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	page, meta, err := fetcher.Fetch(context.Background(), location, in)
	if err != nil || len(page.Records) != 1 || meta.LocationID != "fixture" {
		t.Fatalf("fetch: %#v %#v %v", page, meta, err)
	}
	for _, tc := range []struct {
		name           string
		status         int
		body, encoding string
		max            int
		want           string
	}{
		{"oversize", 200, strings.Repeat("x", 129), "", 128, "response_too_large"},
		{"compressed", 200, "compressed", "gzip", 4096, "unsupported_encoding"},
		{"invalid", 200, `{"records":[]} secret`, "", 4096, "invalid_response"},
		{"limited", 429, "server-secret", "", 4096, "provider_rate_limit"},
		{"outage", 503, "server-secret", "", 4096, "provider_unavailable"},
		{"auth", 401, "server-secret", "", 4096, "provider_auth"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in.Admission.Policy.MaxResponseBytes = tc.max
			fetcher.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Encoding": {tc.encoding}, "Retry-After": {"120"}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			_, _, err := fetcher.Fetch(context.Background(), location, in)
			if err == nil || err.Error() != tc.want || strings.Contains(err.Error(), "server-secret") {
				t.Fatalf("unsafe failure: %v", err)
			}
		})
	}
}

func TestHTTPRedirectAndTimeoutAreBounded(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		http.Redirect(w, r, "https://127.0.0.1/private", http.StatusFound)
	}))
	defer server.Close()
	fetcher := NewHTTPFetcher()
	fetcher.client.Transport = server.Client().Transport // Test-only transport; production always uses safeDial.
	in := FetchRequest{Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{MaxResponseBytes: 4096}}}}
	if _, _, err := fetcher.Fetch(context.Background(), Location{URL: server.URL}, in); err == nil || err.Error() != "provider_unavailable" {
		t.Fatalf("redirect followed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, _, err := fetcher.Fetch(ctx, Location{URL: server.URL + "/slow"}, in); err == nil || time.Since(start) > time.Second {
		t.Fatalf("unbounded timeout: %v", err)
	}
}

func TestEvidenceIdentityNormalizationAndRetention(t *testing.T) {
	a := Admission{ID: "admission", SourceID: "source", AdmissionInput: AdmissionInput{OriginalProducer: "producer", UpstreamProvider: "provider", Dataset: "dataset", ProductVersion: "v1", RevisionPolicy: "provider_version", Rights: Rights{ID: "rights", RetainRaw: true}}}
	r := Record{ProviderRecordID: " ID ", ProviderRevision: " r1 ", OriginalRecord: json.RawMessage(`{ "b":2, "a":1 }`)}
	first, err := normalizeRecord(r, a, "first-run", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r.OriginalRecord = json.RawMessage(`{"a":1,"b":2}`)
	second, err := normalizeRecord(r, a, "retry-run", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.ContentHash != second.ContentHash || first.RawHash == second.RawHash || first.ConnectorRunID == second.ConnectorRunID {
		t.Fatal("retrieval and evidence identities were conflated")
	}
	proxy := a
	proxy.SourceID, proxy.UpstreamProvider, proxy.ConnectorID = "another-source", "another-provider", "another-connector"
	viaProxy, err := normalizeRecord(r, proxy, "proxy-run", time.Now())
	if err != nil || viaProxy.ID == first.ID || viaProxy.OriginRecordKey != first.OriginRecordKey {
		t.Fatal("shared upstream evidence cannot be recognized across connectors")
	}
	r.ProviderRevision = "r2"
	third, err := normalizeRecord(r, a, "third-run", time.Now())
	if err != nil || third.ID == first.ID {
		t.Fatal("revision identity lost")
	}
	a.Rights.RetainRaw = false
	if _, err = normalizeRecord(r, a, "reference-only", time.Now()); err == nil {
		t.Fatal("reference-only evidence lost original reference")
	}
	r.ReferenceURL = "https://provider.example/item"
	e, err := normalizeRecord(r, a, "reference-only", time.Now())
	if err != nil || len(e.OriginalRecord) != 0 || e.ContentHash == "" || e.RawHash == "" {
		t.Fatalf("reference-only retention: %v", err)
	}
	a.RevisionPolicy = "content_hash"
	r.ProviderRevision = ""
	e, err = normalizeRecord(r, a, "hash-revision", time.Now())
	if err != nil || !strings.HasPrefix(e.ProviderRevision, "sha256:") {
		t.Fatalf("content-based revision missing: %v", err)
	}
	for _, id := range []string{" ", "bad\nidentity", strings.Repeat("x", 513)} {
		r.ProviderRecordID = id
		if _, err = normalizeRecord(r, a, "bad", time.Now()); err == nil {
			t.Fatalf("invalid identity accepted: %q", id)
		}
	}
}
