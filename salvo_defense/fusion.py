"""Multi-Sensor Observation Generator & Kalman/GNN Track Fusion (EXERCISE_OBSERVATION).

STRICT PARTITION ISOLATION:
  - ``MultiSensorNetwork`` converts sealed ``EXERCISE_TRUTH`` airborne entities
    (threats + frozen civilian airliners) into noisy, terrain/building-masked
    ``SensorDetection`` messages carrying ONLY opaque contact measurements —
    NEVER true entity IDs or ground-truth class labels!
  - ``MultiTrackFusionEngine`` associates detections to opaque track IDs
    (``trk_001``, ``trk_002``, ...) via Global Nearest Neighbor (GNN) gating,
    runs a 6-state constant-velocity Kalman filter, and maintains Bayesian class
    probabilities strictly from observed speed, altitude, vertical rate, RCS,
    EO/IR thermal signature, and cooperative civil IFF transponder squawks.
"""
from __future__ import annotations

from dataclasses import dataclass, field
import math
from typing import Any

import numpy as np

from sim_core.environment import EnvironmentModel
from sim_core.obstruction import ObstructionGrid
from sim_core.traffic import CivilianTrack

from .contracts import SalvoSensorSpec

CLASSES = ("civilian_airliner", "decoy", "one_way_uav", "cruise_missile", "ballistic_missile")


@dataclass(frozen=True)
class SensorDetection:
    sensor_id: str
    sensor_type: str  # radar | eo_ir | acoustic
    sample_s: float
    deliver_s: float
    pos_enu: np.ndarray  # noisy 3D ENU measurement
    measured_rcs_m2: float | None = None
    measured_ir: float | None = None
    iff_cooperative: bool = False
    squawk: str | None = None


@dataclass
class FusedTrack:
    track_id: str
    state: np.ndarray  # [e, n, u, ve, vn, vu]
    cov: np.ndarray  # 6x6 covariance
    last_update_s: float
    first_seen_s: float
    hits: int = 1
    sensor_types_seen: set[str] = field(default_factory=set)
    rcs_ema: float | None = None
    ir_ema: float | None = None
    iff_cooperative: bool = False
    squawk: str | None = None
    class_probs: dict[str, float] = field(
        default_factory=lambda: {
            "civilian_airliner": 0.05,
            "decoy": 0.15,
            "one_way_uav": 0.35,
            "cruise_missile": 0.25,
            "ballistic_missile": 0.20,
        }
    )

    @property
    def pos(self) -> np.ndarray:
        return self.state[:3]

    @property
    def vel(self) -> np.ndarray:
        return self.state[3:6]

    @property
    def speed_mps(self) -> float:
        return float(np.linalg.norm(self.vel))

    @property
    def top_class(self) -> str:
        return max(self.class_probs.items(), key=lambda kv: kv[1])[0]

    def predict(self, t_s: float) -> np.ndarray:
        dt = max(0.0, t_s - self.last_update_s)
        return self.pos + dt * self.vel

    def snapshot(self) -> dict[str, Any]:
        return {
            "trackId": self.track_id,
            "posEnu": [round(float(v), 1) for v in self.pos],
            "velEnu": [round(float(v), 1) for v in self.vel],
            "speedMps": round(self.speed_mps, 1),
            "hits": self.hits,
            "topClass": self.top_class,
            "classProbs": {k: round(float(v), 3) for k, v in self.class_probs.items()},
            "iffCooperative": self.iff_cooperative,
            "squawk": self.squawk,
            "sensors": sorted(self.sensor_types_seen),
        }


