"""Coverage metrics, blind-spot geometry, and ingress-route warning-time analysis."""
from __future__ import annotations

import heapq
import math
from typing import Any

import numpy as np
from rasterio.features import shapes as raster_shapes
from rasterio.transform import Affine
from shapely.geometry import shape

from .contracts import (
    AssetSpec,
    JammerSpec,
    LayerSpec,
    RouteSpec,
    SensorSpec,
    ThreatSpec,
    WeatherSpec,
    curvature_drop_m,
    effective_range_m,
    jammer_range_scale,
    sector_mask,
)
from .grid import AnalysisGrid, ObstructionGrid
from .viewshed import floor_agl_map, sensor_site_up


def priority_weights(
    grid: AnalysisGrid,
    assets: list[AssetSpec],
    routes: list[RouteSpec] | None = None,
    *,
    route_weight: float = 1.0,
) -> np.ndarray:
    """Per-cell importance: 1 everywhere, asset priority inside warning rings, optional ingress corridor boost."""
    w = np.ones(grid.count, dtype=np.float64)
    for a in assets:
        d = np.hypot(grid.east - a.east, grid.north - a.north)
        w = np.where(d <= a.warning_radius_m, np.maximum(w, float(a.priority)), w)
    if routes and route_weight > 1.0:
        corridor_r = max(140.0, grid.cell_m * 1.8)
        for r in routes:
            if len(r.points_enu) < 2:
                continue
            pts = np.asarray(r.points_enu, dtype=np.float64)
            for k in range(len(pts) - 1):
                x0, y0 = pts[k, 0], pts[k, 1]
                x1, y1 = pts[k + 1, 0], pts[k + 1, 1]
                dx, dy = x1 - x0, y1 - y0
                seg_l2 = dx * dx + dy * dy
                if seg_l2 < 1e-6:
                    continue
                t = np.clip(((grid.east - x0) * dx + (grid.north - y0) * dy) / seg_l2, 0.0, 1.0)
                dist = np.hypot(grid.east - (x0 + t * dx), grid.north - (y0 + t * dy))
                w = np.where(dist <= corridor_r, np.maximum(w, float(route_weight)), w)
    w[~grid.valid] = 0.0
    return w


def coverage_curve(floors: np.ndarray, weights: np.ndarray, grid: AnalysisGrid) -> list[dict[str, Any]]:
    """Evaluate 10-point coverage-vs-altitude curve for smooth HUD charting."""
    alts = [15.0, 30.0, 50.0, 75.0, 100.0, 150.0, 220.0, 300.0, 450.0, 600.0]
    valid = grid.valid
    n_valid = max(int(np.sum(valid)), 1)
    w_total = max(float(np.sum(weights)), 1e-9)
    out = []
    for agl in alts:
        counts = coverage_counts(floors, agl)
        cov = (counts >= 1) & valid
        red = (counts >= 2) & valid
        out.append(
            {
                "aglM": agl,
                "coverageFraction": round(float(np.sum(cov)) / n_valid, 4),
                "weightedCoverageFraction": round(float(np.sum(weights[cov])) / w_total, 4),
                "redundantFraction": round(float(np.sum(red)) / n_valid, 4),
            }
        )
    return out


