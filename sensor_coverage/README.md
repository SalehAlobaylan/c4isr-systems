# EMAD 3D Sensor Placement & Blind-Spot Optimizer (`sensor_coverage/`, `v0.3.0`)

Standalone 3D coverage engine, placement & sector-azimuth optimizer, adversarial stealth-corridor finder, local HTTP service (`127.0.0.1:8766`), and Three.js 3D workbench built on the shared `sim_core/` kernel across all verified Saudi biomes (`riyadh-observation-v1`, `riyadh-metro-24km-v1`, `jeddah-coast-v1`, `asir-highlands-30km-v1`, `jubail-coast-30km-v1`, and `owned-fixture-v1`).

## Key Capabilities
- **Multi-Modal Physical Sensor Models (`contracts.py`, `viewshed.py`):**
  - **Radar:** 4/3-earth radio refraction horizon, $R \propto \mathrm{RCS}^{1/4}$ target scaling, minimum elevation mask, terrain + GHS-OBAT/OSM building occlusion, low-altitude ground clutter floor, and stand-off **EW Jamming burn-through attenuation** ($R' = R / (1 + J/S)^{1/4}$).
  - **EO/IR:** Optical line-of-sight with meteorological visibility clamping, **CAMS Dust Koschmieder preset**, and target size scaling.
  - **Acoustic:** Non-LOS diffraction detection with ERA5 surface-wind noise degradation.
- **Continuous Altitude-Floor Viewsheds (`sim_core/obstruction.py`, `viewshed.py`):**
  - Uses adaptive high-resolution ENU obstruction rasters burning GHS-OBAT/OSM building roofs onto geoid-aligned Copernicus GLO-30 DSM (plus GEBCO 2026 bathymetry on coastal biomes).
  - Computes each sensor's exact **lowest detectable altitude AGL** across every analysis cell in a vectorized radial sweep, enabling instant continuous altitude slicing and real-time 3D LOS occlusion cross-sections.
- **Joint Site + Sector-Azimuth Submodular Optimizer (`optimizer.py`):**
  - Evaluates lattice, terrain high-point, and rooftop candidate sites (and 8 discrete heading bins for directional sector sensors) while respecting locked existing sensors, EW jammers, and minimum separation constraints.
  - Maximizes layer-weighted, asset-priority-weighted coverage plus redundancy bonus via greedy selection ($1 - 1/e$ guarantee) followed by bounded 1-swap refinement.
- **Adversarial Least-Detection Stealth Path Finder (`analysis.py`):**
  - Runs A*/Dijkstra over the 2D viewshed surface (`POST /api/adversarial-route`) from perimeter entry edges to the primary defended asset to reveal the exact terrain/EW-masked corridor an adversary would exploit.
- **Cross-Simulation Network Handoff (`sim_core/handoff.py`):**
  - Exports optimized multi-modal sensor networks (`emad.sensor-network.v1`, `POST /api/export-network`) pinned to `environmentId` + `manifestSha256` for 1-click import into `sim/` (`:8765`) and `salvo_defense/` (`:8767`).

## Usage

```powershell
# Start the standalone Sensor Coverage service & 3D workbench (auto-started by `task dev` too)
task dev:coverage

# Or invoke directly
.\.venv-recovery\Scripts\python.exe -m sensor_coverage service --port 8766

# Run a headless CLI comparison on Riyadh, Asir Highlands, Jubail Coast, or the owned fixture
.\.venv-recovery\Scripts\python.exe -m sensor_coverage run --env asir-highlands-30km-v1
```
