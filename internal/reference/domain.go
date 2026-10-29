// Package reference owns immutable geographic context. Features and elevation
// snapshots are neither observations nor exercise actors.
package reference

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

type Resolution struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
type Accuracy struct {
	HorizontalM float64 `json:"horizontalM"`
	VerticalM   float64 `json:"verticalM"`
	Provenance  string  `json:"provenance"`
}
type Parent struct {
	SHA256        string    `json:"sha256"`
	Bytes         int64     `json:"bytes"`
	SourceID      string    `json:"sourceId"`
	Reference     string    `json:"reference"`
	RetrievedAt   time.Time `json:"retrievedAt"`
	CRS           string    `json:"crs"`
	VerticalDatum string    `json:"verticalDatum,omitempty"`
}
type Input struct {
	Parents              []Parent            `json:"parents,omitempty"`
	Accuracy             *Accuracy           `json:"accuracy,omitempty"`
	SourceID             string              `json:"sourceId"`
	DatasetKey           string              `json:"datasetKey"`
	Version              string              `json:"version"`
	Label                string              `json:"label"`
	Kind                 string              `json:"kind"`
	OriginalProducer     string              `json:"originalProducer"`
	CRS                  string              `json:"crs"`
	VerticalDatum        string              `json:"verticalDatum"`
	Resolution           Resolution          `json:"resolution"`
	AOI                  publicingest.Bounds `json:"aoi"`
	Validity             string              `json:"validity"`
	Quality              string              `json:"quality"`
	Transformation       string              `json:"transformation"`
	Rights               publicingest.Rights `json:"rights"`
	RetrievalReference   string              `json:"retrievalReference"`
	ParentArtifactSHA256 string              `json:"parentArtifactSha256,omitempty"`
	RetrievedAt          time.Time           `json:"retrievedAt"`
	SHA256               string              `json:"sha256"`
	Artifact             []byte              `json:"artifact,omitempty"`
}
type Dataset struct {
	Input
	ID                    string    `json:"id"`
	ApprovedBy            string    `json:"approvedBy"`
	ApprovedAt            time.Time `json:"approvedAt"`
	Bytes                 int       `json:"bytes"`
	FeatureCount          int       `json:"featureCount"`
	EligibleForSimulation bool      `json:"eligibleForSimulation"`
}
type Feature struct {
	CRS        json.RawMessage `json:"crs,omitempty"`
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}
type Collection struct {
	CRS      json.RawMessage `json:"crs,omitempty"`
	Type     string          `json:"type"`
	Features []Feature       `json:"features"`
}

