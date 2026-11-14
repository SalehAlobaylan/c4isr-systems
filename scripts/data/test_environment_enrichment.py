"""Checks for the source meanings that must survive preparation."""
import tempfile
import unittest
from pathlib import Path

import numpy as np
import rasterio
from rasterio.transform import from_origin

import environment_enrichment as e


class EnrichmentTests(unittest.TestCase):
    def test_categorical_validity_and_mask(self):
        classes = np.array([[10, 95], [0, 253]], dtype="uint8")
        visible = np.array([[True, True], [False, False]])
        self.assertEqual(e.class_counts(classes, visible), {"10": 1, "95": 1})
        with self.assertRaisesRegex(ValueError, "unsupported class codes"):
            e.class_counts(classes, np.ones_like(visible))

    def test_ground_scale_mask_and_signed_difference(self):
        with tempfile.TemporaryDirectory() as scratch:
            surface = Path(scratch) / "surface.tif"
            ground = Path(scratch) / "ground.tif"
            for path, values, scale in ((surface, [[8, 10], [12, 20]], 1), (ground, [[100, 90], [120, -9999]], .1)):
                with rasterio.open(path, "w", driver="GTiff", width=2, height=2, count=1, dtype="int32",
                                   crs="EPSG:4326", transform=from_origin(46.7, 24.8, .00025, .00025), nodata=-9999) as dst:
                    dst.write(np.array(values, dtype="int32"), 1)
                    dst.scales = (scale,)
            delta, valid, _ = e.comparison(surface, ground)
            np.testing.assert_array_equal(valid, [[True, True], [True, False]])
            np.testing.assert_allclose(delta[valid], [-2, 1, 0])
        rep = {"layer": "ground", "format": "geotiff", "crs": "EPSG:4326", "verticalDatum": "ellipsoid",
               "productEdition": e.GROUND_EDITION, "dataTime": "2006/2015", "semantics": "modeled-bare-earth"}
        with self.assertRaisesRegex(ValueError, "EGM2008"):
            e.check_representation(rep)

    def test_dimensions_keep_source_meaning(self):
        result = e.dimensions("buildings", {"height": """7'4" """, "building:levels": "5", "roof:height": "3;4"})
        self.assertEqual(result["height"]["value"], 2.2352)
        self.assertEqual(result["height"]["origin"], "provider-stated")
        self.assertEqual(result["height"]["uncertainty"], "UNKNOWN")
        self.assertEqual((result["levels"]["value"], result["levels"]["unit"]), (5, "count"))
        self.assertEqual((result["roofHeight"]["value"], result["roofHeight"]["origin"]), (None, "unknown"))
        self.assertEqual(e.modeled_value({"height": "8.25"})["origin"], "inferred")
        self.assertIsNone(e.modeled_value({"height": "NaN"})["value"])
        with self.assertRaisesRegex(ValueError, "Invalid"):
            e.modeled_value({"height": "Infinity"})

    def test_osm_relation_retains_courtyard_and_identity(self):
        from shapely.geometry import box
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            fixture = root / "courtyard.osm"
            fixture.write_text("""<osm version="0.6">
              <node id="1" lon="46.73" lat="24.70"/><node id="2" lon="46.75" lat="24.70"/>
              <node id="3" lon="46.75" lat="24.72"/><node id="4" lon="46.73" lat="24.72"/>
              <node id="5" lon="46.735" lat="24.705"/><node id="6" lon="46.745" lat="24.705"/>
              <node id="7" lon="46.745" lat="24.715"/><node id="8" lon="46.735" lat="24.715"/>
              <way id="10"><nd ref="1"/><nd ref="2"/><nd ref="3"/><nd ref="4"/><nd ref="1"/></way>
              <way id="11"><nd ref="5"/><nd ref="6"/><nd ref="7"/><nd ref="8"/><nd ref="5"/></way>
              <relation id="20"><member type="way" ref="10" role="outer"/><member type="way" ref="11" role="inner"/>
              <tag k="type" v="multipolygon"/><tag k="building" v="yes"/><tag k="height" v="4 m"/></relation>
              </osm>""")
            rows, omitted = e.base.osm_features(fixture, "buildings", box(46.7, 24.65, 46.8, 24.8), root)
            self.assertEqual(omitted, 0)
            self.assertEqual(len(rows), 1)
            identity, geometry, tags = rows[0]
            self.assertEqual(identity, "osm-relation-20")
            polygons = geometry.geoms if geometry.geom_type == "MultiPolygon" else [geometry]
            self.assertEqual(sum(len(polygon.interiors) for polygon in polygons), 1)
            self.assertEqual(e.dimensions("buildings", tags)["height"]["value"], 4)


if __name__ == "__main__":
    unittest.main()
