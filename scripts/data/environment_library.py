"""Prepare and independently inspect bounded geographic environment packages."""
import argparse
from contextlib import closing
import hashlib
import importlib.metadata
import json
import math
import os
from pathlib import Path
import re
import shutil
import sqlite3
import sys
import tempfile
import zipfile

os.environ["PROJ_NETWORK"] = "OFF"
os.environ["GDAL_PAM_ENABLED"] = "NO"

SCHEMA = "emad.environment-release.v1"
ASSET_ID = re.compile(r"geoasset_[0-9a-f]{64}\Z")
MEMBER = re.compile(r"[a-z0-9][a-z0-9_-]{0,100}\.(tif|gpkg|json)\Z")
HASH = re.compile(r"[0-9a-f]{64}\Z")
MAX_OBJECT = 1 << 30
MAX_TOTAL = 4 << 30
MAX_JSON = 8 << 20
MAX_FEATURES = 100_000
PROFILE_LAYERS = {
    "overview-v1": {"countries", "subdivisions", "places", "coasts"},
    "reference-surface-v1": {"surface"},
    "reference-context-v1": {"surface", "imagery", "roads", "buildings", "water"},
}


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()


def hash_stream(stream, maximum):
    digest = hashlib.sha256()
    count = 0
    while chunk := stream.read(256 << 10):
        count += len(chunk)
        if count > maximum:
            raise ValueError("Object exceeds its byte budget")
        digest.update(chunk)
    return digest.hexdigest(), count


def file_digest(path, maximum=MAX_OBJECT):
    with path.open("rb") as stream:
        return hash_stream(stream, maximum)


def toolchain():
    import fiona
    import pyproj
    import rasterio
    import shapely
    versions = {name: importlib.metadata.version(name) for name in
                ("fiona", "numpy", "osmium", "pyproj", "rasterio", "shapely")}
    versions.update(python=sys.version.split()[0], proj=pyproj.proj_version_str,
                    raster_gdal=rasterio.__gdal_version__, vector_gdal=fiona.__gdal_version__,
                    geos=shapely.geos_version_string, sqlite=sqlite3.sqlite_version,
                    processor=file_digest(Path(__file__))[0])
    return hashlib.sha256(encoded(versions)).hexdigest()


def frame_check(area):
    from pyproj import Transformer
    lon0, lat0 = area.representative_point().coords[0]
    transform = Transformer.from_pipeline(
        "+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad "
        f"+step +proj=cart +ellps=WGS84 +step +proj=topocentric +ellps=WGS84 +lon_0={lon0} +lat_0={lat0} +h_0=0"
    )
    lon_r, lat_r = map(math.radians, (lon0, lat0))
    sin_lon, cos_lon = math.sin(lon_r), math.cos(lon_r)
    sin_lat, cos_lat = math.sin(lat_r), math.cos(lat_r)

    def ecef(lon, lat, h):
        lon, lat = map(math.radians, (lon, lat))
        e2 = 0.0066943799901413165
        n = 6378137 / math.sqrt(1 - e2 * math.sin(lat) ** 2)
        return ((n + h) * math.cos(lat) * math.cos(lon),
                (n + h) * math.cos(lat) * math.sin(lon),
                (n * (1 - e2) + h) * math.sin(lat))

    origin = ecef(lon0, lat0, 0)
    west, south, east, north = area.bounds
    error, controls = 0.0, []
    for lon, lat, height in [(lon0, lat0, 1), (west, south, 0), (east, north, 1000)]:
        dx, dy, dz = [a - b for a, b in zip(ecef(lon, lat, height), origin)]
        enu = (-sin_lon * dx + cos_lon * dy,
               -sin_lat * cos_lon * dx - sin_lat * sin_lon * dy + cos_lat * dz,
               cos_lat * cos_lon * dx + cos_lat * sin_lon * dy + sin_lat * dz)
        checked = transform.transform(lon, lat, height)
        error = max(error, math.dist(enu, checked))
        controls.append({"longitude": lon, "latitude": lat, "ellipsoidHeightM": height, "enuM": checked})
    if error > 0.001:
        raise ValueError("Independent coordinate transformation check exceeded 1 mm")
    return {"axes": ["east", "north", "up"], "origin": [lon0, lat0, 0],
            "heightDatum": "WGS84 ellipsoid", "axisOrder": "longitude, latitude",
            "controls": controls}, round(error, 9)


