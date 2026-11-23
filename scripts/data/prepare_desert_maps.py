"""Prepare an owned or publisher-authorized Desert Maps geographic delivery."""

import argparse
import base64
import hashlib
import json
import unicodedata
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlsplit

from shapely import get_num_coordinates
from shapely.geometry import shape
from shapely.errors import ShapelyError

FAMILIES = {
    "أسماء رئيسة": "primary-name",
    "أسماء ثانوية": "local-name",
    "تجمع مياه": "water-related-feature",
    "تجمع مياه 1": "water-related-feature",
    "تجمع مياه 2": "water-related-feature",
    "موارد مياه": "named-water-source",
    "جبال": "upland",
    "نفود": "sand-terrain",
    "النفود": "sand-terrain",
    "طرق صحراوية": "track-drawing",
    "حدود": "boundary-drawing",
    "رسم حدود": "boundary-drawing",
    "مواقع اثرية": "heritage",
    "مواقع خطرة": "publisher-note",
}
VECTOR_TYPES = {
    "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon"
}
MAX_FEATURES = 5000
MAX_BYTES = 8 << 20
COLLECTION_PREFIX = b'{"type":"FeatureCollection","features":['
COLLECTION_SUFFIX = b']}'


def encode(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False,
                      allow_nan=False).encode("utf-8")


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def search_key(value):
    text = unicodedata.normalize("NFC", value).translate(str.maketrans("أإآٱ", "اااا"))
    return "".join(c for c in text if c != "ـ" and not 0x64B <= ord(c) <= 0x652
                   and c != "\u0670")


def parse_time(value):
    time = datetime.fromisoformat(value)
    if time.utcoffset() is None:
        raise ValueError("timestamps require an explicit offset")
    return time


def require_grant(rights, *operations):
    terms = urlsplit(rights.get("termsUrl", ""))
    if not all(rights.get(k) for k in ("id", "attribution", "legalBasis")) or (
        terms.scheme != "https" or not terms.hostname or terms.username is not None
    ):
        raise ValueError("rights require an ID, attribution, legal basis and HTTPS terms")
    if rights.get("expiresAt") and parse_time(rights["expiresAt"]) <= datetime.now(timezone.utc):
        raise ValueError("source grant has expired")
    if any(rights.get(operation) is not True for operation in operations):
        raise ValueError("source grant does not permit " + ", ".join(operations))


def valid_geometry(raw):
    if not isinstance(raw, dict) or raw.get("type") not in VECTOR_TYPES or "crs" in raw:
        return False
    try:
        coordinates = raw.get("coordinates")
        if raw["type"] in {"Polygon", "MultiPolygon"}:
            polygons = [coordinates] if raw["type"] == "Polygon" else coordinates
            if any(len(ring) < 4 or ring[0] != ring[-1] for polygon in polygons for ring in polygon):
                return False
        geometry = shape(raw)
        west, south, east, north = geometry.bounds
        return (
            not geometry.is_empty and geometry.is_valid and not geometry.has_z
            and not geometry.has_m and -180 <= west <= east <= 180
            and -90 <= south <= north <= 90
        )
    except (ShapelyError, TypeError, ValueError, IndexError, KeyError):
        return False


def reviewed_aliases(entry):
    rows = entry.get("reviewedAliases", [])
    names = []
    for alias in rows:
        name = alias.get("name")
        if not isinstance(name, str) or not name.strip() or "|" in name or not alias.get("reviewRef"):
            raise ValueError("a reviewed alias requires a name and review reference")
        if name not in names:
            names.append(name)
    aliases = " | ".join(names)
    if len(aliases.encode("utf-8")) > 2000:
        raise ValueError("reviewed aliases exceed the geographic catalog budget")
    return aliases


def verify_control_point(region):
    control = region.get("controlPoint")
    if control is None:
        return
    source = next((entry.get("geometry") for layer in region["layers"] if layer["id"] == control["layerId"]
                   for entry in layer["entries"] if entry.get("id") == control["entryId"]), None)
    if not valid_geometry(source) or source["type"] != "Point" or source["coordinates"] != control["coordinates"]:
        raise ValueError("regional control point mismatch; verify longitude and latitude order")


