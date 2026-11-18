"""Offline reader for a prepared geographic environment package.

The reader opens the released ZIP directly. It never rebuilds a package, never
contacts a provider and never depends on a viewer. The same request/result
contract is used by the CLI, the kit verification step and the installed
workflow's native-query worker.

Outcomes are explicit. A measured zero is the number ``0.0`` (never ``None`` or
an omitted field). A masked, outside-area, unavailable, unsupported-conversion
or conflicting-overlap answer is a documented outcome with a reason code.

Edge convention
---------------
A north-up raster cell ``(row, column)`` covers
``[west + column*a, west + (column+1)*a)`` in x and
``(north - (row+1)*|e|, north - row*|e|]`` in y. A coordinate on a shared
tile boundary therefore resolves to the cell whose west or south edge contains
it: the eastern or southern neighbour. Registration ``Point`` shifts the index
by half a cell so grid nodes own their surrounding half-cell.
"""
import argparse
import datetime as dt
import hashlib
import json
import math
from pathlib import Path
import re
import sys
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
import environment_library as base
import environment_suitability as suitability
from environment_library import HASH, MEMBER, archive_entries, archive_json, hash_stream

SCHEMA = "emad.environment-reader.v1"
UNIT_FACTOR = {"metre": 1.0, "meter": 1.0, "m": 1.0, "foot": 1 / .3048, "feet": 1 / .3048, "ft": 1 / .3048}
DATUMS = {"ellipsoid", "EGM2008"}


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


def member_path(package, name):
    return "/vsizip/" + Path(package).resolve().as_posix() + "/" + name


def _point_in_area(area, longitude, latitude):
    from shapely.geometry import Point, shape
    return shape(area).covers(Point(longitude, latitude))


