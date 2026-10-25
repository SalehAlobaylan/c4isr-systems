"""Offline native-raster preparation tests; run with the pinned preparation dependencies."""
import json
import tempfile
import unittest
from pathlib import Path
import numpy as np
import rasterio
from rasterio.transform import from_origin
from prepare_imagery_subset import prepare

class PreparationTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        root = Path(self.directory.name)
        self.scene = {"id": "scene-fixture", "contentHash": "a" * 64, "acquiredAt": "2025-02-01T00:00:00Z", "platform": "sentinel-2b", "tile": "MGRS-37QED", "orbit": 135, "processingVersion": "05.11"}
        self.grid = {"width": 4, "height": 4, "west": 39.16, "north": 21.49, "stepLng": .0002, "stepLat": .0002}
        self.paths = {}
        for name, value in (("B04_10m", 3000), ("B08_10m", 7000), ("SCL_20m", 4)):
            path = root / (name + ".tif")
            values = np.full((4, 4), value, dtype=np.uint16)
            if name == "B04_10m": values[0, 0] = 0
            if name == "SCL_20m": values[3, 3] = 9
            with rasterio.open(path, "w", driver="GTiff", count=1, dtype="uint16", width=4, height=4, crs="EPSG:4326", transform=from_origin(39.16, 21.49, .0002, .0002), nodata=0) as raster:
                raster.write(values, 1)
            self.paths[name] = path
        self.metadata = root / "MTD_MSIL2A.xml"
        self.metadata.write_text('<Product><PROCESSING_BASELINE>05.11</PROCESSING_BASELINE><BOA_QUANTIFICATION_VALUE>10000</BOA_QUANTIFICATION_VALUE><BOA_ADD_OFFSET band_id="3">-1000</BOA_ADD_OFFSET><BOA_ADD_OFFSET band_id="7">-1000</BOA_ADD_OFFSET></Product>', encoding="utf-8")

    def test_offset_nodata_mask_and_repeatability(self):
        tile = prepare(self.scene, self.paths, self.metadata, self.grid)
        self.assertIsNone(tile["red"][0])
        self.assertAlmostEqual(tile["red"][1], .2)
        self.assertAlmostEqual(tile["nir"][1], .6)
        self.assertEqual(tile["scl"][15], 9)
        self.assertEqual(len(tile["sourceHashes"]), 3)
        self.assertEqual(json.dumps(tile, sort_keys=True), json.dumps(prepare(self.scene, self.paths, self.metadata, self.grid), sort_keys=True))

    def test_missing_offset_wrong_baseline_and_bounds_fail(self):
        for field, value in (("processingVersion", "04.00"),):
            with self.assertRaises(ValueError): prepare({**self.scene, field: value}, self.paths, self.metadata, self.grid)
        with self.assertRaises(ValueError): prepare(self.scene, self.paths, self.metadata, {**self.grid, "width": 257})
        self.metadata.write_text('<Product><PROCESSING_BASELINE>05.11</PROCESSING_BASELINE><BOA_QUANTIFICATION_VALUE>10000</BOA_QUANTIFICATION_VALUE></Product>', encoding="utf-8")
        with self.assertRaises(ValueError): prepare(self.scene, self.paths, self.metadata, self.grid)

if __name__ == "__main__": unittest.main()
