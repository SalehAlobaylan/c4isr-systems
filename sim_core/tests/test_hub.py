"""Unit & integration tests for EMAD's Unified Simulation Compute Hub & Truth Filter."""
from __future__ import annotations

import json
from pathlib import Path
import tempfile
import unittest

from sim_core.cache import SharedBiomeCache
from sim_core.hub import HubRouter, UnifiedSimHub
from sim_core.truth_filter import filter_salvo_replay, filter_sim_replay

REPO_ROOT = Path(__file__).resolve().parents[2]


class TestUnifiedHubAndCache(unittest.TestCase):
    def test_shared_biome_cache_deduplicates_across_engines(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            hub = UnifiedSimHub(
                repo_root=REPO_ROOT,
                runs_base=Path(tmp),
                max_loaded_biomes=2,
            )
            try:
                _, env_sim = hub.sim._get_env("owned-fixture-v1")
                _, env_cov, obs_cov = hub.coverage._get_env("owned-fixture-v1")
                _, env_salvo = hub.salvo._get_env("owned-fixture-v1")
                _, env_bench = hub.bench._get_env("owned-fixture-v1")

                # Exact same in-memory EnvironmentModel singleton across all 4 engines!
                self.assertIs(env_sim, env_cov)
                self.assertIs(env_sim, env_salvo)
                self.assertIs(env_sim, env_bench)
                self.assertIsNotNone(obs_cov)

                stats = hub.cache.stats()
                self.assertEqual(stats["misses"], 1)
                self.assertGreaterEqual(stats["hits"], 3)
                self.assertIn("owned-fixture-v1", stats["obstructionGridsLoaded"])
            finally:
                hub.shutdown()

    def test_hub_router_worker_secret_and_namespaced_endpoints(self) -> None:
        secret = "test-loopback-worker-secret-0123456789"
        with tempfile.TemporaryDirectory() as tmp:
            hub = UnifiedSimHub(
                repo_root=REPO_ROOT,
                runs_base=Path(tmp),
                worker_secret=secret,
            )
            try:
                router = HubRouter(hub)

                # Unauthenticated health probe succeeds with redacted details
                st, _, data = router.resolve("/api/health", method="GET")
                self.assertEqual(st, 200)
                h = json.loads(data)
                self.assertEqual(h["service"], "emad-sim-hub")
                self.assertTrue(h["authRequired"])
                self.assertNotIn("cache", h)

                # Unauthenticated API call is blocked (401)
                st_unauth, _, _ = router.resolve("/api/sim/runs", method="GET")
                self.assertEqual(st_unauth, 401)
                st_wrong, _, _ = router.resolve(
                    "/api/sim/runs",
                    method="GET",
                    headers={"X-EMAD-Worker-Secret": "wrong-secret"},
                )
                self.assertEqual(st_wrong, 401)

                # Authenticated calls work across all 4 engine namespaces
                hdrs = {"X-EMAD-Worker-Secret": secret}
                st_h, _, data_h = router.resolve("/api/health", method="GET", headers=hdrs)
                self.assertEqual(st_h, 200)
                self.assertIn("cache", json.loads(data_h))

                st_b, _, data_b = router.resolve("/api/biomes", method="GET", headers=hdrs)
                self.assertEqual(st_b, 200)
                self.assertGreaterEqual(len(json.loads(data_b)["biomes"]), 1)

                st_m, _, _ = router.resolve("/api/bench/models", method="GET", headers=hdrs)
                self.assertEqual(st_m, 200)

                # All 3 viewer HTML shells serve cleanly, and missing trailing slash redirects cleanly
                for eng in ("sim", "coverage", "salvo"):
                    st_red, ctype_red, _ = router.resolve(f"/viewer/{eng}?run=test123", method="GET")
                    self.assertEqual(st_red, 302)
                    self.assertEqual(ctype_red, f"redirect:/viewer/{eng}/?run=test123")
                    sv, ctype, html = router.resolve(f"/viewer/{eng}/", method="GET")
                    self.assertEqual(sv, 200)
                    self.assertIn("text/html", ctype)
                    self.assertGreater(len(html), 200)
                    sh, _, hdata = router.resolve(f"/api/{eng}/health", method="GET", headers=hdrs)
                    self.assertEqual(sh, 200)
                    self.assertTrue(json.loads(hdata)["viewerOrigin"].endswith(f"/viewer/{eng}"))
            finally:
                hub.shutdown()

    def test_hub_executes_sim_and_salvo_and_enforces_operator_truth_filter(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            hub = UnifiedSimHub(repo_root=REPO_ROOT, runs_base=Path(tmp))
            try:
                router = HubRouter(hub)

                # 1. Run Intercept sim synchronously via hub
                body_sim = json.dumps({"environmentId": "owned-fixture-v1", "mode": "compare", "_sync": True}).encode()
                st, _, data = router.resolve("/api/sim/runs", method="POST", body=body_sim)
                self.assertEqual(st, 202)
                sim_job = json.loads(data)
                self.assertEqual(sim_job["status"], "completed")
                self.assertIn("/viewer/sim/?run=", sim_job["viewerUrl"])

                # Debrief view has full truth + evaluation
                st_d, _, d_raw = router.resolve(f"/api/sim/runs/{sim_job['jobId']}/replay.json?view=debrief")
                self.assertEqual(st_d, 200)
                debrief = json.loads(d_raw)
                first_pol = next(iter(debrief["runs"].values()))
                self.assertIn("truth", first_pol)
                self.assertIsNotNone(first_pol["evaluation"])

                # Operator view MUST strip all EXERCISE_TRUTH & EVALUATION fields
                st_o, _, o_raw = router.resolve(f"/api/sim/runs/{sim_job['jobId']}/replay.json?view=operator")
                self.assertEqual(st_o, 200)
                op_doc = json.loads(o_raw)
                self.assertEqual(op_doc["viewScope"], "EXERCISE_OBSERVATION")
                self.assertTrue(op_doc["truthRedacted"])
                self.assertIsNone(op_doc["comparison"])
                for run in op_doc["runs"].values():
                    self.assertNotIn("truth", run)
                    self.assertIsNone(run["evaluation"])
                    self.assertIn("observations", run)
                    self.assertIn("ownForceTelemetry", run)
                    self.assertNotIn("firstClearLosS", run["timelineEvents"])

                # 2. Run Salvo defense synchronously via hub
                body_salvo = json.dumps(
                    {
                        "environmentId": "owned-fixture-v1",
                        "durationS": 55.0,
                        "raid": {"decoys": 1, "uavs": 2, "cruiseMissiles": 0, "ballisticMissiles": 0, "waves": 1},
                        "_sync": True,
                    }
                ).encode()
                st_s, _, s_data = router.resolve("/api/salvo/runs", method="POST", body=body_salvo)
                self.assertEqual(st_s, 202)
                salvo_job = json.loads(s_data)
                self.assertEqual(salvo_job["status"], "completed")
                self.assertIn("/viewer/salvo/?run=", salvo_job["viewerUrl"])

                # Operator view on Salvo strips sealed raid order-of-battle, truth trajectories & truth kill events
                st_so, _, so_raw = router.resolve(f"/api/salvo/runs/{salvo_job['jobId']}/replay.json?view=operator")
                self.assertEqual(st_so, 200)
                so_doc = json.loads(so_raw)
                self.assertEqual(so_doc["viewScope"], "EXERCISE_OBSERVATION")
                self.assertTrue(so_doc["truthRedacted"])
                self.assertIsNone(so_doc["comparison"])
                self.assertNotIn("threats", so_doc.get("scenario") or {})
                for run in so_doc["runs"].values():
                    self.assertNotIn("truth", run)
                    self.assertIsNone(run["evaluation"])
                    self.assertIn("observations", run)
                    self.assertIn("ownForceTelemetry", run)
                    for ev in run["events"]:
                        self.assertNotIn(ev["kind"], ("leaker_impact", "intercept_kill", "intercept_miss"))
            finally:
                hub.shutdown()


if __name__ == "__main__":
    unittest.main()
