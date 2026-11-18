"""Versioned consumer-requirement decisions for a released environment.

This module is deliberately free of GDAL, the database and the network. The Go
API implements the same rules over the same evidence document, and both are
tested against the shared examples in ``testdata/geospatial/suitability-cases``.

Vocabulary
----------
``supported``    every required item passed.
``unsupported``  a proven violation makes a required item unusable.
``unqualified``  the evidence needed to decide is missing.

Spatial predicates use the release's stored area geometry (including holes) and
a fixed 9x9 lattice over the requirement's bounding box. Mask and coverage
evidence come from the release members. The lattice is a documented sampling
predicate, not a claim of full raster coverage.
"""
import hashlib
import json
import math
import re

SCHEMA = "emad.environment-requirement.v1"
RESULT_SCHEMA = "emad.environment-suitability.v1"
EVIDENCE_SCHEMA = "emad.environment-evidence.v1"
LATTICE = 9
HASH = re.compile(r"[a-f0-9]{64}\Z")

# Reason codes are part of the contract. Adding one is additive; renaming or
# changing the meaning of an existing one is a schema change.
REASONS = {
    "release-match": "release and area identity match the requirement",
    "release-mismatch": "the requirement names a different release manifest",
    "area-revision-mismatch": "the requirement names a different area revision",
    "terms-accepted": "issued terms cover the requested operation",
    "terms-missing": "no issued terms accompany this evidence",
    "terms-expiry-unknown": "expiry was not stated for the evaluation instant",
    "terms-expired": "the issued terms have expired",
    "grant-missing": "the issued terms do not grant the requested operation",
    "grant-withdrawn": "the issued grant was withdrawn",
    "resource-accepted": "members and bytes are within the declared limits",
    "resource-exceeded": "the release exceeds the declared member or byte limit",
    "representation-present": "a native representation of the required layer is present",
    "representation-absent": "the required native layer is absent",
    "optional-representation-absent": "an optional layer is absent",
    "optional-omitted": "an optional requirement was omitted without affecting the result",
    "datum-mismatch": "no retained source uses the required vertical datum",
    "unit-unsupported": "the required unit is not a native or exactly convertible unit",
    "mask-evidence-below-requirement": "valid-sample evidence is below the required fraction",
    "accuracy-unknown-declared": "the consumer accepts the release's declared unknown accuracy",
    "accuracy-unevidenced": "independent accuracy evidence is required but absent",
    "accuracy-exceeded": "an accepted assessment exceeds the declared tolerance",
    "accuracy-evidenced": "an accepted assessment covers the requirement and meets the tolerance",
    "assessment-area-insufficient": "the assessment covers less than the requirement area",
    "assessment-member-mismatch": "the assessment names a different member digest",
    "time-background-accepted": "dated background use accepts mixed source times",
    "time-window-accepted": "every required source falls inside the common window",
    "time-incompatible": "a source time falls outside the required window",
    "time-unknown": "a required source has no declared time",
    "area-covered": "the requirement area lies within the release area",
    "area-partially-covered": "part of the requirement area lies outside the release area",
    "area-outside-release": "the requirement area lies outside the release area",
    "area-partially-covered-accepted": "partial area coverage was explicitly accepted",
    "area-invalid": "the requirement area has no inside samples",
    "visual-only-derivative": "only a visual derivative exists for this layer",
}

PASS, UNSUPPORTED, UNQUALIFIED, OMITTED = "pass", "unsupported", "unqualified", "optional-omitted"


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


def digest(value):
    return hashlib.sha256(canonical(value).encode()).hexdigest()


def release_digest(evidence):
    """Digest of the release identity and member evidence used by decisions."""
    subset = {
        "schema": evidence.get("schema"),
        "manifestSha256": evidence.get("manifestSha256"),
        "areaRevisionId": evidence.get("areaRevisionId"),
        "area": evidence.get("area"),
        "members": evidence.get("members", []),
        "inputs": evidence.get("inputs", []),
    }
    return digest(subset)


