"""Fast regular-grid obstruction surface, vectorized batch LOS, and analysis grid.

``ObstructionGrid`` resamples a verified DSM onto a regular local-ENU raster
and burns OSM building roofs (source-tag, GHS-OBAT modeled, or explicitly
``synthetic/derived`` heights) on top so millions of 3D LOS samples per second
can be evaluated via O(1) NumPy indexing.
"""
from __future__ import annotations

from dataclasses import dataclass
import math
from typing import TYPE_CHECKING, Any

import numpy as np
from rasterio.features import rasterize
from rasterio.transform import Affine

if TYPE_CHECKING:
    from .environment import EnvironmentModel


class ObstructionGrid:
    """Regular ENU raster of bare DSM terrain and DSM + building-roof obstruction surface."""

    def __init__(
        self,
        env: "EnvironmentModel",
        *,
        cell_m: float | None = None,
        max_cells: int = 2_000_000,
    ) -> None:
        self.env = env
        span_e = env.east_max - env.east_min
        span_n = env.north_max - env.north_min
        auto_cell = math.sqrt(max(span_e * span_n, 1.0) / max_cells)
        self.cell_m = float(cell_m) if cell_m else float(max(8.0, math.ceil(auto_cell)))
        self.nx = max(2, int(math.ceil(span_e / self.cell_m)))
        self.ny = max(2, int(math.ceil(span_n / self.cell_m)))
        self.east0 = float(env.east_min)
        self.north0 = float(env.north_min)

        ee = self.east0 + (np.arange(self.nx) + 0.5) * self.cell_m
        nn = self.north0 + (np.arange(self.ny) + 0.5) * self.cell_m
        EE, NN = np.meshgrid(ee, nn)
        terrain = env.heights_at(EE, NN)
        self.valid = np.isfinite(terrain)
        self.terrain = np.where(self.valid, terrain, env.up_median).astype(np.float32)

        roofs = np.full((self.ny, self.nx), -1e9, dtype=np.float32)
        self.building_cells = 0
        if env.buildings:
            ordered = sorted(env.buildings, key=lambda b: b.roof_up_m)
            shapes = [(b.geometry_enu, float(b.roof_up_m)) for b in ordered]
            north_up = Affine(self.cell_m, 0.0, self.east0, 0.0, -self.cell_m, self.north0 + self.ny * self.cell_m)
            burned = rasterize(
                shapes,
                out_shape=(self.ny, self.nx),
                transform=north_up,
                fill=-1e9,
                dtype="float32",
                all_touched=False,
            )
            roofs = burned[::-1].copy()
            self.building_cells = int(np.sum(roofs > -1e8))
        self.surface = np.maximum(self.terrain, roofs).astype(np.float32)

    def _index(self, east: np.ndarray, north: np.ndarray) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
        fi = np.floor((np.asarray(east) - self.east0) / self.cell_m).astype(np.int64)
        fj = np.floor((np.asarray(north) - self.north0) / self.cell_m).astype(np.int64)
        inside = (fi >= 0) & (fi < self.nx) & (fj >= 0) & (fj < self.ny)
        return np.clip(fi, 0, self.nx - 1), np.clip(fj, 0, self.ny - 1), inside

    def sample(self, east: np.ndarray, north: np.ndarray, *, buildings: bool = True) -> tuple[np.ndarray, np.ndarray]:
        """Nearest-cell surface sample. Returns (heights, inside_mask)."""
        i, j, inside = self._index(east, north)
        src = self.surface if buildings else self.terrain
        return src[j, i], inside

    def height_at(self, east: float, north: float, *, buildings: bool = True) -> float:
        vals, _ = self.sample(np.asarray([east]), np.asarray([north]), buildings=buildings)
        return float(vals[0])

    def line_of_sight_fast(
        self,
        p0: tuple[float, float, float] | np.ndarray,
        p1: tuple[float, float, float] | np.ndarray,
        *,
        include_buildings: bool = True,
        clearance_m: float = 0.0,
        step_m: float | None = None,
    ) -> dict[str, Any]:
        """Ultra-fast O(N) NumPy raster LOS check (~20-50x faster than Shapely STRtree)."""
        a = np.asarray(p0, dtype=np.float64)
        b = np.asarray(p1, dtype=np.float64)
        delta = b - a
        dist = float(np.linalg.norm(delta))
        if dist < 1e-3:
            return {"clear": True, "blockedBy": None, "distanceM": 0.0, "minClearanceM": float("inf")}

        step = float(step_m) if step_m else self.cell_m
        n_steps = max(3, int(math.ceil(dist / step)))
        t = np.linspace(1.0 / n_steps, 1.0 - 1.0 / n_steps, max(1, n_steps - 1))
        pe = a[0] + t * delta[0]
        pn = a[1] + t * delta[1]
        pu = a[2] + t * delta[2]

        terr, inside = self.sample(pe, pn, buildings=False)
        surf, _ = self.sample(pe, pn, buildings=include_buildings)
        valid = inside
        if not np.any(valid):
            return {"clear": True, "blockedBy": None, "distanceM": round(dist, 2), "minClearanceM": None}

        margins = pu[valid] - (surf[valid].astype(np.float64) + clearance_m)
        min_idx = int(np.argmin(margins))
        min_clr = float(margins[min_idx])
        if min_clr < 0.0:
            v_idx = np.nonzero(valid)[0]
            # First hit along the ray from sensor p0
            first_hit_local = int(np.nonzero(margins < 0.0)[0][0])
            hi = int(v_idx[first_hit_local])
            is_bldg = include_buildings and (float(surf[hi]) - float(terr[hi]) > 1.0)
            return {
                "clear": False,
                "blockedBy": "building" if is_bldg else "terrain",
                "distanceM": round(dist, 2),
                "blockDistanceM": round(float(t[hi] * dist), 2),
                "blockPointEnu": [round(float(pe[hi]), 2), round(float(pn[hi]), 2), round(float(surf[hi]), 2)],
                "minClearanceM": round(min_clr, 2),
            }
        return {
            "clear": True,
            "blockedBy": None,
            "distanceM": round(dist, 2),
            "minClearanceM": round(min_clr, 2),
        }

    def metadata(self) -> dict[str, float | int]:
        return {
            "cellM": round(self.cell_m, 2),
            "nx": self.nx,
            "ny": self.ny,
            "buildingCells": self.building_cells,
            "validFraction": round(float(np.mean(self.valid)), 4),
        }


