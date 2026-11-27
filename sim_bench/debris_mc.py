"""Vectorized Monte Carlo Debris Probability Heatmap & Risk Contour Engine.

Runs ``N`` perturbed wind + intercept-uncertainty ensembles (e.g. N=100-300
trials x 40 fragments = 4,000-12,000 trajectories in a single vectorized NumPy
RK2 pass, completing in ~0.3-0.9 s) and produces:
  - Per-trial statistical distribution (mean, P90, P95 building hits & WorldPop exposure)
  - Spatial impact probability density grid P(hazardous hit | cell)
  - WGS84 GeoJSON low / medium / high risk contour polygons for 2D map overlay
"""
from __future__ import annotations

import math
import time
from typing import Any

import numpy as np
from rasterio.features import shapes as raster_shapes
from rasterio.transform import Affine
import shapely
from shapely.geometry import shape

from sim_core.atmosphere import AtmosphereModel
from sim_core.environment import EnvironmentModel

GRAVITY_MPS2 = 9.80665


def run_monte_carlo_debris(
    *,
    env: EnvironmentModel,
    atm: AtmosphereModel,
    intercept_enu: tuple[float, float, float],
    threat_vel_enu: tuple[float, float, float],
    n_trials: int = 120,
    frags_per_trial: int = 36,
    wind_sigma_mps: float = 3.5,
    pos_sigma_m: float = 45.0,
    seed: int = 20261127,
    cell_m: float = 80.0,
) -> dict[str, Any]:
    """Execute vectorized Monte Carlo debris ensemble and return risk stats + GeoJSON contours."""
    t0 = time.perf_counter()
    n_trials = max(20, min(400, int(n_trials)))
    frags_per_trial = max(12, min(80, int(frags_per_trial)))
    total_n = n_trials * frags_per_trial
    rng = np.random.default_rng(seed)

    # Per-trial atmospheric & intercept-point perturbations
    trial_wind_e = rng.normal(0.0, wind_sigma_mps, size=n_trials)
    trial_wind_n = rng.normal(0.0, wind_sigma_mps, size=n_trials)
    trial_pos_jit = rng.normal(0.0, pos_sigma_m, size=(n_trials, 3))
    trial_pos_jit[:, 2] *= 0.4

    trial_idx = np.repeat(np.arange(n_trials, dtype=np.int32), frags_per_trial)
    p0_base = np.asarray(intercept_enu, dtype=np.float64)
    v0_base = np.asarray(threat_vel_enu, dtype=np.float64)

    pos = p0_base[None, :] + trial_pos_jit[trial_idx]
    # Ensure starting point stays above local terrain
    g_start = env.heights_at(pos[:, 0], pos[:, 1], fallback=env.up_median)
    pos[:, 2] = np.maximum(pos[:, 2], g_start + 30.0)

    # Isotropic burst velocities + log-uniform drag parameters k = 0.5*Cd*A/m
    u_r = rng.uniform(-1.0, 1.0, size=total_n)
    phi = rng.uniform(0.0, 2.0 * math.pi, size=total_n)
    st = np.sqrt(np.maximum(0.0, 1.0 - u_r * u_r))
    bdir = np.column_stack((st * np.cos(phi), st * np.sin(phi), u_r))

    log_k = rng.uniform(math.log(0.0025), math.log(0.065), size=total_n)
    drag_k = np.exp(log_k)
    k_norm = (log_k - math.log(0.0025)) / (math.log(0.065) - math.log(0.0025))
    mass_kg = np.exp(math.log(3.6) * (1.0 - k_norm) + math.log(0.015) * k_norm)
    burst_spd = 35.0 + 125.0 * (0.35 + 0.65 * k_norm * rng.uniform(0.75, 1.2, size=total_n))
    vel = v0_base[None, :] * 0.55 + bdir * burst_spd[:, None]

    # Sample baseline ERA5 wind profile once at intercept altitude & near-ground for fast vectorized RK2
    we_hi, wn_hi, _, rho_hi, _ = atm.sample(
        np.array([p0_base[0]]), np.array([p0_base[1]]), np.array([p0_base[2]])
    )
    we_lo, wn_lo, _, rho_lo, _ = atm.sample(
        np.array([p0_base[0]]), np.array([p0_base[1]]), np.array([float(env.up_median) + 60.0])
    )
    base_we_hi, base_wn_hi = float(we_hi[0]), float(wn_hi[0])
    base_we_lo, base_wn_lo = float(we_lo[0]), float(wn_lo[0])
    z_hi = max(float(p0_base[2]), float(env.up_median) + 120.0)
    z_lo = float(env.up_median) + 60.0
    rho_mean = 0.5 * (float(rho_hi[0]) + float(rho_lo[0]))

    w_jit_e = trial_wind_e[trial_idx]
    w_jit_n = trial_wind_n[trial_idx]

    active = np.ones(total_n, dtype=bool)
    impact_spd = np.zeros(total_n, dtype=np.float64)
    dt = 0.25
    max_steps = 360  # 90 s

    for _ in range(max_steps):
        act = np.nonzero(active)[0]
        if len(act) == 0:
            break
        p = pos[act]
        v = vel[act]
        k = drag_k[act]
        frac = np.clip((p[:, 2] - z_lo) / max(z_hi - z_lo, 1.0), 0.0, 1.0)
        we = base_we_lo + frac * (base_we_hi - base_we_lo) + w_jit_e[act]
        wn = base_wn_lo + frac * (base_wn_hi - base_wn_lo) + w_jit_n[act]

        vre = v[:, 0] - we
        vrn = v[:, 1] - wn
        vru = v[:, 2]
        vmag = np.sqrt(vre * vre + vrn * vrn + vru * vru)
        c_drag = np.minimum(rho_mean * k * vmag, 0.85 / dt)

        a_e = -c_drag * vre
        a_n = -c_drag * vrn
        a_u = -GRAVITY_MPS2 - c_drag * vru

        # Semi-implicit Euler / midpoint step
        v_new_e = v[:, 0] + a_e * dt
        v_new_n = v[:, 1] + a_n * dt
        v_new_u = v[:, 2] + a_u * dt
        p_new_e = p[:, 0] + 0.5 * (v[:, 0] + v_new_e) * dt
        p_new_n = p[:, 1] + 0.5 * (v[:, 1] + v_new_n) * dt
        p_new_u = p[:, 2] + 0.5 * (v[:, 2] + v_new_u) * dt

        g1 = env.heights_at(p_new_e, p_new_n, fallback=env.up_median)
        hit = p_new_u <= g1
        pos[act, 0] = p_new_e
        pos[act, 1] = p_new_n
        pos[act, 2] = np.where(hit, g1, p_new_u)
        vel[act, 0] = v_new_e
        vel[act, 1] = v_new_n
        vel[act, 2] = v_new_u
        if np.any(hit):
            h_idx = act[hit]
            impact_spd[h_idx] = np.linalg.norm(vel[h_idx], axis=1)
            active[h_idx] = False

    if np.any(active):
        impact_spd[active] = np.linalg.norm(vel[active], axis=1)

    energy_j = 0.5 * mass_kg * (impact_spd ** 2)
    hazardous = energy_j >= 80.0

    # Classify impacts using fast obstruction grid / building mask + WorldPop
    obs = env.obstruction_grid()
    bldg_z, inside = obs.sample(pos[:, 0], pos[:, 1], buildings=True)
    terr_z, _ = obs.sample(pos[:, 0], pos[:, 1], buildings=False)
    on_bldg = inside & ((bldg_z - terr_z) > 1.5)
    pop_cell = env.population_at(pos[:, 0], pos[:, 1])

    # Aggregate per-trial statistics
    bldg_per_trial = np.bincount(trial_idx, weights=(on_bldg & hazardous).astype(np.float64), minlength=n_trials)
    hz_per_trial = np.bincount(trial_idx, weights=hazardous.astype(np.float64), minlength=n_trials)
    pop_per_trial = np.bincount(trial_idx, weights=np.where(hazardous, pop_cell * 0.04, 0.0), minlength=n_trials)

    # Spatial probability grid P(>=1 hazardous impact falls in cell during a single intercept)
    hz_e = pos[hazardous, 0]
    hz_n = pos[hazardous, 1]
    hz_t = trial_idx[hazardous]

    pad = cell_m * 2.0
    if len(hz_e) > 0:
        emin = max(env.east_min, float(np.percentile(hz_e, 1)) - pad)
        emax = min(env.east_max, float(np.percentile(hz_e, 99)) + pad)
        nmin = max(env.north_min, float(np.percentile(hz_n, 1)) - pad)
        nmax = min(env.north_max, float(np.percentile(hz_n, 99)) + pad)
    else:
        emin, emax = p0_base[0] - 500.0, p0_base[0] + 500.0
        nmin, nmax = p0_base[1] - 500.0, p0_base[1] + 500.0

    nx = max(6, min(120, int(math.ceil((emax - emin) / cell_m))))
    ny = max(6, min(120, int(math.ceil((nmax - nmin) / cell_m))))
    eff_cell_e = (emax - emin) / nx
    eff_cell_n = (nmax - nmin) / ny

    ci = np.floor((hz_e - emin) / max(eff_cell_e, 1.0)).astype(np.int32)
    cj = np.floor((hz_n - nmin) / max(eff_cell_n, 1.0)).astype(np.int32)
    in_g = (ci >= 0) & (ci < nx) & (cj >= 0) & (cj < ny)

    # Count distinct trials hitting each cell -> true empirical probability P(hit in cell) in [0, 1]
    hit_prob = np.zeros((ny, nx), dtype=np.float32)
    if np.any(in_g):
        flat_keys = (hz_t[in_g].astype(np.int64) * (ny * nx)) + (cj[in_g].astype(np.int64) * nx + ci[in_g].astype(np.int64))
        uniq = np.unique(flat_keys)
        cells = uniq % (ny * nx)
        counts = np.bincount(cells, minlength=ny * nx).reshape(ny, nx)
        hit_prob = (counts / float(n_trials)).astype(np.float32)

    # Vectorize 3 probability risk tiers into WGS84 GeoJSON contours
    features: list[dict[str, Any]] = []
    north_up = Affine(eff_cell_e, 0.0, emin, 0.0, -eff_cell_n, nmax)

    def to_ll(coords: np.ndarray) -> np.ndarray:
        lon, lat = env.enu_to_lonlat_fast(coords[:, 0], coords[:, 1])
        return np.column_stack([lon, lat])

    for kind, thresh, label in (
        ("mc-contour-low", 0.05, "≥5% Hazardous Debris Risk"),
        ("mc-contour-med", 0.20, "≥20% Hazardous Debris Risk"),
        ("mc-contour-high", 0.45, "≥45% High Debris Risk"),
    ):
        mask = (hit_prob >= thresh)[::-1].astype(np.uint8)
        if not np.any(mask):
            continue
        for geom, val in raster_shapes(mask, mask=mask.astype(bool), transform=north_up, connectivity=8):
            if val != 1:
                continue
            poly = shape(geom).simplify(max(eff_cell_e, eff_cell_n) * 0.4, preserve_topology=True)
            if poly.is_empty or poly.area < eff_cell_e * eff_cell_n * 0.5:
                continue
            ll = shapely.transform(poly, to_ll)
            gj = shapely.geometry.mapping(ll)
            features.append(
                {
                    "type": "Feature",
                    "geometry": {"type": gj["type"], "coordinates": gj["coordinates"]},
                    "properties": {
                        "simKind": kind,
                        "probabilityThreshold": thresh,
                        "areaKm2": round(poly.area / 1e6, 3),
                        "label": label,
                    },
                }
            )

    mean_e = float(np.mean(hz_e)) if len(hz_e) else float(p0_base[0])
    mean_n = float(np.mean(hz_n)) if len(hz_n) else float(p0_base[1])
    drift_m = math.hypot(mean_e - float(p0_base[0]), mean_n - float(p0_base[1]))
    clon, clat, _ = env.enu_to_geodetic(mean_e, mean_n, env.up_median)

    return {
        "derivation": "synthetic/derived Monte Carlo RK2 wind-perturbed ensemble",
        "trials": n_trials,
        "fragmentsPerTrial": frags_per_trial,
        "totalTrajectories": total_n,
        "windSigmaMps": wind_sigma_mps,
        "posSigmaM": pos_sigma_m,
        "elapsedS": round(time.perf_counter() - t0, 3),
        "centroidLonLat": [round(clon, 6), round(clat, 6)],
        "meanDriftM": round(drift_m, 1),
        "maxCellHitProbability": round(float(np.max(hit_prob)), 3),
        "riskArea5PctKm2": round(float(np.sum(hit_prob >= 0.05)) * (eff_cell_e * eff_cell_n) / 1e6, 3),
        "riskArea20PctKm2": round(float(np.sum(hit_prob >= 0.20)) * (eff_cell_e * eff_cell_n) / 1e6, 3),
        "buildingHits": {
            "mean": round(float(np.mean(bldg_per_trial)), 2),
            "p90": round(float(np.percentile(bldg_per_trial, 90)), 2),
            "p95": round(float(np.percentile(bldg_per_trial, 95)), 2),
            "max": int(np.max(bldg_per_trial)),
        },
        "populationExposed": {
            "mean": round(float(np.mean(pop_per_trial)), 2),
            "p95": round(float(np.percentile(pop_per_trial, 95)), 2),
        },
        "hazardousPerTrialMean": round(float(np.mean(hz_per_trial)), 1),
        "contoursGeoJson": {"type": "FeatureCollection", "features": features},
    }
