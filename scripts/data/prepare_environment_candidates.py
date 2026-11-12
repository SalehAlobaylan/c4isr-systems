"""Prepare private real-data candidates from the three retained acceptance areas."""
import argparse
import hashlib
import json
import os
from pathlib import Path
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


def document(path):
    return json.loads(path.read_text(encoding="utf-8"))


def original(path):
    return document(path.with_suffix(path.suffix + ".receipt.json"))


def stage(name, output, retained):
    import rasterio
    from shapely.geometry import box, mapping

    area = mapping(box(*ACCEPTANCE_AREAS[name]))
    dem = (LOCAL if name == "jeddah" else retained) / "raw" / f"{name}-copdem30.tif"
    geoid = LOCAL / "raw/us_nga_egm08_25.tif"
    osm = retained / "raw/avignon-buffered.osm" if name == "avignon" else LOCAL / "raw/gcc-states.osm.pbf"
    source_root = LOCAL if name == "jeddah" else ISOLATED
    captures = []
    for p in (source_root / "imagery").glob("*/capture.json"):
        aoi = document(p)["aoi"]
        if [aoi[k] for k in ("west", "south", "east", "north")] == ACCEPTANCE_AREAS[name]:
            captures.append(p.parent)
    if not captures:
        raise ValueError(f"No retained imagery covers {name}")
    chosen = max(captures, key=lambda folder: document(folder / "scene.json")["acquiredAt"])
    scene = document(chosen / "scene.json")
    receipts = document(chosen / "receipts.json")
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
              "profile": {"id": profile}, "area": {"revisionId": name + "-candidate", "geometry": area},
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
    parser.add_argument("area", choices=ACCEPTANCE_AREAS)
    parser.add_argument("--retained", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--rebuild", action="store_true", help="Verify the private candidate by rebuilding from a fresh directory")
    args = parser.parse_args()
    stage(args.area, args.output, args.retained)
    if args.rebuild:
        rebuild_candidate(args.output)


if __name__ == "__main__":
    main()
