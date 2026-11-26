# EMAD 3D Simulation Core (`sim/`)

Reusable, deterministic 3-DOF simulation package operating directly on verified
EMAD geographic environment packages (`prepared.zip`) and retained ERA5
pressure-level wind fields (`emad.era5-pressure-subset.v1`).

## Architecture & Partition Discipline

Every simulation run preserves EMAD's four-partition boundary:

| Artifact | Partition Scope | Contents |
|---|---|---|
| `manifest.json` | `EXERCISE_INPUT` | Pinned environment manifest SHA-256, weather source SHA-256, seed, fidelity profile, scenario config |
| `truth.json` | `EXERCISE_TRUTH` | Sealed trajectories, exact geometric radar LOS, exact intercept point, exact shrapnel impacts |
| `observations.json` | `EXERCISE_OBSERVATION` | Imperfect terrain-masked radar detections (spherical noise + delay + HMAC-opaque contact IDs) and observation-only tracker estimates |
| `evaluation.json` / `comparison.json` | `EVALUATION` | Sensing/interception/shrapnel exposure metrics comparing baseline vs. optimizer policies |
| `replay.json` | Viewer bundle | Compact replay data + terrain/vector context for the offline Three.js 3D viewer |

## Reusable Primitives

- **`sim/environment.py` (`EnvironmentModel`):** Loads verified `prepared.zip` rasters (with EGM2008 geoid undulation and exact `pyproj` ENU alignment matching `prepare_display_terrain.py`), provides vectorized NumPy elevation lookups, 3D terrain + derived-building line-of-sight raycasting, and Shapely `STRtree` impact exposure classification.
- **`sim/atmosphere.py` (`AtmosphereModel`):** Loads retained `emad.era5-pressure-subset.v1` weather grids (embedded or external), enforces `known_at` cutoffs, maps pressure levels to ISA altitudes with neutral boundary-layer shear, and returns vectorized 3D wind `(u, v, w)` and air density `rho(z)`.
- **`sim/physics.py`:** Modular 3-DOF threat flight (AGL terrain-following), acceleration-limited proportional-lead interceptor pursuit, and vectorized RK2 shrapnel dispersion with quadratic aerodynamic drag and 3D wind drift.
- **`sim/sensors.py`:** Synthetic terrain-masked radar (`EXERCISE_TRUTH -> EXERCISE_OBSERVATION`) and alpha-beta observation-only tracker.
- **`sim/engine.py`:** Deterministic end-to-end runner plus head-to-head policy comparison (`earliest` baseline vs. `safe_window` wind-aware exposure optimizer).
- **`sim/viewer/`:** Zero-CDN offline Three.js replay viewer supporting EMAD's verified terrain GLB and Sentinel-2 optical drape.

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
