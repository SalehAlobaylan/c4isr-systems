"""Build the redistributable fixture corpus used by the offline environment reader.

The fixture is owned synthetic data. Every expected answer in ``answers.json``
is derived from the construction constants in this file, not from the reader
under test, and the trusted digests live in a separate file. Regenerating the
fixture requires the pinned geospatial toolchain; verification does not.

Run with the geospatial toolchain environment:

    python scripts/data/build_owned_fixtures.py
"""
import argparse
import datetime as dt
import hashlib
import json
import math
from pathlib import Path
import shutil
import sys
import tempfile
import zipfile

import numpy as np
import rasterio
from rasterio.transform import from_origin
from shapely.geometry import MultiPolygon, Point, Polygon, box, mapping

sys.path.insert(0, str(Path(__file__).resolve().parent))
import environment_library as base

OUTPUT = Path(__file__).resolve().parents[2] / "testdata" / "geospatial" / "owned-fixture-v1"

# --- Construction constants (the independent derivation of every answer) ---
AREA_A = (46.700, 24.700, 46.760, 24.760)
HOLE_A = (46.720, 24.720, 46.730, 24.730)
AREA_B = (46.780, 24.700, 46.800, 24.720)
AREA = MultiPolygon([
    Polygon(box(*AREA_A).exterior.coords, [box(*HOLE_A).exterior.coords]),
    box(*AREA_B),
])

T1 = {"origin": (46.700, 24.760), "width": 30, "height": 60, "scale": 2.0, "offset": 100.0}
T2 = {"origin": (46.730, 24.760), "width": 30, "height": 60, "scale": 1.0, "offset": 0.0}
T3 = {"origin": (46.780, 24.720), "width": 20, "height": 20, "scale": 1.0, "offset": 0.0}
RESOLUTION = .001
NODATA = -9999

# (row, column, raw) overrides applied after the deterministic base field.
T1_OVERRIDES = {(5, 5): NODATA, (10, 10): -50, (20, 20): 25}
T2_OVERRIDES = {(0, 0): 7.0, (15, 5): 0.0}
T3_OVERRIDES = {(3, 4): NODATA, (10, 10): 0}

FIELD_LATITUDES = [25.5, 25.25, 25.0, 24.75, 24.5]
FIELD_LONGITUDES = [46.0, 46.25, 46.5, 46.75, 47.0]
FIELD_LEVELS = [850, 700]
FIELD_TIMES = ["2022-10-23T12:00:00Z", "2023-01-09T12:00:00Z"]


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()


def write(path, value):
    path.write_bytes(encoded(value) if not isinstance(value, bytes) else value)


def raw_value(tile, row, column):
    """Deterministic raw digital number before scale/offset."""
    overrides = {1: T1_OVERRIDES, 2: T2_OVERRIDES, 3: T3_OVERRIDES}[tile]
    if (row, column) in overrides:
        return overrides[(row, column)]
    if tile == 1:
        return (column + row) % 40 - 15
    if tile == 2:
        return float((row * 5 + column * 3) % 30 - 10)
    return (column * 10 + row) - 40


def scientific(tile, row, column):
    spec = {1: T1, 2: T2, 3: T3}[tile]
    raw = raw_value(tile, row, column)
    if raw == NODATA:
        return None
    return round(raw * spec["scale"] + spec["offset"], 9)


def raster(path, tile):
    spec = {1: T1, 2: T2, 3: T3}[tile]
    dtype = "int16"
    values = np.array([[raw_value(tile, r, c) for c in range(spec["width"])]
                       for r in range(spec["height"])], dtype=dtype)
    mask = np.ones((spec["height"], spec["width"]), dtype="uint8")
    mask[values == NODATA] = 0
    with rasterio.open(path, "w", driver="GTiff", width=spec["width"], height=spec["height"], count=1,
                       dtype=dtype, crs="EPSG:4326",
                       transform=from_origin(spec["origin"][0], spec["origin"][1], RESOLUTION, RESOLUTION),
                       nodata=NODATA) as target:
        target.write(values, 1)
        target.write_mask(mask)
        target.scales = (spec["scale"],)
        target.offsets = (spec["offset"],)
        target.units = ("metre",)
        target.update_tags(AREA_OR_POINT="Area")


