"""Stateful step-by-step Live Interactive Salvo Defense Session Engine.

Driven tick-by-tick (``dt = 0.25 s``) by Go's ``SessionCoordinator`` virtual clock
or directly via the Python hub. Enforces strict EMAD 4-partition isolation:
- ``step()`` emits **ONLY** ``EXERCISE_OBSERVATION`` fused tracks, own-force
  battery magazines/interceptors, frozen civilian corridors, and operator-visible
  events — never sealed ``EXERCISE_TRUTH`` threat trajectories or decoy identities.
- Accepts deterministic mid-run operator commands:
  ``SET_POLICY``, ``HOLD_FIRE_ZONE``, ``CLEAR_HOLD_FIRE``, ``ENGAGE_TRACK``,
  ``PRIORITIZE_TRACK``, ``TOGGLE_SENSOR``, ``TOGGLE_BATTERY``.
"""
from __future__ import annotations

import datetime as dt
import math
import secrets
import threading
from typing import Any

import numpy as np

from sim_core.atmosphere import AtmosphereModel
from sim_core.environment import EnvironmentModel
from sim_core.traffic import CivilianTrack, check_airspace_deconfliction, load_frozen_traffic

from .contracts import BATTERY_TIER_SPECS, SCOPE_EXERCISE_OBSERVATION, SalvoScenarioSpec
from .engine import _InterceptorFlight, _match_track_to_truth
from .fusion import MultiSensorNetwork, MultiTrackFusionEngine, SensorDetection
from .raid import ThreatTruth, build_raid_truth
from .wta import BatteryRuntime, choose_assignments, load_ai_wta_model


