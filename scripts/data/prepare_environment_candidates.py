"""Prepare a private reference candidate from retained geographic inputs."""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import zipfile

from environment_library import encoded, file_digest, prepare, rebuild, toolchain, verify

ROOT = Path(__file__).resolve().parents[2]
LOCAL = ROOT / "private/local/data"
ISOLATED = ROOT / "private/qualification-t13-t15/data"
ACCEPTANCE_AREAS = {
    "jeddah": [39.144, 21.464, 39.216, 21.536],
    "al-ain": [55.73, 24.20, 55.75, 24.22],
    "avignon": [4.82, 43.95, 4.84, 43.97],
}
DEM_REFERENCE = "https://dataspace.copernicus.eu/explore-data/data-collections/copernicus-contributing-missions/collections-description/COP-DEM"
BOUND_KEYS = ("west", "south", "east", "north")


def document(path):
    return json.loads(path.read_text(encoding="utf-8"))


def original(path):
    return document(path.with_suffix(path.suffix + ".receipt.json"))


def candidate_area(name, profile):
    if profile is None:
        if name not in ACCEPTANCE_AREAS:
            raise ValueError("Supply an area profile for a new candidate")
        return ACCEPTANCE_AREAS[name], name + "-candidate"
    if profile["schema"] != "emad.environment-candidate-area.v1" or profile["key"] != name:
        raise ValueError("The area profile must match the candidate key")
    revision = profile["revisionId"]
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,79}", revision):
        raise ValueError("Expected a versioned area identifier")
    bounds = [profile["area"][key] for key in BOUND_KEYS]
    if any(type(value) not in (int, float) or not math.isfinite(value) for value in bounds):
        raise ValueError("Area coordinates must be finite numbers")
    west, south, east, north = bounds
    if not (-180 <= west < east <= 180 and -85 <= south < north <= 85 and east - west < 180):
        raise ValueError("Expected bounded longitude-first coordinates")
    return bounds, revision