def osm_roads():
    nodes = {
        1101: (24.705, 46.690), 1102: (24.705, 46.750),
        1201: (24.690, 46.740), 1202: (24.750, 46.740),
        1301: (24.745, 46.710), 1302: (24.745, 46.715), 1303: (24.748, 46.715),
        1401: (24.715, 46.755), 1402: (24.715, 46.790),
    }
    ways = [
        (1001, [1101, 1102], {"highway": "residential", "name": "Crossing Street", "width": "12 m", "lanes": "2"}),
        (1002, [1201, 1202], {"highway": "service", "width": "wide"}),
        (1003, [1301, 1302], {"highway": "footway"}),
        (1004, [1302, 1303], {"highway": "footway"}),
        (1005, [1401, 1402], {"highway": "primary"}),
    ]
    parts = ['<?xml version="1.0" encoding="UTF-8"?>', '<osm version="0.6" generator="owned-fixture">']
    parts += [f' <node id="{n}" lat="{lat}" lon="{lon}"/>' for n, (lat, lon) in nodes.items()]
    for identifier, refs, tags in ways:
        members = "".join(f'<nd ref="{ref}"/>' for ref in refs)
        attributes = "".join(f'<tag k="{k}" v="{v}"/>' for k, v in tags.items())
        parts.append(f' <way id="{identifier}">{members}{attributes}</way>')
    parts.append("</osm>")
    return "\n".join(parts)


def osm_buildings():
    rings = {
        2101: [(24.7420, 46.7420), (24.7420, 46.7480), (24.7480, 46.7480), (24.7480, 46.7420)],
        2102: [(24.7440, 46.7440), (24.7440, 46.7460), (24.7460, 46.7460), (24.7460, 46.7440)],
        2103: [(24.7420, 46.7500), (24.7420, 46.7520), (24.7440, 46.7520), (24.7440, 46.7500)],
        2201: [(24.7520, 46.7520), (24.7520, 46.7560), (24.7560, 46.7560), (24.7560, 46.7520)],
        2301: [(24.7320, 46.7340), (24.7320, 46.7360), (24.7340, 46.7360), (24.7340, 46.7340)],
    }
    parts = ['<?xml version="1.0" encoding="UTF-8"?>', '<osm version="0.6" generator="owned-fixture">']
    for way, ring in rings.items():
        for index, (lat, lon) in enumerate(ring):
            parts.append(f' <node id="{way * 10 + index}" lat="{lat}" lon="{lon}"/>')
    for way, ring in rings.items():
        refs = "".join(f'<nd ref="{way * 10 + index}"/>' for index in range(len(ring)))
        closing = f'<nd ref="{way * 10}"/>'
        tags = "" if way != 2301 else '<tag k="building" v="yes"/>'
        parts.append(f' <way id="{way}">{refs}{closing}{tags}</way>')
    parts.append(' <relation id="2001">'
                 '<member type="way" ref="2101" role="outer"/>'
                 '<member type="way" ref="2102" role="inner"/>'
                 '<member type="way" ref="2103" role="outer"/>'
                 '<tag k="type" v="multipolygon"/><tag k="building" v="yes"/>'
                 '<tag k="height" v="12 m"/><tag k="building:levels" v="4"/></relation>')
    parts.append(' <relation id="2002">'
                 '<member type="way" ref="2201" role="outer"/>'
                 '<tag k="type" v="multipolygon"/><tag k="building" v="yes"/>'
                 '<tag k="height" v="tall"/><tag k="building:levels" v="lots"/></relation>')
    parts.append("</osm>")
    return "\n".join(parts)


def modeled_centroids():
    return {"type": "FeatureCollection", "features": [
        {"type": "Feature", "properties": {"height": "18.5"},
         "geometry": mapping(Point(46.7435, 24.7435))},
        {"type": "Feature", "properties": {"height": "NaN"},
         "geometry": mapping(Point(46.7535, 24.7535))},
    ]}


