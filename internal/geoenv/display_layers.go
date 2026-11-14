package geoenv

import (
	"bytes"
	"encoding/json"
	"image/color"
	"image/png"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type displayEncoding struct{ layer, format, media, extension string }

func displayFormat(recipe string) displayEncoding {
	switch recipe {
	case "terrain-glb-v1", "terrain-glb-v2":
		return displayEncoding{"surface", "display-glb", displayMedia, ".glb"}
	case "ground-glb-v1":
		return displayEncoding{"ground", "display-glb", displayMedia, ".glb"}
	case "radar-display-v1":
		return displayEncoding{"radar", "display-layer-json", "application/json", ".json"}
	case "imagery-display-v1", "roads-display-v1", "buildings-display-v1", "water-display-v1", "land-cover-display-v1", "ground-delta-display-v1":
		return displayEncoding{strings.TrimSuffix(recipe, "-display-v1"), "display-layer-json", "application/json", ".json"}
	case "street-pmtiles-v1":
		return displayEncoding{"streets", "display-pmtiles", "application/vnd.pmtiles", ".pmtiles"}
	}
	return displayEncoding{}
}

var opticalBands = []string{"B04_10m", "B03_10m", "B02_10m", "SCL_20m"}

func displayDependencies(m Manifest, recipe string) ([]DisplayDependency, bool) {
	if recipe == "terrain-glb-v1" || recipe == "terrain-glb-v2" {
		return terrainDependencies(m)
	}
	if recipe == "ground-glb-v1" {
		var expected []DisplayDependency
		var ground, geoid bool
		for _, member := range m.Members {
			if member.Layer == "ground" || member.Layer == "geoid" {
				expected = append(expected, DisplayDependency{member.Name, member.SHA256})
				ground = ground || member.Layer == "ground"
				geoid = geoid || member.Layer == "geoid"
			}
		}
		return expected, ground && geoid && m.ProfileID == enrichmentProfile
	}
	layer := displayFormat(recipe).layer
	var expected []DisplayDependency
	var available, surface bool
	var bands []string
	streets := map[string]bool{}
	for _, member := range m.Members {
		if layer == "land-cover" || layer == "radar" {
			if member.Layer != layer {
				continue
			}
		} else if layer == "ground-delta" {
			if member.Layer != "ground" && member.Layer != "surface" {
				continue
			}
			available = available || member.Layer == "ground"
		} else if layer == "streets" {
			if !slices.Contains([]string{"roads", "buildings", "water"}, member.Layer) {
				continue
			}
			if streets[member.Layer] || parentRepresentation(m, member.InputID).Producer != "OpenStreetMap contributors" {
				return nil, false
			}
			streets[member.Layer] = true
		} else if layer == "imagery" {
			if member.Layer != layer {
				continue
			}
			_, band, ok := strings.Cut(parentRepresentation(m, member.InputID).ProductEdition, " ")
			if !ok || !slices.Contains(opticalBands, band) {
				continue
			}
			bands = append(bands, band)
		} else if member.Layer != layer && member.Layer != "surface" && member.Layer != "geoid" {
			continue
		}
		available = available || member.Layer == layer
		surface = surface || member.Layer == "surface"
		expected = append(expected, DisplayDependency{member.Name, member.SHA256})
	}
	if layer == "imagery" {
		return expected, len(bands) == len(opticalBands) && slices.Contains(bands, opticalBands[0]) &&
			slices.Contains(bands, opticalBands[1]) && slices.Contains(bands, opticalBands[2]) && slices.Contains(bands, opticalBands[3])
	}
	if layer == "streets" {
		return expected, len(streets) == 3
	}
	if layer == "land-cover" {
		return expected, available && m.ProfileID == enrichmentProfile
	}
	if layer == "radar" {
		return expected, available && len(expected) == 1 && m.ProfileID == observationProfile
	}
	return expected, available && surface
}

func parentRepresentation(m Manifest, id string) Representation {
	for _, input := range m.Inputs {
		if input.ID == id {
			return input.Representation
		}
	}
	return Representation{}
}

func strictDisplayJSON(raw []byte, target any) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(target) == nil && dec.Decode(new(any)) == io.EOF
}

