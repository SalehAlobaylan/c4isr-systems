"""Build the shared P01 requirement/evidence/answer vectors.

The vectors are plain JSON so the offline Python reader, the Go API and a
reviewer can evaluate exactly the same inputs. Expected decisions are written
here from the requirement rules, not produced by the implementation under test.

Run from the repository root:

    python scripts/data/build_suitability_cases.py
"""
import argparse
import json
from pathlib import Path

OUTPUT = Path(__file__).resolve().parents[2] / "testdata" / "geospatial" / "suitability-cases"
SCHEMA = "emad.environment-suitability-cases.v1"
MANIFEST = "a" * 64
AREA_REVISION = "owned-fixture-area-v1"
SURFACE = "1" * 64
GROUND = "2" * 64
WEATHER = "3" * 64

AREA = {
    "type": "MultiPolygon",
    "coordinates": [
        [[[46.700, 24.700], [46.760, 24.700], [46.760, 24.760], [46.700, 24.760], [46.700, 24.700]],
         [[46.720, 24.720], [46.720, 24.730], [46.730, 24.730], [46.730, 24.720], [46.720, 24.720]]],
        [[[46.780, 24.700], [46.800, 24.700], [46.800, 24.720], [46.780, 24.720], [46.780, 24.700]]],
    ],
}
INSIDE = {"type": "Polygon",
          "coordinates": [[[46.700, 24.740], [46.730, 24.740], [46.730, 24.760], [46.700, 24.760],
                           [46.700, 24.740]]]}
PARTIAL = {"type": "Polygon",
           "coordinates": [[[46.740, 24.700], [46.770, 24.700], [46.770, 24.760], [46.740, 24.760],
                            [46.740, 24.700]]]}
COVERING = {"type": "Polygon",
            "coordinates": [[[46.690, 24.690], [46.770, 24.690], [46.770, 24.770], [46.690, 24.770],
                             [46.690, 24.690]]]}
SMALL = {"type": "Polygon",
         "coordinates": [[[46.700, 24.740], [46.715, 24.740], [46.715, 24.760], [46.700, 24.760],
                          [46.700, 24.740]]]}


def member(name, layer, digest, size, details):
    return {"name": name, "layer": layer, "component": 0, "sha256": digest, "bytes": size,
            "details": details}


SURFACE_MEMBER = member("raster-00-00.tif", "surface", SURFACE, 2093, {
    "validSamples": 1699, "selectedSamples": 1700, "verticalDatum": "ellipsoid",
    "maskMeaning": "Valid in every input band and within the requested footprint"})
IMAGERY_MEMBER = member("imagery-00-00.tif", "imagery", "6" * 64, 4096, {
    "validSamples": 95, "selectedSamples": 100, "verticalDatum": "not-applicable",
    "maskMeaning": "Valid optical samples inside the requested footprint"})
GROUND_MEMBER = member("raster-10-00.tif", "ground", GROUND, 5000, {
    "validSamples": 900, "selectedSamples": 1000, "verticalDatum": "EGM2008",
    "maskMeaning": "Valid bare-earth samples"})
WEATHER_MEMBER = member("weather-field.json", "weather-field", WEATHER, 4096, {
    "verticalDatum": "not-applicable", "dataTime": "2023-01-09T12:00:00Z"})
SURFACE_INPUT = {"layer": "surface", "verticalDatum": "ellipsoid", "dataTime": "2022-06-01",
                 "productEdition": "owned-surface-v1", "crs": "EPSG:4326"}
GROUND_INPUT = {"layer": "ground", "verticalDatum": "EGM2008", "dataTime": "2015",
                "productEdition": "owned-ground-v1", "crs": "EPSG:4326"}
WEATHER_INPUT = {"layer": "weather-field", "verticalDatum": "not-applicable",
                 "dataTime": "2023-01-09T12:00:00Z", "productEdition": "WeatherBench 2",
                 "crs": "EPSG:4326"}


def evidence(members=None, inputs=None, assessments=None, terms=None, visual=None):
    return {"schema": "emad.environment-evidence.v1", "manifestSha256": MANIFEST,
            "profileId": "reference-surface-v1", "areaRevisionId": AREA_REVISION, "area": AREA,
            "members": members if members is not None else [SURFACE_MEMBER],
            "inputs": inputs if inputs is not None else [SURFACE_INPUT],
            "assessments": assessments or [], "terms": terms,
            "representationAvailability": visual or {}}


