"""Synthetic Sensor Adapters & Observation-Only Tracker.

Strict EMAD partition boundary:
- ``TerrainMaskedRadar`` reads ``EXERCISE_TRUTH`` internally (in the simulator
  runtime) and emits noisy, delayed, terrain-occluded ``EXERCISE_OBSERVATION``
  records carrying HMAC-opaque contact IDs (never leaking true actor IDs).
- ``ConstantVelocityTracker`` consumes ONLY delivered ``EXERCISE_OBSERVATION``
  detections; it never touches ``EXERCISE_TRUTH``.
"""
from __future__ import annotations

from dataclasses import dataclass
import hashlib
import hmac
import math
from typing import Any

import numpy as np

from .contracts import RadarConfig, SCOPE_EXERCISE_OBSERVATION
from .environment import EnvironmentModel
from .physics import TrajectorySample


def _opaque_id(secret: str, message: str, prefix: str) -> str:
    digest = hmac.new(secret.encode("utf-8"), message.encode("utf-8"), hashlib.sha256).hexdigest()
    return f"{prefix}_{digest[:16]}"


class TerrainMaskedRadar:
    """Synthetic 3D radar with real terrain/building LOS masking, noise, and delivery delay."""

    def __init__(self, config: RadarConfig, env: EnvironmentModel, *, seed: int) -> None:
        self.config = config
        self.env = env
        self.seed = int(seed)
        self._secret = f"emad-sim-radar-hmac-v1|{seed}|{config.id}"
        e, n, agl = config.position_enu
        ground = env.height_at(float(e), float(n), fallback=env.up_median)
        self.position_enu = np.array([float(e), float(n), ground + float(agl)], dtype=np.float64)
        self.ground_up_m = ground

    def scan_trajectory(
        self,
        actor_id: str,
        rcs_m2: float,
        samples: list[TrajectorySample],
        *,
        dt_s: float,
    ) -> dict[str, Any]:
        """Scan a true trajectory, returning separate truth LOS records and opaque observations."""
        rng = np.random.default_rng(self.seed + 104729)
        period_steps = max(1, int(round(self.config.period_s / dt_s)))
        contact_id = _opaque_id(self._secret, f"{self.config.id}|{actor_id}", "contact")

        los_truth: list[dict[str, Any]] = []
        observations: list[dict[str, Any]] = []
        scan_num = 0

        for idx in range(0, len(samples), period_steps):
            ts = samples[idx]
            if not ts.active:
                continue
            scan_num += 1

            rel = ts.pos - self.position_enu
            rng_m = float(np.linalg.norm(rel))
            horiz_m = math.hypot(float(rel[0]), float(rel[1]))
            el_deg = math.degrees(math.atan2(float(rel[2]), max(horiz_m, 1e-6)))
            az_deg = (math.degrees(math.atan2(float(rel[0]), float(rel[1]))) + 360.0) % 360.0

            in_range = rng_m <= self.config.max_range_m
            in_elev = self.config.min_elevation_deg <= el_deg <= self.config.max_elevation_deg

            if not in_range:
                los = {"clear": False, "blockedBy": "out_of_range", "distanceM": round(rng_m, 2)}
            elif not in_elev:
                los = {"clear": False, "blockedBy": "elevation_mask", "distanceM": round(rng_m, 2)}
            else:
                los = self.env.line_of_sight(
                    self.position_enu,
                    ts.pos,
                    include_buildings=self.config.include_buildings_in_los,
                )

            detected = False
            miss_reason = los["blockedBy"]

            if los["clear"]:
                if self.config.dropout_every > 0 and (scan_num % self.config.dropout_every) == 0:
                    miss_reason = "scheduled_dropout"
                else:
                    rcs_factor = min(1.05, max(0.45, (max(rcs_m2, 0.01) / 0.25) ** 0.25))
                    range_factor = max(0.2, 1.0 - 0.35 * ((rng_m / self.config.max_range_m) ** 2))
                    p_det = min(0.99, self.config.base_detection_prob * rcs_factor * range_factor)
                    if float(rng.random()) <= p_det:
                        detected = True
                        miss_reason = None
                    else:
                        miss_reason = "probabilistic_miss"

            los_truth.append(
                {
                    "sampleS": round(ts.t_s, 3),
                    "rangeM": round(rng_m, 2),
                    "azimuthDeg": round(az_deg, 2),
                    "elevationDeg": round(el_deg, 2),
                    "losClear": bool(los["clear"]),
                    "blockedBy": los["blockedBy"],
                    "blockPointEnu": los.get("blockPointEnu"),
                    "detected": detected,
                    "missReason": miss_reason,
                }
            )

            if detected:
                # Inject realistic spherical measurement noise (range, azimuth, elevation)
                meas_r = max(10.0, rng_m + float(rng.normal(0.0, self.config.range_noise_m)))
                meas_az = math.radians(az_deg + float(rng.normal(0.0, self.config.azimuth_noise_deg)))
                meas_el = math.radians(el_deg + float(rng.normal(0.0, self.config.elevation_noise_deg)))

                meas_horiz = meas_r * math.cos(meas_el)
                meas_pos = np.array(
                    [
                        self.position_enu[0] + meas_horiz * math.sin(meas_az),
                        self.position_enu[1] + meas_horiz * math.cos(meas_az),
                        self.position_enu[2] + meas_r * math.sin(meas_el),
                    ],
                    dtype=np.float64,
                )
                cross_range_sigma = rng_m * math.radians(self.config.azimuth_noise_deg)
                accuracy_m = math.hypot(self.config.range_noise_m, cross_range_sigma)
                deliver_s = round(ts.t_s + self.config.delay_s, 3)

                observations.append(
                    {
                        "id": _opaque_id(self._secret, f"obs|{contact_id}|{ts.t_s:.3f}", "obs"),
                        "scope": SCOPE_EXERCISE_OBSERVATION,
                        "sensorId": self.config.id,
                        "contactId": contact_id,
                        "sampleS": round(ts.t_s, 3),
                        "deliverS": deliver_s,
                        "measuredEnu": [round(float(v), 2) for v in meas_pos],
                        "measuredRangeM": round(meas_r, 2),
                        "measuredAzimuthDeg": round(math.degrees(meas_az) % 360.0, 2),
                        "measuredElevationDeg": round(math.degrees(meas_el), 2),
                        "accuracyM": round(accuracy_m, 2),
                        "confidence": round(min(0.98, max(0.4, 1.0 - accuracy_m / 450.0)), 3),
                    }
                )

        return {
            "radarPositionEnu": [round(float(v), 2) for v in self.position_enu],
            "losTruth": los_truth,
            "observations": observations,
        }

    def coverage_profile(self, *, probe_agl_m: float = 90.0, n_azimuths: int = 72) -> list[dict[str, Any]]:
        """Compute maximum clear-LOS detection range around 360 degrees at a target AGL."""
        rays = []
        angles = np.linspace(0.0, 360.0, n_azimuths, endpoint=False)
        radii = np.linspace(150.0, self.config.max_range_m, 28)

        for az in angles:
            rad = math.radians(float(az))
            dx = math.sin(rad)
            dy = math.cos(rad)
            clear_r = 0.0
            blocked_by = None
            last_pt = self.position_enu.copy()
            for r in radii:
                pe = float(self.position_enu[0] + dx * r)
                pn = float(self.position_enu[1] + dy * r)
                if not (
                    self.env.east_min <= pe <= self.env.east_max
                    and self.env.north_min <= pn <= self.env.north_max
                ):
                    break
                g = self.env.height_at(pe, pn, fallback=self.env.up_median)
                target = np.array([pe, pn, g + probe_agl_m], dtype=np.float64)
                los = self.env.line_of_sight(
                    self.position_enu, target, include_buildings=self.config.include_buildings_in_los
                )
                if los["clear"]:
                    clear_r = float(r)
                    last_pt = target
                else:
                    blocked_by = los["blockedBy"]
                    break
            rays.append(
                {
                    "azimuthDeg": round(float(az), 1),
                    "maxClearRangeM": round(clear_r, 1),
                    "blockedBy": blocked_by,
                    "edgeEnu": [round(float(v), 2) for v in last_pt],
                }
            )
        return rays


