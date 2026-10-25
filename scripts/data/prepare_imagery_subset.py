"""Prepare one licensed local Sentinel-2 subset outside the HTTP/API process.

Inputs are local retained B04/B08/SCL files and exact product metadata. No
provider credentials, remote URLs, network raster drivers or full-scene arrays
are used here. NDVI analysis uses the resulting hash-pinned subset, not these
mutable filesystem paths. See docs/implementation/P2_1_3_IMAGERY_INTELLIGENCE.md.
"""
import argparse
import hashlib
import json
import math
from pathlib import Path
import xml.etree.ElementTree as ET

VERSION = "emad.prepare-subset.v1"

def digest(path):
    if path.stat().st_size > 512 * 1024 * 1024:
        raise ValueError("source file exceeds 512 MiB preparation budget")
    h = hashlib.sha256()
    with path.open("rb") as f:
        for block in iter(lambda: f.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()

def prepare(scene, paths, metadata, grid):
    import numpy as np
    import rasterio
    from affine import Affine
    from rasterio.warp import reproject, Resampling
    if metadata.stat().st_size > 1024 * 1024:
        raise ValueError("product metadata exceeds 1 MiB")
    xml = metadata.read_bytes()
    if b"<!DOCTYPE" in xml.upper() or b"<!ENTITY" in xml.upper():
        raise ValueError("external/entity XML is not permitted")
    root = ET.fromstring(xml)
    values = lambda tag: [x for x in root.iter() if x.tag.rsplit("}", 1)[-1] == tag]
    quant = values("BOA_QUANTIFICATION_VALUE")
    baseline = values("PROCESSING_BASELINE")
    if len(quant) != 1 or len(baseline) != 1 or baseline[0].text != scene["processingVersion"]:
        raise ValueError("explicit product quantification and matching processing baseline required")
    scale = float(quant[0].text)
    offsets = {x.attrib["band_id"]: float(x.text) for x in values("BOA_ADD_OFFSET")}
    if scale <= 0 or not math.isfinite(scale) or "3" not in offsets or "7" not in offsets:
        raise ValueError("explicit B04/B08 BOA offsets required; no implicit zero offset")
    w, h = grid["width"], grid["height"]
    if not (1 <= w <= 256 and 1 <= h <= 256 and 1e-6 <= grid["stepLng"] <= .001 and 1e-6 <= grid["stepLat"] <= .001):
        raise ValueError("target grid exceeds the initial subset budget")
    transform = Affine(grid["stepLng"], 0, grid["west"], 0, -grid["stepLat"], grid["north"])
    hashes = {band: digest(path) for band, path in paths.items()}
    data = {}
    with rasterio.Env(GDAL_CACHEMAX=64 * 1024 * 1024, GDAL_NUM_THREADS="1", PROJ_NETWORK="OFF", GDAL_DISABLE_READDIR_ON_OPEN="EMPTY_DIR"):
        for band, path in paths.items():
            magic = path.read_bytes()[:16] if path.stat().st_size < 16 else None
            if magic is None:
                with path.open("rb") as f:
                    magic = f.read(16)
            if not (magic.startswith((b"II*\x00", b"MM\x00*", b"II+\x00", b"MM\x00+")) or magic[4:12] == b"jP  \r\n\x87\n"):
                raise ValueError("only local TIFF or JPEG2000 source bytes are accepted")
            with rasterio.open(path) as src:
                if src.driver not in ("GTiff", "JP2OpenJPEG") or src.crs is None or src.count != 1 or src.width * src.height > 125_000_000:
                    raise ValueError("bounded single-band georeferenced raster required")
                dest = np.full((h, w), 0 if band == "SCL_20m" else np.nan, dtype=np.float32)
                reproject(source=rasterio.band(src, 1), destination=dest, src_transform=src.transform, src_crs=src.crs,
                          src_nodata=0, dst_transform=transform, dst_crs="EPSG:4326", dst_nodata=0 if band == "SCL_20m" else np.nan,
                          resampling=Resampling.nearest if band == "SCL_20m" else Resampling.average, num_threads=1, warp_mem_limit=64)
                if band != "SCL_20m":
                    offset = offsets["3" if band == "B04_10m" else "7"]
                    dest = (dest.astype(np.float64) + offset) / scale
                data[band] = dest
    if hashes != {band: digest(path) for band, path in paths.items()}:
        raise ValueError("source files changed during preparation")
    numbers = lambda a: [None if not math.isfinite(float(x)) else float(x) for x in a.ravel()]
    return {"schema": "emad.multispectral-subset.v1", "sceneId": scene["id"], "sceneHash": scene["contentHash"],
            "acquiredAt": scene["acquiredAt"], "platform": scene["platform"], "tile": scene["tile"], "orbit": scene["orbit"],
            "processingVersion": scene["processingVersion"], "grid": {**grid, "crs": "EPSG:4326", "registration": "pixel_edges_north_to_south"},
            "reflectanceUnit": "surface_reflectance", "red": numbers(data["B04_10m"]), "nir": numbers(data["B08_10m"]),
            "scl": [int(x) for x in data["SCL_20m"].ravel()], "sourceHashes": hashes, "synthetic": False,
            "preparationMethod": f"{VERSION}; rasterio={rasterio.__version__}; GDAL={rasterio.__gdal_version__}; reflectance=(DN+offset)/quantification; average bands; nearest SCL; product-metadata-sha256={hashlib.sha256(xml).hexdigest()}",
            "radiometry": {"quantification": scale, "redOffsetDN": offsets["3"], "nirOffsetDN": offsets["7"]}}

def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("scene", "red", "nir", "scl", "metadata", "output"):
        p.add_argument("--" + name, required=True, type=Path)
    p.add_argument("--west", type=float, required=True); p.add_argument("--north", type=float, required=True)
    p.add_argument("--step-lng", type=float, required=True); p.add_argument("--step-lat", type=float, required=True)
    p.add_argument("--width", type=int, required=True); p.add_argument("--height", type=int, required=True)
    a = p.parse_args()
    grid = {"width": a.width, "height": a.height, "west": a.west, "north": a.north, "stepLng": a.step_lng, "stepLat": a.step_lat}
    subset = prepare(json.loads(a.scene.read_text(encoding="utf-8")), {"B04_10m": a.red, "B08_10m": a.nir, "SCL_20m": a.scl}, a.metadata, grid)
    raw = json.dumps(subset, separators=(",", ":"), allow_nan=False).encode()
    if len(raw) > 16 * 1024 * 1024:
        raise ValueError("prepared subset exceeds artifact budget")
    a.output.write_bytes(raw)
    print(f"{len(raw)} bytes; sha256={hashlib.sha256(raw).hexdigest()}")

if __name__ == "__main__":
    main()