def requirement(needs, *, area=None, allow_partial=False, time=None, terms_operation=None,
                require_terms=False, manifest=MANIFEST, resources=None):
    document = {"schema": "emad.environment-requirement.v1", "requirementId": "case",
                "operation": "read-native", "release": {"manifestSha256": manifest,
                                                         "areaRevisionId": AREA_REVISION},
                "resources": resources or {"maxMembers": 64, "maxBytes": 1 << 30},
                "representations": needs}
    if area:
        document["area"] = {"geometry": area, "allowPartial": allow_partial}
    if time:
        document["time"] = time
    if terms_operation or require_terms:
        document["terms"] = {"requireIssued": require_terms, "operation": terms_operation}
    return document


def rep(layer, **extra):
    return {"id": layer, "layer": layer, "required": True, "units": "metre", **extra}


def expected(result, decisions, *, coverage=None, optional_reasons=()):
    return {"overall": result, "coverage": coverage, "decisions": decisions,
            "optionalReasons": list(optional_reasons)}


def decision(name, result, reason, **extra):
    return {"requirement": name, "result": result, "reason": reason, **extra}


CASES = []


def case(identifier, requirement_value, evidence_value, expected_value, derivation, now=None):
    CASES.append({"id": identifier, "derivation": derivation, "requirement": requirement_value,
                  "evidence": evidence_value, "now": now, "expected": expected_value})


case("surface-native-inspection", requirement(
        [rep("surface", accuracy={"mode": "unknown-acceptable"}),
         {"id": "weather", "layer": "weather-field", "required": False}],
        area=INSIDE, time={"mode": "background", "layers": ["surface"]}),
     evidence(),
     expected("supported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("area", "pass", "area-covered", coverage=1.0),
         decision("time", "pass", "time-background-accepted"),
         decision("surface", "pass", "accuracy-unknown-declared"),
         decision("weather", "optional-omitted", "optional-representation-absent"),
     ], coverage=1.0, optional_reasons=["optional-representation-absent"]),
     "Native surface inspection accepts UNKNOWN physical accuracy; weather is an optional omission.")

case("imagery-native-optional-weather", requirement(
        [rep("imagery", accuracy={"mode": "unknown-acceptable"}),
         {"id": "weather", "layer": "weather-field", "required": False}]),
     evidence(members=[IMAGERY_MEMBER],
              inputs=[{"layer": "imagery", "verticalDatum": "not-applicable",
                       "dataTime": "2022-06-01T10:00:00Z", "productEdition": "owned-imagery-v1",
                       "crs": "EPSG:4326"}]),
     expected("supported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("imagery", "pass", "accuracy-unknown-declared"),
         decision("weather", "optional-omitted", "optional-representation-absent"),
     ], optional_reasons=["optional-representation-absent"]),
     "Acceptable native imagery with a missing optional weather layer remains supported.")

case("ground-required-absent", requirement([rep("ground")]),
     evidence(),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("ground", "unsupported", "representation-absent"),
     ]),
     "A required modeled ground layer is not retained by this release.")

ACCURACY = {"mode": "independent-assessment", "use": "independent-ground-elevation", "toleranceM": .25}
ASSESSMENT = {
    "id": "assessment-ground-2026-01", "releaseManifestSha256": MANIFEST,
    "memberSha256": GROUND, "layer": "ground", "use": "independent-ground-elevation",
    "decision": "ACCEPTED", "method": "GNSS campaign, held-out controls",
    "controlSetSha256": "4" * 64, "evidenceSha256": "5" * 64, "statisticM": .12,
    "uncertaintyM": .05, "area": COVERING, "sequence": 1,
    "reviewedAt": "2026-01-15T00:00:00Z"}


def assessment(**changes):
    value = dict(ASSESSMENT)
    value.update(changes)
    return value


case("ground-unknown-accuracy", requirement([rep("ground", accuracy=ACCURACY)]),
     evidence(members=[GROUND_MEMBER], inputs=[GROUND_INPUT]),
     expected("unqualified", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("ground", "unqualified", "accuracy-unevidenced"),
     ]),
     "Independent ground accuracy is required but no accepted assessment exists.")

case("ground-accuracy-evidenced", requirement(
        [rep("ground", accuracy=ACCURACY)], area=INSIDE),
     evidence(members=[GROUND_MEMBER], inputs=[GROUND_INPUT], assessments=[assessment()]),
     expected("supported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("area", "pass", "area-covered", coverage=1.0),
         decision("ground", "pass", "accuracy-evidenced",
                  assessment="assessment-ground-2026-01", statisticM=.12),
     ], coverage=1.0),
     "An accepted assessment covers the requirement area and is inside the declared tolerance.")

