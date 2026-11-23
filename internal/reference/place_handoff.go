package reference

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

type placeHandoffInput struct {
	FeatureID string    `json:"featureId"`
	ReleaseID string    `json:"releaseId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type placeHandoff struct {
	Schema        string    `json:"schema"`
	Purpose       string    `json:"purpose"`
	ReleaseID     string    `json:"geographyReleaseId"`
	ReleaseSHA256 string    `json:"releaseSha256"`
	EditionID     string    `json:"editionId"`
	EditionSHA256 string    `json:"editionSha256"`
	SourceSHA256  string    `json:"sourceSha256"`
	Normalizer    string    `json:"normalizer"`
	Attribution   string    `json:"attribution"`
	RightsID      string    `json:"rightsId"`
	ExpiresAt     time.Time `json:"expiresAt"`
	MemberSHA256  string    `json:"memberSha256"`
	Member        []byte    `json:"member"`
}

func buildPlaceHandoff(in placeHandoffInput, release Release, feature Feature, now time.Time) (placeHandoff, error) {
	p := feature.Properties
	receipt, _ := p["receipt"].(map[string]any)
	rights, _ := p["rights"].(map[string]any)
	permissions, _ := rights["outputPermissions"].(map[string]any)
	if p["kind"] != "place" || receipt["schema"] != "emad.desert-maps-normalized.v1" ||
		permissions["geographicContextExport"] != true {
		return placeHandoff{}, apperr.Forbidden("civilian place handoff requires an admitted edition with distribution permission")
	}
	if !in.ExpiresAt.After(now) {
		return placeHandoff{}, apperr.Validation("handoff expiry must be in the future")
	}
	if sourceExpiry, ok := rights["expiresAt"].(string); ok {
		expiry, err := time.Parse(time.RFC3339Nano, sourceExpiry)
		if err != nil || in.ExpiresAt.After(expiry) {
			return placeHandoff{}, apperr.Forbidden("handoff expiry exceeds the source grant")
		}
	}
	var editionID, editionSHA string
	for _, edition := range release.Editions {
		if edition["id"] == p["editionId"] && edition["sha256"] == p["sha256"] {
			editionID, editionSHA = edition["id"], edition["sha256"]
			break
		}
	}
	if feature.ID != in.FeatureID || release.ID != in.ReleaseID || release.SourceID != p["sourceId"] || editionID == "" {
		return placeHandoff{}, apperr.NotFound("place in pinned geography release", "unavailable")
	}
	sourceSHA, _ := receipt["sourceSha256"].(string)
	attribution, _ := rights["attribution"].(string)
	rightsID, _ := rights["id"].(string)
	if len(sourceSHA) != 64 || attribution == "" || rightsID == "" {
		return placeHandoff{}, apperr.Conflict("place source lineage is incomplete")
	}
	feature.Properties = map[string]any{
		"name": p["name"], "family": p["family"], "publisherRegion": p["publisherRegion"],
		"key": p["regionKey"], "sourceId": p["sourceId"],
	}
	member, err := json.Marshal(feature)
	if err != nil {
		return placeHandoff{}, err
	}
	return placeHandoff{
		Schema: "emad.desert-maps-civilian-reference.v2", Purpose: "civilian-exercise-reference",
		ReleaseID: release.ID, ReleaseSHA256: release.SHA256, EditionID: editionID, EditionSHA256: editionSHA,
		SourceSHA256: sourceSHA, Normalizer: "emad.desert-maps-normalized.v1", Attribution: attribution,
		RightsID: rightsID, ExpiresAt: in.ExpiresAt.UTC(), MemberSHA256: Hash(member), Member: member,
	}, nil
}

func (h *Handler) mountPlaceHandoffs(r chi.Router) {
	r.Post("/geography/place-handoffs", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := require(ctx, "context.export"); err != nil {
			httpx.Error(w, err)
			return
		}
		var in placeHandoffInput
		if !areas.Decode(w, r, &in) {
			return
		}
		if len(in.ReleaseID) > 160 || !strings.HasPrefix(in.ReleaseID, "georel_") || in.FeatureID == "" ||
			len(in.FeatureID) > 256 || in.ExpiresAt.IsZero() {
			httpx.Error(w, apperr.Validation("an exact release, place and expiry are required"))
			return
		}
		places, err := h.s.Geography(ctx, "", "", "", in.FeatureID, 0, "", in.ReleaseID)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		if len(places.Features) != 1 {
			httpx.Error(w, apperr.NotFound("place in pinned geography release", "unavailable"))
			return
		}
		var raw []byte
		if err := db.Scoped(h.s.pool).QueryRow(ctx, `SELECT document FROM geographic_releases WHERE id=$1`, in.ReleaseID).Scan(&raw); err != nil {
			httpx.Error(w, err)
			return
		}
		var release Release
		if err := json.Unmarshal(raw, &release); err != nil {
			httpx.Error(w, err)
			return
		}
		pack, err := buildPlaceHandoff(in, release, places.Features[0], time.Now().UTC())
		if err != nil {
			httpx.Error(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, http.StatusOK, pack)
	})
}