class Reader:
    """A verified, offline view of one prepared environment archive."""

    def __init__(self, package, trusted_manifest=None, topology=None):
        self.package = Path(package)
        self.topology = Path(topology) if topology else None
        with zipfile.ZipFile(self.package) as archive:
            self.entries = archive_entries(archive)
            raw, self.manifest = archive_json(archive, self.entries, "manifest.json")
        self.manifest_raw = raw
        self.manifest_sha = hashlib.sha256(raw).hexdigest()
        if trusted_manifest and self.manifest_sha != trusted_manifest:
            raise ValueError("Prepared manifest differs from its trusted SHA-256")
        self.area = self.manifest["area"]

    # -- inventory -------------------------------------------------------
    def native_members(self, layer=None):
        return [item for item in self.manifest["members"]
                if item["name"].endswith(".tif") and (layer is None or item["layer"] == layer)]

    def vector_members(self, layer=None):
        return [item for item in self.manifest["members"]
                if item["name"].endswith(".gpkg") and (layer is None or item["layer"] == layer)]

    def member(self, name):
        for item in self.manifest["members"]:
            if item["name"] == name:
                return item
        raise ValueError(f"Unknown package member: {name}")

    def grid(self, name):
        import rasterio
        item = self.member(name)
        if not name.endswith(".tif"):
            raise ValueError("Native grid is available for raster members only")
        with rasterio.open(member_path(self.package, name)) as raster:
            return {"member": name, "layer": item["layer"], "component": item["component"],
                    "crs": raster.crs.to_string() if raster.crs else None,
                    "transform": list(raster.transform)[:6],
                    "width": raster.width, "height": raster.height, "bands": raster.count,
                    "dtype": raster.dtypes[0], "nodata": raster.nodata,
                    "scales": list(raster.scales), "offsets": list(raster.offsets),
                    "units": list(raster.units),
                    "registration": (item.get("details") or {}).get("registration",
                                                                     raster.tags().get("AREA_OR_POINT", "Area")),
                    "sha256": item["sha256"], "bytes": item["bytes"]}

    def inventory(self):
        members = []
        for item in self.manifest["members"]:
            record = {"name": item["name"], "layer": item["layer"], "component": item["component"],
                      "sha256": item["sha256"], "bytes": item["bytes"],
                      "inputId": item.get("inputId")}
            if item["name"].endswith(".tif"):
                record["native"] = self.grid(item["name"])
                details = item.get("details") or {}
                record["evidence"] = {key: details.get(key) for key in
                                      ("validSamples", "selectedSamples", "maskMeaning",
                                       "verticalDatum", "registration", "resampling")}
            elif item["name"].endswith(".gpkg"):
                details = item.get("details") or {}
                feature = self.vector_members(item["layer"])
                record["evidence"] = {"featureCount": details.get("featureCount"),
                                      "omissions": details.get("omissions"),
                                      "crs": details.get("crs"),
                                      "positionalAccuracy": details.get("positionalAccuracy"),
                                      "heightMeaning": details.get("heightMeaning"),
                                      "logicalSha256": details.get("logicalSha256"),
                                      "membersForLayer": [m["name"] for m in feature]}
            members.append(record)
        return {"schema": "emad.environment-inventory.v1", "manifestSha256": self.manifest_sha,
                "profileId": self.manifest["profileId"], "areaRevisionId": self.manifest["areaRevisionId"],
                "area": self.area, "frame": self.manifest["frame"], "quality": self.manifest["quality"],
                "toolchainSha256": self.manifest["toolchainSha256"], "members": members,
                "layers": sorted({item["layer"] for item in self.manifest["members"]})}

    # -- native reads ----------------------------------------------------
    def _read_cell(self, item, row, column):
        import rasterio
        from rasterio.windows import Window
        with rasterio.open(member_path(self.package, item["name"])) as raster:
            if not (0 <= row < raster.height and 0 <= column < raster.width):
                return {"outcome": "outside-member", "member": item["name"],
                        "nativeIndex": {"row": row, "column": column}}
            values = raster.read(1, window=Window(column, row, 1, 1), masked=False)
            masks = raster.read_masks(1, window=Window(column, row, 1, 1))
            raw = values[0, 0].item()
            valid = bool(masks[0, 0]) and math.isfinite(float(raw))
            scale = float(raster.scales[0]) if raster.scales else 1.0
            offset = float(raster.offsets[0]) if raster.offsets else 0.0
            unit = raster.units[0] if raster.units and raster.units[0] else "metre"
            details = item.get("details") or {}
            return {"outcome": "value" if valid else "masked", "member": item["name"],
                    "layer": item["layer"], "component": item["component"],
                    "nativeIndex": {"row": row, "column": column}, "raw": raw,
                    "scale": scale, "offset": offset,
                    "value": raw * scale + offset if valid else None, "unit": unit,
                    "verticalDatum": details.get("verticalDatum"),
                    "registration": details.get("registration"),
                    "lineage": self._lineage(item)}

    def _lineage(self, item):
        return {"schema": SCHEMA, "manifestSha256": self.manifest_sha, "toolchainSha256": self.manifest["toolchainSha256"],
                "member": item["name"], "memberSha256": item["sha256"], "layer": item["layer"],
                "component": item["component"], "areaRevisionId": self.manifest["areaRevisionId"]}

    def _convert_unit(self, value, unit):
        if unit in (None, "metre", "meter", "m"):
            return value, 1.0
        factor = UNIT_FACTOR.get(unit)
        if factor is None:
            return None, None
        return value * factor, factor

    @staticmethod
    def _snap_fraction(fraction):
        """Snap a cell fraction that is an integer apart from floating noise.

        A coordinate written as ``46.73`` must land on the tile boundary rather
        than one cell west of it because ``46.73 - 46.7`` rounds down. The
        documented edge convention then selects the eastern or southern cell.
        """
        nearest = round(fraction)
        return float(nearest) if abs(fraction - nearest) <= 1e-9 else fraction

    def _cell_index(self, item, longitude, latitude):
        grid = item.get("_grid") or self.grid(item["name"])
        a, b, c, d, e, f = grid["transform"]
        if b or d or a <= 0 or e >= 0:
            raise ValueError("Unsupported rotated or south-up native grid")
        fraction_x = self._snap_fraction((longitude - c) / a)
        fraction_y = self._snap_fraction((f - latitude) / (-e))
        registration = grid["registration"]
        if registration == "Point":
            fraction_x += .5
            fraction_y += .5
        elif registration != "Area":
            raise ValueError("Unsupported raster registration")
        return math.floor(fraction_y), math.floor(fraction_x), grid

    def _geoid_undulation(self, longitude, latitude):
        import rasterio
        from rasterio.windows import Window
        candidates = [item for item in self.manifest["members"] if item["layer"] == "geoid"]
        for item in candidates:
            with rasterio.open(member_path(self.package, item["name"])) as raster:
                column, row = (~raster.transform) * (longitude, latitude)
                column -= .5
                row -= .5
                c, r = math.floor(column), math.floor(row)
                dx, dy = column - c, row - r
                window = Window(c, r, 2, 2)
                values = raster.read(1, window=window, masked=True)
                if values.shape != (2, 2) or values.mask.any():
                    continue
                a, b, cc, d = map(float, values.ravel())
                undulation = ((1 - dy) * ((1 - dx) * a + dx * b) + dy * ((1 - dx) * cc + dx * d))
                scale = float(raster.scales[0]) if raster.scales else 1.0
                offset = float(raster.offsets[0]) if raster.offsets else 0.0
                return {"outcome": "value", "undulation": undulation * scale + offset,
                        "member": item["name"], "method": "EGM2008-bilinear-grid-undulation",
                        "memberSha256": item["sha256"]}
        return {"outcome": "unsupported-conversion", "reason": "missing-geoid-grid",
                "detail": "No retained geoid member covers the requested coordinate."}

    def convert_height(self, value, from_datum, to_datum, longitude, latitude):
        if not from_datum or not to_datum or from_datum == to_datum:
            return {"outcome": "value", "value": value, "from": from_datum, "to": to_datum,
                    "method": "native-value"}
        if from_datum not in DATUMS or to_datum not in DATUMS:
            return {"outcome": "unsupported-conversion", "reason": "unsupported-datum-pair",
                    "from": from_datum, "to": to_datum}
        geoid = self._geoid_undulation(longitude, latitude)
        if geoid["outcome"] != "value":
            return geoid
        undulation = geoid["undulation"]
        if from_datum == "ellipsoid" and to_datum == "EGM2008":
            converted = value - undulation
            method = "EGM2008 orthometric H = ellipsoidal h - bilinear undulation N"
        else:
            converted = value + undulation
            method = "EGM2008 ellipsoidal h = orthometric H + bilinear undulation N"
        return {"outcome": "value", "value": converted, "undulation": undulation,
                "from": from_datum, "to": to_datum, "method": method, "geoid": geoid["member"]}

    def sample(self, name, row, column, unit="metre"):
        item = self.member(name)
        if not name.endswith(".tif"):
            raise ValueError("sample requires a raster member")
        result = self._read_cell(item, int(row), int(column))
        if result["outcome"] != "value":
            return result
        converted, factor = self._convert_unit(result["value"], unit)
        if converted is None:
            return {"outcome": "unsupported-unit", "unit": unit, "value": result["value"],
                    "lineage": result["lineage"]}
        result.update(value=converted, unit=unit if unit else result["unit"], conversionFactor=factor,
                      method="native-cell")
        return result

    def point(self, layer, longitude, latitude, unit="metre", datum=None, component=None):
        if not _point_in_area(self.area, longitude, latitude):
            return {"outcome": "outside-area", "layer": layer,
                    "longitude": longitude, "latitude": latitude,
                    "detail": "The coordinate is outside the released area, inside a hole or between components."}
        layer_members = [item for item in self.native_members(layer)
                         if component is None or item["component"] == component]
        if not layer_members:
            return {"outcome": "unsupported-layer", "reason": "representation-absent", "layer": layer,
                    "detail": f"No native {layer} member is retained."}
        hits, masked = [], []
        for item in layer_members:
            row, column, grid = self._cell_index(item, longitude, latitude)
            if not (0 <= row < grid["height"] and 0 <= column < grid["width"]):
                continue
            cell = self._read_cell(item, row, column)
            if cell["outcome"] == "value":
                hits.append(cell)
            else:
                masked.append(cell)
        if not hits:
            if masked:
                result = {"outcome": "masked", "layer": layer,
                          "longitude": longitude, "latitude": latitude,
                          "members": [item["member"] for item in masked],
                          "lineage": masked[0]["lineage"]}
                return result
            return {"outcome": "uncovered", "reason": "no-native-member-covers-coordinate",
                    "layer": layer, "longitude": longitude, "latitude": latitude}
        values = [hit["value"] for hit in hits]
        if any(not math.isclose(value, values[0], rel_tol=0, abs_tol=1e-9) for value in values[1:]):
            return {"outcome": "conflicting-overlap", "layer": layer,
                    "longitude": longitude, "latitude": latitude,
                    "candidates": [{"member": hit["member"], "value": hit["value"],
                                    "verticalDatum": hit["verticalDatum"]} for hit in hits],
                    "detail": "Overlapping native members disagree; no value is chosen."}
        chosen = hits[0]
        datum_result = {"outcome": "value", "value": chosen["value"], "from": chosen["verticalDatum"],
                        "to": datum or chosen["verticalDatum"], "method": "native-value"}
        if datum and datum != chosen["verticalDatum"]:
            datum_result = self.convert_height(chosen["value"], chosen["verticalDatum"], datum,
                                               longitude, latitude)
            if datum_result["outcome"] != "value":
                return {"outcome": "unsupported-conversion", "layer": layer,
                        "longitude": longitude, "latitude": latitude,
                        "reason": datum_result["reason"], "from": chosen["verticalDatum"], "to": datum,
                        "detail": datum_result.get("detail")}
        converted, factor = self._convert_unit(datum_result["value"], unit)
        if converted is None:
            return {"outcome": "unsupported-unit", "unit": unit, "value": datum_result["value"]}
        return {"outcome": "value", "layer": layer, "longitude": longitude, "latitude": latitude,
                "value": converted, "unit": unit or chosen["unit"],
                "nativeValue": chosen["value"], "nativeUnit": chosen["unit"],
                "verticalDatum": datum or chosen["verticalDatum"],
                "method": datum_result["method"], "conversionFactor": factor,
                "component": chosen["component"],
                "member": chosen["member"], "members": [hit["member"] for hit in hits],
                "lineage": chosen["lineage"],
                "nativeIndex": chosen["nativeIndex"]}

    # -- features --------------------------------------------------------
    def _feature_dimensions(self, layer, tags):
        from environment_enrichment import BUILDING_CONTEXT, DIMENSIONS, dimensions, modeled_value
        if layer == BUILDING_CONTEXT:
            return {"modeledHeight": modeled_value(tags)}
        if layer in DIMENSIONS:
            return dimensions(layer, tags)
        return {}

    def _topology_for(self, layer, source_id):
        if not self.topology:
            return {"state": "unknown", "reason": "no-topology-artifact",
                    "detail": "Source node membership was not retained for this package."}
        artifact = json.loads(self.topology.read_text(encoding="utf-8"))
        if artifact.get("schema") != "emad.environment-topology.v1":
            raise ValueError("Unsupported topology artifact schema")
        result = {"schema": artifact["schema"], "artifact": artifact.get("artifact"),
                  "sourceEdition": artifact.get("sourceEdition")}
        if source_id in artifact.get("ways", {}):
            record = dict(artifact["ways"][source_id])
            nodes = record.get("nodes", [])
            record["sharedJunctions"] = [
                {"nodeId": shared["nodeId"],
                 "with": [way for way in shared["ways"] if way != source_id]}
                for shared in artifact.get("sharedNodes", []) if source_id in shared["ways"]]
            record["crossingsWith"] = sorted(
                {other for crossing in artifact.get("crossings", []) if source_id in crossing["ways"]
                 for other in crossing["ways"] if other != source_id})
            record["unknown"] = artifact.get("unknownForEarlierReleases", [])
            result["membership"] = record
            result["state"] = "retained"
            result["nodeCount"] = len(nodes)
        elif source_id in artifact.get("relations", {}):
            result["relation"] = artifact["relations"][source_id]
            result["state"] = "retained"
        else:
            result["state"] = "unknown"
            result["reason"] = "feature-not-in-topology-artifact"
            result["omittedRelationships"] = artifact.get("omittedRelationships", [])
        return result

    def _feature_parts(self, layer, source_id=None):
        import fiona
        from shapely.geometry import mapping, shape
        parts = []
        for item in self.vector_members(layer):
            with fiona.open(member_path(self.package, item["name"]), driver="GPKG", layer="features") as source:
                for feature in source:
                    properties = dict(feature.properties)
                    if source_id is not None and properties.get("source_id") != source_id:
                        continue
                    tags = json.loads(properties.get("source_tags") or "{}")
                    geometry = shape(feature.geometry)
                    parts.append({
                        "partId": f"{item['name']}:{feature.id}",
                        "sourceId": properties.get("source_id"),
                        "layer": layer,
                        "component": item["component"],
                        "member": item["name"],
                        "memberSha256": item["sha256"],
                        "geometry": mapping(geometry),
                        "geometryType": geometry.geom_type,
                        "tags": tags,
                        "dimensions": self._feature_dimensions(layer, tags),
                    })
        return parts

    @staticmethod
    def _describe(part):
        from shapely.geometry import shape
        geometry = shape(part["geometry"])
        summary = {"geometryType": geometry.geom_type,
                   "bounds": [round(value, 12) for value in geometry.bounds]}
        if geometry.geom_type == "Polygon":
            summary.update(polygonParts=1, holes=len(geometry.interiors))
        elif geometry.geom_type == "MultiPolygon":
            summary.update(polygonParts=len(geometry.geoms), pieceCount=len(geometry.geoms),
                           holes=sum(len(part.interiors) for part in geometry.geoms))
        elif geometry.geom_type == "MultiLineString":
            summary.update(pieceCount=len(geometry.geoms),
                           pieces=[[list(part.coords[0]), list(part.coords[-1])]
                                   for part in geometry.geoms])
        elif geometry.geom_type == "LineString":
            summary.update(start=[round(value, 12) for value in geometry.coords[0]],
                           end=[round(value, 12) for value in geometry.coords[-1]])
        elif geometry.geom_type == "Point":
            summary.update(point=[round(value, 12) for value in geometry.coords[0]])
        return summary

    def feature(self, layer, source_id):
        parts = self._feature_parts(layer, source_id)
        if not parts:
            return {"outcome": "absent", "layer": layer, "sourceId": source_id}
        described = [{**self._describe(part), **part} for part in parts]
        dimensions = described[0]["dimensions"]
        modeled = dimensions.pop("modeledHeight", None)
        result = {"outcome": "value", "layer": layer, "sourceId": source_id, "partCount": len(parts),
                  "topology": self._topology_for(layer, source_id),
                  "dimensions": dimensions, "parts": described}
        if modeled is not None:
            result["modeledHeight"] = modeled
        return {**result,
                "lineage": {"schema": SCHEMA, "manifestSha256": self.manifest_sha,
                            "members": sorted({part["member"] for part in described})}}

    def features(self, layer, bounds=None, limit=100):
        from shapely.geometry import box, shape
        if not 1 <= limit <= 200:
            raise ValueError("Feature result count must be between 1 and 200")
        selection = box(*bounds) if bounds else None
        out, truncated = [], False
        for part in self._feature_parts(layer):
            geometry = shape(part["geometry"])
            if selection is not None and not geometry.intersects(selection):
                continue
            clipped = geometry.intersection(selection) if selection is not None else geometry
            if clipped.is_empty:
                continue
            if len(out) >= limit:
                truncated = True
                break
            item = {**self._describe(part)}
            item["geometry"] = __import__("shapely.geometry", fromlist=["mapping"]).mapping(clipped)
            item.update(sourceId=part["sourceId"], partId=part["partId"], member=part["member"],
                        tags=part["tags"], dimensions=part["dimensions"])
            out.append(item)
        return {"outcome": "value", "layer": layer, "count": len(out), "truncated": truncated,
                "features": out,
                "lineage": {"schema": SCHEMA, "manifestSha256": self.manifest_sha, "layer": layer}}

    # -- frame -----------------------------------------------------------
    def frame_point(self, longitude, latitude, ellipsoid_height=0.0, check_controls=False):
        from pyproj import Transformer
        origin = self.manifest["frame"]["origin"]
        transformer = Transformer.from_pipeline(
            "+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad "
            "+step +proj=cart +ellps=WGS84 +step +proj=topocentric +ellps=WGS84 "
            f"+lon_0={origin[0]} +lat_0={origin[1]} +h_0={origin[2]}")
        east, north, up = transformer.transform(longitude, latitude, ellipsoid_height, errcheck=True)
        result = {"outcome": "value", "enu": [east, north, up], "origin": origin,
                  "axes": self.manifest["frame"]["axes"],
                  "axisOrder": self.manifest["frame"]["axisOrder"],
                  "heightDatum": self.manifest["frame"]["heightDatum"]}
        if check_controls:
            errors = []
            for control in self.manifest["frame"]["controls"]:
                projected = transformer.transform(control["longitude"], control["latitude"],
                                                  control["ellipsoidHeightM"], errcheck=True)
                errors.append(math.dist(projected, control["enuM"]))
            result["controlMaxErrorM"] = max(errors)
            result["controlCount"] = len(errors)
            if max(errors) > 1e-6:
                result.update(outcome="unexpected-outcome", reason="frame-control-disagreement")
        return result