case("ground-accuracy-exceeded", requirement(
        [rep("ground", accuracy=ACCURACY)], area=INSIDE),
     evidence(members=[GROUND_MEMBER], inputs=[GROUND_INPUT],
              assessments=[assessment(statisticM=.4)]),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("area", "pass", "area-covered", coverage=1.0),
         decision("ground", "unsupported", "accuracy-exceeded",
                  assessment="assessment-ground-2026-01", statisticM=.4, toleranceM=.25),
     ], coverage=1.0),
     "A proven statistic above the declared tolerance is a violation, not missing evidence.")

case("assessment-superseded", requirement(
        [rep("ground", accuracy=ACCURACY)], area=INSIDE),
     evidence(members=[GROUND_MEMBER], inputs=[GROUND_INPUT], assessments=[
         assessment(supersededBy="assessment-ground-2026-02"),
         assessment(id="assessment-ground-2026-02", statisticM=.5, sequence=2,
                    reviewedAt="2026-02-15T00:00:00Z")]),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("area", "pass", "area-covered", coverage=1.0),
         decision("ground", "unsupported", "accuracy-exceeded",
                  assessment="assessment-ground-2026-02", statisticM=.5, toleranceM=.25),
     ], coverage=1.0),
     "A correction supersedes the earlier accepted assessment; the new evidence governs.")

case("assessment-area-insufficient", requirement(
        [rep("ground", accuracy=ACCURACY)], area=INSIDE),
     evidence(members=[GROUND_MEMBER], inputs=[GROUND_INPUT],
              assessments=[assessment(area=SMALL)]),
     expected("unqualified", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("area", "pass", "area-covered", coverage=1.0),
         decision("ground", "unqualified", "assessment-area-insufficient"),
     ], coverage=1.0),
     "A subarea assessment cannot qualify a larger requirement area.")

case("assessment-member-mismatch", requirement(
        [rep("ground", accuracy=ACCURACY)]),
     evidence(members=[GROUND_MEMBER], inputs=[GROUND_INPUT],
              assessments=[assessment(memberSha256="9" * 64)]),
     expected("unqualified", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("ground", "unqualified", "assessment-member-mismatch"),
     ]),
     "An assessment for a different member digest cannot qualify a replacement member.")

