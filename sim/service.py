"""Stateful local simulation service bridging EMAD's 2D Map UI and 3D Replay Viewer.

Provides:
- Automatic discovery of verified prepared EMAD environments (Riyadh observation
  pilot, owned fixture, etc.) with their companion terrain GLB and Sentinel-2
  optical imagery display files.
- In-memory caching of read-only ``EnvironmentModel`` instances so map-triggered
  simulations complete in ~1-2 seconds.
- Asynchronous run execution with live stage status, persisted run artifacts
  under ``tmp/sim-runs/<jobId>/``, and WGS84 GeoJSON result footprints for 2D
  map overlay alongside the dedicated 3D Three.js replay tab.
"""
from __future__ import annotations

from dataclasses import dataclass
import datetime as dt
import json
import math
from pathlib import Path
import re
import secrets
import threading
from typing import Any

from .atmosphere import AtmosphereModel
from .contracts import (
    AtmosphereOverrideConfig,
    FIDELITY_KINEMATIC_3DOF,
    InterceptorSiteConfig,
    RadarConfig,
    SIM_ENGINE_VERSION,
    ScenarioSpec,
    ShrapnelConfig,
    ThreatConfig,
)
from .engine import compare_policies, run_simulation, save_run_bundle
from .environment import EnvironmentModel

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_RUNS_ROOT = REPO_ROOT / "tmp" / "sim-runs"


@dataclass(frozen=True)
class EnvironmentPreset:
    id: str
    title: str
    description: str
    package_path: Path
    expected_manifest_sha256: str | None
    weather_field_path: Path | None
    glb_path: Path | None
    imagery_path: Path | None
    scenario_template_path: Path


def discover_presets(repo_root: Path = REPO_ROOT) -> list[EnvironmentPreset]:
    """Discover all available verified prepared environments on disk."""
    candidates = [
        EnvironmentPreset(
            id="riyadh-observation-v1",
            title="Riyadh Reference Area (GLO-30 + OSM + ERA5)",
            description=(
                "Staged Riyadh observation package: Copernicus GLO-30 DSM, EGM2008 geoid, "
                "1,978 OSM buildings, 3,703 roads, Sentinel-2 optical drape, and embedded ERA5 winds."
            ),
            package_path=repo_root
            / "private/environment-foundation/implementation/riyadh-observation-pilot/staged-prepared.zip",
            expected_manifest_sha256="175515f785c2ae6cdbbec0303140f6d425ff8e773dc24375a64d02d87ac63ab4",
            weather_field_path=None,
            glb_path=repo_root
            / "private/environment-foundation/implementation/riyadh-observation-pilot/staged-surface.glb",
            imagery_path=repo_root
            / "private/environment-foundation/implementation/riyadh-observation-pilot/staged-imagery.json",
            scenario_template_path=repo_root / "sim/scenarios/riyadh_intercept.json",
        ),
        EnvironmentPreset(
            id="owned-fixture-v1",
            title="Owned Fixture Verification Environment",
            description=(
                "Git-committed owned verification environment with multi-component surface rasters, "
                "sample buildings/roads, and companion ERA5 pressure-level wind field."
            ),
            package_path=repo_root / "testdata/geospatial/owned-fixture-v1/prepared.zip",
            expected_manifest_sha256="aaf9f14b027749b5d233018606fb90c81e34808b9522e034fc67b5243c966bdc",
            weather_field_path=repo_root / "testdata/geospatial/owned-fixture-v1/field-era5.json",
            glb_path=None,
            imagery_path=None,
            scenario_template_path=repo_root / "sim/scenarios/fixture_intercept.json",
        ),
    ]
    return [c for c in candidates if c.package_path.is_file() and c.scenario_template_path.is_file()]


def _ellipse_ring_wgs84(
    env: EnvironmentModel,
    center_e: float,
    center_n: float,
    semi_major_m: float,
    semi_minor_m: float,
    angle_deg: float,
    n_pts: int = 48,
) -> list[list[float]]:
    ang = math.radians(angle_deg)
    ca, sa = math.cos(ang), math.sin(ang)
    ring: list[list[float]] = []
    for i in range(n_pts + 1):
        th = (i / n_pts) * 2.0 * math.pi
        lx = semi_major_m * math.cos(th)
        ly = semi_minor_m * math.sin(th)
        e = center_e + lx * ca - ly * sa
        n = center_n + lx * sa + ly * ca
        lon, lat, _ = env.enu_to_geodetic(e, n, env.up_median)
        ring.append([round(lon, 6), round(lat, 6)])
    return ring


