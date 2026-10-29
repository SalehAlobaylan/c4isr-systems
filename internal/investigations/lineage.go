package investigations

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
)

type sourcePolicy struct{ SourceID, AdmissionID string }

// Only typed, frozen lineage positions are traversed. IDs in authored prose
// never manufacture source access or rights. Admission IDs resolve to their
// actual source under RLS, including secondary inputs of derived artifacts.
func (s Service) policies(ctx context.Context, tx pgx.Tx, ref Evidence) ([]sourcePolicy, error) {
	query := ""
	switch ref.Kind {
	case "artifact":
		query = `SELECT document FROM artifacts WHERE id=$1`
	case "observation":
		query = `SELECT document FROM public_evidence WHERE id=$1`
	case "analysis", "job":
		query = `SELECT input FROM imagery_jobs WHERE id=$1`
	case "capture":
		query = `SELECT input FROM imagery_capture_jobs WHERE id=$1`
	case "assessment":
		query = `SELECT document FROM assessment_revisions WHERE id=$1`
	default:
		return nil, nil
	}
	var raw []byte
	if e := tx.QueryRow(ctx, query, ref.ID).Scan(&raw); e != nil {
		return nil, e
	}
	var doc any
	if e := json.Unmarshal(raw, &doc); e != nil {
		return nil, e
	}
	ids := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if id, ok := x["admissionId"].(string); ok && id != "" {
				ids[id] = true
			}
			if deps, ok := x["dependencyAdmissionIds"].([]any); ok {
				for _, v := range deps {
					if id, ok := v.(string); ok && id != "" {
						ids[id] = true
					}
				}
			}
			for _, key := range []string{"scene", "scenes", "artifacts", "input", "evidence", "reportEvidence"} {
				if child, ok := x[key]; ok {
					walk(child)
				}
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(doc)
	if len(ids) > 512 {
		return nil, apperr.Conflict("source lineage exceeds the supported bound")
	}
	keys := []string{}
	for id := range ids {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	out := []sourcePolicy{}
	for _, id := range keys {
		var source string
		if e := tx.QueryRow(ctx, `SELECT source_id FROM source_admissions WHERE id=$1`, id).Scan(&source); e != nil {
			return nil, apperr.Conflict("a frozen source policy is unavailable")
		}
		if e := access.RequireSource(ctx, source); e != nil {
			return nil, e
		}
		out = append(out, sourcePolicy{source, id})
	}
	return out, nil
}
