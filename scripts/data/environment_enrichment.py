"""Versioned preparation, verification and rebuild for geographic enrichment."""
import argparse
from contextlib import contextmanager
import hashlib
import json
import math
from pathlib import Path
import re
import sys
import tempfile
import zipfile

# The worker invokes this entry point in isolated Python mode.
sys.path.insert(0, str(Path(__file__).resolve().parent))
import environment_library as base

PROFILE = "reference-enrichment-v1"
REQUIRED = {"surface", "imagery", "roads", "buildings", "water", "geoid", "land-cover"}
CLASSES = {
    10: ("Tree cover", "006400"), 20: ("Shrubland", "ffbb22"), 30: ("Grassland", "ffff4c"),
    40: ("Cropland", "f096ff"), 50: ("Built-up", "fa0000"), 60: ("Bare / sparse vegetation", "b4b4b4"),
    70: ("Snow and ice", "f0f0f0"), 80: ("Permanent water bodies", "0064c8"),
    90: ("Herbaceous wetland", "0096a0"), 95: ("Mangroves", "00cf75"),
    100: ("Moss and lichen", "fae6a0"),
}
WORLDCOVER_EDITION = "ESA WorldCover 2021 v200"
GROUND_EDITION = "GEDTM30 filtered v20250619"
BUILDING_CONTEXT = "modeled-building-context"
BUILDING_EDITION = "GHS-OBAT R2024A V1.0"
DIMENSIONS = {
    "buildings": {"height": ("height", "m", 2000), "min_height": ("minHeight", "m", 2000),
                  "building:levels": ("levels", "count", 300), "roof:height": ("roofHeight", "m", 500)},
    "roads": {"width": ("width", "m", 1000), "maxheight": ("clearance", "m", 100),
              "lanes": ("lanes", "count", 50)},
    "water": {"width": ("width", "m", 10000), "depth": ("depth", "m", 12000)},
}
BASE_PREPARE, BASE_VERIFY, BASE_CROP, BASE_VERIFY_SPATIAL = base.prepare, base.verify, base.crop_raster, base.verify_spatial


def toolchain():
    return hashlib.sha256(base.encoded({"base": ORIGINAL_TOOLCHAIN(), "enrichment": base.file_digest(Path(__file__))[0]})).hexdigest()


ORIGINAL_TOOLCHAIN = base.toolchain


def typed_dimension(tags, tag, unit, upper):
    raw = tags.get(tag)
    value = None
    if isinstance(raw, str) and len(raw) <= 64:
        raw = raw.strip()
        if unit == "count":
            if re.fullmatch(r"[0-9]+", raw):
                value = int(raw)
        else:
            metric = re.fullmatch(r"([0-9]+(?:\.[0-9]+)?)\s*(?:m)?", raw)
            imperial = re.fullmatch(r"([0-9]+(?:\.[0-9]+)?)\s*ft", raw)
            feet_inches = re.fullmatch(r"""([0-9]+)'(?:\s*([0-9]+(?:\.[0-9]+)?)")?""", raw)
            if metric:
                value = float(metric[1])
            elif imperial:
                value = float(imperial[1]) * .3048
            elif feet_inches and float(feet_inches[2] or 0) < 12:
                value = float(feet_inches[1]) * .3048 + float(feet_inches[2] or 0) * .0254
    if value is not None and ((value >= 0 if tag == "min_height" else value > 0) and value <= upper):
        return {"value": round(value, 6), "unit": unit, "origin": "provider-stated", "evidence": f"source tag: {tag}",
                "uncertainty": "UNKNOWN"}
    return {"value": None, "unit": unit, "origin": "unknown",
            "evidence": f"unsupported source tag: {tag}" if tag in tags else f"missing source tag: {tag}", "uncertainty": "UNKNOWN"}


def dimensions(layer, tags):
    return {name: typed_dimension(tags, tag, unit, upper) for tag, (name, unit, upper) in DIMENSIONS[layer].items()}


def class_counts(values, mask):
    import numpy as np
    values, counts = np.unique(values[mask], return_counts=True)
    unknown = set(values.tolist()) - set(CLASSES)
    if unknown:
        raise ValueError(f"WorldCover contains unsupported class codes: {sorted(unknown)}")
    return {str(code): int(count) for code, count in zip(values, counts)}


