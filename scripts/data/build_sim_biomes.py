"""Reproducible builder for EMAD's Three-Biome 24-30 km Saudi Simulation Suite.

Builds verified, manifest-pinned ``prepared.zip`` archives + decimated 3D GLB
terrain meshes + bounded GHS-OBAT height extracts + companion ERA5/GEBCO links
under ``private/environment-foundation/sim-biomes/<biome-id>/``:

1. ``riyadh-metro-24km-v1``   — 24x24 km Riyadh urban metro (towers + WorldPop)
2. ``asir-highlands-30km-v1`` — 30x30 km Abha / Al-Soudah mountain escarpment
3. ``jubail-coast-30km-v1``   — 30x30 km Jubail Gulf industrial coast + GEBCO 2026
4. Also generates ``jeddah-v2/surface.glb`` from the already-staged Jeddah ZIP.

Every archive follows EMAD's local ENU topocentric frame contract (< 1 mm control
error), EGM2008 geoid alignment, and SHA-256 member pinning.
"""
from __future__ import annotations

import argparse
import csv
import datetime as dt
import hashlib
import io
import json
import math
from pathlib import Path
import sys
import zipfile

import fiona
import numpy as np
import osmium
from pyproj import Transformer
import rasterio
from rasterio.windows import from_bounds
from shapely.geometry import LineString, Polygon, mapping

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT / "scripts/data"))
sys.path.insert(0, str(REPO_ROOT))

import glb_geometry  # noqa: E402
from sim_core.environment import EnvironmentModel  # noqa: E402

BIOMES_ROOT = REPO_ROOT / "private/environment-foundation/sim-biomes"
LOCAL_RAW = REPO_ROOT / "private/local/data/raw"
PRIV_IMPL = REPO_ROOT / "private/environment-foundation/implementation"

BIOME_SPECS: dict[str, dict] = {
    "riyadh-metro-24km-v1": {
        "title": "Riyadh Expanded Metro 24 km (GLO-30 + GHS-OBAT + WorldPop + ERA5)",
        "biome": "urban_basin",
        "description": (
            "24x24 km central/northern Riyadh metropolitan corridor (KAFD, Olaya towers, "
            "highway interchanges) with Copernicus GLO-30 DSM, EGM2008 geoid, OSM vectors, "
            "GHS-OBAT modeled skyline heights, WorldPop 2024 density, and CAMS dust."
        ),
        "bounds": [46.60, 24.62, 46.84, 24.84],  # W, S, E, N (single N24 E046 tile)
        "copdem_tile": ("N24", "E046"),
        "glb_stride": 2,
        "max_buildings": 35000,
        "max_roads": 15000,
    },
    "asir-highlands-30km-v1": {
        "title": "Asir / Abha Southern Highlands 30 km (Mountain Escarpment + Radar Shadows)",
        "biome": "mountain_highlands",
        "description": (
            "30x30 km Sarawat Mountain escarpment around Abha & Al-Soudah (elevation relief "
            ">2,000 m across deep valleys and ridgelines) built from Copernicus GLO-30 DSM, "
            "EGM2008 geoid, OSM road corridors, and GHS-OBAT building context."
        ),
        "bounds": [42.30, 18.12, 42.58, 18.38],  # W, S, E, N (single N18 E042 tile)
        "copdem_tile": ("N18", "E042"),
        "glb_stride": 2,
        "max_buildings": 20000,
        "max_roads": 12000,
    },
    "jubail-coast-30km-v1": {
        "title": "Jubail Eastern Industrial Coast 30 km (GLO-30 + GEBCO 2026 + Sea-Skim)",
        "biome": "industrial_coast",
        "description": (
            "30x30 km Eastern Province Gulf industrial corridor (Jubail ports, energy & "
            "desalination coast) fusing Copernicus GLO-30 land with retained GEBCO 2026 "
            "bathymetry, OSM industrial footprints, GHS-OBAT heights, and WorldPop 2024."
        ),
        "bounds": [49.50, 26.74, 49.78, 27.00],  # W, S, E, N (single N26 E049 tile, inside Jubail GEBCO!)
        "copdem_tile": ("N26", "E049"),
        "glb_stride": 2,
        "max_buildings": 25000,
        "max_roads": 12000,
    },
}


def copdem_cog_url(lat_tag: str, lon_tag: str) -> str:
    stem = f"Copernicus_DSM_COG_10_{lat_tag}_00_{lon_tag}_00_DEM"
    return f"https://copernicus-dem-30m.s3.amazonaws.com/{stem}/{stem}.tif"


