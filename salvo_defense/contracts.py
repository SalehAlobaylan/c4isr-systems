"""Typed contracts & 4-partition envelopes for Multi-Threat Salvo & Layered Defense.

Strictly preserves EMAD's 4-partition architecture:
  - ``EXERCISE_INPUT``: raid composition, battery tiers/magazines, sensor network, frozen civil traffic
  - ``EXERCISE_TRUTH``: sealed threat trajectories, decoy identities, Pk roll outcomes
  - ``EXERCISE_OBSERVATION``: multi-sensor detections, GNN+Kalman fused tracks, Bayesian class estimates
  - ``EVALUATION``: leakers, total defense cost ($k), decoy waste, WorldPop & OSM road debris impact
"""
from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any

SALVO_ENGINE_VERSION = "emad-salvo-py-0.1.0"
SALVO_REPLAY_SCHEMA = "emad.salvo-replay.v1"

SCOPE_EXERCISE_INPUT = "EXERCISE_INPUT"
SCOPE_EXERCISE_TRUTH = "EXERCISE_TRUTH"
SCOPE_EXERCISE_OBSERVATION = "EXERCISE_OBSERVATION"
SCOPE_EVALUATION = "EVALUATION"
DERIVATION_LABEL = "synthetic/derived"

THREAT_CLASS_SPECS: dict[str, dict[str, Any]] = {
    "decoy": {
        "label": "RF Decoy Lure",
        "speedMps": 115.0,
        "rcsM2": 0.60,
        "irSignature": 0.18,  # Weak thermal signature -> EO/IR helps unmask decoys!
        "cruiseAglM": 130.0,
        "damageValue": 0.0,
        "lethal": False,
        "fragmentCount": 14,
    },
    "one_way_uav": {
        "label": "One-Way Attack UAV",
        "speedMps": 68.0,
        "rcsM2": 0.14,
        "irSignature": 0.55,
        "cruiseAglM": 75.0,
        "damageValue": 38.0,
        "lethal": True,
        "fragmentCount": 24,
    },
    "cruise_missile": {
        "label": "Terrain-Skim Cruise Missile",
        "speedMps": 230.0,
        "rcsM2": 0.12,
        "irSignature": 0.92,
        "cruiseAglM": 55.0,
        "damageValue": 80.0,
        "lethal": True,
        "fragmentCount": 32,
    },
    "ballistic_missile": {
        "label": "Short-Range Ballistic Missile",
        "speedMps": 370.0,
        "rcsM2": 0.48,
        "irSignature": 1.00,
        "cruiseAglM": 1600.0,  # apogee
        "damageValue": 110.0,
        "lethal": True,
        "fragmentCount": 38,
    },
}

BATTERY_TIER_SPECS: dict[str, dict[str, Any]] = {
    "tier1_sam": {
        "label": "Tier-1 Area SAM",
        "minRangeM": 1200.0,
        "maxRangeM": 16000.0,
        "maxAltitudeM": 12000.0,
        "speedMps": 760.0,
        "pkBase": 0.88,
        "costKUsd": 1200.0,
        "reloadS": 3.0,
        "maxSimultaneous": 2,
        "defaultMagazine": 6,
        "color": "#38bdf8",
    },
    "tier2_shorad": {
        "label": "Tier-2 SHORAD",
        "minRangeM": 400.0,
        "maxRangeM": 7200.0,
        "maxAltitudeM": 4500.0,
        "speedMps": 440.0,
        "pkBase": 0.82,
        "costKUsd": 180.0,
        "reloadS": 2.2,
        "maxSimultaneous": 2,
        "defaultMagazine": 8,
        "color": "#34d399",
    },
    "tier3_point_defense": {
        "label": "Tier-3 Point Defense",
        "minRangeM": 120.0,
        "maxRangeM": 2900.0,
        "maxAltitudeM": 1800.0,
        "speedMps": 310.0,
        "pkBase": 0.74,
        "costKUsd": 18.0,
        "reloadS": 1.2,
        "maxSimultaneous": 2,
        "defaultMagazine": 16,
        "color": "#fbbf24",
    },
}