def cross_reference(entry):
    match = entry.get("crossReference")
    if match is None:
        return None
    if match.get("state") not in ("matched", "ambiguous", "unmatched") or not match.get("reason"):
        raise ValueError("cross-reference requires a disposition and reason")
    if match["state"] == "matched":
        if not all(match.get(k) for k in ("featureId", "editionId", "reviewRef", "reviewedAt")):
            raise ValueError("matched cross-reference requires exact feature, edition and review")
        parse_time(match["reviewedAt"])
    return match


def normalize(raw, rights):
    require_grant(rights, "retainRaw", "allowAnalysis")
    delivery = json.loads(raw)
    if delivery.get("schema") != "emad.desert-maps-delivery.v1" or delivery.get("crs") != "OGC:CRS84":
        raise ValueError("expected an EMAD Desert Maps delivery in WGS84 longitude, latitude")
    for key in ("publisher", "version"):
        if not isinstance(delivery.get(key), str) or not delivery[key].strip():
            raise ValueError(f"delivery requires {key}")
    parse_time(delivery["retrievedAt"])
    file_hash = digest(raw)
    features, outcomes, layers = [], [], []
    maps = delivery["maps"]
    if not maps or len({item["id"] for item in maps}) != len(maps):
        raise ValueError("delivery requires distinct regional map IDs")
    if "mapIds" in rights and not {item["id"] for item in maps}.issubset(rights["mapIds"]):
        raise ValueError("delivery includes a regional map outside the admitted grant")
    for region in maps:
        if not region["layers"] or len({layer["id"] for layer in region["layers"]}) != len(region["layers"]):
            raise ValueError("map requires distinct layer IDs")
        verify_control_point(region)
        for layer in region["layers"]:
            entries = layer["entries"]
            if layer["entryCount"] != len(entries):
                raise ValueError("layer entry count differs from its delivery manifest")
            summary = {"mapId": region["id"], "layerId": layer["id"], "region": region["region"],
                       "title": layer["title"], "count": len(entries), "searchEligible": 0,
                       "displayEligible": 0}
            reason_counts = Counter()
            ids = set()
            for index, entry in enumerate(entries):
                provider_id = entry.get("id")
                if provider_id is not None:
                    if not isinstance(provider_id, str) or not provider_id or provider_id in ids:
                        raise ValueError("provider entry IDs must be unique within a layer")
                    ids.add(provider_id)
                identity = ["provider", provider_id] if provider_id else ["edition", file_hash, index]
                key = "desert-maps:" + digest(encode([region["id"], layer["id"], identity]))
                name = entry.get("name")
                geometry = entry.get("geometry")
                family = FAMILIES.get(layer["title"])
                reasons = []
                if family is None:
                    reasons.append("unmapped-layer")
                if not valid_geometry(geometry):
                    reasons.append("invalid-or-missing-geometry")
                name_ok = isinstance(name, str) and bool(name.strip()) and len(name.encode("utf-8")) <= 200
                named = not reasons and name_ok and geometry["type"] == "Point"
                if not name_ok:
                    reasons.append("missing-or-overlength-name")
                thematic = family is not None and "invalid-or-missing-geometry" not in reasons
                outcomes.append({"key": key, "mapId": region["id"], "layerId": layer["id"],
                                 "ordinal": index, "providerEntryId": provider_id,
                                 "disposition": "eligible" if thematic else "review",
                                 "searchEligible": named, "displayEligible": thematic, "reasons": reasons})
                summary["searchEligible"] += named
                summary["displayEligible"] += thematic
                reason_counts.update(reasons)
                if thematic:
                    properties = {
                        "key": key, "name": name,
                        "aliases": reviewed_aliases(entry),
                        "reviewedAliases": entry.get("reviewedAliases", []),
                        "searchKey": search_key(name) if name_ok else None,
                        "providerEntryId": provider_id, "publisherRegion": region["region"],
                        "publisher": delivery["publisher"], "mapId": region["id"], "layerId": layer["id"],
                        "layerTitle": layer["title"], "family": family, "description": entry.get("description"),
                        "nameLanguage": entry.get("nameLanguage"), "subtype": entry.get("subtype"),
                        "crossReference": cross_reference(entry),
                        "meaning": "Publisher reference geometry",
                        "geometryRole": {"Point": "anchor", "MultiPoint": "anchor",
                                         "LineString": "drawn-line", "MultiLineString": "drawn-line"
                                         }.get(geometry["type"], "supplied-area"),
                        "sourceObservedAt": entry.get("observedAt"),
                        "sourceVerifiedAt": entry.get("verifiedAt"), "accuracyM": None,
                    }
                    features.append({"type": "Feature", "id": key, "properties": properties,
                                     "geometry": geometry})
            summary["reasonCounts"] = dict(reason_counts)
            layers.append(summary)
    return {
        "schema": "emad.desert-maps-normalized.v1", "sourceSha256": file_hash,
        "publisherVersion": delivery["version"], "retrievedAt": delivery["retrievedAt"],
        "sourceCRS": delivery["crs"],
        "complete": delivery.get("complete") is True, "rights": rights,
        "layers": layers, "entries": outcomes,
        "reference": {"type": "FeatureCollection", "features": features},
    }