def blind_cause_breakdown(
    *,
    obs: ObstructionGrid,
    grid: AnalysisGrid,
    sensors: list[SensorSpec],
    floors_with_buildings: np.ndarray,
    threat: ThreatSpec,
    weather: WeatherSpec,
    agl_m: float,
    include_buildings: bool,
    jammers: list[JammerSpec] | None = None,
) -> dict[str, Any]:
    """Decompose blind area at ``agl_m`` into building-blocked, terrain-blocked, jammed, and range/sector-limited."""
    valid = grid.valid
    blind = (coverage_counts(floors_with_buildings, agl_m) == 0) & valid
    total_blind_km2 = float(np.sum(blind)) * grid.cell_area_km2
    if not np.any(blind) or not sensors:
        return {
            "aglM": float(agl_m),
            "totalBlindKm2": round(total_blind_km2, 3),
            "buildingBlockedKm2": 0.0,
            "terrainBlockedKm2": 0.0,
            "jammedKm2": 0.0,
            "outOfRangeOrSectorKm2": round(total_blind_km2, 3),
        }

    in_envelope = np.zeros(grid.count, dtype=bool)
    in_unjammed_envelope = np.zeros(grid.count, dtype=bool)
    bare_floors_list = []
    for s in sensors:
        rng = effective_range_m(s, threat, weather)
        dx = grid.east - s.east
        dy = grid.north - s.north
        dist = np.hypot(dx, dy)
        az = np.mod(np.arctan2(dx, dy), 2.0 * math.pi)
        sec_ok = sector_mask(az, s) & (agl_m >= s.clutter_floor_agl_m)
        in_unjammed_envelope |= (dist <= rng) & sec_ok
        eff_rng = rng * jammer_range_scale(s, az, jammers) if (jammers and s.type == "radar") else rng
        in_envelope |= (dist <= eff_rng) & sec_ok
        if include_buildings and s.requires_los and s.buildings_block:
            bare_floors_list.append(floor_agl_map(obs, grid, s, rng, include_buildings=False, jammers=jammers))
        else:
            bare_floors_list.append(None)

    if include_buildings:
        bare_seen = np.zeros(grid.count, dtype=bool)
        for si, bf in enumerate(bare_floors_list):
            f_use = bf if bf is not None else floors_with_buildings[si]
            bare_seen |= f_use <= np.float32(agl_m)
        building_blocked = blind & bare_seen
    else:
        building_blocked = np.zeros(grid.count, dtype=bool)

    jammed = blind & in_unjammed_envelope & (~in_envelope) & (~building_blocked)
    out_of_env = blind & (~in_unjammed_envelope)
    terrain_blocked = blind & in_envelope & (~building_blocked)

    return {
        "aglM": float(agl_m),
        "totalBlindKm2": round(total_blind_km2, 3),
        "buildingBlockedKm2": round(float(np.sum(building_blocked)) * grid.cell_area_km2, 3),
        "terrainBlockedKm2": round(float(np.sum(terrain_blocked)) * grid.cell_area_km2, 3),
        "jammedKm2": round(float(np.sum(jammed)) * grid.cell_area_km2, 3),
        "outOfRangeOrSectorKm2": round(float(np.sum(out_of_env)) * grid.cell_area_km2, 3),
    }


