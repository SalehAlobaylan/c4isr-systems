"""Multi-Threat Raid Trajectory Generator (EXERCISE_TRUTH).

Generates realistic 3D trajectories over verified DSM terrain for heterogeneous
salvo raids: RF decoys, low-altitude one-way UAV swarms, terrain-hugging cruise
missiles, and parabolic high-arc short-range ballistic missiles.
"""
from __future__ import annotations

from dataclasses import dataclass
import math
from typing import Any

import numpy as np

from sim_core.environment import EnvironmentModel

from .contracts import SalvoAssetSpec, SalvoThreatSpec


@dataclass
class ThreatTruth:
    id: str
    kind: str
    label: str
    lethal: bool
    damage_value: float
    rcs_m2: float
    ir_signature: float
    spawn_s: float
    target_asset_id: str
    wave: int
    fragment_count: int
    times_s: np.ndarray  # [T]
    positions: np.ndarray  # [T, 3] ENU
    velocities: np.ndarray  # [T, 3] ENU
    arrival_s: float

    def state_at(self, t_s: float) -> tuple[np.ndarray, np.ndarray, bool]:
        """Return (pos_enu, vel_enu, is_airborne) at time ``t_s``."""
        if t_s < self.spawn_s or t_s > self.arrival_s:
            return self.positions[-1], self.velocities[-1], False
        idx = int(np.searchsorted(self.times_s, t_s, side="right")) - 1
        idx = max(0, min(len(self.times_s) - 2, idx))
        t0, t1 = float(self.times_s[idx]), float(self.times_s[idx + 1])
        frac = (t_s - t0) / max(t1 - t0, 1e-6)
        frac = max(0.0, min(1.0, frac))
        pos = (1.0 - frac) * self.positions[idx] + frac * self.positions[idx + 1]
        vel = (1.0 - frac) * self.velocities[idx] + frac * self.velocities[idx + 1]
        return pos, vel, True


def build_raid_truth(
    threats: list[SalvoThreatSpec],
    assets: list[SalvoAssetSpec],
    env: EnvironmentModel,
    *,
    dt_s: float = 0.25,
    duration_s: float = 120.0,
) -> list[ThreatTruth]:
    """Generate deterministic 3D terrain-aware truth trajectories for every threat in a salvo."""
    asset_by_id = {a.id: a for a in assets}
    default_asset = assets[0]
    out: list[ThreatTruth] = []

    for spec in threats:
        r = spec.resolved()
        target = asset_by_id.get(spec.target_asset_id, default_asset)
        se, sn, sagl = float(r["startEnu"][0]), float(r["startEnu"][1]), float(r["startEnu"][2])
        te, tn = float(target.east), float(target.north)
        dist_2d = max(120.0, math.hypot(te - se, tn - sn))
        spd = max(25.0, float(r["speedMps"]))
        flight_s = dist_2d / spd
        spawn_s = float(r["spawnS"])
        arrival_s = min(duration_s, spawn_s + flight_s)

        n_steps = max(4, int(math.ceil((arrival_s - spawn_s) / dt_s)) + 1)
        ts = np.linspace(spawn_s, arrival_s, n_steps, dtype=np.float64)
        u = np.clip((ts - spawn_s) / max(flight_s, 1e-6), 0.0, 1.0)

        # Slight dog-leg lateral offset for cruise missiles & decoys for realistic kinematics
        perp_e = -(tn - sn) / dist_2d
        perp_n = (te - se) / dist_2d
        if spec.kind == "cruise_missile":
            lat_off = np.sin(u * math.pi) * min(380.0, 0.06 * dist_2d)
        elif spec.kind == "decoy":
            lat_off = np.sin(u * 2.0 * math.pi) * min(160.0, 0.03 * dist_2d)
        else:
            lat_off = np.zeros_like(u)

        es = se + u * (te - se) + perp_e * lat_off
        ns = sn + u * (tn - sn) + perp_n * lat_off
        grounds = env.heights_at(es, ns, fallback=env.up_median)

        if spec.kind == "ballistic_missile":
            # Parabolic ballistic arc peaking at apogee_m above mid-path terrain
            apogee = max(500.0, float(r["apogeeM"]))
            agl = 40.0 + 4.0 * apogee * u * (1.0 - u)
            ups = grounds + agl
        else:
            # Terrain-following cruise with terminal dive in final 8%
            terminal_dive = np.where(u > 0.92, (1.0 - u) / 0.08, 1.0)
            agl = np.maximum(12.0, sagl * terminal_dive)
            ups = grounds + agl

        positions = np.column_stack((es, ns, ups))
        # Finite-difference 3D velocity
        velocities = np.zeros_like(positions)
        dt_arr = np.maximum(np.diff(ts), 1e-6)
        velocities[:-1] = np.diff(positions, axis=0) / dt_arr[:, None]
        velocities[-1] = velocities[-2]

        out.append(
            ThreatTruth(
                id=spec.id,
                kind=spec.kind,
                label=r["label"],
                lethal=r["lethal"],
                damage_value=r["damageValue"],
                rcs_m2=r["rcsM2"],
                ir_signature=r["irSignature"],
                spawn_s=spawn_s,
                target_asset_id=target.id,
                wave=r["wave"],
                fragment_count=r["fragmentCount"],
                times_s=ts,
                positions=positions,
                velocities=velocities,
                arrival_s=arrival_s,
            )
        )
    return out
