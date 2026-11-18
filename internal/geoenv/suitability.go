package geoenv

// Consumer requirement decisions over a release evidence document. The rules
// in this file intentionally mirror scripts/data/environment_suitability.py;
// both read the shared vectors in testdata/geospatial/suitability-cases.
//
// Vocabulary:
//
//	supported    every required item passed
//	unsupported  a proven violation makes a required item unusable
//	unqualified  the evidence needed to decide is missing
//
// Spatial predicates use the stored release area (including holes) and a fixed
// 9x9 lattice over the requirement's bounding box. The lattice is a documented
// sampling predicate, not a claim of full raster coverage.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	suitabilitySchema         = "emad.environment-requirement.v1"
	suitabilityResultSchema   = "emad.environment-suitability.v1"
	suitabilityEvidenceSchema = "emad.environment-evidence.v1"
	suitabilityLattice        = 9

	DecisionPass        = "pass"
	DecisionUnsupported = "unsupported"
	DecisionUnqualified = "unqualified"
	DecisionOmitted     = "optional-omitted"
)

// SuitabilityReasons declares the contract reason codes. Adding one is
// additive; renaming or changing the meaning of an existing one is a schema
// change.
var SuitabilityReasons = map[string]string{
	"release-match":                   "release and area identity match the requirement",
	"release-mismatch":                "the requirement names a different release manifest",
	"area-revision-mismatch":          "the requirement names a different area revision",
	"terms-accepted":                  "issued terms cover the requested operation",
	"terms-missing":                   "no issued terms accompany this evidence",
	"terms-expiry-unknown":            "expiry was not stated for the evaluation instant",
	"terms-expired":                   "the issued terms have expired",
	"grant-missing":                   "the issued terms do not grant the requested operation",
	"grant-withdrawn":                 "the issued grant was withdrawn",
	"resource-accepted":               "members and bytes are within the declared limits",
	"resource-exceeded":               "the release exceeds the declared member or byte limit",
	"representation-present":          "a native representation of the required layer is present",
	"representation-absent":           "the required native layer is absent",
	"optional-representation-absent":  "an optional layer is absent",
	"optional-omitted":                "an optional requirement was omitted without affecting the result",
	"datum-mismatch":                  "no retained source uses the required vertical datum",
	"unit-unsupported":                "the required unit is not a native or exactly convertible unit",
	"mask-evidence-below-requirement": "valid-sample evidence is below the required fraction",
	"accuracy-unknown-declared":       "the consumer accepts the release's declared unknown accuracy",
	"accuracy-unevidenced":            "independent accuracy evidence is required but absent",
	"accuracy-exceeded":               "an accepted assessment exceeds the declared tolerance",
	"accuracy-evidenced":              "an accepted assessment covers the requirement and meets the tolerance",
	"assessment-area-insufficient":    "the assessment covers less than the requirement area",
	"assessment-member-mismatch":      "the assessment names a different member digest",
	"time-background-accepted":        "dated background use accepts mixed source times",
	"time-window-accepted":            "every required source falls inside the common window",
	"time-incompatible":               "a source time falls outside the required window",
	"time-unknown":                    "a required source has no declared time",
	"area-covered":                    "the requirement area lies within the release area",
	"area-partially-covered":          "part of the requirement area lies outside the release area",
	"area-outside-release":            "the requirement area lies outside the release area",
	"area-partially-covered-accepted": "partial area coverage was explicitly accepted",
	"area-invalid":                    "the requirement area has no inside samples",
	"visual-only-derivative":          "only a visual derivative exists for this layer",
}

// SuitabilityDecision is one evaluated requirement. The common keys are always
// present; reason-specific evidence keys follow the offline reader.
type SuitabilityDecision map[string]any

// SuitabilityResult is the versioned answer returned by both readers.
type SuitabilityResult struct {
	Schema         string                `json:"schema"`
	RequirementID  any                   `json:"requirementId"`
	Operation      any                   `json:"operation"`
	Overall        string                `json:"overall"`
	Release        map[string]any        `json:"release"`
	EvidenceSHA256 string                `json:"evidenceSha256"`
	Terms          any                   `json:"terms"`
	Coverage       *float64              `json:"coverage"`
	Decisions      []SuitabilityDecision `json:"decisions"`
	Optional       []SuitabilityDecision `json:"optional"`
}