def check_geoid(source, area, scratch):
    import rasterio
    from rasterio.windows import Window
    from pyproj import Transformer, datadir

    grid = scratch / "egm2008-original.tif"
    try:
        try:
            os.link(source, grid)
        except OSError:
            shutil.copyfile(source, grid)
        datadir.append_data_dir(str(scratch.resolve()))
        transformer = Transformer.from_pipeline(
            "+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad "
            "+step +proj=vgridshift +grids=egm2008-original.tif +multiplier=1 "
            "+step +proj=unitconvert +xy_in=rad +xy_out=deg"
        )
        west, south, east, north = area.bounds
        point = area.representative_point().coords[0]
        max_error, samples = 0.0, []
        with rasterio.open(grid) as raster:
            for lon, lat in [(west, south), point, (east, north)]:
                column, row = (~raster.transform) * (lon, lat)
                column -= .5
                row -= .5
                c, r = math.floor(column), math.floor(row)
                dx, dy = column - c, row - r
                values = raster.read(1, window=Window(c, r, 2, 2), masked=True)
                if values.shape != (2, 2) or values.mask.any():
                    raise ValueError("The retained geoid lacks interpolation support over the area")
                a, b, c, d = map(float, values.ravel())
                offset = ((1 - dy) * ((1 - dx) * a + dx * b) + dy * ((1 - dx) * c + dx * d))
                offset = offset * raster.scales[0] + raster.offsets[0]
                actual = transformer.transform(lon, lat, 0, errcheck=True)[2]
                max_error = max(max_error, abs(actual - offset))
                samples.append({"longitude": lon, "latitude": lat, "undulationM": round(offset, 6)})
        if max_error > .001:
            raise ValueError("Independent EGM2008 interpolation check exceeded 1 mm")
        return {"method": "WGS84 ellipsoidal h = EGM2008 orthometric H + bilinear grid undulation N",
                "maxErrorM": round(max_error, 9), "samples": samples}
    finally:
        try:
            grid.unlink(missing_ok=True)
        except PermissionError:
            if os.name != "nt":
                raise


