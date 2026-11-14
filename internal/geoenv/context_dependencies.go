package geoenv

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
)

type ContextDisplay struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

type ContextDependency struct {
	ReleaseID      string           `json:"releaseId"`
	ManifestSHA256 string           `json:"manifestSha256"`
	Displays       []ContextDisplay `json:"displays"`
}

type ContextTerms struct {
	SourceID           string     `json:"sourceId"`
	AdmissionID        string     `json:"admissionId"`
	CurrentAdmissionID string     `json:"currentAdmissionId"`
	RightsID           string     `json:"rightsId"`
	Attribution        string     `json:"attribution"`
	TermsURL           string     `json:"termsUrl"`
	LegalBasis         string     `json:"legalBasis"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
}

func (s Service) ContextDependencies(ctx context.Context, tx pgx.Tx, input ContextDependency) (Manifest, time.Time, []ContextTerms, error) {
	var manifest Manifest
	if err := require(ctx, Read, false); err != nil {
		return manifest, time.Time{}, nil, err
	}
	rel, err := s.displayParentFor(ctx, tx, input.ReleaseID, false)
	if err != nil || rel.ManifestSHA256 != input.ManifestSHA256 {
		return manifest, time.Time{}, nil, apperr.Conflict("exact released environment is unavailable")
	}
	if err = json.Unmarshal(rel.Manifest, &manifest); err != nil {
		return manifest, time.Time{}, nil, err
	}
	bundle, err := s.asset(ctx, rel.BundleID)
	if err != nil {
		return manifest, time.Time{}, nil, err
	}
	expires := bundle.RetainUntil
	for _, ref := range input.Displays {
		d, err := scanDisplay(tx.QueryRow(ctx, displaySelect+`WHERE d.id=$1 AND d.release_id=$2 AND a.state='READY'`, ref.ID, rel.ID))
		if err != nil || d.SHA256 != ref.SHA256 || d.ManifestSHA256 != rel.ManifestSHA256 || !d.RetainUntil.After(time.Now()) {
			return manifest, time.Time{}, nil, apperr.Conflict("exact display dependency is unavailable")
		}
		if d.RetainUntil.Before(expires) {
			expires = d.RetainUntil
		}
	}
	terms := make([]ContextTerms, 0, len(manifest.Inputs))
	for _, asset := range manifest.Inputs {
		p, err := s.policyIn(ctx, tx, asset, true, false)
		if err != nil {
			return manifest, time.Time{}, nil, err
		}
		if allowed := p.Rights.OutputPermissions.GeographicContextExport; allowed == nil || !*allowed {
			return manifest, time.Time{}, nil, apperr.Forbidden("geographic source lacks context handoff permission")
		}
		if asset.RetainUntil.Before(expires) {
			expires = asset.RetainUntil
		}
		if p.Rights.ExpiresAt != nil && p.Rights.ExpiresAt.Before(expires) {
			expires = *p.Rights.ExpiresAt
		}
		if !slices.ContainsFunc(terms, func(t ContextTerms) bool { return t.SourceID == asset.SourceID && t.AdmissionID == asset.AdmissionID }) {
			terms = append(terms, ContextTerms{asset.SourceID, asset.AdmissionID, p.CurrentAdmissionID,
				p.Rights.ID, p.Rights.Attribution, p.Rights.TermsURL, p.Rights.LegalBasis, p.Rights.ExpiresAt})
		}
	}
	return manifest, expires, terms, nil
}
