"""4D Atmospheric Wind, Air Density, and CAMS Dust Extinction Layer.

Reads retained ``emad.era5-pressure-subset.v1`` weather fields (either embedded
inside a verified ``prepared.zip`` or provided as an external pinned JSON),
enforces ``known_at`` cutoff semantics, converts pressure levels (hPa) to
approximate geometric altitude using the International Standard Atmosphere
(explicitly tagged as derived/modeled), applies a neutral boundary-layer power
law below the lowest retained pressure level, optionally attaches CAMS dust
concentration (converting ug/m^3 -> optical visibility via Koschmieder
extinction), and falls back cleanly to a documented synthetic ISA regional
profile when an environment has no embedded ERA5 grid.
"""
from __future__ import annotations

from dataclasses import dataclass, field
import hashlib
import json
import math
from pathlib import Path
from typing import TYPE_CHECKING, Any
import zipfile

import numpy as np

if TYPE_CHECKING:
    from .environment import EnvironmentModel

# ISA constants for pressure-to-altitude conversion (derived/modeled approximation)
P0_HPA = 1013.25
T0_K = 288.15
LAPSE_K_PER_M = 0.0065
G0 = 9.80665
R_DRY = 287.058


@dataclass(frozen=True)
class AtmosphereOverrideConfig:
    valid_time: str | None = None
    known_at: str | None = None
    wind_scale: float = 1.0
    wind_add_enu_mps: list[float] = field(default_factory=lambda: [0.0, 0.0, 0.0])
    boundary_layer_alpha: float = 0.16


def pressure_hpa_to_isa_altitude_m(pressure_hpa: float) -> float:
    """Convert pressure level (hPa) to ISA geometric altitude (m MSL)."""
    ratio = max(float(pressure_hpa), 1.0) / P0_HPA
    exponent = (R_DRY * LAPSE_K_PER_M) / G0
    return (T0_K / LAPSE_K_PER_M) * (1.0 - (ratio ** exponent))


def air_density_kg_m3(altitude_m: np.ndarray, temp_k: np.ndarray | None = None) -> np.ndarray:
    """Approximate air density rho(z) in kg/m^3 via ISA pressure + local temperature."""
    alt = np.maximum(np.asarray(altitude_m, dtype=np.float64), -200.0)
    t_isa = np.maximum(T0_K - LAPSE_K_PER_M * alt, 216.65)
    p_pa = (P0_HPA * 100.0) * np.power(t_isa / T0_K, G0 / (R_DRY * LAPSE_K_PER_M))
    t_use = np.asarray(temp_k, dtype=np.float64) if temp_k is not None else t_isa
    t_use = np.maximum(t_use, 180.0)
    return p_pa / (R_DRY * t_use)


def cams_dust_to_visibility_km(dust_ug_m3: float) -> float:
    """Convert surface dust concentration (ug/m^3) to meteorological visibility via Koschmieder law.

    Empirical Saharan/Arabian mineral dust extinction coefficient ~ 0.00052 km^-1 per ug/m^3
    added to clear-air Rayleigh baseline (0.13 km^-1 -> ~30 km clear air).
    """
    c = max(0.0, float(dust_ug_m3))
    beta_km_inv = 0.13 + 0.00052 * c
    return round(max(0.4, min(30.0, 3.912 / beta_km_inv)), 2)


def load_cams_dust(path: str | Path | None) -> dict[str, Any] | None:
    if not path or not Path(path).is_file():
        return None
    doc = json.loads(Path(path).read_text(encoding="utf-8"))
    fields = doc.get("fields") or []
    if not fields:
        return None
    f0 = fields[0]
    val = float(f0.get("value", 0.0))
    return {
        "source": "derived/cams-koschmieder",
        "dustUgM3": round(val, 1),
        "derivedVisibilityKm": cams_dust_to_visibility_km(val),
        "validFrom": f0.get("validFrom"),
        "area": f0.get("area"),
    }


