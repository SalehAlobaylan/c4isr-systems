package geoenv

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
)

// RetainedAdmission records a steward's reviewed rights for an offline source.
// Its dedicated profile keeps review independent of the live connector head.
type RetainedAdmission struct {
	OriginalProducer    string              `json:"originalProducer"`
	UpstreamProvider    string              `json:"upstreamProvider"`
	Dataset             string              `json:"dataset"`
	ProductVersion      string              `json:"productVersion"`
	CoverageDescription string              `json:"coverageDescription"`
	Area                publicingest.Bounds `json:"area"`
	Rights              publicingest.Rights `json:"rights"`
	Enabled             bool                `json:"enabled"`
	ExpectedRevisionID  string              `json:"expectedRevisionId,omitempty"`
	IntendedUse         string              `json:"intendedUse,omitempty"`
}

func (s Service) admitRetained(ctx context.Context, sourceID string, in RetainedAdmission) (publicingest.Admission, error) {
	if err := require(ctx, Manage, false); err != nil {
		return publicingest.Admission{}, err
	}
	if err := access.Require(ctx, "sources.manage"); err != nil {
		return publicingest.Admission{}, err
	}
	if err := access.RequireSource(ctx, sourceID); err != nil {
		return publicingest.Admission{}, err
	}
	source, err := sources.NewPostgresRepository(s.Pool).Get(ctx, sourceID)
	if err != nil {
		return publicingest.Admission{}, err
	}
	if source.Type != sources.TypeExternal {
		return publicingest.Admission{}, apperr.Validation("retained intake requires a registered external source")
	}
	now := time.Now().UTC()
	input := publicingest.AdmissionInput{
		AnalystQuestion:  in.IntendedUse,
		OriginalProducer: in.OriginalProducer, UpstreamProvider: in.UpstreamProvider, Dataset: in.Dataset, ProductVersion: in.ProductVersion,
		CoverageDescription: in.CoverageDescription, Rights: in.Rights, Enabled: in.Enabled, RevisionPolicy: "content_hash",
		ConnectorID: "geographic-offline-intake", ConnectorPrincipalID: "geographic-offline-intake",
		Policy: publicingest.Policy{AOI: in.Area, LookbackSeconds: 1, FreshForSeconds: 1, UnavailableAfterSeconds: 2, MinIntervalSeconds: 86400, TimeoutSeconds: 1, MaxPages: 1, MaxRecordsPerPage: 1, MaxResponseBytes: 128, MaxRequestsPerDay: 1},
	}
	if err := input.Validate(now); err != nil {
		return publicingest.Admission{}, err
	}
	p, _, _ := access.From(ctx)
	a := publicingest.Admission{AdmissionInput: input, SourceID: sourceID, ApprovedBy: p.ID, ApprovedAt: now}
	a, err = publicingest.NewRepository(s.Pool).Admit(access.WithRetrievalProfile(ctx, "geographic-retained"), a, in.ExpectedRevisionID)
	if err == nil {
		a.ProfileKey = "geographic-retained"
		a.ProfileRevisionID = "rpr_" + a.ID
	}
	return a, err
}