def _ring(ring, x, y):
    hit = False
    count = len(ring)
    for index in range(count):
        x1, y1 = ring[index][0], ring[index][1]
        x2, y2 = ring[(index + 1) % count][0], ring[(index + 1) % count][1]
        if (y1 > y) != (y2 > y):
            intersection = x1 + (y - y1) / (y2 - y1) * (x2 - x1)
            if x < intersection:
                hit = not hit
    return hit


def _polygon(rings, x, y):
    if not rings or not _ring(rings[0], x, y):
        return False
    return not any(_ring(hole, x, y) for hole in rings[1:])


def inside(geometry, x, y):
    if not geometry:
        return False
    kind = geometry.get("type")
    coordinates = geometry.get("coordinates")
    if kind == "Polygon":
        return _polygon(coordinates, x, y)
    if kind == "MultiPolygon":
        return any(_polygon(part, x, y) for part in coordinates)
    return False


def bounds(geometry):
    if not geometry:
        raise ValueError("A requirement area is required for this predicate")
    points = _points(geometry.get("coordinates"))
    xs = [p[0] for p in points]
    ys = [p[1] for p in points]
    return min(xs), min(ys), max(xs), max(ys)


def _points(value):
    if value and isinstance(value[0], (int, float)):
        return [(float(value[0]), float(value[1]))]
    out = []
    for part in value or []:
        out.extend(_points(part))
    return out


def lattice(geometry, resolution=LATTICE):
    west, south, east, north = bounds(geometry)
    samples = []
    for column in range(resolution):
        x = west + (column + .5) * (east - west) / resolution
        for row in range(resolution):
            y = south + (row + .5) * (north - south) / resolution
            samples.append((x, y))
    return samples


def coverage(requirement_area, release_area, resolution=LATTICE):
    points = lattice(requirement_area, resolution)
    required = [point for point in points if inside(requirement_area, *point)]
    if not required:
        return 0.0, 0, 0
    covered = sum(1 for point in required if inside(release_area, *point))
    return covered / len(required), covered, len(required)


def _decision(requirement, kind, result, reason, detail, evidence_sha, **extra):
    record = {"requirement": requirement, "kind": kind, "result": result,
              "reason": reason, "detail": detail, "evidenceSha256": evidence_sha}
    record.update(extra)
    return record


def _fraction(member):
    details = member.get("details") or {}
    selected = details.get("selectedSamples")
    valid = details.get("validSamples")
    if isinstance(selected, int) and isinstance(valid, int) and selected > 0 and 0 <= valid <= selected:
        return valid / selected
    return None


def _current_assessments(evidence):
    return [item for item in evidence.get("assessments", []) if not item.get("supersededBy")]


