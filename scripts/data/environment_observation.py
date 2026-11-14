"""Prepare, verify and rebuild the versioned geographic observation profile."""
import argparse
from contextlib import contextmanager
import hashlib
import json
from pathlib import Path
import sys
from tempfile import TemporaryDirectory
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
import environment_library as base
import observation_formats as formats

PROFILE = "reference-observation-v1"
REQUIRED = {"surface", "geoid"}
BASE_LAYERS = REQUIRED | {"imagery", "roads", "buildings", "water"}
OPTIONAL = {"radar", "weather-field"}
ORIGINAL_TOOLCHAIN, ORIGINAL_PREPARE, ORIGINAL_VERIFY = base.toolchain, base.prepare, base.verify


def toolchain():
    return hashlib.sha256(base.encoded({"base": ORIGINAL_TOOLCHAIN(), "reader": base.file_digest(Path(formats.__file__))[0],
                                       "processor": base.file_digest(Path(__file__))[0]})).hexdigest()


@contextmanager
def profile():
    old = base.toolchain, base.PROFILE_LAYERS
    base.toolchain, base.PROFILE_LAYERS = toolchain, {**base.PROFILE_LAYERS, PROFILE: REQUIRED}
    try:
        yield
    finally:
        base.toolchain, base.PROFILE_LAYERS = old


def rep_matches(rep, layer):
    if layer == "radar":
        return (rep["format"] == "radar-capture-zip" and formats.RADAR_PRODUCT.fullmatch(rep["productEdition"])
                and rep["verticalDatum"] == "not-applicable" and rep["semantics"] == "gamma0-linear-power"
                and rep["crs"].startswith("EPSG:32"))
    return (rep["format"], rep["productEdition"], rep["verticalDatum"], rep["semantics"], rep["crs"]) == (
        "era5-pressure-json", formats.WEATHER_EDITION, "pressure-hPa", "reanalysis-pressure-field", "EPSG:4326")


def radar_check(path, meta):
    import numpy as np
    import rasterio
    with rasterio.open(path, driver="GTiff") as src:
        if (src.count != 1 or src.dtypes != ("float32",) or src.crs.to_string() != meta["crs"] or
                src.nodata != meta["nodata"] or src.scales != (1.,) or src.offsets != (0.,) or
                tuple(src.res) != (10., 10.) or src.width*src.height > 2_000_000):
            raise ValueError("Radar native grid or value interpretation changed")
        data = src.read(1, masked=True)
        valid = ~np.ma.getmaskarray(data) & np.isfinite(data.data) & (data.data > 0)
        if not valid.any():
            raise ValueError("No valid positive linear power samples")
        return {"positivePowerSamples": int(valid.sum()), "minimumLinear": float(data.data[valid].min()),
                "maximumLinear": float(data.data[valid].max())}


def prepare(root):
    from shapely.geometry import shape, box
    import rasterio
    from rasterio.transform import Affine
    recipe_path = root / "recipe.json"
    raw = base.read_small(recipe_path)
    recipe = json.loads(raw)
    if recipe["profile"]["id"] != PROFILE or recipe["toolchainSha256"] != toolchain():
        raise ValueError("Unsupported observation profile or preparation digest")
    area = shape(recipe["area"]["geometry"])
    if area.geom_type == "MultiPolygon" and len(area.geoms) == 1:
        area = area.geoms[0]
    if area.geom_type != "Polygon" or not area.is_valid or area.is_empty:
        raise ValueError("The observation profile requires one bounded polygon")
    inputs = recipe["inputs"]
    if not 1 <= len(inputs) <= 64 or len({a["id"] for a in inputs}) != len(inputs) or sum(a["bytes"] for a in inputs) > base.MAX_TOTAL:
        raise ValueError("Observation profile input budget exceeded")
    extras = [a for a in inputs if a["representation"]["layer"] in OPTIONAL]
    if not extras or len(extras) != len({a["representation"]["layer"] for a in extras}):
        raise ValueError("Supply one or both optional layers, with one exact input per layer")
    baseline = [a for a in inputs if a not in extras]
    if any(a["representation"]["layer"] not in BASE_LAYERS for a in baseline):
        raise ValueError("Unexpected observation profile baseline layer")
    recipe_path.write_bytes(base.encoded({**recipe, "inputs": baseline}))
    try:
        with profile():
            manifest = ORIGINAL_PREPARE(root)
    finally:
        recipe_path.write_bytes(raw)
    manifest["inputs"] = inputs
    quality = {}
    for asset in extras:
        rep = asset["representation"]
        layer = rep["layer"]
        if not base.ASSET_ID.fullmatch(asset["id"]) or not rep_matches(rep, layer):
            raise ValueError("Unsupported observation input representation")
        source = root / "inputs" / asset["id"]
        budget = formats.RADAR_MAX if layer == "radar" else formats.WEATHER_MAX
        if asset["bytes"] > budget or base.file_digest(source, budget) != (asset["sha256"], asset["bytes"]):
            raise ValueError("Observation input bytes differ from the pinned recipe")
        if layer == "radar":
            with TemporaryDirectory(prefix="emad-radar-native-") as folder:
                native = Path(folder) / "native.tif"
                item, meta, receipt = formats.radar_archive(source, native)
                projected = Affine(*item["properties"]["proj:transform"][:6])
                col, row, width, height = receipt["window"]
                if any(type(x) is not int or x < 0 for x in (col,row,width,height)) or not width or not height:
                    raise ValueError("Invalid retained radar source window")
                rows, cols = item["properties"]["proj:shape"]
                with rasterio.open(native, driver="GTiff") as src:
                    if (col+width > cols or row+height > rows or (src.width, src.height) != (width,height)
                            or src.transform != projected * Affine.translation(col,row) or list(src.transform)[:6] != receipt["affine"]
                            or rep["crs"] != meta["crs"] or rep["productEdition"] != meta["product"] or rep["dataTime"] != meta["sourceStart"]
                            or not shape(item["geometry"]).covers(area) or not box(*receipt["selectionBounds"]).covers(area)):
                        raise ValueError("Radar product, grid or footprint differs from its retained input binding")
                radar_check(native, meta)
                output = root / "radar-vv.tif"
                details = base.crop_raster(native, output, area, rep)
                if not details:
                    raise ValueError("No retained radar samples cover this area")
                quality["radar"] = {**meta, **radar_check(output, meta), "metadataSha256": receipt["metadataSha256"],
                                    "retainedNativeSha256": receipt["retainedSha256"], "etag": receipt["etag"]}
                manifest["members"].append(base.member(output, asset, details, 0))
        else:
            output = root / "era5-pressure.json"
            data = json.loads(source.read_bytes())
            count = formats.validate_weather(data)
            if rep["dataTime"] != data["validTimes"][0] or not box(min(data["longitudes"]), min(data["latitudes"]),
                                                                max(data["longitudes"]), max(data["latitudes"])).covers(area):
                raise ValueError("ERA5 field does not bind the requested area and time")
            output.write_bytes(base.encoded(data))
            quality["weather"] = data
            manifest["members"].append(base.member(output, asset, {"samplesPerVariable": count, "spacingDegrees": .25,
                "verticalCoordinate": "pressure-hPa", "physicalAccuracy": "UNKNOWN"}, None))
    manifest["quality"]["observations"] = quality
    manifest["quality"]["unsupported"] = sorted(set(manifest["quality"]["unsupported"]) | {"operational weather", "forecast", "weather at geometric heights"})
    data = base.encoded(manifest)
    (root / "manifest.json").write_bytes(data)
    candidate = root / "observation.zip"
    base.prepared_zip(candidate, root, ["manifest.json", *(m["name"] for m in manifest["members"])])
    if candidate.stat().st_size > base.MAX_OBJECT:
        raise ValueError("Observation package exceeds single object budget")
    candidate.replace(root / "prepared.zip")
    return manifest


