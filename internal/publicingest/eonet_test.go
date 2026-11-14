package publicingest

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"
)

func eonetRequest() FetchRequest {
	return FetchRequest{
		Admission: Admission{SourceID: "eonet", AdmissionInput: AdmissionInput{RevisionPolicy: "content_hash", Rights: Rights{RetainRaw: true},
			Policy: Policy{AOI: Bounds{West: 34, South: 16, East: 56, North: 33}, MaxRecordsPerPage: 100, MaxResponseBytes: 1 << 20}}},
		From: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
		Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

const floodFixture = `{"events":[{"id":"EONET_12","title":"Test flood","link":"https://eonet.gsfc.nasa.gov/api/v3/events/EONET_12",
 "closed":null,"categories":[{"id":"floods"}],"sources":[{"id":"GDACS","url":"https://www.gdacs.org/report.aspx?eventtype=FL&eventid=123"}],
 "geometry":[{"date":"2025-12-05T01:00:00Z","type":"Point","coordinates":[35,32]},
 {"date":"2025-12-07T01:00:00Z","type":"Polygon","coordinates":[[[35,32],[36,32],[36,32.5],[35,32]]]}]}]}`

func TestEONETRetainsRevisionGeometryAndExactCitation(t *testing.T) {
	in := eonetRequest()
	page, err := DecodeEONET([]byte(floodFixture), in)
	if err != nil || len(page.Records) != 1 {
		t.Fatal(page, err)
	}
	r := page.Records[0]
	if r.Context.Report.LineageKeys[1] != "gdacs:FL:123" || len(r.Context.Report.EventHistory.Geometries) != 2 ||
		r.Context.Report.EventHistory.Geometries[1].Type != "Polygon" || r.Context.Report.EventHistory.ClosedAt != nil {
		t.Fatal(r.Context)
	}
	old, err := normalizeRecord(r, in.Admission, "old", in.Now)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.ReplaceAll([]byte(floodFixture), []byte(`"closed":null`), []byte(`"closed":"2025-12-08T00:00:00Z"`))
	next, err := DecodeEONET(changed, in)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := normalizeRecord(next.Records[0], in.Admission, "new", in.Now)
	if err != nil || fresh.ID == old.ID || fresh.ProviderRecordID != old.ProviderRecordID || fresh.Context.Report.EventHistory.ClosedAt == nil {
		t.Fatal(fresh, err)
	}
	narrow := in
	narrow.From = time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC)
	retry, err := DecodeEONET(changed, narrow)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(next.Records[0].Context)
	b, _ := json.Marshal(retry.Records[0].Context)
	if !bytes.Equal(a, b) {
		t.Fatal("selection window changed a retained source revision")
	}
}

func TestEONETCapsAndFailureAreDistinctFromEmpty(t *testing.T) {
	in := eonetRequest()
	u, _ := url.Parse(eonetURL(in))
	if u.Query().Get("bbox") != "34,33,56,16" || u.Query().Get("category") != "floods" {
		t.Fatal(u)
	}
	in.Admission.Policy.MaxRecordsPerPage = 1
	p, err := DecodeEONET([]byte(floodFixture), in)
	if err != nil || p.PartialReason != "provider_result_limit" {
		t.Fatal(p, err)
	}
	p, err = DecodeEONET([]byte(`{"events":[]}`), in)
	if err != nil || p.PartialReason != "" || p.Records == nil {
		t.Fatal(p, err)
	}
	for _, raw := range []string{`{"error":"unavailable"}`, `{"events":null}`, `{"events":[{}]}`} {
		if _, err := DecodeEONET([]byte(raw), in); err == nil {
			t.Fatal("accepted invalid response", raw)
		}
	}
}

func TestEONETRecordedRegionalPilot(t *testing.T) {
	path := os.Getenv("EMAD_EONET_PILOT_RESPONSE")
	if path == "" {
		t.Skip("Set the retained EONET response path")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	in := eonetRequest()
	in.From = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	page, err := DecodeEONET(raw, in)
	if err != nil || page.PartialReason != "" || len(page.Records) != 8 {
		t.Fatal(page, err)
	}
	for _, r := range page.Records {
		if len(r.Context.Report.LineageKeys) != 2 {
			t.Fatal(r.Context.Report)
		}
		if _, err := normalizeRecord(r, in.Admission, "pilot", in.Now); err != nil {
			t.Fatal(err)
		}
	}
}
