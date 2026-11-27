"""Contracts for the EMAD Sensor Coverage & Blind-Spot Optimizer.

All request inputs are frozen as ``EXERCISE_INPUT`` and all computed coverage
maps, objectives, adversarial routes, and warning times are ``EVALUATION``
products. Every coverage surface is explicitly labelled ``synthetic/derived``
because it is a model computed from verified terrain + derived building
heights, never a surveyed sensor-performance truth.
"""
from __future__ import annotations

from dataclasses import asdict, dataclass, field, replace
import math
from typing import Any

COV_ENGINE_VERSION = "emad-cov-py-0.3.0"
COVERAGE_REPLAY_SCHEMA = "emad.coverage-replay.v1"
SCOPE_EXERCISE_INPUT = "EXERCISE_INPUT"
SCOPE_EVALUATION = "EVALUATION"
DERIVATION_LABEL = "synthetic/derived"

# Standard 4/3-earth radio refraction model for radar horizon / curvature drop.
EARTH_RADIUS_M = 6_371_000.0
REFRACTION_K = 4.0 / 3.0

SENSOR_TYPES = ("radar", "eo_ir", "acoustic")

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

    @property
    def is_directional(self) -> bool:
        return self.sector_width_deg < 359.5

    def moved_to(
        self,
        east: float,
        north: float,
        *,
        origin: str = "optimized",
        sector_center_deg: float | None = None,
    ) -> "SensorSpec":
        if sector_center_deg is not None:
            return replace(
                self,
                east=float(east),
                north=float(north),
                sector_center_deg=float(sector_center_deg) % 360.0,
                origin=origin,
            )
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

    def omni_profile_key(self) -> tuple[Any, ...]:
        """Position- and azimuth-independent signature for fast 8-heading sector sweeps."""
        return (
            self.type,
            round(self.mast_agl_m, 2),
            round(self.max_range_m, 1),
            round(self.min_elevation_deg, 2),
            round(self.clutter_floor_agl_m, 2),
            "omni",
        )


@dataclass(frozen=True)
class JammerSpec:
    """Stand-off electronic-warfare (EW) radar jammer specification (synthetic/derived)."""

    id: str
    east: float
    north: float
    agl_m: float = 150.0
    erp_ratio: float = 12.0  # Dimensionless J/S power ratio at 3 km reference range
    beam_width_deg: float = 34.0  # Mainlobe jammed azimuth cone width around sensor->jammer bearing
    label: str = "Stand-off EW Jammer"


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
    optimize_orientation: bool = True


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
    jammers: list[JammerSpec] = field(default_factory=list)
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


def jammer_range_scale(sensor: SensorSpec, az_rad: Any, jammers: list[JammerSpec] | None) -> Any:
    """Directional radar burn-through range scale factor (0..1] under stand-off EW jamming.

    Only radar receivers suffer RF jamming; EO/IR and acoustic sensors remain unaffected.
    Models mainlobe + low sidelobe J/S degradation: R' = R / (1 + sum_j J/S_j(az))^(1/4).
    """
    import numpy as np

    az = np.asarray(az_rad, dtype=np.float64)
    if sensor.type != "radar" or not jammers:
        return np.ones_like(az, dtype=np.float64)

    js_total = np.zeros_like(az, dtype=np.float64)
    for jam in jammers:
        dx = float(jam.east) - float(sensor.east)
        dy = float(jam.north) - float(sensor.north)
        dist = max(400.0, math.hypot(dx, dy))
        jam_az = math.atan2(dx, dy) % (2.0 * math.pi)
        diff = np.abs(np.angle(np.exp(1j * (az - jam_az))))
        half_rad = math.radians(max(6.0, float(jam.beam_width_deg)) * 0.5)
        # Smooth raised-cosine mainlobe + 6% sidelobe leakage
        mainlobe = np.where(diff <= half_rad, 0.5 * (1.0 + np.cos(math.pi * diff / half_rad)), 0.0)
        gain = 0.06 + 0.94 * mainlobe
        dist_factor = (3000.0 / dist) ** 2
        js_total += max(0.0, float(jam.erp_ratio)) * min(9.0, dist_factor) * gain
    return np.power(1.0 + js_total, -0.25)


def sector_mask(az_rad: Any, sensor: SensorSpec) -> Any:
    """Boolean mask of azimuths (radians from north, clockwise) inside the sensor sector."""
    return sector_mask_angles(az_rad, sensor.sector_center_deg, sensor.sector_width_deg)


def sector_mask_angles(az_rad: Any, center_deg: float, width_deg: float) -> Any:
    """Boolean mask of azimuths (radians from north, clockwise) inside a sector."""
    import numpy as np

    if width_deg >= 359.5:
        return np.ones(np.shape(az_rad), dtype=bool)
    center = math.radians(center_deg)
    half = math.radians(width_deg) / 2.0
    diff = np.angle(np.exp(1j * (np.asarray(az_rad) - center)))
    return np.abs(diff) <= half