def crop_raster(source, output, area, rep):
    import numpy as np
    import rasterio
    from rasterio.errors import WindowError
    from rasterio.features import geometry_mask, geometry_window
    from rasterio.shutil import copy
    from rasterio.warp import transform_geom
    from rasterio.windows import Window
    from shapely.geometry import mapping

    temporary = output.with_suffix(".intermediate.tif")
    with rasterio.Env(GDAL_CACHEMAX=128 << 20, GDAL_TIFF_INTERNAL_MASK=True), rasterio.open(source, driver="GTiff") as src:
        if (not src.crs or src.crs != rasterio.crs.CRS.from_user_input(rep["crs"]) or
                not 1 <= src.count <= 16 or src.transform.b or src.transform.d or
                src.transform.a <= 0 or src.transform.e >= 0):
            raise ValueError("Raster CRS, bands or native grid differ from supported input")
        geom = transform_geom("EPSG:4326", src.crs, mapping(area))
        is_geoid = rep["layer"] == "geoid"
        try:
            window = geometry_window(src, [geom], pad_x=2 if is_geoid else 0, pad_y=2 if is_geoid else 0)
        except WindowError:
            return None
        width, height = int(window.width), int(window.height)
        if width * height * src.count * np.dtype(src.dtypes[0]).itemsize > MAX_OBJECT:
            raise ValueError("Native raster window exceeds object budget")
        transform = src.window_transform(window)
        with rasterio.open(temporary, "w", driver="GTiff", width=width, height=height, count=src.count,
                           dtype=src.dtypes[0], crs=src.crs, transform=transform, nodata=src.nodata,
                           tiled=True, blockxsize=512, blockysize=512, compress="deflate") as dst:
            dst.scales, dst.offsets, dst.units = src.scales, src.offsets, src.units
            dst.colorinterp = src.colorinterp
            dst.update_tags(AREA_OR_POINT=src.tags().get("AREA_OR_POINT", "Area"))
            valid, inside = 0, 0
            value_digest = hashlib.sha256()
            for _, block in dst.block_windows(1):
                src_block = Window(block.col_off + window.col_off, block.row_off + window.row_off,
                                   block.width, block.height)
                values = src.read(window=src_block)
                selected = (np.ones(values.shape[1:], dtype=bool) if is_geoid else
                            geometry_mask([geom], out_shape=values.shape[1:],
                                          transform=dst.window_transform(block), invert=True))
                mask = (src.read_masks(window=src_block) > 0).all(axis=0) & selected
                mask &= np.isfinite(values).all(axis=0)
                valid += int(mask.sum())
                inside += int(selected.sum())
                values[:, ~mask] = src.nodata if src.nodata is not None else 0
                dst.write(values, window=block)
                dst.write_mask(mask, window=block)
                value_digest.update(values.astype(values.dtype.newbyteorder("<"), copy=False).tobytes())
                value_digest.update(mask.tobytes())
        if not valid:
            temporary.unlink()
            return None
        copy(temporary, output, driver="COG", BLOCKSIZE=512, COMPRESS="DEFLATE",
             OVERVIEW_RESAMPLING="NEAREST", BIGTIFF="IF_SAFER")
        temporary.unlink()
        return {"crs": src.crs.to_string(), "transform": list(transform)[:6],
                "width": width, "height": height, "nativeResolution": list(src.res),
                "registration": src.tags().get("AREA_OR_POINT", "Area"),
                "validSamples": valid, "selectedSamples": inside, "bands": src.count,
                "maskMeaning": "Valid in every input band and within the requested footprint",
                "logicalSha256": value_digest.hexdigest(), "verticalDatum": rep["verticalDatum"],
                "resampling": "Native grid. Nearest neighbor for display overviews."}


def verify_surface_seams(root, members):
    import numpy as np
    import rasterio
    from rasterio.warp import transform_bounds
    from rasterio.windows import Window
    from shapely.geometry import box

    surfaces = [m for m in members if m["layer"] == "surface"]
    for index, first in enumerate(surfaces):
        for second in surfaces[index + 1:]:
            if first["component"] != second["component"]:
                continue
            with rasterio.open(root / first["name"]) as a, rasterio.open(root / second["name"]) as b:
                overlap = box(*a.bounds).intersection(box(*transform_bounds(b.crs, a.crs, *b.bounds)))
                if overlap.is_empty or overlap.area == 0:
                    continue
                if a.crs != b.crs or a.res != b.res or first["details"]["verticalDatum"] != second["details"]["verticalDatum"]:
                    raise ValueError("Overlapping surface inputs disagree on CRS, resolution or vertical datum")
                col, row = (~a.transform) * (b.transform.c, b.transform.f)
                offset_c, offset_r = round(col), round(row)
                if max(abs(col - offset_c), abs(row - offset_r)) > 1e-7:
                    raise ValueError("Overlapping surface grids have different native registration")
                x0, y0 = max(0, offset_c), max(0, offset_r)
                x1, y1 = min(a.width, offset_c + b.width), min(a.height, offset_r + b.height)
                for y in range(y0, y1, 512):
                    for x in range(x0, x1, 512):
                        w, h = min(512, x1 - x), min(512, y1 - y)
                        first_values = a.read(1, masked=True, window=Window(x, y, w, h))
                        second_values = b.read(1, masked=True, window=Window(x - offset_c, y - offset_r, w, h))
                        common = ~(np.ma.getmaskarray(first_values) | np.ma.getmaskarray(second_values))
                        if not np.array_equal(first_values.data[common], second_values.data[common]):
                            raise ValueError("Overlapping native surface values disagree")


