package publicingest

import (
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"
)

func stacFixture(t *testing.T) ([]byte, FetchRequest) {
	t.Helper()
	raw, e := os.ReadFile("../../testdata/imagery/cdse-two-scenes.json")
	if e != nil {
		t.Fatal(e)
	}
	from, _ := time.Parse(time.RFC3339, "2025-02-01T00:00:00Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-28T23:59:59Z")
	return raw, FetchRequest{From: from, To: to, Admission: Admission{AdmissionInput: AdmissionInput{Dataset: STACDataset, ProductVersion: "stac-1.1-l2a", RevisionPolicy: "content_hash", Policy: Policy{AOI: Bounds{39.15, 21.45, 39.2, 21.5}, MaxRecordsPerPage: 2, MaxResponseBytes: 1024 * 1024}}}}
}
func TestRecordedCDSESTACSceneSemanticsCursorAndFixedEndpoint(t *testing.T) {
	raw, in := stacFixture(t)
	p, e := DecodeSTAC(raw, in)
	if e != nil || len(p.Records) != 2 || !p.HasMore {
		t.Fatalf("native STAC decode %d %v", len(p.Records), e)
	}
	for _, r := range p.Records {
		if r.Observation != nil || len(r.Scene) == 0 || len(r.OriginalRecord) == 0 {
			t.Fatal("scene became an operational observation or lost provenance")
		}
	}
	in.Checkpoint = p.Checkpoint
	href, e := stacURL(Location{URL: STACURL}, in)
	if e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(href)
	if u.Host != "stac.dataspace.copernicus.eu" || u.Query().Get("token") == "" || u.Query().Get("bbox") != "39.15,21.45,39.2,21.5" {
		t.Fatal("trusted URL/cursor mismatch")
	}
	in.From = in.From.Add(time.Second)
	if _, e = stacURL(Location{URL: STACURL}, in); e == nil {
		t.Fatal("partial cursor accepted a different interval")
	}
}
func TestSTACRejectsMissingAssetsAndUntrustedPagination(t *testing.T) {
	raw, in := stacFixture(t)
	for _, name := range []string{"asset", "pagination", "cloud", "time"} {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			_ = json.Unmarshal(raw, &v)
			f := v["features"].([]any)[0].(map[string]any)
			switch name {
			case "asset":
				delete(f["assets"].(map[string]any), "SCL_20m")
			case "pagination":
				v["links"] = []any{map[string]any{"rel": "next", "href": "https://127.0.0.1/private?token=x"}}
			case "cloud":
				f["properties"].(map[string]any)["eo:cloud_cover"] = nil
			case "time":
				f["properties"].(map[string]any)["datetime"] = "2030-01-01T00:00:00Z"
			}
			b, _ := json.Marshal(v)
			if _, e := DecodeSTAC(b, in); e == nil {
				t.Fatal("invalid catalogue accepted")
			}
		})
	}
}
