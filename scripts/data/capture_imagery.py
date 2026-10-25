"""Capture real C1 pixels as retained local windows; never apply SAFE offsets twice."""
import json
import math
from pathlib import Path
from urllib.parse import urlsplit
import numpy as np
import requests
import certifi
import rasterio
from rasterio.warp import transform_bounds, reproject, Resampling
from rasterio.windows import from_bounds, Window
from rasterio.transform import from_bounds as grid_transform
from recovery_data import DATA, encoded, sha, store_json, png, now


def capture(scene, bounds, size=128, cancelled=None):
    if scene['collection'] != 'sentinel-2-c1-l2a':
        raise ValueError('Automatic capture requires the independently admitted C1 source')
    if not (16 <= size <= 256) or max(bounds['east'] - bounds['west'], bounds['north'] - bounds['south']) > .1:
        raise ValueError('Capture exceeds the bounded initial AOI/grid profile')
    destination = DATA / 'imagery' / sha(encoded({'sceneHash': scene['contentHash'], 'aoi': bounds, 'size': size}))
    cached = destination / 'capture.json'
    if cached.exists():
        result = json.loads(cached.read_text())
        for name, digest in result['files'].items():
            if sha((destination / name).read_bytes()) != digest: raise ValueError('Captured imagery integrity mismatch')
        return destination, result
    destination.mkdir(parents=True, exist_ok=True)
    target = grid_transform(bounds['west'], bounds['south'], bounds['east'], bounds['north'], size, size)
    hashes, receipts, bands, colour = {}, {}, {}, {}
    for key in ['B04_10m', 'B08_10m', 'SCL_20m', 'B03_10m', 'B02_10m']:
        if cancelled is not None and cancelled.is_set():raise RuntimeError('capture_lease_unavailable')
        href = scene['assets'][key]['href']
        parsed = urlsplit(href)
        if parsed.scheme != 'https' or parsed.netloc != 'e84-earth-search-sentinel-data.s3.us-west-2.amazonaws.com' or not parsed.path.startswith('/sentinel-2-c1-l2a/') or parsed.query or parsed.fragment:
            raise ValueError('Unadmitted raster host or collection')
        with requests.get(href, headers={'Range': 'bytes=0-15'}, timeout=20, allow_redirects=False, stream=True) as response:
            response.raise_for_status()
            if response.status_code != 206 or not response.headers.get('ETag'):
                raise ValueError('A stable range-addressable source object is required')
            etag = response.headers['ETag']
        local = destination / (key + '.tif')
        with rasterio.Env(PROJ_NETWORK='OFF', GDAL_DISABLE_READDIR_ON_OPEN='EMPTY_DIR', GDAL_HTTP_TIMEOUT='30',
                          GDAL_HTTP_MAX_RETRY='2', CPL_CURL_CA_BUNDLE=certifi.where(),
                          GDAL_HTTP_HEADERS='If-Match: ' + etag, GDAL_CACHEMAX=32 * 1024 * 1024):
            with rasterio.open(href) as source:
                extent = transform_bounds('EPSG:4326', source.crs, bounds['west'], bounds['south'], bounds['east'], bounds['north'], densify_pts=21)
                window = from_bounds(*extent, transform=source.transform)
                left, top = math.floor(window.col_off), math.floor(window.row_off)
                right, bottom = math.ceil(window.col_off + window.width), math.ceil(window.row_off + window.height)
                window = Window(left, top, right-left, bottom-top)
                if left < 0 or top < 0 or right > source.width or bottom > source.height or window.width * window.height > 4_194_304:
                    raise ValueError('Capture window is outside the source or exceeds 4M pixels')
                array = source.read(1, window=window)
                profile = source.profile.copy()
                profile.update(driver='GTiff', width=array.shape[1], height=array.shape[0], transform=source.window_transform(window), compress='deflate', tiled=False)
                profile.pop('blockxsize', None); profile.pop('blockysize', None)
                with rasterio.open(local, 'w', **profile) as retained: retained.write(array, 1)
                out = np.full((size, size), 0 if key == 'SCL_20m' else np.nan, dtype=np.float32)
                reproject(array, out, src_transform=profile['transform'], src_crs=source.crs, src_nodata=0,
                          dst_transform=target, dst_crs='EPSG:4326', dst_nodata=0 if key == 'SCL_20m' else np.nan,
                          resampling=Resampling.nearest if key == 'SCL_20m' else Resampling.average, num_threads=1)
                metadata = scene['rasterBands'][key]
                if key != 'SCL_20m':
                    if 'scale' not in metadata or 'offset' not in metadata: raise ValueError('Explicit C1 radiometry required')
                    out = out.astype(np.float64) * metadata['scale'] + metadata['offset']
                bands[key] = out
                if key in ['B04_10m', 'B03_10m', 'B02_10m']:
                    display=np.full((512,512),np.nan,dtype=np.float32)
                    reproject(array,display,src_transform=profile['transform'],src_crs=source.crs,src_nodata=0,
                              dst_transform=grid_transform(bounds['west'],bounds['south'],bounds['east'],bounds['north'],512,512),
                              dst_crs='EPSG:4326',dst_nodata=np.nan,resampling=Resampling.average,num_threads=1)
                    colour[key]=display.astype(np.float64)*metadata['scale']+metadata['offset']
        with requests.get(href, headers={'Range': 'bytes=0-15'}, timeout=20, allow_redirects=False, stream=True) as response:
            response.raise_for_status()
            if response.headers.get('ETag') != etag: raise ValueError('Source object changed during capture')
        raw = local.read_bytes()
        hashes[key] = sha(raw)
        receipts[key] = {'href': href, 'parentETag': etag, 'captureSha256': sha(raw), 'bytes': len(raw), 'retrievedAt': now(),
                         'meaning': 'Hash covers retained source-resolution pixel window, not the complete upstream product', 'radiometry': metadata}
    def numbers(array): return [round(float(v), 7) if math.isfinite(v) else None for v in array.ravel()]
    subset = {'schema': 'emad.multispectral-subset.v1', 'sceneId': scene['id'], 'sceneHash': scene['contentHash'],
              'acquiredAt': scene['acquiredAt'], 'platform': scene['platform'], 'tile': scene['tile'], 'orbit': scene['orbit'],
              'processingVersion': scene['processingVersion'], 'reflectanceUnit': 'surface_reflectance', 'synthetic': False,
              'grid': {'crs': 'EPSG:4326', 'width': size, 'height': size, 'west': bounds['west'], 'north': bounds['north'],
                       'stepLng': (bounds['east']-bounds['west'])/size, 'stepLat': (bounds['north']-bounds['south'])/size,
                       'registration': 'pixel_edges_north_to_south'},
              'red': numbers(bands['B04_10m']), 'nir': numbers(bands['B08_10m']), 'scl': bands['SCL_20m'].astype(int).ravel().tolist(),
              'sourceHashes': {k: hashes[k] for k in ['B04_10m', 'B08_10m', 'SCL_20m']},
              'preparationMethod': 'emad.c1-window.v1; C1 DN*STAC scale+offset, no SAFE correction; retained raw window hashes; average reflectance and nearest SCL; EPSG:4326 angular grid; rasterio=' + rasterio.__version__}
    store_json(destination / 'subset.json', subset)
    rgb = np.stack([colour[k] for k in ['B04_10m', 'B03_10m', 'B02_10m']], axis=-1)
    rgb = np.uint8(np.clip(np.nan_to_num(rgb, nan=0) / .3, 0, 1) ** (1/2.2) * 255)
    (destination / 'rgb.png').write_bytes(png(rgb))
    valid = np.isin(bands['SCL_20m'].astype(int), [4, 5, 6])
    quality = np.zeros((size, size, 4), dtype=np.uint8)
    quality[~valid] = [214, 160, 42, 190]
    (destination / 'quality.png').write_bytes(png(quality))
    store_json(destination / 'scene.json', scene)
    store_json(destination / 'receipts.json', receipts)
    files = {p.name: sha(p.read_bytes()) for p in destination.iterdir() if p.is_file() and p.name != 'capture.json'}
    result = {'schema': 'emad.imagery-capture.v1', 'sceneId': scene['id'], 'aoi': bounds, 'synthetic': False,
              'files': files, 'validPixels': int(valid.sum()), 'pixels': int(size*size), 'receipts': receipts}
    store_json(cached, result)
    print('Captured real pixels for ' + scene['providerId'], flush=True)
    return destination, result