class MultiSensorNetwork:
    """Terrain- & building-masked multi-modal sensor simulator producing EXERCISE_OBSERVATION."""

    def __init__(
        self,
        sensors: list[SalvoSensorSpec],
        env: EnvironmentModel,
        *,
        visibility_km: float = 10.0,
        surface_wind_mps: float = 4.0,
        seed: int = 42,
    ) -> None:
        self.sensors = sensors
        self.env = env
        self.obs: ObstructionGrid = env.obstruction_grid()
        self.visibility_km = float(visibility_km)
        self.surface_wind_mps = float(surface_wind_mps)
        self.rng = np.random.default_rng(seed + 4099)
        self.sensor_positions: dict[str, np.ndarray] = {}
        for s in sensors:
            g = self.obs.height_at(s.east, s.north, buildings=True)
            self.sensor_positions[s.id] = np.array([s.east, s.north, g + s.mast_agl_m], dtype=np.float64)

    def _effective_range(self, sensor: SalvoSensorSpec, rcs_m2: float) -> float:
        if sensor.type == "radar":
            return float(sensor.max_range_m * (max(rcs_m2, 0.01) ** 0.25))
        if sensor.type == "eo_ir":
            vis_m = max(300.0, self.visibility_km * 1000.0 * 0.9)
            return float(min(sensor.max_range_m, vis_m))
        if sensor.type == "acoustic":
            wf = max(0.35, 1.0 - self.surface_wind_mps / 20.0)
            return float(sensor.max_range_m * wf)
        return float(sensor.max_range_m)

    def scan_entities(
        self,
        t_s: float,
        entities: list[dict[str, Any]],
    ) -> tuple[list[SensorDetection], int, int]:
        """Scan all currently airborne truth entities and emit noisy opaque detections.

        Returns ``(detections, los_clear_checks, los_blocked_checks)``.
        """
        detections: list[SensorDetection] = []
        los_clear = 0
        los_blocked = 0

        for ent in entities:
            ep = ent["pos"]
            rcs = float(ent["rcs_m2"])
            ir_sig = float(ent["ir_signature"])
            iff_coop = bool(ent.get("iff_cooperative", False))
            squawk = ent.get("squawk")

            for s in self.sensors:
                sp = self.sensor_positions[s.id]
                dx = float(ep[0] - sp[0])
                dy = float(ep[1] - sp[1])
                dz = float(ep[2] - sp[2])
                dist = math.sqrt(dx * dx + dy * dy + dz * dz)
                eff_r = self._effective_range(s, rcs)
                if dist > eff_r or dist < 30.0:
                    continue

                if s.sector_width_deg < 359.5:
                    az = math.degrees(math.atan2(dx, dy)) % 360.0
                    diff = abs(((az - s.sector_center_deg + 180.0) % 360.0) - 180.0)
                    if diff > s.sector_width_deg * 0.5:
                        continue

                if s.type in ("radar", "eo_ir"):
                    los = self.obs.line_of_sight_fast(
                        (float(sp[0]), float(sp[1]), float(sp[2])),
                        (float(ep[0]), float(ep[1]), float(ep[2])),
                        include_buildings=True,
                    )
                    if not los["clear"]:
                        los_blocked += 1
                        continue
                    los_clear += 1

                # Stochastic detection probability decaying near max effective range
                ratio = dist / max(eff_r, 1.0)
                pd = max(0.25, 0.96 - 0.35 * (ratio ** 2))
                if float(self.rng.random()) > pd:
                    continue

                sigma_pos = 20.0 if s.type == "radar" else (14.0 if s.type == "eo_ir" else 65.0)
                noisy_pos = ep + self.rng.normal(0.0, sigma_pos, size=3)
                meas_rcs = max(0.01, rcs * float(self.rng.uniform(0.78, 1.24))) if s.type == "radar" else None
                meas_ir = float(np.clip(ir_sig + self.rng.normal(0.0, 0.09), 0.02, 1.2)) if s.type == "eo_ir" else None
                delay = 0.25 if s.type == "radar" else (0.15 if s.type == "eo_ir" else 0.45)

                detections.append(
                    SensorDetection(
                        sensor_id=s.id,
                        sensor_type=s.type,
                        sample_s=t_s,
                        deliver_s=t_s + delay,
                        pos_enu=noisy_pos,
                        measured_rcs_m2=meas_rcs,
                        measured_ir=meas_ir,
                        iff_cooperative=iff_coop if s.type == "radar" else False,
                        squawk=squawk if (iff_coop and s.type == "radar") else None,
                    )
                )
        return detections, los_clear, los_blocked


