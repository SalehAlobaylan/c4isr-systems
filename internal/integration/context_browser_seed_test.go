package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
	"time"
)

func seedContextBrowser(t *testing.T, h *harness, locations publicingest.Registry, fetcher *fixtureFetcher, r access.Resource, headers map[string]string) map[string]any {
	t.Helper()
	out := map[string]any{}
	ctx := context.Background()
	store := access.NewStore(h.pool)
	admin := access.Bootstrap(ctx)
	for _, kind := range []string{"weather", "report"} {
		call := func(token, method, path string, in any, want int, hs map[string]string) map[string]any {
			t.Helper()
			c, b := scopeRequest(t, h, token, method, path, in, hs)
			if c != want {
				t.Fatalf("context browser seed %s %d %v", path, c, b)
			}
			return b
		}
		source := call(integrationToken, "POST", "/api/v1/sources", map[string]any{"name": "Synthetic official " + kind + " fixture", "type": "external"}, 201, headers)["id"].(string)
		principal := "browser-context-" + kind
		worker := scopeToken(t, h, principal, r, []string{"operational.read", "sources.read", "connectors.execute"}, []string{source})
		adapter, url, dataset, pace := publicingest.METARAdapter, publicingest.METARURL, publicingest.METARDataset, 300
		if kind == "report" {
			adapter, url, dataset, pace = publicingest.GDACSAdapter, publicingest.GDACSURL, publicingest.GDACSDataset, 900
		}
		registry, e := publicingest.NewRegistry([]publicingest.Location{{ID: principal, SourceID: source, Resource: r, Adapter: adapter, URL: url, MaxRequestsPerDay: 10}})
		if e != nil {
			t.Fatal(e)
		}
		for k, v := range registry {
			locations[k] = v
		}
		a := publicingest.AdmissionInput{AnalystQuestion: "Which official bulletins and station weather contextualize this selected region?", OriginalProducer: "Synthetic official " + kind + " fixture", UpstreamProvider: "Local native-format fixture", ConnectorID: principal, ConnectorPrincipalID: principal, Dataset: dataset, ProductVersion: "v1", CoverageDescription: "Synthetic records in bounded AOI; not live conditions", RevisionPolicy: "content_hash", Enabled: true, Rights: publicingest.Rights{ID: "owned-context-fixture", LegalBasis: "Synthetic test data owned by test", TermsURL: "https://provider.example.invalid/terms", Attribution: "EMAD synthetic context fixture", RetainRaw: true, AllowAnalysis: true}, Policy: publicingest.Policy{AOI: publicingest.Bounds{West: 39, South: 21, East: 40, North: 22}, LookbackSeconds: 21600, FreshForSeconds: 3600, UnavailableAfterSeconds: 7200, MinIntervalSeconds: pace, TimeoutSeconds: 5, MaxPages: 1, MaxRecordsPerPage: 10, MaxResponseBytes: 1048576, MaxRequestsPerDay: 10}}
		call(integrationToken, "POST", "/api/v1/sources/"+source+"/admission", a, 201, headers)
		at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		for version := 0; version < 2; version++ {
			fetcher.delegate = nativeFixtureFunc(func(_ context.Context, l publicingest.Location, in publicingest.FetchRequest) (publicingest.Page, publicingest.Retrieval, error) {
				var raw []byte
				var p publicingest.Page
				var e error
				if kind == "weather" {
					raw = []byte(fmt.Sprintf(`[{"icaoId":"OEJN","obsTime":%d,"metarType":"METAR","rawOb":"SYNTHETIC OEJN VARIABLE WIND SAMPLE","lat":21.5,"lon":39.2,"temp":0,"dewp":null,"wdir":"VRB","wspd":10,"visib":"10+","altim":1013}]`, at.Add(time.Duration(version)*time.Minute).Unix()))
					p, e = publicingest.DecodeMETAR(raw, in)
				} else {
					raw, _ = json.Marshal(map[string]any{"type": "FeatureCollection", "features": []any{map[string]any{"type": "Feature", "geometry": map[string]any{"type": "Point", "coordinates": []float64{39.2, 21.5}}, "properties": map[string]any{"eventtype": "EQ", "eventid": 987654, "episodeid": version + 1, "name": "Synthetic official earthquake bulletin", "description": fmt.Sprintf("Synthetic provider claim revision %d; impacts not verified.", version+1), "country": "Fixture region", "source": "NEIC", "alertlevel": "Orange", "fromdate": at.Format("2006-01-02T15:04:05"), "todate": at.Format("2006-01-02T15:04:05"), "datemodified": at.Add(time.Duration(version+1) * time.Minute).Format("2006-01-02T15:04:05"), "url": map[string]any{"report": "https://www.gdacs.org/report.aspx?eventid=987654&eventtype=EQ"}}}}})
					p, e = publicingest.DecodeGDACS(raw, in)
				}
				return p, publicingest.Retrieval{LocationID: l.ID, HTTPStatus: 200}, e
			})
			run := call(worker, "POST", "/api/v1/sources/"+source+"/connector-runs", nil, 201, nil)
			if run["newEvidence"] != float64(1) {
				t.Fatal(run)
			}
			if _, e := h.pool.Exec(ctx, `UPDATE source_connector_state SET next_attempt_at=now()-interval '1 hour' WHERE source_id=$1`, source); e != nil {
				t.Fatal(e)
			}
			if _, e := h.pool.Exec(ctx, `UPDATE provider_budget_state SET next_at='epoch' WHERE group_key=$1`, "location:"+principal); e != nil {
				t.Fatal(e)
			}
		}
		fetcher.delegate = nil
		list := call(integrationToken, "GET", "/api/v1/public-context/records", nil, 200, headers)
		for _, v := range list["items"].([]any) {
			ev := v.(map[string]any)["evidence"].(map[string]any)
			if ev["sourceId"] == source {
				out[kind+"Id"] = ev["id"]
			}
		}
		if kind == "report" {
			if _, e := h.pool.Exec(ctx, `INSERT INTO operators(id,name,role) VALUES('browser-review-human','Independent development reviewer','supervisor') ON CONFLICT DO NOTHING`); e != nil {
				t.Fatal(e)
			}
			if e := store.CreateHuman(admin, "browser-report-reviewer", "browser-review-human", access.Grant{Resource: r, Permissions: []string{"operational.read", "sources.read", "assessments.read", "assessments.review"}, SourceIDs: []string{source}}); e != nil {
				t.Fatal(e)
			}
			_, token, e := store.IssueCredential(admin, "browser-report-reviewer", r, time.Now().Add(time.Hour))
			if e != nil {
				t.Fatal(e)
			}
			out["reviewerToken"] = token
		}
	}
	return out
}
