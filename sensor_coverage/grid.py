"""Fast regular-grid obstruction surface + analysis grid built from a verified EMAD environment.

``ObstructionGrid`` resamples the verified DSM (already geoid-aligned by
``sim.environment.EnvironmentModel``) onto a regular local-ENU raster and burns
OSM building roofs (source-tag or explicitly ``synthetic/derived`` heights) on top,
so millions of line-of-sight samples per second can be answered by O(1) array
indexing instead of per-point polygon queries.

``AnalysisGrid`` is the coarser target-cell lattice over which coverage, blind
spots and optimizer objectives are evaluated.
"""
from __future__ import annotations

from dataclasses import dataclass
import math

import numpy as np
from rasterio.features import rasterize
from rasterio.transform import Affine

from sim.environment import EnvironmentModel


class ObstructionGrid:
    """Regular ENU raster of bare DSM terrain and DSM + building-roof obstruction surface."""

    def __init__(
        self,
        env: EnvironmentModel,
        *,
        cell_m: float | None = None,
        max_cells: int = 1_200_000,
    ) -> None:
        self.env = env
        span_e = env.east_max - env.east_min
        span_n = env.north_max - env.north_min
        auto_cell = math.sqrt(max(span_e * span_n, 1.0) / max_cells)
        self.cell_m = float(cell_m) if cell_m else float(max(8.0, auto_cell))
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
            # Burn in ascending roof order so the replace merge keeps the tallest roof per cell.
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
            roofs = burned[::-1].copy()  # row 0 = southernmost row, matching ``terrain``
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
    east: np.ndarray  # flattened cell-centre east (row-major, row 0 = south)
    north: np.ndarray
    ground_up: np.ndarray  # bare DSM surface up at cell centre (float32)
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