def topology():
    return {
        "schema": "emad.environment-topology.v1",
        "artifact": "owned-fixture-topology",
        "sourceEdition": "owned-fixture-osm-2026-11-18",
        "recipes": {"source": "osmium-node-membership-v1", "clip": "area-intersection-v1"},
        "ways": {
            "osm-way-1001": {"nodes": [1101, 1102], "component": 0, "clipped": True},
            "osm-way-1002": {"nodes": [1201, 1202], "component": 0, "clipped": True},
            "osm-way-1003": {"nodes": [1301, 1302], "component": 0, "clipped": False},
            "osm-way-1004": {"nodes": [1302, 1303], "component": 0, "clipped": False},
            "osm-way-1005": {"nodes": [1401, 1402], "components": [0, 1], "clipped": True},
        },
        "relations": {
            "osm-relation-2001": {"members": [
                {"type": "way", "ref": 2101, "role": "outer"},
                {"type": "way", "ref": 2102, "role": "inner"},
                {"type": "way", "ref": 2103, "role": "outer"}], "clipped": False},
            "osm-relation-2002": {"members": [
                {"type": "way", "ref": 2201, "role": "outer"}], "clipped": False},
        },
        "sharedNodes": [{"nodeId": 1302, "ways": ["osm-way-1003", "osm-way-1004"],
                         "longitude": 46.715, "latitude": 24.745}],
        "crossings": [{"ways": ["osm-way-1001", "osm-way-1002"], "longitude": 46.740,
                       "latitude": 24.705, "reason": "crossing-without-shared-node"}],
        "omittedRelationships": ["tagged outer-way membership is reported by the relation entry",
                                 "network routing beyond the retained ways"],
        "unknownForEarlierReleases": ["component-boundary cuts", "node ordering within clipped parts"],
    }


def field_document():
    """Seven-by-five native ERA5-shaped pilot field with a below-ground cell."""
    times, levels = FIELD_TIMES, FIELD_LEVELS
    lat, lon = FIELD_LATITUDES, FIELD_LONGITUDES
    shape = (len(times), len(levels), len(lat), len(lon))
    mask = np.ones(shape, dtype=bool)
    # One below-ground cell in the first hour at 700 hPa over 25.0 N, 46.5 E.
    mask[0, 1, 2, 2] = False

    def samples(kind):
        values = []
        for t in range(shape[0]):
            for p in range(shape[1]):
                for y in range(shape[2]):
                    for x in range(shape[3]):
                        if not mask[t, p, y, x]:
                            values.append(None)
                        else:
                            values.append(kind(t, p, y, x))
        return values

    def temperature_value(t, p, y, x):
        if (t, p, y, x) == (1, 0, 0, 4):
            # An above-ground sample the source edition does not retain.
            return None
        return round(295.15 - 6.5 * p - .5 * y + 2500 * (x - 2) / 1000, 6)

    temperature = samples(temperature_value)
    # A measured zero in a wind component must survive serialization.
    u = samples(lambda t, p, y, x: 0.0 if (t, p, y, x) == (0, 0, 1, 2) else round((x - 2) * .8 + (2 - y) * .4, 6))
    v = samples(lambda t, p, y, x: round((p - 1) * .6 + (y - 2) * .25, 6))
    epoch = dt.datetime(1959, 1, 1, tzinfo=dt.timezone.utc)
    receipts = {}
    for key in (".zmetadata", "latitude/0", "longitude/0", "level/0"):
        receipts[key] = hashlib.sha256(("owned-fixture-era5:" + key).encode()).hexdigest()
    for value in times:
        hour = int((dt.datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=dt.timezone.utc)
                    - epoch).total_seconds() // 3600)
        receipts[f"time/{hour // 70158}"] = hashlib.sha256(f"owned-fixture-era5:time/{hour}".encode()).hexdigest()
        receipts[f"surface_pressure/{hour}.0.0"] = hashlib.sha256(
            f"owned-fixture-era5:surface/{hour}".encode()).hexdigest()
        for name in ("temperature", "u_component_of_wind", "v_component_of_wind"):
            receipts[f"{name}/{hour}.0.0.0"] = hashlib.sha256(
                f"owned-fixture-era5:{name}/{hour}".encode()).hexdigest()
    return {
        "schema": "emad.era5-pressure-subset.v1",
        "edition": "WeatherBench 2 ERA5 1959-2023_01_10-full_37-1h-0p25deg-chunk-1",
        "crs": "EPSG:4326", "verticalCoordinate": "pressure-hPa", "nativeSpacingDegrees": .25,
        "physicalAccuracy": "UNKNOWN", "pressureHPa": levels, "validTimes": times,
        "latitudes": lat, "longitudes": lon, "aboveGroundMask": mask.ravel().tolist(),
        "variables": {
            "temperature": {"units": "K", "samples": temperature},
            "u_component_of_wind": {"units": "m s**-1", "samples": u},
            "v_component_of_wind": {"units": "m s**-1", "samples": v},
        },
        "sourceObjects": [{"key": key, "sha256": digest,
                           "bytes": 1024 + index} for index, (key, digest) in enumerate(sorted(receipts.items()))],
        "temporalMeaning": {
            "kind": "modeled-value-valid-at-instant",
            "declaredTimes": {"observation": None, "issue": None, "valid": times,
                              "platformAvailability": None},
            "meaning": "ERA5 reanalysis value valid at the declared UTC instant. It is not an observation known by a cutoff.",
        },
    }


