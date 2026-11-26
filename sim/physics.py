"""Modular 3-DOF dynamics: Threat flight, Interceptor pursuit, and Wind-Drifted Shrapnel.

All physics operate in the environment's local ENU frame (east, north, up in metres)
and declare fidelity ``kinematic_3dof_drag_wind_v1``. Supports three distinct threat
flight profiles (``one_way_uav``, ``cruise_missile``, ``ballistic_missile``),
acceleration-limited proportional-lead interceptor pursuit, and vectorized RK2
shrapnel propagation modeling both threat and interceptor debris with mass,
quadratic aerodynamic drag, real 3D ERA5 wind drift, derived building-roof
intersections, and kinetic impact energy (Joules) hazard classification.
"""
from __future__ import annotations

from dataclasses import dataclass
import math
from typing import Any

import numpy as np

from .atmosphere import AtmosphereModel
from .contracts import InterceptorSiteConfig, ShrapnelConfig, ThreatConfig
from .environment import EnvironmentModel

GRAVITY_MPS2 = 9.80665


@dataclass
class TrajectorySample:
    t_s: float
    pos: np.ndarray  # shape (3,) ENU
    vel: np.ndarray  # shape (3,) ENU m/s
    active: bool = True


def _build_ballistic_trajectory(
    config: ThreatConfig,
    env: EnvironmentModel,
    *,
    duration_s: float,
    dt_s: float,
) -> list[TrajectorySample]:
    """Generate a gravity-governed 3-DOF ballistic arc between first and last waypoint."""
    wp0 = config.waypoints_enu[0]
    wp1 = config.waypoints_enu[-1]
    e0, n0 = float(wp0[0]), float(wp0[1])
    e1, n1 = float(wp1[0]), float(wp1[1])
    g0 = env.height_at(e0, n0, fallback=env.up_median)
    g1 = env.height_at(e1, n1, fallback=env.up_median)
    z0 = g0 + max(30.0, float(wp0[2]))
    z1 = g1 + max(10.0, float(wp1[2]))

    apogee_agl = max(350.0, float(config.ballistic_apogee_m))
    z_apex = max(z0, z1) + apogee_agl

    # Exact kinematic time-of-flight under uniform gravity g for ascending to z_apex and falling to z1
    vz0 = math.sqrt(2.0 * GRAVITY_MPS2 * max(z_apex - z0, 50.0))
    t_up = vz0 / GRAVITY_MPS2
    t_down = math.sqrt(2.0 * max(z_apex - z1, 50.0) / GRAVITY_MPS2)
    t_flight_grav = t_up + t_down

    # Use arc-length time-of-flight bounded by real 1G gravity flight time so high-apogee
    # arcs remain physically realistic and engageable inside tactical environment boxes.
    horiz_dist = math.hypot(e1 - e0, n1 - n0)
    rise = z_apex - max(z0, z1)
    arc_len = math.hypot(horiz_dist, 2.0 * rise)
    t_flight = min(t_flight_grav, max(20.0, arc_len / max(config.speed_mps, 120.0)))
    # Recompute exact parabolic vertical launch speed & effective g passing through z_apex
    dz = z1 - z0
    g_eff = max(GRAVITY_MPS2, 8.0 * rise / (t_flight * t_flight))
    vz_init = (dz + 0.5 * g_eff * t_flight * t_flight) / t_flight
    ve = (e1 - e0) / t_flight
    vn = (n1 - n0) / t_flight

    n_steps = int(math.floor(duration_s / dt_s)) + 1
    samples: list[TrajectorySample] = []
    for step in range(n_steps):
        t = round(step * dt_s, 6)
        if t < config.spawn_s:
            samples.append(TrajectorySample(t_s=t, pos=np.array([e0, n0, z0]), vel=np.zeros(3), active=False))
            continue
        tau = t - config.spawn_s
        if tau > t_flight:
            samples.append(TrajectorySample(t_s=t, pos=np.array([e1, n1, z1]), vel=np.zeros(3), active=False))
            continue
        e = e0 + ve * tau
        n = n0 + vn * tau
        z = z0 + vz_init * tau - 0.5 * g_eff * tau * tau
        vz = vz_init - g_eff * tau
        g_here = env.height_at(e, n, fallback=env.up_median)
        if z < g_here + 4.0 and tau > 0.5:
            z = g_here + 4.0
        samples.append(
            TrajectorySample(
                t_s=t,
                pos=np.array([e, n, z], dtype=np.float64),
                vel=np.array([ve, vn, vz], dtype=np.float64),
                active=True,
            )
        )
    return samples


