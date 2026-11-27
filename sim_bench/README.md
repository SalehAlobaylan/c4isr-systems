# EMAD Monte Carlo, ML Surrogate & AI/RL Benchmark Suite (`sim_bench/`)

Headless ensemble engine, pure-NumPy ML debris footprint surrogate trainer, deterministic observation-only RL environment, Cross-Entropy Method (CEM) WTA policy trainer, multi-biome benchmark harness, and local HTTP service (`127.0.0.1:8768`).

## Key Capabilities

- **Vectorized Monte Carlo Debris Probability Heatmaps (`debris_mc.py`):**
  - Runs `N` perturbed 3-DOF RK2 shrapnel trials (wind vector uncertainty $\sigma$, intercept timing jitter, fragment drag/burst variance) across verified Saudi biomes and outputs per-cell hazard probability grids plus WGS84 GeoJSON risk contours (`Low >= 5%`, `Medium >= 20%`, `High >= 50%`) rendered directly on the EMAD 2D map.
- **Sub-Millisecond ML Debris Footprint Surrogate (`surrogate.py`):**
  - Pure-NumPy `12 -> 48 -> 48 -> 5` ReLU MLP trained via Adam on headless EMAD shrapnel physics samples across verified biomes and saved to `models/debris_surrogate_v1.npz` (`derived/ml-debris-surrogate-v1`).
  - Predicts wind drift, footprint spread, urban exposure fraction, and WorldPop exposure in `< 0.5 ms` — automatically loaded by both `sim/` Safe-Window and `salvo_defense/` Combinatorial WTA.
- **Observation-Only RL Environment & CEM WTA Trainer (`rl_env.py`, `train_wta.py`):**
  - Deterministic `SalvoRLEnv` enforcing EMAD's strict `EXERCISE_OBSERVATION` isolation (policies never see `EXERCISE_TRUTH`).
  - Trains `models/ai_wta_v1.npz` (`derived/cem-rl-observation-only-wta`) with explicit safety clamps for civilian airspace avoidance and evaluates on held-out seeds against `greedy_earliest` and `combinatorial_wta`.
- **Multi-Biome Benchmark Harness (`benchmark.py`, `service.py`, `server.py`):**
  - Evaluates all available WTA policies across frozen seeds and biomes (`riyadh-observation-v1`, `asir-highlands-30km-v1`, `jubail-coast-30km-v1`, `owned-fixture-v1`), persisting structured scorecards to `tmp/benchmarks/latest.json` and serving them to the `Bench` panel in Operator UI (`:8768`).

## Usage

```powershell
# Start Benchmark & Monte Carlo HTTP service (also auto-started by `task dev`)
task dev:bench

# Train the ML debris surrogate (< 1 min on CPU)
.\.venv-recovery\Scripts\python.exe -m sim_bench train-surrogate --samples 300 --epochs 350

# Train the observation-only RL AI WTA policy
.\.venv-recovery\Scripts\python.exe -m sim_bench train-wta --generations 6 --population 10

# Run the frozen multi-biome benchmark suite
.\.venv-recovery\Scripts\python.exe -m sim_bench benchmark

# Run automated unit tests
.\.venv-recovery\Scripts\python.exe -m unittest discover -s sim_bench/tests -v
```
