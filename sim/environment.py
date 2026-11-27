"""Backwards-compatible facade re-exporting EMAD's unified ``sim_core.environment``."""
from sim_core.environment import BuildingRecord, EnvironmentModel, RoadRecord, SurfaceTile

__all__ = ["BuildingRecord", "EnvironmentModel", "RoadRecord", "SurfaceTile"]
