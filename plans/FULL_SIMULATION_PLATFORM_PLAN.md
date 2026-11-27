# EMAD Full 3D Simulation & AI Evaluation Platform — Architecture & Implementation Plan

**Date:** 27 November 2026  
**Status:** Proposed design & phased execution plan  
**Companion plans:** [Simulation Directions & Infrastructure](SIMULATION_DIRECTIONS_AND_INFRASTRUCTURE.md) · [Environment & 3D Progress](IMPLEMENTATION_PROGRESS.md) · [Architecture & Direction v2](../EMAD_ARCHITECTURE_AND_DIRECTION_v2.md)

---

## 1. Purpose & Gap Summary

EMAD currently has two working, verified single-scenario 3D simulation workbenches:
1. **Intercept & Shrapnel Exposure (`sim/`, `v0.2.0`, port `8765`)** — single threat (UAV / cruise / ballistic) vs. single radar + interceptor, ERA5 wind drift, dual-origin kinetic-energy shrapnel (`>= 80 J`), Earliest vs. Safe-Window comparison.
2. **Sensor Placement & Blind-Spot Optimizer (`sensor_coverage/`, `v0.2.0`, port `8766`)** — multi-modal Radar / EO-IR / Acoustic viewsheds on an 8 m DSM + OSM building raster, 4/3-earth horizon, route-weighted greedy + 1-swap siting optimizer, real-time 3D LOS occlusion cross-sections, live 3D candidate sandbox, and outage testing.

To move from two standalone vertical slices into a **complete, operationally realistic multi-biome simulation and AI-solution platform**, six structural gaps must be closed cleanly without throwing away existing code:

1. **Shared Simulation Kernel (`sim_core/`):** Eliminate duplicated environment/atmosphere/server/Three.js scaffolding and replace slow per-ray Shapely checks with vectorized `ObstructionGrid` raycasting so 24–30 km AOIs stay fast (`< 2 s` load, `< 1.5 s` simulate).
2. **Real Multi-Biome Scale & Data Enrichment:** Move beyond one 6 × 6 km flat Riyadh box with uniform 11 m building heights by:
   - Activating **already-retained local enrichment data** (`ghs-obat-saudi.zip`, `ghs-obat-riyadh.geojson`, `worldpop-2024.tif`, `gedtm30`, `worldcover-2021`, `cams-dust.json`, `jeddah-v2/prepared.zip` + `jeddah-environment.glb`, `gebco-capture-jubail-v1.zip`), and
   - Staging three full-scale **24–30 km Saudi reference biomes** (Riyadh Expanded Urban Metro, Asir/Jazan Southern Highlands, Jubail Eastern Industrial Coast).
3. **Cross-Simulation Interoperability & Sensor Coverage v0.3:** Export optimized sensor networks (`emad.sensor-network.v1`) directly into Intercept & Salvo simulations; add sector-orientation optimization, adversarial stealth-corridor discovery (A* least-detection path), and EW jamming + CAMS dust attenuation.
4. **Live/Frozen Operational Data Interaction:** Bridge EMAD's existing Public Context layers (`OpenSky` aircraft via `GET /api/v1/external-tracks`, `AIS` vessels, `CelesTrak` SGP4 passes, OSM road graph + `940` civil impact) into strictly isolated `EXERCISE_INPUT` frozen traffic & airspace deconfliction envelopes.
5. **Simulation #3 — Multi-Threat Salvo & Layered Defense (`salvo_defense/`, port `8767`):** Mixed raids (decoys + UAV swarms + cruise + ballistic + civilian airliners) against multi-sensor fusion (`EXERCISE_OBSERVATION`-only Kalman/GNN tracker) and 3-tier finite-magazine batteries (SAM / SHORAD / Point Defense) comparing Greedy vs. Combinatorial WTA Optimizer vs. Trained AI Policy.
6. **Headless Monte Carlo, Sub-Second ML Surrogates & AI/RL Benchmarking (`sim_bench/`):** Multi-process batch runner, probabilistic debris-impact heatmaps, `< 1 ms` ML debris footprint surrogate plugged back into real-time optimizers, deterministic `EXERCISE_OBSERVATION`-only RL environment, trained AI WTA policy (`models/ai_wta_v1.npz`), and a multi-biome benchmark leaderboard.