def build_inputs(root):
    inputs = root / "inputs"
    inputs.mkdir()
    assets = []

    def add(layer, fmt, path, vertical_datum="ellipsoid", evidence="source receipt"):
        digest, size = base.file_digest(path)
        identifier = "geoasset_" + hashlib.sha256(("%s:%s:%s" % (layer, fmt, digest)).encode()).hexdigest()
        target = inputs / identifier
        shutil.copyfile(path, target)
        assets.append({"id": identifier, "sha256": digest, "bytes": size,
                       "representation": {"format": fmt, "layer": layer, "crs": "EPSG:4326",
                                          "verticalDatum": vertical_datum, "evidence": evidence}})

    for tile, spec in ((1, T1), (2, T2), (3, T3)):
        path = root / f"surface-{tile}.tif"
        raster(path, tile)
        add("surface", "geotiff", path)

    roads = root / "roads.osm"
    roads.write_text(osm_roads(), encoding="utf-8")
    add("roads", "osm-xml", roads)

    buildings = root / "buildings.osm"
    buildings.write_text(osm_buildings(), encoding="utf-8")
    add("buildings", "osm-xml", buildings)

    centroids = root / "centroids.geojson"
    centroids.write_text(json.dumps(modeled_centroids()), encoding="utf-8")
    add("modeled-building-context", "geojson", centroids, vertical_datum="above-ground")

    recipe = {"schema": "emad.environment-recipe.v1", "toolchainSha256": base.toolchain(),
              "area": {"revisionId": "owned-fixture-area-v1", "geometry": mapping(AREA)},
              "profile": {"id": "reference-surface-v1"}, "inputs": assets}
    recipe["inputSha256"] = hashlib.sha256(encoded(recipe)).hexdigest()
    (root / "recipe.json").write_bytes(encoded(recipe))
    return recipe