def check_representation(rep):
    layer = rep["layer"]
    if layer == "land-cover" and (rep["format"], rep["crs"], rep["verticalDatum"], rep["productEdition"],
                                  rep["dataTime"], rep["semantics"]) != (
            "geotiff", "EPSG:4326", "not-applicable", WORLDCOVER_EDITION, "2021", "categorical-land-cover"):
        raise ValueError("Land cover requires the exact WorldCover categorical representation")
    if layer == "ground" and (rep["format"], rep["crs"], rep["verticalDatum"], rep["productEdition"],
                               rep["dataTime"], rep["semantics"]) != (
            "geotiff", "EPSG:4326", "EGM2008", GROUND_EDITION, "2006/2015", "modeled-bare-earth"):
        raise ValueError("Ground requires the exact modeled GEDTM30 EGM2008 representation")
    if layer == BUILDING_CONTEXT and (rep["format"], rep["crs"], rep["verticalDatum"], rep["productEdition"],
                                      rep["dataTime"], rep["semantics"]) != (
            "geojson", "EPSG:4326", "above-ground", BUILDING_EDITION, "2020", "modeled-building-height-context"):
        raise ValueError("Building context requires the supported modeled GHS-OBAT point representation")


def modeled_value(tags):
    raw = tags["height"]
    value = float(raw)
    if raw == "NaN":
        return {"value": None, "unit": "m", "origin": "unknown", "uncertainty": "UNKNOWN",
                "evidence": "GHS-OBAT has no modeled height for this centroid"}
    if not math.isfinite(value) or not 0 < value <= 2000:
        raise ValueError("Invalid GHS-OBAT modeled height")
    return {"value": value, "unit": "m", "origin": "inferred", "uncertainty": "UNKNOWN",
            "evidence": "GHS-OBAT zonal statistic from modeled building-height grids"}


ORIGINAL_VECTORS = base.vector_features


def vectors(source, rep, area, scratch):
    check_representation(rep)
    if rep["layer"] != BUILDING_CONTEXT:
        return ORIGINAL_VECTORS(source, rep, area, scratch)
    from shapely.geometry import shape
    data = json.loads(base.read_small(source))
    if data["type"] != "FeatureCollection" or not 1 <= len(data["features"]) <= 30000:
        raise ValueError("Invalid GHS-OBAT bounded point count")
    rows, seen = [], set()
    for item in data["features"]:
        geom = shape(item["geometry"])
        tags = item["properties"]
        key = tags["sourceId"]
        if item["type"] != "Feature" or set(tags) != {"sourceId", "height"} or not re.fullmatch(r"[0-9a-f]{32}", key) or key in seen:
            raise ValueError("Invalid GHS-OBAT point attributes or identity")
        if geom.geom_type != "Point" or geom.has_z or geom.is_empty or not geom.is_valid:
            raise ValueError("GHS-OBAT context must be native WGS84 centroids")
        seen.add(key)
        modeled_value(tags)
        if area.covers(geom):
            rows.append((key, geom, tags))
    return sorted(rows, key=lambda row: row[0]), 0


def raster_details(path, layer):
    import numpy as np
    import rasterio
    with rasterio.open(path, driver="GTiff") as src:
        if layer == "land-cover":
            if src.count != 1 or src.dtypes != ("uint8",) or not all(abs(n - 1 / 12000) < 1e-10 for n in src.res):
                raise ValueError("WorldCover must retain its single-band 10 m native grid and integer classes")
            counts = {}
            for _, window in src.block_windows(1):
                for code, count in class_counts(src.read(1, window=window), src.read_masks(1, window=window) > 0).items():
                    counts[code] = counts.get(code, 0) + count
            return {"classCounts": counts, "legend": {str(c): {"label": n, "rgb": rgb} for c, (n, rgb) in CLASSES.items()},
                    "noDataCode": 0, "observationYear": 2021, "physicalAccuracy": "UNKNOWN"}
        if src.count != 1 or src.dtypes != ("int32",) or src.scales != (.1,) or src.offsets != (0.0,) or src.units != ("m",) or not all(
                abs(n - .00025) < 1e-10 for n in src.res):
            raise ValueError("GEDTM30 requires its native single-band grid and explicit decimetre scale")
        lo, hi = math.inf, -math.inf
        for _, window in src.block_windows(1):
            data = src.read(1, window=window, masked=True)
            if data.count():
                lo, hi = min(lo, float(data.min()) * .1), max(hi, float(data.max()) * .1)
        if not np.isfinite([lo, hi]).all() or lo < -12000 or hi > 9000:
            raise ValueError("Invalid ground sample values after applying the scale")
        return {"scale": .1, "offset": 0, "unit": "m", "minimumM": lo, "maximumM": hi,
                "physicalAccuracy": "UNKNOWN"}