def osm_features(path, layer, area, scratch):
    import osmium
    from shapely import from_wkb
    from shapely.geometry import LineString
    factory = osmium.geom.WKBFactory()
    features = []
    omitted = 0
    wanted = {"roads": lambda t: bool(t.get("highway")),
              "buildings": lambda t: bool(t.get("building")),
              "water": lambda t: bool(t.get("water") or t.get("waterway") or t.get("natural") == "water"
                                      or t.get("natural") == "coastline")}[layer]

    def add(identifier, geom, tags):
        nonlocal omitted
        if not geom.is_valid:
            omitted += 1
            return
        if geom.intersects(area):
            clipped = geom.intersection(area)
            if not clipped.is_empty:
                if len(features) >= MAX_FEATURES:
                    raise ValueError("Canonical feature budget exceeded")
                features.append((identifier, clipped, dict(tags)))

    class Extract(osmium.SimpleHandler):
        def way(self, way):
            nonlocal omitted
            if layer == "buildings" or not wanted(way.tags):
                return
            if layer == "water" and way.is_closed() and (way.tags.get("water") or way.tags.get("natural") == "water"):
                return
            try:
                coords = [(node.lon, node.lat) for node in way.nodes]
            except osmium.InvalidLocationError:
                omitted += 1
                return
            if len(coords) >= 2:
                add(f"osm-way-{way.id}", LineString(coords), way.tags)

        def area(self, obj):
            nonlocal omitted
            if layer == "roads" or not wanted(obj.tags):
                return
            try:
                geom = from_wkb(factory.create_multipolygon(obj))
            except (RuntimeError, ValueError):
                omitted += 1
                return
            add(f"osm-{'way' if obj.from_way() else 'relation'}-{obj.orig_id()}", geom, obj.tags)

    # Windows file-backed osmium indexes stay mapped beyond the handler.
    # The worker's memory limit also covers the sparse in-memory index.
    index = scratch / f"osm-node-index-{layer}"
    location_index = "sparse_mem_array" if os.name == "nt" else f"sparse_file_array,{index}"
    try:
        Extract().apply_file(str(path), locations=True, idx=location_index)
    finally:
        try:
            index.unlink(missing_ok=True)
        except PermissionError:
            # The Windows osmium file index can remain mapped until its
            # process exits. The worker removes the full attempt directory.
            if os.name != "nt":
                raise
    return sorted(features, key=lambda row: row[0]), omitted


def vector_features(path, rep, area, scratch):
    import fiona
    from pyproj import Transformer
    from shapely.geometry import shape
    from shapely.ops import transform

    fmt = rep["format"]
    if fmt in {"osm-pbf", "osm-xml"}:
        if rep["layer"] not in {"roads", "buildings", "water"}:
            raise ValueError("OSM extractor requires a supported feature layer")
        pbf = scratch / ("extract.osm.pbf" if fmt == "osm-pbf" else "extract.osm")
        try:
            try:
                os.link(path, pbf)
            except OSError:
                shutil.copyfile(path, pbf)
            return osm_features(pbf, rep["layer"], area, scratch)
        finally:
            pbf.unlink(missing_ok=True)
    if fmt not in {"geojson", "geopackage", "shapefile-zip"}:
        raise ValueError("Unsupported vector representation")
    uri = "zip://" + path.as_posix() if fmt == "shapefile-zip" else str(path)
    features, omitted = [], 0
    with fiona.open(uri, enabled_drivers=["GeoJSON", "GPKG", "ESRI Shapefile"]) as src:
        to_wgs = Transformer.from_crs(src.crs_wkt or src.crs, "EPSG:4326", always_xy=True)
        for item in src:
            if not item.geometry:
                omitted += 1
                continue
            geom = transform(to_wgs.transform, shape(item.geometry))
            if not geom.is_valid:
                omitted += 1
                continue
            if geom.intersects(area):
                if len(features) >= MAX_FEATURES:
                    raise ValueError("Canonical feature budget exceeded")
                features.append((str(item.id), geom.intersection(area), dict(item.properties)))
    return features, omitted