# -- time-aware fields ---------------------------------------------------
def load_field(path):
    from observation_formats import validate_weather
    document = json.loads(Path(path).read_text(encoding="utf-8"))
    validate_weather(document)
    return document


def field_read(path, variable, level, valid_time, longitude, latitude, known_at=None):
    document = load_field(path)
    temporal = document.get("temporalMeaning") or {
        "kind": "modeled-value-valid-at-instant",
        "declaredTimes": {"valid": document["validTimes"], "observation": None, "issue": None,
                          "platformAvailability": None},
        "meaning": "Native pressure-level value valid at the declared UTC instant.",
    }
    if variable not in document["variables"]:
        return {"outcome": "absent", "reason": "variable-not-retained",
                "retained": sorted(document["variables"])}
    if level not in document["pressureHPa"]:
        return {"outcome": "unsupported-vertical-coordinate", "reason": "pressure-axis",
                "detail": "The retained field uses a pressure-axis in hPa; a metre request cannot be resolved.",
                "retainedPressureHPa": document["pressureHPa"],
                "verticalCoordinate": document["verticalCoordinate"]}
    if valid_time not in document["validTimes"]:
        return {"outcome": "unavailable-hour", "validTime": valid_time,
                "declaredTimes": document["validTimes"]}
    if latitude not in document["latitudes"] or longitude not in document["longitudes"]:
        return {"outcome": "off-grid", "reason": "native-cell-required",
                "detail": "Native cell lookup only; interpolated values are not part of this reader.",
                "latitudes": document["latitudes"], "longitudes": document["longitudes"]}
    availability = None
    if known_at:
        if known_at < valid_time:
            return {"outcome": "unavailable-before-validity", "validTime": valid_time,
                    "knownAt": known_at}
        availability = known_at
    L, Y, X = len(document["pressureHPa"]), len(document["latitudes"]), len(document["longitudes"])
    t = document["validTimes"].index(valid_time)
    p = document["pressureHPa"].index(level)
    y = document["latitudes"].index(latitude)
    x = document["longitudes"].index(longitude)
    index = ((t * L + p) * Y + y) * X + x
    above = document["aboveGroundMask"][index]
    value = document["variables"][variable]["samples"][index]
    base_result = {"variable": variable, "validTime": valid_time, "levelHPa": level,
                   "latitude": latitude, "longitude": longitude,
                   "nativeIndex": {"time": t, "level": p, "latitude": y, "longitude": x, "flat": index},
                   "edition": document["edition"], "verticalCoordinate": document["verticalCoordinate"],
                   "temporal": {**temporal, "knownAt": availability},
                   "lineage": {"schema": SCHEMA, "fieldEdition": document["edition"],
                               "fieldSchema": document["schema"], "validTime": valid_time}}
    if not above:
        return {**base_result, "outcome": "masked", "reason": "below-ground"}
    if value is None:
        return {**base_result, "outcome": "masked", "reason": "missing-sample"}
    return {**base_result, "outcome": "value", "value": value,
            "unit": document["variables"][variable]["units"], "maskState": "above-ground"}


