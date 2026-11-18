import unittest
from pathlib import Path

from measure_environment_workload import assess, measure

ROOT = Path(__file__).resolve().parents[2]
PACKAGE = ROOT / "testdata" / "geospatial" / "owned-fixture-v1" / "prepared.zip"


class WorkloadTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.result = measure(PACKAGE, iterations=5, warmup=1)

    def test_operations_are_ordered_and_complete(self):
        self.assertEqual(self.result["schema"], "emad.environment-workload.v1")
        self.assertEqual(self.result["iterations"], 5)
        self.assertEqual(set(self.result["operations"]),
                         {"verify", "inventory", "sample", "point", "feature",
                          "boundedFeatures", "field"})
        for name, value in self.result["operations"].items():
            self.assertLessEqual(value["p50Ms"], value["p95Ms"], name)
            self.assertLessEqual(value["p95Ms"], value["maxMs"], name)
            self.assertGreater(value["count"], 0, name)
        self.assertGreater(self.result["packageBytes"], 0)
        self.assertGreater(self.result["memberBytes"], 0)
        self.assertGreater(self.result["peakRssBytes"], 0)

    def test_cleanup_probe_is_removed(self):
        self.assertEqual(self.result["cleanup"], {"removedProbe": True})

    def test_budget_assessment_passes_for_the_fixture(self):
        result = assess(dict(self.result))
        self.assertEqual(result["status"], "PASS")
        self.assertEqual(result["failures"], [])

    def test_budget_assessment_reports_a_failure(self):
        result = dict(self.result)
        result["budgets"] = {**self.result["budgets"], "sampleP95Ms": 0}
        assessed = assess(result)
        self.assertEqual(assessed["status"], "FAIL")
        self.assertEqual(assessed["failures"][0]["operation"], "sample")

    def test_kit_mode_measures_inventory_verification_and_keeps_reads(self):
        result = measure(PACKAGE, iterations=5, warmup=1, kit=True)
        self.assertIsNotNone(result["kit"])
        self.assertEqual(result["kit"]["purpose"], "native-inspection")
        self.assertEqual(len(result["kit"]["inventorySha256"]), 64)
        self.assertIn("kitVerify", result["operations"])
        self.assertEqual(result["operations"]["sample"]["count"], 5)
        self.assertEqual(set(result["operations"]),
                         {"kitVerify", "verify", "inventory", "sample", "point", "feature",
                          "boundedFeatures", "field"})
        self.assertEqual(assess(result)["status"], "PASS")


if __name__ == "__main__":
    unittest.main()