@dataclass(frozen=True)
class SalvoAssetSpec:
    id: str
    label: str
    east: float
    north: float
    radius_m: float = 420.0
    value: float = 100.0


@dataclass(frozen=True)
class SalvoBatterySpec:
    id: str
    tier: str  # tier1_sam | tier2_shorad | tier3_point_defense
    label: str
    east: float
    north: float
    height_agl_m: float = 8.0
    magazine: int = 8
    min_range_m: float | None = None
    max_range_m: float | None = None
    speed_mps: float | None = None
    pk_base: float | None = None
    cost_k_usd: float | None = None
    reload_s: float | None = None
    max_simultaneous: int | None = None

    def resolved(self) -> dict[str, Any]:
        d = BATTERY_TIER_SPECS[self.tier]
        return {
            "id": self.id,
            "tier": self.tier,
            "label": self.label or d["label"],
            "east": self.east,
            "north": self.north,
            "heightAglM": self.height_agl_m,
            "magazine": self.magazine,
            "minRangeM": float(self.min_range_m if self.min_range_m is not None else d["minRangeM"]),
            "maxRangeM": float(self.max_range_m if self.max_range_m is not None else d["maxRangeM"]),
            "maxAltitudeM": float(d["maxAltitudeM"]),
            "speedMps": float(self.speed_mps if self.speed_mps is not None else d["speedMps"]),
            "pkBase": float(self.pk_base if self.pk_base is not None else d["pkBase"]),
            "costKUsd": float(self.cost_k_usd if self.cost_k_usd is not None else d["costKUsd"]),
            "reloadS": float(self.reload_s if self.reload_s is not None else d["reloadS"]),
            "maxSimultaneous": int(self.max_simultaneous if self.max_simultaneous is not None else d["maxSimultaneous"]),
            "color": d["color"],
        }


@dataclass(frozen=True)
class SalvoSensorSpec:
    id: str
    type: str  # radar | eo_ir | acoustic
    label: str
    east: float
    north: float
    mast_agl_m: float = 15.0
    max_range_m: float = 12000.0
    sector_center_deg: float = 0.0
    sector_width_deg: float = 360.0


@dataclass(frozen=True)
class SalvoThreatSpec:
    id: str
    kind: str  # decoy | one_way_uav | cruise_missile | ballistic_missile
    spawn_s: float
    start_enu: list[float]  # [east, north, agl_m]
    target_asset_id: str
    speed_mps: float | None = None
    rcs_m2: float | None = None
    apogee_m: float = 1600.0
    wave: int = 1

    def resolved(self) -> dict[str, Any]:
        d = THREAT_CLASS_SPECS[self.kind]
        return {
            "id": self.id,
            "kind": self.kind,
            "label": d["label"],
            "spawnS": float(self.spawn_s),
            "startEnu": list(self.start_enu),
            "targetAssetId": self.target_asset_id,
            "speedMps": float(self.speed_mps if self.speed_mps is not None else d["speedMps"]),
            "rcsM2": float(self.rcs_m2 if self.rcs_m2 is not None else d["rcsM2"]),
            "irSignature": float(d["irSignature"]),
            "damageValue": float(d["damageValue"]),
            "lethal": bool(d["lethal"]),
            "apogeeM": float(self.apogee_m),
            "wave": int(self.wave),
            "fragmentCount": int(d["fragmentCount"]),
        }


@dataclass(frozen=True)
class SalvoScenarioSpec:
    scenario_id: str
    title: str
    environment_id: str
    seed: int
    duration_s: float
    dt_s: float
    assets: list[SalvoAssetSpec]
    batteries: list[SalvoBatterySpec]
    sensors: list[SalvoSensorSpec]
    threats: list[SalvoThreatSpec]
    enable_civilian_traffic: bool = True
    visibility_km: float = 10.0
    wind_scale: float = 1.0
    wind_add_enu_mps: list[float] = field(default_factory=lambda: [0.0, 0.0, 0.0])
    policy: str = "combinatorial_wta"  # greedy_earliest | combinatorial_wta | ai_policy
    sensor_network_id: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)