def write_vectors(path, features, omitted):
    import fiona
    from shapely.geometry import mapping
    digest = hashlib.sha256()
    schema = {"geometry": "Unknown", "properties": {"source_id": "str", "source_tags": "str"}}
    with fiona.open(path, "w", driver="GPKG", crs="EPSG:4326", layer="features", schema=schema) as out:
        for identifier, geom, props in features:
            tags = encoded(props).decode()
            if len(tags) > 64 << 10:
                raise ValueError("Feature attributes exceed budget")
            digest.update(encoded([identifier, geom.wkb_hex, tags]))
            out.write({"geometry": mapping(geom), "properties": {"source_id": identifier, "source_tags": tags}})
    with closing(sqlite3.connect(path)) as conn:
        with conn:
            conn.execute("UPDATE gpkg_contents SET last_change='1980-01-01T00:00:00.000Z'")
    return {"featureCount": len(features), "omissions": omitted, "logicalSha256": digest.hexdigest(),
            "crs": "EPSG:4326", "positionalAccuracy": "SOURCE_LIMITED", "heightMeaning": "Source tags only"}


def member(path, asset, info, component):
    digest, count = file_digest(path)
    return {"name": path.name, "layer": asset["representation"]["layer"], "sha256": digest, "bytes": count,
            "inputId": asset["id"], "component": component, "details": info}


def prepared_zip(path, root, names):
    with zipfile.ZipFile(path, "x", compression=zipfile.ZIP_STORED) as bundle:
        for name in sorted(names):
            info = zipfile.ZipInfo(name)
            info.external_attr = 0o100644 << 16
            with (root / name).open("rb") as source, bundle.open(info, "w", force_zip64=True) as dest:
                shutil.copyfileobj(source, dest, 256 << 10)