func validDisplayBytes(raw []byte, d Display, parent Manifest) bool {
	if displayFormat(d.Recipe).format == "display-glb" {
		return validTerrainGLB(raw)
	}
	if d.Recipe == "street-pmtiles-v1" {
		return validStreetArchive(raw, d.Bounds)
	}
	var envelope struct {
		Schema         string          `json:"schema"`
		ManifestSHA256 string          `json:"manifestSha256"`
		Layer          string          `json:"layer"`
		Data           json.RawMessage `json:"data"`
	}
	layer := displayFormat(d.Recipe).layer
	if layer == "" || !strictDisplayJSON(raw, &envelope) || envelope.Schema != "emad.display-layer.v1" ||
		envelope.ManifestSHA256 != d.ManifestSHA256 || envelope.Layer != layer {
		return false
	}
	if layer == "imagery" {
		return validOptical(envelope.Data, d.Bounds, parent)
	}
	if layer == "land-cover" || layer == "ground-delta" {
		return validEnrichmentTexture(envelope.Data, layer, d.Bounds, parent)
	}
	if layer == "radar" {
		return validRadarDisplay(envelope.Data, d.Bounds, parent)
	}
	return validDisplayVectors(envelope.Data, layer, d.Bounds, parent)
}

func validRadarDisplay(raw []byte, bounds [4]float64, parent Manifest) bool {
	var v struct {
		Bounds                     [4]float64
		Width, Height, ValidPixels int
		Texture                    []byte
		Radar                      json.RawMessage
		DisplayDBRange             []int
		Method                     string
	}
	if parent.ProfileID != observationProfile || !strictDisplayJSON(raw, &v) || !jsonEqual(v.Radar, parent.Quality.Observations.Radar) ||
		!slices.Equal(v.DisplayDBRange, []int{-30, 5}) || v.Method != "10 log10(linear power), clipped grayscale; nearest neighbour; invalid samples transparent" ||
		v.Width < 2 || v.Width > 1024 || v.Height < 2 || v.Height > 1024 || v.ValidPixels < 1 || len(v.Texture) > 5<<20 {
		return false
	}
	for i, b := range bounds {
		if math.Abs(b-v.Bounds[i]) > 1e-9 {
			return false
		}
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(v.Texture))
	if err != nil || cfg.Width != v.Width || cfg.Height != v.Height || cfg.ColorModel != color.NRGBAModel {
		return false
	}
	img, err := png.Decode(bytes.NewReader(v.Texture))
	if err != nil {
		return false
	}
	count := 0
	for y := range v.Height {
		for x := range v.Width {
			r, g, b, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if a != 0xffff || r != g || g != b {
				return false
			}
			count++
		}
	}
	return count == v.ValidPixels
}

func validOptical(raw []byte, bounds [4]float64, m Manifest) bool {
	var optical struct {
		Scene, AcquiredAt, SceneMetadataSHA256, Grid, Color string
		Bands                                               []struct {
			ID            string
			Scale, Offset float64
		}
		ValidSCL                   []int
		Bounds                     [4]float64
		Width, Height, ValidPixels int
		Texture                    []byte
	}
	if !strictDisplayJSON(raw, &optical) || optical.Scene == "" || !digestPattern.MatchString(optical.SceneMetadataSHA256) ||
		optical.Grid != "EPSG:4326 pixel-is-area, north-up, nearest DN and SCL; AOI and native masks applied" ||
		optical.Color != "Surface reflectance / 0.35, clamped and encoded as sRGB" ||
		len(optical.Bands) != 3 || !slices.Equal(optical.ValidSCL, []int{4, 5, 6, 7}) ||
		optical.Width < 2 || optical.Width > 1024 || optical.Height < 2 || optical.Height > 1024 ||
		len(optical.Texture) > 5<<20 {
		return false
	}
	for i, edge := range bounds {
		if math.Abs(edge-optical.Bounds[i]) > 1e-9 {
			return false
		}
	}
	for i, band := range optical.Bands {
		if band.ID != opticalBands[i] || band.Scale <= 0 || band.Scale > .01 || math.Abs(band.Offset) > .5 {
			return false
		}
	}
	for _, band := range opticalBands {
		if !slices.ContainsFunc(m.Inputs, func(a Asset) bool {
			return a.Representation.Layer == "imagery" && a.Representation.ProductEdition == optical.Scene+" "+band &&
				a.Representation.DataTime == optical.AcquiredAt
		}) {
			return false
		}
	}
	config, err := png.DecodeConfig(bytes.NewReader(optical.Texture))
	if err != nil || config.Width != optical.Width || config.Height != optical.Height || config.ColorModel != color.NRGBAModel {
		return false
	}
	decoded, err := png.Decode(bytes.NewReader(optical.Texture))
	if err != nil {
		return false
	}
	valid := 0
	for y := range optical.Height {
		for x := range optical.Width {
			_, _, _, alpha := decoded.At(x, y).RGBA()
			switch alpha {
			case 0xffff:
				valid++
			case 0:
			default:
				return false
			}
		}
	}
	return valid > 0 && valid == optical.ValidPixels
}

