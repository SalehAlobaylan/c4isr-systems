"""Deterministic unit and end-to-end tests for the EMAD 3D simulation core.

All tests run exclusively against the git-committed owned fixture
(``testdata/geospatial/owned-fixture-v1``) so they execute offline in CI
without requiring private Riyadh packages.
"""
from __future__ import annotations

import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest

import numpy as np

from sim.atmosphere import AtmosphereModel, pressure_hpa_to_isa_altitude_m
from sim.contracts import (
    AtmosphereOverrideConfig,
    SCOPE_EVALUATION,
    SCOPE_EXERCISE_INPUT,
    SCOPE_EXERCISE_OBSERVATION,
    SCOPE_EXERCISE_TRUTH,
    ScenarioSpec,
    ShrapnelConfig,
    ThreatConfig,
)
from sim.engine import compare_policies, run_simulation, save_run_bundle
from sim.environment import EnvironmentModel
from sim.physics import build_threat_trajectory, simulate_shrapnel_cloud
from sim.service import SimulationService
from sim.viewer.server import ReplayRouter

REPO_ROOT = Path(__file__).resolve().parents[2]
FIXTURE_ZIP = REPO_ROOT / "testdata" / "geospatial" / "owned-fixture-v1" / "prepared.zip"
FIXTURE_ERA5 = REPO_ROOT / "testdata" / "geospatial" / "owned-fixture-v1" / "field-era5.json"
FIXTURE_SCENARIO = REPO_ROOT / "sim" / "scenarios" / "fixture_intercept.json"
FIXTURE_MANIFEST_SHA = "aaf9f14b027749b5d233018606fb90c81e34808b9522e034fc67b5243c966bdc"


