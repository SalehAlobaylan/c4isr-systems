package geoenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The shared corpus is the same file the offline Python reader evaluates. In
// both engines the expected decisions come from the requirement rules, not from
// either implementation.
type suitabilityCases struct {
	Schema         string `json:"schema"`
	Fixture        string `json:"fixture"`
	AreaRevisionID string `json:"areaRevisionId"`
	Cases          []struct {
		ID          string         `json:"id"`
		Derivation  string         `json:"derivation"`
		Requirement map[string]any `json:"requirement"`
		Evidence    map[string]any `json:"evidence"`
		Now         *string        `json:"now"`
		Expected    struct {
			Overall         string `json:"overall"`
			Coverage        *float64
			Decisions       []map[string]any `json:"decisions"`
			OptionalReasons []string         `json:"optionalReasons"`
		} `json:"expected"`
	} `json:"cases"`
}

func readSuitabilityCases(t *testing.T) suitabilityCases {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "geospatial", "suitability-cases", "cases.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document suitabilityCases
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.Schema != "emad.environment-suitability-cases.v1" || len(document.Cases) == 0 {
		t.Fatalf("unexpected shared vector document: %s", document.Schema)
	}
	return document
}

func decodeShared(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEvidenceDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestSuitabilitySharedVectors(t *testing.T) {
	for _, vector := range readSuitabilityCases(t).Cases {
		t.Run(vector.ID, func(t *testing.T) {
			now := (*time.Time)(nil)
			if vector.Now != nil {
				parsed, ok := parseInstant(*vector.Now)
				if !ok {
					t.Fatalf("unparseable evaluation instant %q", *vector.Now)
				}
				now = &parsed
			}
			result, err := EvaluateSuitability(decodeShared(t, vector.Requirement), decodeShared(t, vector.Evidence), now)
			if err != nil {
				t.Fatal(err)
			}
			if result.Schema != suitabilityResultSchema {
				t.Fatalf("schema %q", result.Schema)
			}
			if result.Overall != vector.Expected.Overall {
				t.Fatalf("overall %q, want %q", result.Overall, vector.Expected.Overall)
			}
			if vector.Expected.Coverage == nil {
				if result.Coverage != nil {
					t.Fatalf("coverage %v, want null", *result.Coverage)
				}
			} else if result.Coverage == nil || *result.Coverage != *vector.Expected.Coverage {
				t.Fatalf("coverage %v, want %v", result.Coverage, *vector.Expected.Coverage)
			}
			if len(result.Decisions) != len(vector.Expected.Decisions) {
				t.Fatalf("decisions %d, want %d", len(result.Decisions), len(vector.Expected.Decisions))
			}
			for index, wanted := range vector.Expected.Decisions {
				got := result.Decisions[index]
				for key, value := range wanted {
					if !scalarEqual(got[key], value) {
						t.Fatalf("decision %d %s = %v, want %v", index, key, got[key], value)
					}
				}
			}
			reasons := make([]string, 0, len(result.Optional))
			for _, item := range result.Optional {
				reasons = append(reasons, evidenceString(item["reason"]))
			}
			if len(reasons) != len(vector.Expected.OptionalReasons) {
				t.Fatalf("optional reasons %v, want %v", reasons, vector.Expected.OptionalReasons)
			}
			for index, reason := range vector.Expected.OptionalReasons {
				if reasons[index] != reason {
					t.Fatalf("optional reasons %v, want %v", reasons, vector.Expected.OptionalReasons)
				}
			}
			for _, item := range result.Decisions {
				if _, ok := SuitabilityReasons[evidenceString(item["reason"])]; !ok {
					t.Fatalf("undeclared reason %v", item["reason"])
				}
			}
		})
	}
}

// scalarEqual compares shared-vector expectations, which are JSON strings,
// booleans, null or numbers.
func scalarEqual(got, want any) bool {
	if want == nil {
		return got == nil
	}
	switch expected := want.(type) {
	case string:
		text, ok := got.(string)
		return ok && text == expected
	case bool:
		value, ok := got.(bool)
		return ok && value == expected
	case float64:
		value, ok := evidenceNumber(got)
		return ok && value == expected
	}
	return false
}

func TestSuitabilityDigestMatchesOfflineReader(t *testing.T) {
	// These digests were produced by scripts/data/environment_suitability.py
	// from the shared vectors. Both readers must hash the same evidence bytes.
	expected := map[string]string{
		"surface-native-inspection": "cb8b8fd4fbb936fa76831eaad34310e4c647fb50faa2d81c4d2798cfcdf3a6d4",
		"ground-accuracy-evidenced": "0a87c2febf32ccee93e8407a9c939423a4d23a88bc69094f31dfd74539e33acb",
	}
	for _, vector := range readSuitabilityCases(t).Cases {
		want, ok := expected[vector.ID]
		if !ok {
			continue
		}
		if got := SuitabilityEvidenceDigest(decodeShared(t, vector.Evidence)); got != want {
			t.Fatalf("%s evidence digest %s, want %s", vector.ID, got, want)
		}
	}
}

func TestSuitabilityOptionalOmissionDoesNotChangeResult(t *testing.T) {
	for _, vector := range readSuitabilityCases(t).Cases {
		if vector.ID != "surface-native-inspection" {
			continue
		}
		result, err := EvaluateSuitability(vector.Requirement, vector.Evidence, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Overall != "supported" || len(result.Optional) != 1 ||
			result.Optional[0]["result"] != DecisionOmitted {
			t.Fatalf("unexpected optional handling: %+v", result)
		}
		if result.Release["manifestSha256"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
			result.Release["areaRevisionId"] != "owned-fixture-area-v1" ||
			result.Release["profileId"] != "reference-surface-v1" {
			t.Fatalf("result release identity is missing: %+v", result.Release)
		}
		return
	}
	t.Fatal("shared vector surface-native-inspection is missing")
}

func TestSuitabilityDigestIgnoresAssessmentsAndTerms(t *testing.T) {
	for _, vector := range readSuitabilityCases(t).Cases {
		if vector.ID != "ground-accuracy-evidenced" {
			continue
		}
		first := SuitabilityEvidenceDigest(vector.Evidence)
		altered := decodeShared(t, vector.Evidence)
		altered["assessments"] = []any{}
		altered["terms"] = map[string]any{"expiresAt": "2000-01-01T00:00:00Z"}
		if second := SuitabilityEvidenceDigest(altered); second != first {
			t.Fatalf("evidence digest changed: %s != %s", first, second)
		}
		members := evidenceArray(altered["members"])
		evidenceObject(members[0])["bytes"] = 1
		if third := SuitabilityEvidenceDigest(altered); third == first {
			t.Fatal("member byte change did not change the release digest")
		}
		if len(first) != 64 {
			t.Fatalf("digest length %d", len(first))
		}
		return
	}
	t.Fatal("shared vector ground-accuracy-evidenced is missing")
}

func TestSuitabilityRejectsUnknownSchemas(t *testing.T) {
	document := readSuitabilityCases(t)
	vector := document.Cases[0]
	if _, err := EvaluateSuitability(map[string]any{"schema": "other"}, vector.Evidence, nil); err == nil {
		t.Fatal("unknown requirement schema accepted")
	}
	if _, err := EvaluateSuitability(vector.Requirement, map[string]any{"schema": "other"}, nil); err == nil {
		t.Fatal("unknown evidence schema accepted")
	}
}

func TestSuitabilityRequirementAdmission(t *testing.T) {
	vector := readSuitabilityCases(t).Cases[0]
	requirement := decodeShared(t, vector.Requirement)
	if !validSuitabilityRequirement(requirement) {
		t.Fatal("shared requirement rejected")
	}
	for name, change := range map[string]func(map[string]any){
		"schema":  func(in map[string]any) { in["schema"] = "other" },
		"id":      func(in map[string]any) { delete(in, "requirementId") },
		"release": func(in map[string]any) { in["release"] = map[string]any{} },
		"reps":    func(in map[string]any) { in["representations"] = []any{} },
	} {
		t.Run(name, func(t *testing.T) {
			changed := decodeShared(t, vector.Requirement)
			change(changed)
			if validSuitabilityRequirement(changed) {
				t.Fatal("invalid requirement admitted")
			}
		})
	}
}

func TestSuitabilityHoleIsOutsideReleaseArea(t *testing.T) {
	document := readSuitabilityCases(t)
	for _, vector := range document.Cases {
		if vector.ID != "surface-native-inspection" {
			continue
		}
		area := evidenceObject(vector.Evidence["area"])
		inside := [][2]float64{{46.705, 24.705}, {46.790, 24.705}}
		for _, point := range inside {
			if !insideGeometry(area, point[0], point[1]) {
				t.Fatalf("expected inside at %v", point)
			}
		}
		outside := [][2]float64{{46.725, 24.725}, {46.765, 24.705}}
		for _, point := range outside {
			if insideGeometry(area, point[0], point[1]) {
				t.Fatalf("expected outside at %v", point)
			}
		}
		return
	}
	t.Fatal("shared vector surface-native-inspection is missing")
}

func TestSuitabilityCorrectionUsesNewAssessment(t *testing.T) {
	for _, vector := range readSuitabilityCases(t).Cases {
		if vector.ID != "assessment-superseded" {
			continue
		}
		result, err := EvaluateSuitability(vector.Requirement, vector.Evidence, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range result.Decisions {
			if item["requirement"] != "ground" {
				continue
			}
			if item["reason"] != "accuracy-exceeded" || item["assessment"] != "assessment-ground-2026-02" {
				t.Fatalf("correction not applied: %+v", item)
			}
			return
		}
		t.Fatal("ground decision missing")
	}
	t.Fatal("shared vector assessment-superseded is missing")
}