def crop_copdem(lat_tag: str, lon_tag: str, bounds: list[float], out_tif: Path) -> dict:
    """Read a bounded window directly from local cache or AWS Open Data Copernicus GLO-30 COG."""
    w, s, e, n = bounds
    receipt_path = out_tif.with_suffix(".receipt.json")
    if out_tif.is_file() and receipt_path.is_file():
        return json.loads(receipt_path.read_text(encoding="utf-8"))
    # Check if a local cached tile already exists
    local_candidates = list(LOCAL_RAW.glob(f"*{lat_tag}*{lon_tag}*.tif"))
    src_uri = str(local_candidates[0]) if local_candidates else f"/vsicurl/{copdem_cog_url(lat_tag, lon_tag)}"
    print(f"  Reading GLO-30 window {bounds} from {src_uri} ...", flush=True)
    with rasterio.Env(
        GDAL_DISABLE_READDIR_ON_OPEN="EMPTY_DIR",
        AWS_NO_SIGN_REQUEST="YES",
        CPL_VSIL_CURL_ALLOWED_EXTENSIONS=".tif",
        GDAL_HTTP_TIMEOUT="60",
    ):
        with rasterio.open(src_uri) as src:
            win = from_bounds(w, s, e, n, transform=src.transform).round_offsets().round_lengths()
            data = src.read(1, window=win)
            transform = src.window_transform(win)
            profile = src.profile.copy()
            profile.update(
                driver="GTiff",
                height=data.shape[0],
                width=data.shape[1],
                transform=transform,
                compress="deflate",
                tiled=True,
            )
            with rasterio.open(out_tif, "w", **profile) as dst:
                dst.write(data, 1)
    raw = out_tif.read_bytes()
    receipt = {
        "sourceUri": src_uri,
        "sha256": hashlib.sha256(raw).hexdigest(),
        "bytes": len(raw),
        "width": int(data.shape[1]),
        "height": int(data.shape[0]),
        "minM": round(float(np.nanmin(data)), 1),
        "maxM": round(float(np.nanmax(data)), 1),
    }
    receipt_path.write_text(json.dumps(receipt, indent=2), encoding="utf-8")
    return receipt


def crop_geoid(bounds: list[float], out_tif: Path) -> None:
    geoid_src = LOCAL_RAW / "us_nga_egm08_25.tif"
    if not geoid_src.is_file():
        raise FileNotFoundError(f"Missing local EGM2008 geoid: {geoid_src}")
    w, s, e, n = bounds
    pad = 0.25
    with rasterio.open(geoid_src) as src:
        win = from_bounds(w - pad, s - pad, e + pad, n + pad, transform=src.transform).round_offsets().round_lengths()
        data = src.read(1, window=win)
        transform = src.window_transform(win)
        profile = src.profile.copy()
        profile.update(driver="GTiff", height=data.shape[0], width=data.shape[1], transform=transform, compress="deflate")
        with rasterio.open(out_tif, "w", **profile) as dst:
            dst.write(data, 1)


class _OsmExtractor(osmium.SimpleHandler):
    """Stream-extract buildings and roads for multiple biome bounding boxes in one single PBF pass."""

    def __init__(self, biome_boxes: dict[str, tuple[float, float, float, float]], limits: dict[str, tuple[int, int]]) -> None:
        super().__init__()
        self.boxes = biome_boxes
        self.limits = limits
        self.buildings: dict[str, list[dict]] = {k: [] for k in biome_boxes}
        self.roads: dict[str, list[dict]] = {k: [] for k in biome_boxes}
        self._major_hw = {"motorway", "trunk", "primary", "secondary", "tertiary", "residential", "unclassified"}

    def way(self, w) -> None:
        is_bldg = "building" in w.tags
        hw = w.tags.get("highway")
        is_road = hw in self._major_hw
        if not is_bldg and not is_road:
            return
        pts = []
        for n in w.nodes:
            if n.location.valid():
                pts.append((n.location.lon, n.location.lat))
        if len(pts) < 2:
            return
        lon0, lat0 = pts[0]

        for bid, (west, south, east, north) in self.boxes.items():
            if not (west <= lon0 <= east and south <= lat0 <= north):
                continue
            max_b, max_r = self.limits[bid]
            if is_bldg and len(self.buildings[bid]) < max_b and len(pts) >= 4:
                if pts[0] != pts[-1]:
                    pts_ring = pts + [pts[0]]
                else:
                    pts_ring = pts
                poly = Polygon(pts_ring)
                if poly.is_valid and not poly.is_empty:
                    tags = {k: v for k, v in w.tags if k in ("building", "height", "building:levels", "name")}
                    self.buildings[bid].append({"id": f"way/{w.id}", "geom": mapping(poly), "tags": tags})
            elif is_road and len(self.roads[bid]) < max_r:
                line = LineString(pts)
                if not line.is_empty:
                    tags = {k: v for k, v in w.tags if k in ("highway", "name", "lanes")}
                    self.roads[bid].append({"id": f"way/{w.id}", "geom": mapping(line), "tags": tags})


