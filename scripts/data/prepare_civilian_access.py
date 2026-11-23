"""Prepare source evidence for an independently reviewed civilian access package."""

import argparse
import csv
from hashlib import sha256
import json
from pathlib import Path
import re
import sys
import zipfile

from simulation_inputs import MUNICIPAL_FIELDS, instant, municipal, require, text, utc

Q1_SHA256 = "b3619c468a0e91c06d718ed7421b7f18deb5c7d78535f54046f7f807b8b60a21"


def file_sha(path):
    with path.open("rb") as stream:
        from hashlib import file_digest
        return file_digest(stream, "sha256").hexdigest()


def source(path, identity, edition, attribution):
    return {"id": identity, "edition": edition, "sha256": file_sha(path), "attribution": attribution}


def osm_candidates(path, manifest_pin):
    import fiona
    from environment_reader import Reader, member_path

    reader = Reader(path, manifest_pin)
    members = reader.vector_members("roads")
    require(members, "prepared release has no road member")
    features = []
    with zipfile.ZipFile(path) as archive:
        for member in members:
            require(file_member_sha(archive, member["name"]) == member["sha256"], "prepared road member differs")
            with fiona.open(member_path(path, member["name"]), layer="features") as records:
                for record in records:
                    fields = dict(record.properties)
                    features.append({"sourceFeatureId": fields["source_id"], "member": member["name"],
                                     "memberSha256": member["sha256"], "geometry": dict(record.geometry),
                                     "sourceTags": json.loads(fields["source_tags"] or "{}")})
    return {"manifestSha256": reader.manifest_sha, "areaRevisionId": reader.manifest["areaRevisionId"],
            "coverage": "retained-clipped-features", "connectivity": "requires-reviewed-node-membership",
            "attribution": "© OpenStreetMap contributors, ODbL", "features": features}


def file_member_sha(archive, name):
    from hashlib import file_digest
    with archive.open(name) as member:
        return file_digest(member, "sha256").hexdigest()


def overture_candidates(segments, connectors, edition, osm=None):
    from pyproj import Geod
    from shapely.geometry import shape
    from shapely.ops import substring

    def features(file):
        doc = json.loads(file.read_bytes())
        require(doc["type"] == "FeatureCollection", "expected an Overture FeatureCollection")
        entries = doc["features"]
        require(len(entries) <= 40000, "use a bounded Overture extract")
        return entries

    connector_positions = {}
    for item in features(connectors):
        identity = text(item["id"])
        geometry = shape(item["geometry"])
        require(geometry.geom_type == "Point" and not geometry.is_empty and identity not in connector_positions,
                "invalid or duplicate Overture connector")
        connector_positions[identity] = list(geometry.coords[0])[:2]
    osm_index = set()
    osm_manifest = None
    if osm:
        retained = json.loads(osm.read_bytes())
        osm_manifest = retained["manifestSha256"]
        osm_index = {feature["sourceFeatureId"] for feature in retained["features"]}
    geod = Geod(ellps="WGS84")
    result = []
    seen = set()
    omitted = 0
    for item in features(segments):
        props = item["properties"]
        if props.get("subtype") != "road":
            continue
        way = text(item["id"])
        require(way not in seen, "duplicate Overture road")
        seen.add(way)
        line = shape(item["geometry"])
        if line.geom_type != "LineString" or line.is_empty:
            omitted += 1
            continue
        membership = sorted(((row["at"], row["connector_id"]) for row in props.get("connectors", [])))
        pieces = []
        for (begin, a), (end, b) in zip(membership, membership[1:]):
            if a == b or a not in connector_positions or b not in connector_positions or not 0 <= begin < end <= 1:
                omitted += 1
                continue
            section = substring(line, begin, end, normalized=True)
            length = geod.geometry_length(section)
            if length <= 0:
                omitted += 1
                continue
            pieces.append({"id": f"{way}:{len(pieces)+1}", "fromConnector": a, "toConnector": b,
                           "at": [begin, end], "distanceM": length, "coordinates": [list(point) for point in section.coords],
                           "endpoints": [connector_positions[a], connector_positions[b]]})
        lineage = set()
        for upstream in props.get("sources", []):
            match = re.fullmatch(r"w([1-9][0-9]*)@([1-9][0-9]*)", upstream.get("record_id") or "")
            if upstream.get("provider") == "osm" and match:
                lineage.add("osm-way-" + match[1])
        result.append({"sourceFeatureId": way, "version": props.get("version"), "class": props.get("class"),
                       "osmLineage": sorted(lineage), "retainedOSMLineage": sorted(lineage & osm_index),
                       "pieces": pieces, "rules": {k: props[k] for k in (
                           "access_restrictions", "prohibited_transitions", "level_rules", "routes", "road_flags",
                           "subclass_rules", "width_rules") if k in props}, "sources": props.get("sources", [])})
    credit = "Overture Maps Foundation and OpenStreetMap contributors, ODbL"
    sources = [source(segments, "overture-segments", edition, credit),
               source(connectors, "overture-connectors", edition, credit)]
    if osm:
        sources.append(source(osm, "retained-osm-candidates", "prepared-road-member-references", "© OpenStreetMap contributors, ODbL"))
    return {"edition": edition, "sources": sources, "retainedManifestSha256": osm_manifest,
            "connectivity": "connector-based-candidates-with-native-rules",
            "directionAndTurns": "independent-service-class-review-required",
            "excludedParts": omitted, "roads": result}


