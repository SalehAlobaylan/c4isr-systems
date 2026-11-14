from datetime import datetime, timedelta, timezone
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

import qualify_environment_library as qualification


class EnvironmentObservationTest(unittest.TestCase):
    def test_api_restart_ends_candidate_observation(self):
        pinned = {"name": "api", "startedAt": "before"}
        targets = [{"id": name} for name in ("riyadh", "al-ain", "avignon")]
        with (
            TemporaryDirectory() as folder,
            patch("capture_environment_candidate.container", side_effect=[pinned, {**pinned, "startedAt": "after"}]),
            patch.object(qualification, "observe", return_value=(0.01, "STAGED")),
            patch.object(qualification.time, "sleep"),
        ):
            report = qualification.soak(None, targets, 86400, 30, Path(folder) / "soak.json", {"STAGED"}, pinned, "a" * 64)
        self.assertEqual(report["state"], "GATE_UNMET")
        self.assertEqual(report["requests"], 5)
        self.assertIn("generation changed", report["interruption"])
        self.assertEqual(report["installationSha256"], "a" * 64)
        self.assertEqual(report["apiGeneration"], pinned)

    def test_stale_running_report_is_interrupted_without_rewriting_evidence(self):
        checked = datetime(2026, 11, 12, tzinfo=timezone.utc)
        with TemporaryDirectory() as folder:
            path = Path(folder) / "soak.json"
            qualification.record(path, {"schema": "emad.environment-soak.v1", "state": "RUNNING",
                                        "checkedAt": checked.isoformat(), "sampleGapLimitSeconds": 70})
            original = path.read_bytes()
            self.assertEqual(qualification.observation_status(path, checked + timedelta(seconds=70))["state"], "RUNNING")
            interrupted = qualification.observation_status(path, checked + timedelta(seconds=71))
            self.assertEqual(interrupted["state"], "INTERRUPTED")
            self.assertEqual(interrupted["lastSampleAgeSeconds"], 71)
            self.assertEqual(path.read_bytes(), original)

    def test_suspended_observer_does_not_pass_after_wall_clock_duration(self):
        start = datetime(2026, 11, 12, tzinfo=timezone.utc)
        targets = [{"id": name} for name in ("jeddah", "al-ain", "avignon")]
        with (
            TemporaryDirectory() as folder,
            patch.object(qualification, "observe", return_value=(0.01, "RELEASED")),
            patch.object(qualification, "datetime") as clock,
            patch.object(qualification.time, "monotonic", side_effect=[0, 86401]),
        ):
            clock.now.side_effect = [start, start + timedelta(seconds=86401)]
            report = qualification.soak(None, targets, 86400, 30, Path(folder) / "soak.json", {"RELEASED"})

        self.assertEqual(report["state"], "GATE_UNMET")
        self.assertEqual(report["failures"], 0)
        self.assertEqual(report["sampleGaps"], 1)