class SalvoStepSession:
    """Single deterministic live interactive salvo session."""

    def __init__(
        self,
        *,
        session_id: str,
        spec: SalvoScenarioSpec,
        env: EnvironmentModel,
        atm: AtmosphereModel,
        policy: str = "combinatorial_wta",
        surrogate: Any = None,
    ) -> None:
        self.session_id = session_id
        self.spec = spec
        self.env = env
        self.atm = atm
        self.policy = policy if policy in ("greedy_earliest", "combinatorial_wta", "ai_policy") else "combinatorial_wta"
        self.surrogate = surrogate
        self.ai_weights = load_ai_wta_model() if self.policy == "ai_policy" else None
        self.rng_pk = np.random.default_rng(spec.seed + 77713)

        self.dt_s = float(spec.dt_s)
        self.n_steps = int(math.ceil(spec.duration_s / self.dt_s)) + 1
        self.tick = 0
        self.virtual_time_s = 0.0
        self.completed = False
        self.stopped = False
        self.hold_fire = False
        self.disabled_sensors: set[str] = set()
        self.disabled_batteries: set[str] = set()
        self.priority_tracks: list[str] = []
        self.command_log: list[dict[str, Any]] = []
        self.operator_events: list[dict[str, Any]] = []
        self.final_run_id: str | None = None

        # Sealed EXERCISE_TRUTH raid + frozen civilian airliners
        self.raid_truth: list[ThreatTruth] = build_raid_truth(
            spec.threats, spec.assets, env, dt_s=spec.dt_s, duration_s=spec.duration_s
        )
        self.truth_by_id = {th.id: th for th in self.raid_truth}
        self.alive_threats: dict[str, ThreatTruth] = dict(self.truth_by_id)
        self.asset_by_id = {a.id: a for a in spec.assets}
        self.civilian_tracks: list[CivilianTrack] = (
            load_frozen_traffic(env, duration_s=spec.duration_s, seed=spec.seed)
            if spec.enable_civilian_traffic
            else []
        )

        # Multi-sensor network & observation-only fusion engine
        prof = atm.profile_summary(0.0, 0.0)
        surf_wind = float(prof[0]["speedMps"]) if prof else 4.0
        self.sensor_net = MultiSensorNetwork(
            spec.sensors,
            env,
            visibility_km=spec.visibility_km,
            surface_wind_mps=surf_wind,
            seed=spec.seed,
        )
        self.fusion = MultiTrackFusionEngine(env)
        self.pending_dets: list[SensorDetection] = []
        self.last_tracks: list[Any] = []

        # Finite-magazine batteries
        self.batteries: list[BatteryRuntime] = []
        self.batt_by_id: dict[str, BatteryRuntime] = {}
        for bspec in spec.batteries:
            res = bspec.resolved()
            g = env.height_at(bspec.east, bspec.north, fallback=env.up_median)
            br = BatteryRuntime(
                spec=bspec,
                pos_enu=np.array([bspec.east, bspec.north, g + bspec.height_agl_m], dtype=np.float64),
                magazine_remaining=int(bspec.magazine),
                magazine_initial=int(bspec.magazine),
                resolved=res,
            )
            self.batteries.append(br)
            self.batt_by_id[br.id] = br

        self.interceptors: list[_InterceptorFlight] = []
        self.next_int_seq = 1
        self.leakers_lethal = 0
        self.kills_total = 0
        self._lock = threading.Lock()

    def _apply_command(self, cmd: dict[str, Any]) -> None:
        kind = str(cmd.get("kind") or "").upper()
        payload = cmd.get("payload") or {}
        rec = {
            "tS": round(self.virtual_time_s, 2),
            "tick": self.tick,
            "kind": kind,
            "payload": payload,
            "actorId": cmd.get("actorId") or "operator",
        }
        self.command_log.append(rec)

        if kind == "SET_POLICY":
            pol = str(payload.get("policy") or "")
            if pol in ("greedy_earliest", "combinatorial_wta", "ai_policy"):
                self.policy = pol
                if pol == "ai_policy" and self.ai_weights is None:
                    self.ai_weights = load_ai_wta_model()
        elif kind == "HOLD_FIRE_ZONE":
            self.hold_fire = bool(payload.get("enabled", True))
        elif kind == "CLEAR_HOLD_FIRE":
            self.hold_fire = False
        elif kind == "TOGGLE_SENSOR":
            sid = str(payload.get("sensorId") or "")
            if sid:
                if "enabled" in payload:
                    if payload["enabled"]:
                        self.disabled_sensors.discard(sid)
                    else:
                        self.disabled_sensors.add(sid)
                elif sid in self.disabled_sensors:
                    self.disabled_sensors.remove(sid)
                else:
                    self.disabled_sensors.add(sid)
        elif kind == "TOGGLE_BATTERY":
            bid = str(payload.get("batteryId") or "")
            if bid:
                if "enabled" in payload:
                    if payload["enabled"]:
                        self.disabled_batteries.discard(bid)
                    else:
                        self.disabled_batteries.add(bid)
                elif bid in self.disabled_batteries:
                    self.disabled_batteries.remove(bid)
                else:
                    self.disabled_batteries.add(bid)
        elif kind == "PRIORITIZE_TRACK":
            tid = str(payload.get("trackId") or "")
            if tid and tid not in self.priority_tracks:
                self.priority_tracks.insert(0, tid)
        elif kind == "ENGAGE_TRACK":
            tid = str(payload.get("trackId") or "")
            if tid:
                self._manual_engage_track(tid)

        self.operator_events.append(
            {
                "tS": round(self.virtual_time_s, 2),
                "kind": "operator_command",
                "command": kind,
                "payload": payload,
            }
        )

    def _launch_interceptor(self, batt: BatteryRuntime, track_id: str, aim_enu: np.ndarray) -> bool:
        t_s = self.virtual_time_s
        if batt.id in self.disabled_batteries or not batt.is_ready(t_s):
            return False
        trk = self.fusion.tracks.get(track_id)
        if trk is None:
            return False
        batt.magazine_remaining -= 1
        batt.shots_fired += 1
        batt.active_interceptors += 1
        batt.next_ready_s = t_s + float(batt.resolved["reloadS"])

        truth_lock_id = _match_track_to_truth(trk.predict(t_s), self.alive_threats, t_s)
        init_dir = aim_enu - batt.pos_enu
        dn = max(float(np.linalg.norm(init_dir)), 1.0)
        spd = float(batt.resolved["speedMps"])
        flt = _InterceptorFlight(
            id=f"int_{self.next_int_seq:03d}",
            battery_id=batt.id,
            battery_tier=batt.tier,
            track_id=track_id,
            target_truth_id=truth_lock_id,
            launch_s=t_s,
            pos=batt.pos_enu.copy(),
            vel=(init_dir / dn) * spd,
            speed_mps=spd,
            pk=float(batt.resolved["pkBase"]),
            cost_k_usd=float(batt.resolved["costKUsd"]),
            aim_enu=aim_enu.copy(),
        )
        self.next_int_seq += 1
        self.interceptors.append(flt)
        self.operator_events.append(
            {
                "tS": round(t_s, 2),
                "kind": "interceptor_launch",
                "batteryId": batt.id,
                "trackId": track_id,
            }
        )
        return True

    def _manual_engage_track(self, track_id: str) -> None:
        if self.hold_fire:
            return
        trk = self.fusion.tracks.get(track_id)
        if trk is None:
            return
        t_s = self.virtual_time_s
        pred_pos = trk.predict(t_s + 2.0)
        air_chk = check_airspace_deconfliction(self.civilian_tracks, pred_pos, t_s + 2.0)
        if not air_chk["safe"]:
            self.operator_events.append(
                {
                    "tS": round(t_s, 2),
                    "kind": "airspace_hold_fire",
                    "trackId": track_id,
                    "callsign": air_chk.get("closestCallsign"),
                }
            )
            return
        # Pick cheapest ready battery in range
        candidates = []
        for batt in self.batteries:
            if batt.id in self.disabled_batteries or not batt.is_ready(t_s):
                continue
            d = float(np.linalg.norm(pred_pos - batt.pos_enu))
            if float(batt.resolved["minRangeM"]) <= d <= float(batt.resolved["maxRangeM"]):
                candidates.append((float(batt.resolved["costKUsd"]), d, batt))
        if not candidates:
            return
        candidates.sort(key=lambda x: (x[0], x[1]))
        self._launch_interceptor(candidates[0][2], track_id, pred_pos)

    def _step_one_tick(self) -> None:
        if self.completed or self.stopped:
            return
        t_s = round(self.tick * self.dt_s, 3)
        self.virtual_time_s = t_s

        # 1. Resolve threat arrivals (sealed truth layer)
        for tid in list(self.alive_threats.keys()):
            th = self.alive_threats[tid]
            if t_s < th.spawn_s:
                continue
            pos, _, airborne = th.state_at(t_s)
            tgt = self.asset_by_id.get(th.target_asset_id, self.spec.assets[0])
            dist = math.hypot(float(pos[0]) - tgt.east, float(pos[1]) - tgt.north)
            if (not airborne) or (t_s >= th.arrival_s - 1e-6) or dist <= max(60.0, tgt.radius_m * 0.25):
                del self.alive_threats[tid]
                if th.lethal:
                    self.leakers_lethal += 1

        # 2. Multi-sensor scan every 0.5 s (respecting live disabled_sensors!)
        if self.tick % max(1, int(round(0.5 / self.dt_s))) == 0:
            airborne_entities: list[dict[str, Any]] = []
            for th in self.alive_threats.values():
                pos, _, airborne = th.state_at(t_s)
                if airborne:
                    airborne_entities.append(
                        {"pos": pos, "rcs_m2": th.rcs_m2, "ir_signature": th.ir_signature, "iff_cooperative": False}
                    )
            for ct in self.civilian_tracks:
                if ct.kind == "civilian_airliner":
                    airborne_entities.append(
                        {
                            "pos": ct.position_at(t_s),
                            "rcs_m2": 18.0,
                            "ir_signature": 0.75,
                            "iff_cooperative": ct.iff_cooperative,
                            "squawk": ct.squawk,
                        }
                    )
            if airborne_entities:
                dets, _, _ = self.sensor_net.scan_entities(t_s, airborne_entities)
                if self.disabled_sensors:
                    dets = [d for d in dets if d.sensor_id not in self.disabled_sensors]
                self.pending_dets.extend(dets)

        ready_dets = [d for d in self.pending_dets if d.deliver_s <= t_s + 1e-6]
        if ready_dets:
            self.pending_dets = [d for d in self.pending_dets if d.deliver_s > t_s + 1e-6]
        self.last_tracks = self.fusion.ingest(t_s, ready_dets)

        # 3. Propagate active interceptors
        for flt in self.interceptors:
            if not flt.active:
                continue
            target_th = self.alive_threats.get(flt.target_truth_id) if flt.target_truth_id else None
            if target_th is not None:
                t_pos, t_vel, airborne = target_th.state_at(t_s)
                if not airborne:
                    target_th = None
            if target_th is not None:
                rel = t_pos - flt.pos
                dist = float(np.linalg.norm(rel))
                t_go = dist / max(flt.speed_mps, 1.0)
                lead_pt = t_pos + t_vel * min(t_go, 4.0)
                desired_dir = lead_pt - flt.pos
            else:
                desired_dir = flt.aim_enu - flt.pos
                dist = float(np.linalg.norm(desired_dir))

            d_norm = float(np.linalg.norm(desired_dir))
            if d_norm > 1e-3:
                flt.vel = (desired_dir / d_norm) * flt.speed_mps
            step_move = flt.speed_mps * self.dt_s
            flt.pos = flt.pos + flt.vel * self.dt_s

            hit_now = False
            if target_th is not None:
                t_pos_after, _, _ = target_th.state_at(t_s)
                if float(np.linalg.norm(t_pos_after - flt.pos)) <= max(35.0, step_move * 0.95):
                    hit_now = True
                    flt.pos = t_pos_after.copy()
            elif dist <= max(40.0, step_move) or (t_s - flt.launch_s > 40.0):
                flt.active = False
                flt.outcome = "expired"
                flt.end_s = t_s
                self.batt_by_id[flt.battery_id].active_interceptors = max(
                    0, self.batt_by_id[flt.battery_id].active_interceptors - 1
                )
                continue

            if hit_now and target_th is not None:
                flt.active = False
                flt.end_s = t_s
                batt = self.batt_by_id[flt.battery_id]
                batt.active_interceptors = max(0, batt.active_interceptors - 1)
                if float(self.rng_pk.random()) <= flt.pk:
                    flt.outcome = "kill"
                    batt.kills += 1
                    self.kills_total += 1
                    del self.alive_threats[target_th.id]
                else:
                    flt.outcome = "miss"

        # 4. Query automated WTA every 0.5 s unless HOLD_FIRE is active
        if not self.hold_fire and self.tick % max(1, int(round(0.5 / self.dt_s))) == 0 and self.last_tracks:
            eng_counts: dict[str, int] = {}
            for flt in self.interceptors:
                if flt.active:
                    eng_counts[flt.track_id] = eng_counts.get(flt.track_id, 0) + 1
            active_batts = [b for b in self.batteries if b.id not in self.disabled_batteries]
            if active_batts:
                fire_orders = choose_assignments(
                    t_s=t_s,
                    tracks=self.last_tracks,
                    batteries=active_batts,
                    assets=self.spec.assets,
                    civilian_tracks=self.civilian_tracks,
                    engaged_track_counts=eng_counts,
                    env=self.env,
                    atm=self.atm,
                    policy=self.policy,
                    surrogate=self.surrogate,
                    ai_weights=self.ai_weights,
                )
                for fo in fire_orders:
                    self._launch_interceptor(self.batt_by_id[fo.battery_id], fo.track_id, fo.predicted_intercept_enu)

        self.tick += 1
        if self.tick >= self.n_steps or (len(self.alive_threats) == 0 and not any(f.active for f in self.interceptors)):
            self.completed = True

    def observation_snapshot(self) -> dict[str, Any]:
        """Build strictly EXERCISE_OBSERVATION payload (zero sealed truth)."""
        t_s = self.virtual_time_s
        tracks_out = []
        for trk in self.last_tracks:
            p = trk.predict(t_s)
            spd = float(np.linalg.norm(trk.vel))
            top_cls = max(trk.class_probs, key=trk.class_probs.get)
            tracks_out.append(
                {
                    "trackId": trk.track_id,
                    "posEnu": [round(float(v), 1) for v in p],
                    "speedMps": round(spd, 1),
                    "topClass": top_cls,
                    "classProbs": {k: round(float(v), 3) for k, v in trk.class_probs.items()},
                    "iffCooperative": bool(trk.iff_cooperative),
                    "updates": int(trk.hits),
                    "prioritized": trk.track_id in self.priority_tracks,
                }
            )

        batts_out = [
            {
                "id": b.id,
                "tier": b.tier,
                "label": b.spec.label,
                "color": BATTERY_TIER_SPECS[b.tier]["color"],
                "enu": [round(float(v), 1) for v in b.pos_enu],
                "magazineRemaining": b.magazine_remaining,
                "magazineInitial": b.magazine_initial,
                "shotsFired": b.shots_fired,
                "activeInterceptors": b.active_interceptors,
                "costKUsd": float(b.resolved["costKUsd"]),
                "enabled": b.id not in self.disabled_batteries,
            }
            for b in self.batteries
        ]
        sensors_out = [
            {
                "id": s.id,
                "type": s.type,
                "label": s.label,
                "enabled": s.id not in self.disabled_sensors,
            }
            for s in self.spec.sensors
        ]
        active_ints = [
            {
                "id": f.id,
                "batteryId": f.battery_id,
                "batteryTier": f.battery_tier,
                "trackId": f.track_id,
                "enu": [round(float(v), 1) for v in f.pos],
            }
            for f in self.interceptors
            if f.active
        ]
        civil_out = [
            {
                "id": ct.id,
                "callsign": ct.callsign,
                "kind": ct.kind,
                "enu": [round(float(v), 1) for v in ct.position_at(t_s)],
                "safetyRadiusM": ct.safety_radius_m,
                "safetyHalfHeightM": ct.safety_half_height_m,
            }
            for ct in self.civilian_tracks
        ]
        return {
            "scope": SCOPE_EXERCISE_OBSERVATION,
            "tracks": tracks_out,
            "batteries": batts_out,
            "sensors": sensors_out,
            "activeInterceptors": active_ints,
            "civilianTraffic": civil_out,
            "recentEvents": self.operator_events[-16:],
        }

    def apply_command(self, cmd: dict[str, Any]) -> dict[str, Any]:
        with self._lock:
            if isinstance(cmd, dict):
                self._apply_command(cmd)
            return self.state_dict()

    def step(self, ticks: int = 1, commands: list[dict[str, Any]] | None = None) -> dict[str, Any]:
        with self._lock:
            for cmd in commands or []:
                if isinstance(cmd, dict):
                    self._apply_command(cmd)
            n = max(1, min(40, int(ticks)))
            for _ in range(n):
                if self.completed or self.stopped:
                    break
                self._step_one_tick()
            return self.state_dict()

    def state_dict(self) -> dict[str, Any]:
        return {
            "sessionId": self.session_id,
            "environmentId": self.spec.environment_id,
            "tick": self.tick,
            "virtualTimeS": round(self.virtual_time_s, 2),
            "durationS": self.spec.duration_s,
            "completed": self.completed,
            "stopped": self.stopped,
            "policy": self.policy,
            "holdFire": self.hold_fire,
            "viewScope": SCOPE_EXERCISE_OBSERVATION,
            "observation": self.observation_snapshot(),
            "finalRunId": self.final_run_id,
        }