def _build_result_geojson(env: EnvironmentModel, runs: list[dict[str, Any]]) -> dict[str, Any]:
    """Convert run intercepts and 90% shrapnel dispersion ellipses to WGS84 GeoJSON for 2D map display."""
    features: list[dict[str, Any]] = []
    for run in runs:
        ev = run["evaluation"]
        policy = ev["policy"]
        intr = ev["interception"]
        fp = ev.get("shrapnelFootprint")
        exp = ev.get("shrapnelExposure")
        if intr.get("intercepted") and intr.get("interceptEnu"):
            ie, in_, iu = intr["interceptEnu"]
            lon, lat, _ = env.enu_to_geodetic(ie, in_, iu)
            features.append(
                {
                    "type": "Feature",
                    "geometry": {"type": "Point", "coordinates": [round(lon, 6), round(lat, 6)]},
                    "properties": {
                        "simKind": f"result-intercept-{policy}",
                        "policy": policy,
                        "label": f"{policy} intercept (t={intr['interceptTimeS']}s)",
                    },
                }
            )
        if fp:
            ce, cn, _ = fp["centroidEnu"]
            ring = _ellipse_ring_wgs84(
                env,
                ce,
                cn,
                float(fp["ellipse90SemiMajorM"]),
                float(fp["ellipse90SemiMinorM"]),
                float(fp["ellipseAngleDeg"]),
            )
            features.append(
                {
                    "type": "Feature",
                    "geometry": {"type": "Polygon", "coordinates": [ring]},
                    "properties": {
                        "simKind": f"result-ellipse-{policy}",
                        "policy": policy,
                        "buildingHits": exp.get("buildingHits") if exp else None,
                        "roadHits": exp.get("roadHits") if exp else None,
                        "urbanExposureFraction": exp.get("urbanExposureFraction") if exp else None,
                    },
                }
            )
    return {"type": "FeatureCollection", "features": features}