---

## 2. Inventory of Existing Local Assets Already Ready on Disk

An audit of `private/` and `internal/` confirms major inputs are already retained locally and require zero external re-engineering:

| Asset / Capability | Existing Local Path | Simulation Use |
|---|---|---|
| Riyadh `prepared.zip` + GLB + Sentinel-2 + ERA5 | `private/environment-foundation/implementation/riyadh-observation-pilot/` | Urban baseline biome |
| **GHS-OBAT Saudi Building Heights (200 MB)** | `private/environment-foundation/implementation/riyadh-intake-v1/enrichment/ghs-obat-saudi.zip` & `ghs-obat-riyadh.geojson` | Real modeled tower/villa heights across all Saudi biomes |
| **WorldPop 2024 Population Density** | `private/environment-foundation/implementation/riyadh-access-review/worldpop-2024.tif` | Estimated resident exposure inside shrapnel/impact cones |
| **GEDTM30 Bare-Earth + WorldCover 2021** | `private/environment-foundation/implementation/riyadh-intake-v1/enrichment/` | Ground vs. canopy/structure separation & land-cover clutter |
| **CAMS Dust + METAR + ERA5** | `private/environment-foundation/implementation/supporting-source-review/cams-dust.json`, `metar.json`, `era5-next/` | Dust optical extinction (EO/IR visibility) + multi-level wind shear |
| **Jeddah Coastal Release + 6.4 MB GLB** | `private/environment-foundation/implementation/jeddah-v2/prepared.zip` & `private/local/data/jeddah-environment.glb` | Immediate 2nd real Saudi coastal biome (100% offline) |
| **Jubail GEBCO 2026 Bathymetry + Coastline** | `private/environment-foundation/implementation/coastal-moving-qualification/gebco-capture-jubail-v1.zip` | Eastern Province coastal sea-skimming & maritime masking |
| **GCC OSM + NGA EGM2008 Geoid** | `private/local/data/raw/gcc-states.osm.pbf`, `us_nga_egm08_25.tif` | Road/building/water clipping for 24–30 km Riyadh, Asir & Jubail biomes |
| **3D Actor GLB Assets** | `private/local/data/{fixed-wing,helicopter,uav,vehicle,vessel,sensor}.glb` | Optional high-detail 3D actor meshes for viewers |
| **Live External Tracks API (Go)** | `internal/externaltracks/http.go` (`GET /api/v1/external-tracks`) | Freeze live/historical OpenSky aircraft & AIS vessels into `EXERCISE_INPUT` |
| **Riyadh 940 & Road Control Graph** | `private/environment-foundation/implementation/riyadh-access-review/` | Post-impact road blockage & emergency detour time scoring |
| **CelesTrak SGP4 Orbital Pass** | `private/environment-foundation/implementation/coastal-moving-qualification/noaa20-elements.json` | Optional overhead satellite-pass visibility overlay |

---

## 3. Target System Architecture

