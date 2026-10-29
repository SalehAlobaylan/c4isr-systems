import ast
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from prepare_portable_geography import prepare


class PortableGeographyTests(unittest.TestCase):
    def fixture(self, directory):
        import numpy as np
        import rasterio
        from rasterio.transform import from_bounds
        directory = Path(directory)
        tiles = []
        for i, (west, east) in enumerate([(46, 46.5), (46.5, 47)]):
            path = directory / f'tile-{i}.tif'
            with rasterio.open(path, 'w', driver='GTiff', width=8, height=16, count=1,
                               dtype='float32', crs='EPSG:4326', transform=from_bounds(west, 24, east, 25, 8, 16), nodata=-9999) as dst:
                dst.write(np.full((16, 8), 100, dtype='float32'), 1)
            tiles.append({'sourceId': 'fixture-dem', 'path': str(path), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest(), 'reference': 'https://fixture.invalid/owned-dem'})
        return {'area': {'type': 'Polygon', 'coordinates': [[[46.1, 24.1], [46.9, 24.1], [46.9, 24.7], [46.1, 24.7], [46.1, 24.1]],
                                                          [[46.3, 24.3], [46.5, 24.3], [46.5, 24.5], [46.3, 24.5], [46.3, 24.3]]]},
                'heightUnit': 'metres', 'gridSize': 9, 'demTiles': tiles,
                'referenceMetadata': {'sourceId': 'fixture-dem', 'retrievedAt': '2026-01-01T00:00:00Z', 'verticalDatum': 'WGS84_ELLIPSOID',
                                      'rights': {'retainRaw': True, 'allowAnalysis': True},
                                      'accuracy': {'horizontalM': 10, 'verticalM': 2, 'provenance': 'Synthetic fixture source uncertainty, not survey evidence'}}}

    def test_mosaic_rectangular_grid_holes_and_original_hashes(self):
        with tempfile.TemporaryDirectory() as directory:
            spec = self.fixture(directory)
            target = Path(directory) / 'prepared'
            result = prepare(spec, target)
            grid = json.loads((target / 'dem-grid.json').read_text())
            mask = json.loads((target / 'selection-mask.json').read_text())
            self.assertEqual(grid['width'], 9)
            self.assertNotEqual(grid['width'], grid['height'])
            self.assertEqual(len(mask), grid['width'] * grid['height'])
            self.assertIn(False, mask)
            self.assertIn(True, mask)
            for p in result['parents']:
                self.assertEqual(hashlib.sha256((target / p['file']).read_bytes()).hexdigest(), p['sha256'])
            self.assertEqual(json.loads((target / 'prepared-references.json').read_text())[0]['rights'], spec['referenceMetadata']['rights'])

    def test_bad_hash_and_missing_tiles_fail_without_fabrication(self):
        with tempfile.TemporaryDirectory() as directory:
            spec = self.fixture(directory)
            wrong = copy.deepcopy(spec)
            wrong['demTiles'][0]['sha256'] = '0' * 64
            with self.assertRaisesRegex(ValueError, 'hash mismatch'):
                prepare(wrong, Path(directory) / 'wrong')
            missing = copy.deepcopy(spec)
            missing['demTiles'] = missing['demTiles'][:1]
            with self.assertRaisesRegex(ValueError, 'do not cover'):
                prepare(missing, Path(directory) / 'missing')

    def test_unknown_vertical_datum_and_units_block_before_full_processing(self):
        with tempfile.TemporaryDirectory() as directory:
            spec = self.fixture(directory)
            spec['referenceMetadata']['verticalDatum'] = 'UNKNOWN'
            with self.assertRaisesRegex(ValueError, 'vertical datum'):
                prepare(spec, Path(directory) / 'unsupported')
            self.assertFalse((Path(directory) / 'unsupported').exists())

    def test_historical_models_are_explicitly_separate(self):
        source = Path(__file__).with_name('prepare_environment.py').read_text(encoding='utf-8')
        tree = ast.parse(source)
        build = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == 'build')
        called = {node.func.id for node in ast.walk(build) if isinstance(node, ast.Call) and isinstance(node.func, ast.Name)}
        self.assertNotIn('model', called)
        self.assertIn('build_fixture_models', source)
        self.assertNotIn('JEDDAH', Path(__file__).with_name('prepare_portable_geography.py').read_text())


if __name__ == '__main__':
    unittest.main()