def answers(manifest):
    t1_scale, t1_offset = T1["scale"], T1["offset"]
    t2_scale, t2_offset = T2["scale"], T2["offset"]
    t3_scale, t3_offset = T3["scale"], T3["offset"]
    members = {m["layer"]: [] for m in manifest["members"]}
    for m in manifest["members"]:
        members.setdefault(m["layer"], []).append(m)
    surface = members["surface"]
    return {
        "schema": "emad.environment-known-answers.v1",
        "fixture": "owned-fixture-v1",
        "rights": "Owned synthetic data created for this repository. Redistributable without provider terms.",
        "derivation": {
            "area": "Axis-aligned multipolygon A (46.700-46.760 E, 24.700-24.760 N) with a hole "
                    "(46.720-46.730 E, 24.720-24.730 N) plus a second component B "
                    "(46.780-46.800 E, 24.700-24.720 N).",
            "raster": "North-up 0.001 degree grid. Cell (row, column) covers "
                      "[west + column*0.001, west + (column+1)*0.001) and "
                      "(north - (row+1)*0.001, north - row*0.001]. Scientific value = raw*scale + offset.",
            "vectors": "OSM XML ways retain source node ids. The reader joins the separately published "
                       "topology artifact for node membership. Nothing is inferred at read time.",
            "field": "ERA5-shaped pressure-level subset with descending latitude, two UTC valid times, "
                     "two pressure levels and one below-ground cell.",
            "frame": "Frame controls are computed by the preparation frame check; the reader must reproduce "
                     "each control within 1e-6 m using pyproj.",
        },
        "samples": [
            {"id": "valid-zero", "member": surface[0]["name"], "row": 10, "column": 10,
             "raw": -50, "scale": t1_scale, "offset": t1_offset, "value": 0.0, "unit": "metre",
             "outcome": "value"},
            {"id": "nonzero", "member": surface[0]["name"], "row": 20, "column": 20, "raw": 25,
             "scale": t1_scale, "offset": t1_offset, "value": 150.0, "unit": "metre", "outcome": "value"},
            {"id": "masked", "member": surface[0]["name"], "row": 5, "column": 5,
             "raw": NODATA, "scale": t1_scale, "offset": t1_offset, "value": None,
             "unit": "metre", "outcome": "masked"},
            {"id": "second-zero", "member": surface[2]["name"], "row": 10, "column": 10, "raw": 0,
             "scale": t3_scale, "offset": t3_offset, "value": 0.0, "unit": "metre", "outcome": "value"},
            {"id": "second-origin-masked", "member": surface[2]["name"], "row": 3, "column": 4,
             "raw": NODATA, "scale": t3_scale, "offset": t3_offset, "value": None,
             "unit": "metre", "outcome": "masked"},
        ],
        "points": [
            {"id": "native-zero", "layer": "surface", "longitude": 46.7105, "latitude": 24.7495,
             "value": 0.0, "unit": "metre", "outcome": "value", "component": 0,
             "datum": "ellipsoid", "member": surface[0]["name"]},
            {"id": "nonzero", "layer": "surface", "longitude": 46.7205, "latitude": 24.7395,
             "value": 150.0, "unit": "metre", "outcome": "value", "component": 0,
             "datum": "ellipsoid", "member": surface[0]["name"]},
            {"id": "masked", "layer": "surface", "longitude": 46.7055, "latitude": 24.7545,
             "outcome": "masked", "component": 0, "member": surface[0]["name"]},
            {"id": "hole", "layer": "surface", "longitude": 46.725, "latitude": 24.725,
             "outcome": "outside-area", "component": 0},
            {"id": "seam-east", "layer": "surface", "longitude": 46.730, "latitude": 24.7055,
             "value": -10.0, "unit": "metre", "outcome": "value", "component": 0,
             "datum": "ellipsoid", "member": surface[1]["name"]},
            {"id": "second-component", "layer": "surface", "longitude": 46.7905, "latitude": 24.7095,
             "value": 0.0, "unit": "metre", "outcome": "value", "component": 1,
             "datum": "ellipsoid", "member": surface[2]["name"]},
            {"id": "between-components", "layer": "surface", "longitude": 46.765, "latitude": 24.7005,
             "outcome": "outside-area"},
        ],
        "conversions": [
            {"id": "foot", "point": "nonzero", "unit": "foot",
             "value": 150.0 / 0.3048, "method": "linear-unit-conversion"},
            {"id": "missing-geoid", "point": "nonzero", "datum": "EGM2008",
             "outcome": "unsupported-conversion", "reason": "missing-geoid-grid"},
        ],
        "features": [
            {"id": "crossing-without-junction", "layer": "roads", "sourceId": "osm-way-1001",
             "partCount": 1, "geometryType": "LineString",
             "start": [46.700, 24.705], "end": [46.750, 24.705],
             "topology": {"nodes": [1101, 1102], "clipped": True,
                          "crossingsWith": ["osm-way-1002"]},
             "dimensions": {"width": {"value": 12.0, "unit": "m", "origin": "provider-stated"},
                            "lanes": {"value": 2, "unit": "count", "origin": "provider-stated"}}},
            {"id": "shared-node", "layer": "roads", "sourceId": "osm-way-1004",
             "partCount": 1, "geometryType": "LineString",
             "start": [46.715, 24.745], "end": [46.715, 24.748],
             "topology": {"nodes": [1302, 1303], "clipped": False,
                          "sharedJunctions": [{"nodeId": 1302, "with": "osm-way-1003"}]},
             "dimensions": {}},
            {"id": "clipped-line", "layer": "roads", "sourceId": "osm-way-1005",
             "partCount": 1, "geometryType": "MultiLineString", "pieceCount": 2,
             "bounds": [46.755, 24.715, 46.790, 24.715],
             "topology": {"nodes": [1401, 1402], "clipped": True, "components": [0, 1]}},
            {"id": "multipart-with-hole", "layer": "buildings", "sourceId": "osm-relation-2001",
             "partCount": 1, "geometryType": "MultiPolygon", "polygonParts": 2, "holes": 1,
             "dimensions": {"height": {"value": 12.0, "unit": "m", "origin": "provider-stated"},
                            "levels": {"value": 4, "unit": "count", "origin": "provider-stated"}}},
            {"id": "unsupported-dimension", "layer": "buildings", "sourceId": "osm-relation-2002",
             "partCount": 1, "geometryType": "Polygon",
             "dimensions": {"height": {"value": None, "origin": "unknown",
                                       "evidence": "unsupported source tag: height"},
                            "levels": {"value": None, "origin": "unknown",
                                       "evidence": "unsupported source tag: building:levels"}}},
            {"id": "missing-dimension", "layer": "buildings", "sourceId": "osm-way-2301",
             "partCount": 1, "geometryType": "Polygon",
             "dimensions": {"height": {"value": None, "origin": "unknown",
                                       "evidence": "missing source tag: height"}}},
            {"id": "modeled-centroid", "layer": "modeled-building-context", "sourceId": "0",
             "partCount": 1, "geometryType": "Point",
             "modeledHeight": {"value": 18.5, "unit": "m", "origin": "inferred",
                               "uncertainty": "UNKNOWN"}},
        ],
        "field": [
            {"id": "measured-zero", "variable": "u_component_of_wind",
             "validTime": FIELD_TIMES[0], "levelHPa": 850, "latitude": 25.25, "longitude": 46.5,
             "value": 0.0, "unit": "m s**-1", "outcome": "value"},
            {"id": "below-ground-mask", "variable": "temperature", "validTime": FIELD_TIMES[0],
             "levelHPa": 700, "latitude": 25.0, "longitude": 46.5,
             "outcome": "masked", "reason": "below-ground"},
            {"id": "reversed-latitude", "variable": "temperature", "validTime": FIELD_TIMES[0],
             "levelHPa": 850, "latitude": 24.75, "longitude": 46.75,
             "value": round(295.15 - 6.5 * 0 - .5 * 3 + 2500 * (3 - 2) / 1000, 6),
             "unit": "K", "outcome": "value"},
            {"id": "unavailable-hour", "variable": "temperature", "validTime": "2022-10-23T13:00:00Z",
             "levelHPa": 850, "latitude": 25.25, "longitude": 46.5,
             "outcome": "unavailable-hour"},
            {"id": "metres-against-pressure", "variable": "temperature", "validTime": FIELD_TIMES[0],
             "levelHPa": 3000, "latitude": 25.25, "longitude": 46.5,
             "outcome": "unsupported-vertical-coordinate", "reason": "pressure-axis"},
            {"id": "missing-sample", "variable": "temperature", "validTime": FIELD_TIMES[1],
             "levelHPa": 850, "latitude": 25.5, "longitude": 47.0,
             "outcome": "masked", "reason": "missing-sample"},
            {"id": "mixed-year-second", "variable": "temperature", "validTime": FIELD_TIMES[1],
             "levelHPa": 850, "latitude": 25.5, "longitude": 46.0,
             "value": round(295.15 - 6.5 * 0 - .5 * 0 + 2500 * (0 - 2) / 1000, 6),
             "unit": "K", "outcome": "value"},
        ],
        "frame": [{"control": index, "enu": control["enuM"]}
                  for index, control in enumerate(manifest["frame"]["controls"])],
        "members": [{"name": m["name"], "layer": m["layer"], "component": m["component"],
                     "sha256": m["sha256"], "bytes": m["bytes"]} for m in manifest["members"]],
    }