def evaluate(requirement, evidence, now=None):
    """Return a decision for every requirement in the document."""
    if requirement.get("schema") != SCHEMA:
        raise ValueError("Unsupported requirement schema")
    if evidence.get("schema") != EVIDENCE_SCHEMA:
        raise ValueError("Unsupported evidence schema")
    release_sha = release_digest(evidence)
    decisions = []
    overall = PASS

    def add(item):
        nonlocal overall
        decisions.append(item)
        if item["result"] == UNSUPPORTED:
            overall = UNSUPPORTED
        elif item["result"] == UNQUALIFIED and overall != UNSUPPORTED:
            overall = UNQUALIFIED

    named = requirement.get("release") or {}
    identity_ok = (named.get("manifestSha256") == evidence.get("manifestSha256")
                   and named.get("areaRevisionId") == evidence.get("areaRevisionId"))
    if identity_ok:
        add(_decision("release", "release", PASS, "release-match",
                      "The evidence names the required release and area revision.", release_sha))
    else:
        add(_decision("release", "release", UNSUPPORTED, "release-mismatch",
                      "The requirement and evidence name different releases.", release_sha))

    limits = requirement.get("resources") or {}
    members = evidence.get("members", [])
    total = sum(int(member.get("bytes") or 0) for member in members)
    if len(members) > limits.get("maxMembers", 1024) or total > limits.get("maxBytes", 4 << 30):
        add(_decision("resources", "resources", UNSUPPORTED, "resource-exceeded",
                      "The release exceeds the declared member or byte limit.", release_sha,
                      observed={"members": len(members), "bytes": total}))
    else:
        add(_decision("resources", "resources", PASS, "resource-accepted",
                      "The release is within the declared resource limits.", release_sha,
                      observed={"members": len(members), "bytes": total}))

    terms = evidence.get("terms")
    term_requirement = requirement.get("terms") or {}
    if not terms:
        if term_requirement.get("requireIssued"):
            add(_decision("terms", "terms", UNSUPPORTED, "terms-missing",
                          "No issued terms accompany this evidence.", release_sha))
        else:
            add(_decision("terms", "terms", PASS, "terms-accepted",
                          "No issued terms were required.", release_sha))
    else:
        grant = next((item for item in terms.get("grants", [])
                      if item.get("scope") == term_requirement.get("operation")), None)
        expires = terms.get("expiresAt")
        expiry = _instant(expires) if expires else None
        now_value = _instant(now) if now else None
        if expires and now is None:
            add(_decision("terms", "terms", UNQUALIFIED, "terms-expiry-unknown",
                          "The issued terms state an expiry but the evaluation instant is missing.",
                          release_sha, termsExpiresAt=expires))
        elif expires and (expiry is None or now_value is None):
            add(_decision("terms", "terms", UNQUALIFIED, "terms-expiry-unknown",
                          "The issued expiry or evaluation instant is not recognized.", release_sha,
                          termsExpiresAt=expires))
        elif expiry is not None and expiry <= now_value:
            add(_decision("terms", "terms", UNSUPPORTED, "terms-expired",
                          "The issued terms expired before this evaluation.", release_sha,
                          termsExpiresAt=expires))
        elif term_requirement.get("operation") and grant is None:
            add(_decision("terms", "terms", UNSUPPORTED, "grant-missing",
                          "The issued terms do not grant the requested operation.", release_sha,
                          operation=term_requirement.get("operation")))
        elif term_requirement.get("operation") and grant.get("revoked"):
            add(_decision("terms", "terms", UNSUPPORTED, "grant-withdrawn",
                          "The issued grant for this operation was withdrawn.", release_sha,
                          grantId=grant.get("id")))
        else:
            add(_decision("terms", "terms", PASS, "terms-accepted",
                          "The issued terms cover the requested operation.", release_sha,
                          termsExpiresAt=terms.get("expiresAt"), issuedAt=terms.get("issuedAt")))

    area_requirement = requirement.get("area")
    coverage_fraction = None
    if area_requirement:
        fraction, covered, total_points = coverage(area_requirement.get("geometry"), evidence.get("area"))
        coverage_fraction = fraction
        if total_points == 0:
            add(_decision("area", "area", UNSUPPORTED, "area-invalid",
                          "The requirement area has no inside lattice samples.", release_sha))
        elif covered == 0:
            add(_decision("area", "area", UNSUPPORTED, "area-outside-release",
                          "The requirement area lies outside the release area.", release_sha,
                          coverage=0.0, samples=total_points))
        elif coverage_fraction < 1:
            accepted = bool(area_requirement.get("allowPartial"))
            add(_decision("area", "area", PASS if accepted else UNSUPPORTED,
                          "area-partially-covered-accepted" if accepted else "area-partially-covered",
                          "Only part of the requirement area is inside the release area.",
                          release_sha, coverage=coverage_fraction, samples=total_points))
        else:
            add(_decision("area", "area", PASS, "area-covered",
                          "The requirement area lies within the release area.", release_sha,
                          coverage=1.0, samples=total_points))

    window = requirement.get("time") or {}
    mode = window.get("mode")
    if mode:
        layer_inputs = {layer: [item for item in evidence.get("inputs", [])
                                if item.get("layer") == layer]
                        for layer in window.get("layers", [])}
        if mode == "background":
            add(_decision("time", "time", PASS, "time-background-accepted",
                          "Dated background use accepts the declared source times.", release_sha))
        elif mode == "common-window":
            start, end = window.get("from"), window.get("to")
            if not start or not end:
                add(_decision("time", "time", UNQUALIFIED, "time-unknown",
                              "A common window requires explicit from and to instants.", release_sha))
            else:
                start_value, end_value = _instant(start), _instant(end)
                if start_value is None or end_value is None:
                    add(_decision("time", "time", UNQUALIFIED, "time-unknown",
                                  "A common window bound is not a recognized instant.", release_sha))
                else:
                    outside = []
                    unknown = []
                    for layer, items in layer_inputs.items():
                        for item in items:
                            value = item.get("dataTime")
                            instant = _instant(value) if value else None
                            if not value or instant is None:
                                unknown.append(layer)
                            elif not (start_value <= instant <= end_value):
                                outside.append({"layer": layer, "dataTime": value})
                    if unknown:
                        add(_decision("time", "time", UNQUALIFIED, "time-unknown",
                                      "A required source has no declared time.", release_sha,
                                      unknownSources=sorted(set(unknown))))
                    elif outside:
                        add(_decision("time", "time", UNSUPPORTED, "time-incompatible",
                                      "A source time falls outside the required window.", release_sha,
                                      outside=outside))
                    else:
                        add(_decision("time", "time", PASS, "time-window-accepted",
                                      "Every required source falls inside the common window.", release_sha))
        else:
            add(_decision("time", "time", UNQUALIFIED, "time-unknown",
                          "Unsupported time mode.", release_sha))
    else:
        mode = None

    current = _current_assessments(evidence)
    visual = (evidence.get("representationAvailability") or {}).get("visual-derivative", [])

    for index, representation in enumerate(requirement.get("representations", [])):
        layer = representation.get("layer")
        name = representation.get("id") or layer or f"representation-{index}"
        required = bool(representation.get("required", True))
        layer_members = [member for member in members if member.get("layer") == layer]
        if not layer_members:
            if layer in visual and representation.get("allowVisualDerivative") is False:
                add(_decision(name, "representation", UNSUPPORTED, "visual-only-derivative",
                              "Only a visual derivative of this layer exists; native values are unavailable.",
                              release_sha, layer=layer, purpose="visual-reference"))
            elif required:
                add(_decision(name, "representation", UNSUPPORTED, "representation-absent",
                              f"No native {layer} member is retained.", release_sha, layer=layer))
            else:
                add(_decision(name, "representation", OMITTED, "optional-representation-absent",
                              f"Optional layer {layer} is absent.", release_sha, layer=layer))
                continue
            continue

        datum = representation.get("verticalDatum")
        datums = sorted({(member.get("details") or {}).get("verticalDatum") or "unknown"
                         for member in layer_members})
        if datum and datum not in datums:
            add(_decision(name, "representation", UNSUPPORTED, "datum-mismatch",
                          f"Retained {layer} datums {datums} do not include {datum}.",
                          release_sha, layer=layer, datums=datums))
            continue

        unit = representation.get("units")
        if unit and unit not in ("metre", "meter", "m"):
            add(_decision(name, "representation", UNSUPPORTED, "unit-unsupported",
                          f"Unit {unit} is not native or exactly convertible.", release_sha,
                          layer=layer, unit=unit))
            continue

        fractions = [value for value in (_fraction(member) for member in layer_members) if value is not None]
        minimum = representation.get("minValidFraction")
        if minimum is not None and fractions and max(fractions) < float(minimum):
            add(_decision(name, "representation", UNSUPPORTED, "mask-evidence-below-requirement",
                          "The best retained member's valid fraction is below the requirement.",
                          release_sha, layer=layer, validFraction=max(fractions), required=minimum))
            continue
        if minimum is not None and not fractions:
            add(_decision(name, "representation", UNQUALIFIED, "mask-evidence-below-requirement",
                          "No member records valid-sample evidence for this layer.",
                          release_sha, layer=layer))
            continue

        accuracy = representation.get("accuracy") or {"mode": "unknown-acceptable"}
        if accuracy.get("mode", "unknown-acceptable") == "unknown-acceptable":
            add(_decision(name, "representation", PASS, "accuracy-unknown-declared",
                          "The consumer accepts native inspection with unknown physical accuracy.",
                          release_sha, layer=layer, accuracy="UNKNOWN",
                          assessment=None))
            continue

        use = accuracy.get("use") or "unspecified"
        tolerance = float(accuracy.get("toleranceM", 0))
        member_digests = {member.get("sha256") for member in layer_members}
        candidates = [item for item in current
                      if item.get("layer") == layer and item.get("use") == use
                      and item.get("decision") == "ACCEPTED"]
        if not candidates:
            add(_decision(name, "representation", UNQUALIFIED, "accuracy-unevidenced",
                          "Independent accuracy is required but no accepted assessment exists.",
                          release_sha, layer=layer, use=use))
            continue
        matching = [item for item in candidates if item.get("memberSha256") in member_digests]
        if not matching:
            add(_decision(name, "representation", UNQUALIFIED, "assessment-member-mismatch",
                          "Every accepted assessment names a different member digest.",
                          release_sha, layer=layer, assessments=[item.get("id") for item in candidates]))
            continue
        covering = []
        if area_requirement:
            for item in matching:
                fraction, covered, total_points = coverage(area_requirement.get("geometry"), item.get("area"))
                if total_points and covered == total_points:
                    covering.append(item)
        else:
            covering = matching
        if not covering:
            add(_decision(name, "representation", UNQUALIFIED, "assessment-area-insufficient",
                          "No accepted assessment covers the requirement area.",
                          release_sha, layer=layer,
                          assessments=[item.get("id") for item in matching]))
            continue
        failing = [item for item in covering if float(item.get("statisticM", math.inf)) > tolerance]
        if failing:
            item = failing[0]
            add(_decision(name, "representation", UNSUPPORTED, "accuracy-exceeded",
                          "The accepted assessment statistic exceeds the declared tolerance.",
                          release_sha, layer=layer, statisticM=item.get("statisticM"),
                          toleranceM=tolerance, assessment=item.get("id"),
                          assessmentSha256=item.get("evidenceSha256")))
            continue
        item = sorted(covering, key=lambda value: (-int(value.get("sequence", 0)), str(value.get("id"))))[0]
        add(_decision(name, "representation", PASS, "accuracy-evidenced",
                      "An accepted assessment covers the requirement area and meets the tolerance.",
                      release_sha, layer=layer, statisticM=item.get("statisticM"),
                      toleranceM=tolerance, uncertaintyM=item.get("uncertaintyM"),
                      assessment=item.get("id"), assessmentSha256=item.get("evidenceSha256"),
                      controlSetSha256=item.get("controlSetSha256"), method=item.get("method")))

    return {
        "schema": RESULT_SCHEMA,
        "requirementId": requirement.get("requirementId"),
        "operation": requirement.get("operation"),
        "overall": {"pass": "supported", "unsupported": "unsupported",
                    "unqualified": "unqualified"}[overall],
        "release": {"manifestSha256": evidence.get("manifestSha256"),
                    "areaRevisionId": evidence.get("areaRevisionId"),
                    "profileId": evidence.get("profileId")},
        "evidenceSha256": release_sha,
        "terms": evidence.get("terms"),
        "coverage": coverage_fraction,
        "decisions": decisions,
        "optional": [item for item in decisions if item["result"] == OMITTED],
    }


def _instant(value):
    import datetime as dt
    try:
        return dt.datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()
    except (AttributeError, TypeError, ValueError):
        return None
