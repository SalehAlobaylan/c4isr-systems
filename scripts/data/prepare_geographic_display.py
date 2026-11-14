"""Build a bounded geographic derivative for a pinned durable job."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import sys
from tempfile import TemporaryDirectory
import zipfile
import zlib

import numpy as np
import rasterio
from shapely.geometry import shape

import display_surface_mosaic as mosaic
import environment_library as library
import glb_geometry
import prepare_display_layers as layers
import prepare_display_terrain as terrain

TERRAIN = 'terrain-glb-v2'
RECIPES = (TERRAIN, 'imagery-display-v1', 'roads-display-v1', 'buildings-display-v1', 'water-display-v1')


def toolchain():
    versions = {'terrain': terrain.terrain_toolchain(), 'zlib': zlib.ZLIB_RUNTIME_VERSION}
    for path in (Path(__file__), Path(layers.__file__), Path(mosaic.__file__)):
        versions[path.name] = library.file_digest(path)[0]
    return hashlib.sha256(library.encoded(versions)).hexdigest()


def build_terrain(root, manifest):
    area = shape(manifest['area'])
    components = list(area.geoms) if area.geom_type == 'MultiPolygon' else [area]
    surfaces, geoids = {}, {}
    for member in manifest['members']:
        component = member.get('component')
        if member['layer'] == 'surface':
            if component not in range(len(components)):
                raise ValueError('Surface member has an invalid area component')
            surfaces.setdefault(component, []).append((member, root/member['name']))
        if member['layer'] == 'geoid':
            if component in geoids:
                raise ValueError('An area component requires one exact geoid')
            geoids[component] = root / member['name']

    if set(surfaces) != set(range(len(components))):
        raise ValueError('A native surface is required for each declared area component')
    transform, _ = terrain.frame_transform(manifest['frame'])
    output = glb_geometry.GLB()
    material = output.material('Reference surface', (.49, .48, .42, 1))
    points, triangles = 0, 0
    for index, component in enumerate(components):
        entries = surfaces[index]
        if len(entries) == 1:
            member, path = entries[0]
            datum = member['details']['verticalDatum']
        else:
            values, affine, datum, registration = mosaic.native_mosaic(entries, terrain.MAX_POINTS - points)
            path = root / f'mosaic-{index}.tif'
            with rasterio.open(path, 'w', driver='GTiff', width=values.shape[1], height=values.shape[0], count=1,
                               dtype='float64', crs='EPSG:4326', transform=affine, nodata=np.nan) as dst:
                dst.update_tags(AREA_OR_POINT=registration)
                dst.write(values, 1)
        vertices, faces, count, _ = terrain.mesh_component(path, geoids.get(index), datum, component, transform,
                                                            terrain.MAX_POINTS - points)
        points += count
        triangles += len(faces)
        if triangles > terrain.MAX_TRIANGLES:
            raise ValueError('Terrain triangle budget exceeded')
        output.mesh(vertices, faces, material)
    return output.bytes()


def generate(package, expected, recipe, output, request, scene):
    if recipe != TERRAIN:
        layer = recipe.removesuffix('-display-v1')
        with TemporaryDirectory(prefix='emad-display-receipt-') as scratch:
            original = Path(scratch) / 'request.json'
            layers.generate(package, expected, layer, output, original, scene)
            receipt = json.loads(original.read_bytes())
            receipt['toolchainSha256'] = toolchain()
            with request.open('xb') as stream:
                stream.write(library.encoded(receipt))
        return
    library.verify(package, expected)
    if output.exists() or request.exists():
        raise FileExistsError('Display output or publication request already exists')
    with zipfile.ZipFile(package) as archive, TemporaryDirectory(prefix='emad-mosaic-') as scratch:
        root = Path(scratch)
        entries = library.archive_entries(archive)
        manifest_raw, manifest = library.archive_json(archive, entries, 'manifest.json')
        if hashlib.sha256(manifest_raw).hexdigest() != expected:
            raise ValueError('Parent manifest changed during display preparation')
        dependencies = []
        for member in manifest['members']:
            if member['layer'] not in ('surface', 'geoid'):
                continue
            name = member['name']
            with archive.open(entries[name]) as src, (root/name).open('xb') as dst:
                shutil.copyfileobj(src, dst, 256 << 10)
            if library.file_digest(root/name, member['bytes']) != (member['sha256'], member['bytes']):
                raise ValueError('A terrain dependency changed during preparation')
            dependencies.append({'name': name, 'sha256': member['sha256']})
        raw = build_terrain(root, manifest)
    if len(raw) > layers.MAX_BYTES:
        raise ValueError('Terrain output exceeds the display byte budget')
    digest = hashlib.sha256(raw).hexdigest()
    with output.open('xb') as stream:
        stream.write(raw)
    with request.open('xb') as stream:
        stream.write(library.encoded({'requestKey': f'{recipe}:{digest}', 'manifestSha256': expected,
                                      'toolchainSha256': toolchain(), 'recipe': recipe, 'inputs': dependencies,
                                      'sha256': digest, 'bytes': len(raw)}))


def main():
    if sys.argv[1:] == ['--toolchain']:
        print(toolchain())
        return
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('package', type=Path)
    parser.add_argument('recipe', choices=RECIPES)
    parser.add_argument('output', type=Path)
    parser.add_argument('request', type=Path)
    parser.add_argument('--manifest-sha256', required=True)
    parser.add_argument('--scene-metadata', type=Path)
    args = parser.parse_args()
    try:
        generate(args.package, args.manifest_sha256, args.recipe, args.output, args.request, args.scene_metadata)
    except (KeyError, TypeError, OSError, ValueError, zipfile.BadZipFile) as error:
        parser.exit(2, f'{error}\n')


if __name__ == '__main__':
    main()
