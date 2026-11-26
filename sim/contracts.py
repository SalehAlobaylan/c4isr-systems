"""Typed contracts and EMAD partition envelopes for 3D simulation runs.

Every artifact emitted by ``sim`` declares its EMAD partition scope, schema
version, environment manifest pin, fidelity profile, and reproducibility seed.
"""
from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any

SCHEMA_SCENARIO = "emad.sim-scenario.v1"
SCHEMA_RUN_MANIFEST = "emad.sim-run-manifest.v1"
SCHEMA_TRUTH = "emad.sim-truth.v1"
SCHEMA_OBSERVATIONS = "emad.sim-observations.v1"
SCHEMA_EVALUATION = "emad.sim-evaluation.v1"
SCHEMA_REPLAY = "emad.sim-replay.v1"

SCOPE_EXERCISE_INPUT = "EXERCISE_INPUT"
SCOPE_EXERCISE_TRUTH = "EXERCISE_TRUTH"
SCOPE_EXERCISE_OBSERVATION = "EXERCISE_OBSERVATION"
SCOPE_EVALUATION = "EVALUATION"

FIDELITY_KINEMATIC_3DOF = "kinematic_3dof_drag_wind_v1"
SIM_ENGINE_VERSION = "emad-sim-py-0.2.0"


@dataclass(frozen=True)
class Vec3:
    """Local ENU vector in metres: (east, north, up)."""

    east: float
    north: float
    up: float

    def to_list(self) -> list[float]:
        return [round(float(self.east), 3), round(float(self.north), 3), round(float(self.up), 3)]

    def to_threejs(self) -> list[float]:
        """Convert ENU (east, north, up) to EMAD GLB / Three.js (east, up, -north)."""
        return [round(float(self.east), 3), round(float(self.up), 3), round(float(-self.north), 3)]


@dataclass(frozen=True)
class ThreatConfig:
    """Synthetic inbound aerial threat specification (EXERCISE_INPUT)."""

    id: str
    kind: str  # "one_way_uav", "cruise_missile", or "ballistic_missile"
    waypoints_enu: list[list[float]]  # [[east, north, alt_m], ...]
    altitude_mode: str = "agl"  # "agl" (above terrain surface) or "enu" (absolute local up)
    speed_mps: float = 55.0
    rcs_m2: float = 0.25
    spawn_s: float = 0.0
    mass_kg: float = 45.0
    ballistic_apogee_m: float = 1400.0  # peak AGL for ballistic_missile profile


@dataclass(frozen=True)
class RadarConfig:
    """Synthetic ground radar specification (EXERCISE_INPUT)."""

    id: str
    position_enu: list[float]  # [east, north, height_agl_m]
    max_range_m: float = 6500.0
    min_elevation_deg: float = -2.0
    max_elevation_deg: float = 75.0
    period_s: float = 0.5
    delay_s: float = 0.35
    range_noise_m: float = 18.0
    azimuth_noise_deg: float = 0.45
    elevation_noise_deg: float = 0.55
    base_detection_prob: float = 0.94
    dropout_every: int = 0
    include_buildings_in_los: bool = True


@dataclass(frozen=True)
class InterceptorSiteConfig:
    """Synthetic defensive interceptor launcher specification (EXERCISE_INPUT)."""

    id: str
    position_enu: list[float]  # [east, north, height_agl_m]
    speed_mps: float = 280.0
    max_accel_mps2: float = 120.0
    min_range_m: float = 350.0
    max_range_m: float = 6000.0
    launch_delay_s: float = 1.0
    proximity_fuze_m: float = 18.0
    protected_radius_m: float = 450.0


@dataclass(frozen=True)
class ShrapnelConfig:
    """Fragmentation cloud profile after intercept (EXERCISE_INPUT, synthetic/derived)."""

    fragment_count: int = 180
    burst_speed_min_mps: float = 35.0
    burst_speed_max_mps: float = 165.0
    momentum_retention: float = 0.55
    # Ballistic drag parameter k = 0.5 * Cd * A / m (m^2/kg) sampled log-uniformly
    # Heavy fragments (~0.002) fall almost ballistically; light skin panels (~0.06) drift strongly with wind.
    drag_param_min: float = 0.0025
    drag_param_max: float = 0.065
    interceptor_fragment_fraction: float = 0.22  # share of debris originating from interceptor kill-vehicle/booster
    hazardous_energy_j: float = 80.0  # RCC 321-style kinetic impact hazard threshold (Joules)
    dt_s: float = 0.1
    max_flight_s: float = 180.0


@dataclass(frozen=True)
class AtmosphereOverrideConfig:
    """Optional scenario wind modifier on top of retained ERA5 pressure fields."""

    valid_time: str | None = None
    known_at: str | None = None
    wind_scale: float = 1.0
    wind_add_enu_mps: list[float] = field(default_factory=lambda: [0.0, 0.0, 0.0])
    boundary_layer_alpha: float = 0.16


