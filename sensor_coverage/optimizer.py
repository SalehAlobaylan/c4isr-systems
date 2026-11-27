"""Candidate-site generation and joint (position + sector azimuth) placement optimizer.

Weighted maximum coverage is monotone submodular, so greedy selection carries the
classic (1 - 1/e) approximation guarantee; a bounded 1-swap local search then
repairs greedy's myopic early choices. For directional sector sensors
(``sector_width_deg < 359.5``) with ``optimize_orientation=True``, the optimizer
reuses a single omnidirectional viewshed per candidate site to evaluate 8 azimuth
headings (0..315 deg) jointly with site selection in milliseconds.
"""
from __future__ import annotations

from dataclasses import dataclass
import math
import time
from typing import Any, Callable

import numpy as np

from .contracts import (
    JammerSpec,
    LayerSpec,
    OptimizerSpec,
    SensorSpec,
    ThreatSpec,
    WeatherSpec,
    effective_range_m,
    sector_mask_angles,
)
from .grid import AnalysisGrid, ObstructionGrid
from .viewshed import floor_agl_map

ORIENTATION_HEADINGS_DEG = (0.0, 45.0, 90.0, 135.0, 180.0, 225.0, 270.0, 315.0)


@dataclass
class Candidate:
    east: float
    north: float
    kind: str  # lattice | terrain_high | rooftop
    site_up: float


def generate_candidates(obs: ObstructionGrid, grid: AnalysisGrid, spec: OptimizerSpec) -> list[Candidate]:
    env = obs.env
    span_e = env.east_max - env.east_min
    span_n = env.north_max - env.north_min
    margin = 0.04 * min(span_e, span_n)
    spacing = float(spec.candidate_spacing_m) if spec.candidate_spacing_m else max(
        grid.cell_m * 2.0, math.sqrt(span_e * span_n / 200.0)
    )

    cands: list[Candidate] = []

    def far_enough(e: float, n: float, min_d: float) -> bool:
        return all(math.hypot(e - c.east, n - c.north) >= min_d for c in cands)

    es = np.arange(env.east_min + margin, env.east_max - margin + 1e-6, spacing)
    ns = np.arange(env.north_min + margin, env.north_max - margin + 1e-6, spacing)
    for n in ns:
        for e in es:
            z, inside = obs.sample(np.asarray([e]), np.asarray([n]), buildings=False)
            i, j, _ = obs._index(np.asarray([e]), np.asarray([n]))
            if inside[0] and obs.valid[j[0], i[0]]:
                cands.append(Candidate(float(e), float(n), "lattice", float(z[0])))

    block = max(2, int(round((spacing * 0.75) / obs.cell_m)))
    by, bx = obs.ny // block, obs.nx // block
    if by > 0 and bx > 0:
        t = np.where(obs.valid, obs.terrain, -np.inf)[: by * block, : bx * block]
        blocks = t.reshape(by, block, bx, block).transpose(0, 2, 1, 3).reshape(by, bx, block * block)
        arg = np.argmax(blocks, axis=2)
        vmax = np.max(blocks, axis=2)
        order = np.argsort(vmax.ravel())[::-1]
        added = 0
        for flat in order:
            if added >= 28:
                break
            bj, bi = divmod(int(flat), bx)
            if not np.isfinite(vmax[bj, bi]):
                continue
            rj, ri = divmod(int(arg[bj, bi]), block)
            j = bj * block + rj
            i = bi * block + ri
            e = obs.east0 + (i + 0.5) * obs.cell_m
            n = obs.north0 + (j + 0.5) * obs.cell_m
            if not (env.east_min + margin <= e <= env.east_max - margin and env.north_min + margin <= n <= env.north_max - margin):
                continue
            if far_enough(e, n, spacing * 0.45):
                cands.append(Candidate(float(e), float(n), "terrain_high", float(vmax[bj, bi])))
                added += 1

    if spec.allow_rooftops and env.buildings:
        roofs = sorted(env.buildings, key=lambda b: (b.roof_up_m, b.geometry_enu.area), reverse=True)
        added = 0
        for b in roofs:
            if added >= 30:
                break
            c = b.geometry_enu.representative_point()
            e, n = float(c.x), float(c.y)
            if not (env.east_min + margin <= e <= env.east_max - margin and env.north_min + margin <= n <= env.north_max - margin):
                continue
            if far_enough(e, n, spacing * 0.45):
                cands.append(Candidate(e, n, "rooftop", float(b.roof_up_m)))
                added += 1
    return cands


