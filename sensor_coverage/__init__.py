"""EMAD 3D Sensor Placement & Blind-Spot Optimizer subsystem.

Standalone simulation engine, local HTTP service, and Three.js 3D workbench
evaluating terrain- and building-masked 3D coverage across verified EMAD
environments (``prepared.zip`` + ERA5 atmosphere).
"""
from .contracts import (
    COV_ENGINE_VERSION,
    COVERAGE_REPLAY_SCHEMA,
    CoverageScenario,
    SENSOR_DEFAULTS,
    SENSOR_TYPES,
)
from .engine import run_coverage
from .service import CoverageService

__all__ = [
    "COV_ENGINE_VERSION",
    "COVERAGE_REPLAY_SCHEMA",
    "CoverageScenario",
    "CoverageService",
    "SENSOR_DEFAULTS",
    "SENSOR_TYPES",
    "run_coverage",
]