case("mixed-year-background", requirement(
        [rep("surface")], time={"mode": "background", "layers": ["surface", "weather-field"]}),
     evidence(members=[SURFACE_MEMBER, WEATHER_MEMBER], inputs=[SURFACE_INPUT, WEATHER_INPUT]),
     expected("supported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("time", "pass", "time-background-accepted"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "A declared background-use profile accepts mixed source years.")

case("mixed-year-strict", requirement(
        [rep("surface")], time={"mode": "common-window", "layers": ["surface", "weather-field"],
                                "from": "2022-01-01T00:00:00Z", "to": "2022-12-31T23:59:59Z"}),
     evidence(members=[SURFACE_MEMBER, WEATHER_MEMBER], inputs=[SURFACE_INPUT, WEATHER_INPUT]),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("time", "unsupported", "time-incompatible"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "A strict common window rejects a required source outside the window.")

case("time-unknown", requirement(
        [rep("surface")], time={"mode": "common-window", "layers": ["surface"],
                                "from": "2022-01-01T00:00:00Z", "to": "2022-12-31T23:59:59Z"}),
     evidence(members=[SURFACE_MEMBER], inputs=[{k: v for k, v in SURFACE_INPUT.items() if k != "dataTime"}]),
     expected("unqualified", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("time", "unqualified", "time-unknown"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "A required source without a declared time leaves the strict window unqualified.")

PARTIAL_COVERAGE = 2 / 3

case("partial-area-rejected", requirement([rep("surface")], area=PARTIAL),
     evidence(),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("area", "unsupported", "area-partially-covered", coverage=PARTIAL_COVERAGE),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ], coverage=PARTIAL_COVERAGE),
     "Part of the requirement lies outside the released area; partial use was not accepted.")

case("partial-area-accepted", requirement([rep("surface")], area=PARTIAL, allow_partial=True),
     evidence(),
     expected("supported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("area", "pass", "area-partially-covered-accepted", coverage=PARTIAL_COVERAGE),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ], coverage=PARTIAL_COVERAGE),
     "The consumer explicitly accepts the measured partial area coverage.")

case("visual-only-native-request", requirement(
        [rep("surface", allowVisualDerivative=False)]),
     evidence(members=[], inputs=[], visual={"visual-derivative": ["surface"]}),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("surface", "unsupported", "visual-only-derivative"),
     ]),
     "A native request rejects a layer for which only a visual derivative exists.")

case("release-mismatch", requirement([rep("surface")], manifest="b" * 64),
     evidence(),
     expected("unsupported", [
         decision("release", "unsupported", "release-mismatch"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "A different release manifest digest cannot satisfy the requirement.")

TERMS = {"issuedAt": "2020-01-01T00:00:00Z", "expiresAt": "2021-01-01T00:00:00Z",
         "grants": [{"id": "grant-1", "scope": "read-native", "revoked": False}]}

case("terms-expired", requirement([rep("surface")], terms_operation="read-native", require_terms=True),
     evidence(terms=TERMS),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "unsupported", "terms-expired"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "Expired issued terms prevent a new authorized read.",
     now="2022-01-01T00:00:00Z")

case("terms-expiry-unknown", requirement([rep("surface")], terms_operation="read-native", require_terms=True),
     evidence(terms=TERMS),
     expected("unqualified", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "unqualified", "terms-expiry-unknown"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "Without an evaluation instant an expiry assertion is unqualified.")

case("grant-withdrawn", requirement([rep("surface")], terms_operation="read-native", require_terms=True),
     evidence(terms={"issuedAt": "2020-01-01T00:00:00Z",
                     "grants": [{"id": "grant-1", "scope": "read-native", "revoked": True}]}),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "unsupported", "grant-withdrawn"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "A withdrawn grant prevents a new authorized read.")

case("grant-missing", requirement([rep("surface")], terms_operation="read-native", require_terms=True),
     evidence(terms={"issuedAt": "2020-01-01T00:00:00Z",
                     "grants": [{"id": "grant-2", "scope": "publish", "revoked": False}]}),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "unsupported", "grant-missing"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "Issued terms that do not grant the requested operation cannot authorize it.")

case("mask-below-requirement", requirement([rep("surface", minValidFraction=.9)]),
     evidence(members=[member("raster-00-00.tif", "surface", SURFACE, 2093, {
         "validSamples": 50, "selectedSamples": 100, "verticalDatum": "ellipsoid"})]),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("surface", "unsupported", "mask-evidence-below-requirement"),
     ]),
     "Measured valid-sample evidence below the required fraction is a proven violation.")

case("datum-mismatch", requirement([rep("ground", verticalDatum="EGM2008")]),
     evidence(members=[member("raster-10-00.tif", "ground", GROUND, 5000, {
         "validSamples": 900, "selectedSamples": 1000, "verticalDatum": "ellipsoid"})],
         inputs=[GROUND_INPUT]),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("ground", "unsupported", "datum-mismatch"),
     ]),
     "A vertical datum that is not retained cannot be guessed.")

case("unit-unsupported", requirement([rep("surface", units="furlong")]),
     evidence(),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "pass", "resource-accepted"),
         decision("terms", "pass", "terms-accepted"),
         decision("surface", "unsupported", "unit-unsupported"),
     ]),
     "A unit without an exact conversion is refused rather than approximated.")

case("resource-exceeded", requirement(
        [rep("surface")], resources={"maxMembers": 1, "maxBytes": 1}),
     evidence(),
     expected("unsupported", [
         decision("release", "pass", "release-match"),
         decision("resources", "unsupported", "resource-exceeded"),
         decision("terms", "pass", "terms-accepted"),
         decision("surface", "pass", "accuracy-unknown-declared"),
     ]),
     "A release beyond the declared member or byte limit is unsupported.")


def build(output):
    output.mkdir(parents=True, exist_ok=True)
    document = {"schema": SCHEMA, "fixture": "owned-fixture-v1",
                "areaRevisionId": AREA_REVISION, "cases": CASES}
    (output / "cases.json").write_text(json.dumps(document, sort_keys=True, indent=2) + "\n",
                                       encoding="utf-8")
    (output / "README.md").write_text(
        "# Suitability known-answer vectors\n\n"
        "`cases.json` is the shared input/output corpus for P01. The offline Python reader and\n"
        "the Go `internal/geoenv` decision engine read the same file. Expected results and\n"
        "reason codes are written in `scripts/data/build_suitability_cases.py` from the\n"
        "requirement rules, not generated from either implementation.\n\n"
        "Regenerate with `python scripts/data/build_suitability_cases.py`. Tests only read the\n"
        "committed vectors.\n", encoding="utf-8")
    return len(CASES)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=OUTPUT)
    args = parser.parse_args()
    print(json.dumps({"cases": build(args.output), "output": str(args.output)}))


if __name__ == "__main__":
    main()