// SuitabilityEvidenceDigest hashes the exact release identity, area, member and
// input evidence used by decisions. Assessments and terms are excluded so a new
// review or expiry does not rewrite the release identity.
func SuitabilityEvidenceDigest(evidence map[string]any) string {
	subset := map[string]any{"schema": evidence["schema"]}
	for _, key := range []string{"manifestSha256", "areaRevisionId", "area"} {
		if value, ok := evidence[key]; ok {
			subset[key] = value
		} else {
			subset[key] = nil
		}
	}
	for _, key := range []string{"members", "inputs"} {
		if value, ok := evidence[key]; ok {
			subset[key] = value
		} else {
			subset[key] = []any{}
		}
	}
	return canonicalDigest(subset)
}

// canonicalDigest produces the same bytes as Python's
// json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)
// followed by SHA-256. Callers must decode documents with UseNumber so numeric
// spelling is retained.
func canonicalDigest(value any) string {
	var b strings.Builder
	writeCanonical(&b, value)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func writeCanonical(b *strings.Builder, value any) {
	switch v := value.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		writeCanonicalString(b, v)
	case json.Number:
		b.WriteString(v.String())
	case json.RawMessage:
		var decoded any
		decoder := json.NewDecoder(strings.NewReader(string(v)))
		decoder.UseNumber()
		if decoder.Decode(&decoded) != nil {
			b.WriteString("null")
			return
		}
		writeCanonical(b, decoded)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			b.WriteString("null")
		} else {
			b.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
		}
	case float32:
		writeCanonical(b, float64(v))
	case int:
		b.WriteString(strconv.Itoa(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(v, 10))
	case []any:
		b.WriteByte('[')
		for index, item := range v {
			if index > 0 {
				b.WriteByte(',')
			}
			writeCanonical(b, item)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				b.WriteByte(',')
			}
			writeCanonicalString(b, key)
			b.WriteByte(':')
			writeCanonical(b, v[key])
		}
		b.WriteByte('}')
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			b.WriteString("null")
			return
		}
		var decoded any
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		if decoder.Decode(&decoded) != nil {
			b.WriteString("null")
			return
		}
		writeCanonical(b, decoded)
	}
}

