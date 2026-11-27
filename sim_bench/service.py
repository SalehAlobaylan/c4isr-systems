"""Stateful HTTP Service for EMAD Monte Carlo Debris, ML Surrogate & AI Benchmarks (Port 8768)."""
from __future__ import annotations

import json
from pathlib import Path
import threading
from typing import Any

import numpy as np

from salvo_defense.wta import DEFAULT_AI_WTA_PATH, load_ai_wta_model
from sim_core.atmosphere import AtmosphereModel, AtmosphereOverrideConfig
from sim_core.environment import EnvironmentModel
from sim_core.registry import BiomePreset, discover_biomes

from . import SIM_BENCH_VERSION
from .benchmark import DEFAULT_BENCH_OUT, run_benchmark_suite
from .debris_mc import run_monte_carlo_debris
from .surrogate import DEFAULT_SURROGATE_PATH, load_debris_surrogate

REPO_ROOT = Path(__file__).resolve().parents[1]


class BenchService:
    """Serves Monte Carlo debris risk heatmaps, ML model metadata, and AI WTA leaderboards."""

    def __init__(self, *, repo_root: Path = REPO_ROOT, public_origin: str = "http://127.0.0.1:8768") -> None:
        self.repo_root = Path(repo_root).resolve()
        self.public_origin = public_origin.rstrip("/")
        self._presets: dict[str, BiomePreset] = {p.id: p for p in discover_biomes(self.repo_root)}
        self._env_cache: dict[str, EnvironmentModel] = {}
        self._env_lock = threading.Lock()
        self._bench_lock = threading.Lock()
        self._bench_running = False

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

    def health(self) -> dict[str, Any]:
        return {
            "status": "ok",
            "service": "emad-sim-bench",
            "engineVersion": SIM_BENCH_VERSION,
            "surrogateLoaded": load_debris_surrogate() is not None,
            "aiPolicyLoaded": load_ai_wta_model() is not None,
            "environmentCount": len(self._presets),
            "benchmarkRunning": self._bench_running,
        }

    def list_environments(self) -> list[dict[str, Any]]:
        out = []
        for p in self._presets.values():
            _, env = self._get_env(p.id)
            cx = 0.5 * (env.east_min + env.east_max)
            cy = 0.5 * (env.north_min + env.north_max)
            lon, lat, _ = env.enu_to_geodetic(cx, cy, env.up_median)
            out.append(
                {
                    "id": p.id,
                    "title": p.title,
                    "biome": p.biome,
                    "boundsWgs84": [round(v, 6) for v in env.bounds_wgs84],
                    "centerLonLat": [round(lon, 6), round(lat, 6)],
                    "buildingCount": len(env.buildings),
                }
            )
        return out

    def models_status(self) -> dict[str, Any]:
        sur = load_debris_surrogate()
        ai_meta = None
        if DEFAULT_AI_WTA_PATH.is_file():
            raw = np.load(DEFAULT_AI_WTA_PATH, allow_pickle=False)
            if "metadata_json" in raw:
                ai_meta = json.loads(str(raw["metadata_json"]))
        return {
            "debrisSurrogate": {
                "available": sur is not None,
                "path": str(DEFAULT_SURROGATE_PATH.relative_to(self.repo_root)) if DEFAULT_SURROGATE_PATH.is_file() else None,
                "metadata": sur.metadata if sur is not None else None,
            },
            "aiWtaPolicy": {
                "available": DEFAULT_AI_WTA_PATH.is_file(),
                "path": str(DEFAULT_AI_WTA_PATH.relative_to(self.repo_root)) if DEFAULT_AI_WTA_PATH.is_file() else None,
                "metadata": ai_meta,
            },
        }

    def run_monte_carlo(self, payload: dict[str, Any]) -> dict[str, Any]:
        env_id = str(payload.get("environmentId") or next(iter(self._presets)))
        preset, env = self._get_env(env_id)
        lon = payload.get("lon")
        lat = payload.get("lat")
        if lon is not None and lat is not None:
            ie, in_, _ = env.geodetic_to_enu(float(lon), float(lat), env.up_median)
        else:
            ie = 0.5 * (env.east_min + env.east_max)
            in_ = 0.5 * (env.north_min + env.north_max)
        g = env.height_at(ie, in_, fallback=env.up_median)
        agl = max(50.0, min(3500.0, float(payload.get("interceptAglM", 550.0))))
        spd = max(30.0, min(500.0, float(payload.get("threatSpeedMps", 120.0))))
        hdg_deg = float(payload.get("threatHeadingDeg", 45.0))
        hr = np.radians(hdg_deg)
        vel = (float(np.sin(hr) * spd), float(np.cos(hr) * spd), -12.0)

        wind_add = payload.get("windAddEnuMps") or [0.0, 0.0, 0.0]
        ov = AtmosphereOverrideConfig(
            wind_scale=max(0.0, min(6.0, float(payload.get("windScale", 1.0)))),
            wind_add_enu_mps=[float(wind_add[0]), float(wind_add[1]), 0.0],
        )
        atm = AtmosphereModel.load(env, external_path=preset.weather_field_path, override=ov, cams_dust_path=preset.cams_dust_path)
        res = run_monte_carlo_debris(
            env=env,
            atm=atm,
            intercept_enu=(ie, in_, g + agl),
            threat_vel_enu=vel,
            n_trials=int(payload.get("trials", 120)),
            frags_per_trial=int(payload.get("fragmentsPerTrial", 36)),
            wind_sigma_mps=float(payload.get("windSigmaMps", 3.5)),
            pos_sigma_m=float(payload.get("posSigmaM", 45.0)),
            seed=int(payload.get("seed", 20261127)),
        )
        # Also include instant ML surrogate comparison if trained
        sur = load_debris_surrogate()
        if sur is not None:
            we, wn, _, _, _ = atm.sample(np.array([ie]), np.array([in_]), np.array([g + agl]), ground_up_m=np.array([g]))
            res["surrogatePreview"] = sur.predict_exposure(
                intercept_enu=np.array([ie, in_, g + agl]),
                threat_vel_enu=np.array(vel),
                wind_en=(float(we[0]), float(wn[0])),
                agl_m=agl,
                env=env,
            )
        return res

    def get_leaderboard(self) -> dict[str, Any] | None:
        if DEFAULT_BENCH_OUT.is_file():
            return json.loads(DEFAULT_BENCH_OUT.read_text(encoding="utf-8"))
        return None

    def run_leaderboard(self, payload: dict[str, Any]) -> dict[str, Any]:
        with self._bench_lock:
            self._bench_running = True
            try:
                biomes = payload.get("biomes")
                seeds_in = payload.get("seeds") or [1001, 2002]
                return run_benchmark_suite(
                    env_ids=list(biomes) if biomes else None,
                    seeds=tuple(int(s) for s in seeds_in[:4]),
                    out_path=DEFAULT_BENCH_OUT,
                )
            finally:
                self._bench_running = False
