"""Automated unit & integration tests for EMAD Multi-Threat Salvo & Layered Defense."""
from __future__ import annotations

import json
from pathlib import Path
import tempfile
import unittest

from salvo_defense.contracts import SALVO_ENGINE_VERSION, SALVO_REPLAY_SCHEMA
from salvo_defense.service import SalvoService
from salvo_defense.viewer.server import SalvoRouter


class TestSalvoDefense(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.service = SalvoService(runs_root=Path(self._tmp.name))
        self.router = SalvoRouter(self.service)

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_salvo_service_router_and_wta_vs_greedy(self) -> None:
        st_h, _, raw_h = self.router.resolve("/api/health")
        self.assertEqual(st_h, 200)
        health = json.loads(raw_h)
        self.assertEqual(health["service"], "emad-salvo-defense")
        self.assertEqual(health["engineVersion"], SALVO_ENGINE_VERSION)

        st_i, _, html = self.router.resolve("/")
        self.assertEqual(st_i, 200)
        self.assertIn(b"Multi-Threat Salvo", html)

        st_js, _, _ = self.router.resolve("/app.js")
        self.assertEqual(st_js, 200)

        envs = self.service.list_environments()
        fixture = next((e for e in envs if e["id"] == "owned-fixture-v1"), None)
        self.assertIsNotNone(fixture)
        assert fixture is not None

        draft = json.loads(json.dumps(fixture["defaults"]))
        draft["raid"] = {
            "decoys": 3,
            "uavs": 4,
            "cruiseMissiles": 2,
            "ballisticMissiles": 1,
            "waves": 2,
            "ingressSectorDeg": 225.0,
            "ingressSpreadDeg": 90.0,
        }
        draft["_sync"] = True

        st_p, _, raw_p = self.router.resolve("/api/runs", method="POST", body=json.dumps(draft).encode("utf-8"))
        self.assertEqual(st_p, 202)
        job = json.loads(raw_p)
        self.assertEqual(job["status"], "completed", msg=job.get("error"))
        self.assertIsNotNone(job["comparison"])
        self.assertIsNotNone(job["resultOverlay"])

        comp = job["comparison"]["policies"]
        self.assertIn("greedy_earliest", comp)
        self.assertIn("combinatorial_wta", comp)

        # Fetch full replay and verify EMAD partition separation & zero truth leakage into observations
        st_r, _, raw_r = self.router.resolve(f"/api/runs/{job['jobId']}/replay.json")
        self.assertEqual(st_r, 200)
        replay = json.loads(raw_r)
        self.assertEqual(replay["schema"], SALVO_REPLAY_SCHEMA)

        wta_run = replay["runs"]["combinatorial_wta"]
        self.assertEqual(wta_run["truth"]["scope"], "EXERCISE_TRUTH")
        self.assertEqual(wta_run["observations"]["scope"], "EXERCISE_OBSERVATION")
        self.assertEqual(wta_run["evaluation"]["scope"], "EVALUATION")

        # Verify zero truth IDs leaked into EXERCISE_OBSERVATION track frames
        for frame in wta_run["observations"]["trackFrames"]:
            for trk in frame["tracks"]:
                self.assertTrue(str(trk["trackId"]).startswith("trk_"))
                self.assertNotIn("truthId", trk)

        # Combinatorial WTA should spend less or score higher than naive greedy baseline
        g = comp["greedy_earliest"]
        w = comp["combinatorial_wta"]
        self.assertEqual(w["airspaceViolations"], 0)
        self.assertTrue(
            w["compositeScore"] >= g["compositeScore"] - 2.0 or w["totalCostKUsd"] <= g["totalCostKUsd"],
            msg=f"Expected WTA ({w}) to outperform or undercut Greedy ({g})",
        )

        # Operator COP projection (?view=operator) must strip truth & evaluation while keeping own-force telemetry
        st_op, _, raw_op = self.router.resolve(f"/api/runs/{job['jobId']}/replay.json?view=operator")
        self.assertEqual(st_op, 200)
        op_rep = json.loads(raw_op)
        self.assertTrue(op_rep["truthRedacted"])
        self.assertEqual(op_rep["viewScope"], "EXERCISE_OBSERVATION")
        self.assertIsNone(op_rep["comparison"])
        self.assertNotIn("threats", op_rep.get("scenario", {}))
        op_wta = op_rep["runs"]["combinatorial_wta"]
        self.assertNotIn("truth", op_wta)
        self.assertIsNone(op_wta["evaluation"])
        self.assertGreater(len(op_wta["ownForceTelemetry"]["batteries"]), 0)
        for batt in op_wta["ownForceTelemetry"]["batteries"]:
            self.assertNotIn("kills", batt)

    def test_live_interactive_salvo_session(self) -> None:
        envs = self.service.list_environments()
        fixture = next((e for e in envs if e["id"] == "owned-fixture-v1"), None)
        self.assertIsNotNone(fixture)
        assert fixture is not None

        draft = json.loads(json.dumps(fixture["defaults"]))
        draft["policy"] = "combinatorial_wta"
        st_c, _, raw_c = self.router.resolve("/api/sessions", method="POST", body=json.dumps(draft).encode("utf-8"))
        self.assertEqual(st_c, 201)
        sess = json.loads(raw_c)
        sid = sess["sessionId"]
        self.assertEqual(sess["viewScope"], "EXERCISE_OBSERVATION")
        self.assertNotIn("threats", sess["observation"])

        # Enable HOLD_FIRE_ZONE and step 24 ticks (6 seconds) -> zero shots must be fired!
        st_hf, _, raw_hf = self.router.resolve(
            f"/api/sessions/{sid}/step",
            method="POST",
            body=json.dumps({"ticks": 24, "commands": [{"kind": "HOLD_FIRE_ZONE", "payload": {"enabled": True}}]}).encode("utf-8"),
        )
        self.assertEqual(st_hf, 200)
        s_hf = json.loads(raw_hf)
        self.assertTrue(s_hf["holdFire"])
        self.assertEqual(s_hf["tick"], 24)
        for b in s_hf["observation"]["batteries"]:
            self.assertEqual(b["shotsFired"], 0)
        for tr in s_hf["observation"]["tracks"]:
            self.assertTrue(str(tr["trackId"]).startswith("trk_"))
            self.assertNotIn("lethal", tr)
            self.assertNotIn("truthId", tr)

        # Clear HOLD_FIRE, toggle policy, and step forward -> defense engages
        st_clr, _, raw_clr = self.router.resolve(
            f"/api/sessions/{sid}/step",
            method="POST",
            body=json.dumps({
                "ticks": 24,
                "commands": [
                    {"kind": "CLEAR_HOLD_FIRE", "payload": {}},
                    {"kind": "SET_POLICY", "payload": {"policy": "greedy_earliest"}},
                ],
            }).encode("utf-8"),
        )
        self.assertEqual(st_clr, 200)
        s_clr = json.loads(raw_clr)
        self.assertFalse(s_clr["holdFire"])
        self.assertEqual(s_clr["policy"], "greedy_earliest")

        # Direct /commands endpoint (TOGGLE_BATTERY) & /stop
        first_batt = s_clr["observation"]["batteries"][0]["id"]
        st_cmd, _, raw_cmd = self.router.resolve(
            f"/api/sessions/{sid}/commands",
            method="POST",
            body=json.dumps({"kind": "TOGGLE_BATTERY", "payload": {"batteryId": first_batt, "enabled": False}}).encode("utf-8"),
        )
        self.assertEqual(st_cmd, 202)
        s_cmd = json.loads(raw_cmd)
        b_map = {b["id"]: b["enabled"] for b in s_cmd["observation"]["batteries"]}
        self.assertFalse(b_map[first_batt])

        st_stop, _, raw_stop = self.router.resolve(f"/api/sessions/{sid}/stop", method="POST")
        self.assertEqual(st_stop, 200)
        self.assertTrue(json.loads(raw_stop)["stopped"])


if __name__ == "__main__":
    unittest.main()
