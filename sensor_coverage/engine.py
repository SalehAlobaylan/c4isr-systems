"""End-to-end coverage evaluation: baseline vs optimized layouts on a verified environment."""
from __future__ import annotations

import base64
import datetime as dt
import time
from typing import Any

import numpy as np

from sim.environment import EnvironmentModel

from .analysis import (
    blind_cause_breakdown,
    coverage_curve,
    floor_summary,
    layer_metrics,
    objective_value,
    priority_weights,
    route_metrics,
    sample_route,
)
from .contracts import (
    COV_ENGINE_VERSION,
    COVERAGE_REPLAY_SCHEMA,
    DERIVATION_LABEL,
    SCOPE_EVALUATION,
    SCOPE_EXERCISE_INPUT,
    CoverageScenario,
    SensorSpec,
    SENSOR_DEFAULTS,
    effective_range_m,
)
from .grid import AnalysisGrid, ObstructionGrid
from .optimizer import CandidateFloorCache, optimize_placement
from .viewshed import encode_floor_u16, floor_agl_map, sensor_site_up


def _b64(arr: np.ndarray) -> str:
    return base64.b64encode(np.ascontiguousarray(arr).tobytes()).decode("ascii")


def evaluate_layout(
    *,
    sensors: list[SensorSpec],
    scenario: CoverageScenario,
    env: EnvironmentModel,
    obs: ObstructionGrid,
    grid: AnalysisGrid,
    weights: np.ndarray,
    route_samples: dict[str, dict[str, np.ndarray]],
) -> dict[str, Any]:
    floors_list = []
    sensors_out = []
    for s in sensors:
        rng = effective_range_m(s, scenario.threat, scenario.weather)
        f = floor_agl_map(obs, grid, s, rng, include_buildings=scenario.include_buildings)
        floors_list.append(f)
        site_up, ant_up = sensor_site_up(obs, s)
        lon, lat, _ = env.enu_to_geodetic(s.east, s.north, ant_up)
        sensors_out.append(
            {
                "id": s.id,
                "type": s.type,
                "label": s.label or s.id,
                "origin": s.origin,
                "locked": s.locked,
                "enu": [round(s.east, 2), round(s.north, 2), round(ant_up, 2)],
                "siteUpM": round(site_up, 2),
                "siteOnRoof": bool(site_up - obs.height_at(s.east, s.north, buildings=False) > 2.0),
                "lon": round(lon, 6),
                "lat": round(lat, 6),
                "mastAglM": s.mast_agl_m,
                "maxRangeM": s.max_range_m,
                "effectiveRangeM": round(rng, 1),
                "minElevationDeg": s.min_elevation_deg,
                "clutterFloorAglM": s.clutter_floor_agl_m,
                "sectorCenterDeg": s.sector_center_deg,
                "sectorWidthDeg": s.sector_width_deg,
                "requiresLos": s.requires_los,
                "floorB64": _b64(encode_floor_u16(f)),
            }
        )
    floors = np.stack(floors_list) if floors_list else np.zeros((0, grid.count), dtype=np.float32)

    layers_out = [layer_metrics(floors, weights, grid, l.agl_m) for l in scenario.layers]
    # Sole-coverage contribution of each sensor at each layer (what is lost if it fails)
    for si, so in enumerate(sensors_out):
        sole = []
        for layer in scenario.layers:
            seen = floors <= np.float32(layer.agl_m)
            others = np.delete(seen, si, axis=0)
            only = seen[si] & ~(np.any(others, axis=0) if others.size else np.zeros(grid.count, dtype=bool)) & grid.valid
            sole.append(round(float(np.sum(only)) * grid.cell_area_km2, 3))
        so["soleCoverageKm2"] = sole
        so["coverageKm2"] = [
            round(float(np.sum((floors[si] <= np.float32(l.agl_m)) & grid.valid)) * grid.cell_area_km2, 3)
            for l in scenario.layers
        ]

    sensor_ids = [s.id for s in sensors]
    routes_out = [
        route_metrics(r, route_samples[r.id], floors, sensor_ids, grid) for r in scenario.routes if r.id in route_samples
    ]
    primary_agl = scenario.layers[0].agl_m if scenario.layers else 30.0
    causes = blind_cause_breakdown(
        obs=obs,
        grid=grid,
        sensors=sensors,
        floors_with_buildings=floors,
        threat=scenario.threat,
        weather=scenario.weather,
        agl_m=primary_agl,
        include_buildings=scenario.include_buildings,
    )
    curve = coverage_curve(floors, weights, grid)
    return {
        "sensors": sensors_out,
        "floors": floors,
        "metrics": {
            "scope": SCOPE_EVALUATION,
            "objective": round(
                objective_value(floors, weights, scenario.layers, redundancy_weight=scenario.optimizer.redundancy_weight), 5
            ),
            "layers": layers_out,
            "coverageCurve": curve,
            "blindCauses": causes,
            "floor": floor_summary(floors, weights, grid),
            "routes": routes_out,
            "minWarningTimeS": min((r["warningTimeS"] for r in routes_out), default=None),
        },
    }