def build_threat_trajectory(
    config: ThreatConfig,
    env: EnvironmentModel,
    *,
    duration_s: float,
    dt_s: float,
) -> list[TrajectorySample]:
    """Generate deterministic 3-DOF threat path over the real terrain surface."""
    if len(config.waypoints_enu) < 2:
        raise ValueError("Threat trajectory requires at least two waypoints")

    if config.kind == "ballistic_missile":
        return _build_ballistic_trajectory(config, env, duration_s=duration_s, dt_s=dt_s)

    resolved_pts: list[np.ndarray] = []
    for wp in config.waypoints_enu:
        e, n, z_spec = float(wp[0]), float(wp[1]), float(wp[2])
        if config.altitude_mode == "agl":
            ground = env.height_at(e, n, fallback=env.up_median)
            u = ground + z_spec
        else:
            u = z_spec
        resolved_pts.append(np.array([e, n, u], dtype=np.float64))

    # Precompute cumulative segment lengths in 3D ENU along with original AGL specs
    segs = []
    cum_len = 0.0
    for i in range(len(resolved_pts) - 1):
        p0, p1 = resolved_pts[i], resolved_pts[i + 1]
        d = p1 - p0
        length = float(np.linalg.norm(d))
        if length < 1e-6:
            continue
        agl0 = float(config.waypoints_enu[i][2])
        agl1 = float(config.waypoints_enu[i + 1][2])
        segs.append((cum_len, cum_len + length, p0, p1, d / length, length, agl0, agl1))
        cum_len += length

    total_len = cum_len
    n_steps = int(math.floor(duration_s / dt_s)) + 1
    samples: list[TrajectorySample] = []

    for step in range(n_steps):
        t = round(step * dt_s, 6)
        if t < config.spawn_s:
            samples.append(TrajectorySample(t_s=t, pos=resolved_pts[0].copy(), vel=np.zeros(3), active=False))
            continue

        dist = (t - config.spawn_s) * config.speed_mps
        if dist >= total_len:
            end_pos = resolved_pts[-1].copy()
            samples.append(TrajectorySample(t_s=t, pos=end_pos, vel=np.zeros(3), active=False))
            continue

        for s0, s1, p0, p1, unit_dir, seg_len, agl0, agl1 in segs:
            if dist <= s1 or s1 == total_len:
                frac = min(max((dist - s0) / seg_len, 0.0), 1.0)
                pos = p0 + frac * (p1 - p0)
                if config.altitude_mode == "agl":
                    g_here = env.height_at(float(pos[0]), float(pos[1]), fallback=env.up_median)
                    agl_interp = (1.0 - frac) * agl0 + frac * agl1
                    pos[2] = g_here + agl_interp
                vel = unit_dir * config.speed_mps
                samples.append(TrajectorySample(t_s=t, pos=pos, vel=vel, active=True))
                break

    # Refine vertical velocity component from actual terrain-following z differences
    for i in range(len(samples)):
        if not samples[i].active:
            continue
        if i + 1 < len(samples) and samples[i + 1].active:
            samples[i].vel = (samples[i + 1].pos - samples[i].pos) / dt_s
        elif i > 0 and samples[i - 1].active:
            samples[i].vel = (samples[i].pos - samples[i - 1].pos) / dt_s

    return samples