def bounded_collections(features, max_features, max_bytes, max_vertices=None):
    batch = []
    overhead = len(COLLECTION_PREFIX) + len(COLLECTION_SUFFIX)
    size, vertices = overhead, 0
    for feature in features:
        raw = encode(feature)
        points = get_num_coordinates(shape(feature["geometry"])) if max_vertices else 0
        if len(raw) + overhead > max_bytes or (max_vertices and points > max_vertices):
            raise ValueError("one feature exceeds the reference artifact limits")
        if batch and (
            len(batch) == max_features or size + len(raw) + 1 > max_bytes
            or (max_vertices and vertices + points > max_vertices)
        ):
            yield COLLECTION_PREFIX + b",".join(batch) + COLLECTION_SUFFIX
            batch, size, vertices = [], overhead, 0
        size += len(raw) + bool(batch)
        vertices += points
        batch.append(raw)
    if batch:
        yield COLLECTION_PREFIX + b",".join(batch) + COLLECTION_SUFFIX


def place_batches(normalized):
    selected = {item["key"] for item in normalized["entries"] if item["searchEligible"]}
    features = (
        {**feature, "properties": {**feature["properties"], "kind": "place", "parentKey": "",
                                    "allowTerrainDisplay": normalized["rights"].get("allow3DDisplay") is True}}
        for feature in normalized["reference"]["features"] if feature["id"] in selected
    )
    yield from bounded_collections(features, MAX_FEATURES, MAX_BYTES)


def atlas_rights(rights):
    return {k: v for k, v in rights.items() if k in (
        "id", "legalBasis", "attribution", "termsUrl", "retainRaw", "allowAnalysis", "expiresAt",
    )}


def publish_points(api, source, normalized, expected_head):
    rights = normalized["rights"]
    require_grant(rights, "retainRaw", "allowAnalysis", "allowAtlasDisplay")
    if not normalized["complete"]:
        raise ValueError("partial deliveries cannot replace the active release")
    batches = list(place_batches(normalized))
    if not 1 <= len(batches) <= 256:
        raise ValueError("a release requires 1–256 nonempty geography editions")
    # The operator supplies the head reviewed with the delivery. On retry the API
    # accepts the already-active version without replacing an unrelated head.
    receipt = {k: normalized[k] for k in ("sourceSha256", "publisherVersion", "retrievedAt", "schema")}
    admitted_rights = atlas_rights(rights)
    if rights.get("allowOfflineDistribution") is True and rights.get("allowCivilianExercise") is True:
        admitted_rights["outputPermissions"] = {"geographicContextExport": True}
    edition_ids = []
    for raw in batches:
        sha = digest(raw)
        result = api.call("POST", "/geography/editions", {
            "sourceId": source, "staged": True, "label": "Desert Maps · place names",
            "version": sha, "sha256": sha, "artifact": base64.b64encode(raw).decode("ascii"),
            "receipt": receipt, "rights": admitted_rights,
        })
        edition_ids.append(result["id"])
    version = digest(encode([normalized["schema"], sorted(edition_ids)]))
    return api.call("POST", "/geography/releases", {
        "sourceId": source, "key": "desert-maps-places", "version": version,
        "editionIds": edition_ids, "expectedHead": expected_head, "activate": True,
    })