class SimulationService:
    """In-process simulation manager with environment caching and async job tracking."""

    def __init__(
        self,
        *,
        repo_root: Path = REPO_ROOT,
        runs_root: Path = DEFAULT_RUNS_ROOT,
        public_origin: str = "http://127.0.0.1:8765",
    ) -> None:
        self.repo_root = Path(repo_root).resolve()
        self.runs_root = Path(runs_root).resolve()
        self.runs_root.mkdir(parents=True, exist_ok=True)
        self.public_origin = public_origin.rstrip("/")

        self._presets: dict[str, EnvironmentPreset] = {p.id: p for p in discover_presets(self.repo_root)}
        self._env_cache: dict[str, EnvironmentModel] = {}
        self._env_lock = threading.Lock()
        self._jobs_lock = threading.Lock()
        self._jobs: dict[str, dict[str, Any]] = {}
        self._load_existing_runs()

    def _load_existing_runs(self) -> None:
        """Reload persisted run metadata from disk so history survives server restarts."""
        if not self.runs_root.is_dir():
            return
        for meta_path in sorted(self.runs_root.glob("*/job.json")):
            try:
                rec = json.loads(meta_path.read_text(encoding="utf-8"))
                job_id = str(rec["jobId"])
                if rec.get("status") == "running":
                    rec["status"] = "interrupted"
                    rec["stage"] = "Server restarted before run completed"
                rec["viewerUrl"] = f"{self.public_origin}/?run={job_id}"
                self._jobs[job_id] = rec
            except Exception:
                continue

    def _get_env(self, env_id: str) -> tuple[EnvironmentPreset, EnvironmentModel]:
        preset = self._presets.get(env_id)
        if preset is None:
            raise KeyError(f"Unknown environmentId: {env_id}")
        with self._env_lock:
            model = self._env_cache.get(env_id)
            if model is None:
                tpl = json.loads(preset.scenario_template_path.read_text(encoding="utf-8"))
                model = EnvironmentModel(
                    preset.package_path,
                    expected_manifest_sha256=preset.expected_manifest_sha256,
                    derived_building_height_m=float(tpl.get("derivedBuildingHeightM", 10.0)),
                    road_buffer_m=float(tpl.get("roadExposureBufferM", 12.0)),
                )
                self._env_cache[env_id] = model
        return preset, model

    def _preset_summary(self, preset: EnvironmentPreset) -> dict[str, Any]:
        _, env = self._get_env(preset.id)
        tpl = json.loads(preset.scenario_template_path.read_text(encoding="utf-8"))
        spec = ScenarioSpec.from_dict(tpl)
        atm = AtmosphereModel.load(env, external_path=preset.weather_field_path, override=spec.atmosphere)

        # Exact WGS84 bounding box from ENU bounds
        corners_enu = [
            (env.east_min, env.north_min),
            (env.east_max, env.north_min),
            (env.east_max, env.north_max),
            (env.east_min, env.north_max),
        ]
        lons, lats = [], []
        for ce, cn in corners_enu:
            lon, lat, _ = env.enu_to_geodetic(ce, cn, env.up_median)
            lons.append(lon)
            lats.append(lat)
        bounds_wgs84 = [round(min(lons), 6), round(min(lats), 6), round(max(lons), 6), round(max(lats), 6)]

        def enu_pt_to_wgs84(pt: list[float]) -> list[float]:
            lon, lat, _ = env.enu_to_geodetic(float(pt[0]), float(pt[1]), env.up_median)
            return [round(lon, 6), round(lat, 6), round(float(pt[2]), 1)]

        prot_center = spec.protected_zone_center_enu or [spec.threat.waypoints_enu[-1][0], spec.threat.waypoints_enu[-1][1]]
        prot_lon, prot_lat, _ = env.enu_to_geodetic(float(prot_center[0]), float(prot_center[1]), env.up_median)
        rad_lon, rad_lat, rad_agl = enu_pt_to_wgs84(spec.radar.position_enu)
        lau_lon, lau_lat, lau_agl = enu_pt_to_wgs84(spec.interceptor_site.position_enu)

        return {
            "id": preset.id,
            "title": preset.title,
            "description": preset.description,
            "manifestSha256": env.manifest_sha256,
            "profileId": env.manifest.get("profileId"),
            "buildingCount": len(env.buildings),
            "roadCount": len(env.roads),
            "hasGlb": bool(preset.glb_path and preset.glb_path.is_file()),
            "hasImagery": bool(preset.imagery_path and preset.imagery_path.is_file()),
            "boundsWgs84": bounds_wgs84,
            "origin": list(env.origin),
            "weather": {
                "edition": atm.document.get("edition"),
                "validTime": atm.valid_time,
                "profileAtOrigin": atm.profile_summary(0.0, 0.0),
            },
            "defaults": {
                "title": spec.title,
                "mode": "compare",
                "seed": spec.seed,
                "durationS": spec.duration_s,
                "radar": {
                    "lon": rad_lon,
                    "lat": rad_lat,
                    "heightAglM": rad_agl,
                    "maxRangeM": spec.radar.max_range_m,
                    "includeBuildingsInLos": spec.radar.include_buildings_in_los,
                },
                "launcher": {
                    "lon": lau_lon,
                    "lat": lau_lat,
                    "heightAglM": lau_agl,
                    "speedMps": spec.interceptor_site.speed_mps,
                    "maxRangeM": spec.interceptor_site.max_range_m,
                },
                "protectedZone": {
                    "lon": round(prot_lon, 6),
                    "lat": round(prot_lat, 6),
                    "radiusM": spec.protected_zone_radius_m,
                },
                "threat": {
                    "kind": spec.threat.kind,
                    "speedMps": spec.threat.speed_mps,
                    "rcsM2": spec.threat.rcs_m2,
                    "ballisticApogeeM": spec.threat.ballistic_apogee_m,
                    "waypoints": [enu_pt_to_wgs84(wp) for wp in spec.threat.waypoints_enu],
                },
                "shrapnel": {
                    "fragmentCount": spec.shrapnel.fragment_count,
                },
                "atmosphere": {
                    "windScale": spec.atmosphere.wind_scale,
                    "windAddEnuMps": list(spec.atmosphere.wind_add_enu_mps),
                },
            },
        }

    def list_environments(self) -> list[dict[str, Any]]:
        return [self._preset_summary(p) for p in self._presets.values()]

    def health(self) -> dict[str, Any]:
        with self._jobs_lock:
            running = sum(1 for j in self._jobs.values() if j.get("status") == "running")
            total = len(self._jobs)
        return {
            "status": "ok",
            "engineVersion": SIM_ENGINE_VERSION,
            "fidelityProfile": FIDELITY_KINEMATIC_3DOF,
            "viewerOrigin": self.public_origin,
            "environmentCount": len(self._presets),
            "runningJobs": running,
            "totalRuns": total,
        }

    def list_runs(self) -> list[dict[str, Any]]:
        with self._jobs_lock:
            items = list(self._jobs.values())
        items.sort(key=lambda x: str(x.get("createdAt", "")), reverse=True)
        return items[:30]

    def get_run(self, job_id: str) -> dict[str, Any] | None:
        with self._jobs_lock:
            rec = self._jobs.get(job_id)
            return dict(rec) if rec is not None else None

    def run_assets(self, job_id: str) -> tuple[Path, Path | None, Path | None] | None:
        rec = self.get_run(job_id)
        if rec is None:
            return None
        run_dir = self.runs_root / job_id
        preset = self._presets.get(str(rec.get("environmentId", "")))
        glb = preset.glb_path if (preset and preset.glb_path and preset.glb_path.is_file()) else None
        imagery = preset.imagery_path if (preset and preset.imagery_path and preset.imagery_path.is_file()) else None
        return run_dir, glb, imagery

    def _build_spec_from_request(self, payload: dict[str, Any]) -> tuple[EnvironmentPreset, EnvironmentModel, ScenarioSpec, str]:
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        preset, env = self._get_env(env_id)
        tpl = json.loads(preset.scenario_template_path.read_text(encoding="utf-8"))
        base = ScenarioSpec.from_dict(tpl)

        def to_enu_xy(lon: float, lat: float, label: str) -> tuple[float, float]:
            e, n, _ = env.geodetic_to_enu(float(lon), float(lat), env.up_median)
            pad = 250.0
            if not (
                env.east_min - pad <= e <= env.east_max + pad
                and env.north_min - pad <= n <= env.north_max + pad
            ):
                raise ValueError(
                    f"{label} ({lon:.5f}, {lat:.5f}) lies outside the verified {preset.id} environment bounds."
                )
            return round(e, 2), round(n, 2)

        radar_in = payload.get("radar") or {}
        launcher_in = payload.get("launcher") or {}
        prot_in = payload.get("protectedZone") or {}
        threat_in = payload.get("threat") or {}
        shrap_in = payload.get("shrapnel") or {}
        atm_in = payload.get("atmosphere") or {}

        if "lon" in radar_in and "lat" in radar_in:
            re, rn = to_enu_xy(radar_in["lon"], radar_in["lat"], "Radar site")
            radar_pos = [re, rn, max(2.0, min(200.0, float(radar_in.get("heightAglM", base.radar.position_enu[2]))))]
        else:
            radar_pos = list(base.radar.position_enu)

        if "lon" in launcher_in and "lat" in launcher_in:
            le, ln = to_enu_xy(launcher_in["lon"], launcher_in["lat"], "Launcher site")
            launcher_pos = [le, ln, max(2.0, min(100.0, float(launcher_in.get("heightAglM", base.interceptor_site.position_enu[2]))))]
        else:
            launcher_pos = list(base.interceptor_site.position_enu)

        if "lon" in prot_in and "lat" in prot_in:
            pe, pn = to_enu_xy(prot_in["lon"], prot_in["lat"], "Protected zone")
            prot_center = [pe, pn]
        else:
            prot_center = list(base.protected_zone_center_enu) if base.protected_zone_center_enu else None
        prot_radius = max(100.0, min(2500.0, float(prot_in.get("radiusM", base.protected_zone_radius_m))))

        raw_wps = threat_in.get("waypoints")
        if raw_wps is not None:
            if not isinstance(raw_wps, list) or len(raw_wps) < 2:
                raise ValueError("Threat flight path requires at least 2 waypoints inside the environment.")
            wps_enu = []
            for idx, wp in enumerate(raw_wps):
                we, wn = to_enu_xy(wp[0], wp[1], f"Threat waypoint {idx + 1}")
                wagl = max(15.0, min(3000.0, float(wp[2]) if len(wp) > 2 else 120.0))
                wps_enu.append([we, wn, round(wagl, 1)])
        else:
            wps_enu = [list(w) for w in base.threat.waypoints_enu]

        speed_mps = max(20.0, min(650.0, float(threat_in.get("speedMps", base.threat.speed_mps))))
        rcs_m2 = max(0.01, min(20.0, float(threat_in.get("rcsM2", base.threat.rcs_m2))))
        ballistic_apogee_m = max(300.0, min(5000.0, float(threat_in.get("ballisticApogeeM", base.threat.ballistic_apogee_m))))
        frag_count = max(24, min(500, int(shrap_in.get("fragmentCount", base.shrapnel.fragment_count))))
        seed = int(payload.get("seed", base.seed))
        mode = str(payload.get("mode", "compare"))
        if mode not in ("compare", "earliest", "safe_window"):
            raise ValueError(f"Unsupported simulation mode: {mode}")

        wind_scale = max(0.0, min(8.0, float(atm_in.get("windScale", base.atmosphere.wind_scale))))
        wind_add_raw = atm_in.get("windAddEnuMps", base.atmosphere.wind_add_enu_mps)
        wind_add = [
            max(-50.0, min(50.0, float(wind_add_raw[0]))),
            max(-50.0, min(50.0, float(wind_add_raw[1]))),
            max(-15.0, min(15.0, float(wind_add_raw[2]) if len(wind_add_raw) > 2 else 0.0)),
        ]

        # Estimate total flight duration automatically so longer custom paths don't get cut off
        total_path_m = sum(
            math.hypot(wps_enu[i + 1][0] - wps_enu[i][0], wps_enu[i + 1][1] - wps_enu[i][1])
            for i in range(len(wps_enu) - 1)
        )
        auto_duration = max(base.duration_s, (total_path_m / speed_mps) + 20.0)
        duration_s = min(300.0, round(float(payload.get("durationS", auto_duration)), 1))

        spec = ScenarioSpec(
            scenario_id=str(payload.get("scenarioId") or f"{preset.id}-map-setup"),
            title=str(payload.get("title") or preset.title),
            description=base.description,
            environment_package=str(preset.package_path),
            expected_manifest_sha256=preset.expected_manifest_sha256,
            weather_field_path=str(preset.weather_field_path) if preset.weather_field_path else None,
            seed=seed,
            duration_s=duration_s,
            dt_s=base.dt_s,
            derived_building_height_m=base.derived_building_height_m,
            road_exposure_buffer_m=base.road_exposure_buffer_m,
            threat=ThreatConfig(
                id=base.threat.id,
                kind=str(threat_in.get("kind", base.threat.kind)),
                waypoints_enu=wps_enu,
                altitude_mode="agl",
                speed_mps=speed_mps,
                rcs_m2=rcs_m2,
                spawn_s=0.0,
                mass_kg=base.threat.mass_kg,
                ballistic_apogee_m=ballistic_apogee_m,
            ),
            radar=RadarConfig(
                id=base.radar.id,
                position_enu=radar_pos,
                max_range_m=max(500.0, min(15000.0, float(radar_in.get("maxRangeM", base.radar.max_range_m)))),
                min_elevation_deg=base.radar.min_elevation_deg,
                max_elevation_deg=base.radar.max_elevation_deg,
                period_s=base.radar.period_s,
                delay_s=base.radar.delay_s,
                range_noise_m=base.radar.range_noise_m,
                azimuth_noise_deg=base.radar.azimuth_noise_deg,
                elevation_noise_deg=base.radar.elevation_noise_deg,
                base_detection_prob=base.radar.base_detection_prob,
                dropout_every=base.radar.dropout_every,
                include_buildings_in_los=bool(
                    radar_in.get("includeBuildingsInLos", base.radar.include_buildings_in_los)
                ),
            ),
            interceptor_site=InterceptorSiteConfig(
                id=base.interceptor_site.id,
                position_enu=launcher_pos,
                speed_mps=max(80.0, min(900.0, float(launcher_in.get("speedMps", base.interceptor_site.speed_mps)))),
                max_accel_mps2=base.interceptor_site.max_accel_mps2,
                min_range_m=base.interceptor_site.min_range_m,
                max_range_m=max(
                    600.0, min(15000.0, float(launcher_in.get("maxRangeM", base.interceptor_site.max_range_m)))
                ),
                launch_delay_s=base.interceptor_site.launch_delay_s,
                proximity_fuze_m=base.interceptor_site.proximity_fuze_m,
                protected_radius_m=prot_radius,
            ),
            shrapnel=ShrapnelConfig(
                fragment_count=frag_count,
                burst_speed_min_mps=base.shrapnel.burst_speed_min_mps,
                burst_speed_max_mps=base.shrapnel.burst_speed_max_mps,
                momentum_retention=base.shrapnel.momentum_retention,
                drag_param_min=base.shrapnel.drag_param_min,
                drag_param_max=base.shrapnel.drag_param_max,
                dt_s=base.shrapnel.dt_s,
                max_flight_s=base.shrapnel.max_flight_s,
            ),
            atmosphere=AtmosphereOverrideConfig(
                valid_time=base.atmosphere.valid_time,
                known_at=base.atmosphere.known_at,
                wind_scale=wind_scale,
                wind_add_enu_mps=wind_add,
                boundary_layer_alpha=base.atmosphere.boundary_layer_alpha,
            ),
            protected_zone_center_enu=prot_center,
            protected_zone_radius_m=prot_radius,
            policy="earliest" if mode == "compare" else mode,
            safe_window_preview_fragments=base.safe_window_preview_fragments,
            safe_window_horizon_s=base.safe_window_horizon_s,
            safe_window_step_s=base.safe_window_step_s,
            safe_window_building_weight=base.safe_window_building_weight,
            safe_window_road_weight=base.safe_window_road_weight,
            safe_window_protected_zone_weight=base.safe_window_protected_zone_weight,
        )
        return preset, env, spec, mode

    def start_run(self, payload: dict[str, Any], *, sync: bool = False) -> dict[str, Any]:
        """Validate setup, register job, and execute asynchronously (or synchronously in tests)."""
        preset, env, spec, mode = self._build_spec_from_request(payload)
        requested_id = payload.get("jobId")
        if requested_id is not None:
            requested_str = str(requested_id)
            if not re.fullmatch(r"simjob_[a-f0-9]{12}", requested_str):
                raise ValueError("Invalid jobId format")
            with self._jobs_lock:
                if requested_str in self._jobs:
                    raise ValueError(f"Duplicate jobId: {requested_str}")
            job_id = requested_str
        else:
            job_id = "simjob_" + secrets.token_hex(6)
        now_iso = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")
        run_dir = self.runs_root / job_id
        run_dir.mkdir(parents=True, exist_ok=True)

        record: dict[str, Any] = {
            "jobId": job_id,
            "environmentId": preset.id,
            "environmentTitle": preset.title,
            "title": spec.title,
            "mode": mode,
            "seed": spec.seed,
            "status": "running",
            "stage": "1/4 Loading ERA5 wind field & threat trajectory…",
            "createdAt": now_iso,
            "completedAt": None,
            "viewerUrl": f"{self.public_origin}/?run={job_id}",
            "hasGlb": bool(preset.glb_path and preset.glb_path.is_file()),
            "hasImagery": bool(preset.imagery_path and preset.imagery_path.is_file()),
            "error": None,
            "comparison": None,
            "summary": None,
            "resultOverlay": None,
        }
        with self._jobs_lock:
            self._jobs[job_id] = record
        (run_dir / "job.json").write_text(json.dumps(record, indent=2), encoding="utf-8")

        def execute() -> None:
            def set_stage(stage: str) -> None:
                with self._jobs_lock:
                    self._jobs[job_id]["stage"] = stage

            try:
                atm = AtmosphereModel.load(env, external_path=preset.weather_field_path, override=spec.atmosphere)
                if mode == "compare":
                    set_stage("2/4 Scanning terrain-masked radar & running Earliest baseline…")
                    run_earliest = run_simulation(spec.with_policy("earliest"), env=env, atm=atm)
                    set_stage("3/4 Optimizing Wind-Aware Safe-Window intercept & shrapnel…")
                    run_safe = run_simulation(spec.with_policy("safe_window"), env=env, atm=atm)
                    runs = [run_earliest, run_safe]
                    # Build comparison summary cleanly using compare_policies shape
                    summary_rows = {}
                    for r in runs:
                        ev = r["evaluation"]
                        exp = ev["shrapnelExposure"] or {}
                        fp = ev["shrapnelFootprint"] or {}
                        summary_rows[ev["policy"]] = {
                            "intercepted": ev["interception"]["intercepted"],
                            "launchTimeS": ev["interception"].get("launchTimeS"),
                            "interceptTimeS": ev["interception"].get("interceptTimeS"),
                            "interceptAglM": ev["interception"].get("interceptAglM"),
                            "protectedMarginM": ev["interception"].get("protectedMarginM"),
                            "buildingHits": exp.get("buildingHits"),
                            "roadHits": exp.get("roadHits"),
                            "protectedZoneHits": exp.get("protectedZoneHits"),
                            "openHits": exp.get("openHits"),
                            "urbanExposureFraction": exp.get("urbanExposureFraction"),
                            "hazardousHits": exp.get("hazardousHits"),
                            "hazardousBuildingHits": exp.get("hazardousBuildingHits"),
                            "maxImpactEnergyJ": exp.get("maxImpactEnergyJ"),
                            "driftDistanceM": fp.get("driftDistanceM"),
                        }
                    delta = {}
                    e_row, s_row = summary_rows["earliest"], summary_rows["safe_window"]
                    if e_row["urbanExposureFraction"] is not None and s_row["urbanExposureFraction"] is not None:
                        e_exp, s_exp = e_row["urbanExposureFraction"], s_row["urbanExposureFraction"]
                        red = (e_exp - s_exp) / e_exp if e_exp > 0 else 0.0
                        delta = {
                            "buildingHitsDelta": (s_row["buildingHits"] or 0) - (e_row["buildingHits"] or 0),
                            "roadHitsDelta": (s_row["roadHits"] or 0) - (e_row["roadHits"] or 0),
                            "urbanExposureDelta": round(s_exp - e_exp, 4),
                            "urbanExposureReductionPct": round(red * 100.0, 1),
                        }
                    comparison = {
                        "scenarioId": spec.scenario_id,
                        "seed": spec.seed,
                        "environmentManifestSha256": env.manifest_sha256,
                        "policies": summary_rows,
                        "safeWindowVsEarliest": delta,
                    }
                else:
                    set_stage(f"2/4 Running {mode} simulation with terrain-masked radar & shrapnel…")
                    run_single = run_simulation(spec.with_policy(mode), env=env, atm=atm)
                    runs = [run_single]
                    comparison = None

                set_stage("4/4 Saving partitioned artifacts & 3D replay bundle…")
                clean_draft = {k: v for k, v in payload.items() if k not in ("_sync", "jobId")}
                clean_draft["environmentId"] = preset.id
                save_run_bundle(runs, run_dir, comparison_summary=comparison, source_draft=clean_draft)
                overlay = _build_result_geojson(env, runs)
                done_iso = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")

                primary_ev = runs[-1]["evaluation"]
                final_fields = {
                    "status": "completed",
                    "stage": "Completed",
                    "completedAt": done_iso,
                    "comparison": comparison,
                    "summary": {
                        "policy": primary_ev["policy"],
                        "intercepted": primary_ev["interception"]["intercepted"],
                        "interceptTimeS": primary_ev["interception"].get("interceptTimeS"),
                        "exposure": primary_ev.get("shrapnelExposure"),
                        "footprint": primary_ev.get("shrapnelFootprint"),
                        "sensing": primary_ev.get("sensing"),
                    },
                    "resultOverlay": overlay,
                }
                with self._jobs_lock:
                    self._jobs[job_id].update(final_fields)
                    snapshot = dict(self._jobs[job_id])
                (run_dir / "job.json").write_text(json.dumps(snapshot, indent=2), encoding="utf-8")
            except Exception as exc:
                with self._jobs_lock:
                    self._jobs[job_id].update(status="failed", stage="Failed", error=str(exc))
                    snapshot = dict(self._jobs[job_id])
                (run_dir / "job.json").write_text(json.dumps(snapshot, indent=2), encoding="utf-8")

        if sync:
            execute()
        else:
            threading.Thread(target=execute, name=f"sim-job-{job_id}", daemon=True).start()

        return self.get_run(job_id) or record