def reviewed_graph(candidates, review):
    data = json.loads(candidates.read_bytes())
    checks = json.loads(review.read_bytes())
    require(checks["candidatesSha256"] == file_sha(candidates), "candidate graph digest differs from review")
    parts = {part["id"]: (road, part) for road in data["roads"] for part in road["pieces"]}
    edges = []
    nodes = set()
    seen = set()
    require(0 < len(checks["pieces"]) <= 750, "select a bounded reviewed graph")
    for decision in checks["pieces"]:
        identity = text(decision["id"])
        require(identity not in seen, "review repeats an Overture piece")
        seen.add(identity)
        road, item = parts[identity]
        forward, reverse = decision["forward"], decision["reverse"]
        require(type(forward) is bool and type(reverse) is bool and (forward or reverse),
                "accept at least one reviewed direction per selected piece")
        retained = decision.get("retainedOSMFeatureId")
        require(retained is None or retained in road["retainedOSMLineage"], "reviewed OSM feature has no retained lineage")
        nodes.update((item["fromConnector"], item["toConnector"]))
        for suffix, usable, a, b in (("f", forward, "fromConnector", "toConnector"),
                                     ("r", reverse, "toConnector", "fromConnector")):
            if usable:
                edge = {"id": f"{identity}:{suffix}", "from": item[a], "to": item[b], "distanceM": item["distanceM"],
                        "sourceId": "overture-segments", "featureId": road["sourceFeatureId"]}
                if retained:
                    edge["retainedOSMFeatureId"] = retained
                edges.append(edge)
    boundary = checks.get("boundaryNodes", [])
    require(set(boundary) <= nodes, "review boundary is outside the selected graph")
    sources = data["sources"] + [source(review, "civilian-road-review", text(checks["edition"]), text(checks["reviewer"]))]
    result = {"graphEdition": checks["edition"], "serviceClass": text(checks["serviceClass"]),
              "sources": sources, "nodes": sorted(nodes), "edges": edges, "boundaryNodes": boundary,
              "turnRestrictions": checks["turnRestrictions"]}
    if data.get("retainedManifestSha256"):
        result["retainedManifestSha256"] = data["retainedManifestSha256"]
    return result


def select_municipal(path, declaration):
    raw = path.read_bytes()
    require(sha256(raw).hexdigest() == Q1_SHA256, "municipal file does not match Riyadh Q1 2026 JSON")
    definition = json.loads(declaration.read_bytes())
    rows = municipal(raw, include_rows=True)["rows"]
    source_id = "riyadh-940"
    selected = []
    seen = set()
    for entry in definition["scheduledRows"]:
        ordinal = entry["ordinal"]
        require(type(ordinal) is int and 1 <= ordinal <= len(rows) and ordinal not in seen,
                "municipal row ordinal is invalid or repeated")
        seen.add(ordinal)
        row = rows[ordinal - 1]
        availability = instant(entry["availableAt"])
        require(availability >= instant(row["receiptAt"]), "exercise availability precedes receipt")
        result = {"id": row["id"], "sourceId": source_id, "category": row["raw"][MUNICIPAL_FIELDS["category"]],
                  "receiptAt": row["receiptAt"], "availableAt": utc(availability)}
        if row["reportedPlaceLabel"] is not None:
            result["placeLabel"] = row["reportedPlaceLabel"]
        selected.append(result)
    return {"temporalMeaning": "authored-availability-of-retrospective-reports",
            "scheduleSha256": file_sha(declaration),
            "sources": [source(path, source_id, "2026-q1", "Riyadh Municipality 940 service requests"),
                        source(declaration, "municipal-exercise-schedule", "v1", "Authored report availability schedule")],
            "reports": selected}