class MultiTrackFusionEngine:
    """Observation-only GNN track associator + 6-state Kalman filter + Bayesian classifier."""

    def __init__(self, env: EnvironmentModel, *, gate_m: float = 480.0, coast_timeout_s: float = 5.5) -> None:
        self.env = env
        self.gate_m = float(gate_m)
        self.coast_timeout_s = float(coast_timeout_s)
        self.tracks: dict[str, FusedTrack] = {}
        self._next_id = 1

    def _update_bayesian_class(self, trk: FusedTrack, ground_up: float) -> None:
        """Update class probabilities purely from OBSERVED kinematics, RCS, EO/IR & IFF."""
        spd = trk.speed_mps
        agl = max(0.0, float(trk.pos[2]) - ground_up)
        vz = float(trk.vel[2])

        # Likelihoods L(obs | class)
        lk: dict[str, float] = {}
        if trk.iff_cooperative:
            lk["civilian_airliner"] = 25.0
            lk["decoy"] = 0.04
            lk["one_way_uav"] = 0.02
            lk["cruise_missile"] = 0.01
            lk["ballistic_missile"] = 0.01
        else:
            # Civilian without IFF is very unlikely at combat altitudes/speeds
            lk["civilian_airliner"] = 0.35 if (agl > 750.0 and 130.0 <= spd <= 250.0 and abs(vz) < 25.0) else 0.02

            # One-way UAV: slow (40..110 m/s), low AGL, modest RCS
            lk["one_way_uav"] = math.exp(-0.5 * ((spd - 68.0) / 34.0) ** 2) * (1.2 if agl < 400.0 else 0.35)
            # Cruise missile: fast (160..300 m/s), very low terrain-skimming AGL, low RCS, hot IR
            lk["cruise_missile"] = math.exp(-0.5 * ((spd - 230.0) / 60.0) ** 2) * (1.3 if agl < 350.0 else 0.30)
            # Ballistic missile: very fast (>260 m/s) OR steep vertical rate (|vz| > 55 m/s) OR high AGL (>500 m)
            bal_kin = math.exp(-0.5 * ((spd - 370.0) / 110.0) ** 2)
            bal_arc = 2.6 if (abs(vz) > 45.0 or agl > 480.0) else 0.35
            lk["ballistic_missile"] = bal_kin * bal_arc
            # Decoy: medium speed (~115 m/s), abnormally bright RCS (>0.35 m^2) + weak IR (<0.38)!
            dec_spd = math.exp(-0.5 * ((spd - 115.0) / 45.0) ** 2)
            dec_sig = 1.0
            if trk.rcs_ema is not None:
                dec_sig *= 2.4 if trk.rcs_ema >= 0.36 else 0.45
            if trk.ir_ema is not None:
                dec_sig *= 4.2 if trk.ir_ema <= 0.36 else 0.20
                if trk.ir_ema >= 0.72:
                    lk["cruise_missile"] *= 1.8
                    lk["ballistic_missile"] *= 1.6
                elif trk.ir_ema >= 0.42:
                    lk["one_way_uav"] *= 1.5
            lk["decoy"] = dec_spd * dec_sig

        # Blend prior with likelihood (tempered Bayes update so track stabilizes smoothly over 3-6 hits)
        new_p: dict[str, float] = {}
        for c in CLASSES:
            prior = max(0.01, trk.class_probs.get(c, 0.2))
            new_p[c] = (prior ** 0.72) * (max(1e-4, lk[c]) ** 0.48)
        total = sum(new_p.values())
        if total > 0:
            trk.class_probs = {c: new_p[c] / total for c in CLASSES}

    def ingest(self, t_s: float, detections: list[SensorDetection]) -> list[FusedTrack]:
        """Associate incoming detections (GNN), run Kalman updates, and prune stale tracks."""
        # Prune tracks that have coasted past timeout
        for tid in list(self.tracks.keys()):
            if t_s - self.tracks[tid].last_update_s > self.coast_timeout_s:
                del self.tracks[tid]

        # Greedy global nearest-neighbor association across active tracks
        for det in detections:
            best_tid: str | None = None
            best_dist = self.gate_m
            for tid, trk in self.tracks.items():
                dt = max(0.0, det.sample_s - trk.last_update_s)
                pred = trk.pos + dt * trk.vel
                d = float(np.linalg.norm(det.pos_enu - pred))
                # Expand gate slightly for brand-new 1-hit tracks without velocity estimate yet
                eff_gate = self.gate_m * (1.55 if trk.hits == 1 else 1.0)
                if d < best_dist and d < eff_gate:
                    best_dist = d
                    best_tid = tid

            if best_tid is None:
                tid = f"trk_{self._next_id:03d}"
                self._next_id += 1
                state = np.array([det.pos_enu[0], det.pos_enu[1], det.pos_enu[2], 0.0, 0.0, 0.0], dtype=np.float64)
                cov = np.diag([400.0, 400.0, 400.0, 40000.0, 40000.0, 20000.0])
                trk = FusedTrack(
                    track_id=tid,
                    state=state,
                    cov=cov,
                    last_update_s=det.sample_s,
                    first_seen_s=det.sample_s,
                    hits=1,
                    sensor_types_seen={det.sensor_type},
                    rcs_ema=det.measured_rcs_m2,
                    ir_ema=det.measured_ir,
                    iff_cooperative=det.iff_cooperative,
                    squawk=det.squawk,
                )
                g = self.env.height_at(float(det.pos_enu[0]), float(det.pos_enu[1]), fallback=self.env.up_median)
                self._update_bayesian_class(trk, g)
                self.tracks[tid] = trk
            else:
                trk = self.tracks[best_tid]
                dt = max(0.05, det.sample_s - trk.last_update_s)
                # 6-state constant-velocity Kalman predict + update
                F = np.eye(6, dtype=np.float64)
                F[0, 3] = F[1, 4] = F[2, 5] = dt
                q = 120.0 * dt
                Q = np.diag([q * dt * dt, q * dt * dt, q * dt * dt, q, q, q])
                x_pred = F @ trk.state
                P_pred = F @ trk.cov @ F.T + Q

                H = np.zeros((3, 6), dtype=np.float64)
                H[0, 0] = H[1, 1] = H[2, 2] = 1.0
                r_var = 450.0 if det.sensor_type == "radar" else (240.0 if det.sensor_type == "eo_ir" else 3600.0)
                R = np.eye(3, dtype=np.float64) * r_var

                y = det.pos_enu - H @ x_pred
                S = H @ P_pred @ H.T + R
                K = P_pred @ H.T @ np.linalg.inv(S)

                # Bootstrap velocity cleanly on hit #2
                if trk.hits == 1 and dt >= 0.15:
                    trk.state = np.concatenate((det.pos_enu, (det.pos_enu - trk.pos) / dt))
                    trk.cov = (np.eye(6) - K @ H) @ P_pred
                else:
                    trk.state = x_pred + K @ y
                    trk.cov = (np.eye(6) - K @ H) @ P_pred

                trk.last_update_s = det.sample_s
                trk.hits += 1
                trk.sensor_types_seen.add(det.sensor_type)
                if det.measured_rcs_m2 is not None:
                    trk.rcs_ema = det.measured_rcs_m2 if trk.rcs_ema is None else 0.7 * trk.rcs_ema + 0.3 * det.measured_rcs_m2
                if det.measured_ir is not None:
                    trk.ir_ema = det.measured_ir if trk.ir_ema is None else 0.6 * trk.ir_ema + 0.4 * det.measured_ir
                if det.iff_cooperative:
                    trk.iff_cooperative = True
                    trk.squawk = det.squawk

                g = self.env.height_at(float(trk.pos[0]), float(trk.pos[1]), fallback=self.env.up_median)
                self._update_bayesian_class(trk, g)

        return list(self.tracks.values())