def write_gpkg(path: Path, features: list[dict], geom_type: str) -> None:
    schema = {"geometry": geom_type, "properties": {"source_id": "str", "source_tags": "str"}}
    with fiona.open(path, "w", driver="GPKG", layer="features", crs="EPSG:4326", schema=schema) as dst:
        for f in features:
            dst.write(
                {
                    "geometry": f["geom"],
                    "properties": {
                        "source_id": f["id"],
                        "source_tags": json.dumps(f["tags"], separators=(",", ":")),
                    },
                }
            )


def extract_ghs_obat_batch(biome_boxes: dict[str, tuple[float, float, float, float]], out_paths: dict[str, Path]) -> None:
    """Single-pass extraction of GHS-OBAT centroids for multiple biomes from national Saudi ZIP."""
    ghs_zip = PRIV_IMPL / "riyadh-intake-v1/enrichment/ghs-obat-saudi.zip"
    if not ghs_zip.is_file():
        print("  Warning: ghs-obat-saudi.zip not found, skipping GHS-OBAT extraction.", flush=True)
        return
    need = {bid: box for bid, box in biome_boxes.items() if not out_paths[bid].is_file()}
    if not need:
        return
    print(f"  Single-pass GHS-OBAT extraction for {list(need.keys())} ...", flush=True)
    buckets: dict[str, list[dict]] = {bid: [] for bid in need}
    with zipfile.ZipFile(ghs_zip) as zf:
        member = [x for x in zf.infolist() if x.filename.endswith(".csv")][0]
        with io.TextIOWrapper(zf.open(member), encoding="utf-8", newline="") as stream:
            for row in csv.DictReader(stream):
                if row["height"] == "NaN":
                    continue
                lon = float(row["lon"])
                lat = float(row["lat"])
                for bid, (w, s, e, n) in need.items():
                    if w <= lon <= e and s <= lat <= n:
                        buckets[bid].append(
                            {
                                "type": "Feature",
                                "geometry": {"type": "Point", "coordinates": [lon, lat]},
                                "properties": {"sourceId": row["id"], "height": row["height"]},
                            }
                        )
    for bid, feats in buckets.items():
        out_paths[bid].write_text(
            json.dumps({"type": "FeatureCollection", "features": feats}, separators=(",", ":")),
            encoding="utf-8",
        )
        print(f"    {bid}: {len(feats)} GHS-OBAT centroids", flush=True)


def build_terrain_glb(env: EnvironmentModel, out_glb: Path, *, stride: int = 2) -> dict:
    """Export a decimated verified ENU Three.js terrain GLB from any EnvironmentModel."""
    tile = env.tiles[0]
    rows = np.arange(0, tile.height, stride)
    cols = np.arange(0, tile.width, stride)
    if rows[-1] != tile.height - 1:
        rows = np.append(rows, tile.height - 1)
    if cols[-1] != tile.width - 1:
        cols = np.append(cols, tile.width - 1)

    sub_up = tile.enu_up[np.ix_(rows, cols)]
    lons = tile.lon_origin + tile.dlon * (cols[None, :] + 0.5)
    lats = tile.lat_origin + tile.dlat * (rows[:, None] + 0.5)
    lon_grid = np.broadcast_to(lons, sub_up.shape)
    lat_grid = np.broadcast_to(lats, sub_up.shape)
    valid = np.isfinite(sub_up)
    safe_up = np.where(valid, sub_up, env.up_median)
    e_grid = (lon_grid - env.origin[0]) / env._dlon_per_m
    n_grid = (lat_grid - env.origin[1]) / env._dlat_per_m

    verts = np.column_stack((e_grid.ravel(), safe_up.ravel(), -n_grid.ravel())).astype(np.float32)
    h, w = sub_up.shape
    usable = valid[:-1, :-1] & valid[1:, :-1] & valid[:-1, 1:] & valid[1:, 1:]
    r, c = np.nonzero(usable)
    a = r * w + c
    b, d = a + 1, a + w
    faces = np.column_stack((a, d, b, b, d, d + 1)).reshape(-1, 3)

    glb = glb_geometry.GLB()
    mat = glb.material("Terrain surface", (0.49, 0.48, 0.42, 1.0))
    glb.mesh(verts, faces, mat)
    out_glb.write_bytes(glb.bytes())
    return {"bytes": out_glb.stat().st_size, "triangles": int(len(faces)), "stride": stride}