def display_descriptor(manifest):
    surface = [m for m in manifest["members"] if m["layer"] == "surface"]
    return {
        "schema": "emad.geographic-derivative.v1",
        "displayId": "geodisplay_ownedfixtureterrain",
        "releaseManifestSha256": hashlib.sha256(encoded(manifest)).hexdigest(),
        "purpose": "visual-reference",
        "purposeMeaning": "Visual terrain orientation only. Not a survey, measurement or bare-earth product.",
        "representation": "derived-display",
        "recipe": "terrain-glb-v2",
        "toolchainSha256": "d" * 64,
        "units": "metre",
        "axes": ["east", "up", "south"],
        "localOriginEnu": [0, 0, 0],
        "boundsLonLat": [AREA.bounds[0], AREA.bounds[1], AREA.bounds[2], AREA.bounds[3]],
        "parentMembers": [{"name": m["name"], "sha256": m["sha256"]} for m in surface],
        "renderingAdjustments": [{"kind": "vertex-height-offset", "value": .6, "unit": "metre",
                                  "meaning": "Documented display drape offset; never applied to native reads."}],
        "limitations": ["visual-orientation-only", "no-physical-accuracy-claim",
                        "display-vertices-are-not-native-samples"],
        "computationalUse": False,
        "featureMapping": "unknown",
    }


