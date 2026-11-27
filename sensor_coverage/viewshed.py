"""Vectorized radial-horizon viewshed: lowest detectable altitude per analysis cell.

For a sensor at height ``s`` and a target cell at horizontal distance ``D`` the
straight (4/3-earth refracted) ray is blocked iff some interior sample at
distance ``d < D`` satisfies::

    (z(d) - drop(d) - s) / d  >  (T - drop(D) - s) / D

Defining the running horizon slope ``A(D) = max_{d<D} (z(d) - drop(d) - s) / d``
gives the *lowest visible target height* in closed form::

    floor_up(D) = s + D * max(A(D), tan(min_elevation)) + drop(D)

Supports optional stand-off EW radar jamming burn-through range attenuation
(``jammers``) and omnidirectional floor precomputation for fast joint
(position, sector azimuth) optimization.
"""
from __future__ import annotations

import math

import numpy as np

from .contracts import JammerSpec, SensorSpec, curvature_drop_m, jammer_range_scale, sector_mask
from .grid import AnalysisGrid, ObstructionGrid

NOT_VISIBLE = np.float32(np.inf)


def sensor_site_up(obs: ObstructionGrid, sensor: SensorSpec) -> tuple[float, float]:
    """Return (site surface up incl. rooftops, antenna/optic up)."""
    site = obs.height_at(sensor.east, sensor.north, buildings=True)
    return site, site + float(sensor.mast_agl_m)


def floor_agl_map(
    obs: ObstructionGrid,
    grid: AnalysisGrid,
    sensor: SensorSpec,
    range_m: float,
    *,
    include_buildings: bool = True,
    radial_step_m: float | None = None,
    azimuth_spacing_m: float | None = None,
    jammers: list[JammerSpec] | None = None,
    ignore_sector: bool = False,
) -> np.ndarray:
    """Lowest detectable target altitude above ground (m AGL) for every analysis cell.

    Cells outside effective (possibly jammed) range, outside the sensor sector,
    or outside the verified environment are ``inf`` (never detectable).
    """
    _, s_up = sensor_site_up(obs, sensor)
    dx = grid.east - sensor.east
    dy = grid.north - sensor.north
    dist = np.hypot(dx, dy)
    az = np.mod(np.arctan2(dx, dy), 2.0 * math.pi)

    if jammers and sensor.type == "radar":
        eff_range = range_m * jammer_range_scale(sensor, az, jammers)
        in_range = dist <= eff_range
    else:
        in_range = dist <= range_m

    in_sector = np.ones(grid.count, dtype=bool) if ignore_sector else sector_mask(az, sensor)
    usable = grid.valid & in_range & in_sector
    floor = np.full(grid.count, NOT_VISIBLE, dtype=np.float32)
    if not np.any(usable):
        return floor

    min_el_slope = math.tan(math.radians(max(-89.0, sensor.min_elevation_deg)))
    clutter = max(0.0, float(sensor.clutter_floor_agl_m))

    if not sensor.requires_los:
        floor[usable] = np.float32(clutter)
        return floor

    step = float(radial_step_m) if radial_step_m else float(obs.cell_m)
    az_spacing = float(azimuth_spacing_m) if azimuth_spacing_m else float(grid.cell_m) * 0.7
    n_r = max(2, int(math.ceil(range_m / step)) + 1)
    n_az = int(min(2880, max(360, math.ceil(2.0 * math.pi * range_m / max(az_spacing, 1.0)))))

    radii = (np.arange(n_r, dtype=np.float64) + 1.0) * step
    azimuths = np.arange(n_az, dtype=np.float64) * (2.0 * math.pi / n_az)
    pe = sensor.east + np.sin(azimuths)[:, None] * radii[None, :]
    pn = sensor.north + np.cos(azimuths)[:, None] * radii[None, :]
    z, inside = obs.sample(pe, pn, buildings=include_buildings and sensor.buildings_block)
    slope = (z.astype(np.float64) - curvature_drop_m(radii)[None, :] - s_up) / radii[None, :]
    slope = np.where(inside, slope, -np.inf)
    horizon = np.maximum.accumulate(slope, axis=1)

    idx = np.nonzero(usable)[0]
    d_t = dist[idx]
    k_az = np.mod(np.rint(az[idx] / (2.0 * math.pi / n_az)).astype(np.int64), n_az)
    k_r = np.floor((d_t - step) / step).astype(np.int64) - 1
    has_prior = k_r >= 0
    a_t = np.full(len(idx), -np.inf, dtype=np.float64)
    a_t[has_prior] = horizon[k_az[has_prior], np.minimum(k_r[has_prior], n_r - 1)]
    a_t = np.maximum(a_t, min_el_slope)

    floor_up = s_up + d_t * a_t + curvature_drop_m(d_t)
    floor_agl = np.maximum(floor_up - grid.ground_up[idx].astype(np.float64), clutter)
    floor[idx] = np.maximum(floor_agl, 0.0).astype(np.float32)
    return floor


def encode_floor_u16(floor: np.ndarray) -> np.ndarray:
    """Compact uint16 encoding (metres, ceil); 65535 = never detectable."""
    out = np.full(floor.shape, 65535, dtype=np.uint16)
    finite = np.isfinite(floor)
    out[finite] = np.clip(np.ceil(floor[finite]), 0, 65534).astype(np.uint16)
    return out
