import hashlib
import tempfile
import unittest
import zipfile
from pathlib import Path

import numpy as np
import rasterio
from rasterio.transform import from_origin
from shapely.geometry import MultiPolygon, Polygon, box, mapping

from environment_library import encoded, file_digest, prepare, toolchain, verify


class EnvironmentLibraryTest(unittest.TestCase):
    def recipe(self, root, elevations):
        inputs = root / "inputs"
        inputs.mkdir()
        assets = []
        for index, height in enumerate(elevations):
            identifier = "geoasset_" + hashlib.sha256(str(index).encode()).hexdigest()
            path = inputs / identifier
            values = np.full((64, 64), height, dtype="float32")
            values[15:20, 15:20] = -9999
            with rasterio.open(path, "w", driver="GTiff", width=64, height=64, count=1,
                               dtype="float32", crs="EPSG:4326", transform=from_origin(10, 20.064, .001, .001),
                               nodata=-9999) as target:
                target.write(values, 1)
            digest, size = file_digest(path)
            assets.append({"id": identifier, "sha256": digest, "bytes": size,
                           "representation": {"format": "geotiff", "crs": "EPSG:4326",
                                              "verticalDatum": "ellipsoid", "layer": "surface"}})
        hole = Polygon(box(10.01, 20.02, 10.04, 20.055).exterior.coords,
                       [box(10.02, 20.03, 10.03, 20.04).exterior.coords])
        area = MultiPolygon([hole, box(10.045, 20.02, 10.055, 20.03)])
        recipe = {"schema": "emad.environment-recipe.v1", "toolchainSha256": toolchain(),
                  "area": {"revisionId": "synthetic-holes", "geometry": mapping(area)},
                  "profile": {"id": "reference-surface-v1"}, "inputs": assets}
        recipe["inputSha256"] = hashlib.sha256(encoded(recipe)).hexdigest()
        (root / "recipe.json").write_bytes(encoded(recipe))

    def test_native_mask_holes_multipart_and_archive_integrity(self):
        with tempfile.TemporaryDirectory() as location:
            root = Path(location)
            self.recipe(root, [11])
            manifest = prepare(root)
            self.assertEqual(len(manifest["members"]), 2)
            self.assertEqual(verify(root / "prepared.zip")["status"], "VERIFIED_PACKAGE")
            first = manifest["members"][0]
            with rasterio.open(root / first["name"]) as raster:
                y, x = raster.index(10.025, 20.035)
                self.assertEqual(raster.dataset_mask()[y, x], 0)
                self.assertEqual(raster.transform.a, .001)
            with zipfile.ZipFile(root / "prepared.zip", "a") as archive:
                archive.writestr("../unexpected.tif", "unexpected")
            with self.assertRaisesRegex(ValueError, "members"):
                verify(root / "prepared.zip")

    def test_conflicting_overlap_fails_before_packaging(self):
        with tempfile.TemporaryDirectory() as location:
            root = Path(location)
            self.recipe(root, [11, 12])
            with self.assertRaisesRegex(ValueError, "surface values disagree"):
                prepare(root)
            self.assertFalse((root / "prepared.zip").exists())


if __name__ == "__main__":
    unittest.main()
