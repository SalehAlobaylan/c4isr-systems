"""Automated unit & integration tests for the EMAD Sensor Coverage & Blind-Spot Optimizer."""
from __future__ import annotations

import base64
import json
from pathlib import Path
import tempfile
import unittest

import numpy as np

from sensor_coverage.contracts import (
    COV_ENGINE_VERSION,
    COVERAGE_REPLAY_SCHEMA,
    DERIVATION_LABEL,
    SENSOR_DEFAULTS,
    SensorSpec,
    ThreatSpec,
    WeatherSpec,
    curvature_drop_m,
    effective_range_m,
)
from sensor_coverage.grid import AnalysisGrid, ObstructionGrid
from sensor_coverage.service import CoverageService
from sensor_coverage.viewer.server import CoverageRouter
from sensor_coverage.viewshed import floor_agl_map


class TestPhysicalModels(unittest.TestCase):
    def test_effective_ranges_obey_physics(self) -> None:
        radar = SensorSpec("r1", "radar", 0, 0, 15, 10000, -1, 10)
        eo = SensorSpec("e1", "eo_ir", 0, 0, 10, 5000, -6, 0)
        ac = SensorSpec("a1", "acoustic", 0, 0, 4, 1500, -90, 0)

        # Radar R^4 law: 16x smaller RCS -> 2x shorter range
        r_1 = effective_range_m(radar, ThreatSpec(1.0), WeatherSpec())
        r_16 = effective_range_m(radar, ThreatSpec(1.0 / 16.0), WeatherSpec())
        self.assertAlmostEqual(r_1, 10000.0, places=3)
        self.assertAlmostEqual(r_16, 5000.0, places=3)

        # EO/IR clamped by dust/fog visibility
        eo_clear = effective_range_m(eo, ThreatSpec(0.1), WeatherSpec(visibility_km=12.0))
        eo_dust = effective_range_m(eo, ThreatSpec(0.1), WeatherSpec(visibility_km=1.5))
        self.assertAlmostEqual(eo_clear, 5000.0, places=3)
        self.assertAlmostEqual(eo_dust, 1350.0, places=3)

        # Acoustic degrades in high wind
        ac_calm = effective_range_m(ac, ThreatSpec(0.1), WeatherSpec(surface_wind_mps=0.0))
        ac_windy = effective_range_m(ac, ThreatSpec(0.1), WeatherSpec(surface_wind_mps=12.0))
        self.assertGreater(ac_calm, ac_windy)
        self.assertGreaterEqual(ac_windy, 1500.0 * 0.35 - 1e-6)

    def test_curvature_drop_positive(self) -> None:
        # At 10 km, 4/3-earth drop ~ 5.88 m
        drop = curvature_drop_m(10_000.0)
        self.assertGreater(drop, 5.5)
        self.assertLess(drop, 6.2)