```text
                       EMAD OPERATOR UI (MapLibre :5173)
  +------------------+------------------+------------------+------------------+
  | Simulate (:8765) | Coverage (:8766) |  Salvo (:8767)   | Benchmarks (:8768)|
  | Intercept/Debris | Sensor/BlindSpot | Multi-Threat WTA | MC / Surrogate/AI |
  +--------+---------+--------+---------+--------+---------+--------+---------+
           |                  |                  |                  |
    /sim-api/*         /cov-api/*        /salvo-api/*       /bench-api/*
           |                  |                  |                  |
           v                  v                  v                  v
     +-----------+     +-------------+    +-------------+    +--------------+
     | sim/      |     | sensor_     |    | salvo_      |    | sim_bench/   |
     | (v0.3)    |     | coverage/   |    | defense/    |    | Monte Carlo  |
     |           |     | (v0.3)      |    | (v0.1)      |    | Surrogates   |
     |           |     |             |    |             |    | RL & Eval    |
     +-----+-----+     +------+------+    +------+------+    +------+-------+
           |                  |                  |                  |
           +------------------+--------+---------+------------------+
                                       |
                                       v
           +-------------------------------------------------------+
           |                 sim_core/ (Shared Kernel)             |
           | - EnvironmentModel (DSM + GHS-OBAT + WorldPop + GEBCO)|
           | - ObstructionGrid (8–15 m NumPy LOS & raycast)        |
           | - AtmosphereModel (ERA5 4D + CAMS dust + wind gusts)  |
           | - TrafficAdapter (frozen OpenSky / AIS / SGP4 -> ENU) |
           | - RoadImpactGraph (shrapnel blockage & detour metric) |
           | - BiomeRegistry & Shared Sensor-Network Handoff       |
           | - Shared HTTP/CORS/Shutdown + Three.js Terrain Loader |
           +---------------------------+---------------------------+
                                       |
         +-----------------------------+-----------------------------+
         |                             |                             |
         v                             v                             v
  Verified Biomes              EMAD Go Core (:8080)           Persisted Stores
  - riyadh-observation-v1      - GET /api/v1/external-tracks  - tmp/shared-layouts/
  - riyadh-metro-24km-v1       - Environment Releases         - tmp/sim-runs/
  - jeddah-coast-v1            - Public Context (read-only)   - tmp/cov-runs/
  - asir-highlands-30km-v1                                    - tmp/salvo-runs/
  - jubail-coast-30km-v1                                      - tmp/bench-runs/
  - owned-fixture-v1 (CI)                                     - models/
```

### Non-Negotiable EMAD Architectural Invariants
1. **Four-Partition Strict Isolation:** Every simulator preserves `EXERCISE_INPUT`, `EXERCISE_TRUTH`, `EXERCISE_OBSERVATION`, and `EVALUATION`. Trackers, fire-control optimizers, and AI/RL agents consume **only** `EXERCISE_OBSERVATION` (noisy, delayed, terrain-masked, HMAC-opaque IDs) — never `EXERCISE_TRUTH`.
2. **Public Context Read-Only Boundary:** Civilian aircraft/vessels imported from `GET /api/v1/external-tracks` are frozen into `EXERCISE_INPUT` snapshots with explicit provenance and timestamps; simulations never write back to Public Context and never confuse external tracks with controllable assets.
3. **Explicit Derivation Labeling:** Every modeled height (`derived-from-ghs-obat` vs. `synthetic/derived`), WorldPop estimate, CAMS visibility conversion, coverage surface, and ML surrogate prediction carries an explicit derivation tag and declared uncertainty/error metric.
4. **Zero Heavy External Runtime Lock-In:** Core physics, raycasting, combinatorial WTA optimization, Monte Carlo, ML surrogates, and RL policy training run cleanly in `.venv-recovery` (`numpy`, `rasterio`, `fiona`, `shapely`, `pyproj`) with zero mandatory multi-GB GPU dependencies (while keeping standard NumPy/Gym-compatible interfaces so PyTorch/SB3 can plug in seamlessly later).

---

## 4. Detailed Subsystem Designs

