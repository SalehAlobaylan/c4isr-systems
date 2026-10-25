// Package contextpackage builds bounded, engine-independent geographic inputs.
// It performs no network fetch, scene generation, or actor authoring.
package contextpackage

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
)

const Schema = "emad.context.v1"

type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
type Approval struct {
	By        string    `json:"by"`
	At        time.Time `json:"at"`
	Statement string    `json:"statement"`
}
type ImageryLineage struct {
	AuthorHumanID   string              `json:"authorHumanId,omitempty"`
	ReviewerHumanID string              `json:"reviewerHumanId,omitempty"`
	CurrentRights   publicingest.Rights `json:"currentRights"`
	AdmissionID     string              `json:"admissionId"`
	ArtifactSHA256  string              `json:"artifactSha256,omitempty"`
	SceneIDs        []string            `json:"sceneIds"`
	EvidenceIDs     []string            `json:"evidenceIds"`
	ArtifactID      string              `json:"artifactId,omitempty"`
	JobID           string              `json:"jobId,omitempty"`
	RevisionID      string              `json:"revisionId,omitempty"`
	Synthetic       bool                `json:"synthetic"`
	Role            string              `json:"role"`
}
type Asset struct {
	Lineage              *ImageryLineage      `json:"lineage,omitempty"`
	AvailableUntil       *time.Time           `json:"availableUntil,omitempty"`
	SyntheticDerived     bool                 `json:"syntheticDerived,omitempty"`
	Kind                 string               `json:"kind"`
	DatasetAOI           publicingest.Bounds  `json:"datasetAoi"`
	ParentArtifactSHA256 string               `json:"parentArtifactSha256,omitempty"`
	DatasetID            string               `json:"datasetId"`
	SourceID             string               `json:"sourceId"`
	DatasetKey           string               `json:"datasetKey"`
	Version              string               `json:"version"`
	OriginalProducer     string               `json:"originalProducer"`
	Reference            string               `json:"reference"`
	RetrievedAt          time.Time            `json:"retrievedAt"`
	SHA256               string               `json:"sha256"`
	Bytes                int                  `json:"bytes"`
	CRS                  string               `json:"crs"`
	VerticalDatum        string               `json:"verticalDatum"`
	Resolution           reference.Resolution `json:"resolution"`
	Quality              string               `json:"quality"`
	Validity             string               `json:"validity"`
	Processing           string               `json:"processing"`
	Rights               publicingest.Rights  `json:"rights"`
	Retention            string               `json:"retention"`
}
type Manifest struct {
	Omissions         []string            `json:"omissions,omitempty"`
	Schema            string              `json:"schema"`
	AOI               publicingest.Bounds `json:"aoi"`
	TimeWindow        Window              `json:"timeWindow"`
	CreatedAt         time.Time           `json:"createdAt"`
	Creator           string              `json:"creator"`
	Approval          Approval            `json:"approval"`
	TransformVersion  string              `json:"transformVersion"`
	ProcessingVersion string              `json:"processingVersion"`
	GeometryQuality   string              `json:"geometryQuality"`
	ImageryQuality    string              `json:"imageryQuality"`
	Reproduction      string              `json:"reproduction"`
	SyntheticDerived  bool                `json:"syntheticDerived"`
	Assets            []Asset             `json:"assets"`
}
type Bundle struct {
	ID       string            `json:"id"`
	Manifest Manifest          `json:"manifest"`
	Assets   map[string][]byte `json:"assets"`
}

func ID(m Manifest) string { raw, _ := json.Marshal(m); return "ctx_" + reference.Hash(raw) }

