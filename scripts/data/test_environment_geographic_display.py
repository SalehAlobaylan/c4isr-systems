import json
from pathlib import Path
import struct
from tempfile import TemporaryDirectory
import unittest

import numpy as np
import rasterio
from pyproj.enums import TransformDirection
from rasterio.transform import from_origin
from shapely.geometry import MultiPolygon, Polygon, box, mapping

from display_surface_mosaic import native_mosaic
from environment_library import frame_check
from prepare_geographic_display import build_terrain
from prepare_display_terrain import frame_transform


class NativeTerrainTest(unittest.TestCase):
    def test_disconnected_components_and_aligned_seam_preserve_native_holes(self):
        with TemporaryDirectory() as location:
            root = Path(location)

            def tile(name, west, component, values):
                path = root / name
                with rasterio.open(path, 'w', driver='GTiff', width=values.shape[1], height=values.shape[0],
                                   count=1, dtype='float32', crs='EPSG:4326', nodata=-9999,
                                   transform=from_origin(west, 24.713, .0005, .0005)) as image:
                    image.update_tags(AREA_OR_POINT='Point')
                    image.write(values.astype('float32'), 1)
                return {'name': name, 'layer': 'surface', 'component': component,
                        'details': {'verticalDatum': 'ellipsoid'}}, path

            values = np.full((6, 4), 600)
            values[2, 1] = -9999
            a = tile('west.tif', 46.760, 0, values)
            b = tile('east.tif', 46.762, 0, np.full((6, 4), 600))
            second = tile('remote.tif', 46.766, 1, np.full((6, 4), 610))
            values, _, _, _ = native_mosaic([a, b], 100)
            self.assertEqual(values.shape, (6, 8))
            self.assertTrue(np.isnan(values[2, 1]))
            self.assertTrue(np.isfinite(values[:, 3:5]).all())

            part = Polygon([(46.76,24.71),(46.764,24.71),(46.764,24.713),(46.76,24.713)],
                           [[(46.7626,24.711),(46.7631,24.711),(46.7631,24.7116),(46.7626,24.7116)]])
            area = MultiPolygon([part, box(46.766, 24.710, 46.768, 24.713)])
            frame, _ = frame_check(area)
            manifest = {'area': mapping(area), 'frame': frame, 'members': [a[0], b[0], second[0]]}
            output = build_terrain(root, manifest)
            self.assertEqual(output, build_terrain(root, manifest))
            length = struct.unpack_from('<I', output, 12)[0]
            metadata = json.loads(output[20:20+length])
            binary = output[28+length:]
            primitives = metadata['meshes'][0]['primitives']
            self.assertEqual(len(primitives), 2)

            def array(index, dtype):
                accessor = metadata['accessors'][index]
                view = metadata['bufferViews'][accessor['bufferView']]
                return np.frombuffer(binary, dtype=dtype, count=view['byteLength']//4, offset=view['byteOffset'])

            vertices = array(primitives[0]['attributes']['POSITION'], '<f4').reshape(-1,3)
            indices = array(primitives[0]['indices'], '<u4').reshape(-1,3)
            self.assertLess(len(indices), 2 * 7 * 5)
            transform, _ = frame_transform(frame)
            xyz = vertices.astype('float64')
            lon, lat, _ = transform.transform(xyz[:,0],-xyz[:,2],xyz[:,1],direction=TransformDirection.INVERSE)
            faces = lon[indices]
            self.assertTrue(((faces.min(axis=1) < 46.762) & (faces.max(axis=1) > 46.762)).any())
            cell_centres = list(zip(lon[indices].mean(axis=1),lat[indices].mean(axis=1)))
            self.assertFalse(any(46.76025 < lon < 46.76125 and 24.71125 < lat < 24.71225 for lon, lat in cell_centres))
            self.assertFalse(any(46.7626 < lon < 46.7631 and 24.711 < lat < 24.7116 for lon, lat in cell_centres))

            wrong = tile('overlap.tif', 46.7615, 0, np.full((6, 4), 601))
            with self.assertRaisesRegex(ValueError, 'conflicting native heights'):
                native_mosaic([a, wrong], 100)
            shifted = tile('offset.tif', 46.7621, 0, np.full((6,4), 600))
            with self.assertRaisesRegex(ValueError, 'offset from the native grid'):
                native_mosaic([a, shifted], 100)
            with self.assertRaisesRegex(ValueError, 'sample budget'):
                native_mosaic([a, b], 47)
