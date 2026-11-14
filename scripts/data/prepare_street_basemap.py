"""Build a bounded PMTiles basemap from exact retained environment vectors using GDAL."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
from tempfile import TemporaryDirectory
import zipfile

LAYERS = ('roads', 'buildings', 'water')
RECIPE = 'street-pmtiles-v1'
MIN_ZOOM, MAX_ZOOM = 12, 16
MAX_TILE_BYTES = 32 << 20


def generate(package, expected, output, request):
    if output.exists() or request.exists():
        raise FileExistsError('PMTiles output or publication request already exists')
    with TemporaryDirectory(prefix='emad-street-basemap-') as scratch, zipfile.ZipFile(package) as archive:
        root = Path(scratch)
        data = archive.read('manifest.json')
        if hashlib.sha256(data).hexdigest() != expected:
            raise ValueError('The prepared manifest differs from the pinned release')
        manifest = json.loads(data)
        members = [m for m in manifest['members'] if m['layer'] in LAYERS]
        if sorted(m['layer'] for m in members) != sorted(LAYERS):
            raise ValueError('The street profile needs one exact prepared member per vector layer')
        parent = {i['id']: i for i in manifest['inputs']}
        vector_package = root / 'source.gpkg'
        dependencies = []
        for member in members:
            name, layer = member['name'], member['layer']
            rep = parent[member['inputId']]['representation']
            if rep['producer'] != 'OpenStreetMap contributors' or not name.endswith('.gpkg'):
                raise ValueError('The street profile needs attributed OpenStreetMap vectors')
            info = archive.getinfo(name)
            if info.file_size != member['bytes'] or info.file_size > 128 << 20 or Path(name).name != name:
                raise ValueError('Prepared vector member size or name differs from the manifest')
            vector = archive.read(info)
            if hashlib.sha256(vector).hexdigest() != member['sha256']:
                raise ValueError('Prepared vector member SHA-256 differs from the manifest')
            path = root / name
            path.write_bytes(vector)
            dependencies.append({'name': name, 'sha256': member['sha256']})
            sql = ("SELECT geom, source_id, json_extract(source_tags, '$.name') AS name, "
                   "json_extract(source_tags, '$.highway') AS highway FROM features ORDER BY source_id")
            subprocess.run(['ogr2ogr', *(['-update'] if vector_package.exists() else []), '-f', 'GPKG',
                            str(vector_package), str(path), '-dialect', 'SQLITE', '-sql', sql, '-nln', layer], check=True)
        options = ['NAME=Riyadh reference streets', 'DESCRIPTION=Bounded retained OpenStreetMap data (ODbL 1.0)',
                   'TYPE=baselayer', f'MINZOOM={MIN_ZOOM}', f'MAXZOOM={MAX_ZOOM}', 'MAX_SIZE=1000000']
        target = root / 'street.pmtiles'
        env = dict(os.environ, GDAL_NUM_THREADS='1')
        subprocess.run(['ogr2ogr', '-f', 'PMTiles', *(arg for value in options for arg in ('-dsco', value)),
                        str(target), str(vector_package)], check=True, env=env)
        tile_size = target.stat().st_size
        if not 127 < tile_size <= MAX_TILE_BYTES:
            raise ValueError('Street archive exceeds the portable display budget')
        tiles = target.read_bytes()
    digest = hashlib.sha256(tiles).hexdigest()
    version = subprocess.check_output(['ogr2ogr', '--version'], text=True).strip()
    toolchain = hashlib.sha256(Path(__file__).read_bytes() + b'\0' + version.encode()).hexdigest()
    receipt = {'requestKey': f'{RECIPE}:{digest}', 'manifestSha256': expected, 'recipe': RECIPE,
               'inputs': dependencies, 'toolchainSha256': toolchain, 'sha256': digest, 'bytes': len(tiles)}
    with output.open('xb') as stream:
        stream.write(tiles)
    with request.open('x', encoding='utf-8') as stream:
        json.dump(receipt, stream, indent=2)
    return receipt


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('package', type=Path)
    parser.add_argument('output', type=Path)
    parser.add_argument('request', type=Path)
    parser.add_argument('--manifest-sha256', required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(generate(args.package, args.manifest_sha256, args.output, args.request)))
    except (KeyError, OSError, ValueError, zipfile.BadZipFile, subprocess.CalledProcessError) as error:
        parser.exit(2, f'{error}\n')
