# EMAD 3D Intercept & Shrapnel Exposure Engine (`sim/`, `v0.3.0`)

Reusable, deterministic 3-DOF simulation package operating on `sim_core/` verified
Saudi geographic biomes (`prepared.zip`, GHS-OBAT building heights, WorldPop 2024
population density, GEBCO bathymetry) and retained ERA5 / CAMS atmospheric fields.

## Architecture & Partition Discipline

Every simulation run preserves EMAD's four-partition boundary:

| Artifact | Partition Scope | Contents |
|---|---|---|
| `manifest.json` | `EXERCISE_INPUT` | Pinned environment manifest SHA-256, weather source SHA-256, seed, fidelity profile, scenario config |
| `truth.json` | `EXERCISE_TRUTH` | Sealed trajectories, exact geometric radar LOS, exact intercept point, exact shrapnel impacts |
| `observations.json` | `EXERCISE_OBSERVATION` | Imperfect terrain-masked radar detections (spherical noise + delay + HMAC-opaque contact IDs) and observation-only tracker estimates |
| `evaluation.json` / `comparison.json` | `EVALUATION` | Sensing/interception/shrapnel exposure metrics comparing baseline vs. optimizer policies |
| `replay.json` | Viewer bundle | Compact replay data + terrain/vector context for the offline Three.js 3D viewer |

## Reusable Primitives & v0.3 Capabilities

- **`sim_core/environment.py` (`EnvironmentModel`):** Loads verified `prepared.zip` rasters across all 6 Saudi biomes (Riyadh Pilot, Riyadh Metro 24 km, Jeddah Coast, Asir Highlands 30 km, Jubail Coast 30 km, and Owned Fixture) with EGM2008 geoid alignment, **GHS-OBAT modeled building heights**, **WorldPop 2024 population density**, **GEBCO 2026 bathymetry**, and adaptive vectorized `ObstructionGrid` LOS raycasting.
- **`sim_core/atmosphere.py` (`AtmosphereModel`):** Loads retained `emad.era5-pressure-subset.v1` weather grids plus **CAMS Dust Koschmieder optical extinction**, enforces `known_at` cutoffs, and returns vectorized 3D wind `(u, v, w)` and air density `rho(z)`.
- **`sim_core/traffic.py` & `sim_core/road_impact.py`:** Freezes live/historical `OpenSky` / `AIS` tracks (or deterministic Saudi civil-corridor fallbacks) into read-only `EXERCISE_INPUT` civilian airliners with `1,500 m × 450 m` `HOLD_FIRE` safety cylinders, and computes OSM road blockage km plus Dijkstra emergency-response detour penalties.
- **`sim/physics.py`:** Modular 3-DOF threat flight (AGL terrain-following UAV, cruise, and high-apogee ballistic), acceleration-limited proportional-lead interceptor pursuit, and numerically clamped vectorized RK2 shrapnel dispersion with quadratic aerodynamic drag and 3D wind drift.
- **`sim/sensors.py` & `sim_core/handoff.py`:** Synthetic terrain-masked radar (`EXERCISE_TRUTH -> EXERCISE_OBSERVATION`), alpha-beta observation-only tracker, and 1-click import of `emad.sensor-network.v1` layouts exported from `sensor_coverage/`.
- **`sim/engine.py`:** Deterministic end-to-end runner plus head-to-head policy comparison (`earliest` baseline vs. `safe_window` optimizing over buildings, OSM roads, protected zones, WorldPop population exposure, and civilian airspace safety, cross-checked by `< 0.5 ms` `derived/ml-debris-surrogate-v1` predictions).
- **`sim/viewer/`:** Zero-CDN offline Three.js replay viewer supporting EMAD's verified terrain GLB, Sentinel-2 optical drape, animated civilian airliners + translucent airspace safety cylinders, and WorldPop / road-impact KPIs.

## Quick Start

### Recommended: Map-to-3D Interactive Workflow (Zero CLI Flags)

`task dev` automatically starts the local 3D Simulation Service (`http://127.0.0.1:8765`) alongside the EMAD Map UI (`http://127.0.0.1:5173`). You can also start the simulation service standalone:

```powershell
task dev:sim
# or: .\.venv-recovery\Scripts\python.exe -m sim service --port 8765
```

Then in the EMAD Map UI (`http://127.0.0.1:5173/map`):
1. Click **Simulate** in the left map rail.
2. Select a verified environment (auto-discovered, e.g., **Riyadh Reference Area** — terrain GLB, Sentinel-2 drape, and ERA5 winds link automatically).
3. Click directly on the map to place **Radar**, **Launcher**, **Protected Zone**, and **Threat Waypoints**, adjust speed/wind/policy, and click **Run Simulation & Open 3D View**.
4. A new browser tab opens at `http://127.0.0.1:8765/?run=<jobId>` showing live progress and seamlessly launching the interactive 3D Three.js replay, while the 90% shrapnel dispersion ellipses also render back on your 2D map.

### CLI & Automated Tests

Use the repository's `.venv-recovery` Python environment (which provides `numpy`, `rasterio`, `fiona`, `shapely`, and `pyproj`):

```powershell
# 1. Run unit & end-to-end verification tests on the git-committed owned fixture
.\.venv-recovery\Scripts\python.exe -m unittest discover -s sim/tests -v

# 2. Run a policy comparison (Earliest vs Safe-Window) on the real Riyadh package
.\.venv-recovery\Scripts\python.exe -m sim compare sim/scenarios/riyadh_intercept.json --out tmp/sim-riyadh

# 3. Stress-test stronger crosswinds (e.g., 3x ERA5 wind or +12 m/s eastward Shamal wind)
.\.venv-recovery\Scripts\python.exe -m sim compare sim/scenarios/riyadh_intercept.json --out tmp/sim-riyadh-shamal --wind-add 12,-4,0

# 4. Launch the interactive 3D replay viewer over the verified Riyadh terrain GLB & Sentinel-2 imagery
.\.venv-recovery\Scripts\python.exe -m sim serve tmp/sim-riyadh `
  --glb private/environment-foundation/implementation/riyadh-observation-pilot/staged-surface.glb `
  --imagery private/environment-foundation/implementation/riyadh-observation-pilot/staged-imagery.json `
  --port 8765
```