def stage(name, output, retained, profile=None, imagery_capture=None):
    import rasterio
    from shapely.geometry import box, mapping

    bounds, revision = candidate_area(name, profile)
    area = mapping(box(*bounds))
    dem = (LOCAL if name == "jeddah" else retained) / "raw" / f"{name}-copdem30.tif"
    geoid = LOCAL / "raw/us_nga_egm08_25.tif"
    osm = retained / "raw/avignon-buffered.osm" if name == "avignon" else LOCAL / "raw/gcc-states.osm.pbf"
    if imagery_capture is None:
        source_root = LOCAL if name == "jeddah" else ISOLATED
        captures = [p.parent for p in (source_root / "imagery").glob("*/capture.json")
                    if [document(p)["aoi"][key] for key in BOUND_KEYS] == bounds]
        if not captures:
            raise ValueError(f"No retained imagery covers {name}")
        imagery_capture = max(captures, key=lambda folder: document(folder / "scene.json")["acquiredAt"])
    capture = document(imagery_capture / "capture.json")
    if capture["schema"] != "emad.imagery-capture.v1" or capture["synthetic"] is not False:
        raise ValueError("Expected a retained real-pixel capture")
    if [capture["aoi"][key] for key in BOUND_KEYS] != bounds:
        raise ValueError("The retained capture and study-area bounds differ")
    chosen = imagery_capture
    scene = document(chosen / "scene.json")
    receipts = document(chosen / "receipts.json")
    if capture["sceneId"] != scene["id"] or capture["receipts"] != receipts:
        raise ValueError("Capture receipt and retained scene metadata disagree")
    files = []

    def add(path, receipt, layer, fmt, producer, edition, data_time, datum, semantics, uncertainty, evidence):
        digest = receipt.get("captureSha256", receipt.get("sha256"))
        if file_digest(path, receipt["bytes"]) != (digest, receipt["bytes"]):
            raise ValueError(f"Retained bytes differ from receipt for {path.name}")
        crs, resolution = "EPSG:4326", "Native source geometry"
        if fmt == "geotiff":
            with rasterio.open(path) as raster:
                crs = raster.crs.to_string()
                resolution = f"{raster.res[0]:.9g} × {raster.res[1]:.9g} native CRS units"
        asset_id = "geoasset_" + hashlib.sha256(f"{layer}:{path.name}:{digest}".encode()).hexdigest()
        files.append((path, {"id": asset_id, "requestKey": asset_id, "sourceId": f"candidate-{producer.lower().replace(' ', '-')}",
                             "admissionId": "UNADMITTED_CANDIDATE", "sha256": digest, "bytes": receipt["bytes"],
                             "retainUntil": "2099-01-01T00:00:00Z", "state": "READY", "rights": {},
                             "representation": {"layer": layer, "format": fmt, "producer": producer,
                                                 "productEdition": edition, "dataTime": data_time,
                                                 "retrievedAt": receipt["retrievedAt"], "crs": crs,
                                                 "verticalDatum": datum, "nativeResolution": resolution,
                                                 "semantics": semantics, "uncertainty": uncertainty,
                                                 "evidence": evidence, "footprint": area}}))

    add(dem, original(dem), "surface", "geotiff", "Copernicus", "GLO-30 Public", "TanDEM-X 2011–2015 with older gap-fill",
        "EGM2008", "Digital surface model including structures and vegetation",
        "See source product handbook for stated horizontal and vertical accuracy. No independent area measurement.",
        DEM_REFERENCE)
    add(geoid, original(geoid), "geoid", "geotiff", "NGA", "EGM2008 2.5 arc minutes", "Static reference grid", "EGM2008",
        "Geoid undulation in metres. Add to EGM2008 orthometric height for ellipsoidal height.",
        "Grid model uncertainty varies by area", "https://cdn.proj.org/us_nga_egm08_25.tif")
    osm_receipt = original(osm)
    for layer in ("roads", "buildings", "water"):
        add(osm, osm_receipt, layer, "osm-xml" if name == "avignon" else "osm-pbf", "OpenStreetMap contributors",
            "OSM API buffered extract" if name == "avignon" else "Geofabrik regional snapshot",
            osm_receipt.get("lastModified") or osm_receipt["retrievedAt"], "not_applicable",
            "Attributed vector source geometry and tags", "UNKNOWN", "https://www.openstreetmap.org/copyright")
    for band in ("B02_10m", "B03_10m", "B04_10m", "B08_10m", "SCL_20m"):
        add(chosen / (band + ".tif"), receipts[band], "imagery", "geotiff", "Copernicus",
            scene["providerId"] + " " + band, scene["acquiredAt"], "not_applicable",
            f"Retained Sentinel-2 L2A window {band}", "Source quality indicators retained separately",
            "Capture receipt and source scene retained with the candidate")
    files.sort(key=lambda item: item[1]["id"])
    profile = "reference-context-v1"
    recipe = {"schema": "emad.environment-recipe.v1", "toolchainSha256": toolchain(),
              "profile": {"id": profile}, "area": {"revisionId": revision, "geometry": area},
              "inputs": [asset for _, asset in files]}
    recipe["inputSha256"] = hashlib.sha256(encoded(recipe)).hexdigest()
    output.mkdir(parents=True, exist_ok=False)
    (output / "inputs").mkdir()
    for path, asset in files:
        dest = output / "inputs" / asset["id"]
        try:
            os.link(path, dest)
        except OSError:
            shutil.copyfile(path, dest)
    (output / "recipe.json").write_bytes(encoded(recipe))
    evidence = output / "source-evidence"
    evidence.mkdir()
    for filename in ("capture.json", "scene.json", "receipts.json"):
        shutil.copyfile(chosen / filename, evidence / filename)
    for path in (dem, geoid, osm):
        shutil.copyfile(path.with_suffix(path.suffix + ".receipt.json"), evidence / (path.name + ".receipt.json"))
    prepare(output)
    result = verify(output / "prepared.zip")
    result.update(area=name, releaseState="PRIVATE_CANDIDATE", authority="No shared source admission or release approval recorded")
    (output / "evidence.json").write_bytes(encoded(result))
    print(json.dumps(result), flush=True)


def rebuild_candidate(root):
    bundle = root / "private-rebuild.zip"
    recipe = document(root / "recipe.json")
    with zipfile.ZipFile(bundle, "x", compression=zipfile.ZIP_STORED) as archive:
        archive.write(root / "recipe.json", "recipe.json")
        archive.write(root / "manifest.json", "manifest.json")
        written = set()
        for asset in recipe["inputs"]:
            digest = asset["sha256"]
            if digest in written:
                continue
            archive.write(root / "inputs" / asset["id"], f"inputs/{digest}")
            written.add(digest)
    result = rebuild(bundle, root / "rebuilt-prepared.zip")
    (root / "rebuild-evidence.json").write_bytes(encoded(result))
    print(json.dumps(result), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("area")
    parser.add_argument("--area-profile", type=Path, help="Versioned bounds for a new retained candidate")
    parser.add_argument("--imagery-capture", type=Path, help="Directory with a retained capture for the exact study area")
    parser.add_argument("--retained", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--rebuild", action="store_true", help="Verify the private candidate by rebuilding from a fresh directory")
    args = parser.parse_args()
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,79}", args.area):
        parser.error("Expected a valid study-area key")
    if args.area_profile is not None and args.imagery_capture is None:
        parser.error("A new area profile requires an explicit retained imagery capture")
    stage(args.area, args.output, args.retained,
          document(args.area_profile) if args.area_profile else None, args.imagery_capture)
    if args.rebuild:
        rebuild_candidate(args.output)


if __name__ == "__main__":
    main()