// Validate checks an offline bundle, including missing dependencies and rights.
// requireOffline rejects reference-only packages. No URL is dereferenced.
func (b Bundle) Validate(now time.Time, requireOffline bool) error {
	fail := func() error { return fmt.Errorf("invalid, incomplete, expired, or corrupt context package") }
	m := b.Manifest
	if m.Schema != Schema || b.ID != ID(m) || m.AOI.Validate() != nil || m.TimeWindow.From.IsZero() || m.TimeWindow.To.Before(m.TimeWindow.From) || m.TimeWindow.To.Sub(m.TimeWindow.From) > 366*24*time.Hour || m.Creator == "" || m.Approval.By == "" || m.Approval.Statement == "" || m.Approval.At.IsZero() || m.CreatedAt.IsZero() || m.Approval.At.Before(m.CreatedAt) || m.TransformVersion != simulation.TransformVersion || (m.ProcessingVersion != "reference-snapshot-v1" && m.ProcessingVersion != "context-snapshot-v2") || m.GeometryQuality == "" || m.ImageryQuality == "" || (m.ProcessingVersion == "reference-snapshot-v1" && (m.ImageryQuality != "not_included" || m.SyntheticDerived)) || len(m.Assets) < 1 || len(m.Assets) > 16 {
		return fail()
	}
	if m.Reproduction != "retained_offline" && m.Reproduction != "external_dependencies" {
		return fail()
	}
	if requireOffline && m.Reproduction != "retained_offline" {
		return fmt.Errorf("package requires external dependencies; offline reproduction unavailable")
	}
	seen := map[string]bool{}
	retained := map[string]bool{}
	external := false
	total := 0
	for _, a := range m.Assets {
		if (a.Kind != "vector" && a.Kind != "elevation" && a.Kind != "imagery" && a.Kind != "analysis" && a.Kind != "derived_geometry" && a.Kind != "assessment") || a.DatasetAOI.Validate() != nil || m.AOI.West < a.DatasetAOI.West || m.AOI.South < a.DatasetAOI.South || m.AOI.East > a.DatasetAOI.East || m.AOI.North > a.DatasetAOI.North || seen[a.DatasetID] || a.DatasetID == "" || a.SourceID == "" || a.DatasetKey == "" || a.Version == "" || a.OriginalProducer == "" || a.Reference == "" || a.RetrievedAt.IsZero() || a.CRS != "EPSG:4326" || a.VerticalDatum == "" || a.Quality == "" || a.Validity == "" || a.Processing == "" || a.Resolution.Value <= 0 || a.Resolution.Unit == "" || len(a.SHA256) != 64 || a.Bytes < 1 || a.Bytes > 16777216 || ((a.Kind == "vector" || a.Kind == "elevation") && a.Bytes > 524288) || !a.Rights.AllowSimulationExport || a.Rights.ID == "" || a.Rights.LegalBasis == "" || a.Rights.Attribution == "" || a.Rights.TermsURL == "" || (a.Rights.ExpiresAt != nil && !a.Rights.ExpiresAt.After(now)) {
			return fail()
		}
		if a.AvailableUntil != nil && !a.AvailableUntil.After(now) {
			return fail()
		}
		if a.Kind != "vector" && a.Kind != "elevation" {
			if m.ProcessingVersion != "context-snapshot-v2" || a.Lineage == nil || len(a.Lineage.SceneIDs) < 1 || len(a.Lineage.EvidenceIDs) < 1 || a.Lineage.Role == "" || a.Lineage.AdmissionID == "" || a.Lineage.JobID == "" || a.Lineage.ArtifactID == "" || len(a.Lineage.SceneIDs) != len(a.Lineage.EvidenceIDs) {
				return fail()
			}
			if raw, err := hex.DecodeString(a.Lineage.ArtifactSHA256); err != nil || len(raw) != 32 {
				return fail()
			}
			if (a.Kind == "analysis" || a.Kind == "assessment" || a.Kind == "derived_geometry") && a.Lineage.RevisionID == "" {
				return fail()
			}
			if a.Lineage.RevisionID != "" && (a.Lineage.AuthorHumanID == "" || a.Lineage.ReviewerHumanID == "" || a.Lineage.AuthorHumanID == a.Lineage.ReviewerHumanID) {
				return fmt.Errorf("reviewed context requires distinct resolved registered human identities; re-export historical packages after independent review")
			}
			if !a.Lineage.CurrentRights.AllowSimulationExport || a.Lineage.CurrentRights.ID == "" || a.Lineage.CurrentRights.LegalBasis == "" || a.Lineage.CurrentRights.Attribution == "" || (a.Lineage.CurrentRights.ExpiresAt != nil && !a.Lineage.CurrentRights.ExpiresAt.After(now)) || (a.Retention == "retained" && !a.Lineage.CurrentRights.RetainRaw) {
				return fail()
			}
			u, err := url.Parse(a.Lineage.CurrentRights.TermsURL)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
				return fail()
			}
			if a.Kind == "derived_geometry" && (!a.SyntheticDerived || !m.SyntheticDerived) {
				return fail()
			}
		}
		if a.ParentArtifactSHA256 != "" {
			if raw, err := hex.DecodeString(a.ParentArtifactSHA256); err != nil || len(raw) != 32 {
				return fail()
			}
		}
		if raw, err := hex.DecodeString(a.SHA256); err != nil || len(raw) != 32 {
			return fail()
		}
		if math.IsNaN(a.Resolution.Value) || math.IsInf(a.Resolution.Value, 0) || a.RetrievedAt.After(now.Add(5*time.Minute)) {
			return fail()
		}
		for _, ref := range []string{a.Reference, a.Rights.TermsURL} {
			u, err := url.Parse(ref)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
				return fail()
			}
		}
		seen[a.DatasetID] = true
		data, ok := b.Assets[a.SHA256]
		switch a.Retention {
		case "retained":
			if !a.Rights.RetainRaw || !ok || len(data) != a.Bytes || reference.Hash(data) != a.SHA256 {
				return fail()
			}
			retained[a.SHA256] = true
			total += len(data)
		case "reference_only":
			external = true
			if ok {
				return fail()
			}
		default:
			return fail()
		}
	}
	if total > 8*1024*1024 || len(retained) != len(b.Assets) || (external != (m.Reproduction == "external_dependencies")) {
		return fail()
	}
	return nil
}