@dataclass(frozen=True)
class ScenarioSpec:
    """Complete frozen EXERCISE_INPUT scenario definition."""

    scenario_id: str
    title: str
    description: str
    environment_package: str
    expected_manifest_sha256: str | None
    weather_field_path: str | None  # optional external JSON if not embedded inside prepared.zip
    seed: int
    duration_s: float
    dt_s: float
    derived_building_height_m: float
    road_exposure_buffer_m: float
    threat: ThreatConfig
    radar: RadarConfig
    interceptor_site: InterceptorSiteConfig
    shrapnel: ShrapnelConfig
    atmosphere: AtmosphereOverrideConfig = field(default_factory=AtmosphereOverrideConfig)
    protected_zone_center_enu: list[float] | None = None
    protected_zone_radius_m: float = 450.0
    policy: str = "earliest"  # "earliest" or "safe_window"
    safe_window_preview_fragments: int = 48
    safe_window_horizon_s: float = 28.0
    safe_window_step_s: float = 1.5
    safe_window_building_weight: float = 4.0
    safe_window_road_weight: float = 1.5
    safe_window_protected_zone_weight: float = 10.0

    @classmethod
    def from_dict(cls, raw: dict[str, Any]) -> "ScenarioSpec":
        atm_raw = raw.get("atmosphere") or {}
        return cls(
            scenario_id=str(raw["scenarioId"]),
            title=str(raw.get("title", raw["scenarioId"])),
            description=str(raw.get("description", "")),
            environment_package=str(raw["environmentPackage"]),
            expected_manifest_sha256=raw.get("expectedManifestSha256"),
            weather_field_path=raw.get("weatherFieldPath"),
            seed=int(raw.get("seed", 42)),
            duration_s=float(raw.get("durationS", 90.0)),
            dt_s=float(raw.get("dtS", 0.1)),
            derived_building_height_m=float(raw.get("derivedBuildingHeightM", 10.0)),
            road_exposure_buffer_m=float(raw.get("roadExposureBufferM", 12.0)),
            threat=ThreatConfig(**raw["threat"]),
            radar=RadarConfig(**raw["radar"]),
            interceptor_site=InterceptorSiteConfig(**raw["interceptorSite"]),
            shrapnel=ShrapnelConfig(**raw["shrapnel"]),
            atmosphere=AtmosphereOverrideConfig(**atm_raw),
            protected_zone_center_enu=raw.get("protectedZoneCenterEnu"),
            protected_zone_radius_m=float(raw.get("protectedZoneRadiusM", 450.0)),
            policy=str(raw.get("policy", "earliest")),
            safe_window_preview_fragments=int(raw.get("safeWindowPreviewFragments", 48)),
            safe_window_horizon_s=float(raw.get("safeWindowHorizonS", 28.0)),
            safe_window_step_s=float(raw.get("safeWindowStepS", 1.5)),
            safe_window_building_weight=float(raw.get("safeWindowBuildingWeight", 4.0)),
            safe_window_road_weight=float(raw.get("safeWindowRoadWeight", 1.5)),
            safe_window_protected_zone_weight=float(raw.get("safeWindowProtectedZoneWeight", 10.0)),
        )

    def with_policy(self, policy: str) -> "ScenarioSpec":
        data = asdict(self)
        data["policy"] = policy
        return ScenarioSpec(
            scenario_id=data["scenario_id"],
            title=data["title"],
            description=data["description"],
            environment_package=data["environment_package"],
            expected_manifest_sha256=data["expected_manifest_sha256"],
            weather_field_path=data["weather_field_path"],
            seed=data["seed"],
            duration_s=data["duration_s"],
            dt_s=data["dt_s"],
            derived_building_height_m=data["derived_building_height_m"],
            road_exposure_buffer_m=data["road_exposure_buffer_m"],
            threat=self.threat,
            radar=self.radar,
            interceptor_site=self.interceptor_site,
            shrapnel=self.shrapnel,
            atmosphere=self.atmosphere,
            protected_zone_center_enu=data["protected_zone_center_enu"],
            protected_zone_radius_m=data["protected_zone_radius_m"],
            policy=policy,
            safe_window_preview_fragments=data["safe_window_preview_fragments"],
            safe_window_horizon_s=data["safe_window_horizon_s"],
            safe_window_step_s=data["safe_window_step_s"],
            safe_window_building_weight=data["safe_window_building_weight"],
            safe_window_road_weight=data["safe_window_road_weight"],
            safe_window_protected_zone_weight=data["safe_window_protected_zone_weight"],
        )

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)