def los_cross_section(
    *,
    obs: ObstructionGrid,
    sensors: list[SensorSpec],
    target_east: float,
    target_north: float,
    target_agl_m: float,
    threat: ThreatSpec,
    weather: WeatherSpec,
    include_buildings: bool,
    jammers: list[JammerSpec] | None = None,
    n_profile_samples: int = 72,
) -> dict[str, Any]:
    """Compute exact 8m obstruction-raster LOS & elevation profile from every sensor to a target point."""
    target_ground = obs.height_at(target_east, target_north, buildings=False)
    target_up = target_ground + float(target_agl_m)

    rays: list[dict[str, Any]] = []
    for s in sensors:
        base_rng = effective_range_m(s, threat, weather)
        site_up, s_up = sensor_site_up(obs, s)
        dx = target_east - s.east
        dy = target_north - s.north
        horiz_m = math.hypot(dx, dy)
        az_rad = math.atan2(dx, dy) % (2.0 * math.pi)
        jam_scale = float(jammer_range_scale(s, np.array([az_rad]), jammers)[0]) if (jammers and s.type == "radar") else 1.0
        rng = base_rng * jam_scale
        in_sec = bool(sector_mask(az_rad, s))

        status = "clear"
        block_enu: list[float] | None = None
        block_dist_m: float | None = None
        min_clearance_m: float | None = None

        step = max(obs.cell_m, horiz_m / 400.0) if horiz_m > 1.0 else obs.cell_m
        n_dense = max(4, min(600, int(math.ceil(horiz_m / max(step, 1.0))) + 1))
        fracs = np.linspace(0.0, 1.0, n_dense)
        pe = s.east + fracs * dx
        pn = s.north + fracs * dy
        dists = fracs * horiz_m
        terr_z, _ = obs.sample(pe, pn, buildings=False)
        surf_z, _ = obs.sample(pe, pn, buildings=include_buildings and s.buildings_block)
        drop_d = curvature_drop_m(dists)
        drop_tot = curvature_drop_m(horiz_m)
        ray_z = s_up + fracs * (target_up - s_up) + (drop_d - fracs * drop_tot)

        if horiz_m > base_rng:
            status = "out_of_range"
        elif horiz_m > rng:
            status = "ew_jammed"
        elif not in_sec:
            status = "out_of_sector"
        elif target_agl_m < s.clutter_floor_agl_m:
            status = "below_clutter"
        elif not s.requires_los:
            status = "nlos_clear"
        else:
            # Check min elevation angle
            el_deg = math.degrees(math.atan2((target_up - drop_tot) - s_up, max(horiz_m, 1.0)))
            if el_deg < s.min_elevation_deg:
                status = "below_elevation_mask"
            elif n_dense > 2:
                interior = slice(1, n_dense - 1)
                # Exclude samples within one obstruction cell of endpoints
                valid_int = (dists[interior] > obs.cell_m) & (dists[interior] < max(0.0, horiz_m - obs.cell_m))
                if np.any(valid_int):
                    idx_int = np.nonzero(valid_int)[0] + 1
                    clearances = ray_z[idx_int] - surf_z[idx_int].astype(np.float64)
                    min_i = int(np.argmin(clearances))
                    min_clearance_m = round(float(clearances[min_i]), 1)
                    blocked_indices = idx_int[clearances < 0.0]
                    if len(blocked_indices) > 0:
                        hit_i = int(blocked_indices[0])  # first obstruction hit from sensor!
                        is_bldg = float(surf_z[hit_i]) - float(terr_z[hit_i]) > 1.0
                        status = "blocked_building" if is_bldg else "blocked_terrain"
                        block_dist_m = round(float(dists[hit_i]), 1)
                        block_enu = [
                            round(float(pe[hit_i]), 1),
                            round(float(pn[hit_i]), 1),
                            round(float(surf_z[hit_i]), 1),
                        ]

        # Downsample profile to n_profile_samples for lightweight SVG plotting
        pick = np.linspace(0, n_dense - 1, min(n_profile_samples, n_dense)).astype(int)
        profile = [
            [
                round(float(dists[i]), 1),
                round(float(terr_z[i]), 1),
                round(float(surf_z[i]), 1),
                round(float(ray_z[i]), 1),
            ]
            for i in pick
        ]

        rays.append(
            {
                "sensorId": s.id,
                "label": s.label or s.id,
                "type": s.type,
                "sensorEnu": [round(s.east, 1), round(s.north, 1), round(s_up, 1)],
                "distanceM": round(horiz_m, 1),
                "effectiveRangeM": round(rng, 1),
                "status": status,
                "clear": status in ("clear", "nlos_clear"),
                "minClearanceM": min_clearance_m,
                "blockDistanceM": block_dist_m,
                "blockPointEnu": block_enu,
                "profile": profile,
            }
        )

    return {
        "targetEnu": [round(target_east, 1), round(target_north, 1), round(target_up, 1)],
        "targetGroundUpM": round(target_ground, 1),
        "targetAglM": round(float(target_agl_m), 1),
        "rays": rays,
    }


def coverage_counts(floors: np.ndarray, agl_m: float) -> np.ndarray:
    """Number of sensors detecting each cell at ``agl_m`` (floors: S x N)."""
    if floors.size == 0:
        return np.zeros(floors.shape[1] if floors.ndim == 2 else 0, dtype=np.int16)
    return np.sum(floors <= np.float32(agl_m), axis=0).astype(np.int16)


def _largest_region_km2(mask_2d: np.ndarray, grid: AnalysisGrid) -> float:
    if not np.any(mask_2d):
        return 0.0
    north_up = Affine(grid.cell_m, 0.0, grid.east0, 0.0, -grid.cell_m, grid.north0 + grid.ny * grid.cell_m)
    flipped = mask_2d[::-1].astype(np.uint8)
    best = 0.0
    for geom, val in raster_shapes(flipped, mask=flipped.astype(bool), transform=north_up, connectivity=4):
        if val == 1:
            best = max(best, shape(geom).area)
    return best / 1e6


def blind_polygons_enu(grid: AnalysisGrid, blind_mask: np.ndarray, *, min_area_m2: float | None = None) -> list[Any]:
    """Vectorize blind cells into simplified ENU shapely polygons (largest first)."""
    mask_2d = (blind_mask & grid.valid).reshape(grid.ny, grid.nx)
    if not np.any(mask_2d):
        return []
    north_up = Affine(grid.cell_m, 0.0, grid.east0, 0.0, -grid.cell_m, grid.north0 + grid.ny * grid.cell_m)
    flipped = mask_2d[::-1].astype(np.uint8)
    min_area = min_area_m2 if min_area_m2 is not None else 2.0 * grid.cell_m * grid.cell_m
    polys = []
    for geom, val in raster_shapes(flipped, mask=flipped.astype(bool), transform=north_up, connectivity=4):
        if val != 1:
            continue
        poly = shape(geom)
        if poly.area < min_area:
            continue
        polys.append(poly.simplify(grid.cell_m * 0.45, preserve_topology=True))
    polys.sort(key=lambda p: p.area, reverse=True)
    return polys


