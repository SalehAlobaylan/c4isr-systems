"""Retain a bounded GEDTM30 native window with a publisher version receipt."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import re
from urllib.request import Request, urlopen

import rasterio
from rasterio.windows import Window, from_bounds

URL = ("https://s3.opengeohub.org/global/edtm/"
       "filtered.dtm_edtm_m_30m_s_20060101_20151231_go_epsg.4326.3855_v20250619.tif")


def identity():
    with urlopen(Request(URL, headers={"Range": "bytes=0-255"}), timeout=25) as response:
        length = re.fullmatch(r"bytes 0-255/([0-9]+)", response.headers.get("Content-Range", ""))
        etag = response.headers.get("ETag")
        if response.status != 206 or not length or not etag or len(response.read(257)) != 256:
            raise ValueError("Publisher does not support bounded reads with a visible object version")
        return {"etag": etag, "lastModified": response.headers.get("Last-Modified"), "objectBytes": int(length[1])}


def retain(bounds, etag, output, receipt):
    west, south, east, north = bounds
    if not (-180 <= west < east <= 180 and -85 <= south < north <= 85 and east - west <= .25 and north - south <= .25):
        raise ValueError("Use a bounded native window of at most 0.25° per side")
    if output.exists() or receipt.exists():
        raise FileExistsError("Choose a new window and receipt path")
    before = identity()
    if before["etag"].strip('"') != etag.strip('"'):
        raise ValueError("Publisher object changed from the pinned ETag")
    with rasterio.Env(GDAL_DISABLE_READDIR_ON_OPEN="EMPTY_DIR", GDAL_HTTP_TIMEOUT="20", GDAL_HTTP_MAX_RETRY="2",
                      CPL_VSIL_CURL_USE_HEAD="NO", GDAL_TIFF_INTERNAL_MASK=True,
                      GDAL_HTTP_HEADERS=f"If-Match: {before['etag']}"), rasterio.open("/vsicurl/" + URL) as src:
        if (src.count != 1 or src.crs.to_epsg() != 4326 or src.scales != (.1,) or src.offsets != (0.0,)
                or src.dtypes != ("int32",) or src.tags().get("VERTICAL_DATUM") != "EGM2008/EPSG:3855"):
            raise ValueError("GEDTM30 native grid, scale or datum differs from the pinned representation")
        native = from_bounds(*bounds, src.transform)
        col, row = math.floor(native.col_off), math.floor(native.row_off)
        window = Window(col, row, math.ceil(native.col_off + native.width) - col,
                        math.ceil(native.row_off + native.height) - row)
        if not (0 <= col < col + window.width <= src.width and 0 <= row < row + window.height <= src.height):
            raise ValueError("Native source does not cover the requested window")
        values = src.read(1, window=window, masked=True)
        grid = src.window_transform(window)
        if values.count() == 0 or identity() != before:
            raise ValueError("No valid samples or publisher object changed during the read")
        with rasterio.open(output, "w", driver="GTiff", width=values.shape[1], height=values.shape[0], count=1,
                           dtype=values.dtype, transform=grid, crs=src.crs, nodata=src.nodata, compress="deflate") as dst:
            dst.write(values.filled(src.nodata), 1)
            dst.write_mask(~values.mask)
            dst.scales, dst.offsets, dst.units = src.scales, src.offsets, ("m",)
            dst.update_tags(**src.tags())
    with output.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    info = {"sourceURL": URL, **before, "window": [col, row, window.width, window.height], "bounds": bounds,
            "affine": list(grid)[:6], "crs": "EPSG:4326", "verticalDatum": "EGM2008/EPSG:3855", "unit": "m",
            "scale": .1, "offset": 0, "validSamples": int(values.count()), "retainedSHA256": digest,
            "retainedBytes": output.stat().st_size}
    with receipt.open("x", encoding="utf-8") as stream:
        json.dump(info, stream, indent=2)
    return info


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    parser.add_argument("receipt", type=Path)
    parser.add_argument("--etag", required=True)
    parser.add_argument("--bounds", type=float, nargs=4, required=True)
    args = parser.parse_args()
    print(json.dumps(retain(args.bounds, args.etag, args.output, args.receipt)))
