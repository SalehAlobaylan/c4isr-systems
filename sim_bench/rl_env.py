"""Deterministic Observation-Only RL Environment wrapper around Salvo Defense.

Strictly enforces EMAD's ``EXERCISE_OBSERVATION`` boundary: every policy
evaluated by ``SalvoRLEnv`` receives ONLY the 14-dimensional fused-track /
battery feature vector from ``salvo_defense.wta.compute_pair_features`` and
NEVER sees sealed ``EXERCISE_TRUTH``.
"""
from __future__ import annotations

from typing import Any

import numpy as np

from salvo_defense.engine import run_salvo_policy
from salvo_defense.service import SalvoService
from sim_core.atmosphere import AtmosphereModel, AtmosphereOverrideConfig


class SalvoRLEnv:
    """Fast deterministic episodic evaluator for observation-only WTA policies."""

    def __init__(self, *, env_ids: list[str] | None = None, surrogate: Any = None) -> None:
        self.service = SalvoService()
        self.surrogate = surrogate
        available = list(self.service._presets.keys())
        if env_ids:
            self.env_ids = [e for e in env_ids if e in available]
        else:
            self.env_ids = [e for e in ("riyadh-observation-v1", "asir-highlands-30km-v1", "jubail-coast-30km-v1", "owned-fixture-v1") if e in available]
        if not self.env_ids:
            self.env_ids = available[:1]

        self._atm_cache: dict[str, AtmosphereModel] = {}
        self._defaults_cache: dict[str, dict[str, Any]] = {}
        for eid in self.env_ids:
            preset, env = self.service._get_env(eid)
            self._defaults_cache[eid] = self.service._default_draft(preset, env)
            self._atm_cache[eid] = AtmosphereModel.load(env, external_path=preset.weather_field_path, cams_dust_path=preset.cams_dust_path)

    def evaluate_episode(
        self,
        *,
        env_id: str,
        seed: int,
        raid: dict[str, Any] | None = None,
        policy: str = "ai_policy",
        ai_weights: np.ndarray | None = None,
        visibility_km: float = 10.0,
    ) -> dict[str, Any]:
        preset, env = self.service._get_env(env_id)
        payload = dict(self._defaults_cache[env_id])
        payload["seed"] = int(seed)
        payload["visibilityKm"] = float(visibility_km)
        if raid is not None:
            payload["raid"] = {**payload["raid"], **raid}
        _, _, spec = self.service.build_scenario(payload)
        atm = self._atm_cache[env_id]
        res = run_salvo_policy(
            spec,
            env=env,
            atm=atm,
            policy=policy,
            surrogate=self.surrogate,
            ai_weights=ai_weights,
            record_replay=False,
        )
        return res["evaluation"]
