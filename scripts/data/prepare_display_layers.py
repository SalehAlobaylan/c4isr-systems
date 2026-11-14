"""Prepare independently publishable reference layers from a verified environment ZIP."""
import argparse
import base64
import hashlib
import json
import math
from pathlib import Path
import re
import struct
from tempfile import TemporaryDirectory
import zipfile
import zlib

import fiona
import numpy as np
import rasterio
from rasterio.features import geometry_mask
from rasterio.transform import from_bounds
from rasterio.warp import Resampling, reproject
from shapely.geometry import shape

import environment_library as library
from prepare_display_terrain import frame_transform, terrain_toolchain, undulation

RGB = {"B04_10m": "red", "B03_10m": "green", "B02_10m": "blue"}
SCL = "SCL_20m"
VALID_SCL = [4, 5, 6, 7]
MAX_TEXTURE = 1024
MAX_FEATURES = 30_000
MAX_POINTS = 350_000
MAX_BYTES = 32 << 20


def png_rgba(pixels):
    def chunk(kind, data):
        return struct.pack('>I', len(data)) + kind + data + struct.pack('>I', zlib.crc32(kind + data))

    height, width, _ = pixels.shape
    scanlines = b''.join(b'\0' + row.tobytes() for row in pixels)
    return (b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 6, 0, 0, 0))
            + chunk(b'IDAT', zlib.compress(scanlines, 9)) + chunk(b'IEND', b''))


def optical(members, parents, area, stac_path):
    if stac_path is None:
        raise ValueError('Imagery requires the retained Earth Search scene metadata')
    stac_bytes = stac_path.read_bytes()
    stac = json.loads(stac_bytes)
    scene = stac['id']
    acquired = stac['properties']['datetime']
    if stac['collection'] != 'sentinel-2-c1-l2a':
        raise ValueError('The optical recipe requires an Earth Search Sentinel-2 C1 L2A scene')
    assets = {}
    for member, path in members:
        source = parents[member['inputId']]['representation']
        edition_scene, band = source['productEdition'].rsplit(' ', 1)
        if band not in (*RGB, SCL):
            continue
        if edition_scene != scene or source['dataTime'] != acquired or band in assets:
            raise ValueError('Scene metadata differs from the admitted optical members or has multiple components')
        assets[band] = path
    if set(assets) != {*RGB, SCL}:
        raise ValueError('The optical recipe requires exact B04, B03, B02 and SCL members')
    bounds = area.bounds
    middle = (bounds[1] + bounds[3]) / 2
    width = min(MAX_TEXTURE, max(2, math.ceil((bounds[2] - bounds[0]) * 111320 * math.cos(math.radians(middle)) / 10)))
    height = min(MAX_TEXTURE, max(2, math.ceil((bounds[3] - bounds[1]) * 111320 / 10)))
    transform = from_bounds(*bounds, width, height)
    valid = geometry_mask([area], (height, width), transform, invert=True)
    channels, bands = [], []
    for band in [*RGB, SCL]:
        key = RGB.get(band, 'scl')
        asset = stac['assets'][key]
        if not asset['href'].split('?')[0].endswith('/' + band.split('_')[0] + '.tif'):
            raise ValueError('Scene asset name differs from the admitted band')
        source_band, = asset['raster:bands']
        with rasterio.open(assets[band]) as src:
            values = np.full((height, width), np.nan, dtype=np.float32)
            native = src.read(1, masked=True).astype(np.float32)
            reproject(native.filled(np.nan), values, src_transform=src.transform, src_crs=src.crs,
                      src_nodata=np.nan, dst_transform=transform, dst_crs='EPSG:4326', dst_nodata=np.nan,
                      resampling=Resampling.nearest, num_threads=1)
            if band == SCL:
                valid &= np.isin(values, VALID_SCL)
                continue
            scale, offset = float(source_band['scale']), float(source_band['offset'])
            if not (0 < scale <= .01 and -.5 <= offset <= .5 and source_band['nodata'] == src.nodata):
                raise ValueError('Unexpected radiometric metadata or raster nodata')
            valid &= np.isfinite(values)
            reflectance = np.nan_to_num(values * scale + offset)
            linear = np.clip(reflectance / .35, 0, 1)
            srgb = np.where(linear <= .0031308, 12.92 * linear, 1.055 * linear ** (1 / 2.4) - .055)
            channels.append(np.round(srgb * 255).astype(np.uint8))
            bands.append({'id': band, 'scale': scale, 'offset': offset})
    if not valid.any():
        raise ValueError('Imagery has no usable optical pixels within the release area')
    rgba = np.stack([*channels, valid.astype(np.uint8) * 255], axis=-1)
    rgba[~valid] = 0
    return {'scene': scene, 'acquiredAt': acquired, 'sceneMetadataSha256': hashlib.sha256(stac_bytes).hexdigest(),
            'bands': bands, 'validScl': VALID_SCL, 'bounds': list(bounds), 'width': width, 'height': height,
            'grid': 'EPSG:4326 pixel-is-area, north-up, nearest DN and SCL; AOI and native masks applied',
            'color': 'Surface reflectance / 0.35, clamped and encoded as sRGB', 'validPixels': int(valid.sum()),
            'texture': base64.b64encode(png_rgba(rgba)).decode('ascii')}