def field_times(path):
    document = load_field(path)
    return {"outcome": "value", "schema": document["schema"], "edition": document["edition"],
            "validTimes": document["validTimes"], "pressureHPa": document["pressureHPa"],
            "latitudes": document["latitudes"], "longitudes": document["longitudes"],
            "temporalMeaning": document.get("temporalMeaning"),
            "variables": sorted(document["variables"]),
            "sha256": hashlib.sha256(Path(path).read_bytes()).hexdigest()}


# -- evidence and suitability -------------------------------------------
def evidence_from_manifest(manifest, assessments=(), terms=None, visual=None):
    return {
        "schema": suitability.EVIDENCE_SCHEMA,
        "manifestSha256": hashlib.sha256(canonical(manifest).encode()).hexdigest(),
        "profileId": manifest.get("profileId"),
        "areaRevisionId": manifest.get("areaRevisionId"),
        "area": manifest.get("area"),
        "members": [{"name": item["name"], "layer": item["layer"], "component": item.get("component"),
                     "sha256": item["sha256"], "bytes": item["bytes"],
                     "details": item.get("details") or {}}
                    for item in manifest.get("members", [])],
        "inputs": [{"layer": item.get("representation", {}).get("layer"),
                    "verticalDatum": item.get("representation", {}).get("verticalDatum"),
                    "dataTime": item.get("representation", {}).get("dataTime"),
                    "uncertainty": item.get("representation", {}).get("uncertainty"),
                    "crs": item.get("representation", {}).get("crs"),
                    "productEdition": item.get("representation", {}).get("productEdition")}
                   for item in manifest.get("inputs", [])],
        "assessments": list(assessments),
        "terms": terms,
        "representationAvailability": visual or {},
    }


