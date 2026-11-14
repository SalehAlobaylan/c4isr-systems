"""Derive an inspection terrain GLB from one verified environment package."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys
from tempfile import TemporaryDirectory
import zipfile

import environment_library as library
import numpy as np
import rasterio
import shapely
from pyproj import Transformer

import glb_geometry

RECIPE = "terrain-glb-v1"
MAX_POINTS = 300_000
MAX_TRIANGLES = 500_000


def terrain_toolchain():
    versions = {
        "environment": library.toolchain(),
        "terrain": library.file_digest(Path(__file__))[0],
        "serializer": library.file_digest(Path(glb_geometry.__file__))[0],
    }
    return hashlib.sha256(library.encoded(versions)).hexdigest()


def frame_transform(frame):
    longitude, latitude, height = frame["origin"]
    transform = Transformer.from_pipeline(
        "+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad "
        "+step +proj=cart +ellps=WGS84 "
        f"+step +proj=topocentric +ellps=WGS84 +lon_0={longitude} +lat_0={latitude} +h_0={height}"
    )
    errors = [
        np.linalg.norm(
            np.asarray(transform.transform(c["longitude"], c["latitude"], c["ellipsoidHeightM"], errcheck=True))
            - np.asarray(c["enuM"])
        )
        for c in frame["controls"]
    ]
    if len(errors) < 3 or not max(errors) <= .001:
        raise ValueError("Independent frame controls exceeded 1 mm")
    return transform, float(max(errors))


def undulation(path, longitude, latitude):
    with rasterio.open(path) as grid:
        if grid.crs is None or grid.crs.to_epsg() not in (4326, 4979) or grid.count != 1 or grid.width * grid.height > MAX_POINTS:
            raise ValueError("Expected a bounded single-band WGS84 geoid raster")
        x, y = (~grid.transform) * (longitude, latitude)
        x, y = x - .5, y - .5
        column, row = np.floor(x).astype(int), np.floor(y).astype(int)
        if ((column < 0) | (row < 0) | (column + 1 >= grid.width) | (row + 1 >= grid.height)).any():
            raise ValueError("Retained geoid lacks interpolation support")
        dx, dy = x - column, y - row
        values = grid.read(1, masked=True).astype(np.float64)
        offset = (
            (1 - dy) * ((1 - dx) * values[row, column] + dx * values[row, column + 1])
            + dy * ((1 - dx) * values[row + 1, column] + dx * values[row + 1, column + 1])
        )
        if np.ma.getmaskarray(offset).any():
            raise ValueError("Retained geoid has missing interpolation samples")
        return np.asarray(offset) * grid.scales[0] + grid.offsets[0]


def mesh_component(path, geoid, datum, area, transform, points_left):
    with rasterio.open(path) as raster:
        if raster.crs is None or raster.crs.to_epsg() != 4326 or raster.count != 1:
            raise ValueError("Terrain profile requires a single-band EPSG:4326 surface")
        if not (2 <= raster.width and 2 <= raster.height and raster.width * raster.height <= points_left):
            raise ValueError("Terrain sample grid exceeds the display budget")
        affine = raster.transform
        if affine.b != 0 or affine.d != 0 or affine.a <= 0 or affine.e >= 0:
            raise ValueError("Terrain profile requires a north-up surface grid")
        values = raster.read(1, masked=True).astype(np.float64)
        heights = np.asarray(values) * raster.scales[0] + raster.offsets[0]
        valid = ~np.ma.getmaskarray(values) & np.isfinite(heights)
        row, column = np.indices(heights.shape)
        longitude, latitude = affine * (column + .5, row + .5)

    usable = valid[:-1, :-1] & valid[1:, :-1] & valid[:-1, 1:] & valid[1:, 1:]
    r, c = np.nonzero(usable)
    cells = shapely.box(longitude[r, c], latitude[r + 1, c], longitude[r, c + 1], latitude[r, c])
    covered = shapely.covers(area, cells)
    r, c = r[covered], c[covered]
    if len(r) == 0:
        raise ValueError("No complete terrain cells lie inside the selected area")
    width = heights.shape[1]
    a = r * width + c
    b, d = a + 1, a + width
    triangles = np.column_stack((a, d, b, b, d, d + 1)).reshape(-1, 3)
    used, remapped = np.unique(triangles.ravel(), return_inverse=True)
    lon, lat, height = longitude.ravel()[used], latitude.ravel()[used], heights.ravel()[used]
    if datum == "EGM2008":
        if geoid is None:
            raise ValueError("EGM2008 terrain requires the exact prepared geoid")
        height += undulation(geoid, lon, lat)
    elif datum != "ellipsoid":
        raise ValueError("Unsupported terrain vertical datum")
    east, north, up = transform.transform(lon, lat, height, errcheck=True)
    vertices = np.column_stack((east, up, -north))
    quantized = vertices.astype(np.float32)
    precision = float(np.max(np.linalg.norm(quantized.astype(np.float64) - vertices, axis=1)))
    if not np.isfinite(vertices).all() or precision > .001:
        raise ValueError("Terrain vertices exceeded the display coordinate precision budget")
    return quantized, remapped.reshape(-1, 3), heights.size, precision


def generate(package, expected, output, request_output):
    library.verify(package, expected)
    if output.exists() or request_output.exists():
        raise FileExistsError("Terrain output or publication request already exists")
    with zipfile.ZipFile(package) as archive, TemporaryDirectory(prefix="emad-display-terrain-") as scratch:
        entries = library.archive_entries(archive)
        manifest_bytes, manifest = library.archive_json(archive, entries, "manifest.json")
        if hashlib.sha256(manifest_bytes).hexdigest() != expected:
            raise ValueError("Terrain package manifest changed during preparation")
        area = shapely.geometry.shape(manifest["area"])
        transform, control_error = frame_transform(manifest["frame"])
        glb = glb_geometry.GLB()
        material = glb.material("Reference surface", (.49, .48, .42, 1))
        dependencies, paths = [], {"surface": {}, "geoid": {}}
        for item in manifest["members"]:
            layer = item["layer"]
            if layer not in paths:
                continue
            component = item["component"]
            if component in paths[layer]:
                raise ValueError("Terrain recipe supports one surface and geoid grid per component")
            name = item["name"]
            target = Path(scratch) / name
            with archive.open(entries[name]) as source, target.open("xb") as destination:
                if entries[name].file_size != item["bytes"]:
                    raise ValueError(f"Terrain package member size changed: {name}")
                shutil.copyfileobj(source, destination, 256 << 10)
            if library.file_digest(target, item["bytes"]) != (item["sha256"], item["bytes"]):
                raise ValueError(f"Terrain package member changed: {name}")
            paths[layer][component] = (target, item)
            dependencies.append({"name": name, "sha256": item["sha256"]})
        expected_components = set(range(len(area.geoms))) if area.geom_type == "MultiPolygon" else {0}
        if set(paths["surface"]) != expected_components or len(dependencies) > 128:
            raise ValueError("Terrain recipe needs one surface for every declared area component")
        points, triangles, precision = 0, 0, 0.0
        for component, (path, item) in sorted(paths["surface"].items()):
            geometry = area.geoms[component] if area.geom_type == "MultiPolygon" else area
            geoid = paths["geoid"].get(component, (None, None))[0]
            mesh, faces, examined, error = mesh_component(
                path, geoid, item["details"]["verticalDatum"], geometry, transform, MAX_POINTS - points
            )
            points += examined
            triangles += len(faces)
            if triangles > MAX_TRIANGLES:
                raise ValueError("Terrain triangle budget exceeded")
            precision = max(precision, error)
            glb.mesh(mesh, faces, material)
        raw = glb.bytes()
        if len(raw) > 32 << 20:
            raise ValueError("Terrain GLB exceeds the display byte budget")
        digest = hashlib.sha256(raw).hexdigest()
        toolchain = terrain_toolchain()
        intent = hashlib.sha256(f"{RECIPE}:{toolchain}:{expected}".encode()).hexdigest()
        request = {
            "requestKey": f"terrain-v1:{intent}",
            "manifestSha256": expected,
            "toolchainSha256": toolchain,
            "recipe": RECIPE,
            "inputs": dependencies,
            "sha256": digest,
            "bytes": len(raw),
        }
        with output.open("xb") as stream:
            stream.write(raw)
        with request_output.open("x", encoding="utf-8") as stream:
            json.dump(request, stream, indent=2)
        return {"sha256": digest, "bytes": len(raw), "triangles": triangles, "gridSamples": points,
                "maxFrameControlErrorM": control_error, "maxFloat32ErrorM": precision}


def main():
    if sys.argv[1:] == ["--toolchain"]:
        print(terrain_toolchain())
        return 0
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("request", type=Path)
    parser.add_argument("--manifest-sha256", required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(generate(args.package, args.manifest_sha256, args.output, args.request)))
    except (KeyError, OSError, ValueError, zipfile.BadZipFile) as exc:
        print(str(exc), file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
