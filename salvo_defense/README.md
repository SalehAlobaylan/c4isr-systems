# EMAD Multi-Threat Salvo & Layered Air Defense (`salvo_defense/`)

Standalone multi-threat salvo simulation engine, observation-only multi-sensor fusion tracker, 3-tier layered defense WTA engine, local HTTP service (`127.0.0.1:8767`), and Three.js 3D Salvo Workbench running on `sim_core/` verified Saudi biomes.

## Key Capabilities

- **Four-Partition Strict Isolation:** All trackers and Weapon-Target Assignment (WTA) policies consume **only** `EXERCISE_OBSERVATION` detections (noisy, terrain-masked, HMAC-opaque contact IDs) — never `EXERCISE_TRUTH`.
- **Mixed Threat Raids (`raid.py`, `contracts.py`):**
  - Concurrent wave-staggered raids combining **RF Decoys**, **One-Way UAV Swarms**, **Low-Altitude Cruise Missiles**, and **High-Apogee Ballistic Missiles**, plus frozen **Civilian Airliners** from `sim_core/traffic.py` with active `1,500 m × 450 m` `HOLD_FIRE` airspace cylinders.
- **Multi-Sensor Fusion & Bayesian Track Classification (`fusion.py`):**
  - Accepts imported `emad.sensor-network.v1` layouts exported from `sensor_coverage/` (radars, EO/IR turrets, acoustic arrays).
  - Runs Global Nearest Neighbor (GNN) track association, 6-state constant-velocity Kalman filtering, and Bayesian class probability estimation (`p_decoy`, `p_uav`, `p_cruise`, `p_ballistic`, `p_civilian`).
- **3-Tier Finite-Magazine Batteries & WTA Policies (`wta.py`, `engine.py`):**
  - `tier1_sam` (\$1.2M/shot, 14 km range, ballistic-capable), `tier2_shorad` (\$180k/shot, 7 km), and `tier3_point_defense` (\$18k/shot, 2.2 km).
  - Head-to-head policy comparison:
    1. `greedy_earliest` — reactive fire-at-first-firm-track baseline.
    2. `combinatorial_wta` — exact branch-and-bound bipartite matching with shoot-look-shoot, cost-tier matching, decoy rejection, civilian airspace `HOLD_FIRE`, and `< 0.5 ms` ML debris surrogate / WorldPop exposure penalty.
    3. `ai_policy` — trained CEM RL observation-only policy loaded automatically from `models/ai_wta_v1.npz`.
- **3D Salvo Workbench (`viewer/`) & 2D Map Panel (`SalvoSetupPanel.tsx`):**
  - Live battery magazine bars, fused COP track cards, simultaneous 3D threats, interceptors, civilian airliner safety cylinders, wind-drifted shrapnel clouds, WorldPop exposure, and OSM road blockage / emergency detour scoring.

## Usage

```powershell
# Start standalone Salvo Defense service & 3D workbench (also auto-started by `task dev`)
task dev:salvo

# Or invoke directly
.\.venv-recovery\Scripts\python.exe -m salvo_defense service --port 8767

# Run automated unit & partition-isolation tests
.\.venv-recovery\Scripts\python.exe -m unittest discover -s salvo_defense/tests -v
```