# -- assets (P07) --------------------------------------------------------
def asset_inventory(package, descriptor_path):
    reader = Reader(package)
    descriptor = json.loads(Path(descriptor_path).read_text(encoding="utf-8"))
    if descriptor.get("schema") != "emad.geographic-derivative.v1":
        raise ValueError("Unsupported derivative descriptor schema")
    if descriptor.get("units") != "metre":
        raise ValueError("Derivative descriptor must declare metre units")
    if descriptor.get("axes") != ["east", "up", "south"]:
        raise ValueError("Derivative descriptor must declare the geographic display axes")
    if descriptor.get("purpose") not in ("visual-reference",):
        raise ValueError("Unsupported derivative purpose")
    parents = []
    for parent in descriptor.get("parentMembers", []):
        member = reader.member(parent["name"])
        if member["sha256"] != parent["sha256"]:
            raise ValueError(f"Derivative parent differs from the release member: {parent['name']}")
        parents.append({"name": member["name"], "layer": member["layer"],
                        "component": member["component"], "sha256": member["sha256"]})
    surface_members = {item["name"] for item in reader.native_members("surface")}
    if not any(parent["layer"] == "surface" for parent in parents):
        raise ValueError("Terrain derivative does not resolve to a surface source member")
    computational = bool(descriptor.get("computationalUse"))
    usable = ["visual-inspection"]
    unusable = ["native-ground-elevation", "measurement", "survey-control"]
    if computational:
        raise ValueError("This descriptor claims computational use; a display derivative cannot")
    if not surface_members & {parent["name"] for parent in parents}:
        raise ValueError("Derivative parents do not include the retained surface member")
    return {"schema": "emad.geographic-asset-inventory.v1", "manifestSha256": reader.manifest_sha,
            "displayId": descriptor.get("displayId"), "purpose": descriptor["purpose"],
            "purposeMeaning": descriptor.get("purposeMeaning"),
            "representation": descriptor.get("representation"),
            "recipe": descriptor.get("recipe"), "units": descriptor.get("units"),
            "axes": descriptor.get("axes"), "boundsLonLat": descriptor.get("boundsLonLat"),
            "parents": parents,
            "renderingAdjustments": descriptor.get("renderingAdjustments"),
            "limitations": descriptor.get("limitations"),
            "usableFor": usable, "notUsableFor": unusable,
            "frame": reader.manifest["frame"]}