class AtmosphereModel:
    """Derived 3D wind, air-density & dust-extinction field from EMAD ERA5 pressure-level data."""

    def __init__(
        self,
        document: dict[str, Any],
        source_sha256: str,
        source_label: str,
        env: "EnvironmentModel",
        override: AtmosphereOverrideConfig | None = None,
        cams_dust: dict[str, Any] | None = None,
    ) -> None:
        if document.get("schema") != "emad.era5-pressure-subset.v1":
            raise ValueError(f"Unsupported weather field schema: {document.get('schema')}")

        self.document = document
        self.source_sha256 = source_sha256
        self.source_label = source_label
        self.env = env
        self.override = override or AtmosphereOverrideConfig()
        self.cams_dust = cams_dust

        valid_times: list[str] = list(document["validTimes"])
        self.valid_time = self.override.valid_time or valid_times[0]
        if self.valid_time not in valid_times:
            raise ValueError(f"Requested valid_time {self.valid_time} not in {valid_times}")

        # Enforce P06 known-at semantics: never use a future reanalysis slice before its valid instant
        if self.override.known_at is not None and self.override.known_at < self.valid_time:
            raise ValueError(
                f"Weather field validTime {self.valid_time} is unavailable at knownAt {self.override.known_at}"
            )

        self.lats = np.asarray(document["latitudes"], dtype=np.float64)
        self.lons = np.asarray(document["longitudes"], dtype=np.float64)
        self.pressures_hpa = np.asarray(document["pressureHPa"], dtype=np.float64)
        self.level_altitudes_m = np.asarray(
            [pressure_hpa_to_isa_altitude_m(p) for p in self.pressures_hpa], dtype=np.float64
        )

        # Reorder pressure levels ascending by altitude for linear interpolation
        order = np.argsort(self.level_altitudes_m)
        self.level_altitudes_m = self.level_altitudes_m[order]
        self.pressures_hpa = self.pressures_hpa[order]

        L = len(document["pressureHPa"])
        Y = len(self.lats)
        X = len(self.lons)
        t_idx = valid_times.index(self.valid_time)

        def extract_var(name: str) -> np.ndarray:
            samples = document["variables"][name]["samples"]
            above = document["aboveGroundMask"]
            arr = np.full((L, Y, X), np.nan, dtype=np.float64)
            for p_i in range(L):
                for y_i in range(Y):
                    for x_i in range(X):
                        flat = ((t_idx * L + p_i) * Y + y_i) * X + x_i
                        val = samples[flat]
                        if above[flat] and val is not None:
                            arr[p_i, y_i, x_i] = float(val)
            return arr[order]

        self.u_grid = extract_var("u_component_of_wind")
        self.v_grid = extract_var("v_component_of_wind")
        self.t_grid = extract_var("temperature")

        # Handle descending latitude axis (ERA5 stores north->south)
        self._lat_descending = bool(len(self.lats) > 1 and self.lats[0] > self.lats[-1])
        if self._lat_descending:
            self.lats = self.lats[::-1]
            self.u_grid = self.u_grid[:, ::-1, :]
            self.v_grid = self.v_grid[:, ::-1, :]
            self.t_grid = self.t_grid[:, ::-1, :]

        # Fill any masked cells per level with level median so local bilinear interpolation never yields NaN
        for p_i in range(L):
            for grid in (self.u_grid, self.v_grid, self.t_grid):
                layer = grid[p_i]
                if np.isnan(layer).any():
                    valid_vals = layer[np.isfinite(layer)]
                    fill = float(np.median(valid_vals)) if len(valid_vals) > 0 else 0.0
                    layer[np.isnan(layer)] = fill

    @classmethod
    def synthetic_regional_fallback(
        cls,
        env: "EnvironmentModel",
        *,
        override: AtmosphereOverrideConfig | None = None,
        cams_dust: dict[str, Any] | None = None,
        surface_wind_en: tuple[float, float] = (5.5, -2.5),
    ) -> "AtmosphereModel":
        """Create an explicitly labeled synthetic ISA + Shamal fallback when no local ERA5 file exists."""
        w, s, e, n = env.bounds_wgs84
        u0, v0 = surface_wind_en
        pressures = [1000, 850, 700]
        u_vals = [u0] * 4 + [u0 * 1.4] * 4 + [u0 * 1.9] * 4
        v_vals = [v0] * 4 + [v0 * 1.3] * 4 + [v0 * 1.7] * 4
        t_vals = [303.0] * 4 + [293.0] * 4 + [283.0] * 4
        doc = {
            "schema": "emad.era5-pressure-subset.v1",
            "edition": "synthetic/isa-shamal-fallback",
            "validTimes": ["2022-10-23T12:00:00Z"],
            "pressureHPa": pressures,
            "latitudes": [n, s],
            "longitudes": [w, e],
            "aboveGroundMask": [True] * 12,
            "variables": {
                "u_component_of_wind": {"samples": u_vals},
                "v_component_of_wind": {"samples": v_vals},
                "temperature": {"samples": t_vals},
            },
        }
        raw = json.dumps(doc, sort_keys=True).encode("utf-8")
        return cls(
            doc,
            hashlib.sha256(raw).hexdigest(),
            "synthetic/isa-shamal-fallback",
            env,
            override,
            cams_dust=cams_dust,
        )

    @classmethod
    def load(
        cls,
        env: "EnvironmentModel",
        *,
        external_path: str | Path | None = None,
        override: AtmosphereOverrideConfig | None = None,
        cams_dust_path: str | Path | None = None,
        allow_fallback: bool = True,
    ) -> "AtmosphereModel":
        """Load ERA5 weather field from embedded prepared.zip member, external path, or fallback."""
        cams = load_cams_dust(cams_dust_path)
        if external_path:
            p = Path(external_path)
            if p.is_file():
                raw = p.read_bytes()
                return cls(json.loads(raw), hashlib.sha256(raw).hexdigest(), str(p), env, override, cams_dust=cams)
            raise FileNotFoundError(f"Weather field file not found: {p}")

        for item in env.manifest.get("members", []):
            if item.get("layer") == "weather-field" and item["name"].endswith(".json"):
                with zipfile.ZipFile(env.package_path) as archive:
                    raw = archive.read(item["name"])
                return cls(
                    json.loads(raw),
                    hashlib.sha256(raw).hexdigest(),
                    f"{env.package_path}:{item['name']}",
                    env,
                    override,
                    cams_dust=cams,
                )

        if allow_fallback:
            return cls.synthetic_regional_fallback(env, override=override, cams_dust=cams)

        raise FileNotFoundError(
            "No embedded weather-field member found in prepared.zip and no external weatherFieldPath supplied"
        )

    def with_perturbation(self, add_en_mps: tuple[float, float], scale: float = 1.0) -> "AtmosphereModel":
        """Return a perturbed copy (used by Monte Carlo debris ensembles)."""
        cur = self.override.wind_add_enu_mps
        ov = AtmosphereOverrideConfig(
            valid_time=self.valid_time,
            known_at=self.override.known_at,
            wind_scale=self.override.wind_scale * float(scale),
            wind_add_enu_mps=[cur[0] + float(add_en_mps[0]), cur[1] + float(add_en_mps[1]), cur[2]],
            boundary_layer_alpha=self.override.boundary_layer_alpha,
        )
        return AtmosphereModel(
            self.document, self.source_sha256, self.source_label, self.env, ov, cams_dust=self.cams_dust
        )

    def _bilinear_level(self, grid_2d: np.ndarray, lons: np.ndarray, lats: np.ndarray) -> np.ndarray:
        if len(self.lons) == 1 or len(self.lats) == 1:
            return np.full(lons.shape, float(grid_2d[0, 0]), dtype=np.float64)

        fx = np.interp(lons, self.lons, np.arange(len(self.lons), dtype=np.float64))
        fy = np.interp(lats, self.lats, np.arange(len(self.lats), dtype=np.float64))
        c0 = np.clip(np.floor(fx).astype(int), 0, len(self.lons) - 2)
        r0 = np.clip(np.floor(fy).astype(int), 0, len(self.lats) - 2)
        tx = np.clip(fx - c0, 0.0, 1.0)
        ty = np.clip(fy - r0, 0.0, 1.0)
        return (
            (1.0 - tx) * (1.0 - ty) * grid_2d[r0, c0]
            + tx * (1.0 - ty) * grid_2d[r0, c0 + 1]
            + (1.0 - tx) * ty * grid_2d[r0 + 1, c0]
            + tx * ty * grid_2d[r0 + 1, c0 + 1]
        )

    def sample(
        self,
        east: np.ndarray,
        north: np.ndarray,
        up_m: np.ndarray,
        ground_up_m: np.ndarray | None = None,
    ) -> tuple[np.ndarray, np.ndarray, np.ndarray, np.ndarray, np.ndarray]:
        """Vectorized atmospheric query returning (wind_e, wind_n, wind_u, rho, temp_K)."""
        e_arr = np.atleast_1d(np.asarray(east, dtype=np.float64))
        n_arr = np.atleast_1d(np.asarray(north, dtype=np.float64))
        u_arr = np.atleast_1d(np.asarray(up_m, dtype=np.float64))
        shape = np.broadcast_shapes(e_arr.shape, n_arr.shape, u_arr.shape)
        e_b = np.broadcast_to(e_arr, shape).ravel()
        n_b = np.broadcast_to(n_arr, shape).ravel()
        z_b = np.broadcast_to(u_arr, shape).ravel()

        lons, lats = self.env.enu_to_lonlat_fast(e_b, n_b)
        n_levels = len(self.level_altitudes_m)

        u_levels = np.empty((n_levels, len(z_b)), dtype=np.float64)
        v_levels = np.empty((n_levels, len(z_b)), dtype=np.float64)
        t_levels = np.empty((n_levels, len(z_b)), dtype=np.float64)
        for p_i in range(n_levels):
            u_levels[p_i] = self._bilinear_level(self.u_grid[p_i], lons, lats)
            v_levels[p_i] = self._bilinear_level(self.v_grid[p_i], lons, lats)
            t_levels[p_i] = self._bilinear_level(self.t_grid[p_i], lons, lats)

        if n_levels == 1:
            wind_e = u_levels[0].copy()
            wind_n = v_levels[0].copy()
            temp_k = t_levels[0].copy()
        else:
            idx_hi = np.clip(np.searchsorted(self.level_altitudes_m, z_b, side="right"), 1, n_levels - 1)
            idx_lo = idx_hi - 1
            z_lo = self.level_altitudes_m[idx_lo]
            z_hi = self.level_altitudes_m[idx_hi]
            frac = np.clip((z_b - z_lo) / np.maximum(z_hi - z_lo, 1.0), 0.0, 1.0)
            cols = np.arange(len(z_b))
            wind_e = (1.0 - frac) * u_levels[idx_lo, cols] + frac * u_levels[idx_hi, cols]
            wind_n = (1.0 - frac) * v_levels[idx_lo, cols] + frac * v_levels[idx_hi, cols]
            temp_k = (1.0 - frac) * t_levels[idx_lo, cols] + frac * t_levels[idx_hi, cols]

        z_ref_msl = float(self.level_altitudes_m[0])
        below_ref = z_b < z_ref_msl
        if np.any(below_ref):
            if ground_up_m is not None:
                g_b = np.broadcast_to(np.atleast_1d(np.asarray(ground_up_m, dtype=np.float64)), shape).ravel()
            else:
                g_b = np.full_like(z_b, self.env.up_median)
            z_agl = np.maximum(z_b[below_ref] - g_b[below_ref], 5.0)
            ref_agl = np.maximum(z_ref_msl - g_b[below_ref], 50.0)
            factor = np.power(np.clip(z_agl / ref_agl, 0.02, 1.0), self.override.boundary_layer_alpha)
            wind_e[below_ref] *= factor
            wind_n[below_ref] *= factor
            temp_k[below_ref] = t_levels[0, below_ref] + LAPSE_K_PER_M * (z_ref_msl - z_b[below_ref])

        add_e, add_n, add_u = self.override.wind_add_enu_mps
        wind_e = wind_e * self.override.wind_scale + add_e
        wind_n = wind_n * self.override.wind_scale + add_n
        wind_u = np.full_like(wind_e, float(add_u))

        rho = air_density_kg_m3(z_b, temp_k)
        return (
            wind_e.reshape(shape),
            wind_n.reshape(shape),
            wind_u.reshape(shape),
            rho.reshape(shape),
            temp_k.reshape(shape),
        )

    def profile_summary(self, east: float = 0.0, north: float = 0.0) -> list[dict[str, Any]]:
        """Sample wind profile at key altitudes above terrain for HUD & 3D wind-arrow overlays."""
        ground = self.env.height_at(east, north, fallback=self.env.up_median)
        alts_agl = [50.0, 250.0, 600.0, 1200.0, 2000.0]
        ups = np.array([ground + a for a in alts_agl], dtype=np.float64)
        es = np.full_like(ups, east)
        ns = np.full_like(ups, north)
        we, wn, wu, rho, tk = self.sample(es, ns, ups, ground_up_m=np.full_like(ups, ground))
        out = []
        for i, agl in enumerate(alts_agl):
            spd = math.hypot(float(we[i]), float(wn[i]))
            out.append(
                {
                    "altitudeAglM": agl,
                    "upM": round(float(ups[i]), 1),
                    "windEnuMps": [round(float(we[i]), 2), round(float(wn[i]), 2), round(float(wu[i]), 2)],
                    "speedMps": round(spd, 2),
                    "airDensityKgM3": round(float(rho[i]), 4),
                    "temperatureK": round(float(tk[i]), 2),
                }
            )
        return out

    def metadata(self) -> dict[str, Any]:
        is_fallback = self.source_label == "synthetic/isa-shamal-fallback"
        is_modified = (
            abs(self.override.wind_scale - 1.0) > 1e-9
            or any(abs(v) > 1e-9 for v in self.override.wind_add_enu_mps)
        )
        derived_origin = (
            "synthetic/isa-shamal-fallback"
            if is_fallback
            else ("synthetic/derived-from-era5" if is_modified else "derived-from-era5")
        )
        return {
            "source": self.source_label,
            "sourceSha256": self.source_sha256,
            "edition": self.document.get("edition"),
            "validTime": self.valid_time,
            "knownAt": self.override.known_at,
            "pressureHPa": [float(p) for p in self.pressures_hpa],
            "derivedIsaAltitudesM": [round(float(a), 1) for a in self.level_altitudes_m],
            "verticalModel": "isa-hypsometric-plus-neutral-bl-power-law",
            "derivedOrigin": derived_origin,
            "physicalAccuracy": "UNKNOWN",
            "windScale": self.override.wind_scale,
            "windAddEnuMps": list(self.override.wind_add_enu_mps),
            "camsDust": self.cams_dust,
            "profileAtOrigin": self.profile_summary(0.0, 0.0),
        }
