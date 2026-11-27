"""Stateful Multi-Threat Salvo & Layered Air Defense HTTP Service (Port 8767).

Bridges EMAD's 2D Map ``Salvo`` panel and the dedicated 3D Three.js Salvo
Workbench, reusing ``sim_core``'s unified biome registry, GHS-OBAT/WorldPop
enrichment, and ``emad.sensor-network.v1`` handoff store.
"""
from __future__ import annotations

import datetime as dt
import json
import math
from pathlib import Path
import re
import secrets
import threading
from typing import Any

from sim_core.atmosphere import AtmosphereModel, AtmosphereOverrideConfig
from sim_core.environment import EnvironmentModel
from sim_core.handoff import SensorNetworkStore
from sim_core.registry import BiomePreset, discover_biomes

from .contracts import (
    BATTERY_TIER_SPECS,
    SALVO_ENGINE_VERSION,
    THREAT_CLASS_SPECS,
    SalvoAssetSpec,
    SalvoBatterySpec,
    SalvoScenarioSpec,
    SalvoSensorSpec,
    SalvoThreatSpec,
)
from .engine import run_salvo_comparison
from .wta import load_ai_wta_model

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_RUNS_ROOT = REPO_ROOT / "tmp" / "salvo-runs"
JOB_ID_RE = re.compile(r"salvjob_[a-f0-9]{12}")


def _clampf(val: Any, lo: float, hi: float, default: float) -> float:
    try:
        f = float(val)
    except (TypeError, ValueError):
        return default
    return max(lo, min(hi, f)) if math.isfinite(f) else default


