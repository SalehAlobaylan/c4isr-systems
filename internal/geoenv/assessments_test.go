package geoenv

import (
	"archive/zip"
	"encoding/json"
	"io"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
)

const assessmentDigestA = "1111111111111111111111111111111111111111111111111111111111111111"

func validAssessmentInput() AssessmentInput {
	return AssessmentInput{
		ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MemberSHA256:   "2222222222222222222222222222222222222222222222222222222222222222",
		Layer:          "ground", Use: "independent-ground-elevation",
		Method: "GNSS campaign, held-out controls", ControlSetSHA256: assessmentDigestA,
		StatisticM: .12, UncertaintyM: .05,
		Area:     json.RawMessage(`{"type":"Polygon","coordinates":[[[46.7,24.7],[46.76,24.7],[46.76,24.76],[46.7,24.76],[46.7,24.7]]]}`),
		Decision: "ACCEPTED", Note: "Reviewed against held-out controls.",
	}
}

func TestAssessmentEvidenceValidation(t *testing.T) {
	if !validAssessmentEvidence(validAssessmentInput()) {
		t.Fatal("valid assessment rejected")
	}
	for name, change := range map[string]func(*AssessmentInput){
		"digest":    func(in *AssessmentInput) { in.MemberSHA256 = "short" },
		"layer":     func(in *AssessmentInput) { in.Layer = "Ground Layer" },
		"use":       func(in *AssessmentInput) { in.Use = " " },
		"method":    func(in *AssessmentInput) { in.Method = "" },
		"control":   func(in *AssessmentInput) { in.ControlSetSHA256 = "" },
		"statistic": func(in *AssessmentInput) { in.StatisticM = -1 },
		"uncertainty": func(in *AssessmentInput) {
			in.UncertaintyM = math.Inf(1)
		},
		"decision": func(in *AssessmentInput) { in.Decision = "MAYBE" },
		"note":     func(in *AssessmentInput) { in.Note = "" },
		"area":     func(in *AssessmentInput) { in.Area = json.RawMessage(`{`) },
	} {
		t.Run(name, func(t *testing.T) {
			in := validAssessmentInput()
			change(&in)
			if validAssessmentEvidence(in) {
				t.Fatal("invalid assessment accepted")
			}
		})
	}
}

func TestAssessmentCorrectionDoesNotRequireIdentityFields(t *testing.T) {
	correction := AssessmentCorrection{
		Method: "Corrected held-out controls", ControlSetSHA256: assessmentDigestA,
		StatisticM: .3, UncertaintyM: .1,
		Area:     json.RawMessage(`{"type":"Polygon","coordinates":[[[46.7,24.7],[46.76,24.7],[46.76,24.76],[46.7,24.76],[46.7,24.7]]]}`),
		Decision: "ACCEPTED", Note: "Recomputed from the same control set.",
	}
	if !validAssessmentCorrection(correction) {
		t.Fatal("valid correction rejected because it inherits release identity from the superseded record")
	}
	for name, change := range map[string]func(*AssessmentCorrection){
		"method":  func(in *AssessmentCorrection) { in.Method = "" },
		"control": func(in *AssessmentCorrection) { in.ControlSetSHA256 = "short" },
		"statistic": func(in *AssessmentCorrection) {
			in.StatisticM = math.NaN()
		},
		"decision": func(in *AssessmentCorrection) { in.Decision = "MAYBE" },
		"note":     func(in *AssessmentCorrection) { in.Note = "" },
		"area":     func(in *AssessmentCorrection) { in.Area = json.RawMessage(`{`) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := correction
			change(&changed)
			if validAssessmentCorrection(changed) {
				t.Fatal("invalid correction accepted")
			}
		})
	}
}

