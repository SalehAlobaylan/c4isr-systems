"""Prepare inspectable geographic enrichment derivatives from an exact release."""
import argparse
import base64
import hashlib
import json
from pathlib import Path
import sys
from tempfile import TemporaryDirectory
import zipfile
import zlib

sys.path.insert(0, str(Path(__file__).resolve().parent))
import environment_enrichment as enrichment
import prepare_display_layers as legacy
import prepare_display_terrain as terrain

import numpy as np
import rasterio
from rasterio.features import geometry_mask
from rasterio.transform import from_bounds
from rasterio.warp import reproject, Resampling
from shapely.geometry import shape

BASE = enrichment.base
DELTA = [
    (-10, "< −10 m", "20458a"), (-3, "−10 to < −3 m", "508ac6"), (-1, "−3 to < −1 m", "b3d4e4"),
    (1, "−1 to < +1 m", "f1eee5"), (3, "+1 to < +3 m", "efbd7b"), (10, "+3 to < +10 m", "d56b37"),
    (None, "≥ +10 m", "92292c"),
]


def texture(layer, paths, manifest):
    area = shape(manifest["area"])
    bounds = area.bounds
    rows = [m for m in manifest["members"] if m["name"] in paths]
    primary = [m for m in rows if m["layer"] == ("land-cover" if layer == "land-cover" else "surface")]
    native = min(min(m["details"]["nativeResolution"]) for m in primary)
    width = max(2, min(1024, int(np.ceil((bounds[2] - bounds[0]) / native))))
    height = max(2, min(1024, int(np.ceil((bounds[3] - bounds[1]) / native))))
    grid = from_bounds(*bounds, width, height)
    joined = np.full((height, width), np.nan)
    for m in primary:
        if layer == "land-cover":
            with rasterio.open(paths[m["name"]], driver="GTiff") as src:
                values, native_grid = src.read(1, masked=True).astype(np.float64).filled(np.nan), src.transform
        else:
            ground = next(g for g in rows if g["layer"] == "ground" and g["component"] == m["component"])
            values, valid, native_grid = enrichment.comparison(paths[m["name"]], paths[ground["name"]])
            values[~valid] = np.nan
        tile = np.full_like(joined, np.nan)
        reproject(values, tile, src_transform=native_grid, src_crs="EPSG:4326", src_nodata=np.nan,
                  dst_transform=grid, dst_crs="EPSG:4326", dst_nodata=np.nan, resampling=Resampling.nearest)
        joined[np.isfinite(tile)] = tile[np.isfinite(tile)]
    valid = np.isfinite(joined) & geometry_mask([area.__geo_interface__], out_shape=joined.shape, transform=grid, invert=True)
    if not valid.any():
        raise ValueError("No valid display pixels on the selected footprint")
    image = np.zeros((height, width, 4), dtype=np.uint8)
    legend = []
    palette = ([(code, *entry) for code, entry in enrichment.CLASSES.items()] if layer == "land-cover" else DELTA)
    for i, (code, label, rgb) in enumerate(palette):
        if layer == "land-cover":
            mask = valid & (joined == code)
        else:
            lo = -np.inf if i == 0 else DELTA[i - 1][0]
            mask = valid & (joined >= lo) & (joined < (code if code is not None else np.inf))
        image[mask] = [*bytes.fromhex(rgb), 255]
        legend.append({"code": code, "label": label, "rgb": rgb, "pixels": int(mask.sum())})
    inputs = {a["id"]: a["representation"] for a in manifest["inputs"]}
    editions = [{"layer": m["layer"], "producer": inputs[m["inputId"]]["producer"],
                 "edition": inputs[m["inputId"]]["productEdition"], "dataTime": inputs[m["inputId"]]["dataTime"]} for m in rows]
    return {"bounds": list(bounds), "width": width, "height": height, "validPixels": int(valid.sum()), "legend": legend,
            "method": "EPSG:4326 nearest neighbour; masked pixels are transparent",
            "sources": [dict(values) for values in sorted({tuple(sorted(s.items())) for s in editions})],
            "comparison": manifest["quality"]["enrichment"]["groundComparison"] if layer == "ground-delta" else [],
            "texture": base64.b64encode(legacy.png_rgba(image)).decode("ascii")}


