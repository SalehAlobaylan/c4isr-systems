"""Prepare scoped SAR display and the existing surface mesh for observation releases."""
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
import environment_observation as observation
import prepare_display_layers as shared
import prepare_display_terrain as terrain
import numpy as np
import rasterio
from rasterio.features import geometry_mask
from rasterio.transform import from_bounds
from rasterio.warp import Resampling, reproject
from shapely.geometry import shape

BASE = observation.base
DB_RANGE = [-30, 5]


def generate(package, expected, output, request, layer, stac=None):
    verified = observation.verify(package, expected)
    if verified["profile"] != observation.PROFILE or output.exists() or request.exists():
        raise ValueError("Choose an exact observation release and new output paths")
    if layer in ("surface","imagery"):
        old = terrain.library.verify
        terrain.library.verify = observation.verify
        try:
            if layer == "surface":
                return terrain.generate(package, expected, output, request)
            return shared.generate(package, expected, layer, output, request, stac)
        finally:
            terrain.library.verify = old
    with zipfile.ZipFile(package) as archive, TemporaryDirectory(prefix="emad-radar-display-") as scratch:
        entries = BASE.archive_entries(archive)
        raw_manifest, manifest = BASE.archive_json(archive, entries, "manifest.json")
        if hashlib.sha256(raw_manifest).hexdigest() != expected:
            raise ValueError("Parent manifest changed during preparation")
        member, = [m for m in manifest["members"] if m["layer"] == "radar"]
        raster = Path(archive.extract(entries[member["name"]], scratch))
        if BASE.file_digest(raster, member["bytes"]) != (member["sha256"], member["bytes"]):
            raise ValueError("Radar member changed during preparation")
        area = shape(manifest["area"])
        with rasterio.open(raster, driver="GTiff") as src:
            width, height = max(2, min(1024, src.width)), max(2, min(1024, src.height))
            grid = from_bounds(*area.bounds, width, height)
            values = src.read(1, masked=True).filled(np.nan)
            values[~np.isfinite(values) | (values <= 0)] = np.nan
            destination = np.full((height,width), np.nan, dtype=np.float32)
            reproject(values, destination, src_transform=src.transform, src_crs=src.crs, src_nodata=np.nan,
                      dst_transform=grid, dst_crs="EPSG:4326", dst_nodata=np.nan, resampling=Resampling.nearest)
        valid = np.isfinite(destination) & geometry_mask([area.__geo_interface__], destination.shape, grid, invert=True)
        if not valid.any():
            raise ValueError("Radar texture contains no usable samples")
        pixels = np.zeros((height,width,4), dtype=np.uint8)
        db = 10 * np.log10(destination[valid])
        grey = np.rint(np.clip((db-DB_RANGE[0])/(DB_RANGE[1]-DB_RANGE[0]),0,1)*255).astype(np.uint8)
        pixels[valid] = np.column_stack((grey,grey,grey,np.full(grey.shape,255,dtype=np.uint8)))
        payload = {"bounds": list(area.bounds), "width": width, "height": height, "validPixels": int(valid.sum()),
                   "texture": base64.b64encode(shared.png_rgba(pixels)).decode("ascii"), "radar": verified["quality"]["observations"]["radar"],
                   "displayDbRange": DB_RANGE, "method": "10 log10(linear power), clipped grayscale; nearest neighbour; invalid samples transparent"}
        raw = BASE.encoded({"schema": "emad.display-layer.v1", "manifestSha256": expected, "layer": "radar", "data": payload})
        if len(raw) > shared.MAX_BYTES:
            raise ValueError("Radar display exceeds byte budget")
        digest = hashlib.sha256(raw).hexdigest()
        toolchain = hashlib.sha256(BASE.encoded({"processor": BASE.file_digest(Path(__file__))[0], "observations": observation.toolchain(),
            "serializer": BASE.file_digest(Path(shared.__file__))[0], "zlib": zlib.ZLIB_RUNTIME_VERSION})).hexdigest()
        with output.open("xb") as stream:
            stream.write(raw)
        with request.open("xb") as stream:
            stream.write(BASE.encoded({"requestKey": f"radar-display-v1:{digest}", "manifestSha256": expected,
                 "toolchainSha256": toolchain, "recipe": "radar-display-v1", "inputs": [{"name":member["name"],"sha256":member["sha256"]}],
                 "sha256":digest,"bytes":len(raw)}))
        return {"sha256":digest,"bytes":len(raw),"displayValidPixels":payload["validPixels"],"displayDbRange": DB_RANGE}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument("layer", choices=("surface","radar","imagery"))
    parser.add_argument("output", type=Path)
    parser.add_argument("request", type=Path)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--scene-metadata", type=Path)
    args = parser.parse_args()
    try:
        print(json.dumps(generate(args.package,args.manifest_sha256,args.output,args.request,args.layer,args.scene_metadata)))
    except (ValueError,KeyError,OSError,zipfile.BadZipFile) as exc:
        parser.exit(2,f"{exc}\n")