def planned_works(path, edition, identity, attribution):
    result = []
    seen = set()
    with path.open(encoding="utf-8-sig", newline="") as raw:
        reader = csv.DictReader(raw)
        columns = {"id", "edgeId", "roadLabel", "revision", "availableAt", "from", "toExclusive", "status"}
        require(reader.fieldnames is not None and set(reader.fieldnames) == columns and
                len(reader.fieldnames) == len(columns), "expected the reviewed works CSV columns")
        for row in reader:
            require(set(row) == columns and all(isinstance(value, str) for value in row.values()),
                    "incomplete works record")
            revision = int(row["revision"])
            key = (text(row["id"]), revision)
            require(revision > 0 and key not in seen, "duplicate or invalid works revision")
            seen.add(key)
            start, end, known = (instant(row[key]) for key in ("from", "toExclusive", "availableAt"))
            require(start < end and row["status"] in {"scheduled", "cancelled"}, "invalid works interval or state")
            record = {"id": key[0], "sourceId": identity, "revision": revision, "roadLabel": text(row["roadLabel"]),
                      "availableAt": utc(known), "from": utc(start), "toExclusive": utc(end), "status": row["status"]}
            if row["edgeId"].strip():
                record["edgeId"] = row["edgeId"].strip()
            result.append(record)
    return {"sources": [source(path, identity, edition, attribution)], "permits": result}


def population_cells(path, regions, year, edition, allocation):
    from numpy.ma import is_masked
    import rasterio
    from rasterio.features import geometry_window
    from rasterio.transform import xy
    from shapely.geometry import box, shape
    from shapely import transform
    from pyproj import Transformer

    zones = {}
    doc = json.loads(regions.read_bytes())
    require(doc["type"] == "FeatureCollection" and len(doc["features"]) <= 100, "expected bounded service zones")
    for feature in doc["features"]:
        identity = text(feature["id"])
        geom = shape(feature["geometry"])
        require(geom.is_valid and geom.geom_type in {"Polygon", "MultiPolygon"} and not geom.is_empty and
                identity not in zones, "invalid or repeated service zone")
        require(all(geom.intersection(other).area <= 1e-14 for other in zones.values()), "service zones overlap")
        zones[identity] = geom
    require(zones, "no service zones")
    project = Transformer.from_crs(4326, 6933, always_xy=True).transform
    cells = []
    with rasterio.open(path) as raster:
        require(raster.driver == "GTiff" and raster.crs is not None and raster.crs.to_epsg() == 4326 and raster.count == 1 and
                raster.scales == (1.0,) and raster.offsets == (0.0,) and raster.transform.b == raster.transform.d == 0,
                "expected native WGS84 population counts and a north-up grid")
        area = geometry_window(raster, [g.__geo_interface__ for g in zones.values()])
        require(area.width * area.height <= 10000, "select a smaller service area for population analysis")
        require(raster.transform.a > 0 and raster.transform.e < 0, "population grid must be north-up")
        grid = {"crs": raster.crs.to_string(), "transform": list(raster.transform)[:6],
                "width": raster.width, "height": raster.height}
        values = raster.read(1, window=area, masked=True)
        for r in range(int(area.height)):
            for c in range(int(area.width)):
                row, col = r + int(area.row_off), c + int(area.col_off)
                left, top = xy(raster.transform, row, col, offset="ul")
                right, bottom = xy(raster.transform, row, col, offset="lr")
                cell = box(left, bottom, right, top)
                fractions = {}
                if allocation == "cell-center":
                    matches = sorted(key for key, zone in zones.items() if zone.covers(cell.centroid))
                    if matches:
                        fractions[matches[0]] = 1.0
                else:
                    for identity, zone in zones.items():
                        overlap = cell.intersection(zone)
                        if overlap.area > 0:
                            fractions[identity] = (transform(overlap, project, interleaved=False).area /
                                                   transform(cell, project, interleaved=False).area)
                if not fractions:
                    continue
                total = sum(fractions.values())
                fractions = {key: fraction / max(1.0, total) for key, fraction in fractions.items()}
                value = values[r, c]
                cells.append({"id": f"{row}:{col}", "people": None if is_masked(value) else float(value),
                              "zoneShares": fractions})
    identity = "worldpop"
    return {"sources": [source(path, identity, edition, "WorldPop, University of Southampton, R2025A"),
                        source(regions, "population-service-zones", allocation, "Reviewed civilian service-zone allocation")],
            "allocation": allocation, "zonesSha256": file_sha(regions), "grid": grid,
            "population": {"sourceId": identity, "year": year, "allocation": allocation, "cells": cells}}


