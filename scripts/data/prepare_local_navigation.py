"""Package Saudi and Middle East navigation from independently retained Natural Earth ZIPs."""
import argparse
import base64
import hashlib
import io
import json
from pathlib import Path
import re
import zipfile

import shapefile
from shapely.geometry import shape

from regional_geography import DEFAULT_PROFILE, load_profile, prepare
from recovery_data import encoded


def generate(originals, profile_path):
    profile = load_profile(profile_path)
    profile = {**profile, 'studyAreas': []}
    receipts = []

    def retained(dataset, name):
        path = originals / (name + '.zip')
        receipt = json.loads(path.with_suffix('.zip.receipt.json').read_text(encoding='utf-8'))
        raw = path.read_bytes()
        if (len(raw) != receipt['bytes'] or hashlib.sha256(raw).hexdigest() != receipt['sha256']
                or receipt['requestedUrl'] != f'https://naturalearth.s3.amazonaws.com/{dataset}.zip'):
            raise ValueError(f'Retained navigation source differs from receipt: {path.name}')
        receipts.append(receipt)
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            def part(ext):
                return io.BytesIO(archive.read(next(member for member in archive.namelist() if member.endswith(ext))))

            reader = shapefile.Reader(shp=part('.shp'), shx=part('.shx'), dbf=part('.dbf'), encoding='utf-8')
            return [(item.record.as_dict(), shape(item.shape.__geo_interface__)) for item in reader.iterShapeRecords()], receipt

    result = prepare(profile, retained)
    countries = {f'country:{code}' for code in profile['countries']}
    features = [feature for chunk in result['chunks'] for feature in json.loads(base64.b64decode(chunk['artifact']))['features']]
    saudi = {f['properties']['key'] for f in features if f['properties']['parentKey'] == 'country:SAU'}
    selected = []
    for feature in features:
        props = feature['properties']
        if props['key'] not in countries | saudi and props['parentKey'] not in saudi:
            continue
        geometry = shape(feature['geometry'])
        point = geometry.representative_point()
        arabic = next((name for name in props['aliases'].split(' | ') if re.search('[\u0600-\u06ff]', name)), None)
        props.update({'labelPoint': [point.x, point.y], 'labelArabic': arabic, 'bounds': list(geometry.bounds)})
        selected.append({'id': props['key'], **feature})
    return {'schema': 'emad.local-navigation.v1', 'version': result['version'], 'profile': profile['key'],
            'definition': profile['definition'], 'attribution': 'Made with Natural Earth',
            'termsUrl': 'https://www.naturalearthdata.com/about/terms-of-use/', 'sources': receipts,
            'coverage': sorted(countries), 'features': {'type': 'FeatureCollection', 'features': selected}}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('originals', type=Path)
    parser.add_argument('output', type=Path)
    parser.add_argument('--profile', type=Path, default=DEFAULT_PROFILE)
    args = parser.parse_args()
    navigation = generate(args.originals, args.profile)
    raw = encoded(navigation)
    with args.output.open('xb') as stream:
        stream.write(raw)
    print(json.dumps({'sha256': hashlib.sha256(raw).hexdigest(), 'bytes': len(raw),
                      'features': len(navigation['features']['features']), 'version': navigation['version']}))