func TestAssessmentEvidenceDigestPinsTheRecord(t *testing.T) {
	in := validAssessmentInput()
	a := Assessment{AssessmentInput: in, ID: "geoassess_one", ReleaseID: "georelease_one",
		ReleaseManifestSHA256: in.ManifestSHA256, Sequence: 1}
	decoded, err := DecodeEvidenceDocument(in.Area)
	if err != nil {
		t.Fatal(err)
	}
	first := assessmentEvidenceDigest(a, decoded)
	a.SupersededBy = "geoassess_two"
	if second := assessmentEvidenceDigest(a, decoded); second != first {
		t.Fatal("derived supersededBy changed the pinned evidence digest")
	}
	a.StatisticM = .13
	if third := assessmentEvidenceDigest(a, decoded); third == first {
		t.Fatal("a changed statistic did not change the evidence digest")
	}
	if len(first) != 64 {
		t.Fatalf("digest length %d", len(first))
	}
}

func TestReleaseManifestEvidenceKeepsComponentAndDetails(t *testing.T) {
	raw := json.RawMessage(`{"schema":"emad.environment-release.v1","profileId":"reference-surface-v1",
		"areaRevisionId":"area-v1","area":{"type":"Polygon","coordinates":[[[46.7,24.7],[46.8,24.7],[46.8,24.8],[46.7,24.8],[46.7,24.7]]]},
		"inputs":[{"id":"geoasset_one","sourceId":"source_one","admissionId":"admission_one",
			"representation":{"layer":"surface","format":"geotiff","verticalDatum":"ellipsoid","dataTime":"2022","productEdition":"owned-surface-v1","crs":"EPSG:4326"}}],
		"members":[{"name":"raster-00-00.tif","layer":"surface","sha256":"` + assessmentDigestA + `","bytes":2093,
			"component":0,"details":{"verticalDatum":"ellipsoid","selectedSamples":100,"validSamples":99}}]}`)
	var manifest releaseManifestEvidence
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Members) != 1 || manifest.Members[0].Component == nil || *manifest.Members[0].Component != 0 {
		t.Fatalf("member component not decoded: %+v", manifest.Members)
	}
	var details map[string]any
	if err := json.Unmarshal(manifest.Members[0].Details, &details); err != nil {
		t.Fatal(err)
	}
	if details["verticalDatum"] != "ellipsoid" || details["validSamples"] != float64(99) {
		t.Fatalf("member details not decoded: %+v", details)
	}
	inputs := evidenceInputs(manifest.Manifest)
	if len(inputs) != 1 || inputs[0]["layer"] != "surface" || inputs[0]["verticalDatum"] != "ellipsoid" ||
		inputs[0]["dataTime"] != "2022" || inputs[0]["productEdition"] != "owned-surface-v1" {
		t.Fatalf("inputs not decoded: %+v", inputs)
	}
}

