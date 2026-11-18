import hashlib
import json
import math
import tempfile
import unittest
import zipfile
from pathlib import Path

import numpy as np
import rasterio
from rasterio.transform import from_origin

import environment_library as base
import environment_reader as reader

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "testdata" / "geospatial" / "owned-fixture-v1"
PACKAGE = FIXTURE / "prepared.zip"


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


class ReaderFixtureTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.answers = json.loads((FIXTURE / "answers.json").read_text(encoding="utf-8"))
        cls.digests = json.loads((FIXTURE / "digests.json").read_text(encoding="utf-8"))
        cls.reader = reader.Reader(PACKAGE, cls.digests["manifestSha256"],
                                   FIXTURE / "topology.json")
        cls.topology = FIXTURE / "topology.json"

    def test_inventory_reports_every_member_and_native_grid(self):
        inventory = self.reader.inventory()
        self.assertEqual(inventory["schema"], "emad.environment-inventory.v1")
        self.assertEqual(inventory["manifestSha256"], self.digests["manifestSha256"])
        self.assertEqual(inventory["layers"],
                         ["buildings", "modeled-building-context", "roads", "surface"])
        self.assertEqual(
            [(item["name"], item["sha256"], item["bytes"]) for item in inventory["members"]],
            [(item["name"], item["sha256"], item["bytes"]) for item in self.answers["members"]])
        tiles = [item for item in inventory["members"] if item["name"].endswith(".tif")]
        self.assertEqual(len(tiles), 3)
        self.assertEqual({item["native"]["registration"] for item in tiles}, {"Area"})
        self.assertEqual(tiles[0]["native"]["transform"], [0.001, 0.0, 46.7, 0.0, -0.001, 24.76])
        self.assertEqual(tiles[0]["evidence"]["validSamples"], 1699)
        self.assertEqual(tiles[0]["evidence"]["selectedSamples"], 1700)
        self.assertEqual(tiles[2]["native"]["component"], 1)

    def test_manifest_digest_mismatch_is_rejected(self):
        with self.assertRaises(ValueError):
            reader.Reader(PACKAGE, "f" * 64)

    def test_native_samples_match_independent_answers(self):
        for expected in self.answers["samples"]:
            with self.subTest(expected["id"]):
                result = self.reader.sample(expected["member"], expected["row"],
                                            expected["column"])
                for key in ("outcome", "member", "raw", "scale", "offset", "value", "unit"):
                    self.assertEqual(result[key], expected[key], key)
                self.assertEqual(result["nativeIndex"], {"row": expected["row"],
                                                         "column": expected["column"]})

    def test_measured_zero_survives_serialization(self):
        result = self.reader.sample("raster-00-00.tif", 10, 10)
        encoded = json.dumps(result)
        self.assertIn('"value": 0.0', encoded)
        self.assertIsNotNone(result["value"])
        self.assertIsInstance(result["value"], float)

    def test_out_of_member_sample_is_explicit(self):
        result = self.reader.sample("raster-00-00.tif", 60, 0)
        self.assertEqual(result["outcome"], "outside-member")

    def test_point_reads_match_independent_answers(self):
        for expected in self.answers["points"]:
            with self.subTest(expected["id"]):
                result = self.reader.point(expected["layer"], expected["longitude"],
                                           expected["latitude"], datum=expected.get("datum"),
                                           component=expected.get("component"))
                self.assertEqual(result["outcome"], expected["outcome"])
                if expected["outcome"] == "value":
                    self.assertEqual(result["value"], expected["value"])
                    self.assertEqual(result["member"], expected["member"])
                    self.assertEqual(result["component"], expected["component"])
                    self.assertEqual(result["unit"], expected["unit"])
                    self.assertEqual(result["verticalDatum"], expected["datum"])
                    self.assertEqual(result["method"], "native-value")
                    self.assertEqual(result["lineage"]["memberSha256"],
                                     next(item["sha256"] for item in self.answers["members"]
                                          if item["name"] == expected["member"]))
                elif expected["id"] == "hole":
                    self.assertIn("hole", result["detail"])

    def test_exact_seam_selects_the_eastern_member(self):
        result = self.reader.point("surface", 46.73, 24.7055, component=0)
        self.assertEqual(result["outcome"], "value")
        self.assertEqual(result["member"], "raster-01-00.tif")
        self.assertEqual(result["nativeIndex"], {"row": 54, "column": 0})
        self.assertEqual(result["value"], -10.0)

    def test_missing_ground_is_not_silently_substituted(self):
        result = self.reader.point("ground", 46.7105, 24.7495)
        self.assertEqual(result["outcome"], "unsupported-layer")
        self.assertEqual(result["reason"], "representation-absent")

    def test_nonzero_foot_conversion_is_exact(self):
        expected = next(item for item in self.answers["conversions"] if item["id"] == "foot")
        result = self.reader.sample("raster-00-00.tif", 20, 20, unit="foot")
        self.assertEqual(result["value"], expected["value"])
        self.assertEqual(result["conversionFactor"], 1 / .3048)

    def test_missing_geoid_is_an_explicit_conversion_outcome(self):
        expected = next(item for item in self.answers["conversions"]
                        if item["id"] == "missing-geoid")
        result = self.reader.point("surface", 46.7205, 24.7395, datum=expected["datum"])
        self.assertEqual(result["outcome"], expected["outcome"])
        self.assertEqual(result["reason"], expected["reason"])

    def test_features_match_independent_answers(self):
        index = {item["sourceId"]: item for item in self.answers["features"]}
        for source_id, expected in index.items():
            with self.subTest(source_id=source_id, layer=expected["layer"]):
                result = self.reader.feature(expected["layer"], source_id)
                self.assertEqual(result["outcome"], "value")
                self.assertEqual(result["partCount"], expected["partCount"])
                self.assertEqual(result["parts"][0]["geometryType"], expected["geometryType"])
                for key in ("start", "end", "polygonParts", "holes", "pieceCount", "bounds"):
                    if key in expected:
                        self.assertEqual(result["parts"][0][key], expected[key])
                if "modeledHeight" in expected:
                    for key, value in expected["modeledHeight"].items():
                        self.assertEqual(result["modeledHeight"][key], value)
                else:
                    for name, wanted in expected.get("dimensions", {}).items():
                        for key, value in wanted.items():
                            self.assertEqual(result["dimensions"][name][key], value,
                                             f"{name}.{key}")
                if "topology" in expected:
                    membership = result["topology"]["membership"]
                    for key in ("nodes", "clipped", "crossingsWith", "components"):
                        if key in expected["topology"]:
                            self.assertEqual(membership[key], expected["topology"][key])
                    shared = [{item["nodeId"]: item["with"][0]}
                              for item in membership["sharedJunctions"]]
                    wanted = [{item["nodeId"]: item["with"]}
                              for item in expected["topology"].get("sharedJunctions", [])]
                    self.assertEqual(shared, wanted)

    def test_feature_without_topology_artifact_discloses_unknown(self):
        result = reader.Reader(PACKAGE).feature("roads", "osm-way-1001")
        self.assertEqual(result["topology"]["state"], "unknown")
        self.assertEqual(result["topology"]["reason"], "no-topology-artifact")

    def test_unsupported_dimension_string_is_not_a_number(self):
        result = self.reader.feature("buildings", "osm-relation-2002")
        self.assertIsNone(result["dimensions"]["height"]["value"])
        self.assertEqual(result["dimensions"]["height"]["origin"], "unknown")
        self.assertEqual(result["dimensions"]["height"]["evidence"],
                         "unsupported source tag: height")
        self.assertIsNone(result["dimensions"]["levels"]["value"])
        self.assertIn("unsupported source tag", result["dimensions"]["levels"]["evidence"])

    def test_missing_dimension_is_distinct_from_unsupported(self):
        result = self.reader.feature("buildings", "osm-way-2301")
        self.assertEqual(result["dimensions"]["height"]["evidence"],
                         "missing source tag: height")

    def test_bounded_feature_selection_clips_and_caps(self):
        result = self.reader.features("roads", [46.70, 24.70, 46.72, 24.71])
        self.assertEqual(result["count"], 1)
        self.assertEqual(result["features"][0]["sourceId"], "osm-way-1001")
        self.assertEqual(json.loads(json.dumps(result["features"][0]["geometry"]))["coordinates"],
                         [[46.7, 24.705], [46.72, 24.705]])
        with self.assertRaises(ValueError):
            self.reader.features("roads", None, 0)

    def test_frame_controls_reproduce_within_a_micrometre(self):
        manifest = json.loads((FIXTURE / "manifest.json").read_text(encoding="utf-8"))
        for control in manifest["frame"]["controls"]:
            result = self.reader.frame_point(control["longitude"], control["latitude"],
                                             control["ellipsoidHeightM"])
            self.assertEqual(result["outcome"], "value")
            self.assertLess(math.dist(result["enu"], control["enuM"]), 1e-6)
        checked = self.reader.frame_point(46.73, 24.745, 0, check_controls=True)
        self.assertEqual(checked["controlCount"], 3)
        self.assertLess(checked["controlMaxErrorM"], 1e-6)

    def test_field_reads_match_independent_answers(self):
        for expected in self.answers["field"]:
            with self.subTest(expected["id"]):
                result = reader.field_read(FIXTURE / "field-era5.json", expected["variable"],
                                           expected["levelHPa"], expected["validTime"],
                                           expected["longitude"], expected["latitude"])
                self.assertEqual(result["outcome"], expected["outcome"])
                if expected["outcome"] == "value":
                    self.assertEqual(result["value"], expected["value"])
                    self.assertEqual(result["unit"], expected["unit"])
                    self.assertEqual(result["nativeIndex"]["time"],
                                     self.reader_field_time(expected["validTime"]))
                elif "reason" in expected:
                    self.assertEqual(result["reason"], expected["reason"])

    def reader_field_time(self, value):
        field = json.loads((FIXTURE / "field-era5.json").read_text(encoding="utf-8"))
        return field["validTimes"].index(value)

    def test_field_known_at_cutoff_is_explicit(self):
        result = reader.field_read(FIXTURE / "field-era5.json", "temperature", 850,
                                   "2022-10-23T12:00:00Z", 46.5, 25.25,
                                   known_at="2022-10-23T11:00:00Z")
        self.assertEqual(result["outcome"], "unavailable-before-validity")
        later = reader.field_read(FIXTURE / "field-era5.json", "temperature", 850,
                                  "2022-10-23T12:00:00Z", 46.5, 25.25,
                                  known_at="2022-10-23T13:00:00Z")
        self.assertEqual(later["temporal"]["knownAt"], "2022-10-23T13:00:00Z")
        self.assertEqual(later["temporal"]["kind"], "modeled-value-valid-at-instant")

    def test_field_off_grid_does_not_interpolate(self):
        result = reader.field_read(FIXTURE / "field-era5.json", "temperature", 850,
                                   "2022-10-23T12:00:00Z", 46.6, 25.25)
        self.assertEqual(result["outcome"], "off-grid")

    def test_display_asset_inventory_resolves_exact_parents(self):
        result = reader.asset_inventory(PACKAGE, FIXTURE / "display-descriptor.json")
        self.assertEqual(result["displayId"], "geodisplay_ownedfixtureterrain")
        self.assertEqual(result["purpose"], "visual-reference")
        self.assertIn("native-ground-elevation", result["notUsableFor"])
        self.assertIn("visual-inspection", result["usableFor"])
        self.assertEqual([parent["name"] for parent in result["parents"]],
                         ["raster-00-00.tif", "raster-01-00.tif", "raster-02-01.tif"])

    def test_visual_descriptor_cannot_claim_computational_use(self):
        descriptor = json.loads((FIXTURE / "display-descriptor.json").read_text(encoding="utf-8"))
        descriptor["computationalUse"] = True
        with tempfile.TemporaryDirectory() as location:
            path = Path(location) / "descriptor.json"
            path.write_text(json.dumps(descriptor), encoding="utf-8")
            with self.assertRaises(ValueError):
                reader.asset_inventory(PACKAGE, path)


