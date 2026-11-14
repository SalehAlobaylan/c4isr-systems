package geoenv

import (
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"slices"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
)

const egm2008Grid = "4191d471eefebf24091b56dbc604353cb3b8cf8cc70e448bb9ae56a272bef17a"

var horizontalEPSG = regexp.MustCompile(`^EPSG:[1-9][0-9]{3,5}$`)

type layerCapability struct {
	Layer          string   `json:"layer"`
	Required       bool     `json:"required"`
	Formats        []string `json:"formats"`
	VerticalDatums []string `json:"verticalDatums,omitempty"`
	Meaning        string   `json:"meaning"`
}

type profileContract struct {
	Schema              string            `json:"schema"`
	ProfileID           string            `json:"profileId"`
	Layers              []layerCapability `json:"layers"`
	MaxComponents       int               `json:"maxComponents"`
	MaxInputs           int               `json:"maxInputs"`
	MaxInputBytes       int64             `json:"maxInputBytes"`
	LatitudeLimit       int               `json:"latitudeLimit"`
	LongitudeSpanLimit  int               `json:"longitudeSpanLimit"`
	CoordinateLimitM    float64           `json:"coordinateLimitM"`
	EGM2008GridSHA256   string            `json:"egm2008GridSha256"`
	HorizontalReference string            `json:"horizontalReference"`
	RasterMask          string            `json:"rasterMask"`
}

type profiledCapability struct {
	Profile
	Contract       profileContract `json:"contract"`
	ContractSHA256 string          `json:"contractSha256"`
}

func capabilities(p Profile) profileContract {
	contract := profileContract{"emad.geographic-profile.v1", p.ID, nil, 16, 64, 4 << 30, 85, 180, .001, egm2008Grid,
		"WGS84 longitude and latitude for the area; raster EPSG and native transform recorded per member",
		"Native pixel registration and valid-sample mask within each component"}
	for _, layer := range append(slices.Clone(p.Required), p.Optional...) {
		item := layerCapability{Layer: layer, Required: slices.Contains(p.Required, layer),
			Formats: []string{"geojson", "osm-pbf", "osm-xml", "geopackage", "shapefile-zip"}, Meaning: "Source geometry and tags"}
		switch layer {
		case "surface":
			item.Formats, item.VerticalDatums, item.Meaning = []string{"geotiff"}, []string{"ellipsoid", "EGM2008"}, "Provider surface elevations; inspect source semantics and uncertainty"
		case "geoid":
			item.Formats, item.VerticalDatums, item.Meaning = []string{"geotiff"}, []string{"EGM2008"}, "Pinned NGA grid; bilinear geoid undulation for orthometric-to-ellipsoidal conversion"
		case "ground":
			item.Formats, item.VerticalDatums, item.Meaning = []string{"geotiff"}, []string{"EGM2008"}, "Modeled bare-earth elevation"
		case "bathymetry":
			item.Formats, item.Meaning = []string{"gebco-pair-zip"}, "GEBCO signed elevation with source-type cells and separate coastline"
		case "imagery", "land-cover":
			item.Formats, item.Meaning = []string{"geotiff"}, "Dated native raster samples and validity mask"
		case "radar":
			item.Formats, item.Meaning = []string{"radar-capture-zip"}, "Dated Sentinel-1 native linear-power backscatter"
		case "weather-field":
			item.Formats, item.Meaning = []string{"era5-pressure-json"}, "ERA5 reanalysis pressure-level values"
		case "modeled-building-context":
			item.Formats, item.VerticalDatums, item.Meaning = []string{"geojson"}, []string{"above-ground"}, "Modeled building height context"
		}
		contract.Layers = append(contract.Layers, item)
	}
	return contract
}

func profileDigest(contract profileContract) string {
	raw, _ := jsoncanon.Marshal(contract)
	return artifacts.Hash(raw)
}

func (s Service) listProfiles(w http.ResponseWriter, r *http.Request) {
	if err := require(r.Context(), Read, false); err != nil {
		httpx.Error(w, err)
		return
	}
	items := make([]profiledCapability, 0, len(profiles))
	for _, p := range profiles {
		contract := capabilities(p)
		items = append(items, profiledCapability{p, contract, profileDigest(contract)})
	}
	respond(w, http.StatusOK, items, nil)
}