def local_paths(geometry, surface, geoid, datum, transform):
    kind, coordinates = geometry['type'], geometry['coordinates']
    parts = ([coordinates] if kind == 'LineString' else coordinates if kind in ('MultiLineString', 'Polygon')
             else [ring for polygon in coordinates for ring in polygon])
    x, y = surface[0], surface[1]
    grid = surface[2]
    result = []
    for part in parts:
        # Split long source segments to follow the retained surface between source vertices.
        dense = [part[0]]
        for start, end in zip(part, part[1:]):
            steps = max(1, math.ceil(max(abs(end[0] - start[0]) / x, abs(end[1] - start[1]) / y) * 2))
            if steps > 4096:
                raise ValueError('Vector segment exceeds the drape budget')
            dense.extend(tuple(a + (b - a) * step / steps for a, b in zip(start, end)) for step in range(1, steps + 1))
        lon, lat = np.asarray(dense, dtype=np.float64).T
        cx, cy = surface[3] * (lon, lat)
        cx, cy = cx - .5, cy - .5
        c, r = np.floor(cx).astype(int), np.floor(cy).astype(int)
        dx, dy = cx - c, cy - r
        inside = (c >= 0) & (r >= 0) & (c + 1 < grid.shape[1]) & (r + 1 < grid.shape[0])
        c, r = np.clip(c, 0, grid.shape[1] - 2), np.clip(r, 0, grid.shape[0] - 2)
        a, b, d, e = grid[r, c], grid[r, c + 1], grid[r + 1, c], grid[r + 1, c + 1]
        heights = np.where(dx + dy <= 1, a + (b - a) * dx + (d - a) * dy,
                           e + (d - e) * (1 - dx) + (b - e) * (1 - dy))
        inside &= np.isfinite(heights)
        lon, lat, heights = lon[inside], lat[inside], heights[inside]
        if datum == 'EGM2008' and len(lon):
            heights += undulation(geoid, lon, lat)
        east, north, up = transform.transform(lon, lat, heights, errcheck=True)
        vertices = np.round(np.column_stack((east, up + .6, -north)), 3)
        for run in np.split(vertices, np.cumsum(inside)[np.flatnonzero(~inside)]):
            if len(run) >= 2:
                result.append(run.tolist())
    return result


def vectors(layer, members, parents, frame):
    surfaces = [(m, p) for m, p in members if m['layer'] == 'surface']
    native = [(m, p) for m, p in members if m['layer'] == layer]
    if len(surfaces) != 1 or not native:
        raise ValueError('Vector draping needs a single retained surface and a requested native vector layer')
    item, path = surfaces[0]
    geoid = next((p for m, p in members if m['layer'] == 'geoid'), None)
    datum = item['details']['verticalDatum']
    if datum not in ('EGM2008', 'ellipsoid') or datum == 'EGM2008' and geoid is None:
        raise ValueError('Vector draping requires the applicable vertical correction')
    with rasterio.open(path) as src:
        if src.crs.to_epsg() != 4326 or src.transform.b or src.transform.d:
            raise ValueError('Vector draping requires a north-up WGS84 grid')
        grid = src.read(1, masked=True).astype(np.float64).filled(np.nan) * src.scales[0] + src.offsets[0]
        surface = (src.transform.a, -src.transform.e, grid, ~src.transform)
    transform, control_error = frame_transform(frame)
    features, points, available, omissions = [], 0, 0, 0
    seen = set()
    for member, vector_path in native:
        parent = parents[member['inputId']]
        edition = parent['sha256']
        rep = parent['representation']
        with fiona.open(vector_path, driver='GPKG', layer='features') as collection:
            available += len(collection)
            if available > MAX_FEATURES:
                raise ValueError('Native vector count exceeds the display budget')
            for source in collection:
                source_id = source.properties['source_id']
                geom = shape(source.geometry)
                if not geom.is_valid or geom.geom_type not in ('Polygon', 'MultiPolygon', 'LineString', 'MultiLineString'):
                    raise ValueError(f'Invalid or unsupported vector topology for {source_id}')
                identity = (edition, source_id)
                if identity in seen:
                    raise ValueError(f'Duplicate display feature identity {source_id}')
                seen.add(identity)
                geometry = geom.__geo_interface__
                paths = local_paths(geometry, surface, geoid, datum, transform)
                points += sum(map(len, paths))
                if points > MAX_POINTS:
                    raise ValueError('Draped vector vertex budget exceeded')
                omissions += not paths
                tags = json.loads(source.properties['source_tags'])
                height = {'kind': 'unknown'}
                match = re.fullmatch(r'([0-9]+(?:\.[0-9]+)?)\s*(?:m)?', str(tags.get('height', '')))
                if match and 0 < float(match[1]) <= 2000:
                    height = {'kind': 'source-tag', 'metres': float(match[1])}
                features.append({'type': 'Feature', 'id': f'{edition}:{source_id}', 'geometry': geometry,
                                 'properties': {'sourceId': source_id, 'edition': edition,
                                                'product': rep['productEdition'], 'dataTime': rep['dataTime'],
                                                'producer': rep['producer'], 'tags': tags, 'height': height}, 'paths': paths})
    return {'type': 'FeatureCollection', 'features': features, 'availableFeatures': available,
            'displayedFeatures': available - omissions, 'vertices': points, 'maxFrameControlErrorM': control_error,
            'method': 'Retained WGS84 topology; surface triangle interpolation with 0.6 m display offset'}


