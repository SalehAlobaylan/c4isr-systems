package publicingest

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func contextRequest() FetchRequest {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	return FetchRequest{Now: now, From: now.Add(-6 * time.Hour), To: now, Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{AOI: Bounds{West: 46, South: 24, East: 47, North: 25}, LookbackSeconds: 21600, MaxResponseBytes: 100000, MaxRecordsPerPage: 100}}}}
}
func TestMETARUnitsUnknownsAndStationSemantics(t *testing.T) {
	in := contextRequest()
	raw := `[{"icaoId":"OERK","obsTime":1792497300,"metarType":"METAR","rawOb":"SYNTHETIC OERK VARIABLE WIND","lat":24.7,"lon":46.7,"temp":0,"dewp":null,"wdir":"VRB","wspd":10,"wgst":null,"visib":"10+","altim":1013}]`
	p, e := DecodeMETAR([]byte(raw), in)
	if e != nil || len(p.Records) != 1 {
		t.Fatal(p, e)
	}
	w := p.Records[0].Context.Weather
	if w.TemperatureC == nil || *w.TemperatureC != 0 || w.DewpointC != nil || w.WindDirectionDegrees != nil || !w.VariableWind || w.WindGustMPS != nil || !w.VisibilityLowerBound || w.WindSpeedMPS == nil || *w.WindSpeedMPS < 5.14 || *w.WindSpeedMPS > 5.15 {
		t.Fatal(w)
	}
	for _, tc := range []struct{ old, new string }{{`"lat":24.7`, `"lat":null`}, {`"metarType":"METAR"`, `"metarType":"SYNOP"`}, {`"lon":46.7`, `"lon":80`}, {`1792497300`, `1792400000`}} {
		p, e := DecodeMETAR([]byte(strings.Replace(raw, tc.old, tc.new, 1)), in)
		if e != nil || len(p.Records) != 0 || p.ExcludedRecords != 1 {
			t.Fatal(tc, p, e)
		}
	}
	for _, tc := range []struct{ old, new string }{{`"wspd":10`, `"wspd":-1`}, {`"wdir":"VRB"`, `"wdir":"unrecognized"`}, {`"visib":"10+"`, `"visib":"bad"`}, {`"rawOb":"SYNTHETIC OERK VARIABLE WIND"`, `"rawOb":""`}} {
		if _, e := DecodeMETAR([]byte(strings.Replace(raw, tc.old, tc.new, 1)), in); e == nil {
			t.Fatal("accepted invalid METAR", tc)
		}
	}
	if _, e := DecodeMETAR([]byte(`null`), in); e == nil {
		t.Fatal("null is not a confirmed empty response")
	}
}
func TestGDACSCitedTimesLineageRevisionsAndPartialPage(t *testing.T) {
	in := contextRequest()
	in.Admission.Policy.MaxRecordsPerPage = 1
	raw := `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[46.7,24.7]},"properties":{"eventtype":"EQ","eventid":123,"episodeid":1,"name":"Synthetic bulletin","description":"Synthetic provider earthquake claim","country":"Fixture","source":"NEIC","sourceid":"","alertlevel":"Orange","fromdate":"2026-10-20T11:00:00","todate":"2026-10-20T11:00:00","datemodified":"2026-10-20T11:30:00","url":{"report":"https://www.gdacs.org/report.aspx?eventid=123&eventtype=EQ"}}}]}`
	p, e := DecodeGDACS([]byte(raw), in)
	if e != nil || len(p.Records) != 1 || p.PartialReason != "provider_result_limit" {
		t.Fatal(p, e)
	}
	c := p.Records[0].Context
	if c.Report.LineageKeys[1] != "upstream:usgs-neic" || c.Report.CitedFrom != "2026-10-20T11:00:00" || c.ExtractionMethod == "" || c.Uncertainty == "" {
		t.Fatal(c)
	}
	for _, tc := range []struct{ old, new string }{{`2026-10-20T11:00:00`, `unknown`}, {`"coordinates":[46.7,24.7]`, `"coordinates":[46.7]`}, {`https://www.gdacs.org/report.aspx`, `javascript:alert(1)`}} {
		if _, e := DecodeGDACS([]byte(strings.Replace(raw, tc.old, tc.new, 1)), in); e == nil {
			t.Fatal("invalid report accepted", tc)
		}
	}
}
func TestContextHTTPQueriesNoContentAndRegistry(t *testing.T) {
	in := contextRequest()
	f := NewHTTPFetcher()
	for _, l := range []Location{{ID: "metar", SourceID: "s1", Resource: access.Resource{WorkspaceID: "p", EnvironmentID: "w", Partition: access.PublicEvidence}, Adapter: METARAdapter, URL: METARURL, MaxRequestsPerDay: 10}, {ID: "gdacs", SourceID: "s2", Resource: access.Resource{WorkspaceID: "p", EnvironmentID: "w", Partition: access.PublicEvidence}, Adapter: GDACSAdapter, URL: GDACSURL, MaxRequestsPerDay: 10}} {
		if _, e := NewRegistry([]Location{l}); e != nil {
			t.Fatal(e)
		}
		bad := l
		bad.URL = "https://foreign.example/feed"
		if _, e := NewRegistry([]Location{bad}); e == nil {
			t.Fatal("foreign endpoint")
		}
		f.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			q := r.URL.Query()
			if r.Header.Get("User-Agent") == "" || q.Has("checkpoint") || q.Has("limit") {
				t.Fatal(r.URL)
			}
			body := `{"type":"FeatureCollection","features":[]}`
			status := 200
			if l.Adapter == METARAdapter {
				if q.Get("bbox") != "24,46,25,47" || q.Get("hours") != "6" || q.Get("format") != "json" {
					t.Fatal(q)
				}
				status = 204
				body = ""
			} else if q.Get("eventlist") != "EQ" || q.Get("pageNumber") != "1" || q.Get("fromDate") != in.From.Format(time.RFC3339) {
				t.Fatal(q)
			}
			return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		p, _, e := f.Fetch(context.Background(), l, in)
		if e != nil || len(p.Records) != 0 {
			t.Fatal(p, e)
		}
	}
}
