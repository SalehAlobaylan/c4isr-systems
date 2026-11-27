"""Stateful local Sensor Coverage service bridging EMAD's 2D map and the coverage 3D viewer.

Reuses the verified environment discovery of the 3D simulation workbench
(``sim.service.discover_presets``) so both simulations run on identical frozen
Riyadh / owned-fixture packages, but keeps its own jobs, artifacts
(``tmp/cov-runs/<jobId>/``), HTTP port (8766), and viewer.
"""
from __future__ import annotations

import base64
from concurrent.futures import Executor
import datetime as dt
import json
import math
from pathlib import Path
import re
import secrets
import threading
from typing import Any

import numpy as np
import shapely

from sim_core.atmosphere import AtmosphereModel, load_cams_dust
from sim_core.cache import SharedBiomeCache
from sim_core.environment import EnvironmentModel
from sim_core.handoff import SensorNetworkStore
from sim_core.registry import BiomePreset, discover_biomes
from sim.contracts import ScenarioSpec

from .analysis import blind_polygons_enu, coverage_counts, find_adversarial_route, los_cross_section
from .contracts import (
    COV_ENGINE_VERSION,
    DEFAULT_LAYERS,
    SENSOR_DEFAULTS,
    SENSOR_TYPES,
    AssetSpec,
    CoverageScenario,
    JammerSpec,
    LayerSpec,
    OptimizerSpec,
    RouteSpec,
    SensorSpec,
    ThreatSpec,
    WeatherSpec,
    effective_range_m,
)
from .engine import run_coverage
from .grid import AnalysisGrid, ObstructionGrid
from .optimizer import CandidateFloorCache
from .viewshed import encode_floor_u16, floor_agl_map, sensor_site_up

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_RUNS_ROOT = REPO_ROOT / "tmp" / "cov-runs"
JOB_ID_RE = re.compile(r"covjob_[a-f0-9]{12}")


def _clampf(value: Any, lo: float, hi: float, default: float) -> float:
    try:
        v = float(value)
    except (TypeError, ValueError):
        return default
    if not math.isfinite(v):
        return default
    return max(lo, min(hi, v))