### 4.1 `sim_core/` — Shared High-Throughput Spatial, Atmosphere & Data Kernel
- **`sim_core/environment.py`:**
  - Unified `EnvironmentModel` preserving exact manifest SHA-256 pinning, EGM2008 geoid alignment, and local ENU transforms.
  - **GHS-OBAT Height Fusion:** Reads companion `ghs-obat-*.geojson` (or extracts bounded centroids from `ghs-obat-saudi.zip`), indexes them in a Shapely `STRtree`, and assigns modeled heights to OSM buildings lacking `height`/`building:levels` tags (tagged `"derived-from-ghs-obat"` vs. `"source-tag"` vs. `"synthetic/derived"`).
  - **WorldPop Population Sampler:** Reads companion `worldpop-*.tif`, aligns to local ENU, and exposes `population_density_at(east, north)` (people per hectare) + `estimate_population_exposure(impact_enu, radius_m)`.
  - **GEBCO Bathymetry & Shoreline:** Reads companion `gebco-pair.json` / coastal zip to expose `is_sea(east, north)` and `depth_m(east, north)` for coastal biomes.
  - **Large-AOI Scalability:** Adaptive `export_viewer_context(focus_enu, max_buildings, max_roads)` prioritizing tall/large buildings and primary/trunk/motorway roads so 30 × 30 km biomes serialize compactly (`< 2 MB` viewer context) and load in `< 1.5 s`.
- **`sim_core/obstruction.py`:**
  - Generalized `ObstructionGrid` (migrated from `sensor_coverage/grid.py`) supporting adaptive cell size (`8 m` for 6 km tiles, `15–20 m` for 30 km tiles, capped at `2.5M` cells) plus a vectorized `batch_line_of_sight(origins, targets, include_buildings=True)` method — accelerating multi-threat radar scans by **50×–100×** over per-ray Shapely loops.
- **`sim_core/atmosphere.py`:**
  - Unified 4D `AtmosphereModel` adding:
    - **CAMS Dust Extinction:** Reads `cams-dust.json` ($\mu\text{g/m}^3$) and computes Koschmieder optical visibility $V_{\text{km}} = \text{clamp}(3.912 / (\beta_{\text{rayleigh}} + k_{\text{dust}} \cdot C_{\text{dust}}),\, 0.4,\, 30.0)$ (explicitly labeled `derived/cams-koschmieder`).
    - **Dynamic Gust / Front Perturbation:** Deterministic spatially/temporally correlated wind-gust and visibility-drop models for Monte Carlo stress testing.
- **`sim_core/traffic.py`:**
  - `TrafficSnapshotLoader`: queries local EMAD Go API (`http://127.0.0.1:8080/api/v1/external-tracks?west=...&south=...&east=...&north=...`) using local dev token (when available) OR loads retained snapshot JSONs / deterministic synthetic fallback corridors, projects positions & velocity vectors into local ENU, and freezes them as `EXERCISE_INPUT` civilian airliners (`civilian_airliner`) and commercial ships (`commercial_vessel`) with cylindrical safety volumes (e.g. `1,500 m` horizontal × `450 m` vertical for aircraft).
- **`sim_core/road_impact.py`:**
  - Builds a lightweight adjacency graph from OSM `roads` (`primary`, `secondary`, `tertiary`, `trunk`, `motorway`) using pure Python + `heapq` Dijkstra.
  - Given shrapnel/leaker impact points, flags blocked road edges (`>= N` hazardous hits within `road_buffer_m`), computes **total blocked road km** and **emergency response detour penalty (seconds)** from civil-defense staging nodes to affected zones.
- **`sim_core/registry.py` & `sim_core/handoff.py`:**
  - Single source of truth for discovering all verified/staged biomes (`BiomePreset`) with optional sidecars (`ghs_obat_path`, `worldpop_path`, `gebco_path`, `cams_dust_path`, `weather_field_path`, `glb_path`, `imagery_path`).
  - `SensorNetworkStore` (`tmp/shared-layouts/`): save/list/load `emad.sensor-network.v1` JSON bundles pinned to `environmentId` + `manifestSha256` so `sensor_coverage` feeds directly into `sim` and `salvo_defense`.
- **`sim_core/http_base.py` & `sim_core/viewer_shared/emad-terrain.js`:**
  - Shared `ExclusiveThreadingHTTPServer`, CORS, `/api/shutdown`, `/vendor/three/`, and shared ES module for GLB terrain, Sentinel-2 draping, buildings, roads, and ENU transforms across all Three.js workbenches.

