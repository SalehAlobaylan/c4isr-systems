package publicingest

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"time"
)

type AdapterCapability struct {
	Adapter      string `json:"adapter"`
	Family       string `json:"family"`
	Discovery    bool   `json:"discovery"`
	Historical   bool   `json:"historical"`
	Retention    string `json:"retention"`
	Analysis     bool   `json:"analysis"`
	Capture      bool   `json:"capture"`
	Credentials  string `json:"credentials"`
	LimitMeaning string `json:"limitMeaning"`
}

// This is the installed adapter contract, not a promise of provider completeness.
func Capability(adapter string) AdapterCapability {
	v := AdapterCapability{Adapter: adapter, Discovery: true, Retention: "original admitted rights", LimitMeaning: "approved AOI, lookback, page, response and record bounds; capped/rejected/unpublished results are partial"}
	switch adapter {
	case USGSAdapter:
		v.Family = "earthquakes"
		v.Historical = true
		v.Credentials = "public"
	case FIRMSAdapter:
		v.Family = "thermal"
		v.Historical = true
		v.Credentials = "server map key; at most 10 days"
	case EarthSearchAdapter, STACAdapter:
		v.Family = "imagery"
		v.Historical = true
		v.Capture = true
		v.Analysis = true
		v.Credentials = "public catalog; raster access and rights checked separately"
	case METARAdapter:
		v.Family = "weather"
		v.Credentials = "public current context"
	case GDACSAdapter, GDACSRSSAdapter:
		v.Family = "bulletins"
		v.Credentials = "public current context"
	case EONETAdapter:
		v.Family = "bulletins"
		v.Historical = true
		v.Credentials = "public flood catalogue"
		v.LimitMeaning = "One capped report page; reaching the cap is partial and an empty result is not absence of hazards."
	case OpenSkyAdapter:
		v.Family = "aircraft"
		v.Credentials = "anonymous or server OAuth; current snapshot only"
	case AISAdapter:
		v.Family = "vessels"
		v.Credentials = "server key and reviewed permission; current stream only"
	case "reference-import-v1":
		v.Family = "reference"
		v.Historical = true
		v.Discovery = false
		v.Credentials = "source artifact and reviewed rights"
	case "dem-import-v1":
		v.Family = "terrain"
		v.Historical = true
		v.Discovery = false
		v.Analysis = true
		v.Credentials = "licensed source artifact and vertical datum"
	case "emad-json-page-v1":
		v.Family = "events"
		v.Historical = true
		v.Credentials = "trusted configured adapter; test/custom contract"
	default:
		v.Family = "unknown"
		v.Discovery = false
		v.Credentials = "unsupported adapter"
	}
	return v
}
func CapabilityInventory() []AdapterCapability {
	out := []AdapterCapability{}
	for _, a := range []string{USGSAdapter, FIRMSAdapter, EarthSearchAdapter, STACAdapter, METARAdapter, GDACSAdapter, GDACSRSSAdapter, EONETAdapter, OpenSkyAdapter, AISAdapter, "reference-import-v1", "dem-import-v1"} {
		out = append(out, Capability(a))
	}
	return out
}

type AcquisitionPolicy struct {
	Admission        Admission         `json:"admission"`
	Capability       AdapterCapability `json:"capability"`
	Gate             string            `json:"gate"`
	ExecutionGranted bool              `json:"executionGranted"`
}

func (s *Service) AcquisitionPolicy(ctx context.Context, source, key string) (AcquisitionPolicy, error) {
	var out AcquisitionPolicy
	if e := requirePublic(ctx, source, "sources.read"); e != nil {
		return out, e
	}
	if key == "" {
		key = "default"
	}
	var st state
	var raw []byte
	e := db.Scoped(s.repo.pool).QueryRow(ctx, `SELECT a.document FROM retrieval_profile_state st JOIN source_admissions a ON a.id=st.admission_id AND a.scope_id=st.scope_id WHERE st.source_id=$1 AND st.profile_key=$2`, source, key).Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &st.Admission)
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("retrieval profile", "unavailable")
	}
	if e != nil {
		return out, e
	}
	out.Admission = st.Admission
	out.Admission.ProfileKey = key
	out.Admission.ProfileRevisionID = "rpr_" + st.Admission.ID
	location, ok := s.locations[st.Admission.ConnectorID]
	r, _ := access.ResourceFrom(ctx)
	out.Gate = "ready"
	if !ok || location.SourceID != source || location.Resource != r {
		out.Gate = "not_configured"
	} else {
		out.Capability = Capability(location.Adapter)
		if location.GateState != "" {
			out.Gate = location.GateState
		}
	}
	src, e := s.sources.Get(ctx, source)
	if e != nil {
		return out, e
	}
	if src.Status != "active" || !st.Admission.Enabled || st.Admission.Rights.ExpiresAt != nil && !st.Admission.Rights.ExpiresAt.After(time.Now()) {
		out.Gate = "disabled_or_expired"
	}
	e = db.Scoped(s.repo.pool).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM retrieval_execution_grants WHERE profile_revision_id=$1 AND principal_id=$2 AND enabled)`, out.Admission.ProfileRevisionID, st.Admission.ConnectorPrincipalID).Scan(&out.ExecutionGranted)
	return out, e
}
func (s *Service) RequireAcquisitionPolicy(ctx context.Context, source, key, revision string) (AcquisitionPolicy, error) {
	v, e := s.AcquisitionPolicy(ctx, source, key)
	if e != nil {
		return v, e
	}
	if revision != v.Admission.ProfileRevisionID {
		return v, apperr.Conflict("profile revision changed; review coverage before a new request")
	}
	return v, nil
}
