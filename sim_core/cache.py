"""Thread-safe shared biome memory cache (``EnvironmentModel`` + ``ObstructionGrid``).

Eliminates 4x RAM duplication across ``sim``, ``sensor_coverage``, ``salvo_defense``,
and ``sim_bench`` by keeping a single read-only copy of each loaded biome in memory
behind per-biome locks and a bounded LRU eviction ceiling.
"""
from __future__ import annotations

from collections import OrderedDict
from dataclasses import dataclass
import json
import os
from pathlib import Path
import threading
from typing import Any

from .atmosphere import AtmosphereModel
from .environment import EnvironmentModel
from .obstruction import ObstructionGrid
from .registry import BiomePreset, REPO_ROOT, discover_biomes


@dataclass
class _BiomeSlot:
    preset: BiomePreset
    env: EnvironmentModel
    obs: ObstructionGrid | None = None
    surface_wind: tuple[float, str] | None = None
    heavy: bool = True


class SharedBiomeCache:
    """Single-process read-only cache shared across all EMAD simulation engines."""

    def __init__(
        self,
        repo_root: Path = REPO_ROOT,
        *,
        max_loaded_biomes: int | None = None,
    ) -> None:
        self.repo_root = Path(repo_root).resolve()
        env_cap = os.environ.get("EMAD_SIM_MAX_LOADED_BIOMES")
        if max_loaded_biomes is not None:
            cap = int(max_loaded_biomes)
        elif env_cap and env_cap.isdigit():
            cap = int(env_cap)
        else:
            cap = 3
        self.max_loaded_biomes = max(1, cap)
        self._presets: dict[str, BiomePreset] = {p.id: p for p in discover_biomes(self.repo_root)}
        self._meta_lock = threading.Lock()
        self._biome_locks: dict[str, threading.Lock] = {}
        self._slots: OrderedDict[str, _BiomeSlot] = OrderedDict()
        self._hits = 0
        self._misses = 0
        self._evictions = 0

    @property
    def presets(self) -> dict[str, BiomePreset]:
        return self._presets

    def get_preset(self, env_id: str) -> BiomePreset:
        preset = self._presets.get(env_id)
        if preset is None:
            raise KeyError(f"Unknown environmentId: {env_id}")
        return preset

    def _lock_for(self, env_id: str) -> threading.Lock:
        with self._meta_lock:
            lock = self._biome_locks.get(env_id)
            if lock is None:
                lock = threading.Lock()
                self._biome_locks[env_id] = lock
            return lock

    def _evict_if_needed_locked(self) -> None:
        """Evict least-recently-used heavy biomes when loaded count exceeds LRU cap."""
        heavy_ids = [bid for bid, slot in self._slots.items() if slot.heavy]
        while len(heavy_ids) > self.max_loaded_biomes:
            victim = heavy_ids.pop(0)
            self._slots.pop(victim, None)
            self._evictions += 1

    def _load_slot(self, env_id: str) -> _BiomeSlot:
        preset = self.get_preset(env_id)
        with self._meta_lock:
            existing = self._slots.get(env_id)
            if existing is not None:
                self._slots.move_to_end(env_id)
                self._hits += 1
                return existing

        biome_lock = self._lock_for(env_id)
        with biome_lock:
            with self._meta_lock:
                existing = self._slots.get(env_id)
                if existing is not None:
                    self._slots.move_to_end(env_id)
                    self._hits += 1
                    return existing

            tpl: dict[str, Any] = {}
            if preset.scenario_template_path and preset.scenario_template_path.is_file():
                try:
                    tpl = json.loads(preset.scenario_template_path.read_text(encoding="utf-8"))
                except Exception:
                    tpl = {}

            env = EnvironmentModel(
                preset.package_path,
                expected_manifest_sha256=preset.expected_manifest_sha256,
                derived_building_height_m=float(tpl.get("derivedBuildingHeightM", 11.0)),
                road_buffer_m=float(tpl.get("roadExposureBufferM", 14.0)),
                ghs_obat_path=preset.ghs_obat_path,
                worldpop_path=preset.worldpop_path,
                gebco_path=preset.gebco_path,
            )
            heavy = preset.biome != "verification_fixture" and len(env.buildings) >= 50
            slot = _BiomeSlot(preset=preset, env=env, heavy=heavy)
            with self._meta_lock:
                self._slots[env_id] = slot
                self._slots.move_to_end(env_id)
                self._misses += 1
                self._evict_if_needed_locked()
            return slot

    def get_env(self, env_id: str) -> tuple[BiomePreset, EnvironmentModel]:
        slot = self._load_slot(env_id)
        return slot.preset, slot.env

    def get_env_and_obs(self, env_id: str) -> tuple[BiomePreset, EnvironmentModel, ObstructionGrid]:
        slot = self._load_slot(env_id)
        if slot.obs is None:
            with self._lock_for(env_id):
                if slot.obs is None:
                    slot.obs = slot.env.obstruction_grid()
        return slot.preset, slot.env, slot.obs

    def get_surface_wind(self, env_id: str) -> tuple[float, str]:
        slot = self._load_slot(env_id)
        if slot.surface_wind is not None:
            return slot.surface_wind
        with self._lock_for(env_id):
            if slot.surface_wind is not None:
                return slot.surface_wind
            try:
                atm = AtmosphereModel.load(
                    slot.env,
                    external_path=slot.preset.weather_field_path,
                    cams_dust_path=slot.preset.cams_dust_path,
                )
                prof = atm.profile_summary(0.0, 0.0)
                speed = float(prof[0]["speedMps"]) if prof else 0.0
                ed = atm.document.get("edition") or "ERA5"
                src = f"{ed} {atm.valid_time or ''} · {prof[0]['altitudeAglM']:.0f} m AGL at origin".strip()
            except Exception:
                speed, src = 0.0, "ERA5 unavailable — calm assumed"
            slot.surface_wind = (round(speed, 2), src)
            return slot.surface_wind

    def stats(self) -> dict[str, Any]:
        with self._meta_lock:
            loaded = list(self._slots.keys())
            heavy_loaded = [k for k, v in self._slots.items() if v.heavy]
            obs_loaded = [k for k, v in self._slots.items() if v.obs is not None]
            return {
                "presetCount": len(self._presets),
                "maxLoadedBiomes": self.max_loaded_biomes,
                "loadedBiomes": loaded,
                "heavyLoadedCount": len(heavy_loaded),
                "obstructionGridsLoaded": obs_loaded,
                "hits": self._hits,
                "misses": self._misses,
                "evictions": self._evictions,
            }