class SalvoSessionManager:
    """Thread-safe registry of live interactive Salvo sessions."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._sessions: dict[str, SalvoStepSession] = {}

    def create(
        self,
        *,
        spec: SalvoScenarioSpec,
        env: EnvironmentModel,
        atm: AtmosphereModel,
        policy: str = "combinatorial_wta",
        surrogate: Any = None,
    ) -> SalvoStepSession:
        sid = "sess_" + secrets.token_hex(6)
        sess = SalvoStepSession(
            session_id=sid,
            spec=spec,
            env=env,
            atm=atm,
            policy=policy,
            surrogate=surrogate,
        )
        with self._lock:
            # Cap live sessions in memory to 12 to prevent unbounded growth
            if len(self._sessions) >= 12:
                oldest = next(iter(self._sessions))
                self._sessions.pop(oldest, None)
            self._sessions[sid] = sess
        return sess

    def get(self, session_id: str) -> SalvoStepSession | None:
        with self._lock:
            return self._sessions.get(session_id)

    def list(self) -> list[dict[str, Any]]:
        with self._lock:
            return [s.state_dict() for s in self._sessions.values()]

    def stop(self, session_id: str) -> dict[str, Any] | None:
        with self._lock:
            sess = self._sessions.get(session_id)
            if sess is None:
                return None
            sess.stopped = True
            return sess.state_dict()