func areaDiagnostic(area areas.Revision, contract profileContract) string {
	if len(area.Components) == 0 || len(area.Components) > contract.MaxComponents {
		return "Select a polygon or multipolygon with at most 16 components."
	}
	west, east := math.Inf(1), math.Inf(-1)
	for _, part := range area.Components {
		west, east = min(west, part.Bounds[0]), max(east, part.Bounds[2])
		if part.Bounds[1] < -float64(contract.LatitudeLimit) || part.Bounds[3] > float64(contract.LatitudeLimit) {
			return "Select a non-polar area within 85 degrees north or south."
		}
	}
	if east-west >= float64(contract.LongitudeSpanLimit) {
		return "Split date-line or wide areas into separately prepared areas spanning less than 180 degrees."
	}
	return ""
}

type sourceQuality struct {
	AssetID       string `json:"assetId"`
	Layer         string `json:"layer"`
	DataTime      string `json:"dataTime"`
	HorizontalCRS string `json:"horizontalCrs"`
	VerticalDatum string `json:"verticalDatum"`
	Resolution    string `json:"nativeResolution"`
	ProviderClaim string `json:"providerUncertainty"`
	Semantics     string `json:"semantics"`
}

type qualityEvidence struct {
	Registration    string `json:"registration"`
	SelectedSamples *int64 `json:"selectedSamples"`
	ValidSamples    *int64 `json:"validSamples"`
	FeatureCount    *int64 `json:"featureCount"`
}

func (s Service) structuredReleaseQuality(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Read)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	j, err := s.job(r.Context(), rel.BuildID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if j.recipe.ProfileContract != nil && profileDigest(*j.recipe.ProfileContract) != j.recipe.ProfileContractSHA256 {
		httpx.Error(w, apperr.Conflict("retained profile contract identity differs"))
		return
	}
	var manifest struct {
		Manifest
		Members []struct {
			Member
			Component *int            `json:"component"`
			Details   json.RawMessage `json:"details"`
		} `json:"members"`
	}
	if err := json.Unmarshal(rel.Manifest, &manifest); err != nil {
		httpx.Error(w, apperr.Conflict("release quality is unavailable"))
		return
	}
	inputs := make([]sourceQuality, 0, len(manifest.Inputs))
	for _, input := range manifest.Inputs {
		rep := input.Representation
		inputs = append(inputs, sourceQuality{input.ID, rep.Layer, rep.DataTime, rep.CRS, rep.VerticalDatum, rep.Resolution, rep.Uncertainty, rep.Semantics})
	}
	coverage := make([]map[string]any, 0, len(manifest.Members))
	for _, member := range manifest.Members {
		var details qualityEvidence
		summary := map[string]any{"member": member.Name, "layer": member.Layer, "component": member.Component, "state": "UNKNOWN"}
		if json.Unmarshal(member.Details, &details) == nil {
			if details.SelectedSamples != nil && details.ValidSamples != nil && *details.SelectedSamples > 0 && *details.ValidSamples >= 0 && *details.ValidSamples <= *details.SelectedSamples {
				summary["state"] = "SAMPLE_MASK_RECORDED"
				summary["selectedSamples"], summary["validSamples"] = *details.SelectedSamples, *details.ValidSamples
				summary["validFraction"] = float64(*details.ValidSamples) / float64(*details.SelectedSamples)
				summary["registration"] = details.Registration
			} else if details.FeatureCount != nil && *details.FeatureCount >= 0 {
				summary["state"], summary["featureCount"] = "FEATURE_COUNT_RECORDED", *details.FeatureCount
			}
		}
		coverage = append(coverage, summary)
	}
	out := map[string]any{"schema": "emad.geographic-quality.v1", "manifestSha256": rel.ManifestSHA256,
		"profileId": manifest.ProfileID, "profileContractSha256": j.recipe.ProfileContractSHA256,
		"coordinateCheck":  map[string]any{"maximumErrorM": manifest.Quality.CoordinateErrorM, "limitM": .001, "statistic": "maximum control disagreement"},
		"physicalAccuracy": manifest.Quality.PhysicalAccuracy, "frame": manifest.Frame, "sources": inputs, "members": manifest.Members, "coverage": coverage}
	if j.recipe.ProfileContract != nil {
		out["profileContract"] = j.recipe.ProfileContract
	}
	respond(w, http.StatusOK, out, nil)
}
