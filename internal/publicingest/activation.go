package publicingest

import (
	"context"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
)

type Activation struct {
	GeographicRetention *GeographicRetention     `json:"geographicRetention,omitempty"`
	ProviderCredits     *ProviderCredits         `json:"providerCredits,omitempty"`
	SourceID            string                   `json:"sourceId"`
	State               string                   `json:"state"`
	Reason              string                   `json:"reason"`
	NextAction          string                   `json:"nextAction"`
	Health              sources.IngestionSummary `json:"health"`
}

type GeographicRetention struct {
	Readable        bool `json:"readable"`
	AssetsAvailable bool `json:"assetsAvailable"`
}

type GeographicRetentionReader interface {
	RetainedSource(ctx context.Context, source string) (*GeographicRetention, error)
}

type ProviderCredits struct {
	Bucket     string     `json:"bucket"`
	Remaining  *int64     `json:"remaining"`
	ReportedAt *time.Time `json:"reportedAt"`
}

// Activation reports server bindings and the availability of retained geographic inputs.
// Registration metadata can explain a missing prerequisite, never enable access.
func (s *Service) Activation(ctx context.Context, id string) (Activation, error) {
	out := Activation{SourceID: id, State: "UNAVAILABLE", Reason: "No configured retrieval binding", NextAction: "Configure and admit a source binding on the server."}
	if err := requirePublic(ctx, id, "sources.read"); err != nil {
		return out, err
	}
	src, err := s.sources.Get(ctx, id)
	if err != nil {
		return out, err
	}
	out.Health, err = s.Health(ctx, id)
	if err != nil {
		return out, err
	}
	if s.retained != nil {
		out.GeographicRetention, err = s.retained.RetainedSource(ctx, id)
		if err != nil {
			return out, err
		}
	}
	if src.Metadata["category"] == "GEOGRAPHIC_REFERENCE" {
		var count int
		if err = db.Scoped(s.repo.pool).QueryRow(ctx, `SELECT count(*) FROM reference_datasets WHERE source_id=$1`, id).Scan(&count); err != nil {
			return out, err
		}
		if count > 0 {
			out.State = "RETAINED"
			out.Reason = "Versioned geographic reference artifacts are installed; they are not a live observation feed."
			out.NextAction = "Inspect layer provenance and coverage."
			return out, nil
		}
	}
	resource, _ := access.ResourceFrom(ctx)
	for _, loc := range s.locations {
		if loc.SourceID == id && loc.Resource == resource {
			if loc.GateState != "" {
				out.State = loc.GateState
				out.Reason = "Provider access is not activated in trusted server configuration."
				out.NextAction = loc.GateNextAction
				return out, nil
			}
			out.State = "CONFIGURED"
			if loc.Adapter == OpenSkyAdapter {
				out.ProviderCredits = &ProviderCredits{Bucket: "states"}
				runs, _, err := s.repo.Runs(ctx, id, 1, 0)
				if err != nil {
					return out, err
				}
				if len(runs) > 0 && runs[0].Retrieval != nil && runs[0].Retrieval.CreditsRemaining != nil {
					out.ProviderCredits.Remaining = runs[0].Retrieval.CreditsRemaining
					out.ProviderCredits.ReportedAt = runs[0].FinishedAt
				}
			}
			out.Reason = "Server retrieval binding is present; health describes admitted retrieval and retained coverage."
			out.NextAction = "Inspect retrieval history and coverage."
			if loc.Adapter == FIRMSAdapter && out.Health.LastSuccessAt == nil && out.Health.FailureCode == "page_budget" {
				out.Reason = "FIRMS accepted a bounded day page. The initial frozen lookback is being backfilled; complete query coverage has not been established yet."
				out.NextAction = "The worker resumes the remaining UTC-day pages at the next scheduled attempt. Existing pacing and daily limits remain enforced."
			}
			if out.Health.FailureCode == "provider_auth" {
				out.State = "NEEDS_PROVIDER_ACCESS"
				out.Reason = "Provider rejected retrieval authorization; retained evidence remains available."
				out.NextAction = "Check provider credentials and use permission. Refresh private credentials and restart the source worker after correction."
			}
			return out, nil
		}
	}
	if retained := out.GeographicRetention; retained != nil {
		out.NextAction = ""
		switch {
		case !retained.Readable:
			out.State = "RESTRICTED"
			out.Reason = "The retained environment library requires environment read access."
		case retained.AssetsAvailable:
			out.State = "RETAINED"
			out.Reason = "Retained geographic inputs are available in the environment library."
		default:
			out.Reason = "No retained geographic inputs are currently available to this workspace."
		}
		return out, nil
	}
	activation, _ := src.Metadata["activation"].(map[string]any)
	if state, ok := activation["state"].(string); ok && (state == "NEEDS_CREDENTIALS" || state == "NEEDS_PERMISSION") {
		out.State = state
		out.Reason = "Provider access has not been activated."
		if action, ok := activation["nextAction"].(string); ok {
			out.NextAction = action
		}
	}
	return out, nil
}
