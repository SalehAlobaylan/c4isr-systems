"""Versioned readers for bounded observation inputs; no provider requests here."""
import datetime as dt
import hashlib
import json
import math
import re
from urllib.parse import urlsplit
import zipfile

RADAR_PRODUCT = re.compile(r"S1[ABC]_IW_GRDH_1SDV_\d{8}T\d{6}_\d{8}T\d{6}_\d{6}_[A-F0-9]{6}_rtc\Z")
RADAR_COLLECTION = "sentinel-1-rtc"
RADAR_HOST = "sentinel1euwestrtc.blob.core.windows.net"
WEATHER_EDITION = "WeatherBench 2 ERA5 1959-2023_01_10-full_37-1h-0p25deg-chunk-1"
WEATHER_VARS = {"temperature": ("K", 150, 360), "u_component_of_wind": ("m s**-1", -150, 150),
                "v_component_of_wind": ("m s**-1", -150, 150)}
RADAR_MAX = 16 << 20
WEATHER_MAX = 128 << 10
SHA = re.compile(r"[a-f0-9]{64}\Z")


def date(value):
    return dt.datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=dt.timezone.utc)


def radar_metadata(item):
    from shapely.geometry import shape
    p, a = item["properties"], item["assets"]["vv"]
    if (item["type"] != "Feature" or item["collection"] != RADAR_COLLECTION or not RADAR_PRODUCT.fullmatch(item["id"])
            or p["sar:instrument_mode"] != "IW" or p["sar:product_type"] != "GRD"
            or p["sar:polarizations"] != ["VV", "VH"] or p["s1:processing_level"] != "1"
            or p["sat:orbit_state"] not in ("ascending", "descending") or a["raster:bands"] != [
                {"nodata": -32768, "data_type": "float32", "spatial_resolution": 10}]):
        raise ValueError("Unsupported radar collection, mode, polarization or pixel contract")
    start, end = [dt.datetime.fromisoformat(p[n]) for n in ("start_datetime", "end_datetime")]
    orbit, relative, epsg = (p[n] for n in ("sat:absolute_orbit", "sat:relative_orbit", "proj:epsg"))
    projected = p["proj:transform"]
    url = urlsplit(a["href"])
    geom = shape(item["geometry"])
    if (start.utcoffset() != dt.timedelta(0) or end.utcoffset() != dt.timedelta(0) or not 0 < (end-start).total_seconds() <= 120
            or type(orbit) is not int or orbit <= 0 or type(relative) is not int or not 1 <= relative <= 175
            or type(epsg) is not int or not (32601 <= epsg <= 32660 or 32701 <= epsg <= 32760)
            or len(projected) != 9 or projected[:2] != [10, 0] or projected[3:5] != [0, -10] or projected[6:] != [0, 0, 1]
            or url.scheme != "https" or url.netloc != RADAR_HOST or url.query or url.fragment
            or not url.path.startswith("/sentinel1-grd-rtc/GRD/") or not url.path.endswith("/measurement/iw-vv.rtc.tiff")
            or p["s1:product_identifier"] not in url.path.split("/") or p["s1:product_identifier"].rsplit("_", 1)[0] + "_rtc" != item["id"]
            or not re.fullmatch(r"[a-f0-9]{32}", a["file:checksum"]) or type(a["file:size"]) is not int
            or not 1024 <= a["file:size"] <= 8 << 30 or geom.geom_type not in {"Polygon", "MultiPolygon"} or not geom.is_valid):
        raise ValueError("Radar acquisition, georeferencing or asset identity is invalid")
    return {"product": item["id"], "sourceProduct": p["s1:product_identifier"], "sourceURL": a["href"],
            "sourceBytes": a["file:size"], "providerChecksum": a["file:checksum"], "sourceStart": p["start_datetime"],
            "sourceEnd": p["end_datetime"], "absoluteOrbit": orbit, "relativeOrbit": relative, "direction": p["sat:orbit_state"],
            "mode": "IW", "polarization": "VV", "processing": "Catalyst RTC from Sentinel-1 Level-1 GRD",
            "unit": "gamma0 linear power", "crs": f"EPSG:{epsg}", "nodata": -32768, "spacingM": 10,
            "layoverShadow": "No separate mask published for this edition. Nodata and nonpositive samples are masked."}


