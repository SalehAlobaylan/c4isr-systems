package geoenv

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
)

type ViewingPackageInput struct {
	ManifestSHA256   string     `json:"manifestSha256"`
	ViewerSHA256     string     `json:"viewerSha256"`
	NavigationSHA256 string     `json:"navigationSha256"`
	Displays         []string   `json:"displays"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
}

type viewingTerms struct {
	InputID           string `json:"inputId"`
	OriginalAdmission string `json:"originalAdmissionId"`
	CurrentAdmission  string `json:"currentAdmissionId"`
	OriginalPolicy    string `json:"originalPolicyId"`
	TermsURL          string `json:"termsUrl"`
	Attribution       string `json:"attribution"`
	LegalBasis        string `json:"legalBasis"`
}

type ViewingPackage struct {
	Schema           string         `json:"schema"`
	ID               string         `json:"id"`
	ReleaseID        string         `json:"releaseId"`
	Name             string         `json:"name"`
	ManifestSHA256   string         `json:"manifestSha256"`
	ViewerVersion    string         `json:"viewerVersion"`
	ViewerSHA256     string         `json:"viewerSha256"`
	NavigationSHA256 string         `json:"navigationSha256"`
	IssuedAt         time.Time      `json:"issuedAt"`
	ExpiresAt        time.Time      `json:"expiresAt"`
	Displays         []Display      `json:"displays"`
	Terms            []viewingTerms `json:"terms"`
}

func (s Service) issueViewingPackage(ctx context.Context, releaseID string, in ViewingPackageInput) (ViewingPackage, error) {
	var out ViewingPackage
	if err := require(ctx, Download, false); err != nil {
		return out, err
	}
	if !digestPattern.MatchString(in.ManifestSHA256) || !digestPattern.MatchString(in.ViewerSHA256) ||
		!digestPattern.MatchString(in.NavigationSHA256) || len(in.Displays) < 1 || len(in.Displays) > 6 {
		return out, apperr.Validation("viewing package requires exact display and viewer dependencies")
	}
	principal, _, _ := access.From(ctx)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		rel, err := s.displayParentFor(ctx, tx, releaseID, false)
		if err != nil {
			return err
		}
		if rel.ManifestSHA256 != in.ManifestSHA256 {
			return apperr.Conflict("viewing package requires the exact release manifest")
		}
		var manifest Manifest
		if err := json.Unmarshal(rel.Manifest, &manifest); err != nil {
			return err
		}
		now := time.Now().UTC()
		out = ViewingPackage{Schema: "emad.viewing-issuance.v1", ID: ids.New("geoview"), ReleaseID: rel.ID, Name: rel.Name,
			ManifestSHA256: rel.ManifestSHA256, ViewerVersion: "emad-geographic-viewer-v1", ViewerSHA256: in.ViewerSHA256,
			NavigationSHA256: in.NavigationSHA256, IssuedAt: now}
		if in.ExpiresAt != nil {
			out.ExpiresAt = in.ExpiresAt.UTC()
		}
		limit := func(t time.Time) {
			if out.ExpiresAt.IsZero() || t.Before(out.ExpiresAt) {
				out.ExpiresAt = t
			}
		}
		recipes, dependencies := map[string]bool{}, map[string]bool{}
		for _, id := range in.Displays {
			d, err := scanDisplay(tx.QueryRow(ctx, displaySelect+`WHERE d.id=$1 AND d.release_id=$2 AND a.state='READY'`, id, rel.ID))
			if err != nil {
				return err
			}
			if d.ManifestSHA256 != rel.ManifestSHA256 || recipes[d.Recipe] || displayFormat(d.Recipe).media == "" {
				return apperr.Validation("select one published display per supported recipe from this release")
			}
			recipes[d.Recipe] = true
			limit(d.RetainUntil)
			out.Displays = append(out.Displays, d)
			for _, dep := range d.Inputs {
				dependencies[dep.Name] = true
			}
		}
		if !recipes["terrain-glb-v1"] && !recipes["terrain-glb-v2"] {
			return apperr.Validation("select the terrain required by the portable viewer")
		}
		attachDisplayPurpose(out.Displays)
		inputs := map[string]bool{}
		for _, member := range manifest.Members {
			if dependencies[member.Name] {
				inputs[member.InputID] = true
			}
		}
		for _, input := range manifest.Inputs {
			if !inputs[input.ID] {
				continue
			}
			policy, err := s.policyIn(ctx, tx, input, true, false)
			if err != nil {
				return err
			}
			rights := policy.Rights.OutputPermissions
			if rights.GeographicViewerExport == nil || !*rights.GeographicViewerExport {
				return apperr.Forbidden("one or more selected display sources lack portable viewing permission")
			}
			limit(input.RetainUntil)
			if policy.Rights.ExpiresAt != nil {
				limit(*policy.Rights.ExpiresAt)
			}
			out.Terms = append(out.Terms, viewingTerms{input.ID, input.AdmissionID, policy.CurrentAdmissionID,
				policy.Rights.ID, policy.Rights.TermsURL, policy.Rights.Attribution, policy.Rights.LegalBasis})
		}
		if len(out.Terms) == 0 || !out.ExpiresAt.After(now) {
			return apperr.Conflict("selected viewing rights have expired or are unavailable")
		}
		raw, err := json.Marshal(out)
		if err != nil {
			return err
		}
		audit, err := json.Marshal(map[string]any{"issuanceId": out.ID, "receiptSha256": artifacts.Hash(append(raw, '\n')),
			"displayCount": len(out.Displays), "expiresAt": out.ExpiresAt})
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events(id,occurred_at,actor_type,actor_id,action,subject_type,subject_id,correlation_id,data)
			VALUES($1,$2,'OPERATOR',$3,'environment.viewing.issued','geographic_environment_release',$4,$5,$6)`,
			ids.New("aud"), now, principal.ID, rel.ID, httpx.GetRequestID(ctx), audit)
		return err
	})
	return out, err
}
