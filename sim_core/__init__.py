"""EMAD Shared 3D Simulation Kernel (``sim_core``).

Provides reusable, verified runtime primitives shared across all EMAD
simulation workbenches (``sim``, ``sensor_coverage``, ``salvo_defense``,
and ``sim_bench``):

- ``EnvironmentModel`` with manifest SHA-256 pinning, EGM2008 geoid alignment,
  GHS-OBAT modeled building height fusion, windowed WorldPop 2024 population
  exposure, and GEBCO 2026 coastal bathymetry.
- ``ObstructionGrid`` & ``AnalysisGrid`` for high-throughput vectorized 3D LOS.
- ``AtmosphereModel`` with ERA5 4D pressure-level winds, CAMS dust extinction,
  and strict ``known_at`` cutoff enforcement.
- ``BiomePreset`` / ``discover_biomes`` unified environment discovery.
- ``SensorNetworkStore`` cross-simulation sensor-layout handoff.
- ``ExclusiveThreadingHTTPServer`` & loopback HTTP helpers.
"""
from .atmosphere import AtmosphereModel, AtmosphereOverrideConfig, cams_dust_to_visibility_km
from .environment import BuildingRecord, EnvironmentModel, RoadRecord, SurfaceTile
from .handoff import SENSOR_NETWORK_SCHEMA, SensorNetworkStore
from .obstruction import AnalysisGrid, ObstructionGrid
from .registry import BiomePreset, discover_biomes

__all__ = [
    "AnalysisGrid",
    "AtmosphereModel",
    "AtmosphereOverrideConfig",
    "BiomePreset",
    "BuildingRecord",
    "EnvironmentModel",
    "ObstructionGrid",
    "RoadRecord",
    "SENSOR_NETWORK_SCHEMA",
    "SensorNetworkStore",
    "SurfaceTile",
    "cams_dust_to_visibility_km",
    "discover_biomes",
]