def radar_archive(path, destination):
    with zipfile.ZipFile(path) as archive:
        names = archive.infolist()
        if len(names) != 3 or {n.filename for n in names} != {"scene.json", "receipt.json", "native.tif"} or any(
                n.is_dir() or n.file_size < 1 or n.file_size > RADAR_MAX or (n.external_attr >> 16) & 0o170000 not in (0, 0o100000) for n in names):
            raise ValueError("Unexpected radar capture members")
        if any(archive.getinfo(n).file_size > 128 << 10 for n in ("scene.json", "receipt.json")):
            raise ValueError("Radar metadata exceeds its budget")
        raw = archive.read("scene.json")
        item, receipt = json.loads(raw), json.loads(archive.read("receipt.json"))
        meta = radar_metadata(item)
        if receipt["metadataSha256"] != hashlib.sha256(raw).hexdigest() or receipt["sourceURL"] != meta["sourceURL"] or (
                receipt["sourceBytes"] != meta["sourceBytes"] or receipt["product"] != meta["product"] or
                not re.fullmatch(r'"0x[A-F0-9]+"', receipt["etag"])):
            raise ValueError("Radar receipt differs from the scene")
        pixels = archive.read("native.tif")
        if hashlib.sha256(pixels).hexdigest() != receipt["retainedSha256"] or len(pixels) != receipt["retainedBytes"]:
            raise ValueError("Radar capture pixels differ from receipt")
        destination.write_bytes(pixels)
    return item, meta, receipt


def validate_weather(document):
    if (document["schema"] != "emad.era5-pressure-subset.v1" or document["edition"] != WEATHER_EDITION
            or document["crs"] != "EPSG:4326" or document["nativeSpacingDegrees"] != .25
            or document["verticalCoordinate"] != "pressure-hPa" or document["physicalAccuracy"] != "UNKNOWN"):
        raise ValueError("Unsupported ERA5 field identity")
    times, levels, lat, lon = (document[key] for key in ("validTimes", "pressureHPa", "latitudes", "longitudes"))
    if (not 1 <= len(times) <= 4 or list(map(date, times)) != sorted(set(map(date, times))) or
            not 1 <= len(levels) <= 6 or any(type(x) is not int or x not in (500, 600, 700, 775, 800, 825, 850, 875, 900, 925, 950, 975, 1000) for x in levels)
            or levels != sorted(set(levels), reverse=True)):
        raise ValueError("Invalid ERA5 times or pressure levels")
    for axis, step, limit in ((lat, -.25, 90), (lon, .25, 180)):
        if not 2 <= len(axis) <= 25 or any(type(x) not in (int, float) or not math.isfinite(x) or abs(x) > limit or x * 4 != round(x * 4) for x in axis):
            raise ValueError("Invalid ERA5 coordinate axis")
        if any(b-a != step for a,b in zip(axis, axis[1:])):
            raise ValueError("ERA5 grid must preserve its native spacing and axis order")
    n = len(times)*len(levels)*len(lat)*len(lon)
    if n > 15000 or set(document["variables"]) != set(WEATHER_VARS):
        raise ValueError("Unsupported ERA5 variables or sample count")
    mask = document["aboveGroundMask"]
    if len(mask) != n or any(type(x) is not bool for x in mask):
        raise ValueError("Invalid ERA5 mask")
    for name, (unit, lo, hi) in WEATHER_VARS.items():
        field = document["variables"][name]
        if field["units"] != unit or len(field["samples"]) != n:
            raise ValueError("ERA5 variable units or shape changed")
        for keep, value in zip(mask, field["samples"]):
            if value is not None and (not keep or type(value) not in (int, float) or not lo <= value <= hi):
                raise ValueError("Invalid ERA5 value or below-ground sample")
    sources = document["sourceObjects"]
    if not 8 <= len(sources) <= 21 or any(not SHA.fullmatch(x["sha256"]) or type(x["bytes"]) is not int or not 0 < x["bytes"] <= 160 << 20 for x in sources):
        raise ValueError("Invalid ERA5 source object receipts")
    epoch = date("1959-01-01T00:00:00Z")
    hours = [(date(value)-epoch).total_seconds()/3600 for value in times]
    if any(hour != int(hour) or not 0 <= hour < 561264 for hour in hours):
        raise ValueError("ERA5 valid time lies outside this native hourly edition")
    expected = {".zmetadata", "latitude/0", "longitude/0", "level/0"}
    for hour in map(int,hours):
        expected |= {f"time/{hour//70158}", f"surface_pressure/{hour}.0.0", *(f"{name}/{hour}.0.0.0" for name in WEATHER_VARS)}
    keys = {item["key"] for item in sources}
    if keys != expected or len(keys) != len(sources):
        raise ValueError("ERA5 source receipts do not bind its exact native hours and variables")
    return n
