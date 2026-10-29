package investigations

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/queryscope"
	"strings"
	"testing"
)

func TestFrozenBriefEscapesSourceAndAuthoredMarkup(t *testing.T) {
	o := Output{ID: "output-fixture", ContentDigest: "frozen-content", Manifest: OutputManifest{RendererVersion: BriefRenderer, Brief: &BriefSnapshot{Facts: []BriefFact{{Meaning: "Provider report", Text: `<img src=x onerror=alert(1)> javascript:alert(2)`}}}, Case: Revision{Title: `<script>alert(3)</script> تحليل`, Notes: `</pre><iframe src=https://unreviewed.invalid>`}}}
	o.Manifest.Case.Scope.Temporal.Interval = &queryscope.Interval{}
	o.Manifest.Case.Scope.Area.Geometry = json.RawMessage(`{"type":"MultiPolygon","coordinates":[[[[2,48],[3,48],[3,49],[2,49],[2,48]]]]}`)
	raw, e := (Service{}).renderBrief(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	html := string(raw)
	for _, bad := range []string{"<script>", "<iframe src=", "<img src=x", "<form", "<link ", "<script "} {
		if strings.Contains(html, bad) {
			t.Fatal("active source markup escaped template", bad)
		}
	}
	if !strings.Contains(html, "&lt;script&gt;") || !strings.Contains(html, "Content-Security-Policy") || !strings.Contains(html, "@media print") || !strings.Contains(html, "تحليل") {
		t.Fatal("missing safe printable multilingual content")
	}
	if !strings.Contains(html, "Coverage is unknown") {
		t.Fatal("missing truthful coverage fallback")
	}
}
