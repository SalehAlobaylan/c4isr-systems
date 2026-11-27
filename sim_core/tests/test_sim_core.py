"""Unit & integration tests for EMAD's shared simulation kernel (``sim_core``)."""
from __future__ import annotations

from pathlib import Path
import tempfile
import unittest

import numpy as np

from sim_core.atmosphere import AtmosphereModel, cams_dust_to_visibility_km, load_cams_dust
from sim_core.environment import EnvironmentModel
from sim_core.handoff import SENSOR_NETWORK_SCHEMA, SensorNetworkStore
from sim_core.registry import discover_biomes

REPO_ROOT = Path(__file__).resolve().parents[2]


class TestSimCore(unittest.TestCase):
    def test_cams_dust_koschmieder_conversion(self) -> None:
        v_clear = cams_dust_to_visibility_km(0.0)
        v_haze = cams_dust_to_visibility_km(1000.0)
        v_storm = cams_dust_to_visibility_km(4717.0)
        self.assertGreater(v_clear, 25.0)
        self.assertGreater(v_clear, v_haze)
        self.assertGreater(v_haze, v_storm)
        self.assertLess(v_storm, 2.0)

    def test_sensor_network_handoff_roundtrip(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            store = SensorNetworkStore(Path(tmp))
            saved = store.save(
                name="Riyadh Optimized Net",
                environment_id="riyadh-observation-v1",
                manifest_sha256="abc123",
                sensors=[
                    {"id": "r1", "type": "radar", "label": "North Radar", "lon": 46.76, "lat": 24.71, "mastAglM": 18}
                ],
                source_job_id="covjob_1234567890ab",
            )
            self.assertEqual(saved["schema"], SENSOR_NETWORK_SCHEMA)
            self.assertEqual(saved["sensorCount"], 1)
            loaded = store.get(saved["layoutId"])
            self.assertIsNotNone(loaded)
            assert loaded is not None
            self.assertEqual(loaded["sensors"][0]["label"], "North Radar")
            self.assertEqual(len(store.list(environment_id="riyadh-observation-v1")), 1)
            self.assertEqual(len(store.list(environment_id="other")), 0)

    def test_owned_fixture_and_fast_obstruction_los(self) -> None:
        biomes = discover_biomes(REPO_ROOT)
        fixture = next((b for b in biomes if b.id == "owned-fixture-v1"), None)
        self.assertIsNotNone(fixture)
        assert fixture is not None

        env = EnvironmentModel(fixture.package_path, expected_manifest_sha256=fixture.expected_manifest_sha256)
        obs = env.obstruction_grid()
        cx = 0.5 * (env.east_min + env.east_max)
        cy = 0.5 * (env.north_min + env.north_max)
        g = env.height_at(cx, cy, fallback=env.up_median)

        # High-altitude clear ray
        clear_los = env.line_of_sight((cx - 500, cy, g + 150), (cx + 500, cy, g + 150), use_grid=True)
        self.assertTrue(clear_los["clear"])

        # Sub-ground blocked ray
        blocked_los = obs.line_of_sight_fast((cx - 500, cy, g + 5), (cx + 500, cy, g - 80))
        self.assertFalse(blocked_los["clear"])

        # Synthetic regional atmosphere fallback works cleanly when no ERA5 exists
        atm = AtmosphereModel.synthetic_regional_fallback(env)
        we, wn, wu, rho, tk = atm.sample(np.array([cx]), np.array([cy]), np.array([g + 100]))
        self.assertGreater(float(rho[0]), 0.5)
        self.assertEqual(atm.document["edition"], "synthetic/isa-shamal-fallback")

    def test_riyadh_and_jeddah_enrichment_when_staged(self) -> None:
        biomes = {b.id: b for b in discover_biomes(REPO_ROOT)}
        if "riyadh-observation-v1" in biomes:
            rb = biomes["riyadh-observation-v1"]
            env = EnvironmentModel(
                rb.package_path,
                expected_manifest_sha256=rb.expected_manifest_sha256,
                ghs_obat_path=rb.ghs_obat_path,
                worldpop_path=rb.worldpop_path,
            )
            meta = env.metadata()
            # GHS-OBAT should fuse real modeled heights onto Riyadh OSM buildings
            if rb.ghs_obat_path:
                self.assertGreater(meta["buildingHeightOrigins"].get("derived-from-ghs-obat", 0), 500)
            if rb.worldpop_path:
                self.assertIsNotNone(meta["enrichment"]["worldPop"])
                self.assertGreater(meta["enrichment"]["worldPop"]["totalPopulationInAoi"], 1000)
            if rb.cams_dust_path:
                cams = load_cams_dust(rb.cams_dust_path)
                self.assertIsNotNone(cams)
                assert cams is not None
                self.assertLess(cams["derivedVisibilityKm"], 3.0)

        if "jeddah-coast-v1" in biomes:
            jb = biomes["jeddah-coast-v1"]
            jenv = EnvironmentModel(
                jb.package_path,
                expected_manifest_sha256=jb.expected_manifest_sha256,
                ghs_obat_path=jb.ghs_obat_path if (jb.ghs_obat_path and jb.ghs_obat_path.suffix == ".geojson") else None,
                worldpop_path=jb.worldpop_path,
            )
            self.assertGreater(len(jenv.buildings), 10)
            self.assertGreater(len(jenv.roads), 10)

    def test_civilian_traffic_deconfliction_and_road_impact(self) -> None:
        from sim_core.road_impact import get_road_analyzer
        from sim_core.traffic import check_airspace_deconfliction, load_frozen_traffic

        biomes = {b.id: b for b in discover_biomes(REPO_ROOT)}
        fixture = biomes["owned-fixture-v1"]
        env = EnvironmentModel(fixture.package_path, expected_manifest_sha256=fixture.expected_manifest_sha256)

        tracks = load_frozen_traffic(env, duration_s=90.0, seed=1234, try_live_api=False)
        self.assertGreaterEqual(len(tracks), 2)
        airliner = next(t for t in tracks if t.kind == "civilian_airliner")
        p_t20 = airliner.position_at(20.0)
        # Intercept right next to airliner must trigger HOLD_FIRE
        unsafe = check_airspace_deconfliction(tracks, p_t20 + np.array([150.0, 0.0, 50.0]), 20.0)
        self.assertFalse(unsafe["safe"])
        self.assertEqual(unsafe["warningLevel"], "HOLD_FIRE")
        # Intercept 5 km away must be CLEAR
        safe = check_airspace_deconfliction(tracks, p_t20 + np.array([5000.0, 5000.0, 0.0]), 20.0)
        self.assertTrue(safe["safe"])
        self.assertEqual(safe["warningLevel"], "CLEAR")

        # Road impact graph must detect blocked segments when hazardous debris lands directly on an OSM road
        analyzer = get_road_analyzer(env)
        self.assertGreater(len(analyzer.edges), 0)
        e0 = analyzer.edges[0]
        impact = analyzer.evaluate_debris_impact(
            np.array([e0.mid_e, e0.mid_e + 5.0]),
            np.array([e0.mid_n, e0.mid_n + 5.0]),
            np.array([True, True]),
        )
        self.assertGreaterEqual(impact["blockedEdgeCount"], 1)
        self.assertGreater(impact["blockedRoadKm"], 0.0)


if __name__ == "__main__":
    unittest.main()