class OverlapTest(unittest.TestCase):
    def package(self, root, values):
        area = {"type": "Polygon",
                "coordinates": [[[10.0, 20.0], [10.004, 20.0], [10.004, 20.004],
                                 [10.0, 20.004], [10.0, 20.0]]]}
        members = []
        for index, value in enumerate(values):
            name = f"raster-{index:02d}-00.tif"
            path = root / name
            with rasterio.open(path, "w", driver="GTiff", width=4, height=4, count=1,
                               dtype="int16", crs="EPSG:4326",
                               transform=from_origin(10, 20.004, .001, .001), nodata=-9999) as target:
                target.write(np.full((4, 4), value, dtype="int16"), 1)
                target.scales = (1.0,)
                target.offsets = (0.0,)
                target.units = ("metre",)
            members.append({"name": name, "layer": "surface", "component": 0,
                            "sha256": sha256(path), "bytes": path.stat().st_size,
                            "details": {"validSamples": 16, "selectedSamples": 16,
                                        "verticalDatum": "ellipsoid", "registration": "Area"}})
        manifest = {"schema": "emad.environment-release.v1", "profileId": "reference-surface-v1",
                    "areaRevisionId": "overlap-test", "toolchainSha256": "0" * 64,
                    "area": area, "inputs": [], "members": members,
                    "quality": {"physicalAccuracy": "UNKNOWN"},
                    "frame": {"axes": ["east", "north", "up"], "origin": [10, 20, 0],
                              "heightDatum": "WGS84 ellipsoid",
                              "axisOrder": "longitude, latitude", "controls": []}}
        package = root / "overlap.zip"
        with zipfile.ZipFile(package, "w") as archive:
            archive.writestr("manifest.json", json.dumps(manifest))
            for member in members:
                archive.write(root / member["name"], member["name"])
        return package

    def test_disagreeing_overlap_chooses_no_value(self):
        with tempfile.TemporaryDirectory() as location:
            package = self.package(Path(location), [10, 20])
            result = reader.Reader(package).point("surface", 10.002, 20.002)
        self.assertEqual(result["outcome"], "conflicting-overlap")
        self.assertEqual([item["value"] for item in result["candidates"]], [10.0, 20.0])

    def test_agreeing_overlap_is_deterministic(self):
        with tempfile.TemporaryDirectory() as location:
            package = self.package(Path(location), [10, 10])
            result = reader.Reader(package).point("surface", 10.002, 20.002)
        self.assertEqual(result["outcome"], "value")
        self.assertEqual(result["value"], 10.0)
        self.assertEqual(result["members"], ["raster-00-00.tif", "raster-01-00.tif"])


if __name__ == "__main__":
    unittest.main()