### 4.2 Three-Biome Reference Environment Suite (`scripts/data/build_sim_biomes.py`)
1. **`riyadh-observation-v1` (6 × 6 km, Enriched — 100% offline):** Existing staged Riyadh package enhanced with `ghs-obat-riyadh.geojson`, `worldpop-2024.tif`, and `cams-dust.json`.
2. **`jeddah-coast-v1` (8 × 8 km, Enriched — 100% offline):** Wires already-staged `jeddah-v2/prepared.zip` + `jeddah-environment.glb` + GHS-OBAT extracted from `ghs-obat-saudi.zip` + companion coastal ERA5 profile.
3. **`riyadh-metro-24km-v1` (24 × 24 km, Expanded Urban Basin):** Northern/central Riyadh corridor covering KAFD / Kingdom Centre / major highway interchanges with `ghs-obat-saudi.zip` tower heights, `worldpop-2024.tif`, decimated 3D terrain GLB (`stride=2`, ~240k triangles), and ERA5 winds.
4. **`asir-highlands-30km-v1` (30 × 30 km, Southern Mountain Escarpment — Abha / Jazan corridor):** Severe 1,500–2,500 m relief from Copernicus GLO-30 COG + EGM2008 + `gcc-states.osm.pbf` + `ghs-obat-saudi.zip` + ERA5. Stress-tests extreme mountain radar shadows, valley terrain-following cruise ingress, and ridge-top sensor siting.
5. **`jubail-coast-30km-v1` (30 × 30 km, Eastern Gulf Industrial Coast):** Copernicus GLO-30 land + already-retained `gebco-capture-jubail-v1.zip` bathymetry/shoreline + `gcc-states.osm.pbf` + `ghs-obat-saudi.zip` + ERA5 coastal wind. Stress-tests flat sea-skimming approach paths, maritime clutter, and industrial/desalination site defense.