def thematic_payloads(normalized, source):
    rights = normalized["rights"]
    require_grant(rights, "retainRaw", "allowAnalysis", "allowThematicDisplay")
    if not normalized["complete"]:
        raise ValueError("thematic publication requires a complete delivery")
    inventory_hash = digest(encode(normalized))
    features_by_region = {}
    for feature in normalized["reference"]["features"]:
        properties = feature["properties"]
        vector = {**feature, "properties": {
            **properties, "layer": "desert-maps", "attribution": rights["attribution"],
            "sourceEditionSha256": normalized["sourceSha256"],
        }}
        features_by_region.setdefault(properties["mapId"], []).append(vector)
    for map_id, features in sorted(features_by_region.items()):
        publisher = features[0]["properties"]["publisher"]
        region = features[0]["properties"]["publisherRegion"]
        group = f"desert-maps-{digest(encode(map_id))[:8]}-{inventory_hash[:12]}"
        for i, raw in enumerate(bounded_collections(features, 1000, 524288, 20000), 1):
            geometries = [shape(item["geometry"]).bounds for item in json.loads(raw)["features"]]
            west = max(-180, min(b[0] for b in geometries) - 0.000001)
            south = max(-90, min(b[1] for b in geometries) - 0.000001)
            east = min(180, max(b[2] for b in geometries) + 0.000001)
            north = min(90, max(b[3] for b in geometries) + 0.000001)
            if (east - west) * (north - south) > 400:
                raise ValueError("thematic artifact exceeds the reference AOI limit")
            yield {
                "sourceId": source, "datasetKey": f"{group}-{i}", "version": inventory_hash,
                "label": f"Desert Maps · {region} · {normalized['publisherVersion']}",
                "kind": "vector", "originalProducer": publisher, "crs": "EPSG:4326",
                "verticalDatum": "not_applicable", "resolution": {"value": 0, "unit": "unknown"},
                "aoi": {"west": west, "south": south, "east": east, "north": north},
                "validity": "Source edition; dates and present conditions may be unknown",
                "quality": "Publisher reference geometry; positional accuracy unknown",
                "transformation": "WGS84 longitude/latitude, source geometry preserved",
                "rights": atlas_rights(rights),
                "retrievedAt": normalized["retrievedAt"],
                "retrievalReference": "https://sites.google.com/view/desertmaps/home",
                "parentArtifactSha256": normalized["sourceSha256"],
                "sha256": digest(raw), "artifact": base64.b64encode(raw).decode("ascii"),
            }


def compare(previous, candidate):
    """Compare complete editions without interpreting a missing entry as withdrawn."""
    old = {entry["key"]: entry for entry in previous["entries"]}
    new = {entry["key"]: entry for entry in candidate["entries"]}
    old_features = {f["id"]: f for f in previous["reference"]["features"]}
    new_features = {f["id"]: f for f in candidate["reference"]["features"]}
    lost_search = {key for key in old.keys() & new.keys()
                   if old[key]["searchEligible"] and not new[key]["searchEligible"]}
    return {
        "previousSha256": previous["sourceSha256"],
        "candidateSha256": candidate["sourceSha256"],
        "candidateComplete": candidate["complete"],
        "added": sorted(new.keys() - old.keys()),
        "changed": sorted(key for key in old.keys() & new.keys()
                          if old_features.get(key) != new_features.get(key)),
        "needsContinuityReview": sorted((old.keys() - new.keys()) | lost_search),
    }


