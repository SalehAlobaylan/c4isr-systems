import hashlib
import json
import unittest
from pathlib import Path

import environment_suitability as suitability

ROOT = Path(__file__).resolve().parents[2]
CASES = ROOT / "testdata" / "geospatial" / "suitability-cases" / "cases.json"


class SuitabilityCasesTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.document = json.loads(CASES.read_text(encoding="utf-8"))

    def decisions(self, result):
        return [{key: item.get(key) for key in ("requirement", "result", "reason")}
                for item in result["decisions"]]

    def test_every_shared_vector_matches_its_expected_result(self):
        self.assertEqual(self.document["schema"], "emad.environment-suitability-cases.v1")
        for case in self.document["cases"]:
            with self.subTest(case["id"]):
                result = suitability.evaluate(case["requirement"], case["evidence"],
                                              case.get("now"))
                expected = case["expected"]
                self.assertEqual(result["schema"], suitability.RESULT_SCHEMA)
                self.assertEqual(result["requirementId"], case["requirement"]["requirementId"])
                self.assertEqual(result["overall"], expected["overall"])
                if expected["coverage"] is None:
                    self.assertIsNone(result["coverage"])
                else:
                    self.assertAlmostEqual(result["coverage"], expected["coverage"], places=12)
                self.assertEqual(self.decisions(result),
                                 [{key: item.get(key) for key in
                                   ("requirement", "result", "reason")}
                                  for item in expected["decisions"]])
                for got, wanted in zip(result["decisions"], expected["decisions"]):
                    for key, value in wanted.items():
                        self.assertEqual(got.get(key), value, f"{case['id']}: {key}")
                self.assertEqual([item["reason"] for item in result["optional"]],
                                 expected["optionalReasons"])

    def test_optional_omissions_never_change_the_overall_result(self):
        case = next(item for item in self.document["cases"]
                    if item["id"] == "surface-native-inspection")
        result = suitability.evaluate(case["requirement"], case["evidence"])
        self.assertEqual(result["overall"], "supported")
        self.assertEqual([item["result"] for item in result["optional"]],
                         ["optional-omitted"])

    def test_every_reason_is_declared(self):
        for case in self.document["cases"]:
            result = suitability.evaluate(case["requirement"], case["evidence"],
                                          case.get("now"))
            for decision in result["decisions"]:
                self.assertIn(decision["reason"], suitability.REASONS)

    def test_evidence_digest_is_stable_and_ignores_assessments_and_terms(self):
        case = next(item for item in self.document["cases"]
                    if item["id"] == "ground-accuracy-evidenced")
        first = suitability.evaluate(case["requirement"], case["evidence"])
        second = suitability.evaluate(case["requirement"], case["evidence"])
        self.assertEqual(first["evidenceSha256"], second["evidenceSha256"])
        altered = json.loads(json.dumps(case["evidence"]))
        altered["assessments"] = []
        altered["terms"] = {"expiresAt": "2000-01-01T00:00:00Z"}
        third = suitability.evaluate(case["requirement"], altered)
        self.assertEqual(first["evidenceSha256"], third["evidenceSha256"])
        self.assertEqual(len(first["evidenceSha256"]), 64)
        int(first["evidenceSha256"], 16)

    def test_release_digest_changes_when_member_bytes_change(self):
        case = next(item for item in self.document["cases"]
                    if item["id"] == "surface-native-inspection")
        baseline = suitability.release_digest(case["evidence"])
        altered = json.loads(json.dumps(case["evidence"]))
        altered["members"][0]["bytes"] += 1
        self.assertNotEqual(baseline, suitability.release_digest(altered))

    def test_area_holes_are_not_inside_the_release(self):
        area = next(item for item in self.document["cases"]
                    if item["id"] == "surface-native-inspection")["evidence"]["area"]
        self.assertTrue(suitability.inside(area, 46.705, 24.705))
        self.assertFalse(suitability.inside(area, 46.725, 24.725))
        self.assertFalse(suitability.inside(area, 46.765, 24.705))
        self.assertTrue(suitability.inside(area, 46.790, 24.705))

    def test_invalid_documents_are_refused(self):
        case = self.document["cases"][0]
        with self.assertRaises(ValueError):
            suitability.evaluate({**case["requirement"], "schema": "other"}, case["evidence"])
        with self.assertRaises(ValueError):
            suitability.evaluate(case["requirement"], {**case["evidence"], "schema": "other"})

    def test_terms_require_an_evaluation_instant_only_when_expiry_is_stated(self):
        case = next(item for item in self.document["cases"] if item["id"] == "terms-expired")
        result = suitability.evaluate(case["requirement"], case["evidence"])
        self.assertEqual(result["overall"], "unqualified")
        decision = next(item for item in result["decisions"] if item["requirement"] == "terms")
        self.assertEqual(decision["reason"], "terms-expiry-unknown")
        with_now = suitability.evaluate(case["requirement"], case["evidence"], case["now"])
        self.assertEqual(with_now["overall"], "unsupported")

    def test_correction_uses_the_new_assessment(self):
        case = next(item for item in self.document["cases"]
                    if item["id"] == "assessment-superseded")
        result = suitability.evaluate(case["requirement"], case["evidence"])
        decision = next(item for item in result["decisions"]
                        if item["requirement"] == "ground")
        self.assertEqual(decision["reason"], "accuracy-exceeded")
        self.assertEqual(decision["assessment"], "assessment-ground-2026-02")

    def test_unparseable_expiry_is_unqualified_not_accepted(self):
        case = next(item for item in self.document["cases"] if item["id"] == "terms-expired")
        evidence = json.loads(json.dumps(case["evidence"]))
        evidence["terms"]["expiresAt"] = "not-an-instant"
        result = suitability.evaluate(case["requirement"], evidence, case["now"])
        decision = next(item for item in result["decisions"] if item["requirement"] == "terms")
        self.assertEqual(result["overall"], "unqualified")
        self.assertEqual(decision["reason"], "terms-expiry-unknown")

    def test_unparseable_source_time_is_unknown(self):
        case = next(item for item in self.document["cases"] if item["id"] == "mixed-year-strict")
        evidence = json.loads(json.dumps(case["evidence"]))
        evidence["inputs"][0]["dataTime"] = "not-an-instant"
        result = suitability.evaluate(case["requirement"], evidence)
        decision = next(item for item in result["decisions"] if item["requirement"] == "time")
        self.assertEqual(result["overall"], "unqualified")
        self.assertEqual(decision["reason"], "time-unknown")


if __name__ == "__main__":
    unittest.main()
