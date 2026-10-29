"""Normalize explicit retained inputs for a bounded area; no network or fixtures.

The spec supplies reviewed reference metadata (including rights and accuracy),
hash-pinned local DEM tiles and optional vectors. Original bytes are copied into
the private output directory. This tool grants no rights or source admission.
"""
import argparse
import base64
import hashlib
import json
import math
from pathlib import Path
import shutil


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def prepare(spec, output):
    import numpy as np
    import rasterio
    from rasterio.warp import transform_bounds
    from shapely.geometry import shape, mapping, Point, box
    from shapely.ops import unary_union
    from pyproj import Transformer, CRS, network
    network.set_network_enabled(False)

    area = shape(spec['area'])
    if not area.is_valid or area.geom_type not in ['Polygon', 'MultiPolygon'] or area.is_empty:
        raise ValueError('valid WGS84 polygon required')
    west, south, east, north = area.bounds
    size = int(spec['gridSize'])
    if not 2 <= size <= 64 or east - west > 2 or north - south > 2 or max(abs(south), abs(north)) > 85:
        raise ValueError('supported bounded grid is 2–64 nodes/side within 2 degrees and 85 latitude')
    parents = spec['demTiles']
    if not 1 <= len(parents) <= 16:
        raise ValueError('provide 1–16 retained DEM tiles')
    metadata = dict(spec['referenceMetadata'])
    if metadata.get('verticalDatum') not in ['WGS84_ELLIPSOID', 'EGM2008', 'GEOID_UNDULATION_EGM2008']:
        raise ValueError('declare supported input vertical datum; conversion requires a separate admitted geoid grid')
    if spec.get('heightUnit') != 'metres' or metadata.get('rights', {}).get('retainRaw') is not True or metadata.get('rights', {}).get('allowAnalysis') is not True:
        raise ValueError('explicit metre heights and reviewed retention/analysis rights required')
    accuracy = metadata.get('accuracy', {})
    if accuracy.get('horizontalM', 0) <= 0 or accuracy.get('verticalM', -1) < 0 or len(accuracy.get('provenance', '')) < 10:
        raise ValueError('source-backed accuracy/provenance required, before processing')
    output = Path(output).resolve()
    output.mkdir(parents=True, exist_ok=False)
    original_dir = output / 'originals'
    original_dir.mkdir()
    receipts, sources, bounds = [], [], []
    total = 0
    io_env = rasterio.Env(PROJ_NETWORK='OFF', GDAL_DISABLE_READDIR_ON_OPEN='EMPTY_DIR', GDAL_PAM_ENABLED='NO', GDAL_CACHEMAX=16777216)
    io_env.__enter__()
    try:
        for i, item in enumerate(parents):
            if not metadata.get('sourceId') or item.get('sourceId') != metadata['sourceId']:
                raise ValueError('a normalized mosaic must retain one admitted source identity; prepare mixed providers separately')
            path = Path(item['path']).resolve()
            total += path.stat().st_size
            if total > 256 * 1024 * 1024:
                raise ValueError('retained parent input budget exceeds 256 MiB')
            h = hashlib.sha256()
            with path.open('rb') as stream:
                for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                    h.update(chunk)
            if h.hexdigest() != item['sha256']:
                raise ValueError('retained parent hash mismatch')
            src = rasterio.open(path, driver='GTiff')
            if src.driver != 'GTiff' or src.crs is None or src.count != 1:
                src.close()
                raise ValueError('one-band DEM with explicit CRS required')
            if src.units[0] not in [None, 'm', 'metre', 'metres', 'meter', 'meters']:
                src.close()
                raise ValueError('unsupported DEM height unit; normalize explicitly')
            crs = CRS.from_user_input(src.crs).to_2d()
            if crs.area_of_use and not box(*crs.area_of_use.bounds).covers(area):
                src.close()
                raise ValueError('source horizontal CRS area of use does not cover selected area')
            sources.append(src)
            bounds.append(box(*transform_bounds(src.crs, 'EPSG:4326', *src.bounds, densify_pts=21)))
            name = f'originals/dem-{i:02d}.tif'
            shutil.copyfile(path, output / name)
            receipts.append({'file': name, 'sha256': item['sha256'], 'bytes': path.stat().st_size,
                             'reference': item['reference'], 'crs': src.crs.to_string(),
                             'bounds': list(src.bounds), 'resolution': list(src.res),
                             'verticalDatum': metadata['verticalDatum'], 'sourceId': metadata['sourceId'], 'retrievedAt': item.get('retrievedAt', metadata.get('retrievedAt'))})
        if not unary_union(bounds).covers(area):
            raise ValueError('required DEM tiles do not cover the selected area; no download is inferred')
        step = max(east - west, north - south) / (size - 1)
        width = int(math.ceil((east - west) / step - 1e-9)) + 1
        height = int(math.ceil((north - south) / step - 1e-9)) + 1
        grid_east, grid_north = west + (width - 1) * step, south + (height - 1) * step
        xs, ys = west + np.arange(width) * step, south + np.arange(height) * step
        values = []
        mask = []
        transforms = [Transformer.from_crs('EPSG:4326', CRS.from_user_input(src.crs).to_2d(), always_xy=True, allow_ballpark=False, only_best=True) for src in sources]
        for y in ys:
            for x in xs:
                selected = area.covers(Point(float(x), float(y)))
                mask.append(selected)
                value = None
                # Retain the rectangular normalized parent. Selection masks are
                # separate; final preparation reapplies the exact case polygon.
                for src, transform in zip(sources, transforms):
                    tx, ty = transform.transform(float(x), float(y), errcheck=True)
                    if not (src.bounds.left <= tx <= src.bounds.right and src.bounds.bottom <= ty <= src.bounds.top):
                        continue
                    cell = next(src.sample([(tx, ty)], masked=True))[0]
                    if not np.ma.is_masked(cell) and math.isfinite(float(cell)):
                        if value is not None and abs(value - float(cell)) > accuracy['verticalM']:
                            raise ValueError('overlapping DEM tiles disagree beyond admitted vertical uncertainty')
                        value = float(cell)
                values.append(value)
        if not any(selected and value is not None for selected, value in zip(mask, values)):
            raise ValueError('selected DEM has no valid samples')
        # Equal angular X/Y steps with independent row/column counts. The
        # rectangular grid may extend slightly beyond the polygon's bounds;
        # the explicit selection mask preserves holes/outside/nodata states.
        dx = dy = step
        grid = {'width': width, 'height': height, 'west': west, 'south': south,
                'stepX': dx, 'stepY': dy, 'registration': 'point_samples', 'values': values}
        raw = json.dumps(grid, separators=(',', ':'), allow_nan=False).encode()
        if len(raw) > 524288:
            raise ValueError('normalized reference exceeds 512 KiB')
        metadata.update({'kind': 'elevation', 'crs': 'EPSG:4326', 'aoi': {'west': west, 'south': south, 'east': grid_east, 'north': grid_north},
                         'resolution': {'value': dx * 3600, 'unit': 'arc_seconds'},
                         'sha256': digest(raw), 'artifact': base64.b64encode(raw).decode(),
                         'parentArtifactSha256': parents[0]['sha256'],
                         'transformation': 'emad.portable-normalization.v1; explicit hash-pinned mosaic, nearest source-node sampling, WGS84 point grid; no silent vertical conversion; full parent receipt lists every tile'})
        metadata['parents'] = [{k: p[k] for k in ['sha256', 'bytes', 'reference', 'crs', 'verticalDatum', 'sourceId', 'retrievedAt']} for p in receipts]
        (output / 'dem-grid.json').write_bytes(raw)
        (output / 'selection-mask.json').write_text(json.dumps(mask), encoding='utf-8')
        prepared = [metadata]
        for i, item in enumerate(spec.get('vectors', [])):
            if i >= 16:
                raise ValueError('vector input count exceeds 16')
            path = Path(item['path']).resolve()
            if path.stat().st_size > 524288:
                raise ValueError('bounded vector input exceeds 512 KiB')
            raw = path.read_bytes()
            if digest(raw) != item['sha256']:
                raise ValueError('vector parent hash mismatch')
            collection = json.loads(raw)
            if len(collection['features']) > 1000:
                raise ValueError('vector feature budget exceeded')
            features = []
            for f in collection['features']:
                geometry = shape(f['geometry'])
                if not geometry.is_valid:
                    raise ValueError('invalid source vector geometry')
                clipped = geometry.intersection(area)
                if not clipped.is_empty:
                    features.append({**f, 'geometry': mapping(clipped)})
            target = f'originals/vector-{i:02d}.geojson'
            shutil.copyfile(path, output / target)
            receipts.append({'file': target, 'sha256': item['sha256'], 'bytes': len(raw), 'reference': item['reference']})
            if features:
                normalized = json.dumps({'type': 'FeatureCollection', 'features': features}, separators=(',', ':'), allow_nan=False).encode()
                value = dict(item['referenceMetadata'])
                value.update({'sha256': digest(normalized), 'artifact': base64.b64encode(normalized).decode(),
                              'parentArtifactSha256': item['sha256'], 'transformation': 'Exact WGS84 polygon intersection; original retained in parent receipt'})
                prepared.append(value)
        receipt = {'schema': 'emad.portable-normalization.v1', 'area': spec['area'], 'parents': receipts,
                   'normalizedSha256': digest((output / 'dem-grid.json').read_bytes()),
                   'nodataSelected': sum(selected and v is None for selected, v in zip(mask, values)),
                   'limits': {'maxPixels': 4096, 'maxParentBytes': 256 * 1024 * 1024},
                   'limitations': ['Nearest source-node resampling and interpolation are approximations; source accuracy is not upgraded.',
                                   'Only declared local parents were read; source/vertical-grid coverage is never inferred from a round trip.',
                                   'Source administration must approve these references and the qualification profile before product processing.']}
        (output / 'parent-receipt.json').write_text(json.dumps(receipt, indent=2), encoding='utf-8')
        (output / 'prepared-references.json').write_text(json.dumps(prepared, indent=2), encoding='utf-8')
        return receipt
    finally:
        for src in sources:
            src.close()
        io_env.__exit__(None, None, None)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--spec', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    print(json.dumps(prepare(json.loads(Path(args.spec).read_text(encoding='utf-8')), args.output), indent=2))