def layer_metrics(floors: np.ndarray, weights: np.ndarray, grid: AnalysisGrid, agl_m: float) -> dict[str, Any]:
    counts = coverage_counts(floors, agl_m)
    valid = grid.valid
    n_valid = max(int(np.sum(valid)), 1)
    covered = (counts >= 1) & valid
    redundant = (counts >= 2) & valid
    blind = (counts == 0) & valid
    w_total = max(float(np.sum(weights)), 1e-9)
    priority_mask = (weights > 1.0) & valid
    out = {
        "aglM": float(agl_m),
        "coverageFraction": round(float(np.sum(covered)) / n_valid, 4),
        "weightedCoverageFraction": round(float(np.sum(weights[covered])) / w_total, 4),
        "redundantFraction": round(float(np.sum(redundant)) / n_valid, 4),
        "weightedRedundantFraction": round(float(np.sum(weights[redundant])) / w_total, 4),
        "blindAreaKm2": round(float(np.sum(blind)) * grid.cell_area_km2, 3),
        "largestBlindRegionKm2": round(_largest_region_km2(blind.reshape(grid.ny, grid.nx), grid), 3),
        "priorityZoneCoverageFraction": (
            round(float(np.sum(covered & priority_mask)) / max(int(np.sum(priority_mask)), 1), 4)
            if np.any(priority_mask)
            else None
        ),
    }
    return out


def objective_value(
    floors: np.ndarray,
    weights: np.ndarray,
    layers: list[LayerSpec],
    *,
    redundancy_weight: float,
) -> float:
    """Layer-weighted, priority-weighted coverage with a small redundancy bonus (0..1+)."""
    w_total = max(float(np.sum(weights)), 1e-9)
    lw_total = max(sum(l.weight for l in layers), 1e-9)
    score = 0.0
    for layer in layers:
        counts = coverage_counts(floors, layer.agl_m)
        cov = float(np.sum(weights[counts >= 1])) / w_total
        red = float(np.sum(weights[counts >= 2])) / w_total
        score += layer.weight * (cov + redundancy_weight * red)
    return score / lw_total


def floor_summary(floors: np.ndarray, weights: np.ndarray, grid: AnalysisGrid) -> dict[str, Any]:
    """Combined lowest-detectable-altitude surface statistics (min over sensors)."""
    if floors.size == 0:
        return {"medianFloorAglM": None, "priorityMedianFloorAglM": None, "neverDetectedFraction": 1.0}
    combined = np.min(floors, axis=0)
    valid = grid.valid
    finite = np.isfinite(combined) & valid
    pri = finite & (weights > 1.0)
    return {
        "medianFloorAglM": round(float(np.median(combined[finite])), 1) if np.any(finite) else None,
        "priorityMedianFloorAglM": round(float(np.median(combined[pri])), 1) if np.any(pri) else None,
        "neverDetectedFraction": round(1.0 - float(np.sum(finite)) / max(int(np.sum(valid)), 1), 4),
    }


def sample_route(route: RouteSpec, obs: ObstructionGrid, spacing_m: float) -> dict[str, np.ndarray]:
    pts = np.asarray(route.points_enu, dtype=np.float64)
    seg_len = np.hypot(np.diff(pts[:, 0]), np.diff(pts[:, 1]))
    cum = np.concatenate([[0.0], np.cumsum(seg_len)])
    total = float(cum[-1])
    n = max(2, int(math.ceil(total / max(spacing_m, 1.0))) + 1)
    s = np.linspace(0.0, total, n)
    east = np.interp(s, cum, pts[:, 0])
    north = np.interp(s, cum, pts[:, 1])
    agl = np.interp(s, cum, pts[:, 2])
    ground, _ = obs.sample(east, north, buildings=False)
    return {"s": s, "east": east, "north": north, "agl": agl, "up": ground.astype(np.float64) + agl, "total": total}


