"""Retain bounded provider samples and receipts for the geographic observation pilot."""
import argparse
import datetime as dt
import hashlib
import json
import math
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import urlencode, urlsplit
from urllib.request import Request, HTTPRedirectHandler, build_opener
import zipfile

from observation_formats import RADAR_COLLECTION, WEATHER_EDITION, WEATHER_VARS, date, radar_metadata, validate_weather

STAC = "https://planetarycomputer.microsoft.com/api/stac/v1"
ERA = "https://storage.googleapis.com/weatherbench2/datasets/era5/" + WEATHER_EDITION.removeprefix("WeatherBench 2 ERA5 ") + ".zarr/"


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *_):
        return None


def acquire(url, budget, headers=None):
    try:
        with build_opener(NoRedirect).open(Request(url, headers={"Accept-Encoding": "identity", **(headers or {})}), timeout=40) as response:
            raw = response.read(budget + 1)
            if len(raw) > budget:
                raise ValueError("Provider response exceeds its byte budget")
            return raw, response.status, response.headers
    except HTTPError as exc:
        raise ValueError(f"Provider request failed with HTTP {exc.code}") from None


def encode(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()


def write(path, raw):
    with path.open("xb") as f:
        f.write(raw)


def radar(config, root):
    import numpy as np
    import rasterio
    from rasterio.features import geometry_window
    from rasterio.warp import transform_geom
    from shapely.geometry import box, shape, mapping
    query = STAC + "/search?" + urlencode({"collections": RADAR_COLLECTION, "bbox": ",".join(map(str, config["bounds"])),
                                            "datetime": "/".join(config["interval"]), "limit": 10})
    raw, _, _ = acquire(query, 1 << 20)
    write(root / "radar-discovery.json", raw)
    search = json.loads(raw)
    if search["type"] != "FeatureCollection" or len(search["features"]) > 10:
        raise ValueError("Invalid bounded STAC response")
    candidates = [x for x in search["features"] if x.get("id") == config["product"]]
    if len(candidates) != 1:
        raise ValueError("Pinned radar item unavailable in bounded STAC page; no coverage claim")
    item = candidates[0]
    meta = radar_metadata(item)
    area = box(*config["bounds"])
    if not shape(item["geometry"]).covers(area):
        raise ValueError("Pinned radar footprint does not cover the requested area")
    if dt.datetime.fromisoformat(meta["sourceStart"]) < date(config["interval"][0]) or dt.datetime.fromisoformat(meta["sourceEnd"]) > date(config["interval"][1]):
        raise ValueError("Radar acquisition falls outside the requested interval")
    url = urlsplit(meta["sourceURL"])
    sign_url = "https://planetarycomputer.microsoft.com/api/sas/v1/token/" + url.hostname.split(".")[0] + "/" + url.path.split("/")[1]
    token = json.loads(acquire(sign_url, 16 << 10)[0])["token"]
    signed = meta["sourceURL"] + "?" + token
    headers = {"Range": "bytes=0-255", "If-Match": config["etag"]}
    first, status, identity = acquire(signed, 256, headers)
    if status != 206 or identity.get("Content-Range") != f"bytes 0-255/{meta['sourceBytes']}" or identity.get("ETag") != config["etag"]:
        raise ValueError("Radar source version or bounded access differs from the pin")
    native = root / "radar-native.tif"
    try:
        with rasterio.Env(GDAL_DISABLE_READDIR_ON_OPEN="EMPTY_DIR", CPL_VSIL_CURL_USE_HEAD="NO", GDAL_HTTP_TIMEOUT="30",
                          GDAL_HTTP_MAX_RETRY="1", GDAL_HTTP_HEADERS=f"If-Match: {config['etag']}", GDAL_TIFF_INTERNAL_MASK=True):
            with rasterio.open("/vsicurl/" + signed) as src:
                p = item["properties"]
                if (src.crs.to_string() != meta["crs"] or list(src.transform) != p["proj:transform"] or
                        list(src.shape) != p["proj:shape"] or src.count != 1 or src.dtypes != ("float32",) or
                        src.nodata != meta["nodata"] or src.scales != (1.,) or src.offsets != (0.,)):
                    raise ValueError("Native radar bytes differ from STAC raster metadata")
                geometry = transform_geom("EPSG:4326", src.crs, mapping(area))
                window = geometry_window(src, [geometry])
                if window.width * window.height > 2_000_000:
                    raise ValueError("Radar window exceeds its sample budget")
                values = src.read(1, window=window, masked=True)
                grid = src.window_transform(window)
                profile = dict(driver="GTiff", count=1, dtype="float32", width=values.shape[1], height=values.shape[0],
                               crs=src.crs, transform=grid, nodata=src.nodata, compress="deflate")
                with rasterio.open(native, "w", **profile) as dst:
                    dst.write(values.filled(src.nodata), 1)
                    dst.write_mask(~np.ma.getmaskarray(values))
                    dst.update_tags(AREA_OR_POINT=src.tags().get("AREA_OR_POINT", "Area"))
    except rasterio.errors.RasterioIOError:
        # GDAL errors can include the signed URL. Keep it out of logs/receipts.
        raise ValueError("Provider raster could not be retained through bounded reads") from None
    after, status, end = acquire(signed, 256, headers)
    if status != 206 or end.get("ETag") != config["etag"] or first != after:
        raise ValueError("Radar source changed during capture")
    scene = encode(item)
    pixels = native.read_bytes()
    receipt = {**meta, "etag": config["etag"], "lastModified": identity.get("Last-Modified"), "query": query,
               "discoverySha256": hashlib.sha256(raw).hexdigest(), "metadataSha256": hashlib.sha256(scene).hexdigest(),
               "retainedSha256": hashlib.sha256(pixels).hexdigest(), "retainedBytes": len(pixels),
               "retainedAt": dt.datetime.now(dt.timezone.utc).isoformat(), "selectionBounds": config["bounds"],
               "window": [int(x) for x in window.flatten()], "affine": list(grid)[:6],
               "validSamples": int(values.count())}
    write(root / "radar-receipt.json", encode(receipt))
    with zipfile.ZipFile(root / "radar-capture.zip", "x") as capture:
        for name, raw in (("native.tif", pixels), ("receipt.json", encode(receipt)), ("scene.json", scene)):
            capture.writestr(zipfile.ZipInfo(name), raw)
    return receipt


def weather(config, root, retained):
    import numpy as np
    from numcodecs import get_codec
    loc = Path(retained) if retained else root
    receipts = []
    total = 0
    maximum = config["maxSourceBytes"]
    if type(maximum) is not int or not 1 <= maximum <= 600 << 20:
        raise ValueError("Choose a bounded weather source byte budget")

    def source(key, budget):
        nonlocal total
        budget = min(budget, maximum-total)
        if budget <= 0:
            raise ValueError("ERA5 source byte budget exhausted")
        path = loc / ("weather.json" if key == ".zmetadata" else key.replace("/", "-"))
        if retained:
            if not 0 < path.stat().st_size <= budget:
                raise ValueError("Retained ERA5 object exceeds its byte budget")
            raw = path.read_bytes()
        else:
            raw, _, _ = acquire(ERA + key, budget)
            write(path, raw)
        digest = hashlib.sha256(raw).hexdigest()
        pins = config.get("sourceSha256", {})
        if pins and pins.get(key) != digest:
            raise ValueError(f"ERA5 object changed from its configured hash: {key}")
        total += len(raw)
        receipts.append({"key": key, "sha256": digest, "bytes": len(raw)})
        return raw

    consolidated = json.loads(source(".zmetadata", 1 << 20))
    metadata = consolidated["metadata"]
    epoch = date("1959-01-01T00:00:00Z")
    at = date(config["time"])
    hours = (at-epoch).total_seconds()/3600
    if hours != int(hours) or hours < 0:
        raise ValueError("Choose an exact hour in the source edition")
    index = int(hours)

    def chunk(name, key, axes, dtype, chunks, unit=None):
        info = metadata[name + "/.zarray"]
        attrs = metadata[name + "/.zattrs"]
        if (info["zarr_format"] != 2 or info["dtype"] != dtype or info["order"] != "C" or info["filters"] is not None
                or info["chunks"] != chunks or attrs["_ARRAY_DIMENSIONS"] != axes or
                unit is not None and attrs.get("units") != unit or info["compressor"].get("id") != "blosc"):
            raise ValueError("ERA5 Zarr metadata changed from the supported edition")
        payload = source(name + "/" + key, 160 << 20)
        # Blosc v1 header binds compressed/expanded sizes before decoding.
        expected = math.prod(chunks) * np.dtype(dtype).itemsize
        if len(payload) < 16 or int.from_bytes(payload[4:8], "little") != expected or int.from_bytes(payload[12:16], "little") != len(payload):
            raise ValueError("ERA5 chunk header does not match its native array")
        decoded = get_codec(info["compressor"]).decode(payload)
        return np.frombuffer(decoded, dtype=dtype).reshape(chunks)

    lat = chunk("latitude", "0", ["latitude"], "<f4", [721], "degrees_north")
    lon = chunk("longitude", "0", ["longitude"], "<f4", [1440], "degrees_east")
    levels = chunk("level", "0", ["level"], "<i8", [37])
    tc = metadata["time/.zarray"]["chunks"]
    if tc != [70158] or index >= metadata["time/.zarray"]["shape"][0]:
        raise ValueError("Hour outside supported ERA5 archive")
    time = chunk("time", str(index // tc[0]), ["time"], "<i8", tc, "hours since 1959-01-01 00:00:00")
    if time[index % tc[0]] != index:
        raise ValueError("ERA5 source time coordinate differs from the requested hour")
    west, south, east, north = config["bounds"]
    if not (0 <= west < east <= 180 and -90 <= south < north <= 90 and east-west <= 6 and north-south <= 6
            and 1 <= len(config["pressureHPa"]) <= 6 and set(config["variables"]) == set(WEATHER_VARS)):
        raise ValueError("Weather selection exceeds the supported spatial, level or variable budget")
    ys, xs = np.where((lat >= south) & (lat <= north))[0], np.where((lon >= west) & (lon <= east))[0]
    ps = np.array([np.where(levels == p)[0].item() for p in config["pressureHPa"]])
    sp = chunk("surface_pressure", f"{index}.0.0", ["time", "latitude", "longitude"], "<f4", [1, 721, 1440], "Pa")
    mask = np.isfinite(sp[0][np.ix_(ys, xs)])[None, :, :] & (sp[0][np.ix_(ys, xs)][None, :, :] >= levels[ps, None, None] * 100)
    variables = {}
    for name in config["variables"]:
        unit = WEATHER_VARS[name][0]
        source_values = chunk(name, f"{index}.0.0.0", ["time", "level", "latitude", "longitude"], "<f4", [1,37,721,1440], unit)
        values = source_values[0][np.ix_(ps, ys, xs)]
        variables[name] = {"units": unit, "samples": [float(v) if keep and np.isfinite(v) else None for v, keep in zip(values.flat, mask.flat)]}
        del source_values
    result = {"schema": "emad.era5-pressure-subset.v1", "edition": WEATHER_EDITION, "crs": "EPSG:4326",
              "verticalCoordinate": "pressure-hPa", "nativeSpacingDegrees": .25, "physicalAccuracy": "UNKNOWN",
              "pressureHPa": levels[ps].tolist(), "validTimes": [config["time"]], "latitudes": lat[ys].tolist(), "longitudes": lon[xs].tolist(),
              "aboveGroundMask": mask.ravel().tolist(), "variables": variables, "sourceObjects": receipts}
    count = validate_weather(result)
    raw = encode(result)
    write(root / "era5-pressure-subset.json", raw)
    summary = {"source": ERA, "validTime": config["time"], "nativeSamplesPerVariable": count, "aboveGroundSamples": int(mask.sum()),
               "sourceBytes": sum(x["bytes"] for x in receipts), "subsetSha256": hashlib.sha256(raw).hexdigest(), "subsetBytes": len(raw)}
    write(root / "era5-receipt.json", encode(summary))
    return summary


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("layer", choices=("radar", "weather"))
    parser.add_argument("output", type=Path)
    parser.add_argument("--config", type=Path, default=Path(__file__).with_name("observation_pilot.json"))
    parser.add_argument("--retained-weather", type=Path)
    args = parser.parse_args()
    try:
        config = json.loads(args.config.read_bytes())[args.layer]
        args.output.mkdir(parents=True, exist_ok=True)
        print(json.dumps(radar(config, args.output) if args.layer == "radar" else weather(config, args.output, args.retained_weather)))
    except (ValueError, KeyError, OSError) as exc:
        parser.exit(2, f"{exc}\n")