def verify_kit(kit, inventory_sha=None, now=None):
    ops = Path(__file__).resolve().parents[1] / "ops"
    if str(ops) not in sys.path:
        sys.path.insert(0, str(ops))
    from environment_kit import verify as verify_kit_directory
    return verify_kit_directory(Path(kit), inventory_sha, now)


# -- CLI -----------------------------------------------------------------
def _print(value):
    print(json.dumps(value, sort_keys=True, indent=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)

    inventory = sub.add_parser("inventory")
    inventory.add_argument("package", type=Path)
    inventory.add_argument("--manifest-sha256")
    inventory.add_argument("--verify", action="store_true")

    sample = sub.add_parser("sample")
    sample.add_argument("package", type=Path)
    sample.add_argument("--manifest-sha256")
    sample.add_argument("--member", required=True)
    sample.add_argument("--row", type=int, required=True)
    sample.add_argument("--column", type=int, required=True)
    sample.add_argument("--unit", default="metre")

    point = sub.add_parser("point")
    point.add_argument("package", type=Path)
    point.add_argument("--manifest-sha256")
    point.add_argument("--layer", required=True, choices=("surface", "ground", "bathy", "land-cover"))
    point.add_argument("--longitude", type=float, required=True)
    point.add_argument("--latitude", type=float, required=True)
    point.add_argument("--unit", default="metre")
    point.add_argument("--datum")
    point.add_argument("--component", type=int)

    feature = sub.add_parser("feature")
    feature.add_argument("package", type=Path)
    feature.add_argument("--manifest-sha256")
    feature.add_argument("--topology", type=Path)
    feature.add_argument("--layer", required=True)
    feature.add_argument("--source-id", required=True)

    features = sub.add_parser("features")
    features.add_argument("package", type=Path)
    features.add_argument("--manifest-sha256")
    features.add_argument("--layer", required=True)
    features.add_argument("--bounds", help="west,south,east,north")
    features.add_argument("--limit", type=int, default=100)

    frame = sub.add_parser("frame")
    frame.add_argument("package", type=Path)
    frame.add_argument("--manifest-sha256")
    frame.add_argument("--longitude", type=float, required=True)
    frame.add_argument("--latitude", type=float, required=True)
    frame.add_argument("--height", type=float, default=0.0)
    frame.add_argument("--check-controls", action="store_true")

    field = sub.add_parser("field")
    field.add_argument("field", type=Path)
    field.add_argument("--variable", required=True)
    field.add_argument("--level-hpa", type=int, required=True)
    field.add_argument("--valid-time", required=True)
    field.add_argument("--longitude", type=float, required=True)
    field.add_argument("--latitude", type=float, required=True)
    field.add_argument("--known-at")

    field_times_parser = sub.add_parser("field-times")
    field_times_parser.add_argument("field", type=Path)

    asset = sub.add_parser("asset")
    asset.add_argument("package", type=Path)
    asset.add_argument("--descriptor", type=Path, required=True)

    suit = sub.add_parser("suitability")
    suit.add_argument("package", type=Path, help="Prepared package, or omit with --evidence")
    suit.add_argument("--evidence", type=Path)
    suit.add_argument("--requirements", type=Path, required=True)
    suit.add_argument("--assessments", type=Path,
                      help="JSON array of immutable assessment records")
    suit.add_argument("--terms", type=Path, help="Issued terms used for offline evaluation")
    suit.add_argument("--visual-derivative", action="append", default=[],
                      help="Layer for which only a visual derivative is available")
    suit.add_argument("--now", help="Evaluation instant; required to state issued-term expiry")

    kit = sub.add_parser("verify-kit")
    kit.add_argument("kit", type=Path)
    kit.add_argument("--inventory-sha256")
    kit.add_argument("--now")

    args = parser.parse_args()
    try:
        if args.action == "inventory":
            reader = Reader(args.package, args.manifest_sha256)
            result = reader.inventory()
            if args.verify:
                result["verification"] = base.verify(args.package, args.manifest_sha256)
            _print(result)
        elif args.action == "sample":
            _print(Reader(args.package, args.manifest_sha256).sample(args.member, args.row, args.column, args.unit))
        elif args.action == "point":
            _print(Reader(args.package, args.manifest_sha256).point(
                args.layer, args.longitude, args.latitude, args.unit, args.datum, args.component))
        elif args.action == "feature":
            _print(Reader(args.package, args.manifest_sha256, args.topology).feature(args.layer, args.source_id))
        elif args.action == "features":
            bounds = [float(value) for value in args.bounds.split(",")] if args.bounds else None
            _print(Reader(args.package, args.manifest_sha256).features(args.layer, bounds, args.limit))
        elif args.action == "frame":
            _print(Reader(args.package, args.manifest_sha256).frame_point(
                args.longitude, args.latitude, args.height, args.check_controls))
        elif args.action == "field":
            _print(field_read(args.field, args.variable, args.level_hpa, args.valid_time,
                              args.longitude, args.latitude, args.known_at))
        elif args.action == "field-times":
            _print(field_times(args.field))
        elif args.action == "asset":
            _print(asset_inventory(args.package, args.descriptor))
        elif args.action == "suitability":
            requirement = json.loads(args.requirements.read_text(encoding="utf-8"))
            if args.evidence:
                evidence = json.loads(args.evidence.read_text(encoding="utf-8"))
            else:
                reader = Reader(args.package)
                assessments = json.loads(args.assessments.read_text(encoding="utf-8")) if args.assessments else []
                terms = json.loads(args.terms.read_text(encoding="utf-8")) if args.terms else None
                evidence = evidence_from_manifest(
                    reader.manifest, assessments, terms,
                    {"visual-derivative": args.visual_derivative} if args.visual_derivative else None)
            _print(suitability.evaluate(requirement, evidence, args.now))
        else:
            _print(verify_kit(args.kit, args.inventory_sha256, args.now))
        return 0
    except (KeyError, OSError, TypeError, ValueError, zipfile.BadZipFile) as error:
        print(str(error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