class SalvoService:
    """In-process Salvo Defense orchestrator with environment cache and async run jobs."""

    def __init__(
        self,
        *,
        repo_root: Path = REPO_ROOT,
        runs_root: Path = DEFAULT_RUNS_ROOT,
        public_origin: str = "http://127.0.0.1:8767",
    ) -> None:
        self.repo_root = Path(repo_root).resolve()
        self.runs_root = Path(runs_root).resolve()
        self.runs_root.mkdir(parents=True, exist_ok=True)
        self.public_origin = public_origin.rstrip("/")

        self._presets: dict[str, BiomePreset] = {p.id: p for p in discover_biomes(self.repo_root)}
        self._network_store = SensorNetworkStore(self.repo_root / "tmp" / "shared-layouts")
        self._env_cache: dict[str, EnvironmentModel] = {}
        self._env_lock = threading.Lock()
        self._jobs_lock = threading.Lock()
        self._jobs: dict[str, dict[str, Any]] = {}
        self._load_existing_runs()

    def _load_existing_runs(self) -> None:
        for mp in sorted(self.runs_root.glob("*/job.json")):
            try:
                rec = json.loads(mp.read_text(encoding="utf-8"))
                jid = str(rec["jobId"])
                if rec.get("status") == "running":
                    rec["status"] = "interrupted"
                    rec["stage"] = "Server restarted before run completed"
                rec["viewerUrl"] = f"{self.public_origin}/?run={jid}"
                self._jobs[jid] = rec
            except Exception:
                continue

    def _get_env(self, env_id: str) -> tuple[BiomePreset, EnvironmentModel]:
        preset = self._presets.get(env_id)
        if preset is None:
            raise KeyError(f"Unknown environmentId: {env_id}")
        with self._env_lock:
            env = self._env_cache.get(env_id)
            if env is None:
                env = EnvironmentModel(
                    preset.package_path,
                    expected_manifest_sha256=preset.expected_manifest_sha256,
                    ghs_obat_path=preset.ghs_obat_path,
                    worldpop_path=preset.worldpop_path,
                    gebco_path=preset.gebco_path,
                )
                self._env_cache[env_id] = env
        return preset, env

    def _enu_to_ll(self, env: EnvironmentModel, e: float, n: float) -> list[float]:
        lon, lat, _ = env.enu_to_geodetic(float(e), float(n), env.up_median)
        return [round(lon, 6), round(lat, 6)]

    def _default_draft(self, preset: BiomePreset, env: EnvironmentModel) -> dict[str, Any]:
        span_e = env.east_max - env.east_min
        span_n = env.north_max - env.north_min
        span = min(span_e, span_n)
        cx = 0.5 * (env.east_min + env.east_max)
        cy = 0.5 * (env.north_min + env.north_max)

        # Defended hub slightly NE of center
        ae, an = cx + 0.14 * span_e, cy + 0.14 * span_n
        a_ll = self._enu_to_ll(env, ae, an)
        assets = [
            {"id": "asset-1", "label": "Primary Command Hub", "lon": a_ll[0], "lat": a_ll[1], "radiusM": round(min(600.0, max(360.0, span * 0.06)), 0), "value": 100.0}
        ]

        # 3-Tier Layered Defense Batteries
        b1_ll = self._enu_to_ll(env, ae - 0.12 * span_e, an + 0.06 * span_n)
        b2_ll = self._enu_to_ll(env, ae - 0.06 * span_e, an - 0.09 * span_n)
        b3_ll = self._enu_to_ll(env, ae + 0.03 * span_e, an - 0.02 * span_n)
        batteries = [
            {"id": "sam-1", "tier": "tier1_sam", "label": "Tier-1 Area SAM", "lon": b1_ll[0], "lat": b1_ll[1], "magazine": 5},
            {"id": "shorad-1", "tier": "tier2_shorad", "label": "Tier-2 SHORAD", "lon": b2_ll[0], "lat": b2_ll[1], "magazine": 7},
            {"id": "pd-1", "tier": "tier3_point_defense", "label": "Tier-3 Point Defense", "lon": b3_ll[0], "lat": b3_ll[1], "magazine": 14},
        ]

        # Multi-Modal Sensor Network (Radar + EO/IR for decoy discrimination + Acoustic)
        s1_ll = self._enu_to_ll(env, ae - 0.08 * span_e, an + 0.02 * span_n)
        s2_ll = self._enu_to_ll(env, ae - 0.02 * span_e, an - 0.05 * span_n)
        sensors = [
            {"id": "rad-1", "type": "radar", "label": "Long-Range Radar", "lon": s1_ll[0], "lat": s1_ll[1], "mastAglM": 18.0, "maxRangeM": round(max(10000.0, span * 0.95), 0)},
            {"id": "eo-1", "type": "eo_ir", "label": "EO/IR Discriminator", "lon": s2_ll[0], "lat": s2_ll[1], "mastAglM": 14.0, "maxRangeM": round(max(6500.0, span * 0.55), 0)},
        ]

        return {
            "environmentId": preset.id,
            "title": f"{preset.title.split('(')[0].strip()} — Mixed Salvo Layered Defense",
            "presetId": "mixed_swarm_decoys",
            "seed": 20261127,
            "durationS": 115.0,
            "enableCivilianTraffic": True,
            "visibilityKm": 10.0,
            "windScale": 1.0,
            "windAddEnuMps": [0.0, 0.0, 0.0],
            "raid": {
                "decoys": 3,
                "uavs": 5,
                "cruiseMissiles": 2,
                "ballisticMissiles": 1,
                "waves": 2,
                "ingressSectorDeg": 225.0,
                "ingressSpreadDeg": 95.0,
            },
            "assets": assets,
            "batteries": batteries,
            "sensors": sensors,
            "sensorNetworkId": None,
        }

    def _synthesize_raid_threats(
        self,
        env: EnvironmentModel,
        raid_cfg: dict[str, Any],
        assets: list[SalvoAssetSpec],
        seed: int,
    ) -> list[SalvoThreatSpec]:
        import numpy as np

        rng = np.random.default_rng(seed + 1117)
        n_dec = int(_clampf(raid_cfg.get("decoys"), 0, 12, 3))
        n_uav = int(_clampf(raid_cfg.get("uavs"), 0, 16, 5))
        n_cru = int(_clampf(raid_cfg.get("cruiseMissiles"), 0, 10, 2))
        n_bal = int(_clampf(raid_cfg.get("ballisticMissiles"), 0, 6, 1))
        waves = max(1, min(3, int(_clampf(raid_cfg.get("waves"), 1, 3, 2))))
        center_az = float(_clampf(raid_cfg.get("ingressSectorDeg"), 0.0, 360.0, 225.0))
        spread_deg = float(_clampf(raid_cfg.get("ingressSpreadDeg"), 20.0, 340.0, 95.0))

        cx = 0.5 * (env.east_min + env.east_max)
        cy = 0.5 * (env.north_min + env.north_max)
        rx = 0.44 * (env.east_max - env.east_min)
        ry = 0.44 * (env.north_max - env.north_min)

        items: list[tuple[str, float]] = []
        # Wave 1: decoys lead slightly ahead of UAVs to bait greedy SAMs!
        for i in range(n_dec):
            items.append(("decoy", 0.0 + i * 1.4))
        for i in range(n_uav):
            w_idx = i % waves
            items.append(("one_way_uav", 1.5 + w_idx * 14.0 + (i // waves) * 2.2))
        for i in range(n_cru):
            w_idx = i % waves
            items.append(("cruise_missile", 4.0 + w_idx * 16.0 + i * 2.5))
        for i in range(n_bal):
            items.append(("ballistic_missile", 6.0 + i * 12.0))

        threats: list[SalvoThreatSpec] = []
        for idx, (kind, spawn_s) in enumerate(items):
            az_deg = center_az + float(rng.uniform(-0.5 * spread_deg, 0.5 * spread_deg))
            az_rad = math.radians(az_deg)
            se = cx + math.sin(az_rad) * rx
            sn = cy + math.cos(az_rad) * ry
            sagl = float(THREAT_CLASS_SPECS[kind]["cruiseAglM"])
            if kind == "ballistic_missile":
                sagl = 120.0
            target_asset = assets[idx % len(assets)]
            threats.append(
                SalvoThreatSpec(
                    id=f"{kind[:3]}-{idx + 1:02d}",
                    kind=kind,
                    spawn_s=round(spawn_s, 1),
                    start_enu=[round(se, 1), round(sn, 1), round(sagl, 1)],
                    target_asset_id=target_asset.id,
                    apogee_m=float(THREAT_CLASS_SPECS[kind]["cruiseAglM"]) if kind == "ballistic_missile" else 0.0,
                    wave=1 if spawn_s < 12.0 else 2,
                )
            )
        if not threats:
            raise ValueError("Raid must include at least one threat.")
        return threats

    def health(self) -> dict[str, Any]:
        with self._jobs_lock:
            running = sum(1 for j in self._jobs.values() if j.get("status") == "running")
            total = len(self._jobs)
        return {
            "status": "ok",
            "service": "emad-salvo-defense",
            "engineVersion": SALVO_ENGINE_VERSION,
            "aiPolicyAvailable": load_ai_wta_model() is not None,
            "capabilities": ["multi-sensor-fusion", "layered-wta", "decoy-discrimination", "airspace-deconfliction", "network-import"],
            "viewerOrigin": self.public_origin,
            "environmentCount": len(self._presets),
            "runningJobs": running,
            "totalRuns": total,
        }

    def list_environments(self) -> list[dict[str, Any]]:
        out = []
        for preset in self._presets.values():
            _, env = self._get_env(preset.id)
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
                    "boundsWgs84": [round(v, 6) for v in env.bounds_wgs84],
                    "origin": list(env.origin),
                    "batteryTiers": BATTERY_TIER_SPECS,
                    "threatClasses": THREAT_CLASS_SPECS,
                    "defaults": self._default_draft(preset, env),
                }
            )
        return out

    def list_networks(self, environment_id: str | None = None) -> list[dict[str, Any]]:
        return self._network_store.list(environment_id=environment_id)

    def list_runs(self) -> list[dict[str, Any]]:
        with self._jobs_lock:
            items = [dict(j) for j in self._jobs.values()]
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
        preset = self._presets.get(str(rec.get("environmentId", "")))
        glb = preset.glb_path if (preset and preset.glb_path and preset.glb_path.is_file()) else None
        imagery = preset.imagery_path if (preset and preset.imagery_path and preset.imagery_path.is_file()) else None
        return self.runs_root / job_id, glb, imagery

    def build_scenario(self, payload: dict[str, Any]) -> tuple[BiomePreset, EnvironmentModel, SalvoScenarioSpec]:
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        preset, env = self._get_env(env_id)
        defaults = self._default_draft(preset, env)

        def to_en(lon: Any, lat: Any, label: str) -> tuple[float, float]:
            e, n, _ = env.geodetic_to_enu(float(lon), float(lat), env.up_median)
            pad = 250.0
            if not (env.east_min - pad <= e <= env.east_max + pad and env.north_min - pad <= n <= env.north_max + pad):
                raise ValueError(f"{label} lies outside verified {preset.id} bounds.")
            return round(e, 2), round(n, 2)

        assets: list[SalvoAssetSpec] = []
        for k, a in enumerate(payload.get("assets") or defaults["assets"]):
            ae, an = to_en(a["lon"], a["lat"], f"Asset {k + 1}")
            assets.append(
                SalvoAssetSpec(
                    id=str(a.get("id") or f"asset-{k + 1}"),
                    label=str(a.get("label") or f"Asset {k + 1}"),
                    east=ae,
                    north=an,
                    radius_m=_clampf(a.get("radiusM"), 120.0, 2500.0, 420.0),
                    value=_clampf(a.get("value"), 10.0, 500.0, 100.0),
                )
            )
        if not assets:
            raise ValueError("At least one defended asset is required.")

        batteries: list[SalvoBatterySpec] = []
        for k, b in enumerate(payload.get("batteries") or defaults["batteries"]):
            tier = str(b.get("tier") or "tier2_shorad")
            if tier not in BATTERY_TIER_SPECS:
                raise ValueError(f"Unsupported battery tier: {tier}")
            be, bn = to_en(b["lon"], b["lat"], f"Battery {k + 1}")
            batteries.append(
                SalvoBatterySpec(
                    id=str(b.get("id") or f"{tier}-{k + 1}"),
                    tier=tier,
                    label=str(b.get("label") or BATTERY_TIER_SPECS[tier]["label"]),
                    east=be,
                    north=bn,
                    magazine=int(_clampf(b.get("magazine"), 1, 40, BATTERY_TIER_SPECS[tier]["defaultMagazine"])),
                )
            )
        if not batteries:
            raise ValueError("Place at least one defense battery.")

        # Optional import of saved sensor_coverage network
        sensors_raw = payload.get("sensors") or defaults["sensors"]
        net_id = payload.get("sensorNetworkId")
        if net_id:
            net = self._network_store.get(str(net_id))
            if net and net.get("sensors"):
                sensors_raw = net["sensors"]

        sensors: list[SalvoSensorSpec] = []
        for k, s in enumerate(sensors_raw[:16]):
            stype = str(s.get("type") or "radar")
            if stype not in ("radar", "eo_ir", "acoustic"):
                stype = "radar"
            se, sn = to_en(s["lon"], s["lat"], f"Sensor {k + 1}")
            sensors.append(
                SalvoSensorSpec(
                    id=str(s.get("id") or f"sens-{k + 1}"),
                    type=stype,
                    label=str(s.get("label") or f"{stype.upper()} {k + 1}"),
                    east=se,
                    north=sn,
                    mast_agl_m=_clampf(s.get("mastAglM"), 2.0, 120.0, 15.0),
                    max_range_m=_clampf(s.get("maxRangeM"), 500.0, 35000.0, 12000.0),
                    sector_center_deg=_clampf(s.get("sectorCenterDeg"), 0.0, 360.0, 0.0),
                    sector_width_deg=_clampf(s.get("sectorWidthDeg"), 15.0, 360.0, 360.0),
                )
            )

        seed = int(payload.get("seed", defaults["seed"]))
        threats = self._synthesize_raid_threats(env, payload.get("raid") or defaults["raid"], assets, seed)
        wind_add_in = payload.get("windAddEnuMps") or [0.0, 0.0, 0.0]

        spec = SalvoScenarioSpec(
            scenario_id=f"{preset.id}-salvo",
            title=str(payload.get("title") or defaults["title"])[:120],
            environment_id=preset.id,
            seed=seed,
            duration_s=_clampf(payload.get("durationS"), 40.0, 220.0, 115.0),
            dt_s=0.25,
            assets=assets,
            batteries=batteries,
            sensors=sensors,
            threats=threats,
            enable_civilian_traffic=bool(payload.get("enableCivilianTraffic", True)),
            visibility_km=_clampf(payload.get("visibilityKm"), 0.5, 30.0, 10.0),
            wind_scale=_clampf(payload.get("windScale"), 0.0, 6.0, 1.0),
            wind_add_enu_mps=[
                _clampf(wind_add_in[0] if len(wind_add_in) > 0 else 0.0, -40.0, 40.0, 0.0),
                _clampf(wind_add_in[1] if len(wind_add_in) > 1 else 0.0, -40.0, 40.0, 0.0),
                0.0,
            ],
            sensor_network_id=str(net_id) if net_id else None,
        )
        return preset, env, spec

    def _build_overlay(self, env: EnvironmentModel, replay: dict[str, Any]) -> dict[str, Any]:
        features: list[dict[str, Any]] = []
        wta = replay["runs"].get("combinatorial_wta") or next(iter(replay["runs"].values()))
        for th in wta["truth"]["threats"]:
            pts = th["samples"]
            if len(pts) >= 2:
                coords = []
                for p in pts[:: max(1, len(pts) // 24)]:
                    lon, lat, _ = env.enu_to_geodetic(p[1], p[2], p[3])
                    coords.append([round(lon, 6), round(lat, 6)])
                lon_e, lat_e, _ = env.enu_to_geodetic(pts[-1][1], pts[-1][2], pts[-1][3])
                coords.append([round(lon_e, 6), round(lat_e, 6)])
                features.append(
                    {
                        "type": "Feature",
                        "geometry": {"type": "LineString", "coordinates": coords},
                        "properties": {
                            "simKind": "salvo-decoy-track" if not th["lethal"] else "salvo-threat-track",
                            "threatKind": th["kind"],
                            "status": th["status"],
                            "label": f"{th['id']} ({th['status']})",
                        },
                    }
                )
        for ev in wta["events"]:
            if ev["kind"] in ("intercept_kill", "leaker_impact") and ev.get("enu"):
                lon, lat, _ = env.enu_to_geodetic(ev["enu"][0], ev["enu"][1], ev["enu"][2])
                features.append(
                    {
                        "type": "Feature",
                        "geometry": {"type": "Point", "coordinates": [round(lon, 6), round(lat, 6)]},
                        "properties": {
                            "simKind": "salvo-kill" if ev["kind"] == "intercept_kill" else "salvo-leaker",
                            "label": f"{ev['kind']} @ t={ev['tS']}s",
                        },
                    }
                )
        return {"type": "FeatureCollection", "features": features}

    def start_run(self, payload: dict[str, Any], *, sync: bool = False) -> dict[str, Any]:
        preset, env, spec = self.build_scenario(payload)
        req_id = payload.get("jobId")
        if req_id is not None:
            job_id = str(req_id)
            if not JOB_ID_RE.fullmatch(job_id):
                raise ValueError("Invalid jobId format")
        else:
            job_id = "salvjob_" + secrets.token_hex(6)

        run_dir = self.runs_root / job_id
        run_dir.mkdir(parents=True, exist_ok=True)
        now_iso = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")
        rec: dict[str, Any] = {
            "jobId": job_id,
            "environmentId": preset.id,
            "environmentTitle": preset.title,
            "title": spec.title,
            "status": "running",
            "stage": "1/3 Running Multi-Sensor Fusion & Greedy Baseline…",
            "createdAt": now_iso,
            "completedAt": None,
            "viewerUrl": f"{self.public_origin}/?run={job_id}",
            "hasGlb": bool(preset.glb_path and preset.glb_path.is_file()),
            "hasImagery": bool(preset.imagery_path and preset.imagery_path.is_file()),
            "threatCount": len(spec.threats),
            "batteryCount": len(spec.batteries),
            "error": None,
            "comparison": None,
            "resultOverlay": None,
        }
        with self._jobs_lock:
            self._jobs[job_id] = rec
        (run_dir / "job.json").write_text(json.dumps(rec, indent=2), encoding="utf-8")

        # Optional ML debris surrogate if trained in Phase 6
        surrogate = None
        try:
            from sim_bench.surrogate import load_debris_surrogate

            surrogate = load_debris_surrogate()
        except Exception:
            surrogate = None

        def execute() -> None:
            try:
                ov = AtmosphereOverrideConfig(
                    wind_scale=spec.wind_scale,
                    wind_add_enu_mps=list(spec.wind_add_enu_mps),
                )
                atm = AtmosphereModel.load(
                    env,
                    external_path=preset.weather_field_path,
                    override=ov,
                    cams_dust_path=preset.cams_dust_path,
                )
                with self._jobs_lock:
                    self._jobs[job_id]["stage"] = "2/3 Comparing Greedy vs Combinatorial Layered WTA…"
                draft_clean = {k: v for k, v in payload.items() if k not in ("_sync", "jobId")}
                draft_clean["environmentId"] = preset.id
                replay = run_salvo_comparison(spec, env=env, atm=atm, surrogate=surrogate, source_draft=draft_clean)
                replay["jobId"] = job_id
                replay["environmentId"] = preset.id
                with self._jobs_lock:
                    self._jobs[job_id]["stage"] = "3/3 Saving 3D Salvo replay & map overlay…"
                (run_dir / "replay.json").write_text(json.dumps(replay, separators=(",", ":")), encoding="utf-8")
                overlay = self._build_overlay(env, replay)
                done_iso = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")
                with self._jobs_lock:
                    self._jobs[job_id].update(
                        status="completed",
                        stage="Completed",
                        completedAt=done_iso,
                        comparison=replay["comparison"],
                        policies=replay["policies"],
                        elapsedS=replay["elapsedS"],
                        resultOverlay=overlay,
                    )
                    snap = dict(self._jobs[job_id])
                (run_dir / "job.json").write_text(json.dumps(snap, indent=2), encoding="utf-8")
            except Exception as exc:
                with self._jobs_lock:
                    self._jobs[job_id].update(status="failed", stage="Failed", error=str(exc))
                    snap = dict(self._jobs[job_id])
                (run_dir / "job.json").write_text(json.dumps(snap, indent=2), encoding="utf-8")

        if sync:
            execute()
        else:
            threading.Thread(target=execute, name=f"salvo-job-{job_id}", daemon=True).start()
        return self.get_run(job_id) or rec