def route_metrics(
    route: RouteSpec,
    samples: dict[str, np.ndarray],
    floors: np.ndarray,
    sensor_ids: list[str],
    grid: AnalysisGrid,
) -> dict[str, Any]:
    idx = grid.nearest_index(samples["east"], samples["north"])
    if floors.size:
        seen_by = floors[:, idx] <= samples["agl"][None, :].astype(np.float32)
    else:
        seen_by = np.zeros((0, len(idx)), dtype=bool)
    detected = np.any(seen_by, axis=0) if seen_by.size else np.zeros(len(idx), dtype=bool)
    total = float(samples["total"])
    speed = max(route.speed_mps, 1.0)
    out: dict[str, Any] = {
        "routeId": route.id,
        "label": route.label,
        "lengthM": round(total, 1),
        "speedMps": route.speed_mps,
        "flightTimeS": round(total / speed, 1),
        "trackedFraction": round(float(np.mean(detected)), 4),
        "detected": "".join("1" if d else "0" for d in detected),
    }
    if np.any(detected):
        first = int(np.argmax(detected))
        remaining = total - float(samples["s"][first])
        # Longest undetected gap after first detection (track custody loss)
        gap = best_gap = 0
        for d in detected[first:]:
            gap = 0 if d else gap + 1
            best_gap = max(best_gap, gap)
        step_m = total / max(len(detected) - 1, 1)
        out.update(
            {
                "firstDetectionIndex": first,
                "firstDetectionEnu": [
                    round(float(samples["east"][first]), 1),
                    round(float(samples["north"][first]), 1),
                    round(float(samples["up"][first]), 1),
                ],
                "firstDetectionBy": [sensor_ids[k] for k in np.nonzero(seen_by[:, first])[0]],
                "warningDistanceM": round(remaining, 1),
                "warningTimeS": round(remaining / speed, 1),
                "longestGapAfterDetectionS": round(best_gap * step_m / speed, 1),
            }
        )
    else:
        out.update(
            {
                "firstDetectionIndex": None,
                "firstDetectionEnu": None,
                "firstDetectionBy": [],
                "warningDistanceM": 0.0,
                "warningTimeS": 0.0,
                "longestGapAfterDetectionS": None,
            }
        )
    return out


