package publicingest

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func movingRequest() FetchRequest {
	now := time.Now().UTC()
	return FetchRequest{Now: now, From: now.Add(-time.Minute), To: now, Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{AOI: Bounds{West: 39, South: 21, East: 40, North: 22}, LookbackSeconds: 300, MaxRecordsPerPage: 10, MaxResponseBytes: 1048576}}}}
}
func skyBytes(in FetchRequest, position any) []byte {
	raw, _ := json.Marshal(map[string]any{"time": in.Now.Unix(), "states": []any{[]any{"abcdef", "TEST123", "Provider-inferred country", in.Now.Add(-10 * time.Second).Unix(), in.Now.Unix(), 39.2, position, 9000, false, 180.0, 90.0, nil, nil, nil, nil, false, 0}}})
	return raw
}
func aisBytes(at time.Time) []byte {
	raw, _ := json.Marshal(map[string]any{"MessageType": "PositionReport", "MetaData": map[string]any{"MMSI": 123456789, "ShipName": "SYNTHETIC VESSEL", "time_utc": at.Format(time.RFC3339Nano)}, "Message": map[string]any{"PositionReport": map[string]any{"UserID": 123456789, "Valid": true, "Latitude": 21.4, "Longitude": 39.2, "Sog": 12.4, "Cog": 360, "TrueHeading": 511, "PositionAccuracy": true}}})
	return raw
}
func TestOpenSkyTimeAndClaimsRemainSeparate(t *testing.T) {
	in := movingRequest()
	p, err := DecodeOpenSky(skyBytes(in, 21.4), in)
	if err != nil || len(p.Records) != 1 {
		t.Fatal(p, err)
	}
	o := p.Records[0].Moving
	if !o.ObservedAt.Equal(time.Unix(in.Now.Add(-10*time.Second).Unix(), 0)) || o.Claims["affiliation"] != "unassessed" || o.SpeedMPS == nil || *o.SpeedMPS != 180 {
		t.Fatal(o)
	}
	if p, err = DecodeOpenSky(skyBytes(in, nil), in); err != nil || len(p.Records) != 0 || p.ExcludedRecords != 1 {
		t.Fatal("missing position invented", p, err)
	}
	if _, err = DecodeOpenSky(skyBytes(in, 95), in); err == nil {
		t.Fatal("invalid latitude accepted")
	}
	stale := in
	stale.Now = stale.Now.Add(time.Hour)
	if _, err = DecodeOpenSky(skyBytes(in, 21.4), stale); err == nil {
		t.Fatal("old snapshot refreshed")
	}
}
func TestAISUnitsSentinelsAndOriginTime(t *testing.T) {
	in := movingRequest()
	raw := aisBytes(in.Now.Add(-5 * time.Second))
	r, err := DecodeAIS(raw, in)
	if err != nil || r == nil || r.Moving.Course != nil || r.Moving.Claims["trueHeadingDegrees"] != nil || r.Moving.SpeedMPS == nil || *r.Moving.SpeedMPS < 6.37 || *r.Moving.SpeedMPS > 6.39 {
		t.Fatal(r, err)
	}
	var doc map[string]any
	json.Unmarshal(raw, &doc)
	doc["MetaData"].(map[string]any)["time_utc"] = "missing"
	invalid, _ := json.Marshal(doc)
	if _, err = DecodeAIS(invalid, in); err == nil {
		t.Fatal("missing full UTC origin invented")
	}
}

type redirectTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c := r.Clone(r.Context())
	u := *r.URL
	u.Scheme = t.target.Scheme
	u.Host = t.target.Host
	c.URL = &u
	return t.base.RoundTrip(c)
}
func TestAISCompressedBinaryCaptureBudgetsAndCredentials(t *testing.T) {
	in := movingRequest()
	in.Admission.Policy.MaxRecordsPerPage = 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
		if err != nil {
			return
		}
		defer c.CloseNow()
		_, raw, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var subscription map[string]any
		json.Unmarshal(raw, &subscription)
		if subscription["APIKey"] != "fixture-secret-only" || len(subscription["BoundingBoxes"].([]any)) != 1 {
			t.Error("invalid bounded server subscription")
		}
		c.Write(r.Context(), websocket.MessageBinary, []byte(`{"MessageType":"SubscriptionConfirmation","Message":{"CompressionEnabled":true}}`))
		c.Write(r.Context(), websocket.MessageBinary, aisBytes(time.Now().UTC().Add(-time.Second)))
		<-r.Context().Done()
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	f := NewHTTPFetcher()
	f.client = &http.Client{Transport: redirectTransport{http.DefaultTransport, target}}
	work, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	page, meta, err := f.Fetch(work, Location{ID: "ais-test", Adapter: AISAdapter, URL: AISURL, token: "fixture-secret-only"}, in)
	if err != nil || len(page.Records) != 1 || !meta.CompressionEnabled || !meta.Discontinuous || page.PartialReason != "stream_record_budget" {
		t.Fatal(page, meta, err)
	}
	all, _ := json.Marshal(struct {
		P Page
		R Retrieval
	}{page, meta})
	if bytes.Contains(all, []byte("fixture-secret-only")) {
		t.Fatal("server credential leaked into retained provenance")
	}
	if string(all) == "" {
		t.Fatal("missing provenance")
	}
}

func TestAISRejectsUnconfirmedUncompressedAndCredentialEchoFrames(t *testing.T) {
	for name, frame := range map[string]string{
		"ais_compression_required":     `{"MessageType":"SubscriptionConfirmation","Message":{"CompressionEnabled":false}}`,
		"ais_unconfirmed_data":         `{"MessageType":"PositionReport"}`,
		"ais_credential_echo_rejected": `{"error":"fixture-secret-only"}`,
		"ais_subscription_rejected":    `{"error":"Invalid API key"}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
				if err != nil {
					return
				}
				defer c.CloseNow()
				if _, _, err = c.Read(r.Context()); err != nil {
					return
				}
				c.Write(r.Context(), websocket.MessageBinary, []byte(frame))
			}))
			defer server.Close()
			target, _ := url.Parse(server.URL)
			f := NewHTTPFetcher()
			f.client = &http.Client{Transport: redirectTransport{http.DefaultTransport, target}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _, err := f.Fetch(ctx, Location{ID: "ais-test", Adapter: AISAdapter, URL: AISURL, token: "fixture-secret-only"}, movingRequest())
			if err == nil || err.Error() != name {
				t.Fatal(name, err)
			}
		})
	}
}

func TestOpenSkyNativeHTTPBoundsSnapshotAndRetryAfter(t *testing.T) {
	in := movingRequest()
	f := NewHTTPFetcher()
	calls := 0
	f.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Scheme != "https" || r.URL.Host != "opensky-network.org" || r.URL.Path != "/api/states/all" || len(r.URL.Query()) != 4 || r.URL.Query().Get("lamin") != "21" || r.URL.Query().Get("lomax") != "40" || r.Header.Get("Authorization") != "" {
			t.Fatal("native anonymous AOI contract changed", r.URL)
		}
		if calls == 2 {
			return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"120"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(skyBytes(in, 21.4)))}, nil
	})}
	location := Location{ID: "sky-test", URL: OpenSkyURL, Adapter: OpenSkyAdapter}
	page, meta, err := f.Fetch(context.Background(), location, in)
	if err != nil || len(page.Records) != 1 || !meta.Discontinuous || meta.ResponseHash == "" {
		t.Fatal(page, meta, err)
	}
	_, _, err = f.Fetch(context.Background(), location, in)
	e, ok := err.(*RetrievalError)
	if !ok || e.Code != "provider_rate_limit" || !e.RetryAt.Equal(in.Now.Add(120*time.Second)) {
		t.Fatal(err)
	}
}
