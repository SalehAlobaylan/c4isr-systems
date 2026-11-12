from datetime import datetime, timedelta, timezone
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

import qualify_environment_library as qualification


class EnvironmentObservationTest(unittest.TestCase):
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
