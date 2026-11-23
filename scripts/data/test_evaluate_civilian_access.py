from hashlib import sha256
import json
from pathlib import Path
import unittest

from evaluate_civilian_access import score_access


FIXTURES = Path(__file__).resolve().parents[2] / "testdata/simulation-inputs"


class AccessEvaluationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.truth = (FIXTURES / "riyadh-access-answers.json").read_bytes()
        cls.submissions = (FIXTURES / "riyadh-access-submissions.json").read_bytes()

    def test_scores_independent_reopening_and_unresolved_cases(self):
        result = score_access(self.truth, self.submissions)
        self.assertEqual(result["exerciseSha256"], sha256((FIXTURES / "riyadh-access-exercise.json").read_bytes()).hexdigest())
        reviewed, early = (result["runs"][key] for key in ("fixture-reviewed", "fixture-early-clear"))
        self.assertEqual(reviewed["timeToVerifiedReopening"], {"p50Seconds": 3420, "p90Seconds": None})
        self.assertEqual(reviewed["qualifiedCases"], 2)
        self.assertEqual(reviewed["unresolvedAtCutoff"], 1)
        self.assertEqual(early["prematureReopenings"], 1)
        self.assertEqual(early["unscorableReopenings"], 1)
        self.assertEqual(early["excessAssignments"], 1)

    def test_rejects_moved_pin_and_partial_case_set(self):
        document = json.loads(self.submissions)
        original = document["exerciseSha256"]
        document["exerciseSha256"] = "1" * 64
        with self.assertRaisesRegex(ValueError, "different frozen exercise"):
            score_access(self.truth, json.dumps(document).encode())
        document["exerciseSha256"] = original
        document["runs"][0]["cases"].pop()
        with self.assertRaisesRegex(ValueError, "exactly the same"):
            score_access(self.truth, json.dumps(document).encode())
