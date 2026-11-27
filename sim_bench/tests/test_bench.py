"""Automated tests for EMAD Monte Carlo Debris, ML Surrogate, RL Env & Benchmark Suite."""
from __future__ import annotations

import json
import unittest

from sim_bench.server import BenchRouter
from sim_bench.service import BenchService


class TestSimBench(unittest.TestCase):
    def setUp(self) -> None:
        self.service = BenchService()
        self.router = BenchRouter(self.service)

    def test_health_models_and_monte_carlo_router(self) -> None:
        st_h, _, raw_h = self.router.resolve("/api/health")
        self.assertEqual(st_h, 200)
        h = json.loads(raw_h)
        self.assertEqual(h["service"], "emad-sim-bench")

        st_m, _, raw_m = self.router.resolve("/api/models")
        self.assertEqual(st_m, 200)
        models = json.loads(raw_m)
        self.assertIn("debrisSurrogate", models)
        self.assertIn("aiWtaPolicy", models)

        # Run fast vectorized Monte Carlo debris heatmap on owned-fixture-v1
        st_mc, _, raw_mc = self.router.resolve(
            "/api/monte-carlo",
            method="POST",
            body=json.dumps(
                {
                    "environmentId": "owned-fixture-v1",
                    "trials": 60,
                    "fragmentsPerTrial": 28,
                    "interceptAglM": 500.0,
                    "threatSpeedMps": 110.0,
                    "windSigmaMps": 4.0,
                }
            ).encode("utf-8"),
        )
        self.assertEqual(st_mc, 200)
        mc = json.loads(raw_mc)
        self.assertEqual(mc["trials"], 60)
        self.assertGreater(mc["totalTrajectories"], 1000)
        self.assertIn("buildingHits", mc)
        self.assertIn("contoursGeoJson", mc)
        self.assertEqual(mc["contoursGeoJson"]["type"], "FeatureCollection")

    def test_trained_ml_surrogate_and_rl_policy_when_present(self) -> None:
        import numpy as np
        from sim_bench.rl_env import SalvoRLEnv
        from sim_bench.surrogate import load_debris_surrogate
        from salvo_defense.wta import load_ai_wta_model

        surrogate = load_debris_surrogate()
        if surrogate is not None:
            preset, env = self.service._get_env("owned-fixture-v1")
            _ = preset
            env.obstruction_grid()  # warm cached spatial grid once
            g = env.height_at(0.0, 0.0, fallback=env.up_median)
            pred = surrogate.predict_exposure(
                intercept_enu=np.array([0.0, 0.0, g + 500.0]),
                threat_vel_enu=np.array([100.0, 0.0, -5.0]),
                wind_en=(12.0, -3.0),
                agl_m=500.0,
                env=env,
            )
            self.assertEqual(pred["derivation"], "derived/ml-debris-surrogate-v1")
            self.assertLess(pred["inferenceMs"], 2.0)
            self.assertGreaterEqual(pred["urbanExposureFraction"], 0.0)
            self.assertLessEqual(pred["urbanExposureFraction"], 1.0)

        ai_model = load_ai_wta_model()
        if ai_model is not None:
            rl = SalvoRLEnv(env_ids=["owned-fixture-v1"], surrogate=surrogate)
            ev_g = rl.evaluate_episode(env_id="owned-fixture-v1", seed=4242, policy="greedy_earliest")
            ev_ai = rl.evaluate_episode(env_id="owned-fixture-v1", seed=4242, policy="ai_policy")
            self.assertEqual(ev_ai["airspaceViolations"], 0)
            self.assertGreaterEqual(ev_ai["compositeScore"], ev_g["compositeScore"] - 5.0)


if __name__ == "__main__":
    unittest.main()
