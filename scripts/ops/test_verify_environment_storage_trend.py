from datetime import datetime, timedelta, timezone
import unittest

from observe_environment_storage import GAUGES
from verify_environment_storage_trend import verify


class EnvironmentStorageTrendTest(unittest.TestCase):
    def setUp(self):
        start = datetime(2026, 11, 14, tzinfo=timezone.utc)
        pins = [{"id": "riyadh", "sha256": "a" * 64}]
        self.candidate = {"installationSha256": "b" * 64, "releases": pins}
        self.soak = {"startedAt": start.isoformat(), "checkedAt": (start + timedelta(days=1)).isoformat(),
                     "state": "PRE_RELEASE_OBSERVATION_PASS", "elapsedSeconds": 86400, "failures": 0,
                     "sampleGaps": 0, "candidate": pins}
        self.samples = [{"at": (start + timedelta(minutes=minute)).isoformat(), "status": "OK",
                        "installationSha256": self.candidate["installationSha256"],
                        "gauges": {key: 4096 if key == "retained_asset_bytes" else 0 for key in GAUGES}}
                        for minute in range(1, 1441)]

    def test_exact_idle_window_passes(self):
        self.assertEqual(verify(self.candidate, self.soak, self.samples)["retainedBytes"], 4096)

    def test_samples_cannot_bridge_an_interruption(self):
        del self.samples[100:103]
        with self.assertRaisesRegex(ValueError, "missing"):
            verify(self.candidate, self.soak, self.samples)

    def test_storage_growth_keeps_gate_open(self):
        self.samples[-1]["gauges"]["retained_asset_bytes"] += 1
        with self.assertRaisesRegex(ValueError, "changed"):
            verify(self.candidate, self.soak, self.samples)

    def test_bound_storage_rejects_samples_from_another_api_generation(self):
        generation = {"startedAt": self.soak["startedAt"]}
        self.candidate["services"] = {"api-1": generation}
        self.soak.update(apiGeneration=generation, installationSha256=self.candidate["installationSha256"])
        for sample in self.samples:
            sample["apiStartedAt"] = generation["startedAt"]
        self.assertEqual(verify(self.candidate, self.soak, self.samples)["sampleCount"], 1440)
        self.samples[0]["apiStartedAt"] = "different generation"
        with self.assertRaisesRegex(ValueError, "missing"):
            verify(self.candidate, self.soak, self.samples)
