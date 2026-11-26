"""High-throughput in-memory 3D Terrain, LOS, and Exposure Engine.

Loads a verified EMAD prepared environment package (``prepared.zip``) directly
without rebuilding it, preserves the manifest SHA-256 and local ENU coordinate
frame, aligns surface elevations identically to ``prepare_display_terrain.py``
(including EGM2008 geoid undulation when declared), and builds fast in-memory
NumPy + Shapely spatial indexes for:

- vectorized terrain height lookup ``heights_at(east, north)``
- 3D terrain + optional derived-building line-of-sight raycasting
- exact ground / OSM building / OSM road impact classification
"""
from __future__ import annotations

from dataclasses import dataclass
import hashlib
import json
import math
from pathlib import Path
from typing import Any
import zipfile

import fiona
import numpy as np
from pyproj import Transformer
import rasterio
import shapely
from shapely import STRtree
from shapely.geometry import LineString, MultiLineString, MultiPolygon, Point, Polygon, shape


def _vsizip_path(package: Path, member_name: str) -> str:
    return "/vsizip/" + package.resolve().as_posix() + "/" + member_name


def _bilinear_geoid(geoid_path: str, lons: np.ndarray, lats: np.ndarray) -> np.ndarray:
    with rasterio.open(geoid_path) as grid:
        inv = ~grid.transform
        x = float(inv.c) + float(inv.a) * lons + float(inv.b) * lats
        y = float(inv.f) + float(inv.d) * lons + float(inv.e) * lats
        x = x - 0.5
        y = y - 0.5
        col = np.clip(np.floor(x).astype(int), 0, grid.width - 2)
        row = np.clip(np.floor(y).astype(int), 0, grid.height - 2)
        dx = np.clip(x - col, 0.0, 1.0)
        dy = np.clip(y - row, 0.0, 1.0)
        values = grid.read(1, masked=True).astype(np.float64)
        offset = (
            (1.0 - dy) * ((1.0 - dx) * values[row, col] + dx * values[row, col + 1])
            + dy * ((1.0 - dx) * values[row + 1, col] + dx * values[row + 1, col + 1])
        )
        if np.ma.getmaskarray(offset).any():
            raise ValueError("Retained geoid has missing samples inside surface footprint")
        scale = float(grid.scales[0]) if grid.scales else 1.0
        bias = float(grid.offsets[0]) if grid.offsets else 0.0
        return np.asarray(offset, dtype=np.float64) * scale + bias


@dataclass
class SurfaceTile:
    """One native surface raster projected into the package's local ENU frame."""

    member: str
    component: int
    vertical_datum: str
    width: int
    height: int
    # Exact north-up affine in geographic (lon, lat): lon = c + a*(col+0.5), lat = f + e*(row+0.5)
    lon_origin: float
    lat_origin: float
    dlon: float
    dlat: float  # negative for north-up
    # ENU up (metres) on the native pixel grid; NaN where masked/invalid
    enu_up: np.ndarray
    east_min: float
    east_max: float
    north_min: float
    north_max: float
    cell_size_m: float

    def sample_lonlat(self, lons: np.ndarray, lats: np.ndarray) -> np.ndarray:
        """Vectorized bilinear interpolation on native pixel centres."""
        fx = (lons - self.lon_origin) / self.dlon - 0.5
        fy = (lats - self.lat_origin) / self.dlat - 0.5

        out = np.full(lons.shape, np.nan, dtype=np.float64)
        in_bounds = (fx >= -0.5) & (fx <= self.width - 0.5) & (fy >= -0.5) & (fy <= self.height - 0.5)
        if not np.any(in_bounds):
            return out

        fxc = np.clip(fx[in_bounds], 0.0, self.width - 1.000001)
        fyc = np.clip(fy[in_bounds], 0.0, self.height - 1.000001)
        c0 = np.floor(fxc).astype(int)
        r0 = np.floor(fyc).astype(int)
        c1 = np.minimum(c0 + 1, self.width - 1)
        r1 = np.minimum(r0 + 1, self.height - 1)
        tx = fxc - c0
        ty = fyc - r0

        z00 = self.enu_up[r0, c0]
        z10 = self.enu_up[r0, c1]
        z01 = self.enu_up[r1, c0]
        z11 = self.enu_up[r1, c1]

        w00 = (1.0 - tx) * (1.0 - ty)
        w10 = tx * (1.0 - ty)
        w01 = (1.0 - tx) * ty
        w11 = tx * ty

        # Normalized weighted interpolation across valid corners so masked border
        # cells degrade gracefully to nearest valid neighbours without creating holes.
        v00 = np.isfinite(z00)
        v10 = np.isfinite(z10)
        v01 = np.isfinite(z01)
        v11 = np.isfinite(z11)
        w_sum = np.where(v00, w00, 0.0) + np.where(v10, w10, 0.0) + np.where(v01, w01, 0.0) + np.where(v11, w11, 0.0)
        z_sum = (
            np.where(v00, w00 * z00, 0.0)
            + np.where(v10, w10 * z10, 0.0)
            + np.where(v01, w01 * z01, 0.0)
            + np.where(v11, w11 * z11, 0.0)
        )
        valid = w_sum > 0.0
        vals = np.full(fxc.shape, np.nan, dtype=np.float64)
        vals[valid] = z_sum[valid] / w_sum[valid]
        out[in_bounds] = vals
        return out


