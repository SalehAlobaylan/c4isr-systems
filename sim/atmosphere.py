"""Backwards-compatible facade re-exporting EMAD's unified ``sim_core.atmosphere``."""
from sim_core.atmosphere import (
    AtmosphereModel,
    air_density_kg_m3,
    cams_dust_to_visibility_km,
    load_cams_dust,
    pressure_hpa_to_isa_altitude_m,
)

__all__ = [
    "AtmosphereModel",
    "air_density_kg_m3",
    "cams_dust_to_visibility_km",
    "load_cams_dust",
    "pressure_hpa_to_isa_altitude_m",
]