def generate(package, expected, layer, output, request, stac):
    verification = enrichment.verify(package, expected)
    if verification["profile"] != enrichment.PROFILE:
        raise ValueError("Enrichment displays require a verified enriched reference release")
    if output.exists() or request.exists():
        raise FileExistsError("Display output or publication request already exists")
    with zipfile.ZipFile(package) as archive, TemporaryDirectory(prefix="emad-enrichment-display-") as scratch:
        entries = BASE.archive_entries(archive)
        raw_manifest, manifest = BASE.archive_json(archive, entries, "manifest.json")
        if hashlib.sha256(raw_manifest).hexdigest() != expected:
            raise ValueError("Parent manifest changed during display preparation")
        parents = {asset["id"]: asset for asset in manifest["inputs"]}
        selected = ({"ground", "surface"} if layer == "ground-delta" else {layer, "geoid"} if layer in {"ground", "surface"}
                    else {layer} if layer in {"land-cover", "imagery"}
                    else {layer, "surface", "geoid"})
        members, dependencies = [], []
        for m in manifest["members"]:
            name = m["name"]
            if m["layer"] not in selected or (layer == "imagery" and parents[m["inputId"]]["representation"][
                    "productEdition"].rsplit(" ", 1)[-1] not in (*legacy.RGB, legacy.SCL)):
                continue
            path = Path(archive.extract(entries[name], scratch))
            if BASE.file_digest(path, m["bytes"]) != (m["sha256"], m["bytes"]):
                raise ValueError("Parent member changed during display preparation")
            members.append((m, path))
            dependencies.append({"name": name, "sha256": m["sha256"]})
        if layer in {"ground", "surface"}:
            recipe = "ground-glb-v1" if layer == "ground" else "terrain-glb-v1"
            return mesh(manifest, members, expected, recipe, layer, output, request, dependencies)
        if layer in {"land-cover", "ground-delta"}:
            if not members or layer == "ground-delta" and not any(m["layer"] == "ground" for m, _ in members):
                raise ValueError("Selected enrichment layer is unavailable")
            data = texture(layer, {m["name"]: path for m, path in members}, manifest)
        elif layer == "imagery":
            data = legacy.optical(members, parents, shape(manifest["area"]), stac)
        else:
            data = legacy.vectors(layer, members, parents, manifest["frame"])
            for feature in data["features"]:
                props = feature["properties"]
                props["dimensions"] = enrichment.dimensions(layer, props["tags"])
                height = props["dimensions"].get("height")
                if height and height["value"] is not None:
                    props["height"] = {"kind": "source-tag", "metres": height["value"]}
        raw = BASE.encoded({"schema": "emad.display-layer.v1", "manifestSha256": expected, "layer": layer, "data": data})
    if len(raw) > legacy.MAX_BYTES:
        raise ValueError("Display layer exceeds the byte budget")
    recipe = f"{layer}-display-v1"
    digest = hashlib.sha256(raw).hexdigest()
    toolchain = hashlib.sha256(BASE.encoded({"enrichment": enrichment.toolchain(), "terrain": terrain.terrain_toolchain(),
                                           "processor": BASE.file_digest(Path(__file__))[0],
                                           "legacyLayer": BASE.file_digest(Path(legacy.__file__))[0],
                                           "zlib": zlib.ZLIB_RUNTIME_VERSION})).hexdigest()
    with output.open("xb") as stream:
        stream.write(raw)
    with request.open("xb") as stream:
        stream.write(BASE.encoded({"requestKey": f"{recipe}:{digest}", "manifestSha256": expected,
                                   "toolchainSha256": toolchain, "recipe": recipe, "inputs": dependencies,
                                   "sha256": digest, "bytes": len(raw)}))
    return {"layer": layer, "sha256": digest, "bytes": len(raw)}


def mesh(manifest, members, expected, recipe, layer, output, request, dependencies):
    area = shape(manifest["area"])
    parts = list(area.geoms) if area.geom_type == "MultiPolygon" else [area]
    positions, control_error = terrain.frame_transform(manifest["frame"])
    items = {(item["layer"], item["component"]): (item, path) for item, path in members}
    if len(items) != len(members) or not all((layer, n) in items for n in range(len(parts))):
        raise ValueError("Terrain derivative requires exactly one native grid per component")
    glb = terrain.glb_geometry.GLB()
    material = glb.material("Modeled bare earth" if layer == "ground" else "Reference surface", (.49, .48, .42, 1))
    points, triangles, precision = 0, 0, 0.0
    for n, part in enumerate(parts):
        item, path = items[layer, n]
        geoid = items.get(("geoid", n), (None, None))[1]
        verts, faces, examined, error = terrain.mesh_component(
            path, geoid, item["details"]["verticalDatum"], part, positions, terrain.MAX_POINTS - points)
        points += examined
        triangles += len(faces)
        if triangles > terrain.MAX_TRIANGLES:
            raise ValueError("Terrain triangle budget exceeded")
        precision = max(precision, error)
        glb.mesh(verts, faces, material)
    raw = glb.bytes()
    if len(raw) > legacy.MAX_BYTES:
        raise ValueError("Terrain exceeds the display byte budget")
    digest = hashlib.sha256(raw).hexdigest()
    toolchain = hashlib.sha256(BASE.encoded({"enrichment": enrichment.toolchain(), "terrain": terrain.terrain_toolchain(),
                                           "processor": BASE.file_digest(Path(__file__))[0]})).hexdigest()
    with output.open("xb") as stream:
        stream.write(raw)
    with request.open("xb") as stream:
        stream.write(BASE.encoded({"requestKey": f"{recipe}:{digest}", "manifestSha256": expected,
                                   "toolchainSha256": toolchain, "recipe": recipe, "inputs": dependencies,
                                   "sha256": digest, "bytes": len(raw)}))
    return {"sha256": digest, "bytes": len(raw), "triangles": triangles, "gridSamples": points,
            "maxFrameControlErrorM": control_error, "maxFloat32ErrorM": precision}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument("layer", choices=("surface", "ground", "land-cover", "ground-delta", "imagery", *enrichment.DIMENSIONS))
    parser.add_argument("output", type=Path)
    parser.add_argument("request", type=Path)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--scene-metadata", type=Path)
    args = parser.parse_args()
    try:
        print(json.dumps(generate(args.package, args.manifest_sha256, args.layer, args.output, args.request, args.scene_metadata)))
    except (KeyError, OSError, ValueError, zipfile.BadZipFile) as error:
        parser.exit(2, f"{error}\n")