// Grid is a bounded regular point grid, south-to-north rows and west-to-east
// columns, heights in metres relative to the declared vertical datum.
type Grid struct {
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	West         float64    `json:"west"`
	South        float64    `json:"south"`
	StepX        float64    `json:"stepX"`
	StepY        float64    `json:"stepY"`
	Values       []*float64 `json:"values"`
	Registration string     `json:"registration"`
}

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func (in Input) validate() (Collection, error) {
	fail := func() (Collection, error) {
		return Collection{}, apperr.Validation("invalid or incomplete reference dataset metadata/artifact")
	}
	for _, s := range []string{in.SourceID, in.DatasetKey, in.Version, in.Label, in.OriginalProducer, in.VerticalDatum, in.Validity, in.Quality, in.Transformation, in.Rights.ID, in.Rights.LegalBasis, in.Rights.Attribution} {
		if strings.TrimSpace(s) == "" || len(s) > 2000 {
			return fail()
		}
	}
	if len(in.Parents) > 16 {
		return fail()
	}
	var parentBytes int64
	for _, p := range in.Parents {
		u, e := url.Parse(p.Reference)
		raw, err := hex.DecodeString(p.SHA256)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || p.SourceID != in.SourceID || p.RetrievedAt.IsZero() || p.RetrievedAt.After(time.Now().Add(5*time.Minute)) || p.Bytes < 1 || p.Bytes > 256<<20 || err != nil || len(raw) != 32 {
			return fail()
		}
		for key := range u.Query() {
			key = strings.ToLower(key)
			if strings.Contains(key, "token") || strings.Contains(key, "key") || strings.Contains(key, "auth") || strings.Contains(key, "signature") {
				return fail()
			}
		}
		parentBytes += p.Bytes
	}
	if parentBytes > 256<<20 {
		return fail()
	}
	if in.Accuracy != nil && (in.Accuracy.HorizontalM <= 0 || in.Accuracy.VerticalM < 0 || math.IsNaN(in.Accuracy.HorizontalM) || math.IsInf(in.Accuracy.HorizontalM, 0) || math.IsNaN(in.Accuracy.VerticalM) || math.IsInf(in.Accuracy.VerticalM, 0) || len(in.Accuracy.Provenance) < 10 || len(in.Accuracy.Provenance) > 2000) {
		return fail()
	}
	if in.Kind != "vector" && in.Kind != "elevation" {
		return fail()
	}
	if in.Kind == "elevation" && (strings.EqualFold(in.VerticalDatum, "unknown") || in.VerticalDatum == "not_applicable" || in.Resolution.Unit != "arc_seconds") {
		return fail()
	}
	if in.CRS != "EPSG:4326" {
		return Collection{}, apperr.Validation("initial reference import requires EPSG:4326; reproject explicitly before import")
	}
	if err := in.AOI.Validate(); err != nil {
		return Collection{}, err
	}
	if in.Resolution.Value <= 0 || math.IsNaN(in.Resolution.Value) || math.IsInf(in.Resolution.Value, 0) {
		return fail()
	}
	if in.Resolution.Unit != "arc_seconds" && in.Resolution.Unit != "metres" && in.Resolution.Unit != "map_scale" {
		return fail()
	}
	for _, s := range []string{in.RetrievalReference, in.Rights.TermsURL} {
		u, err := url.Parse(s)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return fail()
		}
		for key := range u.Query() {
			key = strings.ToLower(key)
			if strings.Contains(key, "token") || strings.Contains(key, "key") || strings.Contains(key, "auth") || strings.Contains(key, "signature") {
				return fail()
			}
		}
	}
	if !in.Rights.RetainRaw || len(in.Artifact) == 0 || len(in.Artifact) > 524288 || Hash(in.Artifact) != in.SHA256 || !json.Valid(in.Artifact) {
		return fail()
	}
	if in.Rights.ExpiresAt != nil && !in.Rights.ExpiresAt.After(time.Now()) {
		return fail()
	}
	if in.RetrievedAt.IsZero() || in.RetrievedAt.After(time.Now().Add(5*time.Minute)) {
		return fail()
	}
	if in.ParentArtifactSHA256 != "" {
		b, err := hex.DecodeString(in.ParentArtifactSHA256)
		if err != nil || len(b) != 32 {
			return fail()
		}
	}
	if in.Kind == "vector" {
		var c Collection
		if json.Unmarshal(in.Artifact, &c) != nil || c.Type != "FeatureCollection" || len(c.CRS) > 0 || len(c.Features) < 1 || len(c.Features) > 1000 {
			return fail()
		}
		seen := map[string]bool{}
		for _, f := range c.Features {
			if f.Type != "Feature" || f.ID == "" || len(f.ID) > 256 || seen[f.ID] || len(f.Geometry) == 0 || len(f.CRS) > 0 {
				return fail()
			}
			seen[f.ID] = true
		}
		return c, nil
	}
	var g Grid
	if json.Unmarshal(in.Artifact, &g) != nil || g.Width < 2 || g.Height < 2 || g.Width > 256 || g.Height > 256 || len(g.Values) != g.Width*g.Height || g.StepX <= 0 || g.StepY <= 0 || g.Registration != "point_samples" {
		return fail()
	}
	for _, v := range []float64{g.West, g.South, g.StepX, g.StepY} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fail()
		}
	}
	if g.West < in.AOI.West || g.South < in.AOI.South || g.West+float64(g.Width-1)*g.StepX > in.AOI.East+1e-8 || g.South+float64(g.Height-1)*g.StepY > in.AOI.North+1e-8 {
		return fail()
	}
	if math.Abs(g.StepX*3600-in.Resolution.Value) > 1e-6 || math.Abs(g.StepY*3600-in.Resolution.Value) > 1e-6 {
		return fail()
	}
	samples := 0
	for _, v := range g.Values {
		if v == nil {
			continue
		}
		if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < -12000 || *v > 10000 {
			return fail()
		}
		samples++
	}
	if samples == 0 {
		return fail()
	}
	return Collection{}, nil
}