func validDisplayVectors(raw []byte, layer string, bounds [4]float64, m Manifest) bool {
	var collection struct {
		Type, Method                                   string
		AvailableFeatures, DisplayedFeatures, Vertices int
		MaxFrameControlErrorM                          float64
		Features                                       []struct {
			Type, ID string
			Geometry struct {
				Type        string
				Coordinates json.RawMessage
			}
			Properties struct {
				SourceID, Edition, Product, DataTime, Producer string
				Tags                                           map[string]string
				Height                                         struct {
					Kind   string
					Metres *float64
				}
				Dimensions map[string]struct {
					Value                               *float64
					Unit, Origin, Evidence, Uncertainty string
				}
			}
			Paths [][][]float64
		}
	}
	if !strictDisplayJSON(raw, &collection) || collection.Type != "FeatureCollection" ||
		collection.Method != "Retained WGS84 topology; surface triangle interpolation with 0.6 m display offset" ||
		collection.AvailableFeatures != len(collection.Features) || len(collection.Features) > 30000 || collection.Features == nil ||
		collection.MaxFrameControlErrorM < 0 || collection.MaxFrameControlErrorM > .001 {
		return false
	}
	seen := make(map[string]bool, len(collection.Features))
	displayed, vertices, geographicVertices := 0, 0, 0
	for _, feature := range collection.Features {
		p := feature.Properties
		if feature.Type != "Feature" || feature.ID != p.Edition+":"+p.SourceID || seen[feature.ID] ||
			!keyPattern.MatchString(p.SourceID) || !digestPattern.MatchString(p.Edition) || p.Tags == nil || len(p.Tags) > 500 ||
			!slices.ContainsFunc(m.Inputs, func(a Asset) bool {
				r := a.Representation
				return a.SHA256 == p.Edition && r.Layer == layer && r.ProductEdition == p.Product && r.DataTime == p.DataTime && r.Producer == p.Producer
			}) || !validVectorGeometry(feature.Geometry.Type, feature.Geometry.Coordinates, bounds, &geographicVertices) {
			return false
		}
		seen[feature.ID] = true
		if m.ProfileID == enrichmentProfile || p.Dimensions != nil {
			spec := dimensionTags[layer]
			if len(p.Dimensions) != len(spec) {
				return false
			}
			for name, rule := range spec {
				d, ok := p.Dimensions[name]
				tag, hasTag := p.Tags[rule.tag]
				if !ok || d.Unit != rule.unit || d.Uncertainty != "UNKNOWN" {
					return false
				}
				expected, hasValue := normalizeDimension(tag, rule.tag, rule.unit, rule.max)
				if hasValue != (d.Value != nil) || hasValue && math.Abs(expected-*d.Value) > .00000051 {
					return false
				}
				if d.Origin == "provider-stated" {
					if !hasTag || d.Value == nil || *d.Value < 0 || *d.Value == 0 && name != "minHeight" || *d.Value > rule.max ||
						rule.unit == "count" && *d.Value != math.Trunc(*d.Value) || d.Evidence != "source tag: "+rule.tag {
						return false
					}
				} else if d.Origin != "unknown" || d.Value != nil || hasTag && d.Evidence != "unsupported source tag: "+rule.tag ||
					!hasTag && d.Evidence != "missing source tag: "+rule.tag {
					return false
				}
			}
			if layer == "buildings" {
				value := p.Dimensions["height"].Value
				if (value == nil) != (p.Height.Metres == nil) || value != nil && *value != *p.Height.Metres {
					return false
				}
			}
		}
		if p.Height.Kind == "unknown" && p.Height.Metres != nil || p.Height.Kind == "source-tag" &&
			(p.Height.Metres == nil || *p.Height.Metres <= 0 || *p.Height.Metres > 2000 || p.Tags["height"] == "") ||
			p.Height.Kind != "unknown" && p.Height.Kind != "source-tag" {
			return false
		}
		if len(feature.Paths) > 0 {
			displayed++
		}
		for _, path := range feature.Paths {
			vertices += len(path)
			if len(path) < 2 || vertices > 350000 {
				return false
			}
			for _, p := range path {
				if len(p) != 3 || math.Abs(p[0]) > 200000 || math.Abs(p[1]) > 20000 || math.Abs(p[2]) > 200000 {
					return false
				}
			}
		}
	}
	return displayed == collection.DisplayedFeatures && vertices == collection.Vertices
}