def verify(path, expected=None):
    with profile():
        result = ORIGINAL_VERIFY(path, expected)
    if result["profile"] != PROFILE:
        return result
    with zipfile.ZipFile(path) as z, TemporaryDirectory(prefix="emad-observation-verify-") as folder:
        entries = base.archive_entries(z)
        _, manifest = base.archive_json(z, entries, "manifest.json")
        quality = result["quality"]["observations"]
        inputs = {a["id"]: a["representation"] for a in manifest["inputs"]}
        extra = {}
        for m in manifest["members"]:
            layer = m["layer"]
            if layer in BASE_LAYERS:
                continue
            if layer in extra or layer not in OPTIONAL or not rep_matches(inputs[m["inputId"]], layer):
                raise ValueError("Unexpected observation member")
            extra[layer] = m
            if layer == "weather-field":
                raw, data = base.archive_json(z, entries, m["name"])
                if len(raw) > formats.WEATHER_MAX or formats.validate_weather(data) != m["details"]["samplesPerVariable"] or data != quality.get("weather"):
                    raise ValueError("Weather field or quality metadata differs from retained samples")
            else:
                raster = Path(z.extract(entries[m["name"]], folder))
                meta = quality["radar"]
                rep = inputs[m["inputId"]]
                if (rep["productEdition"] != meta["product"] or rep["dataTime"] != meta["sourceStart"] or
                        rep["crs"] != meta["crs"] or meta["mode"] != "IW" or meta["polarization"] != "VV" or
                        meta["unit"] != "gamma0 linear power" or any(meta.get(k) != v for k,v in radar_check(raster,meta).items())):
                    raise ValueError("Radar quality differs from retained raster")
        if not extra or set(extra) != {a["layer"] for a in inputs.values() if a["layer"] in OPTIONAL}:
            raise ValueError("Observation members do not match the selected capabilities")
    return result


def rebuild(source, output):
    old = base.prepare, base.verify
    base.prepare, base.verify = prepare, verify
    try:
        return base.rebuild(source, output)
    finally:
        base.prepare, base.verify = old


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest="action", required=True)
    sub.add_parser("toolchain")
    sub.add_parser("prepare").add_argument("path", type=Path)
    v = sub.add_parser("verify")
    v.add_argument("path", type=Path)
    v.add_argument("--manifest-sha256")
    b = sub.add_parser("rebuild")
    b.add_argument("path", type=Path)
    b.add_argument("output", type=Path)
    a = p.parse_args()
    try:
        if a.action == "toolchain":
            print(toolchain())
        elif a.action == "prepare":
            result = prepare(a.path)
            verify(a.path / "prepared.zip")
            print(json.dumps({"status": "PREPARED", "members": len(result["members"])}))
        elif a.action == "verify":
            print(json.dumps(verify(a.path, a.manifest_sha256)))
        else:
            print(json.dumps(rebuild(a.path, a.output)))
    except (ValueError, KeyError, TypeError, OSError, zipfile.BadZipFile) as exc:
        p.exit(2, f"{exc}\n")