@dataclass
class TrackEstimate:
    contact_id: str
    t_s: float  # delivery time when estimate became available to operator/AI
    sample_s: float
    pos_enu: np.ndarray
    vel_enu: np.ndarray
    update_count: int
    confidence: float


class ConstantVelocityTracker:
    """Alpha-beta 3D kinematic track filter operating strictly on EXERCISE_OBSERVATION."""

    def __init__(self, *, alpha: float = 0.55, beta: float = 0.22, firm_updates: int = 3) -> None:
        self.alpha = alpha
        self.beta = beta
        self.firm_updates = firm_updates
        self._state: dict[str, dict[str, Any]] = {}

    def ingest(self, obs: dict[str, Any]) -> TrackEstimate:
        cid = str(obs["contactId"])
        meas = np.asarray(obs["measuredEnu"], dtype=np.float64)
        sample_s = float(obs["sampleS"])
        deliver_s = float(obs["deliverS"])

        if cid not in self._state:
            st = {
                "pos": meas.copy(),
                "vel": np.zeros(3, dtype=np.float64),
                "sample_s": sample_s,
                "updates": 1,
            }
            self._state[cid] = st
        else:
            st = self._state[cid]
            dt = max(sample_s - st["sample_s"], 1e-3)
            if st["updates"] == 1:
                st["vel"] = (meas - st["pos"]) / dt
                st["pos"] = meas.copy()
            else:
                pred_pos = st["pos"] + st["vel"] * dt
                residual = meas - pred_pos
                st["pos"] = pred_pos + self.alpha * residual
                st["vel"] = st["vel"] + (self.beta / dt) * residual
            st["sample_s"] = sample_s
            st["updates"] += 1

        # Extrapolate estimated position forward by sensor delivery latency so fire-control
        # compensates for observation delay using ONLY observed velocity!
        latency_s = max(0.0, deliver_s - sample_s)
        pos_at_deliver = st["pos"] + st["vel"] * latency_s
        conf = min(0.98, 0.35 + 0.18 * min(st["updates"], 4))
        return TrackEstimate(
            contact_id=cid,
            t_s=deliver_s,
            sample_s=sample_s,
            pos_enu=pos_at_deliver.copy(),
            vel_enu=st["vel"].copy(),
            update_count=st["updates"],
            confidence=round(conf, 3),
        )

    def is_firm(self, est: TrackEstimate) -> bool:
        return est.update_count >= self.firm_updates