class TestEnvironmentAndAtmosphere(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.env = EnvironmentModel(
            FIXTURE_ZIP,
            expected_manifest_sha256=FIXTURE_MANIFEST_SHA,
            derived_building_height_m=12.0,
        )

    def test_manifest_verification_rejects_bad_digest(self) -> None:
        with self.assertRaises(ValueError):
            EnvironmentModel(FIXTURE_ZIP, expected_manifest_sha256="0" * 64)

    def test_terrain_heights_and_vectorized_lookup(self) -> None:
        h_origin = self.env.height_at(0.0, 0.0)
        self.assertTrue(np.isfinite(h_origin))
        # Batch lookup must match scalar lookup
        es = np.array([0.0, -500.0, 500.0])
        ns = np.array([0.0, -500.0, -500.0])
        hs = self.env.heights_at(es, ns)
        self.assertEqual(hs.shape, (3,))
        self.assertAlmostEqual(float(hs[0]), h_origin, places=6)

    def test_line_of_sight_clear_and_terrain_blocked(self) -> None:
        g = self.env.height_at(0.0, -1000.0, fallback=self.env.up_median)
        clear = self.env.line_of_sight(
            (0.0, -1000.0, g + 120.0),
            (500.0, -1000.0, g + 150.0),
            include_buildings=False,
        )
        self.assertTrue(clear["clear"])

        # Ray passing beneath surface elevation must be blocked by terrain
        blocked = self.env.line_of_sight(
            (0.0, -1000.0, g - 25.0),
            (500.0, -1000.0, g - 25.0),
            include_buildings=False,
        )
        self.assertFalse(blocked["clear"])
        self.assertEqual(blocked["blockedBy"], "terrain")

    def test_atmosphere_known_at_cutoff_and_pressure_altitude(self) -> None:
        alt_850 = pressure_hpa_to_isa_altitude_m(850.0)
        alt_700 = pressure_hpa_to_isa_altitude_m(700.0)
        self.assertGreater(alt_700, alt_850)
        self.assertTrue(1300.0 < alt_850 < 1600.0)

        # Requesting a future validTime before known_at must raise ValueError
        with self.assertRaises(ValueError):
            AtmosphereModel.load(
                self.env,
                external_path=FIXTURE_ERA5,
                override=AtmosphereOverrideConfig(
                    valid_time="2023-01-09T12:00:00Z",
                    known_at="2022-10-23T12:00:00Z",
                ),
            )

        atm = AtmosphereModel.load(
            self.env,
            external_path=FIXTURE_ERA5,
            override=AtmosphereOverrideConfig(valid_time="2022-10-23T12:00:00Z"),
        )
        we, wn, wu, rho, tk = atm.sample(np.array([0.0]), np.array([0.0]), np.array([1500.0]))
        self.assertTrue(np.isfinite(we[0]) and np.isfinite(wn[0]))
        self.assertTrue(0.7 < float(rho[0]) < 1.3)
        self.assertTrue(250.0 < float(tk[0]) < 330.0)


class TestShrapnelPhysics(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.env = EnvironmentModel(FIXTURE_ZIP, expected_manifest_sha256=FIXTURE_MANIFEST_SHA)

    def test_crosswind_shifts_shrapnel_centroid_downwind(self) -> None:
        cfg = ShrapnelConfig(fragment_count=80)
        g = self.env.height_at(0.0, -1000.0, fallback=self.env.up_median)
        burst = np.array([0.0, -1000.0, g + 450.0])
        vel = np.array([0.0, 0.0, 0.0])

        atm_calm = AtmosphereModel.load(
            self.env,
            external_path=FIXTURE_ERA5,
            override=AtmosphereOverrideConfig(wind_scale=0.0, wind_add_enu_mps=[0.0, 0.0, 0.0]),
        )
        atm_east = AtmosphereModel.load(
            self.env,
            external_path=FIXTURE_ERA5,
            override=AtmosphereOverrideConfig(wind_scale=0.0, wind_add_enu_mps=[18.0, 0.0, 0.0]),
        )
        res_calm = simulate_shrapnel_cloud(burst, vel, cfg, self.env, atm_calm, seed=77, record_trails=False)
        res_east = simulate_shrapnel_cloud(burst, vel, cfg, self.env, atm_east, seed=77, record_trails=False)

        east_shift = res_east["footprint"]["centroidEnu"][0] - res_calm["footprint"]["centroidEnu"][0]
        self.assertGreater(east_shift, 120.0, f"Expected substantial eastward wind drift, got {east_shift:.1f}m")

    def test_shrapnel_kinetic_energy_hazard_and_dual_origin(self) -> None:
        cfg = ShrapnelConfig(fragment_count=100, interceptor_fragment_fraction=0.25, hazardous_energy_j=80.0)
        g = self.env.height_at(0.0, -1000.0, fallback=self.env.up_median)
        burst = np.array([0.0, -1000.0, g + 350.0])
        threat_vel = np.array([65.0, 20.0, -15.0])
        int_vel = np.array([-220.0, 140.0, 90.0])
        atm = AtmosphereModel.load(self.env, external_path=FIXTURE_ERA5)

        res = simulate_shrapnel_cloud(
            burst, threat_vel, cfg, self.env, atm, seed=99, record_trails=False, interceptor_vel=int_vel
        )
        origins = {imp["origin"] for imp in res["fragments"]}
        self.assertEqual(origins, {"threat", "interceptor"})
        self.assertTrue(all(imp["massKg"] > 0 and imp["impactEnergyJ"] > 0 for imp in res["fragments"]))
        exp = res["exposure"]
        self.assertGreater(exp["threatDebrisCount"], 50)
        self.assertGreater(exp["interceptorDebrisCount"], 10)
        self.assertGreater(exp["hazardousHits"], 0)
        self.assertGreater(exp["maxImpactEnergyJ"], 80.0)

    def test_ballistic_missile_trajectory_reaches_high_apogee(self) -> None:
        cfg = ThreatConfig(
            id="ballistic-test",
            kind="ballistic_missile",
            waypoints_enu=[[-1500.0, -1500.0, 120.0], [800.0, 800.0, 40.0]],
            speed_mps=340.0,
            ballistic_apogee_m=1500.0,
        )
        traj = build_threat_trajectory(cfg, self.env, dt_s=0.2, duration_s=45.0)
        self.assertGreater(len(traj), 20)
        agls = [
            float(p.pos[2]) - self.env.height_at(float(p.pos[0]), float(p.pos[1]), fallback=self.env.up_median)
            for p in traj
        ]
        max_agl = max(agls)
        self.assertGreater(max_agl, 1300.0, f"Expected high ballistic apogee, got {max_agl:.1f}m AGL")
        self.assertLess(agls[-1], max_agl - 600.0)


class TestEndToEndAndReplayServer(unittest.TestCase):
    def test_determinism_partition_isolation_and_policy_comparison(self) -> None:
        spec = ScenarioSpec.from_dict(json.loads(FIXTURE_SCENARIO.read_text(encoding="utf-8")))
        run_a = run_simulation(spec, base_dir=FIXTURE_SCENARIO.parent)
        run_b = run_simulation(spec, base_dir=FIXTURE_SCENARIO.parent)

        # Byte-identical deterministic evaluation & observations for identical seed
        self.assertEqual(run_a["evaluation"], run_b["evaluation"])
        self.assertEqual(run_a["observations"], run_b["observations"])

        # Partition scopes strictly labeled
        self.assertEqual(run_a["manifest"]["scope"], SCOPE_EXERCISE_INPUT)
        self.assertEqual(run_a["truth"]["scope"], SCOPE_EXERCISE_TRUTH)
        self.assertEqual(run_a["observations"]["scope"], SCOPE_EXERCISE_OBSERVATION)
        self.assertEqual(run_a["evaluation"]["scope"], SCOPE_EVALUATION)

        # EXERCISE_OBSERVATION must NEVER leak raw actor IDs
        obs_text = json.dumps(run_a["observations"])
        self.assertNotIn(spec.threat.id, obs_text)
        self.assertGreater(len(run_a["observations"]["detections"]), 3)
        self.assertTrue(run_a["evaluation"]["interception"]["intercepted"])

        # Compare earliest vs safe_window and verify bundle + local HTTP viewer server
        runs, comparison = compare_policies(spec, base_dir=FIXTURE_SCENARIO.parent)
        self.assertEqual(len(runs), 2)
        self.assertIn("earliest", comparison["policies"])
        self.assertIn("safe_window", comparison["policies"])

        with TemporaryDirectory(prefix="emad-sim-test-") as tmp:
            out_dir = Path(tmp)
            replay_path = save_run_bundle(runs, out_dir, comparison_summary=comparison)
            self.assertTrue(replay_path.is_file())
            self.assertTrue((out_dir / "earliest" / "truth.json").is_file())
            self.assertTrue((out_dir / "safe_window" / "observations.json").is_file())

            # Verify viewer routes, MIME types, replay bundle, and traversal protection offline
            router = ReplayRouter(out_dir)
            status, ctype, html_bytes = router.resolve("/")
            self.assertEqual(status, 200)
            self.assertIn("text/html", ctype)
            self.assertIn("EMAD 3D Simulation Workbench", html_bytes.decode("utf-8"))

            status, _, _ = router.resolve("/favicon.ico")
            self.assertEqual(status, 204)

            status, ctype, js_bytes = router.resolve("/app.js")
            self.assertEqual(status, 200)
            self.assertIn("javascript", ctype)
            self.assertIn("enuToThree", js_bytes.decode("utf-8"))

            status, _, replay_bytes = router.resolve("/data/replay.json")
            self.assertEqual(status, 200)
            payload = json.loads(replay_bytes.decode("utf-8"))
            self.assertEqual(payload["schema"], "emad.sim-replay.v1")
            self.assertIn("safe_window", payload["runs"])
            sw = payload["runs"]["safe_window"]
            self.assertTrue(len(sw.get("timelineEvents", [])) >= 2)
            cands = sw.get("optimizerCandidates", [])
            self.assertTrue(len(cands) >= 1)
            self.assertEqual(sum(1 for c in cands if c.get("selected")), 1)

            status, _, _ = router.resolve("/vendor/three/../../package.json")
            self.assertEqual(status, 403)

    def test_stateful_simulation_service_and_map_wgs84_workflow(self) -> None:
        with TemporaryDirectory(prefix="emad-sim-svc-") as tmp:
            svc = SimulationService(
                repo_root=REPO_ROOT,
                runs_root=Path(tmp),
                public_origin="http://127.0.0.1:8765",
            )
            router = ReplayRouter(service=svc)

            st, _, body = router.resolve("/api/health")
            self.assertEqual(st, 200)
            self.assertEqual(json.loads(body)["status"], "ok")

            st, _, body = router.resolve("/api/environments")
            self.assertEqual(st, 200)
            envs = json.loads(body)["environments"]
            fixture_env = next((e for e in envs if e["id"] == "owned-fixture-v1"), None)
            self.assertIsNotNone(fixture_env)
            assert fixture_env is not None

            # Reject out-of-bounds WGS84 coordinates with clear 400 error
            bad_payload = {
                "environmentId": "owned-fixture-v1",
                "radar": {"lon": 0.0, "lat": 0.0, "heightAglM": 15.0},
            }
            st, _, err_body = router.resolve("/api/runs", method="POST", body=json.dumps(bad_payload).encode("utf-8"))
            self.assertEqual(st, 400)
            self.assertIn("outside", json.loads(err_body)["error"].lower())

            # Submit valid map WGS84 setup with client-supplied jobId synchronously
            defaults = fixture_env["defaults"]
            req_payload = {
                "jobId": "simjob_aabbccddeeff",
                "environmentId": "owned-fixture-v1",
                "mode": "compare",
                "seed": 12345,
                "radar": defaults["radar"],
                "launcher": defaults["launcher"],
                "protectedZone": defaults["protectedZone"],
                "threat": defaults["threat"],
                "shrapnel": {"fragmentCount": 60},
                "atmosphere": defaults["atmosphere"],
                "_sync": True,
            }
            st, _, created_body = router.resolve(
                "/api/runs", method="POST", body=json.dumps(req_payload).encode("utf-8")
            )
            self.assertEqual(st, 202)
            job = json.loads(created_body)
            self.assertEqual(job["jobId"], "simjob_aabbccddeeff")
            self.assertEqual(job["status"], "completed")
            self.assertTrue(job["viewerUrl"].startswith("http://127.0.0.1:8765/?run=simjob_"))
            self.assertIsNotNone(job["resultOverlay"])
            self.assertGreaterEqual(len(job["resultOverlay"]["features"]), 2)

            # Verify run status & replay endpoints
            st, _, run_body = router.resolve(f"/api/runs/{job['jobId']}")
            self.assertEqual(st, 200)
            self.assertEqual(json.loads(run_body)["jobId"], job["jobId"])

            st, _, rep_body = router.resolve(f"/api/runs/{job['jobId']}/replay.json")
            self.assertEqual(st, 200)
            replay_doc = json.loads(rep_body)
            self.assertIn("earliest", replay_doc["runs"])
            self.assertIn("safe_window", replay_doc["runs"])
            self.assertIsNotNone(replay_doc.get("sourceDraft"))
            self.assertEqual(replay_doc["sourceDraft"]["environmentId"], "owned-fixture-v1")


if __name__ == "__main__":
    unittest.main()