def simulate_interceptor_pursuit(
    site: InterceptorSiteConfig,
    env: EnvironmentModel,
    threat_samples: list[TrajectorySample],
    *,
    launch_time_s: float,
    dt_s: float,
    aim_lead_pos: np.ndarray | None = None,
) -> dict[str, Any]:
    """Simulate 3-DOF interceptor launch and proportional-lead pursuit against true threat."""
    site_e, site_n, site_agl = site.position_enu
    site_ground = env.height_at(float(site_e), float(site_n), fallback=env.up_median)
    launch_pos = np.array([float(site_e), float(site_n), site_ground + float(site_agl)], dtype=np.float64)

    liftoff_s = launch_time_s + site.launch_delay_s
    start_idx = int(math.ceil(liftoff_s / dt_s))
    if start_idx >= len(threat_samples):
        return {"intercepted": False, "reason": "launch-after-scenario-end", "samples": []}

    pos = launch_pos.copy()
    t0_target = aim_lead_pos if aim_lead_pos is not None else threat_samples[start_idx].pos
    init_dir = t0_target - pos
    norm0 = float(np.linalg.norm(init_dir))
    vel = (init_dir / max(norm0, 1e-6)) * (site.speed_mps * 0.45)

    out_samples: list[TrajectorySample] = []
    min_miss_m = float("inf")

    for i in range(start_idx, len(threat_samples)):
        ts = threat_samples[i]
        if not ts.active:
            break

        rel = ts.pos - pos
        dist = float(np.linalg.norm(rel))
        min_miss_m = min(min_miss_m, dist)
        out_samples.append(TrajectorySample(t_s=ts.t_s, pos=pos.copy(), vel=vel.copy(), active=True))

        # Adaptive fuze check accounts for high closing speed (e.g. ballistic/cruise intercepts)
        rel_speed = float(np.linalg.norm(ts.vel - vel))
        step_fuze = max(site.proximity_fuze_m, rel_speed * dt_s * 0.6)
        if dist <= step_fuze:
            return {
                "intercepted": True,
                "interceptTimeS": round(ts.t_s, 3),
                "interceptIndex": i,
                "interceptPosEnu": ts.pos.copy(),
                "threatVelEnu": ts.vel.copy(),
                "interceptorVelEnu": vel.copy(),
                "closingSpeedMps": round(rel_speed, 1),
                "missDistanceM": round(dist, 2),
                "launchTimeS": round(launch_time_s, 3),
                "liftoffTimeS": round(liftoff_s, 3),
                "flightDurationS": round(ts.t_s - liftoff_s, 3),
                "samples": out_samples,
            }

        if float(np.linalg.norm(pos - launch_pos)) > site.max_range_m * 1.18:
            return {
                "intercepted": False,
                "reason": "exceeded-max-range",
                "missDistanceM": round(min_miss_m, 2),
                "launchTimeS": round(launch_time_s, 3),
                "samples": out_samples,
            }

        t_go = min(dist / max(site.speed_mps, 1.0), 15.0)
        predicted_target = ts.pos + ts.vel * t_go
        desired_dir = predicted_target - pos
        desired_norm = float(np.linalg.norm(desired_dir))
        desired_vel = (desired_dir / max(desired_norm, 1e-6)) * site.speed_mps

        dv = desired_vel - vel
        dv_norm = float(np.linalg.norm(dv))
        eff_accel = max(site.max_accel_mps2, site.speed_mps * 0.55)
        max_dv = eff_accel * dt_s
        if dv_norm > max_dv:
            vel = vel + dv * (max_dv / dv_norm)
        else:
            vel = desired_vel

        spd = float(np.linalg.norm(vel))
        if spd > site.speed_mps:
            vel = vel * (site.speed_mps / spd)

        pos = pos + vel * dt_s

        g_here = env.height_at(float(pos[0]), float(pos[1]), fallback=env.up_median)
        if pos[2] < g_here + 5.0:
            pos[2] = g_here + 5.0

    return {
        "intercepted": False,
        "reason": "threat-completed-route-before-intercept",
        "missDistanceM": round(min_miss_m, 2),
        "launchTimeS": round(launch_time_s, 3),
        "samples": out_samples,
    }