def crop(source, output, area, rep):
    check_representation(rep)
    layer = rep["layer"]
    if layer == "ground":
        import rasterio
        with rasterio.open(source, driver="GTiff") as src:
            if src.tags().get("VERTICAL_DATUM") != "EGM2008/EPSG:3855":
                raise ValueError("GEDTM30 retained window lacks the publisher's EGM2008 datum tag")
    details = BASE_CROP(source, output, area, rep)
    if details and layer in {"land-cover", "ground"}:
        details.update(raster_details(output, layer))
    return details


def comparison(surface_path, ground_path):
    import numpy as np
    import rasterio
    from rasterio.warp import reproject, Resampling
    with rasterio.open(surface_path) as surface, rasterio.open(ground_path) as ground:
        if surface.width * surface.height > 2_000_000 or ground.width * ground.height > 2_000_000:
            raise ValueError("Ground comparison exceeds its sample budget")
        if surface.crs.to_epsg() != 4326 or ground.crs.to_epsg() != 4326 or surface.count != 1:
            raise ValueError("Ground comparison requires one native north-up WGS84 surface grid")
        sm = surface.read(1, masked=True).astype(np.float64).filled(np.nan) * surface.scales[0] + surface.offsets[0]
        gm = ground.read(1, masked=True).astype(np.float64).filled(np.nan) * ground.scales[0] + ground.offsets[0]
        aligned = np.full(sm.shape, np.nan)
        reproject(gm, aligned, src_transform=ground.transform, src_crs=ground.crs, src_nodata=np.nan,
                  dst_transform=surface.transform, dst_crs=surface.crs, dst_nodata=np.nan, resampling=Resampling.nearest)
        valid = np.isfinite(sm) & np.isfinite(aligned)
        if not valid.any():
            raise ValueError("Ground and surface have no common valid samples")
        return sm - aligned, valid, surface.transform