func TestBuildEvidenceMatchesOfflineReaderDigest(t *testing.T) {
	archive, err := zip.OpenReader(filepath.Join("..", "..", "testdata", "geospatial", "owned-fixture-v1", "prepared.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	var raw []byte
	for _, file := range archive.File {
		if file.Name != "manifest.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(raw) == 0 {
		t.Fatal("owned fixture manifest is missing")
	}
	var manifest releaseManifestEvidence
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	evidence, err := buildEvidence(manifest, artifacts.Hash(raw), []any{}, nil, []string{})
	if err != nil {
		t.Fatal(err)
	}
	// Produced by scripts/data/environment_reader.py evidence_from_manifest and
	// environment_suitability.release_digest for the committed owned fixture.
	const want = "bcf918ab071791821d9da6ceeef802a394c594063bcb3b8d2bc2009d6db4514c"
	if got := SuitabilityEvidenceDigest(evidence); got != want {
		t.Fatalf("evidence digest %s, want %s", got, want)
	}
	if evidence["inputs"].([]any)[0].(map[string]any)["dataTime"] != nil {
		t.Fatal("missing source times must stay explicit nulls like the offline reader")
	}
}

func TestDisplayPurposeDescribesDerivativeUse(t *testing.T) {
	terrain := Display{
		DisplayInput: DisplayInput{Recipe: "terrain-glb-v2",
			ToolchainSHA256: assessmentDigestA,
			Inputs:          []DisplayDependency{{Name: "raster-00-00.tif", SHA256: assessmentDigestA}}},
		Frame:  Frame{Origin: []float64{46.7, 24.7, 0}},
		Bounds: [4]float64{46.7, 24.7, 46.76, 24.76},
	}
	purpose := displayPurpose(terrain, displayFormat(terrain.Recipe))
	if purpose.Schema != "emad.geographic-derivative.v1" || purpose.Purpose != "visual-reference" ||
		purpose.ComputationalUse || purpose.Units != "metre" || purpose.Representation != "display-glb" ||
		len(purpose.ParentMembers) != 1 || purpose.ParentMembers[0].Name != "raster-00-00.tif" {
		t.Fatalf("unexpected terrain purpose: %+v", purpose)
	}
	if len(purpose.Axes) != 3 || purpose.Axes[0] != "east" || purpose.Axes[1] != "up" || purpose.Axes[2] != "south" {
		t.Fatalf("unexpected axes: %+v", purpose.Axes)
	}
	if len(purpose.Origin) != 3 {
		t.Fatalf("unexpected origin: %+v", purpose.Origin)
	}
	if len(purpose.LocalOriginENU) != 3 || purpose.LocalOriginENU[0] != 0 {
		t.Fatalf("unexpected local origin: %+v", purpose.LocalOriginENU)
	}
	if len(purpose.RenderingAdjustments) != 1 || purpose.RenderingAdjustments[0].Kind != "vertical-exaggeration" ||
		purpose.RenderingAdjustments[0].Value != 1 {
		t.Fatalf("unexpected rendering adjustment: %+v", purpose.RenderingAdjustments)
	}
	if len(purpose.BoundsLonLat) != 4 || purpose.BoundsLonLat[2] != 46.76 {
		t.Fatalf("unexpected bounds: %+v", purpose.BoundsLonLat)
	}
	if len(purpose.Limitations) < 3 {
		t.Fatalf("terrain limitations missing: %+v", purpose)
	}

	vector := terrain
	vector.Recipe = "roads-display-v1"
	roads := displayPurpose(vector, displayFormat(vector.Recipe))
	if roads.FeatureMapping != "edition:sourceId" || len(roads.RenderingAdjustments) != 1 ||
		roads.RenderingAdjustments[0].Kind != "vertex-height-offset" ||
		math.Abs(roads.RenderingAdjustments[0].Value-.6) > 1e-9 {
		t.Fatalf("vector rendering adjustments missing: %+v", roads)
	}

	imagery := terrain
	imagery.Recipe = "imagery-display-v1"
	optical := displayPurpose(imagery, displayFormat(imagery.Recipe))
	if optical.FeatureMapping != "" || len(optical.RenderingAdjustments) != 0 {
		t.Fatalf("raster imagery must not claim a vector drape or feature mapping: %+v", optical)
	}
	if !slices.Contains(optical.Limitations, "Colour stretch and resampling are display-only; native band values require the imagery members.") {
		t.Fatalf("imagery limitations missing: %+v", optical.Limitations)
	}
}

func TestAttachDisplayPurposeLeavesAmbiguousFactsUnknown(t *testing.T) {
	legacy := Display{DisplayInput: DisplayInput{Recipe: "street-pmtiles-v1"}, Bounds: [4]float64{46.7, 24.7, 46.8, 24.8}}
	items := []Display{legacy}
	attachDisplayPurpose(items)
	if items[0].Purpose == nil || items[0].Purpose.Recipe != "street-pmtiles-v1" {
		t.Fatal("legacy display purpose not established from the known recipe")
	}
	unknown := Display{DisplayInput: DisplayInput{Recipe: "unknown-recipe-v9"}}
	items = []Display{unknown}
	attachDisplayPurpose(items)
	if items[0].Purpose != nil {
		t.Fatal("unknown recipe must not claim a purpose")
	}
}