def simulate_shrapnel_cloud(
    intercept_pos: np.ndarray,
    threat_vel: np.ndarray,
    config: ShrapnelConfig,
    env: EnvironmentModel,
    atm: AtmosphereModel,
    *,
    seed: int,
    interceptor_vel: np.ndarray | None = None,
    fragment_count_override: int | None = None,
    record_trails: bool = True,
    max_recorded_trails: int = 56,
    trail_stride: int = 4,
    protected_center: tuple[float, float] | None = None,
    protected_radius_m: float = 0.0,
) -> dict[str, Any]:
    """Vectorized 3-DOF ballistic shrapnel & debris propagation with drag, wind, and impact energy."""
    rng = np.random.default_rng(seed)
    n = int(fragment_count_override if fragment_count_override is not None else config.fragment_count)
    n = max(1, n)

    # Assign debris origin: threat body vs interceptor kill-vehicle/booster
    n_int = int(round(n * config.interceptor_fragment_fraction)) if interceptor_vel is not None else 0
    is_interceptor = np.zeros(n, dtype=bool)
    if n_int > 0:
        is_interceptor[n - n_int :] = True

    # Sample isotropic burst directions on unit sphere
    u_rand = rng.uniform(-1.0, 1.0, size=n)
    phi = rng.uniform(0.0, 2.0 * math.pi, size=n)
    sin_theta = np.sqrt(np.maximum(0.0, 1.0 - u_rand * u_rand))
    burst_dir = np.column_stack((sin_theta * np.cos(phi), sin_theta * np.sin(phi), u_rand))

    # Log-uniform drag parameter k = 0.5 * Cd * A / m (m^2/kg)
    log_min = math.log(config.drag_param_min)
    log_max = math.log(config.drag_param_max)
    log_k = rng.uniform(log_min, log_max, size=n)
    drag_k = np.exp(log_k)
    k_norm = (log_k - log_min) / max(log_max - log_min, 1e-6)

    # Correlate fragment mass inversely with drag parameter k (heavy motor/warhead chunks -> light panels)
    # mass ranges smoothly from ~3.8 kg (heavy low-drag) down to ~0.012 kg (light high-drag)
    mass_kg = np.exp(math.log(3.8) * (1.0 - k_norm) + math.log(0.012) * k_norm) * rng.uniform(0.75, 1.35, size=n)

    burst_speed = config.burst_speed_min_mps + (config.burst_speed_max_mps - config.burst_speed_min_mps) * (
        0.35 + 0.65 * k_norm * rng.uniform(0.7, 1.2, size=n)
    )

    pos = np.tile(np.asarray(intercept_pos, dtype=np.float64), (n, 1))
    parent_vel = np.tile(np.asarray(threat_vel, dtype=np.float64), (n, 1))
    if interceptor_vel is not None and np.any(is_interceptor):
        parent_vel[is_interceptor] = np.asarray(interceptor_vel, dtype=np.float64) * 0.45

    vel = parent_vel * config.momentum_retention + burst_dir * burst_speed[:, None]

    active = np.ones(n, dtype=bool)
    impact_time = np.zeros(n, dtype=np.float64)
    impact_speed = np.zeros(n, dtype=np.float64)

    trail_indices = (
        np.argsort(drag_k)[np.linspace(0, n - 1, min(n, max_recorded_trails), dtype=int)]
        if record_trails
        else np.array([], dtype=int)
    )
    trail_set = set(int(i) for i in trail_indices)
    trails: dict[int, list[list[float]]] = {
        int(idx): [[0.0, round(float(pos[idx, 0]), 2), round(float(pos[idx, 1]), 2), round(float(pos[idx, 2]), 2)]]
        for idx in trail_indices
    }

    dt = config.dt_s
    max_steps = int(math.ceil(config.max_flight_s / dt))
    gravity = np.array([0.0, 0.0, -GRAVITY_MPS2], dtype=np.float64)

    def accel_for(p_act: np.ndarray, v_act: np.ndarray, k_act: np.ndarray, g_act: np.ndarray) -> np.ndarray:
        we, wn, wu, rho, _ = atm.sample(p_act[:, 0], p_act[:, 1], p_act[:, 2], ground_up_m=g_act)
        wind = np.column_stack((we, wn, wu))
        v_rel = v_act - wind
        v_rel_mag = np.linalg.norm(v_rel, axis=1)
        drag = -(rho * k_act * v_rel_mag)[:, None] * v_rel
        return gravity[None, :] + drag

    for step in range(1, max_steps + 1):
        act_idx = np.nonzero(active)[0]
        if len(act_idx) == 0:
            break

        p0 = pos[act_idx]
        v0 = vel[act_idx]
        k0 = drag_k[act_idx]
        g0 = env.heights_at(p0[:, 0], p0[:, 1], fallback=env.up_median)

        a0 = accel_for(p0, v0, k0, g0)
        p_mid = p0 + 0.5 * dt * v0
        v_mid = v0 + 0.5 * dt * a0
        a_mid = accel_for(p_mid, v_mid, k0, g0)

        p1 = p0 + dt * v_mid
        v1 = v0 + dt * a_mid

        g1 = env.heights_at(p1[:, 0], p1[:, 1], fallback=env.up_median)
        hit = p1[:, 2] <= g1

        if np.any(hit):
            dz_prev = np.maximum(p0[hit, 2] - g0[hit], 0.0)
            dz_next = np.maximum(g1[hit] - p1[hit, 2], 1e-9)
            alpha = np.clip(dz_prev / (dz_prev + dz_next), 0.0, 1.0)
            p_hit = p0[hit] + alpha[:, None] * (p1[hit] - p0[hit])
            p_hit[:, 2] = env.heights_at(p_hit[:, 0], p_hit[:, 1], fallback=env.up_median)
            v_hit = v0[hit] + alpha[:, None] * (v1[hit] - v0[hit])

            hit_global = act_idx[hit]
            pos[hit_global] = p_hit
            vel[hit_global] = v_hit
            impact_time[hit_global] = (step - 1 + alpha) * dt
            impact_speed[hit_global] = np.linalg.norm(v_hit, axis=1)
            active[hit_global] = False

        still = ~hit
        if np.any(still):
            still_global = act_idx[still]
            pos[still_global] = p1[still]
            vel[still_global] = v1[still]

        if record_trails:
            t_now = round(step * dt, 2)
            hit_set = set(int(g) for g in act_idx[hit]) if np.any(hit) else set()
            if step % trail_stride == 0:
                for idx_i in trail_set:
                    if active[idx_i]:
                        trails[idx_i].append(
                            [t_now, round(float(pos[idx_i, 0]), 2), round(float(pos[idx_i, 1]), 2), round(float(pos[idx_i, 2]), 2)]
                        )
            for idx_i in hit_set & trail_set:
                trails[idx_i].append(
                    [
                        round(float(impact_time[idx_i]), 2),
                        round(float(pos[idx_i, 0]), 2),
                        round(float(pos[idx_i, 1]), 2),
                        round(float(pos[idx_i, 2]), 2),
                    ]
                )

    if np.any(active):
        impact_time[active] = config.max_flight_s
        impact_speed[active] = np.linalg.norm(vel[active], axis=1)

    exposure = env.classify_impacts(
        pos[:, 0],
        pos[:, 1],
        protected_center=protected_center,
        protected_radius_m=protected_radius_m,
    )

    # Elevate building impact markers onto the derived building roof height for crisp 3D placement
    if env.buildings:
        roof_by_id = {b.source_id: b.roof_up_m for b in env.buildings}
        for i, bid in enumerate(exposure["building_ids"]):
            if bid is not None and bid in roof_by_id:
                pos[i, 2] = max(pos[i, 2], roof_by_id[bid])

    impact_energy_j = 0.5 * mass_kg * (impact_speed ** 2)
    hazardous = impact_energy_j >= config.hazardous_energy_j

    on_building = exposure["on_building"]
    on_road = exposure["on_road"] & (~on_building)
    in_protected = exposure["in_protected"]

    summary = dict(exposure["summary"])
    summary.update(
        {
            "hazardousThresholdJ": config.hazardous_energy_j,
            "hazardousHits": int(np.sum(hazardous)),
            "hazardousBuildingHits": int(np.sum(on_building & hazardous)),
            "hazardousRoadHits": int(np.sum(on_road & hazardous)),
            "hazardousProtectedHits": int(np.sum(in_protected & hazardous)),
            "threatDebrisCount": int(np.sum(~is_interceptor)),
            "interceptorDebrisCount": int(np.sum(is_interceptor)),
            "maxImpactEnergyJ": round(float(np.max(impact_energy_j)), 1),
            "meanImpactEnergyJ": round(float(np.mean(impact_energy_j)), 1),
        }
    )

    mean_e = float(np.mean(pos[:, 0]))
    mean_n = float(np.mean(pos[:, 1]))
    drift_e = mean_e - float(intercept_pos[0])
    drift_n = mean_n - float(intercept_pos[1])
    drift_m = math.hypot(drift_e, drift_n)
    if n >= 3:
        cov = np.cov(pos[:, 0], pos[:, 1])
        eigvals, eigvecs = np.linalg.eigh(cov)
        order = np.argsort(eigvals)[::-1]
        eigvals = np.maximum(eigvals[order], 0.0)
        eigvecs = eigvecs[:, order]
        semi_major_90 = float(math.sqrt(4.60517 * eigvals[0]))
        semi_minor_90 = float(math.sqrt(4.60517 * eigvals[1]))
        angle_rad = math.atan2(float(eigvecs[1, 0]), float(eigvecs[0, 0]))
    else:
        semi_major_90, semi_minor_90, angle_rad = 10.0, 10.0, 0.0

    fragments_out = []
    for i in range(n):
        frag_class = "heavy" if drag_k[i] < 0.008 else ("medium" if drag_k[i] < 0.025 else "light")
        fragments_out.append(
            {
                "id": f"frag-{i:03d}",
                "origin": "interceptor" if is_interceptor[i] else "threat",
                "class": frag_class,
                "massKg": round(float(mass_kg[i]), 3),
                "dragParam": round(float(drag_k[i]), 5),
                "impactEnu": [round(float(pos[i, 0]), 2), round(float(pos[i, 1]), 2), round(float(pos[i, 2]), 2)],
                "impactTimeAfterInterceptS": round(float(impact_time[i]), 2),
                "impactSpeedMps": round(float(impact_speed[i]), 2),
                "impactEnergyJ": round(float(impact_energy_j[i]), 1),
                "hazardous": bool(hazardous[i]),
                "exposureCategory": exposure["categories"][i],
                "buildingSourceId": exposure["building_ids"][i],
            }
        )

    trails_out = [
        {
            "fragmentId": f"frag-{idx:03d}",
            "origin": fragments_out[idx]["origin"],
            "class": fragments_out[idx]["class"],
            "hazardous": fragments_out[idx]["hazardous"],
            "points": pts,
        }
        for idx, pts in trails.items()
    ]

    return {
        "fragmentCount": n,
        "exposure": summary,
        "footprint": {
            "centroidEnu": [
                round(mean_e, 2),
                round(mean_n, 2),
                round(env.height_at(mean_e, mean_n, fallback=env.up_median), 2),
            ],
            "driftFromInterceptEnuM": [round(drift_e, 2), round(drift_n, 2)],
            "driftDistanceM": round(drift_m, 2),
            "ellipse90SemiMajorM": round(semi_major_90, 2),
            "ellipse90SemiMinorM": round(semi_minor_90, 2),
            "ellipseAngleDeg": round(math.degrees(angle_rad), 2),
            "meanFallTimeS": round(float(np.mean(impact_time)), 2),
            "maxFallTimeS": round(float(np.max(impact_time)), 2),
        },
        "fragments": fragments_out,
        "trails": trails_out,
    }