def quality(root, manifest):
    import fiona
    import numpy as np
    from shapely.geometry import shape
    parents = {a["id"]: a["representation"] for a in manifest["inputs"]}
    layers = {}
    for m in manifest["members"]:
        layers.setdefault(m["layer"], []).append(m)
    parts = shape(manifest["area"])
    component_count = len(parts.geoms) if parts.geom_type == "MultiPolygon" else 1
    for layer in ("land-cover", "ground"):
        if any(rep["layer"] == layer for rep in parents.values()):
            members = layers.get(layer, [])
            if sorted(m["component"] for m in members) != list(range(component_count)):
                raise ValueError(f"{layer} requires exactly one retained native grid per component")
            if any(m["details"]["validSamples"] <= 0 for m in members):
                raise ValueError(f"{layer} has a component with no coverage")
    comparisons = []
    for g in layers.get("ground", []):
        matches = [s for s in layers["surface"] if s["component"] == g["component"]]
        if len(matches) != 1 or parents[matches[0]["inputId"]]["verticalDatum"] != "EGM2008":
            raise ValueError("Ground comparison requires one matching EGM2008 surface per component")
        s = matches[0]
        delta, valid, grid = comparison(root / s["name"], root / g["name"])
        values = delta[valid]
        comparisons.append({"component": g["component"], "surface": s["name"], "ground": g["name"],
                            "verticalDatum": "EGM2008", "unit": "m", "grid": list(grid)[:6], "method": "DSM minus DTM; nearest on native DSM grid",
                            "pairedSamples": len(values), "surfaceValidSamples": s["details"]["validSamples"],
                            "minimumM": float(values.min()), "medianM": float(np.median(values)), "maximumM": float(values.max()),
                            "p05M": float(np.percentile(values, 5)), "p95M": float(np.percentile(values, 95)),
                            "physicalAccuracy": "UNKNOWN"})
    observations = land_cover_context(root, layers, parents)
    audit = {}
    for layer, definitions in DIMENSIONS.items():
        report = {"featureCount": 0, "relationFeatures": 0, "polygonHoles": 0, "nativeMembers": [], "sourceOmissions": 0,
                  "dimensions": {name: {"provider-stated": 0, "unknown": 0} for name, _, _ in definitions.values()}}
        for m in layers[layer]:
            report["nativeMembers"].append(m["name"])
            report["sourceOmissions"] += m["details"]["omissions"]
            with fiona.open(root / m["name"], layer="features", driver="GPKG") as dataset:
                for feature in dataset:
                    geom = shape(feature.geometry)
                    polygons = list(geom.geoms) if geom.geom_type == "MultiPolygon" else [geom] if geom.geom_type == "Polygon" else []
                    report["polygonHoles"] += sum(len(p.interiors) for p in polygons)
                    report["relationFeatures"] += int(feature.properties["source_id"].startswith("osm-relation-"))
                    report["featureCount"] += 1
                    for name, result in dimensions(layer, json.loads(feature.properties["source_tags"])).items():
                        report["dimensions"][name][result["origin"]] += 1
        audit[layer] = report
    modeled = []
    for m in layers.get(BUILDING_CONTEXT, []):
        count, values = 0, []
        with fiona.open(root / m["name"], layer="features", driver="GPKG") as dataset:
            for feature in dataset:
                tags = json.loads(feature.properties["source_tags"])
                if shape(feature.geometry).geom_type != "Point" or tags["sourceId"] != feature.properties["source_id"]:
                    raise ValueError("GHS-OBAT point identity changed")
                count += 1
                result = modeled_value(tags)
                if result["value"] is not None:
                    values.append(result["value"])
        modeled.append({"member": m["name"], "inputId": m["inputId"], "centroids": count,
                        "modeledHeights": len(values), "unknown": count - len(values),
                        "medianM": float(np.median(values)) if values else None, "unit": "m", "origin": "inferred",
                        "physicalAccuracy": "UNKNOWN", "edition": BUILDING_EDITION,
                        "meaning": "Separate building context, derived from gridded statistics. No OSM identity match or measured height."})
    if any(r["layer"] == BUILDING_CONTEXT for r in parents.values()) and not modeled:
        raise ValueError("Building context has no retained native points")
    return {"groundComparison": comparisons, "landCoverContext": observations, "dimensionCoverage": audit,
            "modeledBuildingContext": modeled,
            "dimensionScope": "Retained source-tag evidence only. Source feature geometry is preserved; physical completeness and measured dimensions are unknown."}


def land_cover_context(root, layers, parents):
    import numpy as np
    import rasterio
    from rasterio.warp import reproject, Resampling
    report = []
    for m in layers["land-cover"]:
        scenes = [s for s in layers["imagery"] if s["component"] == m["component"]
                  and parents[s["inputId"]]["productEdition"].endswith(" SCL_20m")]
        if len(scenes) != 1:
            continue
        scene = scenes[0]
        with rasterio.open(root / m["name"]) as land, rasterio.open(root / scene["name"]) as scl:
            # Sample a fixed lattice on the native WorldCover grid. The later
            # scene classification is screening context and not field truth.
            stride = max(8, math.ceil(max(land.width, land.height) / 96))
            if land.width * land.height > 2_000_000:
                raise ValueError("Land cover context exceeds its sample budget")
            land_values = land.read(1, masked=True)[::stride, ::stride].filled(0)
            if scl.width * scl.height > 2_000_000:
                raise ValueError("Scene classification comparison exceeds its sample budget")
            grid = land.transform * rasterio.Affine.translation((1 - stride) / 2, (1 - stride) / 2) * rasterio.Affine.scale(stride)
            later = np.zeros(land_values.shape, dtype=np.uint8)
            reproject(scl.read(1, masked=True).filled(0), later, src_transform=scl.transform, src_crs=scl.crs,
                      src_nodata=0, dst_transform=grid, dst_crs=land.crs, dst_nodata=0, resampling=Resampling.nearest)
            eligible = (land_values > 0) & np.isin(later, [4, 5, 6])
            codes = [(50, 4), (60, 4), (80, 4), (10, 6), (60, 6)]
            flagged = []
            for code, scene_class in codes:
                positions = np.argwhere(eligible & (land_values == code) & (later == scene_class))
                for row, col in positions[:3]:
                    lon, lat = grid * (int(col) + .5, int(row) + .5)
                    flagged.append({"longitude": lon, "latitude": lat, "worldCover": code, "sceneClass": scene_class})
            report.append({"component": m["component"], "landCover": m["name"], "sceneClassification": scene["name"],
                           "sceneDate": parents[scene["inputId"]]["dataTime"], "nativePixelStride": stride,
                           "clearScreenedSamples": int(eligible.sum()), "reviewExamples": flagged,
                           "method": "Selected disagreement examples for visual review. Epoch, grid and class meanings differ; SCL is not ground truth."})
    return report