def _comparison(base: dict[str, Any], opt: dict[str, Any] | None) -> dict[str, Any] | None:
    if opt is None:
        return None
    bm, om = base["metrics"], opt["metrics"]
    layers = []
    for bl, ol in zip(bm["layers"], om["layers"]):
        layers.append(
            {
                "aglM": bl["aglM"],
                "coverageDelta": round(ol["coverageFraction"] - bl["coverageFraction"], 4),
                "weightedCoverageDelta": round(ol["weightedCoverageFraction"] - bl["weightedCoverageFraction"], 4),
                "blindAreaDeltaKm2": round(ol["blindAreaKm2"] - bl["blindAreaKm2"], 3),
            }
        )
    routes = []
    for br, orr in zip(bm["routes"], om["routes"]):
        routes.append(
            {
                "routeId": br["routeId"],
                "warningTimeDeltaS": round(orr["warningTimeS"] - br["warningTimeS"], 1),
                "trackedFractionDelta": round(orr["trackedFraction"] - br["trackedFraction"], 4),
            }
        )
    b_obj, o_obj = bm["objective"], om["objective"]
    return {
        "objectiveDelta": round(o_obj - b_obj, 5),
        "objectiveGainPct": round(((o_obj - b_obj) / b_obj) * 100.0, 1) if b_obj > 0 else None,
        "layers": layers,
        "routes": routes,
    }