// writeCanonicalString matches Python's default ensure_ascii=True encoding.
func writeCanonicalString(b *strings.Builder, value string) {
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r > 0x7e {
				if r <= 0xffff {
					fmt.Fprintf(b, `\u%04x`, r)
				} else {
					r -= 0x10000
					fmt.Fprintf(b, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

func evidenceObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func evidenceArray(value any) []any {
	array, _ := value.([]any)
	return array
}

func evidenceString(value any) string {
	text, _ := value.(string)
	return text
}

func evidenceNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		number, err := v.Float64()
		return number, err == nil
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

func evidenceInt(value any) (int64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	text := number.String()
	if strings.ContainsAny(text, "eE") {
		parsed, err := number.Float64()
		if err != nil || parsed != math.Trunc(parsed) || math.Abs(parsed) > 1<<53 {
			return 0, false
		}
		return int64(parsed), true
	}
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		if strings.Trim(text[dot+1:], "0") != "" {
			return 0, false
		}
		text = text[:dot]
	}
	integer, err := strconv.ParseInt(text, 10, 64)
	return integer, err == nil
}

func truthyDefault(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return truthy(value)
}

// truthy mirrors Python's boolean test for the JSON values used by evidence
// documents: null, false, zero, an empty string, array or object are false.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		number, err := v.Float64()
		return err != nil || number != 0
	case float64:
		return v != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return true
}

// insideGeometry tests a point against GeoJSON Polygon or MultiPolygon
// coordinates. Rings after the first are holes.
func insideGeometry(geometry map[string]any, x, y float64) bool {
	if geometry == nil {
		return false
	}
	switch evidenceString(geometry["type"]) {
	case "Polygon":
		return insidePolygon(evidenceArray(geometry["coordinates"]), x, y)
	case "MultiPolygon":
		for _, part := range evidenceArray(geometry["coordinates"]) {
			if insidePolygon(evidenceArray(part), x, y) {
				return true
			}
		}
	}
	return false
}

func insidePolygon(rings []any, x, y float64) bool {
	if len(rings) == 0 || !insideRing(evidenceArray(rings[0]), x, y) {
		return false
	}
	for _, hole := range rings[1:] {
		if insideRing(evidenceArray(hole), x, y) {
			return false
		}
	}
	return true
}

func insideRing(ring []any, x, y float64) bool {
	hit := false
	count := len(ring)
	for index := 0; index < count; index++ {
		first := evidenceArray(ring[index])
		second := evidenceArray(ring[(index+1)%count])
		if len(first) < 2 || len(second) < 2 {
			continue
		}
		x1, ok1 := evidenceNumber(first[0])
		y1, ok2 := evidenceNumber(first[1])
		x2, ok3 := evidenceNumber(second[0])
		y2, ok4 := evidenceNumber(second[1])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			continue
		}
		if (y1 > y) != (y2 > y) {
			intersection := x1 + (y-y1)/(y2-y1)*(x2-x1)
			if x < intersection {
				hit = !hit
			}
		}
	}
	return hit
}

func geometryPoints(value any) [][2]float64 {
	points := [][2]float64{}
	values := evidenceArray(value)
	if len(values) > 0 {
		if x, ok := evidenceNumber(values[0]); ok && len(values) > 1 {
			if y, ok := evidenceNumber(values[1]); ok {
				return append(points, [2]float64{x, y})
			}
		}
	}
	for _, part := range values {
		points = append(points, geometryPoints(part)...)
	}
	return points
}

func geometryBounds(geometry map[string]any) ([4]float64, error) {
	if geometry == nil {
		return [4]float64{}, fmt.Errorf("a requirement area is required for this predicate")
	}
	points := geometryPoints(geometry["coordinates"])
	if len(points) == 0 {
		return [4]float64{}, fmt.Errorf("requirement area has no coordinates")
	}
	bounds := [4]float64{points[0][0], points[0][1], points[0][0], points[0][1]}
	for _, point := range points[1:] {
		bounds[0] = math.Min(bounds[0], point[0])
		bounds[1] = math.Min(bounds[1], point[1])
		bounds[2] = math.Max(bounds[2], point[0])
		bounds[3] = math.Max(bounds[3], point[1])
	}
	return bounds, nil
}

func lattice(geometry map[string]any, resolution int) ([][2]float64, error) {
	bounds, err := geometryBounds(geometry)
	if err != nil {
		return nil, err
	}
	west, south, east, north := bounds[0], bounds[1], bounds[2], bounds[3]
	samples := make([][2]float64, 0, resolution*resolution)
	for column := 0; column < resolution; column++ {
		x := west + (float64(column)+.5)*(east-west)/float64(resolution)
		for row := 0; row < resolution; row++ {
			y := south + (float64(row)+.5)*(north-south)/float64(resolution)
			samples = append(samples, [2]float64{x, y})
		}
	}
	return samples, nil
}

// geometryCoverage returns how many requirement lattice samples fall inside the
// release area.
func geometryCoverage(requirementArea, releaseArea map[string]any, resolution int) (float64, int, int, error) {
	points, err := lattice(requirementArea, resolution)
	if err != nil {
		return 0, 0, 0, err
	}
	required := make([][2]float64, 0, len(points))
	for _, point := range points {
		if insideGeometry(requirementArea, point[0], point[1]) {
			required = append(required, point)
		}
	}
	if len(required) == 0 {
		return 0, 0, 0, nil
	}
	covered := 0
	for _, point := range required {
		if insideGeometry(releaseArea, point[0], point[1]) {
			covered++
		}
	}
	return float64(covered) / float64(len(required)), covered, len(required), nil
}

func decision(requirement, kind, result, reason, detail, evidence string) SuitabilityDecision {
	return SuitabilityDecision{
		"requirement": requirement, "kind": kind, "result": result,
		"reason": reason, "detail": detail, "evidenceSha256": evidence,
	}
}

func memberFraction(member map[string]any) (float64, bool) {
	details := evidenceObject(member["details"])
	selected, selectedOK := evidenceInt(details["selectedSamples"])
	valid, validOK := evidenceInt(details["validSamples"])
	if selectedOK && validOK && selected > 0 && valid >= 0 && valid <= selected {
		return float64(valid) / float64(selected), true
	}
	return 0, false
}

func parseInstant(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// EvaluateSuitability returns a decision for every requirement in the
// document. now may be nil when no expiry assertion has to be resolved.
func EvaluateSuitability(requirement, evidence map[string]any, now *time.Time) (SuitabilityResult, error) {
	result := SuitabilityResult{
		Schema: suitabilityResultSchema, RequirementID: requirement["requirementId"],
		Operation: requirement["operation"], Decisions: []SuitabilityDecision{}, Optional: []SuitabilityDecision{},
	}
	if evidenceString(requirement["schema"]) != suitabilitySchema {
		return result, fmt.Errorf("unsupported requirement schema")
	}
	if evidenceString(evidence["schema"]) != suitabilityEvidenceSchema {
		return result, fmt.Errorf("unsupported evidence schema")
	}
	releaseSHA := SuitabilityEvidenceDigest(evidence)
	result.EvidenceSHA256 = releaseSHA
	result.Terms = evidence["terms"]
	result.Release = map[string]any{"manifestSha256": evidence["manifestSha256"],
		"areaRevisionId": evidence["areaRevisionId"], "profileId": evidence["profileId"]}
	overall := DecisionPass
	add := func(item SuitabilityDecision) {
		result.Decisions = append(result.Decisions, item)
		switch item["result"] {
		case DecisionUnsupported:
			overall = DecisionUnsupported
		case DecisionUnqualified:
			if overall != DecisionUnsupported {
				overall = DecisionUnqualified
			}
		}
	}

	named := evidenceObject(requirement["release"])
	identityOK := evidenceString(named["manifestSha256"]) == evidenceString(evidence["manifestSha256"]) &&
		evidenceString(named["areaRevisionId"]) == evidenceString(evidence["areaRevisionId"])
	if identityOK {
		add(decision("release", "release", DecisionPass, "release-match",
			"The evidence names the required release and area revision.", releaseSHA))
	} else {
		add(decision("release", "release", DecisionUnsupported, "release-mismatch",
			"The requirement and evidence name different releases.", releaseSHA))
	}

	limits := evidenceObject(requirement["resources"])
	members := evidenceArray(evidence["members"])
	var total int64
	for _, item := range members {
		if bytes, ok := evidenceInt(evidenceObject(item)["bytes"]); ok {
			total += bytes
		}
	}
	maxMembers := int64(1024)
	if value, ok := evidenceInt(limits["maxMembers"]); ok {
		maxMembers = value
	}
	maxBytes := int64(4 << 30)
	if value, ok := evidenceInt(limits["maxBytes"]); ok {
		maxBytes = value
	}
	if int64(len(members)) > maxMembers || total > maxBytes {
		item := decision("resources", "resources", DecisionUnsupported, "resource-exceeded",
			"The release exceeds the declared member or byte limit.", releaseSHA)
		item["observed"] = map[string]any{"members": len(members), "bytes": total}
		add(item)
	} else {
		item := decision("resources", "resources", DecisionPass, "resource-accepted",
			"The release is within the declared resource limits.", releaseSHA)
		item["observed"] = map[string]any{"members": len(members), "bytes": total}
		add(item)
	}

	terms := evidenceObject(evidence["terms"])
	termRequirement := evidenceObject(requirement["terms"])
	if !truthy(evidence["terms"]) {
		if truthy(termRequirement["requireIssued"]) {
			add(decision("terms", "terms", DecisionUnsupported, "terms-missing",
				"No issued terms accompany this evidence.", releaseSHA))
		} else {
			add(decision("terms", "terms", DecisionPass, "terms-accepted",
				"No issued terms were required.", releaseSHA))
		}
	} else {
		var grant map[string]any
		for _, candidate := range evidenceArray(terms["grants"]) {
			item := evidenceObject(candidate)
			if evidenceString(item["scope"]) == evidenceString(termRequirement["operation"]) {
				grant = item
				break
			}
		}
		expires := evidenceString(terms["expiresAt"])
		operation := evidenceString(termRequirement["operation"])
		if expires != "" && now == nil {
			item := decision("terms", "terms", DecisionUnqualified, "terms-expiry-unknown",
				"The issued terms state an expiry but the evaluation instant is missing.", releaseSHA)
			item["termsExpiresAt"] = expires
			add(item)
		} else if expires != "" {
			expiry, expiryOK := parseInstant(expires)
			if !expiryOK {
				item := decision("terms", "terms", DecisionUnqualified, "terms-expiry-unknown",
					"The issued expiry is not a recognized instant.", releaseSHA)
				item["termsExpiresAt"] = expires
				add(item)
			} else if !expiry.After(*now) {
				item := decision("terms", "terms", DecisionUnsupported, "terms-expired",
					"The issued terms expired before this evaluation.", releaseSHA)
				item["termsExpiresAt"] = expires
				add(item)
			} else if operation != "" && grant == nil {
				item := decision("terms", "terms", DecisionUnsupported, "grant-missing",
					"The issued terms do not grant the requested operation.", releaseSHA)
				item["operation"] = operation
				add(item)
			} else if operation != "" && truthy(grant["revoked"]) {
				item := decision("terms", "terms", DecisionUnsupported, "grant-withdrawn",
					"The issued grant for this operation was withdrawn.", releaseSHA)
				item["grantId"] = grant["id"]
				add(item)
			} else {
				item := decision("terms", "terms", DecisionPass, "terms-accepted",
					"The issued terms cover the requested operation.", releaseSHA)
				item["termsExpiresAt"] = terms["expiresAt"]
				item["issuedAt"] = terms["issuedAt"]
				add(item)
			}
		} else if operation != "" && grant == nil {
			item := decision("terms", "terms", DecisionUnsupported, "grant-missing",
				"The issued terms do not grant the requested operation.", releaseSHA)
			item["operation"] = operation
			add(item)
		} else if operation != "" && truthy(grant["revoked"]) {
			item := decision("terms", "terms", DecisionUnsupported, "grant-withdrawn",
				"The issued grant for this operation was withdrawn.", releaseSHA)
			item["grantId"] = grant["id"]
			add(item)
		} else {
			item := decision("terms", "terms", DecisionPass, "terms-accepted",
				"The issued terms cover the requested operation.", releaseSHA)
			item["termsExpiresAt"] = terms["expiresAt"]
			item["issuedAt"] = terms["issuedAt"]
			add(item)
		}
	}

	areaRequirement := evidenceObject(requirement["area"])
	var coverageFraction *float64
	if truthy(requirement["area"]) {
		fraction, covered, totalPoints, err := geometryCoverage(evidenceObject(areaRequirement["geometry"]), evidenceObject(evidence["area"]), suitabilityLattice)
		if err != nil {
			return result, err
		}
		coverageFraction = &fraction
		item := SuitabilityDecision{}
		switch {
		case totalPoints == 0:
			item = decision("area", "area", DecisionUnsupported, "area-invalid",
				"The requirement area has no inside lattice samples.", releaseSHA)
		case covered == 0:
			item = decision("area", "area", DecisionUnsupported, "area-outside-release",
				"The requirement area lies outside the release area.", releaseSHA)
			item["coverage"], item["samples"] = 0.0, totalPoints
		case fraction < 1:
			accepted := truthy(areaRequirement["allowPartial"])
			reason := "area-partially-covered"
			itemResult := DecisionUnsupported
			if accepted {
				reason, itemResult = "area-partially-covered-accepted", DecisionPass
			}
			item = decision("area", "area", itemResult, reason,
				"Only part of the requirement area is inside the release area.", releaseSHA)
			item["coverage"], item["samples"] = fraction, totalPoints
		default:
			item = decision("area", "area", DecisionPass, "area-covered",
				"The requirement area lies within the release area.", releaseSHA)
			item["coverage"], item["samples"] = 1.0, totalPoints
		}
		add(item)
	}
	result.Coverage = coverageFraction

	window := evidenceObject(requirement["time"])
	if window != nil && evidenceString(window["mode"]) != "" {
		mode := evidenceString(window["mode"])
		rawLayers := evidenceArray(window["layers"])
		layers := make([]string, 0, len(rawLayers))
		seenLayers := map[string]bool{}
		for _, candidate := range rawLayers {
			name := evidenceString(candidate)
			if !seenLayers[name] {
				seenLayers[name] = true
				layers = append(layers, name)
			}
		}
		layerInputs := make([][]map[string]any, len(layers))
		for index, name := range layers {
			for _, candidate := range evidenceArray(evidence["inputs"]) {
				input := evidenceObject(candidate)
				if evidenceString(input["layer"]) == name {
					layerInputs[index] = append(layerInputs[index], input)
				}
			}
		}
		switch mode {
		case "background":
			add(decision("time", "time", DecisionPass, "time-background-accepted",
				"Dated background use accepts the declared source times.", releaseSHA))
		case "common-window":
			start, end := evidenceString(window["from"]), evidenceString(window["to"])
			if start == "" || end == "" {
				add(decision("time", "time", DecisionUnqualified, "time-unknown",
					"A common window requires explicit from and to instants.", releaseSHA))
				break
			}
			startAt, startOK := parseInstant(start)
			endAt, endOK := parseInstant(end)
			if !startOK || !endOK {
				add(decision("time", "time", DecisionUnqualified, "time-unknown",
					"A common window bound is not a recognized instant.", releaseSHA))
				break
			}
			outside := []any{}
			unknown := []string{}
			for index, name := range layers {
				for _, input := range layerInputs[index] {
					value := evidenceString(input["dataTime"])
					if value == "" {
						unknown = append(unknown, name)
						continue
					}
					instant, ok := parseInstant(value)
					if !ok {
						unknown = append(unknown, name)
						continue
					}
					if instant.Before(startAt) || instant.After(endAt) {
						outside = append(outside, map[string]any{"layer": name, "dataTime": value})
					}
				}
			}
			if len(unknown) > 0 {
				sort.Strings(unknown)
				unique := []string{}
				for index, name := range unknown {
					if index == 0 || name != unknown[index-1] {
						unique = append(unique, name)
					}
				}
				item := decision("time", "time", DecisionUnqualified, "time-unknown",
					"A required source has no declared time.", releaseSHA)
				item["unknownSources"] = unique
				add(item)
			} else if len(outside) > 0 {
				item := decision("time", "time", DecisionUnsupported, "time-incompatible",
					"A source time falls outside the required window.", releaseSHA)
				item["outside"] = outside
				add(item)
			} else {
				add(decision("time", "time", DecisionPass, "time-window-accepted",
					"Every required source falls inside the common window.", releaseSHA))
			}
		default:
			add(decision("time", "time", DecisionUnqualified, "time-unknown",
				"Unsupported time mode.", releaseSHA))
		}
	}

	current := []map[string]any{}
	for _, candidate := range evidenceArray(evidence["assessments"]) {
		item := evidenceObject(candidate)
		if evidenceString(item["supersededBy"]) == "" {
			current = append(current, item)
		}
	}
	visual := []any{}
	if availability := evidenceObject(evidence["representationAvailability"]); availability != nil {
		visual = evidenceArray(availability["visual-derivative"])
	}

	for index, candidate := range evidenceArray(requirement["representations"]) {
		representation := evidenceObject(candidate)
		layer := evidenceString(representation["layer"])
		name := evidenceString(representation["id"])
		if name == "" {
			name = layer
		}
		if name == "" {
			name = fmt.Sprintf("representation-%d", index)
		}
		required := truthyDefault(representation["required"], true)
		layerMembers := []map[string]any{}
		for _, member := range members {
			item := evidenceObject(member)
			if evidenceString(item["layer"]) == layer {
				layerMembers = append(layerMembers, item)
			}
		}
		if len(layerMembers) == 0 {
			visualOnly := false
			for _, item := range visual {
				if evidenceString(item) == layer {
					visualOnly = true
				}
			}
			allowVisual := true
			if value, ok := representation["allowVisualDerivative"]; ok {
				allowVisual = value != false
			}
			if visualOnly && !allowVisual {
				item := decision(name, "representation", DecisionUnsupported, "visual-only-derivative",
					"Only a visual derivative of this layer exists; native values are unavailable.", releaseSHA)
				item["layer"], item["purpose"] = layer, "visual-reference"
				add(item)
			} else if required {
				item := decision(name, "representation", DecisionUnsupported, "representation-absent",
					fmt.Sprintf("No native %s member is retained.", layer), releaseSHA)
				item["layer"] = layer
				add(item)
			} else {
				item := decision(name, "representation", DecisionOmitted, "optional-representation-absent",
					fmt.Sprintf("Optional layer %s is absent.", layer), releaseSHA)
				item["layer"] = layer
				add(item)
			}
			continue
		}

		datum := evidenceString(representation["verticalDatum"])
		datums := []string{}
		seen := map[string]bool{}
		for _, member := range layerMembers {
			value := evidenceString(evidenceObject(member["details"])["verticalDatum"])
			if value == "" {
				value = "unknown"
			}
			if !seen[value] {
				seen[value] = true
				datums = append(datums, value)
			}
		}
		sort.Strings(datums)
		if datum != "" && !seen[datum] {
			item := decision(name, "representation", DecisionUnsupported, "datum-mismatch",
				fmt.Sprintf("Retained %s datums %v do not include %s.", layer, datums, datum), releaseSHA)
			item["layer"], item["datums"] = layer, datums
			add(item)
			continue
		}

		unit := evidenceString(representation["units"])
		if unit != "" && unit != "metre" && unit != "meter" && unit != "m" {
			item := decision(name, "representation", DecisionUnsupported, "unit-unsupported",
				fmt.Sprintf("Unit %s is not native or exactly convertible.", unit), releaseSHA)
			item["layer"], item["unit"] = layer, unit
			add(item)
			continue
		}

		fractions := []float64{}
		for _, member := range layerMembers {
			if value, ok := memberFraction(member); ok {
				fractions = append(fractions, value)
			}
		}
		minimum, hasMinimum := evidenceNumber(representation["minValidFraction"])
		if hasMinimum && len(fractions) > 0 {
			best := fractions[0]
			for _, value := range fractions[1:] {
				if value > best {
					best = value
				}
			}
			if best < minimum {
				item := decision(name, "representation", DecisionUnsupported, "mask-evidence-below-requirement",
					"The best retained member's valid fraction is below the requirement.", releaseSHA)
				item["layer"], item["validFraction"], item["required"] = layer, best, representation["minValidFraction"]
				add(item)
				continue
			}
		}
		if hasMinimum && len(fractions) == 0 {
			item := decision(name, "representation", DecisionUnqualified, "mask-evidence-below-requirement",
				"No member records valid-sample evidence for this layer.", releaseSHA)
			item["layer"] = layer
			add(item)
			continue
		}

		accuracy := evidenceObject(representation["accuracy"])
		mode := "unknown-acceptable"
		if accuracy != nil && evidenceString(accuracy["mode"]) != "" {
			mode = evidenceString(accuracy["mode"])
		}
		if mode == "unknown-acceptable" {
			item := decision(name, "representation", DecisionPass, "accuracy-unknown-declared",
				"The consumer accepts native inspection with unknown physical accuracy.", releaseSHA)
			item["layer"], item["accuracy"], item["assessment"] = layer, "UNKNOWN", nil
			add(item)
			continue
		}

		use := "unspecified"
		if accuracy != nil && evidenceString(accuracy["use"]) != "" {
			use = evidenceString(accuracy["use"])
		}
		tolerance := 0.0
		if accuracy != nil {
			if value, ok := evidenceNumber(accuracy["toleranceM"]); ok {
				tolerance = value
			}
		}
		memberDigests := map[string]bool{}
		for _, member := range layerMembers {
			memberDigests[evidenceString(member["sha256"])] = true
		}
		candidates := []map[string]any{}
		for _, item := range current {
			if evidenceString(item["layer"]) == layer && evidenceString(item["use"]) == use &&
				evidenceString(item["decision"]) == "ACCEPTED" {
				candidates = append(candidates, item)
			}
		}
		if len(candidates) == 0 {
			item := decision(name, "representation", DecisionUnqualified, "accuracy-unevidenced",
				"Independent accuracy is required but no accepted assessment exists.", releaseSHA)
			item["layer"], item["use"] = layer, use
			add(item)
			continue
		}
		matching := []map[string]any{}
		for _, item := range candidates {
			if memberDigests[evidenceString(item["memberSha256"])] {
				matching = append(matching, item)
			}
		}
		if len(matching) == 0 {
			item := decision(name, "representation", DecisionUnqualified, "assessment-member-mismatch",
				"Every accepted assessment names a different member digest.", releaseSHA)
			item["layer"], item["assessments"] = layer, assessmentIDs(candidates)
			add(item)
			continue
		}
		covering := []map[string]any{}
		if truthy(requirement["area"]) {
			for _, item := range matching {
				fraction, covered, totalPoints, err := geometryCoverage(evidenceObject(areaRequirement["geometry"]), evidenceObject(item["area"]), suitabilityLattice)
				_ = fraction
				if err != nil {
					return result, err
				}
				if totalPoints > 0 && covered == totalPoints {
					covering = append(covering, item)
				}
			}
		} else {
			covering = matching
		}
		if len(covering) == 0 {
			item := decision(name, "representation", DecisionUnqualified, "assessment-area-insufficient",
				"No accepted assessment covers the requirement area.", releaseSHA)
			item["layer"], item["assessments"] = layer, assessmentIDs(matching)
			add(item)
			continue
		}
		failing := []map[string]any{}
		for _, item := range covering {
			statistic := math.Inf(1)
			if value, ok := evidenceNumber(item["statisticM"]); ok {
				statistic = value
			}
			if statistic > tolerance {
				failing = append(failing, item)
			}
		}
		if len(failing) > 0 {
			item := failing[0]
			failed := decision(name, "representation", DecisionUnsupported, "accuracy-exceeded",
				"The accepted assessment statistic exceeds the declared tolerance.", releaseSHA)
			failed["layer"], failed["statisticM"], failed["toleranceM"] = layer, item["statisticM"], tolerance
			failed["assessment"], failed["assessmentSha256"] = item["id"], item["evidenceSha256"]
			add(failed)
			continue
		}
		sort.SliceStable(covering, func(i, j int) bool {
			left, _ := evidenceInt(covering[i]["sequence"])
			right, _ := evidenceInt(covering[j]["sequence"])
			if left != right {
				return left > right
			}
			return evidenceString(covering[i]["id"]) < evidenceString(covering[j]["id"])
		})
		item := covering[0]
		evidenced := decision(name, "representation", DecisionPass, "accuracy-evidenced",
			"An accepted assessment covers the requirement area and meets the tolerance.", releaseSHA)
		evidenced["layer"], evidenced["statisticM"], evidenced["toleranceM"] = layer, item["statisticM"], tolerance
		evidenced["uncertaintyM"], evidenced["assessment"], evidenced["assessmentSha256"] = item["uncertaintyM"], item["id"], item["evidenceSha256"]
		evidenced["controlSetSha256"], evidenced["method"] = item["controlSetSha256"], item["method"]
		add(evidenced)
	}

	switch overall {
	case DecisionPass:
		result.Overall = "supported"
	case DecisionUnsupported:
		result.Overall = "unsupported"
	default:
		result.Overall = "unqualified"
	}
	for _, item := range result.Decisions {
		if item["result"] == DecisionOmitted {
			result.Optional = append(result.Optional, item)
		}
	}
	return result, nil
}

func assessmentIDs(items []map[string]any) []any {
	ids := make([]any, 0, len(items))
	for _, item := range items {
		ids = append(ids, item["id"])
	}
	return ids
}

// DecodeEvidenceDocument decodes JSON while retaining numeric spelling so
// digest bytes match the offline reader.
func DecodeEvidenceDocument(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("ambiguous JSON value")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON document must be an object")
	}
	return object, nil
}