class CandidateFloorCache:
    """Small LRU cache of candidate floor matrices keyed by environment + sensor profile."""

    def __init__(self, max_entries: int = 12) -> None:
        self.max_entries = max_entries
        self._store: dict[tuple[Any, ...], np.ndarray] = {}

    def get(self, key: tuple[Any, ...]) -> np.ndarray | None:
        val = self._store.pop(key, None)
        if val is not None:
            self._store[key] = val
        return val

    def put(self, key: tuple[Any, ...], val: np.ndarray) -> None:
        self._store[key] = val
        while len(self._store) > self.max_entries:
            self._store.pop(next(iter(self._store)))


def _gain_vector(
    cov: np.ndarray,  # L x M x N bool
    counts: np.ndarray,  # L x N int
    weights: np.ndarray,
    layer_w: np.ndarray,
    redundancy_weight: float,
) -> np.ndarray:
    gains = np.zeros(cov.shape[1], dtype=np.float64)
    for li in range(cov.shape[0]):
        unseen = (counts[li] == 0).astype(np.float64) * weights
        single = (counts[li] == 1).astype(np.float64) * weights
        layer_cov = cov[li].astype(np.float32)
        gains += layer_w[li] * (layer_cov @ unseen + redundancy_weight * (layer_cov @ single))
    return gains


def _jammers_sig(jammers: list[JammerSpec] | None) -> tuple[Any, ...]:
    if not jammers:
        return ()
    return tuple((round(j.east, 1), round(j.north, 1), round(j.erp_ratio, 2), round(j.beam_width_deg, 1)) for j in jammers)


