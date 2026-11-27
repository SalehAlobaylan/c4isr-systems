# EMAD 3D Sensor Placement & Blind-Spot Optimizer (`sensor_coverage/`)

Standalone 3D coverage engine, placement optimizer, local HTTP service (`127.0.0.1:8766`), and Three.js 3D workbench running on EMAD's verified geographic packages (`prepared.zip` + ERA5 atmosphere).

## Key Capabilities
- **Multi-Modal Physical Sensor Models (`contracts.py`, `viewshed.py`):**
  - **Radar:** 4/3-earth radio refraction horizon, $R \propto \mathrm{RCS}^{1/4}$ target scaling, minimum elevation mask, terrain + OSM building occlusion, and low-altitude ground clutter floor.
  - **EO/IR:** Optical line-of-sight with meteorological visibility clamping and target size scaling.
  - **Acoustic:** Non-LOS diffraction detection with ERA5 surface-wind noise degradation.
- **Continuous Altitude-Floor Viewsheds (`grid.py`, `viewshed.py`):**
  - Builds a high-resolution ENU obstruction raster burning OSM building roofs (`source-tag` or `synthetic/derived`) onto the geoid-aligned Copernicus GLO-30 DSM.
  - Computes each sensor's exact **lowest detectable altitude AGL** across every analysis cell in a single vectorized radial sweep, enabling instant continuous altitude slicing in the 3D workbench.
- **Submodular Greedy + 1-Swap Placement Optimizer (`optimizer.py`):**
  - Generates lattice, terrain high-point, and building rooftop candidate sites while respecting locked existing sensors and minimum separation constraints.
  - Maximizes layer-weighted, asset-priority-weighted coverage plus redundancy bonus via greedy selection ($1 - 1/e$ guarantee) followed by bounded 1-swap refinement.
- **Ingress-Route Warning-Time Analysis (`analysis.py`):**
  - Evaluates low-altitude UAV and cruise-missile ingress corridors for first detection point, warning distance/time, tracked fraction, and post-detection track custody gaps.

## Usage

```powershell
# Start the standalone Sensor Coverage service & 3D workbench (auto-started by `task dev` too)
task dev:coverage

# Or invoke directly
.\.venv-recovery\Scripts\python.exe -m sensor_coverage service --port 8766

# Run a headless CLI comparison on Riyadh or the owned fixture
.\.venv-recovery\Scripts\python.exe -m sensor_coverage run --env riyadh-observation-v1
```
