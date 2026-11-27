"""Contracts for the EMAD Sensor Coverage & Blind-Spot Optimizer.

All request inputs are frozen as ``EXERCISE_INPUT`` and all computed coverage
maps, objectives, and route warning times are ``EVALUATION`` products. Every
coverage surface is explicitly labelled ``synthetic/derived`` because it is a
model computed from verified terrain + derived building heights, never a
surveyed sensor-performance truth.
"""
from __future__ import annotations

from dataclasses import asdict, dataclass, field, replace
import math
from typing import Any

COV_ENGINE_VERSION = "emad-cov-py-0.2.0"
COVERAGE_REPLAY_SCHEMA = "emad.coverage-replay.v1"
SCOPE_EXERCISE_INPUT = "EXERCISE_INPUT"
SCOPE_EVALUATION = "EVALUATION"
DERIVATION_LABEL = "synthetic/derived"

# Standard 4/3-earth radio refraction model for radar horizon / curvature drop.
EARTH_RADIUS_M = 6_371_000.0
REFRACTION_K = 4.0 / 3.0

SENSOR_TYPES = ("radar", "eo_ir", "acoustic")

# Per-type physical defaults. ``maxRangeM`` is interpreted as:
#  - radar: instrumented detection range against a 1 m^2 reference target (R^4 law scaling)
#  - eo_ir: clear-air recognition range (further limited by meteorological visibility)
#  - acoustic: calm-air detection range (reduced by surface wind noise; no LOS needed)
SENSOR_DEFAULTS: dict[str, dict[str, Any]] = {
    "radar": {
        "mastAglM": 15.0,
        "maxRangeM": 9000.0,
        "minElevationDeg": -1.0,
        "clutterFloorAglM": 10.0,
        "requiresLos": True,
        "buildingsBlock": True,
    },
    "eo_ir": {
        "mastAglM": 10.0,
        "maxRangeM": 5000.0,
        "minElevationDeg": -6.0,
        "clutterFloorAglM": 0.0,
        "requiresLos": True,
        "buildingsBlock": True,
    },
    "acoustic": {
        "mastAglM": 4.0,
        "maxRangeM": 1500.0,
        "minElevationDeg": -90.0,
        "clutterFloorAglM": 0.0,
        "requiresLos": False,
        "buildingsBlock": False,
    },
}

DEFAULT_LAYERS: list[dict[str, float]] = [
    {"aglM": 30.0, "weight": 0.35},
    {"aglM": 60.0, "weight": 0.30},
    {"aglM": 120.0, "weight": 0.20},
    {"aglM": 300.0, "weight": 0.15},
]


def curvature_drop_m(distance_m: Any) -> Any:
    """Apparent drop of a straight radio ray over the 4/3-earth at horizontal distance ``d``."""
    return (distance_m * distance_m) / (2.0 * REFRACTION_K * EARTH_RADIUS_M)


@dataclass(frozen=True)
class SensorSpec:
    id: str
    type: str
    east: float
    north: float
    mast_agl_m: float
    max_range_m: float
    min_elevation_deg: float
    clutter_floor_agl_m: float
    sector_center_deg: float = 0.0
    sector_width_deg: float = 360.0
    locked: bool = False
    label: str = ""
    origin: str = "baseline"  # baseline | optimized | extra

    @property
    def requires_los(self) -> bool:
        return bool(SENSOR_DEFAULTS[self.type]["requiresLos"])

    @property
    def buildings_block(self) -> bool:
        return bool(SENSOR_DEFAULTS[self.type]["buildingsBlock"])

    def moved_to(self, east: float, north: float, *, origin: str = "optimized") -> "SensorSpec":
        return replace(self, east=float(east), north=float(north), origin=origin)

    def profile_key(self) -> tuple[Any, ...]:
        """Position-independent signature used to share candidate viewshed caches."""
        return (
            self.type,
            round(self.mast_agl_m, 2),
            round(self.max_range_m, 1),
            round(self.min_elevation_deg, 2),
            round(self.clutter_floor_agl_m, 2),
            round(self.sector_center_deg, 1),
            round(self.sector_width_deg, 1),
        )


@dataclass(frozen=True)
class AssetSpec:
    id: str
    east: float
    north: float
    warning_radius_m: float
    priority: float = 4.0
    label: str = "Defended asset"


@dataclass(frozen=True)
class RouteSpec:
    id: str
    points_enu: list[list[float]]  # [east, north, aglM]
    speed_mps: float
    label: str = "Ingress route"


@dataclass(frozen=True)
class LayerSpec:
    agl_m: float
    weight: float


@dataclass(frozen=True)
class ThreatSpec:
    rcs_m2: float = 0.1
    label: str = "Small UAV"


@dataclass(frozen=True)
class WeatherSpec:
    visibility_km: float = 10.0
    surface_wind_mps: float = 0.0
    wind_source: str = "ERA5 pressure-level profile (50 m AGL at origin)"


@dataclass(frozen=True)
class OptimizerSpec:
    enabled: bool = True
    extra_sensors: int = 0
    extra_sensor_type: str = "radar"
    min_separation_m: float = 600.0
    allow_rooftops: bool = True
    candidate_spacing_m: float | None = None
    redundancy_weight: float = 0.15
    route_weight: float = 2.5
    swap_passes: int = 2


@dataclass(frozen=True)
class CoverageScenario:
    scenario_id: str
    title: str
    sensors: list[SensorSpec]
    assets: list[AssetSpec]
    routes: list[RouteSpec]
    layers: list[LayerSpec]
    threat: ThreatSpec
    weather: WeatherSpec
    optimizer: OptimizerSpec
    include_buildings: bool = True
    analysis_cell_m: float | None = None
    extra: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


def effective_range_m(sensor: SensorSpec, threat: ThreatSpec, weather: WeatherSpec) -> float:
    """Physics-scaled detection range for this sensor against this threat in this weather."""
    if sensor.type == "radar":
        # Radar equation: detection range scales with RCS^(1/4) relative to 1 m^2 reference.
        return float(sensor.max_range_m * (max(threat.rcs_m2, 1e-4) / 1.0) ** 0.25)
    if sensor.type == "eo_ir":
        size_factor = min(1.2, max(0.5, (max(threat.rcs_m2, 1e-4) / 0.1) ** 0.25))
        vis_limit = max(200.0, weather.visibility_km * 1000.0 * 0.9)
        return float(min(sensor.max_range_m * size_factor, vis_limit))
    if sensor.type == "acoustic":
        wind_factor = max(0.35, 1.0 - weather.surface_wind_mps / 20.0)
        return float(sensor.max_range_m * wind_factor)
    raise ValueError(f"Unsupported sensor type: {sensor.type}")


def sector_mask(az_rad: Any, sensor: SensorSpec) -> Any:
    """Boolean mask of azimuths (radians from north, clockwise) inside the sensor sector."""
    import numpy as np

    if sensor.sector_width_deg >= 359.9:
        return np.ones(np.shape(az_rad), dtype=bool)
    center = math.radians(sensor.sector_center_deg)
    half = math.radians(sensor.sector_width_deg) / 2.0
    diff = np.angle(np.exp(1j * (np.asarray(az_rad) - center)))
    return np.abs(diff) <= half