def optimize_placement(
    *,
    obs: ObstructionGrid,
    grid: AnalysisGrid,
    baseline: list[SensorSpec],
    layers: list[LayerSpec],
    weights: np.ndarray,
    threat: ThreatSpec,
    weather: WeatherSpec,
    spec: OptimizerSpec,
    include_buildings: bool,
    jammers: list[JammerSpec] | None = None,
    cache: CandidateFloorCache | None = None,
    cache_scope: tuple[Any, ...] = (),
    extra_template: Callable[[int], SensorSpec] | None = None,
) -> dict[str, Any]:
    t0 = time.perf_counter()
    candidates = generate_candidates(obs, grid, spec)
    if not candidates:
        raise ValueError("No valid candidate sites inside the environment for optimization.")
    cand_e = np.array([c.east for c in candidates])
    cand_n = np.array([c.north for c in candidates])

    locked = [s for s in baseline if s.locked]
    movable = [s for s in baseline if not s.locked]
    if extra_template is not None:
        movable += [extra_template(k) for k in range(max(0, int(spec.extra_sensors)))]
    if not movable:
        return {
            "sensors": list(baseline),
            "candidates": candidates,
            "candidateGain": np.zeros(len(candidates)),
            "trace": [],
            "evaluations": 0,
            "elapsedS": round(time.perf_counter() - t0, 3),
            "note": "All sensors are locked; nothing to optimize.",
        }

    layer_w = np.array([l.weight for l in layers], dtype=np.float64)
    layer_w = layer_w / max(float(np.sum(layer_w)), 1e-9)
    w_total = max(float(np.sum(weights)), 1e-9)
    w_norm = weights / w_total
    coarse_step = max(obs.cell_m, grid.cell_m * 0.5)
    jam_sig = _jammers_sig(jammers)

    # Precompute candidate->cell azimuth matrix only if any movable sensor is directional
    any_directional = any(s.is_directional and spec.optimize_orientation for s in movable)
    cand_az: np.ndarray | None = None
    if any_directional:
        dx = grid.east[None, :] - cand_e[:, None]
        dy = grid.north[None, :] - cand_n[:, None]
        cand_az = np.mod(np.arctan2(dx, dy), 2.0 * math.pi).astype(np.float32)

    def omni_floors(sensor: SensorSpec) -> np.ndarray:
        rng = effective_range_m(sensor, threat, weather)
        key = (
            *cache_scope,
            sensor.omni_profile_key(),
            round(rng, 1),
            include_buildings,
            round(spec.candidate_spacing_m or 0.0, 1),
            spec.allow_rooftops,
            jam_sig if sensor.type == "radar" else (),
        )
        floors = cache.get(key) if cache else None
        if floors is None:
            floors = np.stack(
                [
                    floor_agl_map(
                        obs,
                        grid,
                        sensor.moved_to(c.east, c.north),
                        rng,
                        include_buildings=include_buildings,
                        radial_step_m=coarse_step,
                        azimuth_spacing_m=grid.cell_m,
                        jammers=jammers,
                        ignore_sector=True,
                    )
                    for c in candidates
                ]
            )
            if cache:
                cache.put(key, floors)
        return floors

    # Each movable sensor profile maps to (cov: L x M x N bool, site_idx: M int, headings: M float)
    options_by_key: dict[tuple[Any, ...], tuple[np.ndarray, np.ndarray, np.ndarray]] = {}

    def get_options(sensor: SensorSpec) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
        opt_dir = sensor.is_directional and spec.optimize_orientation
        key = (sensor.omni_profile_key(), round(sensor.sector_width_deg, 1), opt_dir, round(sensor.sector_center_deg, 1) if not opt_dir else None)
        if key in options_by_key:
            return options_by_key[key]

        floors = omni_floors(sensor)  # C x N
        n_c = len(candidates)
        if not sensor.is_directional:
            cov = np.stack([floors <= np.float32(l.agl_m) for l in layers])
            site_idx = np.arange(n_c, dtype=np.int32)
            headings = np.full(n_c, sensor.sector_center_deg, dtype=np.float64)
        else:
            headings_list = list(ORIENTATION_HEADINGS_DEG) if opt_dir else [sensor.sector_center_deg]
            if opt_dir and all(abs(((sensor.sector_center_deg - h + 180.0) % 360.0) - 180.0) > 10.0 for h in headings_list):
                headings_list.append(sensor.sector_center_deg)
            assert cand_az is not None
            base_cov = np.stack([floors <= np.float32(l.agl_m) for l in layers])  # L x C x N
            cov_blocks = []
            site_blocks = []
            head_blocks = []
            for h_deg in headings_list:
                sec = sector_mask_angles(cand_az, h_deg, sensor.sector_width_deg)  # C x N
                cov_blocks.append(base_cov & sec[None, :, :])
                site_blocks.append(np.arange(n_c, dtype=np.int32))
                head_blocks.append(np.full(n_c, h_deg, dtype=np.float64))
            cov = np.concatenate(cov_blocks, axis=1)  # L x (C*H) x N
            site_idx = np.concatenate(site_blocks)
            headings = np.concatenate(head_blocks)

        options_by_key[key] = (cov, site_idx, headings)
        return cov, site_idx, headings

    for s in movable:
        get_options(s)

    counts = np.zeros((len(layers), grid.count), dtype=np.int32)
    for s in locked:
        rng = effective_range_m(s, threat, weather)
        f = floor_agl_map(
            obs,
            grid,
            s,
            rng,
            include_buildings=include_buildings,
            radial_step_m=coarse_step,
            azimuth_spacing_m=grid.cell_m,
            jammers=jammers,
        )
        for li, layer in enumerate(layers):
            counts[li] += (f <= np.float32(layer.agl_m)).astype(np.int32)

    fixed_pts = [(s.east, s.north) for s in locked]
    min_sep = max(0.0, float(spec.min_separation_m))

    def sep_mask_sites(exclude_idx: int | None, placed_sites: list[int]) -> np.ndarray:
        ok = np.ones(len(candidates), dtype=bool)
        if min_sep <= 0:
            return ok
        for e, n in fixed_pts:
            ok &= np.hypot(cand_e - e, cand_n - n) >= min_sep
        for k, ci in enumerate(placed_sites):
            if k == exclude_idx or ci < 0:
                continue
            ok &= np.hypot(cand_e - cand_e[ci], cand_n - cand_n[ci]) >= min_sep
        return ok

    order = sorted(range(len(movable)), key=lambda k: effective_range_m(movable[k], threat, weather), reverse=True)
    placed_opt = [-1] * len(movable)
    placed_site = [-1] * len(movable)
    trace: list[dict[str, Any]] = []
    evaluations = 0
    first_site_gain: np.ndarray | None = None

    def current_objective() -> float:
        total = 0.0
        for li in range(len(layers)):
            total += layer_w[li] * (
                float(np.sum(w_norm[counts[li] >= 1])) + spec.redundancy_weight * float(np.sum(w_norm[counts[li] >= 2]))
            )
        return total

    running = current_objective()
    for k in order:
        cov, site_idx, headings = get_options(movable[k])
        gains = _gain_vector(cov, counts, w_norm, layer_w, spec.redundancy_weight)
        evaluations += len(site_idx)
        if first_site_gain is None:
            # Collapse multi-heading gains back to max gain per candidate site for 3D candidate markers
            sg = np.zeros(len(candidates), dtype=np.float64)
            np.maximum.at(sg, site_idx, gains)
            first_site_gain = sg
        ok_sites = sep_mask_sites(None, placed_site)
        gains_masked = np.where(ok_sites[site_idx], gains, -np.inf)
        best = int(np.argmax(gains_masked))
        if not np.isfinite(gains_masked[best]):
            best = int(np.argmax(np.where(np.isfinite(gains), gains, -1)))
        placed_opt[k] = best
        placed_site[k] = int(site_idx[best])
        counts += cov[:, best, :].astype(np.int32)
        running = current_objective()
        entry: dict[str, Any] = {
            "phase": "greedy",
            "sensorId": movable[k].id,
            "candidateIndex": placed_site[k],
            "candidateKind": candidates[placed_site[k]].kind,
            "objective": round(running, 5),
        }
        if movable[k].is_directional:
            entry["sectorCenterDeg"] = round(float(headings[best]), 1)
        trace.append(entry)

    for pass_i in range(max(0, int(spec.swap_passes))):
        improved = False
        for k in order:
            cov, site_idx, headings = get_options(movable[k])
            cur_opt = placed_opt[k]
            counts -= cov[:, cur_opt, :].astype(np.int32)
            gains = _gain_vector(cov, counts, w_norm, layer_w, spec.redundancy_weight)
            evaluations += len(site_idx)
            current_gain = gains[cur_opt]
            ok_sites = sep_mask_sites(k, placed_site)
            gains_masked = np.where(ok_sites[site_idx], gains, -np.inf)
            best = int(np.argmax(gains_masked))
            if np.isfinite(gains_masked[best]) and gains_masked[best] > current_gain + 1e-6:
                placed_opt[k] = best
                placed_site[k] = int(site_idx[best])
                improved = True
            counts += cov[:, placed_opt[k], :].astype(np.int32)
            if placed_opt[k] != cur_opt:
                running = current_objective()
                entry = {
                    "phase": f"swap-{pass_i + 1}",
                    "sensorId": movable[k].id,
                    "candidateIndex": placed_site[k],
                    "candidateKind": candidates[placed_site[k]].kind,
                    "objective": round(running, 5),
                }
                if movable[k].is_directional:
                    entry["sectorCenterDeg"] = round(float(headings[placed_opt[k]]), 1)
                trace.append(entry)
        if not improved:
            break

    optimized = list(locked)
    for k, s in enumerate(movable):
        _, _, headings = get_options(s)
        c = candidates[placed_site[k]]
        new_heading = float(headings[placed_opt[k]]) if s.is_directional else s.sector_center_deg
        optimized.append(
            s.moved_to(
                c.east,
                c.north,
                origin="extra" if s.origin == "extra" else "optimized",
                sector_center_deg=new_heading,
            )
        )

    return {
        "sensors": optimized,
        "candidates": candidates,
        "candidateGain": first_site_gain if first_site_gain is not None else np.zeros(len(candidates)),
        "trace": trace,
        "evaluations": evaluations,
        "elapsedS": round(time.perf_counter() - t0, 3),
        "note": None,
        "optimizedOrientations": bool(any_directional),
    }