def assemble(exercise, graph, municipal_file=None, works_file=None, population=None):
    """Join normalized inputs without inventing evidence or changing identity."""
    package = json.loads(exercise.read_bytes())
    require(package["schema"] == "emad.civilian-road-exercise.v1", "expected an authored or reviewed exercise definition")
    sources = {item["id"]: item for item in package.get("sources", [])}
    require(len(sources) == len(package.get("sources", [])), "exercise repeats a source identity")
    for path, fields in (
        (graph, ("graphEdition", "serviceClass", "nodes", "edges", "boundaryNodes", "turnRestrictions", "retainedManifestSha256")),
        (municipal_file, ("reports",)),
        (works_file, ("permits",)),
        (population, ("population",)),
    ):
        if path is None:
            continue
        data = json.loads(path.read_bytes())
        for entry in data["sources"]:
            identity = entry["id"]
            require(identity not in sources or sources[identity] == entry, "source identity conflicts across fragments")
            sources[identity] = entry
        for key in fields:
            if key == "retainedManifestSha256" and key not in data:
                continue
            if key in {"reports", "permits"}:
                package[key] = package.get(key, []) + data[key]
                continue
            require(key not in package or not package[key], f"prepared fragment would replace {key}")
            package[key] = data[key]
    package["sources"] = list(sources.values())
    return package


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    osm = commands.add_parser("osm")
    osm.add_argument("prepared", type=Path)
    osm.add_argument("--manifest-sha256", required=True)
    overture = commands.add_parser("overture")
    overture.add_argument("segments", type=Path)
    overture.add_argument("connectors", type=Path)
    overture.add_argument("--edition", required=True)
    overture.add_argument("--osm-candidates", type=Path)
    graph = commands.add_parser("graph")
    graph.add_argument("candidates", type=Path)
    graph.add_argument("review", type=Path)
    reports = commands.add_parser("municipal")
    reports.add_argument("input", type=Path)
    reports.add_argument("schedule", type=Path)
    works = commands.add_parser("works")
    works.add_argument("input", type=Path)
    works.add_argument("--edition", required=True)
    works.add_argument("--source-id", required=True)
    works.add_argument("--attribution", required=True)
    population = commands.add_parser("population")
    population.add_argument("raster", type=Path)
    population.add_argument("zones", type=Path)
    population.add_argument("--year", type=int, required=True)
    population.add_argument("--edition", required=True)
    population.add_argument("--allocation", choices=["area-weighted", "cell-center"], default="area-weighted")
    bundle = commands.add_parser("assemble")
    bundle.add_argument("definition", type=Path)
    bundle.add_argument("--graph", required=True, type=Path)
    bundle.add_argument("--municipal-fragment", type=Path)
    bundle.add_argument("--works-fragment", type=Path)
    bundle.add_argument("--population-fragment", type=Path)
    args = parser.parse_args()
    handlers = {
        "osm": lambda: osm_candidates(args.prepared, args.manifest_sha256),
        "overture": lambda: overture_candidates(args.segments, args.connectors, args.edition, args.osm_candidates),
        "graph": lambda: reviewed_graph(args.candidates, args.review),
        "municipal": lambda: select_municipal(args.input, args.schedule),
        "works": lambda: planned_works(args.input, args.edition, args.source_id, args.attribution),
        "population": lambda: population_cells(args.raster, args.zones, args.year, args.edition, args.allocation),
        "assemble": lambda: assemble(args.definition, args.graph, args.municipal_fragment,
                                     args.works_fragment, args.population_fragment),
    }
    print(json.dumps(handlers[args.command](), ensure_ascii=True, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except (ImportError, OSError, ValueError, TypeError, KeyError) as err:
        sys.exit(f"Access input refused: {err}")