def prepare(root):
    from shapely.geometry import shape
    recipe = json.loads(read_small(root / "recipe.json"))
    if recipe["schema"] != "emad.environment-recipe.v1" or recipe["toolchainSha256"] != toolchain():
        raise ValueError("The pinned preparation toolchain differs from the installed toolchain")
    area = shape(recipe["area"]["geometry"])
    if area.geom_type not in {"Polygon", "MultiPolygon"} or not area.is_valid or area.is_empty:
        raise ValueError("A valid polygon or multipolygon is required")
    west, south, east, north = area.bounds
    if east - west >= 180 or south < -85 or north > 85:
        raise ValueError("Date-line and polar preparations are unsupported")
    components = list(area.geoms) if area.geom_type == "MultiPolygon" else [area]
    inputs = recipe["inputs"]
    if not 1 <= len(components) <= 16 or not 1 <= len(inputs) <= 64:
        raise ValueError("Component or source budget exceeded")
    frame, coordinate_error = frame_check(area)
    members, total, vertical = [], 0, None
    profile = recipe["profile"]["id"]
    required = set(PROFILE_LAYERS[profile])
    if any(a["representation"]["layer"] == "surface" and a["representation"]["verticalDatum"] == "EGM2008" for a in inputs):
        required.add("geoid")
    for number, asset in enumerate(inputs):
        identifier = asset["id"]
        if not ASSET_ID.fullmatch(identifier):
            raise ValueError("Invalid retained input identity")
        path = root / "inputs" / identifier
        digest, count = file_digest(path, asset["bytes"])
        total += count
        if digest != asset["sha256"] or count != asset["bytes"] or total > MAX_TOTAL:
            raise ValueError("Retained input bytes do not match the pinned recipe")
        rep = asset["representation"]
        if rep["format"] == "geotiff":
            if rep["layer"] == "geoid":
                if vertical is not None or digest != "4191d471eefebf24091b56dbc604353cb3b8cf8cc70e448bb9ae56a272bef17a":
                    raise ValueError("This profile supports the pinned NGA EGM2008 grid only")
                vertical = check_geoid(path, area, root)
            for i, component in enumerate(components):
                output = root / f"raster-{number:02d}-{i:02d}.tif"
                details = crop_raster(path, output, component, rep)
                if details:
                    members.append(member(output, asset, details, i))
        else:
            output = root / f"vector-{number:02d}.gpkg"
            features, omitted = vector_features(path, rep, area, root)
            if not features and "complete-empty:" not in rep["evidence"]:
                raise ValueError(f"{rep['layer']} has no retained features or reviewed completeness receipt")
            members.append(member(output, asset, write_vectors(output, features, omitted), None))
    if required - {part["layer"] for part in members}:
        raise ValueError("Required layers have no valid retained samples")
    for layer in required & {"surface", "imagery", "geoid"}:
        if {part["component"] for part in members if part["layer"] == layer} != set(range(len(components))):
            raise ValueError(f"{layer} has no valid samples in at least one component")
    verify_surface_seams(root, members)
    if len(members) > 1023 or sum(m["bytes"] for m in members) > MAX_TOTAL:
        raise ValueError("Prepared member budget exceeded")
    manifest = {"schema": SCHEMA, "profileId": profile, "toolchainSha256": recipe["toolchainSha256"],
                "inputSha256": recipe["inputSha256"], "areaRevisionId": recipe["area"]["revisionId"],
                "area": recipe["area"]["geometry"], "frame": frame, "inputs": inputs, "members": members,
                "quality": {"coordinateErrorM": coordinate_error, "physicalAccuracy": "UNKNOWN",
                            "meaning": "Native reference data with explicit masks and source limitations",
                            "heightMethod": "Native elevations retained. Convert EGM2008 height using the supplied grid and method.",
                            "verticalCheck": vertical,
                            "unsupported": ["bathymetry", "verified building dimensions", "volumetric weather",
                                            "survey accuracy", "bare-earth replacement", "materials", "land cover"]}}
    raw = encoded(manifest)
    if len(raw) > MAX_JSON:
        raise ValueError("Manifest exceeds metadata budget")
    (root / "manifest.json").write_bytes(raw)
    prepared_zip(root / "prepared.zip", root, ["manifest.json", *(m["name"] for m in members)])
    if (root / "prepared.zip").stat().st_size > MAX_OBJECT:
        raise ValueError("Prepared package exceeds the single-object budget")
    return manifest


def read_small(path):
    with path.open("rb") as stream:
        raw = stream.read(MAX_JSON + 1)
    if len(raw) > MAX_JSON:
        raise ValueError("Metadata exceeds byte budget")
    return raw


def archive_entries(z):
    entries = z.infolist()
    if not 1 <= len(entries) <= 1024 or len({e.filename for e in entries}) != len(entries):
        raise ValueError("Unexpected or duplicate archive members")
    if any(e.is_dir() or (e.external_attr >> 16) & 0o170000 not in (0, 0o100000) for e in entries):
        raise ValueError("Archive members must be regular files")
    if sum(e.file_size for e in entries) > MAX_TOTAL + 2 * MAX_JSON:
        raise ValueError("Archive exceeds expanded byte budget")
    return {e.filename: e for e in entries}


def archive_json(z, entries, name):
    if name not in entries or entries[name].file_size > MAX_JSON:
        raise ValueError(f"Missing or oversized {name}")
    with z.open(entries[name]) as stream:
        raw = stream.read(MAX_JSON + 1)
    if len(raw) > MAX_JSON:
        raise ValueError(f"Oversized {name}")
    return raw, json.loads(raw)