def build(output):
    if output.exists():
        shutil.rmtree(output)
    output.mkdir(parents=True)
    with tempfile.TemporaryDirectory(prefix="emad-owned-fixture-") as location:
        root = Path(location)
        build_inputs(root)
        manifest = base.prepare(root)
        result = base.verify(root / "prepared.zip")
        if result["status"] != "VERIFIED_PACKAGE":
            raise SystemExit("The owned fixture did not verify")
        shutil.copyfile(root / "prepared.zip", output / "prepared.zip")
        shutil.copyfile(root / "manifest.json", output / "manifest.json")
        answers_value = answers(manifest)
        write(output / "answers.json", answers_value)
        write(output / "topology.json", topology())
        write(output / "field-era5.json", field_document())
        write(output / "display-descriptor.json", display_descriptor(manifest))
        digests = {
            "schema": "emad.environment-fixture-digests.v1",
            "fixture": "owned-fixture-v1",
            "package": "prepared.zip",
            "preparedSha256": base.file_digest(output / "prepared.zip")[0],
            "preparedBytes": (output / "prepared.zip").stat().st_size,
            "manifestSha256": hashlib.sha256(encoded(manifest)).hexdigest(),
            "manifestBytes": (output / "manifest.json").stat().st_size,
            "answersSha256": base.file_digest(output / "answers.json")[0],
            "topologySha256": base.file_digest(output / "topology.json")[0],
            "fieldSha256": base.file_digest(output / "field-era5.json")[0],
            "displayDescriptorSha256": base.file_digest(output / "display-descriptor.json")[0],
            "toolchainSha256": manifest["toolchainSha256"],
        }
        write(output / "digests.json", digests)
        (output / "README.md").write_text(readme(digests), encoding="utf-8")
    return manifest


def readme(digests):
    return f"""# Owned environment fixture v1

This directory is owned synthetic data created for the simulation-preparation
reader. It is redistributable without provider terms. No provider product bytes
are present.

- `prepared.zip` - `{digests['preparedSha256']}` ({digests['preparedBytes']} bytes)
- `manifest.json` - `{digests['manifestSha256']}`
- `answers.json` - expected answers, derived from construction constants
- `topology.json` - separately versioned source-relationship companion artifact
- `field-era5.json` - ERA5-shaped pressure-level pilot field (synthetic values)
- `display-descriptor.json` - purpose-specific visual derivative metadata

Regenerate with `python scripts/data/build_owned_fixtures.py`. Verification never
regenerates the fixture: the reader only reads the committed bytes and compares
answers and digests.

## Derivation

The area is an axis-aligned multipolygon with a hole and a second component. The
surface grid is north-up at 0.001 degrees with a declared scale and offset. The
answers are computed from those constants, not from the reader: for a cell
`(row, column)` the west edge is `west + column*0.001`, the north edge is
`north - row*0.001`, and the scientific value is `raw*scale + offset`.

OSM ways preserve their source node identifiers. Crossing ways that do not share
a node remain distinct in `topology.json`, and two ways sharing node 1302 are
recorded as a junction. Clipped ways and multipart relation footprints are
recorded with the clipping recipe.
"""


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=OUTPUT)
    args = parser.parse_args()
    manifest = build(args.output)
    print(json.dumps({"members": len(manifest["members"]), "output": str(args.output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
