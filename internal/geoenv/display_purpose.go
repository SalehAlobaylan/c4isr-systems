package geoenv

// Purpose-specific metadata for authorized geographic display derivatives.
// A display is a visual reference; it is never a source of native values.
// Fields follow the offline derivative descriptor so P05 can hand the same
// document to a disconnected reader.

import "slices"

// DisplayPurpose describes what a derived display may and may not be used for.
type DisplayPurpose struct {
	Schema               string                       `json:"schema"`
	Purpose              string                       `json:"purpose"`
	PurposeMeaning       string                       `json:"purposeMeaning"`
	Representation       string                       `json:"representation"`
	Recipe               string                       `json:"recipe"`
	ToolchainSHA256      string                       `json:"toolchainSha256"`
	Units                string                       `json:"units"`
	Axes                 []string                     `json:"axes"`
	Origin               []float64                    `json:"origin"`
	LocalOriginENU       []float64                    `json:"localOriginEnu"`
	BoundsLonLat         []float64                    `json:"boundsLonLat"`
	ParentMembers        []DisplayPurposeParent       `json:"parentMembers"`
	RenderingAdjustments []DisplayRenderingAdjustment `json:"renderingAdjustments"`
	FeatureMapping       string                       `json:"featureMapping,omitempty"`
	Limitations          []string                     `json:"limitations"`
	ComputationalUse     bool                         `json:"computationalUse"`
}

type DisplayRenderingAdjustment struct {
	Kind    string  `json:"kind"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Meaning string  `json:"meaning"`
}

type DisplayPurposeParent struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

func displayPurpose(d Display, format displayEncoding) DisplayPurpose {
	purpose := DisplayPurpose{
		Schema: "emad.geographic-derivative.v1", Purpose: "visual-reference",
		PurposeMeaning: "Rendered visual reference derived from retained native members; it is not a source of native values.",
		Representation: format.format, Recipe: d.Recipe, ToolchainSHA256: d.ToolchainSHA256,
		Units: "metre", Axes: []string{"east", "up", "south"}, Origin: []float64{},
		LocalOriginENU: []float64{0, 0, 0}, BoundsLonLat: []float64{},
		ParentMembers: []DisplayPurposeParent{}, RenderingAdjustments: []DisplayRenderingAdjustment{},
		Limitations: []string{
			"Visual reference only; inspect the named native member for values, datum and accuracy.",
			"Derived from the named parent members; this derivative is not independently validated for measurement.",
		},
		ComputationalUse: false,
	}
	if len(d.Frame.Origin) == 3 {
		purpose.Origin = slices.Clone(d.Frame.Origin)
	}
	purpose.BoundsLonLat = slices.Clone(d.Bounds[:])
	for _, parent := range d.Inputs {
		purpose.ParentMembers = append(purpose.ParentMembers, DisplayPurposeParent{Name: parent.Name, SHA256: parent.SHA256})
	}
	switch format.format {
	case "display-glb":
		purpose.RenderingAdjustments = append(purpose.RenderingAdjustments, DisplayRenderingAdjustment{
			Kind: "vertical-exaggeration", Value: 1, Unit: "ratio",
			Meaning: "Display geometry is not vertically exaggerated; it is still not a native elevation source."})
		purpose.Limitations = append(purpose.Limitations,
			"The terrain mesh is a rendering surface; native elevations require the surface or ground member.")
	case "display-layer-json":
		switch d.Recipe {
		case "roads-display-v1", "buildings-display-v1", "water-display-v1":
			purpose.RenderingAdjustments = append(purpose.RenderingAdjustments, DisplayRenderingAdjustment{
				Kind: "vertex-height-offset", Value: 0.6, Unit: "metre",
				Meaning: "Documented display drape offset; never applied to native reads."})
			purpose.FeatureMapping = "edition:sourceId"
			purpose.Limitations = append(purpose.Limitations,
				"Draped vertices carry a documented 0.6 m display offset and do not preserve native positions.")
		case "imagery-display-v1":
			purpose.Limitations = append(purpose.Limitations,
				"Colour stretch and resampling are display-only; native band values require the imagery members.")
		case "land-cover-display-v1":
			purpose.Limitations = append(purpose.Limitations,
				"Categorical colours are display-only; native land-cover codes require the land-cover member.")
		case "radar-display-v1":
			purpose.Limitations = append(purpose.Limitations,
				"Grey-scale clipping is display-only; native linear-power backscatter requires the radar member.")
		case "ground-delta-display-v1":
			purpose.Limitations = append(purpose.Limitations,
				"Delta colouring is display-only; native values require the ground and surface members.")
		}
	case "display-pmtiles":
		purpose.Limitations = append(purpose.Limitations,
			"Tiled display generalization omits features according to the display budget.")
	}
	return purpose
}

func withDisplayPurpose(d Display) Display {
	if d.Purpose != nil {
		return d
	}
	format := displayFormat(d.Recipe)
	if format.format == "" {
		// An unrecognized recipe cannot establish a purpose.
		return d
	}
	purpose := displayPurpose(d, format)
	d.Purpose = &purpose
	return d
}

func attachDisplayPurpose(displays []Display) {
	for index := range displays {
		displays[index] = withDisplayPurpose(displays[index])
	}
}