def verify_spatial(path, item, manifest):
    from shapely.geometry import shape
    area = shape(manifest["area"])
    details = item["details"]
    if path.suffix == ".gpkg":
        import fiona
        digest = hashlib.sha256()
        count = 0
        bounds = area.buffer(1e-9)
        with fiona.open(path, driver="GPKG", layer="features") as features:
            if features.crs.to_epsg() != 4326:
                raise ValueError("Unexpected vector CRS")
            for feature in features:
                geom = shape(feature.geometry)
                if geom.is_empty or not geom.is_valid or not bounds.covers(geom):
                    raise ValueError("Vector geometry lies outside the retained area")
                props = feature.properties
                digest.update(encoded([props["source_id"], geom.wkb_hex, props["source_tags"]]))
                count += 1
        if count != details["featureCount"] or digest.hexdigest() != details["logicalSha256"]:
            raise ValueError("Vector values differ from the manifest")
    elif path.suffix == ".tif":
        import numpy as np
        import rasterio
        from rasterio.features import geometry_mask
        from rasterio.warp import transform_geom
        from shapely.geometry import mapping
        parts = list(area.geoms) if area.geom_type == "MultiPolygon" else [area]
        component = parts[item["component"]]
        valid, selected = 0, 0
        digest = hashlib.sha256()
        with rasterio.open(path, driver="GTiff") as raster:
            if (raster.crs.to_string() != details["crs"] or list(raster.transform)[:6] != details["transform"]
                    or [raster.width, raster.height, raster.count] != [details["width"], details["height"], details["bands"]]
                    or raster.tags(ns="IMAGE_STRUCTURE").get("LAYOUT") != "COG"):
                raise ValueError("Unexpected native grid or COG layout")
            geometry = transform_geom("EPSG:4326", raster.crs, mapping(component))
            for _, window in raster.block_windows(1):
                values = raster.read(window=window)
                mask = raster.dataset_mask(window=window) > 0
                clip = (np.ones(mask.shape, dtype=bool) if item["layer"] == "geoid" else
                        geometry_mask([geometry], out_shape=mask.shape, transform=raster.window_transform(window), invert=True))
                if (mask & ~clip).any():
                    raise ValueError("Raster mask includes pixels outside its component")
                valid += int(mask.sum())
                selected += int(clip.sum())
                digest.update(values.astype(values.dtype.newbyteorder("<"), copy=False).tobytes())
                digest.update(mask.tobytes())
        if valid != details["validSamples"] or selected != details["selectedSamples"] or digest.hexdigest() != details["logicalSha256"]:
            raise ValueError("Raster values or mask differ from the manifest")


def verify(bundle, expected_digest=None):
    """Check the portable bundle without a database, worker or provider."""
    with zipfile.ZipFile(bundle) as z, tempfile.TemporaryDirectory(prefix="emad-verify-") as scratch:
        entries = archive_entries(z)
        raw, manifest = archive_json(z, entries, "manifest.json")
        if expected_digest and hashlib.sha256(raw).hexdigest() != expected_digest:
            raise ValueError("Manifest differs from the pinned release digest")
        if manifest["schema"] != SCHEMA or manifest["quality"]["physicalAccuracy"] != "UNKNOWN":
            raise ValueError("Unsupported manifest or accuracy claim")
        if not 0 <= manifest["quality"]["coordinateErrorM"] <= .001:
            raise ValueError("Coordinate check failed")
        required = set(PROFILE_LAYERS[manifest["profileId"]])
        sources = {item["id"]: item for item in manifest["inputs"]}
        if len(sources) != len(manifest["inputs"]) or any(not ASSET_ID.fullmatch(a) for a in sources):
            raise ValueError("Invalid input dependencies")
        if any(a["representation"]["layer"] == "surface" and a["representation"]["verticalDatum"] == "EGM2008" for a in sources.values()):
            required.add("geoid")
            check = manifest["quality"]["verticalCheck"]
            if not check or not 0 <= check["maxErrorM"] <= .001 or len(check["samples"]) < 3:
                raise ValueError("Missing independent EGM2008 check")
        expected = {"manifest.json"}
        layers = set()
        for item in manifest["members"]:
            name, layer = item["name"], item["layer"]
            if not MEMBER.fullmatch(name) or name in expected or name not in entries:
                raise ValueError("Unexpected package path")
            if item["inputId"] not in sources or sources[item["inputId"]]["representation"]["layer"] != layer:
                raise ValueError("Member dependencies differ from the source representation")
            expected.add(name)
            layers.add(layer)
            info = entries[name]
            if not 1 <= item["bytes"] <= MAX_OBJECT or info.file_size != item["bytes"] or not HASH.fullmatch(item["sha256"]):
                raise ValueError("Package member metadata mismatch")
            with z.open(info) as stream:
                digest, size = hash_stream(stream, item["bytes"])
            if digest != item["sha256"] or size != item["bytes"]:
                raise ValueError(f"Integrity failure for {name}")
            path = Path(scratch) / name
            if path.suffix in {".tif", ".gpkg"}:
                with z.open(info) as source, path.open("xb") as destination:
                    shutil.copyfileobj(source, destination, 256 << 10)
                verify_spatial(path, item, manifest)
                path.unlink()
        if expected != set(entries) or not required.issubset(layers):
            raise ValueError("Package members or required layers differ from the manifest")
    return {"status": "VERIFIED_PACKAGE", "manifestSha256": hashlib.sha256(raw).hexdigest(),
            "profile": manifest["profileId"], "objects": len(expected), "quality": manifest["quality"]}


