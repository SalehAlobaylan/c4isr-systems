package investigations

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"image/png"
	"math"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/environmentprep"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
)

const BriefRenderer = "emad.analyst-brief.v1"

type BriefFact struct {
	Details  string   `json:"details,omitempty"`
	Evidence Evidence `json:"evidence"`
	Meaning  string   `json:"meaning"`
	Text     string   `json:"text"`
}
type BriefSnapshot struct {
	EvaluatedAt time.Time         `json:"evaluatedAt"`
	Facts       []BriefFact       `json:"facts"`
	Receipts    []json.RawMessage `json:"receipts"`
	Limitations []string          `json:"limitations"`
}

func (s Service) freezeBrief(ctx context.Context, tx pgx.Tx, c Case) (BriefSnapshot, error) {
	b := BriefSnapshot{EvaluatedAt: time.Now().UTC(), Facts: []BriefFact{}, Receipts: []json.RawMessage{}, Limitations: []string{"The outline is a schematic WGS84 study boundary, without a live basemap or legal-boundary claim.", "Provider reports, derived indicators and authored findings have different meanings; a report is not independently established fact.", "Only overlapping receipt footprints/windows are included; a complete source query does not establish complete study-area coverage, observational completeness or factual absence.", "External evidence can become unavailable. No live evidence is fetched when opening this historical brief.", "This frozen content is a draft until a separate exact-render approval attestation exists."}}
	refs := append([]Evidence{}, c.Revision.Evidence...)
	for _, f := range c.Revision.Findings {
		refs = append(refs, f.Evidence...)
	}
	seen := map[string]bool{}
	for _, r := range refs {
		if seen[r.Kind+r.ID] {
			continue
		}
		seen[r.Kind+r.ID] = true
		fact := BriefFact{Evidence: r, Meaning: "Exact retained reference", Text: r.ID}
		var raw []byte
		query := ""
		switch r.Kind {
		case "observation":
			fact.Meaning = "Provider report (not independent confirmation)"
			query = `SELECT document FROM public_evidence WHERE id=$1`
		case "analysis":
			fact.Meaning = "Derived indicator (not a human conclusion)"
			query = `SELECT result FROM imagery_jobs WHERE id=$1`
		case "assessment":
			fact.Meaning = "Human assessment at the cited revision"
			query = `SELECT document FROM assessment_revisions WHERE id=$1`
		}
		if query != "" {
			if e := tx.QueryRow(ctx, query, r.ID).Scan(&raw); e != nil {
				return b, e
			}
			var doc map[string]any
			_ = json.Unmarshal(raw, &doc)
			if r.Kind == "analysis" {
				var result struct {
					ValidCells   int      `json:"validCells"`
					DeclineCells int      `json:"declineCells"`
					Limitations  []string `json:"limitations"`
				}
				_ = json.Unmarshal(raw, &result)
				fact.Text = fmt.Sprintf("Derived comparison: %d jointly eligible grid cells; %d cells met the configured decline criterion. Counts describe an algorithmic indicator, not confirmed environmental damage.", result.ValidCells, result.DeclineCells)
				fact.Details = strings.Join(result.Limitations, " · ")
			} else if r.Kind == "observation" {
				var evidence publicingest.Evidence
				_ = json.Unmarshal(raw, &evidence)
				if evidence.Observation != nil {
					report := evidence.Observation
					fact.Text = report.Title
					properties, _ := json.Marshal(report.Properties)
					quality, _ := json.Marshal(report.Quality)
					fact.Details = fmt.Sprintf("Reported acquisition/observation time %s · semantic kind %s · geometry role %s · source-reported properties (original units/method): %s · provider quality: %s", report.ObservedAt.Format(time.RFC3339), report.SemanticKind, report.GeometryRole, properties, quality)
				} else {
					fact.Text = evidence.ProviderRecordID
					fact.Details = "No normalized observation fields were retained in this exact revision; source identity alone establishes no finding."
				}
			} else {
				for _, k := range []string{"conclusion", "title", "providerRecordId"} {
					if v, ok := doc[k].(string); ok && v != "" {
						fact.Text = v
						break
					}
				}
			}
		}
		if len(fact.Text)+len(fact.Details) > 40000 {
			return b, apperr.Validation("brief evidence summary exceeds the supported bound")
		}
		b.Facts = append(b.Facts, fact)
	}
	rows, e := tx.Query(ctx, `SELECT rr.document FROM retrieval_receipts rr WHERE rr.source_id IN(SELECT jsonb_array_elements_text(required_sources) FROM investigations WHERE id=$1) AND rr.document->'aoi' IS NOT NULL AND ST_Intersects(ST_MakeEnvelope((rr.document#>>'{aoi,west}')::double precision,(rr.document#>>'{aoi,south}')::double precision,(rr.document#>>'{aoi,east}')::double precision,(rr.document#>>'{aoi,north}')::double precision,4326),ST_SetSRID(ST_GeomFromGeoJSON($4::jsonb),4326)) AND (rr.document->>'from')::timestamptz<$3 AND (rr.document->>'to')::timestamptz>$2 ORDER BY rr.document->>'completedAt' DESC,rr.run_id LIMIT 65`, c.ID, c.Revision.Scope.Temporal.Interval.From, c.Revision.Scope.Temporal.Interval.To, c.Revision.Scope.Area.Geometry)
	if e != nil {
		return b, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if e := rows.Scan(&raw); e != nil {
			return b, e
		}
		b.Receipts = append(b.Receipts, raw)
	}
	if len(b.Receipts) > 64 {
		b.Receipts = b.Receipts[:64]
		b.Limitations = append(b.Limitations, "Only the latest 64 intersecting-period source receipts are included; coverage is not exhaustive.")
	}
	return b, rows.Err()
}