@contextmanager
def legacy_contract():
    # Keep the legacy implementation/digest unchanged for its existing releases.
    # This process runs a single command. Restore all bindings on exit.
    previous = {name: getattr(base, name) for name in ("toolchain", "PROFILE_LAYERS", "crop_raster", "vector_features", "prepare", "verify")}
    base.PROFILE_LAYERS = {**base.PROFILE_LAYERS, PROFILE: REQUIRED}
    base.toolchain, base.crop_raster, base.prepare, base.verify = toolchain, crop, prepare, verify
    base.vector_features = vectors
    try:
        yield
    finally:
        for name, value in previous.items():
            setattr(base, name, value)


def prepare(root):
    recipe = json.loads(base.read_small(root / "recipe.json"))
    if recipe["profile"]["id"] != PROFILE:
        raise ValueError("The enrichment worker accepts only the enrichment profile")
    with legacy_contract():
        manifest = BASE_PREPARE(root)
    manifest["quality"]["enrichment"] = quality(root, manifest)
    manifest["quality"]["unsupported"] = ["bathymetry", "verified building dimensions", "volumetric weather",
                                         "survey accuracy", "physical materials", "ground replacement"]
    raw = base.encoded(manifest)
    if len(raw) > base.MAX_JSON:
        raise ValueError("Enriched manifest exceeds metadata budget")
    (root / "manifest.json").write_bytes(raw)
    candidate = root / "enriched.zip"
    base.prepared_zip(candidate, root, ["manifest.json", *(m["name"] for m in manifest["members"])])
    if candidate.stat().st_size > base.MAX_OBJECT:
        raise ValueError("Enriched package exceeds the single-object budget")
    candidate.replace(root / "prepared.zip")
    return manifest


def verify(bundle, expected_digest=None):
    with legacy_contract():
        result = BASE_VERIFY(bundle, expected_digest)
    if result["profile"] != PROFILE:
        return result
    with zipfile.ZipFile(bundle) as archive, tempfile.TemporaryDirectory(prefix="emad-enrichment-") as scratch:
        entries = base.archive_entries(archive)
        _, manifest = base.archive_json(archive, entries, "manifest.json")
        for parent in manifest["inputs"]:
            check_representation(parent["representation"])
        for m in manifest["members"]:
            if m["layer"] not in REQUIRED | {"ground", BUILDING_CONTEXT}:
                raise ValueError("Unexpected layer in the enrichment profile")
            path = Path(archive.extract(entries[m["name"]], scratch))
            if m["layer"] in {"land-cover", "ground"}:
                if any(m["details"].get(k) != v for k, v in raster_details(path, m["layer"]).items()):
                    raise ValueError("Raster meaning, values or class counts differ from the enrichment manifest")
        if quality(Path(scratch), manifest) != manifest["quality"].get("enrichment"):
            raise ValueError("Enrichment comparison or dimension audit differs from the retained members")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    sub.add_parser("toolchain")
    sub.add_parser("prepare").add_argument("path", type=Path)
    inspect = sub.add_parser("verify")
    inspect.add_argument("path", type=Path)
    inspect.add_argument("--manifest-sha256")
    rebuild = sub.add_parser("rebuild")
    rebuild.add_argument("path", type=Path)
    rebuild.add_argument("output", type=Path)
    args = parser.parse_args()
    try:
        if args.action == "toolchain":
            print(toolchain())
        elif args.action == "prepare":
            manifest = prepare(args.path)
            verify(args.path / "prepared.zip")
            print(json.dumps({"status": "PREPARED", "members": len(manifest["members"])}))
        elif args.action == "verify":
            print(json.dumps(verify(args.path, args.manifest_sha256)))
        else:
            with legacy_contract():
                print(json.dumps(base.rebuild(args.path, args.output)))
    except (ValueError, KeyError, OSError, zipfile.BadZipFile) as exc:
        parser.exit(2, f"{exc}\n")


if __name__ == "__main__":
    main()