def find_adversarial_route(
    *,
    obs: ObstructionGrid,
    grid: AnalysisGrid,
    floors: np.ndarray,
    sensor_ids: list[str],
    asset_east: float,
    asset_north: float,
    agl_m: float = 35.0,
    speed_mps: float = 65.0,
    start_edge: str = "any",  # any | north | south | east | west
) -> dict[str, Any]:
    """A* / Dijkstra least-detection adversarial stealth path finder from perimeter to defended asset.

    Discovers the worst-case blind valley / urban-canyon corridor that minimizes
    cumulative sensor exposure at ``agl_m`` before reaching ``(asset_east, asset_north)``.
    """
    nx, ny = grid.nx, grid.ny
    n_cells = grid.count
    valid = grid.valid
    counts = coverage_counts(floors, agl_m) if floors.size else np.zeros(n_cells, dtype=np.int16)

    # Cell traversal cost multiplier: 1.0 in blind spots, steeply penalized where >=1 sensor detects
    cell_mult = np.where(counts == 0, 1.0, 1.0 + 22.0 * counts.astype(np.float64))
    # Slight preference to fly around tall obstacles if cruising below building height
    bldg_h, _ = obs.sample(grid.east, grid.north, buildings=True)
    terr_h = grid.ground_up
    obs_above = (bldg_h - terr_h) > max(5.0, agl_m * 0.85)
    cell_mult = np.where(obs_above, cell_mult * 1.35, cell_mult)

    goal_idx = int(grid.nearest_index(np.array([asset_east]), np.array([asset_north]))[0])
    goal_r, goal_c = divmod(goal_idx, nx)

    # Multi-source Dijkstra starting from valid AOI perimeter cells (exact optimum from any perimeter entry!)
    dist_g = np.full(n_cells, np.inf, dtype=np.float64)
    parent = np.full(n_cells, -1, dtype=np.int32)
    heap: list[tuple[float, int]] = []

    for r in range(ny):
        for c in range(nx):
            on_n = r == ny - 1
            on_s = r == 0
            on_e = c == nx - 1
            on_w = c == 0
            if start_edge == "north" and not on_n:
                continue
            if start_edge == "south" and not on_s:
                continue
            if start_edge == "east" and not on_e:
                continue
            if start_edge == "west" and not on_w:
                continue
            if start_edge == "any" and not (on_n or on_s or on_e or on_w):
                continue
            idx = r * nx + c
            if not valid[idx]:
                continue
            # Small heuristic bias toward goal so A* expands far fewer nodes
            h_rem = math.hypot((goal_c - c) * grid.cell_m, (goal_r - r) * grid.cell_m)
            dist_g[idx] = 0.0
            heapq.heappush(heap, (h_rem, idx))

    if not heap:
        raise ValueError("No valid perimeter entry cells found for adversarial routing.")

    step_card = float(grid.cell_m)
    step_diag = float(grid.cell_m) * math.sqrt(2.0)
    neighbors = (
        (-1, 0, step_card),
        (1, 0, step_card),
        (0, -1, step_card),
        (0, 1, step_card),
        (-1, -1, step_diag),
        (-1, 1, step_diag),
        (1, -1, step_diag),
        (1, 1, step_diag),
    )

    found = False
    while heap:
        f_cur, u = heapq.heappop(heap)
        ur, uc = divmod(u, nx)
        g_u = dist_g[u]
        if u == goal_idx:
            found = True
            break
        # Skip stale heap entries
        h_u = math.hypot((goal_c - uc) * step_card, (goal_r - ur) * step_card)
        if f_cur > g_u + h_u + 1e-6:
            continue

        for dr, dc, step_d in neighbors:
            vr, vc = ur + dr, uc + dc
            if vr < 0 or vr >= ny or vc < 0 or vc >= nx:
                continue
            v = vr * nx + vc
            if not valid[v]:
                continue
            edge_cost = step_d * 0.5 * (cell_mult[u] + cell_mult[v])
            cand_g = g_u + edge_cost
            if cand_g < dist_g[v]:
                dist_g[v] = cand_g
                parent[v] = u
                h_v = math.hypot((goal_c - vc) * step_card, (goal_r - vr) * step_card)
                heapq.heappush(heap, (cand_g + h_v, v))

    if not found:
        raise ValueError("Unable to find a valid adversarial corridor to the defended asset.")

    # Reconstruct cell path perimeter -> goal
    path_cells: list[int] = []
    cur = goal_idx
    while cur >= 0:
        path_cells.append(cur)
        cur = int(parent[cur])
    path_cells.reverse()

    # Subsample slightly to smooth staircase jitter while preserving valley turns
    if len(path_cells) > 3:
        stride = max(1, len(path_cells) // 48)
        picked = path_cells[::stride]
        if picked[-1] != path_cells[-1]:
            picked.append(path_cells[-1])
    else:
        picked = path_cells

    pts_enu = [
        [round(float(grid.east[i]), 1), round(float(grid.north[i]), 1), round(float(agl_m), 1)]
        for i in picked
    ]
    pts_enu[-1][0] = round(float(asset_east), 1)
    pts_enu[-1][1] = round(float(asset_north), 1)

    route_spec = RouteSpec(
        id="adversarial-stealth-route",
        points_enu=pts_enu,
        speed_mps=float(speed_mps),
        label=f"Adversarial Stealth Corridor ({int(round(agl_m))}m AGL)",
    )
    smp = sample_route(route_spec, obs, max(20.0, grid.cell_m * 0.5))
    metrics = route_metrics(route_spec, smp, floors, sensor_ids, grid)

    env = obs.env
    lons, lats = env.enu_to_lonlat_fast(smp["east"], smp["north"])
    points_ll = [[round(float(lons[k]), 6), round(float(lats[k]), 6), round(float(agl_m), 1)] for k in range(len(lons))]
    samples_3d = [
        [round(float(smp["east"][k]), 1), round(float(smp["north"][k]), 1), round(float(smp["up"][k]), 1), round(float(smp["agl"][k]), 1)]
        for k in range(len(smp["s"]))
    ]
    return {
        "routeId": route_spec.id,
        "label": route_spec.label,
        "derivation": "synthetic/derived least-detection A* stealth corridor",
        "aglM": round(float(agl_m), 1),
        "speedMps": round(float(speed_mps), 1),
        "startEdge": start_edge,
        "stealthFraction": round(1.0 - float(metrics["trackedFraction"]), 4),
        "trackedFraction": metrics["trackedFraction"],
        "lengthM": metrics["lengthM"],
        "flightTimeS": metrics["flightTimeS"],
        "warningTimeS": metrics["warningTimeS"],
        "warningDistanceM": metrics["warningDistanceM"],
        "longestGapAfterDetectionS": metrics["longestGapAfterDetectionS"],
        "firstDetectionIndex": metrics["firstDetectionIndex"],
        "firstDetectionEnu": metrics["firstDetectionEnu"],
        "firstDetectionBy": metrics["firstDetectionBy"],
        "detected": metrics["detected"],
        "samples": samples_3d,
        "pointsLonLat": points_ll,
    }
