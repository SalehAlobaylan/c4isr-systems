package geoenv

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

type lineageInput struct {
	AssetID            string         `json:"assetId"`
	SourceID           string         `json:"sourceId"`
	AdmissionID        string         `json:"admissionId"`
	CurrentAdmissionID string         `json:"currentAdmissionId"`
	SHA256             string         `json:"sha256"`
	Bytes              int64          `json:"bytes"`
	ReferenceID        string         `json:"referenceId,omitempty"`
	SceneID            string         `json:"sceneId,omitempty"`
	Representation     Representation `json:"representation"`
	Attribution        string         `json:"attribution"`
	RetainUntil        time.Time      `json:"retainUntil"`
	PolicyExpiresAt    *time.Time     `json:"policyExpiresAt,omitempty"`
	OriginalDownload   bool           `json:"originalDownload"`
	ViewerExport       bool           `json:"viewerExport"`
	ContextExport      bool           `json:"contextExport"`
	ParentState        string         `json:"parentState"`
}

type releaseLineage struct {
	ReleaseID      string         `json:"releaseId"`
	ManifestSHA256 string         `json:"manifestSha256"`
	Inputs         []lineageInput `json:"inputs"`
	Members        []Member       `json:"members"`
}

func (s Service) releaseLineage(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Read)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	var manifest Manifest
	if err := json.Unmarshal(rel.Manifest, &manifest); err != nil {
		httpx.Error(w, apperr.Conflict("release manifest is unavailable"))
		return
	}
	out := releaseLineage{
		ReleaseID: rel.ID, ManifestSHA256: rel.ManifestSHA256,
		Inputs: make([]lineageInput, 0, len(manifest.Inputs)), Members: manifest.Members,
	}
	for _, asset := range manifest.Inputs {
		current, err := s.policy(r.Context(), asset, true, false)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		rights := current.Rights.OutputPermissions
		out.Inputs = append(out.Inputs, lineageInput{
			AssetID: asset.ID, SourceID: asset.SourceID, AdmissionID: asset.AdmissionID,
			CurrentAdmissionID: current.CurrentAdmissionID, SHA256: asset.SHA256, Bytes: asset.Bytes,
			ReferenceID: asset.ReferenceID, SceneID: asset.SceneID, Representation: asset.Representation,
			Attribution: asset.Rights.Attribution, RetainUntil: asset.RetainUntil,
			PolicyExpiresAt:  current.Rights.ExpiresAt,
			OriginalDownload: rights != nil && rights.RawDistribution != nil && *rights.RawDistribution,
			ViewerExport:     rights != nil && rights.GeographicViewerExport != nil && *rights.GeographicViewerExport,
			ContextExport:    rights != nil && rights.GeographicContextExport != nil && *rights.GeographicContextExport,
			ParentState:      "RETAINED_INPUT",
		})
		if asset.ReferenceID != "" || asset.SceneID != "" {
			var linked bool
			if err := db.Scoped(s.Pool).QueryRow(r.Context(), `SELECT CASE WHEN $1<>'' THEN EXISTS(
			 SELECT 1 FROM reference_datasets WHERE id=$1 AND source_id=$3 AND document->>'parentArtifactSha256'=$4)
			 ELSE EXISTS(SELECT 1 FROM imagery_scenes WHERE id=$2 AND source_id=$3 AND admission_id=$5) END`,
				asset.ReferenceID, asset.SceneID, asset.SourceID, asset.SHA256, asset.AdmissionID).Scan(&linked); err != nil {
				httpx.Error(w, err)
				return
			}
			out.Inputs[len(out.Inputs)-1].ParentState = "UNAVAILABLE"
			if linked {
				out.Inputs[len(out.Inputs)-1].ParentState = "LINKED"
			}
		}
	}
	respond(w, http.StatusOK, out, nil)
}