def generate(package, expected, layer, output, request, stac):
    library.verify(package, expected)
    if output.exists() or request.exists():
        raise FileExistsError('Display output or publication request already exists')
    with zipfile.ZipFile(package) as archive, TemporaryDirectory(prefix='emad-display-layer-') as scratch:
        entries = library.archive_entries(archive)
        manifest_raw, manifest = library.archive_json(archive, entries, 'manifest.json')
        if hashlib.sha256(manifest_raw).hexdigest() != expected:
            raise ValueError('Parent manifest changed during display preparation')
        parents = {asset['id']: asset for asset in manifest['inputs']}
        selected = {layer} if layer == 'imagery' else {layer, 'surface', 'geoid'}
        members, dependencies = [], []
        for member in manifest['members']:
            if member['layer'] not in selected:
                continue
            if layer == 'imagery' and parents[member['inputId']]['representation']['productEdition'].rsplit(' ', 1)[-1] not in (*RGB, SCL):
                continue
            name = member['name']
            path = Path(archive.extract(entries[name], scratch))
            if library.file_digest(path, member['bytes']) != (member['sha256'], member['bytes']):
                raise ValueError(f'Display member changed: {name}')
            members.append((member, path))
            dependencies.append({'name': name, 'sha256': member['sha256']})
        recipe = f'{layer}-display-v1'
        data = (optical(members, parents, shape(manifest['area']), stac) if layer == 'imagery'
                else vectors(layer, members, parents, manifest['frame']))
        raw = library.encoded({'schema': 'emad.display-layer.v1', 'manifestSha256': expected, 'layer': layer, 'data': data})
    if len(raw) > MAX_BYTES:
        raise ValueError('Display layer exceeds the byte budget')
    digest = hashlib.sha256(raw).hexdigest()
    toolchain = hashlib.sha256(library.encoded({'terrain': terrain_toolchain(), 'layer': library.file_digest(Path(__file__))[0],
                                               'zlib': zlib.ZLIB_RUNTIME_VERSION})).hexdigest()
    with output.open('xb') as stream:
        stream.write(raw)
    with request.open('xb') as stream:
        stream.write(library.encoded({'requestKey': f'{recipe}:{digest}', 'manifestSha256': expected,
                                      'toolchainSha256': toolchain, 'recipe': recipe, 'inputs': dependencies,
                                      'sha256': digest, 'bytes': len(raw)}))
    return {'layer': layer, 'sha256': digest, 'bytes': len(raw), **{k: v for k, v in data.items() if k not in ('texture', 'features')}}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('package', type=Path)
    parser.add_argument('layer', choices=('imagery', 'roads', 'buildings', 'water'))
    parser.add_argument('output', type=Path)
    parser.add_argument('request', type=Path)
    parser.add_argument('--manifest-sha256', required=True)
    parser.add_argument('--scene-metadata', type=Path)
    args = parser.parse_args()
    try:
        print(json.dumps(generate(args.package, args.manifest_sha256, args.layer, args.output, args.request, args.scene_metadata)))
    except (KeyError, OSError, ValueError, zipfile.BadZipFile) as error:
        parser.exit(2, f'{error}\n')