class TestFixtureEndToEndAndRouter(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.runs_root = Path(self._tmp.name)
        self.service = CoverageService(runs_root=self.runs_root)
        self.router = CoverageRouter(service=self.service)

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_owned_fixture_baseline_optimization_and_router(self) -> None:
        envs = self.service.list_environments()
        fixture = next((e for e in envs if e["id"] == "owned-fixture-v1"), None)
        self.assertIsNotNone(fixture, "owned-fixture-v1 must be discovered")
        assert fixture is not None

        # Verify health & environments endpoints via socket-free CoverageRouter
        st, _, raw = self.router.resolve("/api/health")
        self.assertEqual(st, 200)
        health = json.loads(raw)
        self.assertEqual(health["status"], "ok")
        self.assertEqual(health["service"], "emad-sensor-coverage")
        self.assertEqual(health["engineVersion"], COV_ENGINE_VERSION)

        st_fav, _, _ = self.router.resolve("/favicon.ico")
        self.assertEqual(st_fav, 204)

        st_idx, ct_idx, html_raw = self.router.resolve("/")
        self.assertEqual(st_idx, 200)
        self.assertIn("text/html", ct_idx)
        self.assertIn(b"Sensor Coverage", html_raw)

        st_js, _, _ = self.router.resolve("/app.js")
        self.assertEqual(st_js, 200)

        st_trav, _, _ = self.router.resolve("/vendor/three/../../package.json")
        self.assertEqual(st_trav, 403)

        # Build draft with 1 locked sensor + 1 movable sensor + 1 extra gap-filler
        draft = json.loads(json.dumps(fixture["defaults"]))
        draft["sensors"] = draft["sensors"][:2]
        draft["sensors"][0]["locked"] = True
        draft["optimizer"]["enabled"] = True
        draft["optimizer"]["extraSensors"] = 1
        draft["optimizer"]["minSeparationM"] = 450.0
        draft["analysisCellM"] = 120.0
        draft["_sync"] = True

        st_post, _, post_raw = self.router.resolve(
            "/api/runs", method="POST", body=json.dumps(draft).encode("utf-8")
        )
        self.assertEqual(st_post, 202)
        job = json.loads(post_raw)
        self.assertEqual(job["status"], "completed", msg=job.get("error"))
        self.assertIsNotNone(job["summary"])
        self.assertIsNotNone(job["resultOverlay"])

        base_m = job["summary"]["baseline"]
        opt_m = job["summary"]["optimized"]
        self.assertIsNotNone(opt_m)
        # Adding an optimized extra sensor & relocating movable sensor should not decrease objective
        self.assertGreaterEqual(opt_m["objective"], base_m["objective"] - 1e-4)

        # Fetch full replay.json via router and inspect schema & binary grids
        st_rep, _, rep_raw = self.router.resolve(f"/api/runs/{job['jobId']}/replay.json")
        self.assertEqual(st_rep, 200)
        replay = json.loads(rep_raw)
        self.assertEqual(replay["schema"], COVERAGE_REPLAY_SCHEMA)
        self.assertEqual(replay["derivation"], DERIVATION_LABEL)

        g = replay["grid"]
        n_cells = g["nx"] * g["ny"]
        ground = np.frombuffer(base64.b64decode(g["groundB64"]), dtype=np.float32)
        valid = np.frombuffer(base64.b64decode(g["validB64"]), dtype=np.uint8)
        self.assertEqual(len(ground), n_cells)
        self.assertEqual(len(valid), n_cells)
        self.assertGreater(int(np.sum(valid)), 0)

        # Locked sensor must stay at its exact baseline ENU position
        base_locked = replay["configs"]["baseline"]["sensors"][0]
        opt_sensors = replay["configs"]["optimized"]["sensors"]
        self.assertEqual(len(opt_sensors), 3)  # 2 baseline + 1 extra
        opt_locked = next(s for s in opt_sensors if s["id"] == base_locked["id"])
        self.assertAlmostEqual(opt_locked["enu"][0], base_locked["enu"][0], places=1)
        self.assertAlmostEqual(opt_locked["enu"][1], base_locked["enu"][1], places=1)

        # Every sensor's floorB64 decodes cleanly to uint16[n_cells]
        for s in opt_sensors:
            fl = np.frombuffer(base64.b64decode(s["floorB64"]), dtype=np.uint16)
            self.assertEqual(len(fl), n_cells)
            self.assertTrue(np.any(fl < 65535))

        # Verify routes, altitude curve, and blind-cause breakdown were evaluated
        opt_metrics = replay["configs"]["optimized"]["metrics"]
        self.assertGreater(len(opt_metrics["routes"]), 0)
        self.assertEqual(len(opt_metrics["coverageCurve"]), 10)
        self.assertIn("buildingBlockedKm2", opt_metrics["blindCauses"])
        self.assertIn("terrainBlockedKm2", opt_metrics["blindCauses"])

        # Verify interactive POST /api/los-profile endpoint
        cx = 0.5 * (replay["environment"]["boundsEnu"]["eastMin"] + replay["environment"]["boundsEnu"]["eastMax"])
        cy = 0.5 * (replay["environment"]["boundsEnu"]["northMin"] + replay["environment"]["boundsEnu"]["northMax"])
        st_los, _, los_raw = self.router.resolve(
            "/api/los-profile",
            method="POST",
            body=json.dumps(
                {
                    "environmentId": "owned-fixture-v1",
                    "targetEast": cx,
                    "targetNorth": cy,
                    "targetAglM": 30.0,
                    "sensors": opt_sensors,
                }
            ).encode("utf-8"),
        )
        self.assertEqual(st_los, 200)
        los_data = json.loads(los_raw)
        self.assertEqual(len(los_data["rays"]), len(opt_sensors))
        self.assertGreaterEqual(len(los_data["rays"][0]["profile"]), 4)

        # Verify interactive POST /api/preview-sensor endpoint (~15 ms sandbox drop)
        st_prev, _, prev_raw = self.router.resolve(
            "/api/preview-sensor",
            method="POST",
            body=json.dumps(
                {
                    "environmentId": "owned-fixture-v1",
                    "type": "radar",
                    "east": cx,
                    "north": cy,
                    "mastAglM": 20.0,
                    "gridCellM": g["cellM"],
                }
            ).encode("utf-8"),
        )
        self.assertEqual(st_prev, 200)
        prev = json.loads(prev_raw)
        self.assertEqual(prev["origin"], "preview")
        self.assertEqual(prev["gridNx"], g["nx"])
        self.assertEqual(prev["gridNy"], g["ny"])
        prev_floor = np.frombuffer(base64.b64decode(prev["floorB64"]), dtype=np.uint16)
        self.assertEqual(len(prev_floor), n_cells)
        self.assertTrue(np.any(prev_floor < 65535))

    def test_mast_height_and_acoustic_nlos_properties(self) -> None:
        _, env, obs = self.service._get_env("owned-fixture-v1")
        grid = AnalysisGrid.build(obs, cell_m=120.0)
        cx = 0.5 * (env.east_min + env.east_max)
        cy = 0.5 * (env.north_min + env.north_max)

        low_radar = SensorSpec("r_low", "radar", cx, cy, 4.0, 5000.0, -1.0, 10.0)
        high_radar = SensorSpec("r_high", "radar", cx, cy, 45.0, 5000.0, -1.0, 10.0)
        f_low = floor_agl_map(obs, grid, low_radar, 5000.0, include_buildings=True)
        f_high = floor_agl_map(obs, grid, high_radar, 5000.0, include_buildings=True)
        # Higher mast should detect at least as many cells at 40m AGL
        self.assertGreaterEqual(int(np.sum(f_high <= 40.0)), int(np.sum(f_low <= 40.0)))

        # Acoustic sensor covers all valid cells inside its radius regardless of terrain/buildings
        ac = SensorSpec("ac1", "acoustic", cx, cy, 4.0, 1200.0, -90.0, 0.0)
        f_ac = floor_agl_map(obs, grid, ac, 1200.0, include_buildings=True)
        dist = np.hypot(grid.east - cx, grid.north - cy)
        expected_in = grid.valid & (dist <= 1200.0)
        self.assertTrue(np.array_equal(f_ac <= 10.0, expected_in))


if __name__ == "__main__":
    unittest.main()