def prepare_handoff(normalized, selected_key, geographic_release_id, expiry):
    rights = normalized["rights"]
    require_grant(rights, "allowOfflineDistribution", "allowCivilianExercise")
    expires = parse_time(expiry)
    if expires <= datetime.now(timezone.utc) or (
        rights.get("expiresAt") and expires > parse_time(rights["expiresAt"])
    ):
        raise ValueError("handoff expiry is outside the source grant")
    if not geographic_release_id or not normalized["complete"]:
        raise ValueError("handoff requires a pinned geography release and complete delivery")
    try:
        feature = next(f for f in normalized["reference"]["features"] if f["id"] == selected_key)
    except StopIteration as failure:
        raise ValueError("selected feature is unavailable in this edition") from failure
    return {
        "schema": "emad.desert-maps-civilian-reference.v1", "purpose": "civilian-exercise-reference",
        "geographyReleaseId": geographic_release_id,
        "normalizedSha256": digest(encode(normalized)), "featureSha256": digest(encode(feature)),
        "attribution": rights["attribution"], "rightsId": rights["id"],
        "expiresAt": expiry, "feature": feature,
    }


def read_handoff(raw, expected_hash, now):
    if digest(raw) != expected_hash:
        raise ValueError("handoff does not match its trusted digest")
    handoff = json.loads(raw)
    if handoff["schema"] not in ("emad.desert-maps-civilian-reference.v1", "emad.desert-maps-civilian-reference.v2"):
        raise ValueError("unsupported handoff")
    if parse_time(handoff["expiresAt"]) <= now:
        raise ValueError("handoff has expired")
    if handoff["schema"] == "emad.desert-maps-civilian-reference.v2":
        member = base64.b64decode(handoff["member"], validate=True)
        if digest(member) != handoff["memberSha256"]:
            raise ValueError("handoff feature digest mismatch")
        return json.loads(member)
    if digest(encode(handoff["feature"])) != handoff["featureSha256"]:
        raise ValueError("handoff feature digest mismatch")
    return handoff["feature"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("delivery", type=Path)
    parser.add_argument("--rights", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--publish-source", help="Registered geography source ID")
    parser.add_argument("--expected-head", help="Exact reviewed release head, or empty for first release")
    parser.add_argument("--previous", type=Path, help="Write the edition comparison to stdout")
    parser.add_argument("--reviewed-removals", type=Path,
                        help="Reviewed comparison JSON permitting the listed missing keys")
    parser.add_argument("--thematic-requests", type=Path, help="Prepare separately reviewable reference imports")
    parser.add_argument("--thematic-source", help="Registered source ID for thematic reference imports")
    args = parser.parse_args()
    if args.publish_source and args.expected_head is None:
        parser.error("--publish-source requires --expected-head")
    if args.publish_source and args.expected_head and not args.previous:
        parser.error("replacing a release requires --previous")
    normalized = normalize(args.delivery.read_bytes(), json.loads(args.rights.read_bytes()))
    prepared = encode(normalized) + b"\n"
    try:
        with args.output.open("xb") as output:
            output.write(prepared)
    except FileExistsError:
        if args.output.read_bytes() != prepared:
            raise
    if args.previous:
        difference = compare(json.loads(args.previous.read_bytes()), normalized)
        print(json.dumps(difference))
        if args.publish_source and difference["needsContinuityReview"] and (
            not args.reviewed_removals or json.loads(args.reviewed_removals.read_bytes()) != difference
        ):
            parser.error("missing records require a reviewed comparison for this exact delivery pair")
    if args.thematic_requests:
        if not args.thematic_source:
            parser.error("--thematic-requests requires --thematic-source")
        requests = list(thematic_payloads(normalized, args.thematic_source))
        if len(requests) > 64:
            parser.error("thematic imports exceed the reference catalog budget")
        with args.thematic_requests.open("xb") as output:
            output.write(encode(requests) + b"\n")
    if args.publish_source:
        from install_local import API
        release = publish_points(API(profile="local-source-admin"), args.publish_source,
                                 normalized, args.expected_head)
        print(json.dumps({"releaseId": release["id"]}))


if __name__ == "__main__":
    main()