@dataclass
class BuildingRecord:
    source_id: str
    geometry_enu: Polygon | MultiPolygon
    ground_up_m: float
    height_m: float
    height_origin: str  # "source-tag" or "synthetic/derived"
    roof_up_m: float


@dataclass
class RoadRecord:
    source_id: str
    geometry_enu: LineString | MultiLineString
    highway: str


class EnvironmentModel:
    """In-memory 3D environment built directly from a verified EMAD prepared.zip."""

    def __init__(
        self,
        package_path: str | Path,
        *,
        expected_manifest_sha256: str | None = None,
        derived_building_height_m: float = 10.0,
        road_buffer_m: float = 12.0,
        load_vectors: bool = True,
    ) -> None:
        self.package_path = Path(package_path)
        if not self.package_path.is_file():
            raise FileNotFoundError(f"Prepared environment package not found: {self.package_path}")

        self.derived_building_height_m = float(derived_building_height_m)
        self.road_buffer_m = float(road_buffer_m)

        with zipfile.ZipFile(self.package_path) as archive:
            manifest_raw = archive.read("manifest.json")
            self.manifest = json.loads(manifest_raw)
            self.manifest_sha256 = hashlib.sha256(manifest_raw).hexdigest()
            self.embedded_members = set(archive.namelist())

        if expected_manifest_sha256 and self.manifest_sha256 != expected_manifest_sha256:
            raise ValueError(
                f"Manifest SHA-256 mismatch: expected {expected_manifest_sha256}, got {self.manifest_sha256}"
            )

        self.frame = self.manifest["frame"]
        lon0, lat0, h0 = self.frame["origin"]
        self.origin = (float(lon0), float(lat0), float(h0))

        self._to_enu = Transformer.from_pipeline(
            "+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad "
            "+step +proj=cart +ellps=WGS84 "
            f"+step +proj=topocentric +ellps=WGS84 +lon_0={lon0} +lat_0={lat0} +h_0={h0}"
        )
        self._from_enu = Transformer.from_pipeline(
            "+proj=pipeline "
            f"+step +inv +proj=topocentric +ellps=WGS84 +lon_0={lon0} +lat_0={lat0} +h_0={h0} "
            "+step +inv +proj=cart +ellps=WGS84 "
            "+step +proj=unitconvert +xy_in=rad +xy_out=deg"
        )
        self.frame_control_max_error_m = self._verify_frame_controls()

        # Fast local tangent-plane linear Jacobian for ENU -> (lon, lat) lookups.
        # Verified against exact pyproj inverse at load time; sub-decimetre over 20 km.
        lon_e, _, _ = self._from_enu.transform(1000.0, 0.0, 0.0, errcheck=True)
        _, lat_n, _ = self._from_enu.transform(0.0, 1000.0, 0.0, errcheck=True)
        self._dlon_per_m = (float(lon_e) - self.origin[0]) / 1000.0
        self._dlat_per_m = (float(lat_n) - self.origin[1]) / 1000.0

        self.tiles: list[SurfaceTile] = []
        self._load_surface_tiles()
        if not self.tiles:
            raise ValueError("No surface terrain raster members found in prepared environment package")

        self.east_min = min(t.east_min for t in self.tiles)
        self.east_max = max(t.east_max for t in self.tiles)
        self.north_min = min(t.north_min for t in self.tiles)
        self.north_max = max(t.north_max for t in self.tiles)
        all_valid = np.concatenate([t.enu_up[np.isfinite(t.enu_up)] for t in self.tiles])
        self.up_min = float(np.min(all_valid))
        self.up_max = float(np.max(all_valid))
        self.up_median = float(np.median(all_valid))
        self.min_cell_size_m = min(t.cell_size_m for t in self.tiles)

        self.buildings: list[BuildingRecord] = []
        self.roads: list[RoadRecord] = []
        self._building_geoms: list[Any] = []
        self._building_tree: STRtree | None = None
        self._road_buffers: list[Any] = []
        self._road_tree: STRtree | None = None

        if load_vectors:
            self._load_vectors()

    def _verify_frame_controls(self) -> float:
        errors = []
        for ctrl in self.frame.get("controls", []):
            projected = self._to_enu.transform(
                ctrl["longitude"], ctrl["latitude"], ctrl["ellipsoidHeightM"], errcheck=True
            )
            errors.append(math.dist(projected, ctrl["enuM"]))
        max_err = max(errors) if errors else 0.0
        if max_err > 0.001:
            raise ValueError(f"Frame controls exceeded 1 mm tolerance: {max_err:.6f} m")
        return max_err

    def _load_surface_tiles(self) -> None:
        geoids_by_component: dict[int, str] = {}
        for item in self.manifest["members"]:
            if item["layer"] == "geoid" and item["name"].endswith(".tif"):
                geoids_by_component[int(item.get("component") or 0)] = _vsizip_path(
                    self.package_path, item["name"]
                )

        for item in self.manifest["members"]:
            if item["layer"] != "surface" or not item["name"].endswith(".tif"):
                continue
            comp = int(item.get("component") or 0)
            datum = (item.get("details") or {}).get("verticalDatum") or "ellipsoid"
            path = _vsizip_path(self.package_path, item["name"])
            with rasterio.open(path) as raster:
                affine = raster.transform
                if affine.b != 0 or affine.d != 0 or affine.a <= 0 or affine.e >= 0:
                    raise ValueError(f"Surface tile {item['name']} is not a north-up grid")
                raw = raster.read(1, masked=True).astype(np.float64)
                scale = float(raster.scales[0]) if raster.scales else 1.0
                offset = float(raster.offsets[0]) if raster.offsets else 0.0
                heights = np.asarray(raw, dtype=np.float64) * scale + offset
                valid = (~np.ma.getmaskarray(raw)) & np.isfinite(heights)
                rows, cols = np.indices(heights.shape)
                lons = float(affine.c) + float(affine.a) * (cols + 0.5)
                lats = float(affine.f) + float(affine.e) * (rows + 0.5)

                if datum == "EGM2008":
                    geoid = geoids_by_component.get(comp)
                    if geoid is None:
                        raise ValueError(f"EGM2008 surface tile {item['name']} missing companion geoid")
                    und = _bilinear_geoid(geoid, lons, lats)
                    ellipsoid_h = heights + und
                elif datum == "ellipsoid":
                    ellipsoid_h = heights
                else:
                    raise ValueError(f"Unsupported vertical datum: {datum}")

                safe_h = np.where(valid, ellipsoid_h, 0.0)
                east, north, up = self._to_enu.transform(lons, lats, safe_h, errcheck=True)
                enu_up = np.where(valid, up, np.nan)

                valid_e = east[valid]
                valid_n = north[valid]
                dx_m = abs(affine.a) / max(self._dlon_per_m, 1e-12)
                dy_m = abs(affine.e) / max(self._dlat_per_m, 1e-12)
                self.tiles.append(
                    SurfaceTile(
                        member=item["name"],
                        component=comp,
                        vertical_datum=datum,
                        width=raster.width,
                        height=raster.height,
                        lon_origin=float(affine.c),
                        lat_origin=float(affine.f),
                        dlon=float(affine.a),
                        dlat=float(affine.e),
                        enu_up=enu_up,
                        east_min=float(np.min(valid_e)),
                        east_max=float(np.max(valid_e)),
                        north_min=float(np.min(valid_n)),
                        north_max=float(np.max(valid_n)),
                        cell_size_m=float(min(dx_m, dy_m)),
                    )
                )

    def _lonlat_to_en_coords(self, coords: np.ndarray) -> np.ndarray:
        out = np.empty_like(coords, dtype=np.float64)
        out[:, 0] = (coords[:, 0] - self.origin[0]) / self._dlon_per_m
        out[:, 1] = (coords[:, 1] - self.origin[1]) / self._dlat_per_m
        if coords.shape[1] > 2:
            out[:, 2:] = coords[:, 2:]
        return out

    def _parse_building_height(self, tags: dict[str, Any]) -> tuple[float, str]:
        raw_h = tags.get("height")
        if raw_h is not None:
            text = str(raw_h).strip().lower().replace("m", "").strip()
            try:
                val = float(text)
                if 1.5 <= val <= 600.0:
                    return val, "source-tag"
            except ValueError:
                pass
        raw_levels = tags.get("building:levels")
        if raw_levels is not None:
            try:
                levels = float(str(raw_levels).strip())
                if 1.0 <= levels <= 150.0:
                    return levels * 3.3, "derived-from-levels"
            except ValueError:
                pass
        return self.derived_building_height_m, "synthetic/derived"

    def _load_vectors(self) -> None:
        for item in self.manifest["members"]:
            if not item["name"].endswith(".gpkg"):
                continue
            layer = item["layer"]
            if layer not in ("buildings", "roads"):
                continue
            path = _vsizip_path(self.package_path, item["name"])
            with fiona.open(path, driver="GPKG", layer="features") as src:
                for feat in src:
                    if feat.geometry is None:
                        continue
                    geom_ll = shape(feat.geometry)
                    if geom_ll.is_empty:
                        continue
                    geom_en = shapely.transform(geom_ll, self._lonlat_to_en_coords)
                    props = dict(feat.properties)
                    source_id = str(props.get("source_id") or feat.id)
                    tags = json.loads(props.get("source_tags") or "{}")
                    if layer == "buildings" and geom_en.geom_type in ("Polygon", "MultiPolygon"):
                        c = geom_en.centroid
                        ground_u = self.height_at(c.x, c.y)
                        if not math.isfinite(ground_u):
                            ground_u = self.up_median
                        h_m, origin_tag = self._parse_building_height(tags)
                        self.buildings.append(
                            BuildingRecord(
                                source_id=source_id,
                                geometry_enu=geom_en,
                                ground_up_m=ground_u,
                                height_m=h_m,
                                height_origin=origin_tag,
                                roof_up_m=ground_u + h_m,
                            )
                        )
                    elif layer == "roads" and geom_en.geom_type in ("LineString", "MultiLineString"):
                        self.roads.append(
                            RoadRecord(
                                source_id=source_id,
                                geometry_enu=geom_en,
                                highway=str(tags.get("highway") or "unclassified"),
                            )
                        )

        if self.buildings:
            self._building_geoms = [b.geometry_enu for b in self.buildings]
            self._building_tree = STRtree(self._building_geoms)
        if self.roads:
            self._road_buffers = [r.geometry_enu.buffer(self.road_buffer_m, quad_segs=2) for r in self.roads]
            self._road_tree = STRtree(self._road_buffers)

    # -- Coordinate conversions ------------------------------------------
    def enu_to_lonlat_fast(self, east: np.ndarray, north: np.ndarray) -> tuple[np.ndarray, np.ndarray]:
        return self.origin[0] + east * self._dlon_per_m, self.origin[1] + north * self._dlat_per_m

    def enu_to_geodetic(self, east: float, north: float, up: float) -> tuple[float, float, float]:
        lon, lat, h = self._from_enu.transform(east, north, up, errcheck=True)
        return float(lon), float(lat), float(h)

    def geodetic_to_enu(self, lon: float, lat: float, ellipsoid_h: float = 0.0) -> tuple[float, float, float]:
        e, n, u = self._to_enu.transform(lon, lat, ellipsoid_h, errcheck=True)
        return float(e), float(n), float(u)

    # -- Fast terrain height queries -------------------------------------
    def heights_at(self, east: np.ndarray, north: np.ndarray, *, fallback: float | None = None) -> np.ndarray:
        """Vectorized terrain surface ENU up (metres) lookup."""
        e_arr = np.asarray(east, dtype=np.float64)
        n_arr = np.asarray(north, dtype=np.float64)
        scalar = e_arr.ndim == 0 and n_arr.ndim == 0
        e_flat = np.atleast_1d(e_arr).ravel()
        n_flat = np.atleast_1d(n_arr).ravel()

        lons, lats = self.enu_to_lonlat_fast(e_flat, n_flat)
        out = np.full(e_flat.shape, np.nan, dtype=np.float64)
        for tile in self.tiles:
            missing = np.isnan(out)
            if not np.any(missing):
                break
            cand = (
                missing
                & (e_flat >= tile.east_min - tile.cell_size_m)
                & (e_flat <= tile.east_max + tile.cell_size_m)
                & (n_flat >= tile.north_min - tile.cell_size_m)
                & (n_flat <= tile.north_max + tile.cell_size_m)
            )
            if not np.any(cand):
                continue
            vals = tile.sample_lonlat(lons[cand], lats[cand])
            ok = np.isfinite(vals)
            idx = np.nonzero(cand)[0]
            out[idx[ok]] = vals[ok]

        if fallback is not None:
            out = np.where(np.isfinite(out), out, fallback)

        if scalar:
            return np.asarray(out[0], dtype=np.float64)
        return out.reshape(np.broadcast_shapes(e_arr.shape, n_arr.shape))

    def height_at(self, east: float, north: float, *, fallback: float | None = None) -> float:
        val = float(self.heights_at(np.asarray(east), np.asarray(north), fallback=fallback))
        return val

    # -- 3D Line-of-Sight raycasting -------------------------------------
    def line_of_sight(
        self,
        p0: tuple[float, float, float] | np.ndarray,
        p1: tuple[float, float, float] | np.ndarray,
        *,
        step_m: float | None = None,
        include_buildings: bool = True,
        clearance_m: float = 0.0,
    ) -> dict[str, Any]:
        """Check unobstructed 3D LOS from p0 to p1 against DSM terrain + optional buildings."""
        a = np.asarray(p0, dtype=np.float64)
        b = np.asarray(p1, dtype=np.float64)
        delta = b - a
        dist = float(np.linalg.norm(delta))
        if dist < 1e-3:
            return {"clear": True, "blockedBy": None, "distanceM": 0.0, "minClearanceM": float("inf")}

        step = float(step_m) if step_m is not None else max(10.0, self.min_cell_size_m * 0.5)
        n_steps = max(3, int(math.ceil(dist / step)))
        # Exclude exact endpoints (sensor mast & target) using interior fractions
        t = np.linspace(1.0 / n_steps, 1.0 - 1.0 / n_steps, max(1, n_steps - 1))
        pts = a[None, :] + t[:, None] * delta[None, :]

        terrain_u = self.heights_at(pts[:, 0], pts[:, 1])
        valid = np.isfinite(terrain_u)
        if np.any(valid):
            margins = pts[valid, 2] - (terrain_u[valid] + clearance_m)
            min_idx_valid = int(np.argmin(margins))
            min_clearance = float(margins[min_idx_valid])
            if min_clearance < 0.0:
                valid_indices = np.nonzero(valid)[0]
                hit_i = int(valid_indices[min_idx_valid])
                return {
                    "clear": False,
                    "blockedBy": "terrain",
                    "distanceM": round(dist, 2),
                    "blockDistanceM": round(float(t[hit_i] * dist), 2),
                    "blockPointEnu": [round(float(v), 2) for v in pts[hit_i]],
                    "minClearanceM": round(min_clearance, 2),
                }
        else:
            min_clearance = float("inf")

        if include_buildings and self._building_tree is not None:
            seg_2d = LineString([(float(a[0]), float(a[1])), (float(b[0]), float(b[1]))])
            hits = self._building_tree.query(seg_2d, predicate="intersects")
            if len(hits) > 0:
                horiz_dist = math.hypot(float(delta[0]), float(delta[1]))
                if horiz_dist > 1e-3:
                    for idx in hits:
                        bldg = self.buildings[int(idx)]
                        inter = seg_2d.intersection(bldg.geometry_enu)
                        if inter.is_empty:
                            continue
                        rep = inter.representative_point()
                        frac = math.hypot(rep.x - float(a[0]), rep.y - float(a[1])) / horiz_dist
                        frac = min(max(frac, 0.0), 1.0)
                        if frac <= 0.01 or frac >= 0.99:
                            continue
                        ray_u = float(a[2] + frac * delta[2])
                        if ray_u < bldg.roof_up_m + clearance_m:
                            return {
                                "clear": False,
                                "blockedBy": "building",
                                "buildingSourceId": bldg.source_id,
                                "distanceM": round(dist, 2),
                                "blockDistanceM": round(frac * dist, 2),
                                "blockPointEnu": [
                                    round(float(rep.x), 2),
                                    round(float(rep.y), 2),
                                    round(ray_u, 2),
                                ],
                                "minClearanceM": round(ray_u - bldg.roof_up_m, 2),
                            }

        return {
            "clear": True,
            "blockedBy": None,
            "distanceM": round(dist, 2),
            "minClearanceM": round(min_clearance, 2) if math.isfinite(min_clearance) else None,
        }

    # -- Impact exposure classification ----------------------------------
    def classify_impacts(
        self,
        east: np.ndarray,
        north: np.ndarray,
        *,
        protected_center: tuple[float, float] | None = None,
        protected_radius_m: float = 0.0,
    ) -> dict[str, Any]:
        """Classify N ground impact points against buildings, road buffers, and protected zone."""
        e_arr = np.asarray(east, dtype=np.float64).ravel()
        n_arr = np.asarray(north, dtype=np.float64).ravel()
        count = len(e_arr)

        on_building = np.zeros(count, dtype=bool)
        on_road = np.zeros(count, dtype=bool)
        in_protected = np.zeros(count, dtype=bool)
        building_ids: list[str | None] = [None] * count
        categories: list[str] = ["open"] * count

        points = [Point(float(e_arr[i]), float(n_arr[i])) for i in range(count)]

        if self._building_tree is not None and count > 0:
            # STRtree.query with array returns shape (2, k): [input_idx, tree_idx]
            pairs = self._building_tree.query(points, predicate="intersects")
            for pt_i, tree_i in zip(pairs[0], pairs[1]):
                pi = int(pt_i)
                if not on_building[pi]:
                    on_building[pi] = True
                    building_ids[pi] = self.buildings[int(tree_i)].source_id

        if self._road_tree is not None and count > 0:
            pairs = self._road_tree.query(points, predicate="intersects")
            for pt_i in pairs[0]:
                on_road[int(pt_i)] = True

        if protected_center is not None and protected_radius_m > 0.0 and count > 0:
            d2 = (e_arr - protected_center[0]) ** 2 + (n_arr - protected_center[1]) ** 2
            in_protected = d2 <= (protected_radius_m ** 2)

        for i in range(count):
            if in_protected[i]:
                categories[i] = "protected_zone"
            elif on_building[i]:
                categories[i] = "building"
            elif on_road[i]:
                categories[i] = "road"
            else:
                categories[i] = "open"

        return {
            "count": count,
            "on_building": on_building,
            "on_road": on_road,
            "in_protected": in_protected,
            "building_ids": building_ids,
            "categories": categories,
            "summary": {
                "total": count,
                "buildingHits": int(np.sum(on_building)),
                "roadHits": int(np.sum(on_road & (~on_building))),
                "protectedZoneHits": int(np.sum(in_protected)),
                "openHits": int(np.sum((~on_building) & (~on_road) & (~in_protected))),
                "urbanExposureFraction": round(
                    float(np.sum(on_building | on_road | in_protected)) / max(count, 1), 4
                ),
            },
        }

    # -- Metadata & lightweight 3D viewer export -------------------------
    def metadata(self) -> dict[str, Any]:
        return {
            "packagePath": str(self.package_path),
            "manifestSha256": self.manifest_sha256,
            "profileId": self.manifest.get("profileId"),
            "areaRevisionId": self.manifest.get("areaRevisionId"),
            "frame": {
                "origin": list(self.origin),
                "axes": self.frame.get("axes"),
                "heightDatum": self.frame.get("heightDatum"),
                "controlMaxErrorM": round(self.frame_control_max_error_m, 6),
            },
            "boundsEnu": {
                "eastMin": round(self.east_min, 2),
                "eastMax": round(self.east_max, 2),
                "northMin": round(self.north_min, 2),
                "northMax": round(self.north_max, 2),
                "upMin": round(self.up_min, 2),
                "upMax": round(self.up_max, 2),
            },
            "surfaceTiles": len(self.tiles),
            "buildingCount": len(self.buildings),
            "roadCount": len(self.roads),
            "derivedBuildingHeightPolicy": {
                "defaultHeightM": self.derived_building_height_m,
                "origin": "synthetic/derived when OSM height/levels absent",
                "physicalAccuracy": "UNKNOWN",
            },
        }

    def export_viewer_context(self, *, max_buildings: int = 2000, max_roads: int = 1200) -> dict[str, Any]:
        """Export compact ENU polygons/polylines and a coarse fallback heightfield for 3D rendering."""
        buildings_out = []
        for b in self.buildings[:max_buildings]:
            poly = b.geometry_enu.geoms[0] if b.geometry_enu.geom_type == "MultiPolygon" else b.geometry_enu
            coords = [[round(x, 1), round(y, 1)] for x, y in list(poly.exterior.coords)[::1]]
            if len(coords) >= 4:
                buildings_out.append(
                    {
                        "id": b.source_id,
                        "ring": coords,
                        "baseUp": round(b.ground_up_m, 2),
                        "heightM": round(b.height_m, 2),
                        "origin": b.height_origin,
                    }
                )

        roads_out = []
        # Prioritize longer / primary roads when downsampling
        sorted_roads = sorted(self.roads, key=lambda r: r.geometry_enu.length, reverse=True)
        for r in sorted_roads[:max_roads]:
            lines = list(r.geometry_enu.geoms) if r.geometry_enu.geom_type == "MultiLineString" else [r.geometry_enu]
            for line in lines:
                pts = list(line.coords)
                if len(pts) < 2:
                    continue
                e_arr = np.array([p[0] for p in pts], dtype=np.float64)
                n_arr = np.array([p[1] for p in pts], dtype=np.float64)
                u_arr = self.heights_at(e_arr, n_arr, fallback=self.up_median)
                roads_out.append(
                    {
                        "highway": r.highway,
                        "points": [
                            [round(float(e_arr[i]), 1), round(float(n_arr[i]), 1), round(float(u_arr[i]) + 1.2, 1)]
                            for i in range(len(pts))
                        ],
                    }
                )

        # Compact 64x64 fallback terrain grid in case no GLB is supplied to the viewer
        grid_n = 64
        eg = np.linspace(self.east_min, self.east_max, grid_n)
        ng = np.linspace(self.north_min, self.north_max, grid_n)
        EE, NN = np.meshgrid(eg, ng)
        UU = self.heights_at(EE, NN, fallback=self.up_median)
        return {
            "boundsEnu": self.metadata()["boundsEnu"],
            "origin": list(self.origin),
            "fallbackTerrainGrid": {
                "nx": grid_n,
                "ny": grid_n,
                "eastMin": round(self.east_min, 2),
                "eastMax": round(self.east_max, 2),
                "northMin": round(self.north_min, 2),
                "northMax": round(self.north_max, 2),
                "up": [round(float(v), 2) for v in UU.ravel()],
            },
            "buildings": buildings_out,
            "roads": roads_out,
        }