@dataclass
class AnalysisGrid:
    east0: float
    north0: float
    cell_m: float
    nx: int
    ny: int
    east: np.ndarray
    north: np.ndarray
    ground_up: np.ndarray
    valid: np.ndarray

    @property
    def count(self) -> int:
        return self.nx * self.ny

    @property
    def cell_area_km2(self) -> float:
        return (self.cell_m * self.cell_m) / 1e6

    @classmethod
    def build(cls, obs: ObstructionGrid, *, cell_m: float | None = None, max_cells: int = 6500) -> "AnalysisGrid":
        env = obs.env
        span_e = env.east_max - env.east_min
        span_n = env.north_max - env.north_min
        auto_cell = math.sqrt(max(span_e * span_n, 1.0) / max_cells)
        cell = float(cell_m) if cell_m else float(max(25.0, math.ceil(auto_cell / 5.0) * 5.0))
        nx = max(2, int(math.ceil(span_e / cell)))
        ny = max(2, int(math.ceil(span_n / cell)))
        ee = env.east_min + (np.arange(nx) + 0.5) * cell
        nn = env.north_min + (np.arange(ny) + 0.5) * cell
        EE, NN = np.meshgrid(ee, nn)
        e_flat = EE.ravel()
        n_flat = NN.ravel()
        ground, inside = obs.sample(e_flat, n_flat, buildings=False)
        oi, oj, _ = obs._index(e_flat, n_flat)
        valid = inside & obs.valid[oj, oi]
        return cls(
            east0=float(env.east_min),
            north0=float(env.north_min),
            cell_m=cell,
            nx=nx,
            ny=ny,
            east=e_flat,
            north=n_flat,
            ground_up=ground.astype(np.float32),
            valid=valid,
        )

    def nearest_index(self, east: np.ndarray, north: np.ndarray) -> np.ndarray:
        fi = np.clip(np.floor((np.asarray(east) - self.east0) / self.cell_m).astype(np.int64), 0, self.nx - 1)
        fj = np.clip(np.floor((np.asarray(north) - self.north0) / self.cell_m).astype(np.int64), 0, self.ny - 1)
        return fj * self.nx + fi

    def metadata(self) -> dict[str, float | int]:
        return {
            "eastMin": round(self.east0, 2),
            "northMin": round(self.north0, 2),
            "cellM": round(self.cell_m, 2),
            "nx": self.nx,
            "ny": self.ny,
            "validCells": int(np.sum(self.valid)),
        }
