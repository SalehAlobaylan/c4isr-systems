import json
import unittest
from pathlib import Path

import environment_library as base
from observation_formats import validate_weather

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "testdata" / "geospatial" / "owned-fixture-v1"


def digest(path):
    return base.file_digest(path)[0]


class OwnedFixtureTest(unittest.TestCase):
    def setUp(self):
        self.digests = json.loads((FIXTURE / "digests.json").read_text(encoding="utf-8"))
        self.answers = json.loads((FIXTURE / "answers.json").read_text(encoding="utf-8"))
        self.manifest = json.loads((FIXTURE / "manifest.json").read_text(encoding="utf-8"))
        self.topology = json.loads((FIXTURE / "topology.json").read_text(encoding="utf-8"))
        self.descriptor = json.loads((FIXTURE / "display-descriptor.json").read_text(encoding="utf-8"))

    def test_trusted_digests_bind_the_committed_bytes(self):
        self.assertEqual(self.digests["schema"], "emad.environment-fixture-digests.v1")
        self.assertEqual(self.digests["fixture"], "owned-fixture-v1")
        self.assertEqual(self.digests["preparedSha256"], digest(FIXTURE / "prepared.zip"))
        self.assertEqual(self.digests["preparedBytes"], (FIXTURE / "prepared.zip").stat().st_size)
        self.assertEqual(self.digests["manifestSha256"], digest(FIXTURE / "manifest.json"))
        self.assertEqual(self.digests["manifestBytes"], (FIXTURE / "manifest.json").stat().st_size)
        self.assertEqual(self.digests["answersSha256"], digest(FIXTURE / "answers.json"))
        self.assertEqual(self.digests["topologySha256"], digest(FIXTURE / "topology.json"))
        self.assertEqual(self.digests["fieldSha256"], digest(FIXTURE / "field-era5.json"))
        self.assertEqual(self.digests["displayDescriptorSha256"],
                         digest(FIXTURE / "display-descriptor.json"))

    def test_prepared_archive_verifies_with_the_existing_verifier(self):
        result = base.verify(FIXTURE / "prepared.zip", self.digests["manifestSha256"])
        self.assertEqual(result["status"], "VERIFIED_PACKAGE")
        self.assertEqual(result["manifestSha256"], self.digests["manifestSha256"])
        self.assertEqual(result["profile"], "reference-surface-v1")

    def test_answers_match_the_release_members(self):
        self.assertEqual(self.answers["schema"], "emad.environment-known-answers.v1")
        self.assertEqual(self.answers["fixture"], "owned-fixture-v1")
        self.assertEqual(
            [(item["name"], item["layer"], item["component"], item["sha256"], item["bytes"])
             for item in self.answers["members"]],
            [(item["name"], item["layer"], item["component"], item["sha256"], item["bytes"])
             for item in self.manifest["members"]])

    def test_answers_are_separate_from_trusted_digests(self):
        # A reader can be handed answers it did not derive; the trusted digest
        # file and the answers file must never be the same document.
        self.assertNotEqual(digest(FIXTURE / "answers.json"), digest(FIXTURE / "digests.json"))
        self.assertIn("derivation", self.answers)
        for key in ("area", "raster", "vectors", "frame", "field"):
            self.assertIn(key, self.answers["derivation"])

    def test_topology_artifact_records_crossings_and_shared_nodes(self):
        self.assertEqual(self.topology["schema"], "emad.environment-topology.v1")
        crossing = next(item for item in self.topology["crossings"]
                        if set(item["ways"]) == {"osm-way-1001", "osm-way-1002"})
        self.assertEqual(crossing["longitude"], 46.740)
        self.assertEqual(crossing["latitude"], 24.705)
        self.assertEqual(self.topology["ways"]["osm-way-1001"]["nodes"], [1101, 1102])
        self.assertEqual(self.topology["ways"]["osm-way-1004"]["nodes"], [1302, 1303])
        shared = next(item for item in self.topology["sharedNodes"]
                      if item["nodeId"] == 1302)
        self.assertEqual(shared["ways"], ["osm-way-1003", "osm-way-1004"])
        self.assertEqual((shared["longitude"], shared["latitude"]), (46.715, 24.745))
        self.assertTrue(self.topology["ways"]["osm-way-1005"]["clipped"])
        self.assertEqual(self.topology["ways"]["osm-way-1005"]["nodes"], [1401, 1402])
        self.assertEqual(self.topology["ways"]["osm-way-1005"]["components"], [0, 1])

    def test_weather_fixture_keeps_the_native_edition_and_mask(self):
        field = json.loads((FIXTURE / "field-era5.json").read_text(encoding="utf-8"))
        self.assertEqual(validate_weather(field), 100)
        self.assertEqual(field["validTimes"], ["2022-10-23T12:00:00Z", "2023-01-09T12:00:00Z"])
        self.assertEqual(field["latitudes"], [25.5, 25.25, 25.0, 24.75, 24.5])
        self.assertEqual(field["pressureHPa"], [850, 700])
        self.assertEqual(field["variables"]["u_component_of_wind"]["samples"][7], 0.0)
        self.assertFalse(field["aboveGroundMask"][37])
        self.assertIsNone(field["variables"]["temperature"]["samples"][54])
        self.assertTrue(field["aboveGroundMask"][54])

    def test_display_descriptor_names_exact_parent_members(self):
        self.assertEqual(self.descriptor["schema"], "emad.geographic-derivative.v1")
        self.assertEqual(self.descriptor["purpose"], "visual-reference")
        self.assertFalse(self.descriptor["computationalUse"])
        self.assertEqual(self.descriptor["units"], "metre")
        self.assertEqual(self.descriptor["axes"], ["east", "up", "south"])
        self.assertEqual(self.descriptor["localOriginEnu"], [0, 0, 0])
        self.assertEqual(self.descriptor["boundsLonLat"], [46.7, 24.7, 46.8, 24.76])
        self.assertEqual(self.descriptor["renderingAdjustments"][0]["value"], 0.6)
        self.assertEqual(self.descriptor["renderingAdjustments"][0]["unit"], "metre")
        members = {item["name"]: item["sha256"] for item in self.manifest["members"]}
        self.assertEqual([item["sha256"] for item in self.descriptor["parentMembers"]],
                         [members[item["name"]] for item in self.descriptor["parentMembers"]])
        self.assertIn("no-physical-accuracy-claim", self.descriptor["limitations"])

    def test_answers_were_derived_from_construction_not_from_the_reader(self):
        source = (ROOT / "scripts" / "data" / "build_owned_fixtures.py").read_text(encoding="utf-8")
        for marker in ("150.0 / 0.3048", "round(295.15", "osm-way-1001", "FIELD_TIMES"):
            self.assertIn(marker, source)


if __name__ == "__main__":
    unittest.main()
