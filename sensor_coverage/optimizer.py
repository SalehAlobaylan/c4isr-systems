"""Candidate-site generation and greedy + swap sensor-placement optimization.

Weighted maximum coverage is monotone submodular, so greedy selection carries the
classic (1 - 1/e) approximation guarantee; a bounded 1-swap local search then
repairs greedy's myopic early choices. Locked (existing) sensors are kept fixed
and only movable / extra sensors are re-sited.
"""
from __future__ import annotations

from dataclasses import dataclass
import math
import time
from typing import Any, Callable

import numpy as np

from .contracts import LayerSpec, OptimizerSpec, SensorSpec, ThreatSpec, WeatherSpec, effective_range_m
from .grid import AnalysisGrid, ObstructionGrid
from .viewshed import floor_agl_map


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

    # Terrain highs: block-wise maxima of the bare DSM (classic radar siting heuristic)
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

    # Rooftops: mast on tallest building roofs (urban siting option)
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
    cov: np.ndarray,  # L x C x N bool
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

    def profile_cov(sensor: SensorSpec) -> np.ndarray:
        rng = effective_range_m(sensor, threat, weather)
        key = (*cache_scope, sensor.profile_key(), round(rng, 1), include_buildings, round(spec.candidate_spacing_m or 0.0, 1), spec.allow_rooftops)
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
                    )
                    for c in candidates
                ]
            )
            if cache:
                cache.put(key, floors)
        return np.stack([floors <= np.float32(l.agl_m) for l in layers])  # L x C x N

    cov_by_profile: dict[tuple[Any, ...], np.ndarray] = {}
    for s in movable:
        if s.profile_key() not in cov_by_profile:
            cov_by_profile[s.profile_key()] = profile_cov(s)

    counts = np.zeros((len(layers), grid.count), dtype=np.int32)
    for s in locked:
        rng = effective_range_m(s, threat, weather)
        f = floor_agl_map(obs, grid, s, rng, include_buildings=include_buildings, radial_step_m=coarse_step, azimuth_spacing_m=grid.cell_m)
        for li, layer in enumerate(layers):
            counts[li] += (f <= np.float32(layer.agl_m)).astype(np.int32)

    fixed_pts = [(s.east, s.north) for s in locked]
    min_sep = max(0.0, float(spec.min_separation_m))

    def sep_mask(exclude_idx: int | None, placed: list[int]) -> np.ndarray:
        ok = np.ones(len(candidates), dtype=bool)
        if min_sep <= 0:
            return ok
        for e, n in fixed_pts:
            ok &= np.hypot(cand_e - e, cand_n - n) >= min_sep
        for k, ci in enumerate(placed):
            if k == exclude_idx or ci < 0:
                continue
            ok &= np.hypot(cand_e - cand_e[ci], cand_n - cand_n[ci]) >= min_sep
        return ok

    order = sorted(range(len(movable)), key=lambda k: effective_range_m(movable[k], threat, weather), reverse=True)
    placed = [-1] * len(movable)
    trace: list[dict[str, Any]] = []
    evaluations = 0
    first_gain: np.ndarray | None = None
    running = 0.0

    def current_objective() -> float:
        total = 0.0
        for li in range(len(layers)):
            total += layer_w[li] * (
                float(np.sum(w_norm[counts[li] >= 1])) + spec.redundancy_weight * float(np.sum(w_norm[counts[li] >= 2]))
            )
        return total

    running = current_objective()
    for k in order:
        cov = cov_by_profile[movable[k].profile_key()]
        gains = _gain_vector(cov, counts, w_norm, layer_w, spec.redundancy_weight)
        evaluations += len(candidates)
        if first_gain is None:
            first_gain = gains.copy()
        gains = np.where(sep_mask(None, placed), gains, -np.inf)
        best = int(np.argmax(gains))
        if not np.isfinite(gains[best]):
            best = int(np.argmax(np.where(np.isfinite(gains), gains, -1)))
        placed[k] = best
        counts += cov[:, best, :].astype(np.int32)
        running = current_objective()
        trace.append(
            {
                "phase": "greedy",
                "sensorId": movable[k].id,
                "candidateIndex": best,
                "candidateKind": candidates[best].kind,
                "objective": round(running, 5),
            }
        )

    for pass_i in range(max(0, int(spec.swap_passes))):
        improved = False
        for k in order:
            cov = cov_by_profile[movable[k].profile_key()]
            ci = placed[k]
            counts -= cov[:, ci, :].astype(np.int32)
            gains = _gain_vector(cov, counts, w_norm, layer_w, spec.redundancy_weight)
            evaluations += len(candidates)
            current_gain = gains[ci]
            gains = np.where(sep_mask(k, placed), gains, -np.inf)
            best = int(np.argmax(gains))
            if np.isfinite(gains[best]) and gains[best] > current_gain + 1e-6:
                placed[k] = best
                improved = True
            counts += cov[:, placed[k], :].astype(np.int32)
            if placed[k] != ci:
                running = current_objective()
                trace.append(
                    {
                        "phase": f"swap-{pass_i + 1}",
                        "sensorId": movable[k].id,
                        "candidateIndex": placed[k],
                        "candidateKind": candidates[placed[k]].kind,
                        "objective": round(running, 5),
                    }
                )
        if not improved:
            break

    optimized = list(locked)
    for k, s in enumerate(movable):
        c = candidates[placed[k]]
        optimized.append(s.moved_to(c.east, c.north, origin="extra" if s.origin == "extra" else "optimized"))

    return {
        "sensors": optimized,
        "candidates": candidates,
        "candidateGain": first_gain if first_gain is not None else np.zeros(len(candidates)),
        "trace": trace,
        "evaluations": evaluations,
        "elapsedS": round(time.perf_counter() - t0, 3),
        "note": None,
    }
