"""Independent offline coordinate/geometry reader using PROJ and GEOS.

No network requests, actor setup, application runtime or replay are required.
Round trips alone are not evidence of vertical accuracy: the recorded independent
controls and source uncertainty must also pass.
"""
import argparse
import hashlib
import json
import math
from pathlib import Path, PurePosixPath
import zipfile


def validate(path):
    from pyproj import Transformer
    from shapely.geometry import shape, Point, Polygon
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
        if len(names) != len(set(names)) or len(names) > 40:
            raise ValueError('duplicate or excessive entries')
        total = 0
        for info in archive.infolist():
            p = PurePosixPath(info.filename)
            if p.is_absolute() or '..' in p.parts or '\\' in info.filename or ':' in info.filename or info.file_size > 8 * 1024 * 1024:
                raise ValueError('unsafe package path/size')
            total += info.file_size
        if total > 16 * 1024 * 1024:
            raise ValueError('package exceeds independent-reader budget')
        manifest = json.loads(archive.read('manifest.json'))
        if manifest['schema'] != 'emad.portable-geography.v1':
            raise ValueError('unsupported geographic package')
        for f in manifest['files']:
            raw = archive.read(f['name'])
            if len(raw) != f['bytes'] or hashlib.sha256(raw).hexdigest() != f['sha256']:
                raise ValueError('file hash/size mismatch')
        mesh = json.loads(archive.read('terrain-mesh.json'))
        inp = manifest['input']
        area = shape(inp['area']['geometry'])
        origin = inp['input']['origin']
        profile = inp['input']['qualification']
        pipeline = ('+proj=pipeline +step +proj=cart +ellps=WGS84 +step +proj=topocentric +ellps=WGS84 '
                    f"+lon_0={origin['longitude']} +lat_0={origin['latitude']} +h_0={origin['height']}")
        transform = Transformer.from_pipeline(pipeline)
        max_error = 0.0
        for p in mesh['points']:
            lon, lat, h = p['longitude'], p['latitude'], p['ellipsoidHeight']
            if not all(math.isfinite(v) for v in [lon, lat, h, *p['enu']]) or not area.covers(Point(lon, lat)):
                raise ValueError('non-finite or out-of-selection vertex')
            expected = transform.transform(lon, lat, h, errcheck=True)
            error = math.dist(expected, p['enu'])
            max_error = max(max_error, error)
            if error > 0.001:
                raise ValueError('independent PROJ frame validation failed')
        for triangle in mesh['triangles']:
            if len(set(triangle)) != 3 or any(i < 0 or i >= len(mesh['points']) for i in triangle):
                raise ValueError('invalid triangle indices')
            polygon = Polygon([(mesh['points'][i]['longitude'], mesh['points'][i]['latitude']) for i in triangle])
            if not area.buffer(1e-12).covers(polygon):
                raise ValueError('surface bridges an omitted polygon/hole')
        for control in profile['controls']:
            actual = transform.transform(control['longitude'], control['latitude'], control['ellipsoidHeight'], errcheck=True)
            if math.hypot(actual[0] - control['east'], actual[1] - control['north']) > profile['horizontalToleranceM'] or abs(actual[2] - control['up']) > profile['verticalToleranceM']:
                raise ValueError('independent control validation failed')
        if manifest['maxControlHorizontalErrorM'] > profile['horizontalToleranceM'] or manifest['maxControlVerticalErrorM'] > profile['verticalToleranceM']:
            raise ValueError('recorded terrain control failure')
        if 'vectors.geojson' in names:
            for feature in json.loads(archive.read('vectors.geojson'))['features']:
                if not area.buffer(1e-12).covers(shape(feature['geometry'])):
                    raise ValueError('vector outside selected polygon')
        for name in names:
            if not name.startswith('imagery/') or name.endswith('-mask.json'):
                continue
            image = json.loads(archive.read(name))
            g = image['grid']
            mask = json.loads(archive.read(name.replace('.json', '-mask.json')))
            count = g['width'] * g['height']
            if any(len(image[key]) != count for key in ['red', 'nir', 'scl']) or len(mask) != count:
                raise ValueError('invalid imagery dimensions/mask')
            for i, selected in enumerate(mask):
                x, y = i % g['width'], i // g['width']
                west, north = g['west'] + x * g['stepLng'], g['north'] - y * g['stepLat']
                footprint = Polygon([(west, north), (west + g['stepLng'], north), (west + g['stepLng'], north - g['stepLat']), (west, north - g['stepLat'])])
                if selected and not area.buffer(1e-12).covers(footprint):
                    raise ValueError('imagery coverage outside selection')
                if not selected and (image['red'][i] is not None or image['nir'][i] is not None):
                    raise ValueError('omitted imagery has unmasked samples')
        return {'schema': 'emad.portable-reader-check.v1', 'status': 'PASS', 'vertices': len(mesh['points']),
                'triangles': len(mesh['triangles']), 'maxIndependentFrameErrorM': max_error,
                'qualification': 'Independent PROJ/GEOS coordinate check; supplied controls do not establish an observed survey or full production qualification.'}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('package')
    args = parser.parse_args()
    print(json.dumps(validate(Path(args.package)), indent=2))