type briefFigure struct {
	Source  template.URL
	Caption string
}
type briefView struct {
	CoverageRows []publicingest.Receipt
	Output       Output
	Snapshot     BriefSnapshot
	Outline      template.HTML
	Figures      []briefFigure
	Manifest     string
	Coverage     string
}

var briefTemplate = template.Must(template.New("brief").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"><title>{{.Output.Manifest.Case.Title}}</title><style>body{max-width:58rem;margin:2rem auto;padding:0 1rem;font:17px/1.6 system-ui,sans-serif;color:#172b32;background:white}h1,h2,h3{line-height:1.2}section{margin:2rem 0}p,li,pre{overflow-wrap:anywhere}pre{white-space:pre-wrap;font-size:.8rem}svg,img{max-width:100%;height:auto}figure{margin:1rem 0}figcaption,small{font-size:.85rem}header{border-bottom:2px solid #1d6172}.notice{border-inline-start:4px solid #1d6172;padding:1rem;background:#f0f6f7}dt{font-weight:bold}dd{margin:0 0 1rem}article{border-bottom:1px solid #cbd5d9}@media print{.reproduction details{display:none}body{max-width:none;margin:0;font-size:11pt}h2,h3{break-after:avoid}figure,article{break-inside:avoid}details>summary{display:none}details{display:block}details>pre{display:block}}</style></head><body><header><p>EMAD · Frozen analytical brief · {{.Output.Manifest.Purpose}}</p><h1 dir="auto">{{.Output.Manifest.Case.Title}}</h1><p dir="auto">{{.Output.Manifest.Case.Question}}</p><p class="notice">Frozen content: unreviewed unless accompanied by its matching independent approval attestation. Review status is separate and bound to this exact output/render hash. This document does not grant distribution permission.</p><p>Case revision {{.Output.RevisionID}} · Content manifest {{.Output.ContentDigest}}</p></header><main><section aria-labelledby="study"><h2 id="study">Area and period</h2><p dir="auto">{{.Output.Manifest.Case.Scope.Area.Name}}</p>{{.Outline}}<p>Longitude/latitude in WGS84. North is up; this schematic uses unprojected angular coordinates and is not a distance measurement.</p><p>{{.Output.Manifest.Case.Scope.Temporal.Mode}} · {{.Output.Manifest.Case.Scope.Temporal.Interval.From}} to {{.Output.Manifest.Case.Scope.Temporal.Interval.To}} · end exclusive · UTC</p><p>Scope {{.Output.Manifest.Case.Scope.ScopeHash}}</p></section><section><h2>Authored notes and findings</h2><p dir="auto" style="white-space:pre-wrap">{{.Output.Manifest.Case.Notes}}</p>{{range .Output.Manifest.Case.Findings}}<article><h3 dir="auto">{{.Title}}</h3><p>Human-authored finding · {{.Purpose}} · {{.RevisionID}}</p><p dir="auto">{{.Conclusion}}</p><dl><dt>Confidence and rationale</dt><dd dir="auto">{{.Confidence}}</dd><dt>Uncertainty and alternatives</dt><dd dir="auto">{{.Uncertainty}}</dd></dl><p>Original scope {{.ScopeHash}}</p><ul>{{range .Evidence}}<li>{{.Kind}} · {{.ID}} · SHA-256 {{.SHA256}}</li>{{end}}</ul></article>{{else}}<p>No authored findings were included.</p>{{end}}</section><section><h2>Retained figures and comparisons</h2>{{range .Figures}}<figure><img src="{{.Source}}" alt="Explicitly included retained source figure"><figcaption>{{.Caption}}</figcaption></figure>{{else}}<p>No retained figures were declared for inclusion. No live screenshot or replacement image was generated.</p>{{end}}</section><section><h2>Coverage at the frozen cutoff</h2><p>Evaluated {{.Snapshot.EvaluatedAt}}. Empty or partial retrieval does not establish absence.</p><ul>{{range .CoverageRows}}<li>Source {{.SourceID}} · {{.Status}} · {{.From}} to {{.To}} · {{.RetainedRecords}} retained records · {{.RejectedRecords}} rejected · {{.UnpublishedRecords}} unpublished · provider truncation: {{.ProviderTruncated}}. Source query complete: {{.QueryComplete}}. This is not a claim of observational completeness.</li>{{end}}</ul>{{if not .CoverageRows}}<p>{{.Coverage}}</p>{{end}}</section><section><h2>Exact evidence and meaning</h2>{{range .Snapshot.Facts}}<article><h3>{{.Meaning}}</h3><p dir="auto">{{.Text}}</p>{{if .Details}}<p dir="auto">{{.Details}}</p>{{end}}<p>{{.Evidence.Kind}} · {{.Evidence.ID}} · source {{.Evidence.SourceID}} · SHA-256 {{.Evidence.SHA256}}</p></article>{{end}}</section><section><h2>Attribution, omissions and limits</h2><ul>{{range .Output.Manifest.Dependencies}}<li>{{.SourceID}} · {{.Rights.Attribution}} · policy {{.CurrentPolicyID}} evaluated {{.EvaluatedAt}}</li>{{end}}{{range .Snapshot.Limitations}}<li>{{.}}</li>{{end}}{{range .Output.Manifest.Omissions}}<li>{{.}}</li>{{end}}</ul><p>Retain until {{.Output.RetainUntil}}. Rendering does not extend input retention.</p></section><section class="reproduction"><h2>Reproduction manifest</h2><p>Exact identities and permitted inclusions are frozen below. Source originals not explicitly permitted for raw distribution remain external dependencies.</p><details><summary>Inspect exact machine-readable reproduction metadata</summary><pre>{{.Manifest}}</pre></details></section></main></body></html>`))

func outline(raw json.RawMessage) template.HTML {
	var g struct {
		Type        string          `json:"type"`
		Coordinates [][][][]float64 `json:"coordinates"`
	}
	if json.Unmarshal(raw, &g) != nil || g.Type != "MultiPolygon" {
		return "<p>Study boundary unavailable in this renderer; use the exact geometry in the manifest.</p>"
	}
	west, south, east, north := 180., 90., -180., -90.
	count := 0
	for _, p := range g.Coordinates {
		for _, r := range p {
			for _, c := range r {
				if len(c) != 2 || math.IsNaN(c[0]) || math.IsNaN(c[1]) {
					return ""
				}
				west = math.Min(west, c[0])
				east = math.Max(east, c[0])
				south = math.Min(south, c[1])
				north = math.Max(north, c[1])
				count++
			}
		}
	}
	if count > 20000 || east <= west || north <= south {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 320" role="img" aria-label="Frozen study boundary; north up; schematic WGS84 coordinates"><rect width="640" height="320" fill="#f0f6f7"/>`)
	for _, p := range g.Coordinates {
		b.WriteString(`<path fill="#d0e6eb" stroke="#1d6172" stroke-width="2" fill-rule="evenodd" d="`)
		for _, r := range p {
			for i, c := range r {
				op := "L"
				if i == 0 {
					op = "M"
				}
				fmt.Fprintf(&b, "%s%.3f %.3f ", op, 20+600*(c[0]-west)/(east-west), 300-280*(c[1]-south)/(north-south))
			}
			b.WriteString("Z ")
		}
		b.WriteString(`"/>`)
	}
	fmt.Fprintf(&b, `<text x="20" y="18" font-size="12">N ↑ · %.5f° to %.5f° longitude · %.5f° to %.5f° latitude</text></svg>`, west, east, south, north)
	return template.HTML(b.String())
}

func (s Service) renderBrief(ctx context.Context, o Output) ([]byte, error) {
	if o.Manifest.Brief == nil || o.Manifest.RendererVersion != BriefRenderer {
		return nil, apperr.Conflict("unsupported frozen brief renderer")
	}
	v := briefView{Output: o, Snapshot: *o.Manifest.Brief, Outline: outline(o.Manifest.Case.Scope.Area.Geometry), Figures: []briefFigure{}}
	doc, _ := json.MarshalIndent(o.Manifest, "", "  ")
	v.Manifest = string(doc)
	coverage, _ := json.MarshalIndent(o.Manifest.Brief.Receipts, "", "  ")
	v.Coverage = string(coverage)
	for _, raw := range o.Manifest.Brief.Receipts {
		var receipt publicingest.Receipt
		if e := json.Unmarshal(raw, &receipt); e != nil {
			return nil, e
		}
		v.CoverageRows = append(v.CoverageRows, receipt)
	}
	if len(o.Manifest.Brief.Receipts) == 0 {
		v.Coverage = "No intersecting-period receipt was retained in this snapshot. Coverage is unknown."
	}
	total := 0
	for _, a := range o.Manifest.Assets {
		if a.MediaType != "image/png" {
			return nil, apperr.Validation("brief figures require explicitly included PNG assets")
		}
		_, raw, e := artifacts.NewService(s.Pool, s.Store).Read(ctx, a.ID)
		if e != nil {
			return nil, e
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 4<<20 {
			return nil, apperr.Validation("invalid bounded retained PNG figure")
		}
		if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
			return nil, apperr.Validation("invalid retained PNG bytes")
		}
		total += len(raw)
		if total > 4<<20 {
			return nil, apperr.Validation("brief figure budget exceeded")
		}
		v.Figures = append(v.Figures, briefFigure{template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)), fmt.Sprintf("Retained figure · %s · source %s · created %s · SHA-256 %s · %s · Acquisition date, units and method must be interpreted from its exact cited analysis/source manifest; missing metadata is not inferred.", a.ID, a.SourceID, a.CreatedAt.Format(time.RFC3339), a.SHA256, a.Rights.Attribution)})
	}
	var b bytes.Buffer
	if e := briefTemplate.Execute(&b, v); e != nil {
		return nil, e
	}
	if b.Len() > 8<<20 {
		return nil, apperr.Validation("rendered brief exceeds 8 MiB")
	}
	return b.Bytes(), nil
}
func (s Service) BriefBundle(ctx context.Context, id string) ([]byte, error) {
	o, html, e := s.ReadOutput(ctx, id, true)
	if e != nil {
		return nil, e
	}
	if o.Manifest.Brief == nil || o.MediaType != "text/html" {
		return nil, apperr.Validation("not a rendered brief")
	}
	review, e := s.latestReview(ctx, o.CaseID)
	if e != nil {
		return nil, e
	}
	if review == nil || review.Action != "APPROVED" || review.OutputID != o.ID || review.RenderSHA256 != o.RenderSHA256 || review.OutputContentDigest != o.ContentDigest {
		return nil, apperr.Conflict("approval changed during packaging")
	}
	if e := s.checkOutput(ctx, nil, o, true, true); e != nil {
		return nil, e
	}
	var human bytes.Buffer
	approvalTemplate := template.Must(template.New("approval").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"><title>Independent approval attestation</title><style>body{max-width:50rem;margin:2rem auto;padding:1rem;font:17px/1.6 system-ui}p{overflow-wrap:anywhere}</style></head><body><h1>Independent approval attestation</h1><p>{{.Action}} by {{.ActorHumanID}} at {{.RecordedAt}}</p><p>Case revision {{.RevisionID}}</p><p>Output {{.OutputID}}</p><p>Content manifest {{.OutputContentDigest}}</p><p>Exact rendered SHA-256 {{.RenderSHA256}}</p><p dir="auto">{{.Reason}}</p><p>This is a historical attestation. Source support and permissions were checked for distribution; they can change later. The frozen figures/text were not rerendered when approval was recorded.</p><p><a href="brief.html">Open the exact frozen brief</a>. Verify its hash against this attestation when transferring the package. Machine-readable details are in approval.json and manifest.json.</p></body></html>`))
	if e := approvalTemplate.Execute(&human, review); e != nil {
		return nil, e
	}
	attestation, _ := json.Marshal(review)
	manifest, _ := json.Marshal(o.Manifest)
	return environmentprep.Archive(map[string][]byte{"brief.html": html, "manifest.json": manifest, "approval.json": attestation, "approval.html": human.Bytes()}, 16<<20)
}

// Reader data is always fetched through ReadOutput; the UI embeds returned HTML
// in a sandbox without scripts, forms, navigation or same-origin authority.
