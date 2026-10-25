// Package modelassets owns protected, versioned simulation authoring assets.
// Visual geometry never implies behavior, control authority or validated physics.
package modelassets

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const Importer = "emad-static-gltf-v1"
const MaxBytes = 16 * 1024 * 1024

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

type License struct {
	ID                  string `json:"id"`
	LicenseID           string `json:"licenseId"`
	TermsURL            string `json:"termsUrl"`
	Attribution         string `json:"attribution"`
	LegalBasis          string `json:"legalBasis"`
	AllowStorage        bool   `json:"allowStorage"`
	AllowSimulation     bool   `json:"allowSimulation"`
	AllowRedistribution bool   `json:"allowRedistribution"`
	ArchivePermission   string `json:"archivePermission"`
}

func text(v string, max int) bool {
	return len(strings.TrimSpace(v)) > 0 && len(v) <= max && !strings.ContainsAny(v, "\x00\r")
}
func reference(v string) bool {
	u, e := url.Parse(v)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}

type UploadInput struct {
	SourceVersion   string  `json:"sourceVersion"`
	RequestKey      string  `json:"requestKey"`
	Name            string  `json:"name"`
	Kind            string  `json:"kind"`
	SHA256          string  `json:"sha256"`
	Bytes           int64   `json:"bytes"`
	MediaType       string  `json:"mediaType"`
	SourceReference string  `json:"sourceReference"`
	Provenance      string  `json:"provenance"`
	SourceFileID    string  `json:"sourceFileId,omitempty"`
	ParentVersionID string  `json:"parentVersionId,omitempty"`
	Engine          string  `json:"engine,omitempty"`
	EngineBuild     string  `json:"engineBuild,omitempty"`
	License         License `json:"license"`
}

func (in UploadInput) Validate() error {
	bad := func() error {
		return apperr.Validation("invalid model asset metadata, license or bounded import profile")
	}
	l := in.License
	if !text(in.SourceVersion, 128) || len(in.SourceReference) > 2048 || len(in.SourceFileID) > 160 || len(in.ParentVersionID) > 160 || !keyPattern.MatchString(in.RequestKey) || !text(in.Name, 200) || !hashPattern.MatchString(in.SHA256) || in.Bytes < 1 || in.Bytes > MaxBytes || !reference(in.SourceReference) || !text(in.Provenance, 2000) || !text(l.ID, 128) || !text(l.LicenseID, 128) || !reference(l.TermsURL) || !text(l.Attribution, 2000) || !text(l.LegalBasis, 4000) || !l.AllowStorage || !l.AllowSimulation || l.ArchivePermission != "perpetual" {
		return bad()
	}
	switch in.Kind {
	case "MODEL":
		if in.MediaType != "model/gltf-binary" && in.MediaType != "model/gltf+json" {
			return bad()
		}
		if in.ParentVersionID != "" || in.Engine != "" || in.EngineBuild != "" {
			return bad()
		}
	case "SOURCE":
		if in.MediaType != "application/octet-stream" || in.SourceFileID != "" || in.ParentVersionID != "" || in.Engine != "" || in.EngineBuild != "" {
			return bad()
		}
	case "ENGINE_VARIANT":
		if in.MediaType != "application/octet-stream" || !text(in.ParentVersionID, 100) || !text(in.Engine, 100) || !text(in.EngineBuild, 200) || in.SourceFileID != "" {
			return bad()
		}
	default:
		return bad()
	}
	return nil
}

type Upload struct {
	UploadInput
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Attempts  int       `json:"attempts"`
	ErrorCode string    `json:"errorCode"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	ObjectKey string    `json:"-"`
}
type Validation struct {
	Schema               string            `json:"schema"`
	Importer             string            `json:"importer"`
	ValidatorVersion     string            `json:"validatorVersion"`
	Units                string            `json:"units"`
	Axes                 string            `json:"axes"`
	Fidelity             string            `json:"fidelity"`
	Vertices             int               `json:"vertices"`
	Triangles            int               `json:"triangles"`
	Primitives           int               `json:"primitives"`
	Nodes                int               `json:"nodes"`
	Instances            int               `json:"instances"`
	Materials            []json.RawMessage `json:"materials"`
	Images               []json.RawMessage `json:"images"`
	TextureCount         int               `json:"textureCount"`
	DecodedTexturePixels int               `json:"decodedTexturePixels"`
	Bounds               struct {
		Min [3]float64 `json:"min"`
		Max [3]float64 `json:"max"`
	} `json:"bounds"`
	Warnings []string `json:"warnings"`
}
type ModelAsset struct {
	ManifestBytes  int64      `json:"manifestBytes"`
	ID             string     `json:"id"`
	Upload         Upload     `json:"upload"`
	Validation     Validation `json:"validation"`
	WorkerBuild    string     `json:"workerBuild"`
	ValidatorBuild string     `json:"validatorBuild"`
	ManifestSHA256 string     `json:"manifestSha256"`
	CreatedAt      time.Time  `json:"createdAt"`
	ManifestKey    string     `json:"-"`
}
type ActorTemplate struct {
	CreatedBy      string    `json:"createdBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt,omitzero"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ModelVersionID string    `json:"modelVersionId"`
	Behavior       string    `json:"behavior"`
	MaxSpeedMPS    float64   `json:"maxSpeedMps"`
	Fidelity       string    `json:"fidelity"`
	Limitations    string    `json:"limitations"`
}
type EnvironmentAsset struct {
	Prepared           *PreparedEnvironment `json:"prepared,omitempty"`
	ReleaseToOperators bool                 `json:"releaseToOperators,omitempty"`
	CreatedBy          string               `json:"createdBy,omitempty"`
	CreatedAt          time.Time            `json:"createdAt,omitzero"`
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	ModelVersionID     string               `json:"modelVersionId"`
	Frame              simulation.Frame     `json:"frame"`
	ModelAxesToENU     string               `json:"modelAxesToEnu"`
	Provenance         string               `json:"provenance"`
	Fidelity           string               `json:"fidelity"`
}
type PreparedEnvironment struct {
	Schema              string              `json:"schema"`
	AOI                 publicingest.Bounds `json:"aoi"`
	Frame               simulation.Frame    `json:"frame"`
	ModelSHA256         string              `json:"modelSha256"`
	TerrainParentSHA256 string              `json:"terrainParentSha256"`
	GeoidSHA256         string              `json:"geoidSha256"`
	OSMParentSHA256     string              `json:"osmParentSha256"`
	ImagerySHA256       string              `json:"imagerySha256"`
	InputVerticalDatum  string              `json:"inputVerticalDatum"`
	OutputVerticalDatum string              `json:"outputVerticalDatum"`
	Transformation      string              `json:"transformation"`
	TerrainVertices     int                 `json:"terrainVertices"`
	Buildings           int                 `json:"buildings"`
	RoadSegments        int                 `json:"roadSegments"`
	Limitations         []string            `json:"limitations"`
}
