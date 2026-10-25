package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"strings"
	"testing"
	"time"
)

func setupContext(t *testing.T, f *publicFixture, report bool) {
	t.Helper()
	adapter, url, dataset, pace, lookback := publicingest.METARAdapter, publicingest.METARURL, publicingest.METARDataset, 300, 21600
	if report {
		adapter, url, dataset, pace, lookback = publicingest.GDACSAdapter, publicingest.GDACSURL, publicingest.GDACSDataset, 900, 7*86400
	}
	registry, e := publicingest.NewRegistry([]publicingest.Location{{ID: "fixture-location", SourceID: f.source, Resource: f.resource, Adapter: adapter, URL: url, MaxRequestsPerDay: 20}})
	if e != nil {
		t.Fatal(e)
	}
	for k, v := range registry {
		f.registry[k] = v
	}
	f.admission.Dataset = dataset
	f.admission.ProductVersion = "v1"
	f.admission.RevisionPolicy = "content_hash"
	f.admission.AnalystQuestion = "Which official hazard bulletins and station observations contextualize this selected region?"
	f.admission.Policy.MinIntervalSeconds = pace
	f.admission.Policy.LookbackSeconds = lookback
	f.admission.Policy.MaxResponseBytes = 1048576
	f.admit(t)
}
func TestP33StationWeatherRevisionsRightsAndNoTracks(t *testing.T) {
	f := newPublicFixture(t)
	setupContext(t, f, false)
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	temperature := 0
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		raw := fmt.Sprintf(`[{"icaoId":"OERK","obsTime":%d,"metarType":"METAR","rawOb":"SYNTHETIC STATION SAMPLE","lat":24.7,"lon":46.7,"temp":%d,"wdir":"VRB","visib":"10+"}]`, at.Unix(), temperature)
		p, e := publicingest.DecodeMETAR([]byte(raw), in)
		return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
	})
	f.execute(t)
	f.ready(t)
	if run := f.execute(t); run["duplicates"] != float64(1) {
		t.Fatal(run)
	}
	temperature = 1
	at = at.Add(time.Minute)
	f.ready(t)
	f.execute(t)
	code, b := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-context/records?kind=WEATHER&bbox=46,24,47,25", nil, nil)
	if code != 200 || b["total"] != float64(1) {
		t.Fatal(code, b)
	}
	item := b["items"].([]any)[0].(map[string]any)
	evidence := item["evidence"].(map[string]any)
	id := evidence["id"].(string)
	weather := evidence["context"].(map[string]any)["weather"].(map[string]any)
	if weather["temperatureC"] != float64(1) || weather["windDirectionDegrees"] != nil || weather["variableWind"] != true {
		t.Fatal(weather)
	}
	code, b = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-context/records/"+id+"/history", nil, nil)
	if code != 200 || b["total"] != float64(2) {
		t.Fatal(code, b)
	}
	if code, _ := scopeRequest(t, f.h, f.manager, "GET", "/api/v1/public-context/records?bbox=NaN,24,47,25", nil, nil); code != 400 {
		t.Fatal(code)
	}
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE public_context_records SET record_key='tampered' WHERE evidence_id=$1`, id); e == nil {
		t.Fatal("mutable context history")
	}
	for _, table := range []string{"tracks", "observations", "assets", "classifications"} {
		var n int
		if e := f.h.pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table+` WHERE scope_id=$1`, f.resource.ID()).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
	// Native pilot admission never enables a generic feed without a question.
	if e := access.NewStore(f.h.pool).AddGrant(access.Bootstrap(context.Background()), "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"assessments.create"}}); e != nil {
		t.Fatal(e)
	}
	if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/assessments/report-proposals", map[string]any{"reportEvidenceId": id, "conclusion": "Weather is a station sample, not a report assessment.", "method": "OPERATOR"}, nil); code != 400 {
		t.Fatal("weather entered report proposal", code)
	}
	f.admission.AnalystQuestion = strings.Repeat(" ", 40)
	if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", f.admission, nil); code != 400 {
		t.Fatal("blank question accepted", code)
	}
	f.admission.AnalystQuestion = ""
	if code, _ := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/sources/"+f.source+"/admission", f.admission, nil); code != 400 {
		t.Fatal("missing question accepted", code)
	}
}
func TestP33ReportEvidenceIndependentReviewAndLineage(t *testing.T) {
	f := newPublicFixture(t)
	setupContext(t, f, true)
	ctx := context.Background()
	store := access.NewStore(f.h.pool)
	admin := access.Bootstrap(ctx)
	if e := store.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"assessments.create", "assessments.read", "assessments.review"}}); e != nil {
		t.Fatal(e)
	}
	if _, e := f.h.pool.Exec(ctx, `INSERT INTO operators(id,name,role) VALUES('report-review-human','Independent development reviewer','supervisor')`); e != nil {
		t.Fatal(e)
	}
	if e := store.CreateHuman(admin, "report-reviewer", "report-review-human", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "assessments.read", "assessments.review"}, SourceIDs: []string{f.source}}); e != nil {
		t.Fatal(e)
	}
	_, reviewer, e := store.IssueCredential(admin, "report-reviewer", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	call := func(token, method, path string, in any, want int) map[string]any {
		t.Helper()
		code, b := scopeRequest(t, f.h, token, method, path, in, nil)
		if code != want {
			t.Fatalf("%s %s %d %v", method, path, code, b)
		}
		return b
	}
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	modified := at.Add(time.Minute)
	claim := "Synthetic automatic earthquake bulletin; impacts remain unverified."
	f.fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
		raw, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "features": []any{map[string]any{"type": "Feature", "geometry": map[string]any{"type": "Point", "coordinates": []float64{46.7, 24.7}}, "properties": map[string]any{"eventtype": "EQ", "eventid": 123, "episodeid": 1, "name": "Synthetic official bulletin", "description": claim, "country": "Fixture", "source": "NEIC", "fromdate": at.Format("2006-01-02T15:04:05"), "todate": at.Format("2006-01-02T15:04:05"), "datemodified": modified.Format("2006-01-02T15:04:05"), "url": map[string]any{"report": "https://www.gdacs.org/report.aspx?eventid=123&eventtype=EQ"}}}}})
		p, e := publicingest.DecodeGDACS(raw, in)
		return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
	})
	f.execute(t)
	list := call(f.manager, "GET", "/api/v1/public-context/records?kind=REPORT", nil, 200)
	ev := list["items"].([]any)[0].(map[string]any)["evidence"].(map[string]any)
	id := ev["id"].(string)
	if ev["context"].(map[string]any)["report"].(map[string]any)["lineageKeys"].([]any)[1] != "upstream:usgs-neic" {
		t.Fatal(ev)
	}
	proposal := map[string]any{"reportEvidenceId": id, "conclusion": "Synthetic provider claim requires confirmation against its cited upstream evidence.", "method": "AI", "modelScore": .9}
	call(f.manager, "POST", "/api/v1/assessments/imagery-proposals", proposal, 400)
	v := call(f.manager, "POST", "/api/v1/assessments/report-proposals", proposal, 201)
	assessment := v["assessmentId"].(string)
	rootID := v["id"].(string)
	if v["status"] != "PROPOSED" || v["confidence"] != nil || v["evidence"] != nil {
		t.Fatal("unreviewed claim became fact", v)
	}
	action := map[string]any{"expectedRevisionId": v["id"], "action": "review", "rationale": "Independent review of cited sources, uncertainty and shared lineage."}
	call(f.manager, "POST", "/api/v1/assessments/"+assessment+"/review", action, 403)
	v = call(reviewer, "POST", "/api/v1/assessments/"+assessment+"/review", action, 201)
	call(reviewer, "POST", "/api/v1/assessments/"+assessment+"/review", action, 409)
	action["expectedRevisionId"] = v["id"]
	action["action"] = "publish"
	call(reviewer, "POST", "/api/v1/assessments/"+assessment+"/review", action, 400)
	// Current rights revoke publication without destroying the proposal snapshot.
	f.admission.Rights.ID = "restricted-rights"
	f.admission.Rights.AllowAnalysis = false
	f.admission.Enabled = false
	f.admit(t)
	action["confidence"] = .5
	action["confidenceRationale"] = "Single shared upstream source; impacts remain unverified."
	call(reviewer, "POST", "/api/v1/assessments/"+assessment+"/review", action, 403)
	f.admission.Rights.ID = "fixture-rights-v1"
	f.admission.Rights.AllowAnalysis = true
	f.admission.Enabled = true
	f.admit(t)
	published := call(reviewer, "POST", "/api/v1/assessments/"+assessment+"/review", action, 201)
	if published["confidence"] != .5 || published["modelScore"] != .9 {
		t.Fatal(published)
	}
	claim = "Corrected synthetic provider claim, still sharing NEIC lineage."
	modified = modified.Add(time.Minute)
	f.ready(t)
	f.execute(t)
	latest := call(f.manager, "GET", "/api/v1/public-context/records?kind=REPORT", nil, 200)["items"].([]any)[0].(map[string]any)["evidence"].(map[string]any)
	if latest["id"] == id {
		t.Fatal("provider correction overwritten")
	}
	exact := call(reviewer, "GET", "/api/v1/assessments/"+assessment+"/revisions/"+published["id"].(string), nil, 200)
	if exact["reportEvidence"].(map[string]any)["contentHash"] != ev["contentHash"] {
		t.Fatal("published support drifted", exact)
	}
	hist := call(f.manager, "GET", "/api/v1/public-context/records/"+id+"/history", nil, 200)
	if hist["total"] != float64(2) {
		t.Fatal(hist)
	}
	if b := call(f.manager, "GET", "/api/v1/assessments/intelligence", nil, 200); b["total"] != float64(0) {
		t.Fatal("report leaked into imagery workflow", b)
	}
	if b := call(f.manager, "GET", "/api/v1/assessments/reports", nil, 200); b["total"] != float64(1) {
		t.Fatal(b)
	}
	restricted := scopeToken(t, f.h, "report-other-source", f.resource, []string{"operational.read", "sources.read", "assessments.read"}, []string{"unrelated"})
	for _, path := range []string{"/api/v1/public-context/records/" + id, "/api/v1/public-context/records/" + id + "/history", "/api/v1/assessments/" + assessment + "/revisions/" + rootID} {
		call(restricted, "GET", path, nil, 404)
	}
	other := scopeToken(t, f.h, "context-other-workspace", access.Resource{WorkspaceID: "other", EnvironmentID: "world", Partition: access.PublicEvidence}, []string{"operational.read", "sources.read"}, []string{f.source})
	call(other, "GET", "/api/v1/public-context/records/"+id, nil, 404)
	exercise := scopeToken(t, f.h, "context-exercise-reader", access.LegacyResource(), []string{"operational.read", "sources.read"}, nil)
	call(exercise, "GET", "/api/v1/public-context/records", nil, 403)
	proposal["expectedRevisionId"] = published["id"]
	proposal["reportEvidenceId"] = latest["id"]
	draft := call(f.manager, "POST", "/api/v1/assessments/"+assessment+"/revisions", proposal, 201)
	call(f.manager, "POST", "/api/v1/assessments/"+assessment+"/revisions", proposal, 409)
	if draft["status"] != "DRAFT" {
		t.Fatal(draft)
	}
	generic := call(f.manager, "GET", "/api/v1/assessments/"+assessment, nil, 200)
	if generic["publishedRevisionId"] != published["id"] {
		t.Fatal("published pointer lost", generic)
	}
	for _, table := range []string{"tracks", "observations", "assets", "classifications"} {
		var n int
		if e := f.h.pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE scope_id=$1`, f.resource.ID()).Scan(&n); e != nil || n != 0 {
			t.Fatal(table, n, e)
		}
	}
}