var dimensionTags = map[string]map[string]struct {
	tag, unit string
	max       float64
}{
	"buildings": {"height": {"height", "m", 2000}, "minHeight": {"min_height", "m", 2000}, "levels": {"building:levels", "count", 300}, "roofHeight": {"roof:height", "m", 500}},
	"roads":     {"width": {"width", "m", 1000}, "clearance": {"maxheight", "m", 100}, "lanes": {"lanes", "count", 50}},
	"water":     {"width": {"width", "m", 10000}, "depth": {"depth", "m", 12000}},
}

var dimensionMetric = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*m?$`)
var dimensionFeet = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*ft$`)
var dimensionFeetInches = regexp.MustCompile(`^([0-9]+)'(?:\s*([0-9]+(?:\.[0-9]+)?)")?$`)
var dimensionCount = regexp.MustCompile(`^[0-9]+$`)

func normalizeDimension(raw, tag, unit string, limit float64) (float64, bool) {
	if len(raw) > 64 {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	var n float64
	switch {
	case unit == "count":
		if !dimensionCount.MatchString(raw) {
			return 0, false
		}
		n, _ = strconv.ParseFloat(raw, 64)
	default:
		if m := dimensionMetric.FindStringSubmatch(raw); m != nil {
			n, _ = strconv.ParseFloat(m[1], 64)
		} else if m := dimensionFeet.FindStringSubmatch(raw); m != nil {
			n, _ = strconv.ParseFloat(m[1], 64)
			n *= .3048
		} else if m := dimensionFeetInches.FindStringSubmatch(raw); m != nil {
			feet, _ := strconv.ParseFloat(m[1], 64)
			inches, _ := strconv.ParseFloat(m[2], 64)
			if inches >= 12 {
				return 0, false
			}
			n = feet*.3048 + inches*.0254
		} else {
			return 0, false
		}
	}
	return n, n <= limit && (n > 0 || tag == "min_height")
}

func validEnrichmentTexture(raw []byte, layer string, bounds [4]float64, m Manifest) bool {
	var texture struct {
		Bounds                     [4]float64
		Width, Height, ValidPixels int
		Method                     string
		Texture                    []byte
		Legend                     []struct {
			Code       *int
			Label, RGB string
			Pixels     int
		}
		Sources    []struct{ Layer, Producer, Edition, DataTime string }
		Comparison json.RawMessage
	}
	if m.ProfileID != enrichmentProfile || !strictDisplayJSON(raw, &texture) ||
		texture.Method != "EPSG:4326 nearest neighbour; masked pixels are transparent" ||
		texture.Width < 2 || texture.Width > 1024 || texture.Height < 2 || texture.Height > 1024 ||
		len(texture.Texture) > 5<<20 || texture.ValidPixels <= 0 || len(texture.Sources) < 1 || len(texture.Sources) > 32 {
		return false
	}
	for i, edge := range bounds {
		if math.Abs(edge-texture.Bounds[i]) > 1e-9 {
			return false
		}
	}
	type entry struct {
		code       int
		label, rgb string
	}
	legend := []entry{{10, "Tree cover", "006400"}, {20, "Shrubland", "ffbb22"}, {30, "Grassland", "ffff4c"},
		{40, "Cropland", "f096ff"}, {50, "Built-up", "fa0000"}, {60, "Bare / sparse vegetation", "b4b4b4"},
		{70, "Snow and ice", "f0f0f0"}, {80, "Permanent water bodies", "0064c8"}, {90, "Herbaceous wetland", "0096a0"},
		{95, "Mangroves", "00cf75"}, {100, "Moss and lichen", "fae6a0"}}
	allowed := []string{"land-cover"}
	if layer == "ground-delta" {
		allowed = []string{"ground", "surface"}
		legend = []entry{{-10, "< −10 m", "20458a"}, {-3, "−10 to < −3 m", "508ac6"}, {-1, "−3 to < −1 m", "b3d4e4"},
			{1, "−1 to < +1 m", "f1eee5"}, {3, "+1 to < +3 m", "efbd7b"}, {10, "+3 to < +10 m", "d56b37"},
			{0, "≥ +10 m", "92292c"}}
		if len(m.Quality.Enrichment.GroundComparison) == 0 || !jsonEqual(texture.Comparison, m.Quality.Enrichment.GroundComparison) {
			return false
		}
	} else if !jsonEqual(texture.Comparison, json.RawMessage("[]")) {
		return false
	}
	for _, source := range texture.Sources {
		if !slices.Contains(allowed, source.Layer) || !slices.ContainsFunc(m.Inputs, func(a Asset) bool {
			r := a.Representation
			return r.Layer == source.Layer && r.Producer == source.Producer && r.ProductEdition == source.Edition && r.DataTime == source.DataTime
		}) {
			return false
		}
	}
	for _, kind := range allowed {
		if !slices.ContainsFunc(texture.Sources, func(s struct{ Layer, Producer, Edition, DataTime string }) bool { return s.Layer == kind }) {
			return false
		}
	}
	if len(legend) != len(texture.Legend) {
		return false
	}
	counts := map[uint32]int{}
	for i, item := range texture.Legend {
		choice := legend[i]
		if item.Label != choice.label || item.RGB != choice.rgb || item.Pixels < 0 ||
			(choice.code == 0) != (item.Code == nil) || item.Code != nil && *item.Code != choice.code {
			return false
		}
		rgb, _ := strconv.ParseUint(item.RGB, 16, 32)
		counts[uint32(rgb)] = item.Pixels
	}
	config, err := png.DecodeConfig(bytes.NewReader(texture.Texture))
	if err != nil || config.Width != texture.Width || config.Height != texture.Height || config.ColorModel != color.NRGBAModel {
		return false
	}
	decoded, err := png.Decode(bytes.NewReader(texture.Texture))
	if err != nil {
		return false
	}
	valid := 0
	for y := range texture.Height {
		for x := range texture.Width {
			r, g, b, a := decoded.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			rgb := (r>>8)<<16 | (g>>8)<<8 | b>>8
			if a != 0xffff || counts[rgb] <= 0 {
				return false
			}
			counts[rgb]--
			valid++
		}
	}
	return valid == texture.ValidPixels && !slices.ContainsFunc(texture.Legend, func(item struct {
		Code       *int
		Label, RGB string
		Pixels     int
	}) bool {
		rgb, _ := strconv.ParseUint(item.RGB, 16, 32)
		return counts[uint32(rgb)] != 0
	})
}

func validVectorGeometry(kind string, raw json.RawMessage, bounds [4]float64, vertices *int) bool {
	line := func(points [][]float64, ring bool) bool {
		*vertices += len(points)
		if len(points) < 2 || *vertices > 350000 {
			return false
		}
		for _, p := range points {
			if len(p) != 2 || p[0] < bounds[0]-1e-8 || p[0] > bounds[2]+1e-8 || p[1] < bounds[1]-1e-8 || p[1] > bounds[3]+1e-8 {
				return false
			}
		}
		return !ring || len(points) >= 4 && slices.Equal(points[0], points[len(points)-1])
	}
	group := func(lines [][][]float64, ring bool) bool {
		return len(lines) > 0 && !slices.ContainsFunc(lines, func(points [][]float64) bool { return !line(points, ring) })
	}
	switch kind {
	case "LineString":
		var points [][]float64
		return json.Unmarshal(raw, &points) == nil && line(points, false)
	case "MultiLineString", "Polygon":
		var lines [][][]float64
		return json.Unmarshal(raw, &lines) == nil && group(lines, kind == "Polygon")
	case "MultiPolygon":
		var polygons [][][][]float64
		return json.Unmarshal(raw, &polygons) == nil && len(polygons) > 0 &&
			!slices.ContainsFunc(polygons, func(rings [][][]float64) bool { return !group(rings, true) })
	}
	return false
}