class CoverageService:
    """In-process coverage manager with environment / obstruction caching and async jobs."""

    def __init__(
        self,
        *,
        repo_root: Path = REPO_ROOT,
        runs_root: Path = DEFAULT_RUNS_ROOT,
        public_origin: str = "http://127.0.0.1:8766",
        biome_cache: SharedBiomeCache | None = None,
        network_store: SensorNetworkStore | None = None,
        compute_executor: Executor | None = None,
        viewer_path_prefix: str = "",
    ) -> None:
        self.repo_root = Path(repo_root).resolve()
        self.runs_root = Path(runs_root).resolve()
        self.runs_root.mkdir(parents=True, exist_ok=True)
        self.public_origin = public_origin.rstrip("/")
        self.viewer_path_prefix = viewer_path_prefix.rstrip("/")
        self._biome_cache = biome_cache or SharedBiomeCache(self.repo_root)
        self._presets: dict[str, BiomePreset] = self._biome_cache.presets
        self._network_store = network_store or SensorNetworkStore(self.repo_root / "tmp" / "shared-layouts")
        self._compute_executor = compute_executor
        self._jobs_lock = threading.Lock()
        self._run_lock = threading.Lock()  # one heavy optimization at a time
        self._jobs: dict[str, dict[str, Any]] = {}
        self._cancel_flags: dict[str, threading.Event] = {}
        self._floor_cache = CandidateFloorCache()
        self._load_existing_runs()

    def _viewer_url(self, job_id: str) -> str:
        prefix = f"{self.viewer_path_prefix}/" if self.viewer_path_prefix else "/"
        return f"{self.public_origin}{prefix}?run={job_id}"

    # -- persistence ------------------------------------------------------
    def _load_existing_runs(self) -> None:
        for meta_path in sorted(self.runs_root.glob("*/job.json")):
            try:
                rec = json.loads(meta_path.read_text(encoding="utf-8"))
                job_id = str(rec["jobId"])
                if rec.get("status") == "running":
                    rec["status"] = "interrupted"
                    rec["stage"] = "Server restarted before run completed"
                rec["viewerUrl"] = self._viewer_url(job_id)
                self._jobs[job_id] = rec
            except Exception:
                continue

    # -- environment ------------------------------------------------------
    def _template(self, preset: BiomePreset) -> dict[str, Any]:
        if preset.scenario_template_path and preset.scenario_template_path.is_file():
            return json.loads(preset.scenario_template_path.read_text(encoding="utf-8"))
        return {}

    def _get_env(self, env_id: str) -> tuple[BiomePreset, EnvironmentModel, ObstructionGrid]:
        return self._biome_cache.get_env_and_obs(env_id)

    def _era5_surface_wind(self, preset: BiomePreset, env: EnvironmentModel) -> tuple[float, str]:
        return self._biome_cache.get_surface_wind(preset.id)

    def _enu_to_ll(self, env: EnvironmentModel, e: float, n: float) -> list[float]:
        lon, lat, _ = env.enu_to_geodetic(float(e), float(n), env.up_median)
        return [round(lon, 6), round(lat, 6)]

    def _default_draft(self, preset: BiomePreset, env: EnvironmentModel) -> dict[str, Any]:
        tpl = self._template(preset)
        span_e = env.east_max - env.east_min
        span_n = env.north_max - env.north_min
        cx = 0.5 * (env.east_min + env.east_max)
        cy = 0.5 * (env.north_min + env.north_max)

        def clamp_en(e: float, n: float) -> tuple[float, float]:
            pad = 0.06 * min(span_e, span_n)
            return (
                max(env.east_min + pad, min(env.east_max - pad, e)),
                max(env.north_min + pad, min(env.north_max - pad, n)),
            )

        wps = (tpl.get("threat") or {}).get("waypoints_enu") or []
        p_center = tpl.get("protectedZoneCenterEnu") or (
            wps[-1][:2] if wps else [cx + 0.18 * span_e, cy + 0.18 * span_n]
        )
        pe, pn = clamp_en(float(p_center[0]), float(p_center[1]))
        radar_pos = (tpl.get("radar") or {}).get("position_enu") or [pe - 0.06 * span_e, pn - 0.06 * span_n, 15.0]
        re_, rn_ = clamp_en(float(radar_pos[0]), float(radar_pos[1]))
        radar_range = round(max(9000.0, min(22000.0, min(span_e, span_n) * 0.72)), 0)

        def sensor(sid: str, stype: str, e: float, n: float, label: str) -> dict[str, Any]:
            d = SENSOR_DEFAULTS[stype]
            e, n = clamp_en(e, n)
            lon, lat = self._enu_to_ll(env, e, n)
            return {
                "id": sid,
                "type": stype,
                "label": label,
                "lon": lon,
                "lat": lat,
                "mastAglM": d["mastAglM"],
                "maxRangeM": radar_range if stype == "radar" else d["maxRangeM"],
                "minElevationDeg": d["minElevationDeg"],
                "clutterFloorAglM": d["clutterFloorAglM"],
                "sectorCenterDeg": 0.0,
                "sectorWidthDeg": 360.0,
                "locked": False,
            }

        sensors = [
            sensor("radar-1", "radar", re_, rn_, "Site radar"),
            sensor("eo-1", "eo_ir", pe + 120.0, pn - 80.0, "Site EO/IR"),
            sensor("acoustic-1", "acoustic", pe - 100.0, pn + 60.0, "Site acoustic"),
        ]
        a_lon, a_lat = self._enu_to_ll(env, pe, pn)
        assets = [
            {
                "id": "asset-1",
                "label": "Defended site",
                "lon": a_lon,
                "lat": a_lat,
                "warningRadiusM": round(min(3000.0, 0.45 * min(span_e, span_n)), 0),
                "priority": 4.0,
            }
        ]

        routes = []
        if len(wps) >= 2:
            pts = []
            for w in wps:
                e, n = clamp_en(float(w[0]), float(w[1]))
                pts.append([*self._enu_to_ll(env, e, n), float(w[2])])
            routes.append(
                {
                    "id": "route-1",
                    "label": "Template UAV ingress",
                    "speedMps": float((tpl.get("threat") or {}).get("speed_mps", 68.0)),
                    "points": pts,
                }
            )
        for rid, label, start, agl, speed in (
            ("route-2", "Low NW UAV ingress", (env.east_min + 0.1 * span_e, env.north_max - 0.1 * span_n), 40.0, 60.0),
            ("route-3", "Terrain-hugging SE cruise", (env.east_max - 0.08 * span_e, env.north_min + 0.1 * span_n), 35.0, 220.0),
        ):
            se, sn = clamp_en(*start)
            me, mn = clamp_en((se + pe) / 2.0 + 0.08 * span_e, (sn + pn) / 2.0)
            routes.append(
                {
                    "id": rid,
                    "label": label,
                    "speedMps": speed,
                    "points": [
                        [*self._enu_to_ll(env, se, sn), agl],
                        [*self._enu_to_ll(env, me, mn), agl],
                        [*self._enu_to_ll(env, pe, pn), agl],
                    ],
                }
            )

        wind, _ = self._era5_surface_wind(preset, env)
        return {
            "environmentId": preset.id,
            "title": f"{preset.title.split('(')[0].strip()} — Sensor Coverage Study",
            "sensors": sensors,
            "assets": assets,
            "routes": routes,
            "layers": [dict(l) for l in DEFAULT_LAYERS],
            "threat": {"rcsM2": 0.1, "label": "Small one-way UAV"},
            "weather": {"visibilityKm": 10.0, "useEra5Wind": True, "surfaceWindMps": wind},
            "optimizer": {
                "enabled": True,
                "extraSensors": 0,
                "extraSensorType": "radar",
                "minSeparationM": 600.0,
                "allowRooftops": True,
                "swapPasses": 2,
                "redundancyWeight": 0.15,
                "routeWeight": 2.5,
                "optimizeOrientation": True,
            },
            "jammers": [],
            "includeBuildings": True,
            "mapLayerAglM": DEFAULT_LAYERS[0]["aglM"],
        }

    def _bounds_wgs84(self, env: EnvironmentModel) -> list[float]:
        lons, lats = [], []
        for ce, cn in ((env.east_min, env.north_min), (env.east_max, env.north_min), (env.east_max, env.north_max), (env.east_min, env.north_max)):
            lon, lat, _ = env.enu_to_geodetic(ce, cn, env.up_median)
            lons.append(lon)
            lats.append(lat)
        return [round(min(lons), 6), round(min(lats), 6), round(max(lons), 6), round(max(lats), 6)]

    def list_environments(self) -> list[dict[str, Any]]:
        out = []
        for preset in self._presets.values():
            _, env, obs = self._get_env(preset.id)
            wind, wind_src = self._era5_surface_wind(preset, env)
            cams = load_cams_dust(preset.cams_dust_path)
            out.append(
                {
                    "id": preset.id,
                    "title": preset.title,
                    "biome": preset.biome,
                    "description": preset.description,
                    "manifestSha256": env.manifest_sha256,
                    "buildingCount": len(env.buildings),
                    "roadCount": len(env.roads),
                    "enrichment": env.metadata().get("enrichment"),
                    "hasGlb": bool(preset.glb_path and preset.glb_path.is_file()),
                    "hasImagery": bool(preset.imagery_path and preset.imagery_path.is_file()),
                    "boundsWgs84": self._bounds_wgs84(env),
                    "origin": list(env.origin),
                    "terrainRangeM": [round(env.up_min, 1), round(env.up_max, 1)],
                    "obstructionGrid": obs.metadata(),
                    "weather": {"surfaceWindMps": wind, "source": wind_src, "camsDust": cams},
                    "sensorDefaults": SENSOR_DEFAULTS,
                    "defaults": self._default_draft(preset, env),
                }
            )
        return out

    def health(self) -> dict[str, Any]:
        with self._jobs_lock:
            running = sum(1 for j in self._jobs.values() if j.get("status") == "running")
            total = len(self._jobs)
        return {
            "status": "ok",
            "service": "emad-sensor-coverage",
            "engineVersion": COV_ENGINE_VERSION,
            "capabilities": [
                "los-profile",
                "preview-sensor",
                "blind-causes",
                "altitude-curve",
                "adversarial-route",
                "ew-jamming",
                "orientation-opt",
                "export-network",
            ],
            "viewerOrigin": f"{self.public_origin}{self.viewer_path_prefix}",
            "environmentCount": len(self._presets),
            "runningJobs": running,
            "totalRuns": total,
        }

    def list_runs(self) -> list[dict[str, Any]]:
        with self._jobs_lock:
            items = [dict(j) for j in self._jobs.values()]
        items.sort(key=lambda x: str(x.get("createdAt", "")), reverse=True)
        return items[:30]

    def get_run(self, job_id: str) -> dict[str, Any] | None:
        with self._jobs_lock:
            rec = self._jobs.get(job_id)
            if rec is None:
                return None
            out = dict(rec)
            out["viewerUrl"] = self._viewer_url(job_id)
            return out

    def cancel_run(self, job_id: str) -> dict[str, Any] | None:
        with self._jobs_lock:
            rec = self._jobs.get(job_id)
            if rec is None:
                return None
            flag = self._cancel_flags.get(job_id)
            if flag is not None:
                flag.set()
            if rec.get("status") in ("running", "queued"):
                rec["status"] = "cancelled"
                rec["stage"] = "Cancelled by operator"
                rec["completedAt"] = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")
            out = dict(rec)
            out["viewerUrl"] = self._viewer_url(job_id)
        run_dir = self.runs_root / job_id
        if run_dir.is_dir():
            (run_dir / "job.json").write_text(json.dumps(out, indent=2), encoding="utf-8")
        return out

    def run_assets(self, job_id: str) -> tuple[Path, Path | None, Path | None] | None:
        rec = self.get_run(job_id)
        if rec is None:
            return None
        preset = self._presets.get(str(rec.get("environmentId", "")))
        glb = preset.glb_path if (preset and preset.glb_path and preset.glb_path.is_file()) else None
        imagery = preset.imagery_path if (preset and preset.imagery_path and preset.imagery_path.is_file()) else None
        return self.runs_root / job_id, glb, imagery

    # -- request parsing -------------------------------------------------
    def build_scenario(self, payload: dict[str, Any]) -> tuple[BiomePreset, EnvironmentModel, ObstructionGrid, CoverageScenario]:
        if not self._presets:
            raise ValueError("No verified environments are available on this machine.")
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        preset, env, obs = self._get_env(env_id)

        def to_en(lon: Any, lat: Any, label: str) -> tuple[float, float]:
            e, n, _ = env.geodetic_to_enu(float(lon), float(lat), env.up_median)
            pad = 50.0
            if not (env.east_min - pad <= e <= env.east_max + pad and env.north_min - pad <= n <= env.north_max + pad):
                raise ValueError(f"{label} ({float(lon):.5f}, {float(lat):.5f}) lies outside the verified {preset.id} bounds.")
            return round(e, 2), round(n, 2)

        sensors_in = payload.get("sensors") or []
        if not isinstance(sensors_in, list) or not sensors_in:
            raise ValueError("Place at least one sensor before running the coverage study.")
        if len(sensors_in) > 16:
            raise ValueError("At most 16 sensors are supported per study.")
        sensors: list[SensorSpec] = []
        seen_ids: set[str] = set()
        for k, s in enumerate(sensors_in):
            stype = str(s.get("type", "radar"))
            if stype not in SENSOR_TYPES:
                raise ValueError(f"Unsupported sensor type: {stype}")
            d = SENSOR_DEFAULTS[stype]
            sid = re.sub(r"[^a-zA-Z0-9_-]", "", str(s.get("id") or f"{stype}-{k + 1}"))[:32] or f"{stype}-{k + 1}"
            if sid in seen_ids:
                sid = f"{sid}-{k + 1}"
            seen_ids.add(sid)
            e, n = to_en(s.get("lon"), s.get("lat"), f"Sensor {sid}")
            sensors.append(
                SensorSpec(
                    id=sid,
                    type=stype,
                    east=e,
                    north=n,
                    mast_agl_m=_clampf(s.get("mastAglM"), 1.0, 120.0, d["mastAglM"]),
                    max_range_m=_clampf(s.get("maxRangeM"), 200.0, 40000.0, d["maxRangeM"]),
                    min_elevation_deg=_clampf(s.get("minElevationDeg"), -90.0, 30.0, d["minElevationDeg"]),
                    clutter_floor_agl_m=_clampf(s.get("clutterFloorAglM"), 0.0, 500.0, d["clutterFloorAglM"]),
                    sector_center_deg=_clampf(s.get("sectorCenterDeg"), -360.0, 360.0, 0.0) % 360.0,
                    sector_width_deg=_clampf(s.get("sectorWidthDeg"), 10.0, 360.0, 360.0),
                    locked=bool(s.get("locked", False)),
                    label=str(s.get("label") or sid)[:60],
                )
            )

        assets = []
        for k, a in enumerate(payload.get("assets") or []):
            e, n = to_en(a.get("lon"), a.get("lat"), f"Asset {k + 1}")
            assets.append(
                AssetSpec(
                    id=str(a.get("id") or f"asset-{k + 1}")[:32],
                    east=e,
                    north=n,
                    warning_radius_m=_clampf(a.get("warningRadiusM"), 100.0, 20000.0, 2500.0),
                    priority=_clampf(a.get("priority"), 1.0, 20.0, 4.0),
                    label=str(a.get("label") or f"Asset {k + 1}")[:60],
                )
            )

        routes = []
        for k, r in enumerate(payload.get("routes") or []):
            pts_in = r.get("points") or []
            if len(pts_in) < 2:
                continue
            pts = []
            for j, p in enumerate(pts_in):
                e, n = to_en(p[0], p[1], f"Route {k + 1} point {j + 1}")
                pts.append([e, n, _clampf(p[2] if len(p) > 2 else 60.0, 5.0, 5000.0, 60.0)])
            routes.append(
                RouteSpec(
                    id=str(r.get("id") or f"route-{k + 1}")[:32],
                    points_enu=pts,
                    speed_mps=_clampf(r.get("speedMps"), 10.0, 1200.0, 60.0),
                    label=str(r.get("label") or f"Route {k + 1}")[:60],
                )
            )

        layers_in = payload.get("layers") or DEFAULT_LAYERS
        layers = sorted(
            {round(_clampf(l.get("aglM"), 5.0, 5000.0, 60.0), 1): LayerSpec(round(_clampf(l.get("aglM"), 5.0, 5000.0, 60.0), 1), _clampf(l.get("weight"), 0.0, 10.0, 1.0)) for l in layers_in}.values(),
            key=lambda l: l.agl_m,
        )
        if not layers or sum(l.weight for l in layers) <= 0:
            raise ValueError("Select at least one threat altitude layer with positive weight.")

        threat_in = payload.get("threat") or {}
        weather_in = payload.get("weather") or {}
        era5_wind, era5_src = self._era5_surface_wind(preset, env)
        if weather_in.get("useEra5Wind", True):
            wind, wind_src = era5_wind, era5_src
        else:
            wind, wind_src = _clampf(weather_in.get("surfaceWindMps"), 0.0, 40.0, 0.0), "operator override"
        opt_in = payload.get("optimizer") or {}
        extra_type = str(opt_in.get("extraSensorType", "radar"))
        if extra_type not in SENSOR_TYPES:
            raise ValueError(f"Unsupported extra sensor type: {extra_type}")
        spacing = opt_in.get("candidateSpacingM")

        jammers = []
        for k, j in enumerate(payload.get("jammers") or []):
            if not isinstance(j, dict):
                continue
            je, jn = to_en(j.get("lon"), j.get("lat"), f"Jammer {k + 1}")
            jammers.append(
                JammerSpec(
                    id=str(j.get("id") or f"jammer-{k + 1}")[:32],
                    east=je,
                    north=jn,
                    agl_m=_clampf(j.get("aglM"), 10.0, 6000.0, 150.0),
                    erp_ratio=_clampf(j.get("erpRatio"), 0.5, 200.0, 14.0),
                    beam_width_deg=_clampf(j.get("beamWidthDeg"), 8.0, 180.0, 34.0),
                    label=str(j.get("label") or f"EW Jammer {k + 1}")[:60],
                )
            )

        scenario = CoverageScenario(
            scenario_id=f"{preset.id}-coverage",
            title=str(payload.get("title") or f"{preset.title} — Sensor Coverage")[:120],
            sensors=sensors,
            assets=assets,
            routes=routes,
            layers=list(layers),
            threat=ThreatSpec(
                rcs_m2=_clampf(threat_in.get("rcsM2"), 0.001, 50.0, 0.1),
                label=str(threat_in.get("label") or "Threat")[:60],
            ),
            weather=WeatherSpec(
                visibility_km=_clampf(weather_in.get("visibilityKm"), 0.2, 50.0, 10.0),
                surface_wind_mps=wind,
                wind_source=wind_src,
            ),
            optimizer=OptimizerSpec(
                enabled=bool(opt_in.get("enabled", True)),
                extra_sensors=int(_clampf(opt_in.get("extraSensors"), 0, 6, 0)),
                extra_sensor_type=extra_type,
                min_separation_m=_clampf(opt_in.get("minSeparationM"), 0.0, 10000.0, 600.0),
                allow_rooftops=bool(opt_in.get("allowRooftops", True)),
                candidate_spacing_m=_clampf(spacing, 100.0, 5000.0, 400.0) if spacing else None,
                redundancy_weight=_clampf(opt_in.get("redundancyWeight"), 0.0, 1.0, 0.15),
                route_weight=_clampf(opt_in.get("routeWeight"), 1.0, 10.0, 2.5),
                swap_passes=int(_clampf(opt_in.get("swapPasses"), 0, 5, 2)),
                optimize_orientation=bool(opt_in.get("optimizeOrientation", True)),
            ),
            include_buildings=bool(payload.get("includeBuildings", True)),
            analysis_cell_m=_clampf(payload.get("analysisCellM"), 20.0, 500.0, 60.0) if payload.get("analysisCellM") else None,
            jammers=jammers,
        )
        return preset, env, obs, scenario

    # -- map overlay -----------------------------------------------------
    def _result_overlay(self, env: EnvironmentModel, replay: dict[str, Any], map_layer_agl: float) -> dict[str, Any]:
        features: list[dict[str, Any]] = []
        floors = replay["_floors"]
        grid = floors["grid"]

        def to_ll(coords: np.ndarray) -> np.ndarray:
            lon, lat = env.enu_to_lonlat_fast(coords[:, 0], coords[:, 1])
            return np.column_stack([lon, lat])

        for cfg in ("baseline", "optimized"):
            f = floors.get(cfg)
            data = replay["configs"].get(cfg)
            if f is None or data is None:
                continue
            blind = coverage_counts(f, map_layer_agl) == 0
            for poly in blind_polygons_enu(grid, blind)[:60]:
                ll = shapely.transform(poly, to_ll)
                gj = shapely.geometry.mapping(ll)
                features.append(
                    {
                        "type": "Feature",
                        "geometry": {"type": gj["type"], "coordinates": json.loads(json.dumps(gj["coordinates"]))},
                        "properties": {"simKind": f"cov-blind-{cfg}", "areaKm2": round(poly.area / 1e6, 3), "aglM": map_layer_agl},
                    }
                )
            for s in data["sensors"]:
                features.append(
                    {
                        "type": "Feature",
                        "geometry": {"type": "Point", "coordinates": [s["lon"], s["lat"]]},
                        "properties": {"simKind": f"cov-result-sensor-{cfg}", "label": s["label"], "sensorType": s["type"]},
                    }
                )
            for r in data["metrics"]["routes"]:
                if r.get("firstDetectionEnu"):
                    lon, lat = env.enu_to_lonlat_fast(np.asarray([r["firstDetectionEnu"][0]]), np.asarray([r["firstDetectionEnu"][1]]))
                    features.append(
                        {
                            "type": "Feature",
                            "geometry": {"type": "Point", "coordinates": [round(float(lon[0]), 6), round(float(lat[0]), 6)]},
                            "properties": {"simKind": f"cov-detect-{cfg}", "label": f"{r['label']}: {r['warningTimeS']} s warning"},
                        }
                    )
        base = replay["configs"]["baseline"]["sensors"]
        opt = (replay["configs"].get("optimized") or {}).get("sensors") or []
        by_id = {s["id"]: s for s in base}
        for s in opt:
            b = by_id.get(s["id"])
            if b and (abs(b["lon"] - s["lon"]) > 1e-6 or abs(b["lat"] - s["lat"]) > 1e-6):
                features.append(
                    {
                        "type": "Feature",
                        "geometry": {"type": "LineString", "coordinates": [[b["lon"], b["lat"]], [s["lon"], s["lat"]]]},
                        "properties": {"simKind": "cov-move"},
                    }
                )
        return {"type": "FeatureCollection", "features": features}

    # -- run execution ---------------------------------------------------
    def start_run(self, payload: dict[str, Any], *, sync: bool = False) -> dict[str, Any]:
        preset, env, obs, scenario = self.build_scenario(payload)
        requested = payload.get("jobId")
        if requested is not None:
            job_id = str(requested)
            if not JOB_ID_RE.fullmatch(job_id):
                raise ValueError("Invalid jobId format")
            with self._jobs_lock:
                if job_id in self._jobs:
                    raise ValueError(f"Duplicate jobId: {job_id}")
        else:
            job_id = "covjob_" + secrets.token_hex(6)
        map_layer = _clampf(payload.get("mapLayerAglM"), 5.0, 5000.0, scenario.layers[0].agl_m)
        now_iso = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")
        run_dir = self.runs_root / job_id
        run_dir.mkdir(parents=True, exist_ok=True)
        record: dict[str, Any] = {
            "jobId": job_id,
            "environmentId": preset.id,
            "environmentTitle": preset.title,
            "title": scenario.title,
            "status": "running",
            "stage": "1/5 Loading verified environment & obstruction raster…",
            "createdAt": now_iso,
            "completedAt": None,
            "viewerUrl": self._viewer_url(job_id),
            "hasGlb": bool(preset.glb_path and preset.glb_path.is_file()),
            "hasImagery": bool(preset.imagery_path and preset.imagery_path.is_file()),
            "sensorCount": len(scenario.sensors),
            "mapLayerAglM": map_layer,
            "error": None,
            "summary": None,
            "resultOverlay": None,
        }
        cancel_ev = threading.Event()
        with self._jobs_lock:
            self._jobs[job_id] = record
            self._cancel_flags[job_id] = cancel_ev
        (run_dir / "job.json").write_text(json.dumps(record, indent=2), encoding="utf-8")
        draft = {k: v for k, v in payload.items() if k not in ("_sync", "jobId")}
        draft["environmentId"] = preset.id

        def set_stage(stage: str) -> None:
            if cancel_ev.is_set():
                raise RuntimeError("Job cancelled")
            with self._jobs_lock:
                self._jobs[job_id]["stage"] = stage

        def execute() -> None:
            try:
                with self._run_lock:
                    replay = run_coverage(
                        scenario, env=env, obs=obs, cache=self._floor_cache, cache_scope=(preset.id,), on_stage=set_stage
                    )
                overlay = self._result_overlay(env, replay, map_layer)
                replay.pop("_floors", None)
                replay["sourceDraft"] = draft
                replay["jobId"] = job_id
                replay["environmentId"] = preset.id
                (run_dir / "replay.json").write_text(json.dumps(replay, separators=(",", ":")), encoding="utf-8")

                def pick(cfg: str) -> dict[str, Any] | None:
                    c = replay["configs"].get(cfg)
                    if not c:
                        return None
                    m = c["metrics"]
                    layer = min(m["layers"], key=lambda l: abs(l["aglM"] - map_layer))
                    return {
                        "objective": m["objective"],
                        "aglM": layer["aglM"],
                        "coverageFraction": layer["coverageFraction"],
                        "weightedCoverageFraction": layer["weightedCoverageFraction"],
                        "blindAreaKm2": layer["blindAreaKm2"],
                        "blindCauses": m.get("blindCauses"),
                        "priorityZoneCoverageFraction": layer["priorityZoneCoverageFraction"],
                        "minWarningTimeS": m["minWarningTimeS"],
                        "routes": [
                            {"routeId": r["routeId"], "label": r["label"], "warningTimeS": r["warningTimeS"], "trackedFraction": r["trackedFraction"]}
                            for r in m["routes"]
                        ],
                    }

                summary = {
                    "layers": replay["layers"],
                    "baseline": pick("baseline"),
                    "optimized": pick("optimized"),
                    "comparison": replay["comparison"],
                    "optimizer": {
                        k: replay["optimizer"].get(k) for k in ("enabled", "evaluations", "elapsedS", "note")
                    },
                    "optimizedSensors": [
                        {"id": s["id"], "type": s["type"], "label": s["label"], "lon": s["lon"], "lat": s["lat"], "origin": s["origin"], "siteOnRoof": s["siteOnRoof"]}
                        for s in ((replay["configs"].get("optimized") or {}).get("sensors") or [])
                    ],
                    "elapsedS": replay["elapsedS"],
                }
                done = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")
                with self._jobs_lock:
                    self._jobs[job_id].update(
                        status="completed", stage="Completed", completedAt=done, summary=summary, resultOverlay=overlay
                    )
                    snap = dict(self._jobs[job_id])
                (run_dir / "job.json").write_text(json.dumps(snap, indent=2), encoding="utf-8")
            except Exception as exc:  # pragma: no cover - surfaced to UI
                with self._jobs_lock:
                    if cancel_ev.is_set():
                        self._jobs[job_id].update(status="cancelled", stage="Cancelled by operator", error=None)
                    else:
                        self._jobs[job_id].update(status="failed", stage="Failed", error=str(exc))
                    snap = dict(self._jobs[job_id])
                (run_dir / "job.json").write_text(json.dumps(snap, indent=2), encoding="utf-8")
            finally:
                with self._jobs_lock:
                    self._cancel_flags.pop(job_id, None)

        if sync:
            execute()
        elif self._compute_executor is not None:
            self._compute_executor.submit(execute)
        else:
            threading.Thread(target=execute, name=f"cov-job-{job_id}", daemon=True).start()
        return self.get_run(job_id) or record

    # -- Real-time interactive 3D workbench queries (~5-15 ms!) -----------
    def los_profile(self, payload: dict[str, Any]) -> dict[str, Any]:
        """Compute exact 8m terrain+building LOS rays & elevation profiles from sensors to a clicked 3D cell."""
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        _, env, obs = self._get_env(env_id)

        target_east = float(payload["targetEast"])
        target_north = float(payload["targetNorth"])
        target_agl = _clampf(payload.get("targetAglM"), 1.0, 5000.0, 30.0)
        include_buildings = bool(payload.get("includeBuildings", True))
        threat = ThreatSpec(rcs_m2=_clampf(payload.get("rcsM2"), 0.001, 50.0, 0.1))
        weather = WeatherSpec(
            visibility_km=_clampf(payload.get("visibilityKm"), 0.2, 50.0, 10.0),
            surface_wind_mps=_clampf(payload.get("surfaceWindMps"), 0.0, 40.0, 0.0),
        )

        sensors_in = payload.get("sensors") or []
        sensors: list[SensorSpec] = []
        for k, s in enumerate(sensors_in[:20]):
            stype = str(s.get("type", "radar"))
            if stype not in SENSOR_TYPES:
                continue
            d = SENSOR_DEFAULTS[stype]
            enu = s.get("enu")
            if isinstance(enu, list) and len(enu) >= 2:
                se, sn = float(enu[0]), float(enu[1])
            else:
                se, sn, _ = env.geodetic_to_enu(float(s["lon"]), float(s["lat"]), env.up_median)
            sensors.append(
                SensorSpec(
                    id=str(s.get("id") or f"{stype}-{k + 1}"),
                    type=stype,
                    east=se,
                    north=sn,
                    mast_agl_m=_clampf(s.get("mastAglM"), 1.0, 120.0, d["mastAglM"]),
                    max_range_m=_clampf(s.get("maxRangeM"), 200.0, 40000.0, d["maxRangeM"]),
                    min_elevation_deg=_clampf(s.get("minElevationDeg"), -90.0, 30.0, d["minElevationDeg"]),
                    clutter_floor_agl_m=_clampf(s.get("clutterFloorAglM"), 0.0, 500.0, d["clutterFloorAglM"]),
                    sector_center_deg=_clampf(s.get("sectorCenterDeg"), -360.0, 360.0, 0.0) % 360.0,
                    sector_width_deg=_clampf(s.get("sectorWidthDeg"), 10.0, 360.0, 360.0),
                    label=str(s.get("label") or s.get("id") or f"{stype}-{k + 1}"),
                )
            )
        return los_cross_section(
            obs=obs,
            sensors=sensors,
            target_east=target_east,
            target_north=target_north,
            target_agl_m=target_agl,
            threat=threat,
            weather=weather,
            include_buildings=include_buildings,
            jammers=self._parse_inline_jammers(env, payload.get("jammers")),
        )

    def preview_sensor(self, payload: dict[str, Any]) -> dict[str, Any]:
        """Compute one candidate sensor's exact viewshed floor on the current grid in ~10-15 ms."""
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        _, env, obs = self._get_env(env_id)

        stype = str(payload.get("type", "radar"))
        if stype not in SENSOR_TYPES:
            raise ValueError(f"Unsupported sensor type: {stype}")
        d = SENSOR_DEFAULTS[stype]
        east = float(payload["east"])
        north = float(payload["north"])
        if not (env.east_min - 50.0 <= east <= env.east_max + 50.0 and env.north_min - 50.0 <= north <= env.north_max + 50.0):
            raise ValueError("Candidate site lies outside the verified environment bounds.")

        grid_cell_m = _clampf(payload.get("gridCellM"), 20.0, 500.0, 80.0) if payload.get("gridCellM") else None
        grid = AnalysisGrid.build(obs, cell_m=grid_cell_m)
        include_buildings = bool(payload.get("includeBuildings", True))
        threat = ThreatSpec(rcs_m2=_clampf(payload.get("rcsM2"), 0.001, 50.0, 0.1))
        weather = WeatherSpec(
            visibility_km=_clampf(payload.get("visibilityKm"), 0.2, 50.0, 10.0),
            surface_wind_mps=_clampf(payload.get("surfaceWindMps"), 0.0, 40.0, 0.0),
        )
        sensor = SensorSpec(
            id=str(payload.get("id") or f"preview-{stype}"),
            type=stype,
            east=east,
            north=north,
            mast_agl_m=_clampf(payload.get("mastAglM"), 1.0, 120.0, d["mastAglM"]),
            max_range_m=_clampf(payload.get("maxRangeM"), 200.0, 40000.0, d["maxRangeM"]),
            min_elevation_deg=_clampf(payload.get("minElevationDeg"), -90.0, 30.0, d["minElevationDeg"]),
            clutter_floor_agl_m=_clampf(payload.get("clutterFloorAglM"), 0.0, 500.0, d["clutterFloorAglM"]),
            sector_center_deg=_clampf(payload.get("sectorCenterDeg"), -360.0, 360.0, 0.0) % 360.0,
            sector_width_deg=_clampf(payload.get("sectorWidthDeg"), 10.0, 360.0, 360.0),
            label=str(payload.get("label") or f"Test {stype.replace('_', '/').upper()}"),
            origin="preview",
        )
        rng = effective_range_m(sensor, threat, weather)
        floor = floor_agl_map(obs, grid, sensor, rng, include_buildings=include_buildings)
        site_up, ant_up = sensor_site_up(obs, sensor)
        lon, lat, _ = env.enu_to_geodetic(east, north, ant_up)
        return {
            "id": sensor.id,
            "type": sensor.type,
            "label": sensor.label,
            "origin": "preview",
            "locked": False,
            "enu": [round(east, 2), round(north, 2), round(ant_up, 2)],
            "siteUpM": round(site_up, 2),
            "siteOnRoof": bool(site_up - obs.height_at(east, north, buildings=False) > 2.0),
            "lon": round(lon, 6),
            "lat": round(lat, 6),
            "mastAglM": sensor.mast_agl_m,
            "maxRangeM": sensor.max_range_m,
            "effectiveRangeM": round(rng, 1),
            "minElevationDeg": sensor.min_elevation_deg,
            "clutterFloorAglM": sensor.clutter_floor_agl_m,
            "sectorCenterDeg": sensor.sector_center_deg,
            "sectorWidthDeg": sensor.sector_width_deg,
            "requiresLos": sensor.requires_los,
            "gridNx": grid.nx,
            "gridNy": grid.ny,
            "floorB64": base64.b64encode(np.ascontiguousarray(encode_floor_u16(floor)).tobytes()).decode("ascii"),
        }

    def _parse_inline_jammers(self, env: EnvironmentModel, raw_list: Any) -> list[JammerSpec]:
        jammers: list[JammerSpec] = []
        for k, j in enumerate((raw_list or [])[:8]):
            if not isinstance(j, dict):
                continue
            enu = j.get("enu")
            if isinstance(enu, list) and len(enu) >= 2:
                je, jn = float(enu[0]), float(enu[1])
            elif j.get("lon") is not None and j.get("lat") is not None:
                je, jn, _ = env.geodetic_to_enu(float(j["lon"]), float(j["lat"]), env.up_median)
            else:
                continue
            jammers.append(
                JammerSpec(
                    id=str(j.get("id") or f"jammer-{k + 1}"),
                    east=je,
                    north=jn,
                    agl_m=_clampf(j.get("aglM"), 10.0, 6000.0, 150.0),
                    erp_ratio=_clampf(j.get("erpRatio"), 0.5, 200.0, 14.0),
                    beam_width_deg=_clampf(j.get("beamWidthDeg"), 8.0, 180.0, 34.0),
                    label=str(j.get("label") or f"EW Jammer {k + 1}"),
                )
            )
        return jammers

    def adversarial_route(self, payload: dict[str, Any]) -> dict[str, Any]:
        """Find the worst-case least-detection A* stealth corridor to a defended asset (~15-45 ms)."""
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        _, env, obs = self._get_env(env_id)

        grid_cell_m = _clampf(payload.get("gridCellM"), 20.0, 500.0, 80.0) if payload.get("gridCellM") else None
        grid = AnalysisGrid.build(obs, cell_m=grid_cell_m)
        include_buildings = bool(payload.get("includeBuildings", True))
        agl_m = _clampf(payload.get("aglM"), 10.0, 2000.0, 35.0)
        speed_mps = _clampf(payload.get("speedMps"), 10.0, 800.0, 65.0)
        start_edge = str(payload.get("startEdge") or "any").lower()
        if start_edge not in ("any", "north", "south", "east", "west"):
            start_edge = "any"

        threat = ThreatSpec(rcs_m2=_clampf(payload.get("rcsM2"), 0.001, 50.0, 0.1))
        weather = WeatherSpec(
            visibility_km=_clampf(payload.get("visibilityKm"), 0.2, 50.0, 10.0),
            surface_wind_mps=_clampf(payload.get("surfaceWindMps"), 0.0, 40.0, 0.0),
        )
        jammers = self._parse_inline_jammers(env, payload.get("jammers"))

        asset_enu = payload.get("assetEnu")
        if isinstance(asset_enu, list) and len(asset_enu) >= 2:
            ae, an = float(asset_enu[0]), float(asset_enu[1])
        elif payload.get("assetLon") is not None and payload.get("assetLat") is not None:
            ae, an, _ = env.geodetic_to_enu(float(payload["assetLon"]), float(payload["assetLat"]), env.up_median)
        else:
            ae = 0.5 * (env.east_min + env.east_max)
            an = 0.5 * (env.north_min + env.north_max)

        sensors_in = payload.get("sensors") or []
        floors_list = []
        sensor_ids = []
        for k, s in enumerate(sensors_in[:20]):
            stype = str(s.get("type", "radar"))
            if stype not in SENSOR_TYPES:
                continue
            d = SENSOR_DEFAULTS[stype]
            enu = s.get("enu")
            if isinstance(enu, list) and len(enu) >= 2:
                se, sn = float(enu[0]), float(enu[1])
            else:
                se, sn, _ = env.geodetic_to_enu(float(s["lon"]), float(s["lat"]), env.up_median)
            spec = SensorSpec(
                id=str(s.get("id") or f"{stype}-{k + 1}"),
                type=stype,
                east=se,
                north=sn,
                mast_agl_m=_clampf(s.get("mastAglM"), 1.0, 120.0, d["mastAglM"]),
                max_range_m=_clampf(s.get("maxRangeM"), 200.0, 40000.0, d["maxRangeM"]),
                min_elevation_deg=_clampf(s.get("minElevationDeg"), -90.0, 30.0, d["minElevationDeg"]),
                clutter_floor_agl_m=_clampf(s.get("clutterFloorAglM"), 0.0, 500.0, d["clutterFloorAglM"]),
                sector_center_deg=_clampf(s.get("sectorCenterDeg"), -360.0, 360.0, 0.0) % 360.0,
                sector_width_deg=_clampf(s.get("sectorWidthDeg"), 10.0, 360.0, 360.0),
                label=str(s.get("label") or s.get("id") or f"{stype}-{k + 1}"),
            )
            rng = effective_range_m(spec, threat, weather)
            floors_list.append(
                floor_agl_map(obs, grid, spec, rng, include_buildings=include_buildings, jammers=jammers)
            )
            sensor_ids.append(spec.id)

        floors = np.stack(floors_list) if floors_list else np.zeros((0, grid.count), dtype=np.float32)
        return find_adversarial_route(
            obs=obs,
            grid=grid,
            floors=floors,
            sensor_ids=sensor_ids,
            asset_east=ae,
            asset_north=an,
            agl_m=agl_m,
            speed_mps=speed_mps,
            start_edge=start_edge,
        )

    # -- Cross-simulation sensor-network handoff --------------------------
    def export_network(self, payload: dict[str, Any]) -> dict[str, Any]:
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        _, env, _ = self._get_env(env_id)
        sensors = payload.get("sensors") or []
        job_id = payload.get("jobId")
        if not sensors and job_id:
            assets = self.run_assets(str(job_id))
            if assets:
                rp = assets[0] / "replay.json"
                if rp.is_file():
                    rep = json.loads(rp.read_text(encoding="utf-8"))
                    cfg = rep["configs"].get("optimized") or rep["configs"].get("baseline")
                    sensors = cfg["sensors"] if cfg else []
        if not sensors:
            raise ValueError("Provide sensors or a completed jobId to export a sensor network.")
        return self._network_store.save(
            name=str(payload.get("name") or f"{env_id} optimized sensor network"),
            environment_id=env_id,
            manifest_sha256=env.manifest_sha256,
            sensors=sensors,
            source_job_id=str(job_id) if job_id else None,
            metrics=payload.get("metrics"),
        )

    def list_networks(self, environment_id: str | None = None) -> list[dict[str, Any]]:
        return self._network_store.list(environment_id=environment_id)