def rebuild(bundle, output):
    with tempfile.TemporaryDirectory(prefix="emad-rebuild-") as location:
        root = Path(location)
        with zipfile.ZipFile(bundle) as z:
            entries = archive_entries(z)
            recipe_raw, recipe = archive_json(z, entries, "recipe.json")
            _, original = archive_json(z, entries, "manifest.json")
            (root / "recipe.json").write_bytes(recipe_raw)
            (root / "inputs").mkdir()
            names = {"recipe.json", "manifest.json"}
            extracted = {}
            bindings = set()
            for asset in recipe["inputs"]:
                identifier = asset["id"]
                digest = asset["sha256"]
                name = f"inputs/{digest}"
                if not ASSET_ID.fullmatch(identifier) or identifier in bindings or not HASH.fullmatch(digest) or name not in entries:
                    raise ValueError("Unexpected rebuild source identity")
                bindings.add(identifier)
                if not 1 <= asset["bytes"] <= MAX_OBJECT or entries[name].file_size != asset["bytes"]:
                    raise ValueError("Rebuild source byte budget mismatch")
                target = root / "inputs" / identifier
                if digest in extracted:
                    os.link(extracted[digest], target)
                else:
                    with z.open(entries[name]) as source, target.open("xb") as destination:
                        shutil.copyfileobj(source, destination, 256 << 10)
                    extracted[digest] = target
                names.add(name)
            if names != set(entries):
                raise ValueError("Unexpected rebuild archive contents")
        generated = prepare(root)
        exact = encoded(generated) == encoded(original)
        for manifest in (generated, original):
            for obj in manifest["members"]:
                obj.pop("sha256")
                obj.pop("bytes")
        if encoded(generated) != encoded(original):
            raise ValueError("Rebuild differs in canonical metadata, geometry or normalized values")
        with (root / "prepared.zip").open("rb") as source, output.open("xb") as destination:
            shutil.copyfileobj(source, destination, 256 << 10)
        return {"status": "EQUIVALENT", "exactManifest": exact, **{
            key: value for key, value in verify(output).items() if key != "status"}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    sub.add_parser("toolchain")
    sub.add_parser("prepare").add_argument("path", type=Path)
    verify_command = sub.add_parser("verify")
    verify_command.add_argument("path", type=Path)
    verify_command.add_argument("--manifest-sha256")
    rebuild_command = sub.add_parser("rebuild")
    rebuild_command.add_argument("path", type=Path)
    rebuild_command.add_argument("output", type=Path)
    args = parser.parse_args()
    try:
        if args.action == "toolchain":
            print(toolchain())
        elif args.action == "prepare":
            manifest = prepare(args.path)
            verify(args.path / "prepared.zip")
            print(json.dumps({"status": "PREPARED", "members": len(manifest["members"])}))
        elif args.action == "verify":
            print(json.dumps(verify(args.path, args.manifest_sha256)))
        else:
            print(json.dumps(rebuild(args.path, args.output)))
    except (ValueError, KeyError, zipfile.BadZipFile) as exc:
        print(str(exc), file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