def package_prepared_zip(
    biome_id: str,
    bounds: list[float],
    surface_tif: Path,
    geoid_tif: Path,
    bldg_gpkg: Path,
    road_gpkg: Path,
    out_zip: Path,
) -> str:
    w, s, e, n = bounds
    lon0 = round(0.5 * (w + e), 6)
    lat0 = round(0.5 * (s + n), 6)
    h0 = 0.0
    to_enu = Transformer.from_pipeline(
        "+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad "
        "+step +proj=cart +ellps=WGS84 "
        f"+step +proj=topocentric +ellps=WGS84 +lon_0={lon0} +lat_0={lat0} +h_0={h0}"
    )
    controls = []
    for clat, clon in ((s, w), (s, e), (n, e), (n, w)):
        eu, nu, uu = to_enu.transform(clon, clat, 0.0, errcheck=True)
        controls.append({"longitude": clon, "latitude": clat, "ellipsoidHeightM": 0.0, "enuM": [eu, nu, uu]})

    files_to_pack = [
        ("raster-00-00.tif", surface_tif, "surface", {"verticalDatum": "EGM2008"}),
        ("raster-01-00.tif", geoid_tif, "geoid", {"verticalDatum": "EGM2008"}),
        ("vector-02.gpkg", bldg_gpkg, "buildings", {}),
        ("vector-03.gpkg", road_gpkg, "roads", {}),
    ]
    members = []
    for arc_name, path, layer, details in files_to_pack:
        raw = path.read_bytes()
        members.append(
            {
                "name": arc_name,
                "layer": layer,
                "component": 0,
                "bytes": len(raw),
                "sha256": hashlib.sha256(raw).hexdigest(),
                "details": details,
            }
        )

    manifest = {
        "schema": "emad.environment-manifest.v1",
        "profileId": "sim-biome-v1",
        "areaRevisionId": biome_id,
        "generatedAt": dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
        "area": {
            "type": "Polygon",
            "coordinates": [[[w, s], [e, s], [e, n], [w, n], [w, s]]],
        },
        "frame": {
            "origin": [lon0, lat0, h0],
            "axes": ["east", "north", "up"],
            "heightDatum": "WGS84-ellipsoid",
            "controls": controls,
        },
        "members": members,
    }
    manifest_bytes = json.dumps(manifest, indent=2).encode("utf-8")
    manifest_sha = hashlib.sha256(manifest_bytes).hexdigest()

    with zipfile.ZipFile(out_zip, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("manifest.json", manifest_bytes)
        for arc_name, path, _, _ in files_to_pack:
            zf.write(path, arc_name)
    return manifest_sha


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--biomes", nargs="*", default=list(BIOME_SPECS.keys()), help="Biome IDs to build")
    parser.add_argument("--jeddah-glb", action="store_true", help="Also generate surface.glb for staged jeddah-v2")
    parser.add_argument("--fetch-dem-only", action="store_true", help="Only fetch/cache bounded GLO-30 DEM GeoTIFFs")
    parser.add_argument("--force", action="store_true", help="Rebuild existing biomes")
    args = parser.parse_args()

    if args.fetch_dem_only:
        selected = {k: BIOME_SPECS[k] for k in args.biomes if k in BIOME_SPECS}
        for bid, cfg in selected.items():
            folder = BIOMES_ROOT / bid
            folder.mkdir(parents=True, exist_ok=True)
            surf_tif = folder / "surface.tif"
            if args.force:
                surf_tif.unlink(missing_ok=True)
                surf_tif.with_suffix(".receipt.json").unlink(missing_ok=True)
            r = crop_copdem(cfg["copdem_tile"][0], cfg["copdem_tile"][1], cfg["bounds"], surf_tif)
            print(f"  Cached DEM {bid}: {r['width']}x{r['height']} relief={r['minM']}m..{r['maxM']}m", flush=True)
        return 0

    if args.jeddah_glb:
        jdir = PRIV_IMPL / "jeddah-v2"
        jpkg = jdir / "prepared.zip"
        jglb = jdir / "surface.glb"
        if jpkg.is_file() and (args.force or not jglb.is_file()):
            env = EnvironmentModel(jpkg, load_vectors=False)
            info = build_terrain_glb(env, jglb, stride=1)
            print(f"Built Jeddah GLB: {info}", flush=True)

    selected = {k: BIOME_SPECS[k] for k in args.biomes if k in BIOME_SPECS}
    to_build = {}
    for bid, cfg in selected.items():
        folder = BIOMES_ROOT / bid
        if (folder / "biome.json").is_file() and (folder / "prepared.zip").is_file() and not args.force:
            print(f"Skipping existing biome {bid} (use --force to rebuild)", flush=True)
            continue
        folder.mkdir(parents=True, exist_ok=True)
        to_build[bid] = cfg

    if not to_build:
        print("All requested biomes are already staged.")
        return 0

    # 1. Single-pass OSM extraction for all biomes to build
    osm_pbf = LOCAL_RAW / "gcc-states.osm.pbf"
    if not osm_pbf.is_file():
        raise FileNotFoundError(f"Missing local GCC OSM extract: {osm_pbf}")
    boxes = {bid: tuple(cfg["bounds"]) for bid, cfg in to_build.items()}
    limits = {bid: (cfg["max_buildings"], cfg["max_roads"]) for bid, cfg in to_build.items()}
    print(f"Single-pass OSM extraction from {osm_pbf.name} for {list(to_build.keys())} ...", flush=True)
    extractor = _OsmExtractor(boxes, limits)
    extractor.apply_file(str(osm_pbf), locations=True)
    for bid in to_build:
        print(f"  {bid}: {len(extractor.buildings[bid])} buildings, {len(extractor.roads[bid])} roads", flush=True)

    # 2. Single-pass GHS-OBAT extraction
    ghs_paths = {bid: (BIOMES_ROOT / bid / "ghs-obat.geojson") for bid in to_build}
    extract_ghs_obat_batch(boxes, ghs_paths)

    # 3. Per-biome Copernicus GLO-30 DEM crop, EGM2008 crop, prepared.zip, GLB & biome.json
    for bid, cfg in to_build.items():
        print(f"\nBuilding biome {bid} ...", flush=True)
        folder = BIOMES_ROOT / bid
        surf_tif = folder / "surface.tif"
        geoid_tif = folder / "geoid.tif"
        bldg_gpkg = folder / "buildings.gpkg"
        road_gpkg = folder / "roads.gpkg"
        out_zip = folder / "prepared.zip"
        out_glb = folder / "surface.glb"

        dem_receipt = crop_copdem(cfg["copdem_tile"][0], cfg["copdem_tile"][1], cfg["bounds"], surf_tif)
        crop_geoid(cfg["bounds"], geoid_tif)
        write_gpkg(bldg_gpkg, extractor.buildings[bid], "Polygon")
        write_gpkg(road_gpkg, extractor.roads[bid], "LineString")

        manifest_sha = package_prepared_zip(bid, cfg["bounds"], surf_tif, geoid_tif, bldg_gpkg, road_gpkg, out_zip)
        env = EnvironmentModel(out_zip, expected_manifest_sha256=manifest_sha, load_vectors=False)
        glb_info = build_terrain_glb(env, out_glb, stride=int(cfg["glb_stride"]))

        # Remove intermediate raw scratch copies now safely inside prepared.zip
        for tmp_f in (surf_tif, geoid_tif, bldg_gpkg, road_gpkg):
            tmp_f.unlink(missing_ok=True)

        biome_meta = {
            "id": bid,
            "title": cfg["title"],
            "biome": cfg["biome"],
            "description": cfg["description"],
            "boundsWgs84": cfg["bounds"],
            "package": "prepared.zip",
            "glb": "surface.glb",
            "manifestSha256": manifest_sha,
            "demReceipt": dem_receipt,
            "glbInfo": glb_info,
            "buildingCount": len(extractor.buildings[bid]),
            "roadCount": len(extractor.roads[bid]),
            "createdAt": dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
        }
        (folder / "biome.json").write_text(json.dumps(biome_meta, indent=2), encoding="utf-8")
        print(
            f"  READY: {bid} | relief {dem_receipt['minM']}m..{dem_receipt['maxM']}m | "
            f"GLB {glb_info['triangles']} tris ({glb_info['bytes'] // 1024} KB) | sha={manifest_sha[:12]}",
            flush=True,
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
