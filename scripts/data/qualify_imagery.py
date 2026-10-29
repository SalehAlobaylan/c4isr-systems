"""Explicit live-provider pixel qualification, isolated from application grants.

No credentials, admissions, publications or case writes. Retain the exact live
STAC responses, native windows and receipts below --output; never call these
production-ready acquisitions or independent human review.
"""
import argparse
import json
import os
from pathlib import Path
import re
import requests
from shapely.geometry import box, shape


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    os.environ['EMAD_PRIVATE_DIR'] = str(output)
    from capture_imagery import capture
    from recovery_data import encoded, sha, now, store_json
    results = []
    for name, bounds in [('uae-al-ain', [55.73, 24.20, 55.75, 24.22]),
                         ('france-avignon', [4.82, 43.95, 4.84, 43.97])]:
        entry = {'area': name, 'bounds': bounds, 'state': 'NOT_RUN', 'synthetic': False}
        results.append(entry)
        try:
            response = requests.get('https://earth-search.aws.element84.com/v1/search', params={
                'collections': 'sentinel-2-c1-l2a', 'bbox': ','.join(map(str, bounds)),
                'datetime': '2025-06-01T00:00:00Z/2025-07-31T23:59:59Z', 'limit': 100}, timeout=45)
            response.raise_for_status()
            (output / (name + '-catalog.json')).write_bytes(response.content)
            groups = {}
            for feature in response.json()['features']:
                p = feature['properties']
                if p.get('eo:cloud_cover', 100) > 30 or not shape(feature['geometry']).covers(box(*bounds)):
                    continue
                orbit = re.search(r'_R(\d{3})_', p.get('s2:product_uri', ''))
                if not orbit:
                    continue
                key = (p['platform'], p['grid:code'], int(orbit[1]), p['s2:processing_baseline'])
                groups.setdefault(key, []).append(feature)
            pair = next((sorted(group, key=lambda f: f['properties']['datetime'])[:2]
                         for group in sorted(groups.values(), key=len, reverse=True)
                         if len({f['properties']['datetime'] for f in group}) >= 2), None)
            if pair is None:
                raise ValueError('No same-platform/tile/orbit/baseline pair in the bounded live catalog page')
            captured = []
            for feature in pair:
                p = feature['properties']
                orbit = int(re.search(r'_R(\d{3})_', p['s2:product_uri'])[1])
                mapping = {'red': 'B04_10m', 'green': 'B03_10m', 'blue': 'B02_10m',
                           'nir': 'B08_10m', 'scl': 'SCL_20m', 'product_metadata': 'product_metadata'}
                scene = {'id': 'live_qualification_' + sha(encoded(feature)), 'contentHash': sha(encoded(feature)),
                         'providerId': feature['id'], 'collection': feature['collection'], 'acquiredAt': p['datetime'],
                         'platform': p['platform'], 'tile': p['grid:code'], 'orbit': orbit,
                         'processingVersion': p['s2:processing_baseline'],
                         'assets': {target: {'href': feature['assets'][key]['href']} for key, target in mapping.items()},
                         'rasterBands': {target: feature['assets'][key]['raster:bands'][0]
                                         for key, target in mapping.items() if key != 'product_metadata'}}
                aoi = dict(zip(['west', 'south', 'east', 'north'], bounds))
                destination, result = capture(scene, aoi, size=128)
                captured.append({'providerId': scene['providerId'], 'acquiredAt': scene['acquiredAt'],
                                 'subset': str((destination / 'subset.json').relative_to(output)),
                                 'rgb': str((destination / 'rgb.png').relative_to(output)),
                                 'validPixels': result['validPixels'], 'pixels': result['pixels'],
                                 'files': result['files'], 'receipts': result['receipts']})
            entry.update(state='LIVE_PIXELS_RETAINED', captures=captured)
        except Exception as error:
            entry.update(state='FAILED', reason=str(error))
        store_json(output / 'live-imagery-report.json', {'retrievedAt': now(), 'results': results,
                   'meaning': 'Fresh anonymous provider retrieval of historical acquisitions. Isolated pixel qualification; no application admission, durable API workflow or independent publication is claimed.'})
        print(name + ': ' + entry['state'], flush=True)


if __name__ == '__main__':
    main()