### 4.3 Sensor Coverage v0.3 Enhancements (`sensor_coverage/`)
- **Joint Position + Orientation Optimization:** For directional sector sensors (`sectorWidthDeg < 360`), evaluates 8 discrete heading bins (`0°, 45°, ..., 315°`) per candidate site inside the greedy + 1-swap loop.
- **Adversarial Least-Detection Path Finder (`POST /api/adversarial-route`):** Runs Dijkstra / A* over the 2D analysis grid with edge cost proportional to `sum(detection_probability_at_agl)` + terrain slope penalty from any perimeter entry edge to a defended asset, surfacing the exact hidden corridor an adversary would exploit — displayed as a pulsing violet stealth trajectory in 3D and 2D!
- **EW / Stand-Off Jamming & Dust Plumes:** Adds optional `jammers: [{ lon, lat, powerFactor, sectorWidthDeg, sectorCenterDeg }]` degrading radar effective range in jammed azimuth sectors ($R_{\text{eff}}' = R_{\text{eff}} / (1 + J/S)^{1/4}$) and GPS-denied zones, plus automatic CAMS dust visibility preset.
- **1-Click Export to Intercept / Salvo:** Saves optimized layouts via `sim_core/handoff.py` (`POST /api/export-network`).

### 4.4 Simulation #3 — Multi-Threat Salvo & Layered Defense (`salvo_defense/`, Port `8767`)
- **Actors & Raid Engine (`salvo_defense/raid.py`):**
  - Supports 2–20 concurrent or wave-staggered threats (`decoy`, `one_way_uav`, `cruise_missile`, `ballistic_missile`) with configurable spawn times, altitudes, speeds, RCS, warhead mass, and target assets (plus optional adversarial stealth routing!).
  - Integrates frozen civilian airliners from `sim_core/traffic.py` with active 3D airspace safety bubbles.
- **Multi-Sensor Fusion & Track Classification (`salvo_defense/fusion.py`):**
  - Strictly consumes `EXERCISE_OBSERVATION` detections from all radars, EO/IR turrets, and acoustic arrays (importable from `sensor_coverage`!).
  - Performs Global Nearest Neighbor (Hungarian / greedy distance-gated) association, 3D constant-velocity Kalman filtering, and Bayesian threat-class probability updates (`p_decoy`, `p_uav`, `p_cruise`, `p_ballistic`, `p_civilian`) using observed speed, altitude profile, RCS estimate, EO/IR confirmation, and transponder correlation.
- **3-Tier Layered Defense Batteries (`salvo_defense/batteries.py`):**
  - `tier1_sam` (long range `14 km`, `650 m/s`, high ceiling, cost `10`, magazine `6`, reload `2.5 s`)
  - `tier2_shorad` (medium range `7 km`, `360 m/s`, agile, cost `3`, magazine `12`, reload `1.5 s`)
  - `tier3_point_defense` (short range `2.2 km`, `240 m/s`, low debris, cost `1`, magazine `24`, reload `0.6 s`)
  - Shoot-look-shoot tracking (`in_flight_interceptors_by_track`) to prevent wasteful simultaneous overkill.
- **Pluggable Weapon-Target Assignment Policies (`salvo_defense/wta.py`):**
  1. `greedy_earliest`: Baseline reactive fire-at-first-firm-track policy.
  2. `combinatorial_wta`: Rolling-horizon threat-priority + cost-matching + shoot-look-shoot + **sub-second surrogate shrapnel/WorldPop exposure penalty** + **civilian airspace hold-fire check** + decoy rejection threshold.
  3. `ai_policy`: Loads trained weights from `models/ai_wta_v1.npz` (trained in Phase 6 via `sim_bench`), operating strictly on `EXERCISE_OBSERVATION` features!
- **Service, 3D Workbench & 2D Map Panel:**
  - Standalone HTTP service (`python -m salvo_defense service --port 8767`), `task dev:salvo`, wired into `task dev` and `vite.config.ts` (`/salvo-api/*`, `/__local/salvo/*`).
  - Dedicated Three.js 3D Salvo Workbench (`salvo_defense/viewer/`) with multi-threat models, civilian airliners + safety cylinders, live battery magazine HUD bars, multi-sensor track beams, simultaneous shrapnel clouds, WorldPop/road-blockage metrics, and 3-way policy comparison switcher.
  - Dedicated `Salvo` button on the 2D MapLibre rail (`SalvoSetupPanel.tsx`, `salvoClient.ts`) with presets (`Mixed Swarm + Decoys`, `Ballistic + Cruise Saturation`, `Civilian Airspace Deconfliction`, `Imported Coverage Network Test`).

### 4.5 Headless Monte Carlo, ML Surrogates & AI/RL Benchmarking (`sim_bench/`, Port `8768` or CLI + Embedded Tab)
- **Monte Carlo Debris Probability Heatmap (`sim_bench/debris_mc.py`):**
  - Multi-process runner executing `N = 200–1,000` perturbed trials (wind vector jitter $\pm 4\text{ m/s}$, intercept timing jitter $\pm 0.5\text{ s}$, fragment drag/burst variance).
  - Aggregates per-cell probability of receiving $\ge 1$ hazardous ($\ge 80\text{ J}$) impact, expected building hits, expected WorldPop residents exposed, and 50%/90%/99% risk contours — exportable as GeoJSON (for 2D map) and `DataTexture` (for 3D workbench).
- **Sub-Second ML Debris Surrogate (`sim_bench/surrogate.py`):**
  - Feature vector (14 dims): intercept `(e, n, agl)`, threat velocity `(ve, vn, vu)`, threat mass, ERA5 wind at `[50m, 250m, 600m, 1200m]`, local urban density.
  - Pure-NumPy 2-hidden-layer MLP (`64 -> 64 -> 7` outputs with Adam optimizer, L2 regularization, feature normalization, and train/val split) trained on `2,000–5,000` headless physics samples in `< 30 s` on CPU; saved to `models/debris_surrogate_v1.npz` with validation metrics (`R²`, `MAE` in metres & exposure %).
  - **< 0.3 ms inference** per candidate intercept point — integrated directly as an accelerated preview engine inside `sim` Safe-Window and `salvo_defense` Combinatorial WTA!
- **AI / RL Environment & Policy Trainer (`sim_bench/rl_env.py`, `sim_bench/train_wta.py`):**
  - Deterministic `SalvoRLEnv` exposing strictly `EXERCISE_OBSERVATION` per-track + per-battery feature matrices and shaped reward:
    $$R = -100 \cdot \text{leakers} - 80 \cdot \text{airspace\_violations} - 12 \cdot \text{urban\_pop\_exposure} - 2 \cdot \text{interceptor\_cost} - 15 \cdot \text{decoy\_shots} + 10 \cdot \text{confirmed\_intercepts}$$
  - Trains a compact neural/linear scoring policy (`Cross-Entropy Method / Evolution Strategies` in pure NumPy over parallel episodes — stable, deterministic, zero PyTorch dependency!) across randomized multi-biome raids and saves `models/ai_wta_v1.npz`.
- **Multi-Biome Benchmark Suite (`sim_bench/benchmark.py`):**
  - Evaluates `greedy_earliest` vs. `combinatorial_wta` vs. `ai_policy` across 30–50 frozen seeds per biome (Riyadh, Jeddah, Asir, Jubail), producing structured JSON/Markdown leaderboards and an interactive **Benchmarks & Monte Carlo** service/UI panel (`http://127.0.0.1:8768` or integrated drawer tab).

---

## 5. Phased Execution Plan & Verification Gates

| Phase | Scope & Key Deliverables | Core Files | Acceptance Gate |
|---|---|---|---|
| **Phase 0** | Commit & push verified `sensor_coverage` `v0.2.0` baseline cleanly (per `AGENTS.md`). | Existing worktree | Clean `git status` for simulation files; pushed to `origin/main`. |
| **Phase 1** | Extract `sim_core/` kernel; migrate `sim/` & `sensor_coverage/`; enable **GHS-OBAT heights**, **WorldPop exposure**, **CAMS dust**, **Jeddah coastal biome** (100% offline); add `emad.sensor-network.v1` export/import handoff. | `sim_core/{environment,obstruction,atmosphere,registry,handoff,http_base}.py`, `sim/`, `sensor_coverage/` | All `sim_core`, `sim`, and `sensor_coverage` tests pass; Riyadh shows variable GHS-OBAT skyline + WorldPop exposure; Jeddah loads in both 3D workbenches; exported sensor network imports into `sim`. |
| **Phase 2** | Build `scripts/data/build_sim_biomes.py` and stage **30 km Asir Highlands**, **30 km Jubail Coast (with GEBCO)**, and **24 km Riyadh Metro** biomes + large-AOI LOD/decimation. | `scripts/data/build_sim_biomes.py`, `sim_core/environment.py`, `sim_core/registry.py` | 30 km biomes verify SHA-256 manifests, load in `< 2 s`, simulate in `< 1.5 s`, and demonstrate dramatic mountain terrain shadows in Asir + sea-skimming corridors in Jubail. |
| **Phase 3** | Upgrade `sensor_coverage` to `v0.3.0`: **sector azimuth optimization**, **A* Adversarial Stealth Path finder**, **EW jammer + CAMS dust attenuation**, and **1-click Export Network** UI. | `sensor_coverage/{optimizer,analysis,service}.py`, `sensor_coverage/viewer/`, `CoverageSetupPanel.tsx` | Unit tests prove orientation gain, stealth route stays in lowest-detection corridor, jammer reduces burn-through range; verified in browser with 0 console/network errors. |
| **Phase 4** | Build `sim_core/traffic.py` (**OpenSky/AIS/SGP4 -> frozen `EXERCISE_INPUT`**) + `sim_core/road_impact.py` (**OSM road blockage & emergency detour time**); add civilian airspace deconfliction & road-impact scoring to `sim/` (`v0.3.0`). | `sim_core/{traffic,road_impact}.py`, `sim/{engine,physics,service}.py`, `sim/viewer/`, `SimulationSetupPanel.tsx` | Airliner safety bubble triggers hold-fire / reroute avoidance in Safe-Window; shrapnel reports blocked road km + detour seconds; strict Public Context read-only isolation tested. |
| **Phase 5** | Build **Simulation #3: Multi-Threat Salvo & Layered Defense (`salvo_defense/`, port `8767`)** — multi-sensor fusion, mixed raids (decoys + UAV + cruise + ballistic + airliners), 3-tier batteries, `greedy` vs. `combinatorial_wta`, standalone 3D Workbench, `Salvo` map rail panel, `task dev:salvo` & `vite.config.ts` proxy. | `salvo_defense/`, `apps/operator-ui/.../{SalvoSetupPanel.tsx,salvoClient.ts}`, `vite.config.ts`, `scripts/ops/local.py`, `Taskfile.yml` | Unit tests confirm zero truth-leakage in fusion, fewer leakers & decoy waste under `combinatorial_wta`, zero airspace violations; live 2D->3D browser E2E passes with 0 errors. |
| **Phase 6** | Build **`sim_bench/`**: multi-process **Monte Carlo Debris Probability Heatmap** (rendered in 2D & 3D), **Sub-Second ML Debris Surrogate** (`< 1 ms`, plugged into Safe-Window & Salvo WTA), **RL `SalvoRLEnv` + Trained AI WTA Policy (`models/ai_wta_v1.npz`)**, and **Multi-Biome Benchmark Leaderboard** (CLI + UI). | `sim_bench/{runner,debris_mc,surrogate,rl_env,train_wta,benchmark,service}.py`, `models/`, UI panel | Surrogate achieves `R² > 0.90` & `< 1 ms` inference; trained AI WTA policy runs live as Policy #3 in `salvo_defense` and beats `greedy_earliest` across multi-biome frozen seeds; MC heatmap renders cleanly in 2D & 3D. |
| **Phase 7** | Full cross-subsystem regression suite, browser E2E verification across all 3 workbenches + benchmark UI, performance budget verification, and final documentation + clean commits. | `*/tests/`, `plans/`, `README`s | 100% Python unit/integration tests + `pnpm typecheck/build` + live browser checks pass with 0 console/network errors across ports `5173`, `8765`, `8766`, `8767`. |

---

## 6. Key Risks & Mitigations

| Risk | Mitigation |
|---|---|
| **30 km AOI memory / JSON payload bloat** | Use `ObstructionGrid` (15 m raster = 4M bytes) for physics; decimate display GLB (`stride=2`, ~240k tris); LOD-filter buildings/roads exported to `viewerContext`; keep `floorB64` binary-packed (`uint16`). |
| **External download failure when acquiring GLO-30 tiles for Asir & Jubail** | Phase 1 delivers Riyadh Enriched + Jeddah Coastal **100% offline** first; Phase 2 acquisition scripts pin SHA-256 receipts and skip cleanly if offline without breaking CI or offline fixtures. |
| **Live OpenSky rate limits / offline runs** | `sim_core/traffic.py` reads EMAD's local Go `/api/v1/external-tracks` cache first, supports retained JSON snapshots, and provides deterministic Saudi air-corridor fallback fixtures so tests & offline demos always work. |
| **Four-partition truth leakage into AI/WTA policies** | Enforce structural separation: `SalvoRLEnv` and `wta.py` accept ONLY `TrackPicture` built from `EXERCISE_OBSERVATION` (HMAC-opaque contact IDs); dedicated unit tests assert `EXERCISE_TRUTH` objects are never passed into any policy or tracker. |
| **Port / stale-process collisions across 3+ local Python services** | Reuse `sim_core/http_base.py` (`SO_EXCLUSIVEADDRUSE` + `/api/shutdown` + `/api/health` service discriminator & `capabilities`) and `createLocalPythonServicePlugin` in `vite.config.ts` across ports `8765`, `8766`, `8767`, `8768`. |