def run_coverage(
    scenario: CoverageScenario,
    *,
    env: EnvironmentModel,
    obs: ObstructionGrid,
    cache: CandidateFloorCache | None = None,
    cache_scope: tuple[Any, ...] = (),
    on_stage: Any = None,
) -> dict[str, Any]:
    """Evaluate baseline layout, optimize placement (optional), and assemble the replay bundle."""
    t0 = time.perf_counter()

    def stage(msg: str) -> None:
        if on_stage:
            on_stage(msg)

    grid = AnalysisGrid.build(obs, cell_m=scenario.analysis_cell_m)
    weights = priority_weights(grid, scenario.assets, scenario.routes, route_weight=scenario.optimizer.route_weight)
    route_samples = {r.id: sample_route(r, obs, grid.cell_m * 0.5) for r in scenario.routes if len(r.points_enu) >= 2}

    stage("2/5 Computing baseline terrain + building viewsheds…")
    base = evaluate_layout(
        sensors=scenario.sensors, scenario=scenario, env=env, obs=obs, grid=grid, weights=weights, route_samples=route_samples
    )

    opt_eval = None
    opt_info: dict[str, Any] = {"enabled": False}
    if scenario.optimizer.enabled:
        stage("3/5 Generating candidate sites & running greedy + swap placement optimizer…")
        t_type = scenario.optimizer.extra_sensor_type

        def extra_template(k: int) -> SensorSpec:
            d = SENSOR_DEFAULTS[t_type]
            return SensorSpec(
                id=f"extra-{t_type}-{k + 1}",
                type=t_type,
                east=0.0,
                north=0.0,
                mast_agl_m=float(d["mastAglM"]),
                max_range_m=float(d["maxRangeM"]),
                min_elevation_deg=float(d["minElevationDeg"]),
                clutter_floor_agl_m=float(d["clutterFloorAglM"]),
                label=f"Extra {t_type.replace('_', '/').upper()} {k + 1}",
                origin="extra",
            )

        result = optimize_placement(
            obs=obs,
            grid=grid,
            baseline=scenario.sensors,
            layers=scenario.layers,
            weights=weights,
            threat=scenario.threat,
            weather=scenario.weather,
            spec=scenario.optimizer,
            include_buildings=scenario.include_buildings,
            cache=cache,
            cache_scope=cache_scope,
            extra_template=extra_template,
        )
        stage("4/5 Evaluating optimized layout & ingress-route warning times…")
        opt_eval = evaluate_layout(
            sensors=result["sensors"], scenario=scenario, env=env, obs=obs, grid=grid, weights=weights, route_samples=route_samples
        )
        gains = np.asarray(result["candidateGain"], dtype=np.float64)
        g_max = float(np.max(gains)) if gains.size and np.max(gains) > 0 else 1.0
        opt_info = {
            "enabled": True,
            "method": "greedy weighted max-coverage (1-1/e) + 1-swap local search",
            "candidates": [
                {
                    "enu": [round(c.east, 1), round(c.north, 1), round(c.site_up, 1)],
                    "kind": c.kind,
                    "gain": round(float(gains[i]) / g_max, 4) if gains.size else 0.0,
                }
                for i, c in enumerate(result["candidates"])
            ],
            "trace": result["trace"],
            "evaluations": result["evaluations"],
            "elapsedS": result["elapsedS"],
            "note": result["note"],
            "minSeparationM": scenario.optimizer.min_separation_m,
            "extraSensors": scenario.optimizer.extra_sensors,
        }
    else:
        stage("4/5 Evaluating ingress-route warning times…")

    stage("5/5 Assembling coverage replay bundle…")
    assets_out = []
    for a in scenario.assets:
        up = obs.height_at(a.east, a.north, buildings=False)
        lon, lat, _ = env.enu_to_geodetic(a.east, a.north, up)
        assets_out.append(
            {
                "id": a.id,
                "label": a.label,
                "enu": [round(a.east, 1), round(a.north, 1), round(up, 1)],
                "lon": round(lon, 6),
                "lat": round(lat, 6),
                "warningRadiusM": a.warning_radius_m,
                "priority": a.priority,
            }
        )
    routes_out = []
    for r in scenario.routes:
        smp = route_samples.get(r.id)
        if smp is None:
            continue
        routes_out.append(
            {
                "id": r.id,
                "label": r.label,
                "speedMps": r.speed_mps,
                "lengthM": round(float(smp["total"]), 1),
                "samples": [
                    [round(float(smp["east"][k]), 1), round(float(smp["north"][k]), 1), round(float(smp["up"][k]), 1), round(float(smp["agl"][k]), 1)]
                    for k in range(len(smp["s"]))
                ],
            }
        )

    configs: dict[str, Any] = {
        "baseline": {"label": "Baseline (operator layout)", "sensors": base["sensors"], "metrics": base["metrics"]},
        "optimized": (
            {"label": "Optimized placement", "sensors": opt_eval["sensors"], "metrics": opt_eval["metrics"]} if opt_eval else None
        ),
    }
    weather = scenario.weather
    return {
        "schema": COVERAGE_REPLAY_SCHEMA,
        "engineVersion": COV_ENGINE_VERSION,
        "derivation": DERIVATION_LABEL,
        "generatedAt": dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
        "elapsedS": round(time.perf_counter() - t0, 3),
        "scenario": {"scope": SCOPE_EXERCISE_INPUT, **scenario.to_dict()},
        "environment": env.metadata(),
        "viewerContext": env.export_viewer_context(),
        "obstructionGrid": obs.metadata(),
        "grid": {
            **grid.metadata(),
            "groundB64": _b64(grid.ground_up.astype(np.float32)),
            "weightsB64": _b64(weights.astype(np.float32)),
            "validB64": _b64(grid.valid.astype(np.uint8)),
            "floorEncoding": "uint16 metres AGL (ceil); 65535 = never detectable",
        },
        "layers": [{"aglM": l.agl_m, "weight": l.weight} for l in scenario.layers],
        "threat": {"rcsM2": scenario.threat.rcs_m2, "label": scenario.threat.label},
        "weather": {
            "visibilityKm": weather.visibility_km,
            "surfaceWindMps": round(weather.surface_wind_mps, 2),
            "windSource": weather.wind_source,
        },
        "assets": assets_out,
        "routes": routes_out,
        "configs": configs,
        "optimizer": opt_info,
        "comparison": _comparison(base, opt_eval),
        "_floors": {"baseline": base["floors"], "optimized": opt_eval["floors"] if opt_eval else None, "grid": grid},
    }
