# Simulation Directions and Supporting Infrastructure

**Date:** 25 November 2026  
**Scope:** Planning reference bridging EMAD's completed geographic environment & 3D foundation into the upcoming 3D simulation phase.  
**Related records:** [Environment & 3D progress](IMPLEMENTATION_PROGRESS.md) · [Simulation preparation delivery](../docs/implementation/SIMULATION_PREPARATION_DELIVERY.md) · [Data strategy v2](../EMAD_FINAL_DATA_STRATEGY_v2.md) · [Architecture & direction v2](../EMAD_ARCHITECTURE_AND_DIRECTION_v2.md) · [Source expansion review](../docs/data/WORLDMONITOR_SOURCE_REVIEW_2026_11_17.md)

---

## 1. Why EMAD's Current Environment Stack Unlocks Realistic Simulations

Rather than building isolated 3D visual mockups, EMAD already provides versioned, verified geographic packages and a strict data-isolation model that allow simulations to interact directly with real-world terrain, weather, traffic, and urban context:

- **Terrain, elevation & coordinates (`F01–F10`, `V01–V08`, `D02`, `P02–P03`):** Copernicus GLO-30 DSM, OpenGeoHub GEDTM30 ground (DTM), NGA EGM2008 geoid, deterministic local ENU/ECEF frames, and verified offline ZIP packages (`environment_reader.py`).
- **Surface, imagery & urban vectors (`D01`, `D03`, `D08`):** Sentinel-2 optical scenes, Sentinel-1 SAR VV backscatter, ESA WorldCover 2021 land cover, OSM roads/buildings/water with topology companions, and GHS-OBAT modeled building-height context.
- **Arabic regional geography (`Desert Maps`):** Cataloged place names, wadis, mountains, sand terrain, and desert tracks prioritizing Riyadh, Eastern Province, and Jazan ([Desert Maps plan](DESERT_MAPS_PLACE_REFERENCE.md)).
- **Coastal & atmospheric layers (`D05`, `D06`, `P06`):** Jubail GEBCO 2026 bathymetry and shoreline, PortWatch daily port baselines, METAR station observations, and ERA5 multi-level pressure/wind/temperature grids (`emad.era5-pressure-subset.v1`) with explicit `known-at` vs. valid-time semantics.
- **Moving traffic, orbital & civil-context layers (`D04`, `D07`, simulation-sources):** OpenSky/adsb.lol civilian aircraft, AISStream vessels, CelesTrak SGP4 orbital passes, USGS/FIRMS/GDACS/EONET hazards, Riyadh 940 municipal requests, SRCA/MOH regional capacity, and planned WorldPop/Open-Meteo/CAMS/NOTAM/GPSJAM/Cloudflare Radar sources.
- **Four-partition evaluation discipline:** `PUBLIC_EVIDENCE` feeds frozen, versioned `EXERCISE_INPUT` packages; simulation runtimes emit deliberately imperfect `EXERCISE_OBSERVATION` streams while keeping `EXERCISE_TRUTH` sealed for scoring. This lets human operators, deterministic rules, optimization solvers, and AI/RL agents be benchmarked head-to-head on identical frozen Saudi environments.

---

## 2. Core Simulation Directions

### 2.1 Post-Intercept Shrapnel & Debris Footprint ("Intercept-to-Impact")

Intercepting a missile or drone in flight does not end the hazard—**intercept altitude, closing geometry, fragmentation profile, and multi-altitude wind shear** determine whether heavy debris and shrapnel land in open desert or over dense urban corridors and critical facilities.

- **Environment & data interactions:**
  - **Multi-level atmospheric winds (`ERA5` pressure levels + `METAR` / planned `Open-Meteo`):** Heavy warhead/motor segments follow near-ballistic arcs, while lighter airframe fragments and shrapnel drift kilometers downwind across altitude layers.
  - **Terrain, urban footprints & exposure (`GLO-30` DSM, `GEDTM30`, `OSM` buildings/roads, `GHS-OBAT`, planned `WorldPop`, `Desert Maps`):** Resolves 3D ground and building intersections to estimate impact risk across urban Riyadh, Eastern industrial/energy corridors, or open desert.
  - **Post-impact civil response (`940` reports, road topology, `FIRMS`-style thermal cues):** Links predicted debris impact zones directly to road obstructions, secondary fire risk, and emergency crew dispatch.
- **Solutions to develop & benchmark (AI + engineering):**
  - **AI "Safe Intercept Window" Optimizer:** Recommends engagement timing and intercept geometry that minimize projected debris exposure over populated/critical zones while keeping intercept probability above threshold.
  - **Sub-Second Debris Impact Cone Surrogate:** Uses thousands of headless Monte Carlo ballistic+wind runs to train a fast ML surrogate predicting the 3D debris rain footprint within milliseconds of an intercept, enabling automated localized shelter warnings and pre-positioned civil defense routing.

### 2.2 Multi-Threat Missile & Drone Interception Across Saudi Biomes

Evaluating the same defensive problem across distinct Saudi geographies exposes radically different physical and operational challenges:

- **Southern Highlands (Asir / Jazan):** Steep relief (`GLO-30` DSM + `Desert Maps` mountains and wadis) produces deep **radar shadows and terrain masking**, allowing low-flying cruise missiles or one-way UAVs to stay occluded behind ridgelines until close range.
- **Eastern Province / Gulf Coast (Jubail & coastal corridor):** Flat sea-skimming approach paths over water (`GEBCO` coast), dense **civilian aviation (`OpenSky`/`adsb.lol`)**, busy **shipping lanes (`AISStream`/`PortWatch`)**, and industrial thermal clutter.
- **Weather & atmospheric degradation (`ERA5`, `METAR`, planned `CAMS` dust/aerosols):** Dust storms, low visibility, and high winds degrade EO/IR optical tracking and alter small-UAV flight envelopes, shifting detection reliance dynamically across radar, RF, and acoustic modalities.
- **Solutions to develop & benchmark (AI + engineering):**
  - **3D Sensor-Placement & Blind-Spot Optimizer:** Raycasted 3D line-of-sight (LOS) analysis over terrain meshes to optimize radar/EO/acoustic placement, closing valley blind spots in the South and low-altitude gaps along the Gulf coast.
  - **Layered Defense & Salvo Allocation (Weapon-Target Assignment):** Benchmarks RL/optimization schedulers against fixed rules when defending against mixed raids (cheap decoys + slow UAVs + fast missiles) so scarce interceptors are conserved for high-priority threats.
  - **Airspace Deconfliction:** Integrates retained `OpenSky`/`adsb.lol` flight corridors and `NOTAM`/`eAIP` boundaries to test automated civilian-airspace warnings and safe rerouting clear of engagement volumes and debris cones.

---

## 3. Additional Simulation Domains Leveraging the Full Data Stack

| # | Simulation Domain | Target Saudi Region(s) | Interacting EMAD Data Layers | AI / Technical Solutions to Build & Test |
|---|---|---|---|---|
| **1** | **Urban Counter-UAS & Drone-vs-Drone Pursuit** | Riyadh urban core; Eastern industrial zones | 3D buildings (`OSM` + `GHS-OBAT`), `WorldCover`, local wind gusts, `GPSJAM` degradation, airspace restrictions | Multi-agent RL / cooperative pursuit-evasion for interceptor drones operating in urban canyons with LOS occlusion, GPS dropouts, and collateral-free capture zones |
| **2** | **Multi-Sensor Fusion Under Deception & Jamming (EW)** | Eastern Gulf coast; Red Sea / Jazan | `OpenSky`, `AISStream`, `GPSJAM`, `CAMS` dust, `Sentinel-1` SAR + synthetic imperfect sensors (`EXERCISE_OBSERVATION`) | Kalman/JPDA + ML anomaly detection to expose spoofed ADS-B/AIS tracks, separate low-RCS drones from civilian traffic/clutter, and maintain track custody across jamming or terrain blackouts |
| **3** | **Coastal & Maritime Critical-Infrastructure Defense** | Jubail (`D06` coastal pilot); Jazan port | `GEBCO` bathymetry & shoreline, `AISStream`, `PortWatch` baselines, planned `Open-Meteo` marine waves/currents | Detecting unmanned surface vessels (USVs) or sea-skimming drones concealed inside busy commercial shipping lanes near desalination/energy facilities, plus simulating multi-day port disruption and recovery |
| **4** | **Satellite-Pass Exploitation & Autonomous ISR Tasking** | National / Middle East regional | `CelesTrak` SGP4 orbital passes (`D07`), cloud/dust cover (`Sentinel-2` SCL / `CAMS`), `Sentinel-1` SAR revisit windows, `FIRMS` | Computing exact overhead EO/SAR visibility windows (factoring weather and terrain shadow) and using AI schedulers to dispatch reconnaissance UAVs specifically to cover blind gaps |
| **5** | **Degraded-Comms & Resilient Command Coordination** | Southern highlands; Riyadh | `Cloudflare Radar` outage timelines, `delivery-replay` latency/packet loss, `Desert Maps` remote terrain | Stress-testing interception networks, sensor fusion, and drone coordination under link outages, delayed/conflicting reports, or disconnected C2—developing resilient edge fallback behavior |
| **6** | **End-to-End Strike → Civil Defense & Rapid Recovery** | Riyadh (`940` + road graph); Asir (bottleneck mountain roads + `EONET` floods) | Shrapnel/impact footprint → road closures, `WorldPop` exposure, `940` call spikes, `SRCA`/`MOH` capacity, aerial inspection imagery | AI incident triage and autonomous inspection-drone routing to verify actual road/infrastructure damage vs. false alarms, minimizing reopening and emergency response times |

---

## 4. Infrastructure Primitives to Plan Before Full 3D Simulation

To ensure simulations share reusable, high-integrity foundations instead of one-off scripts, five core infrastructure capabilities should be planned next:

### 4.1 High-Throughput Runtime 3D Spatial & LOS Query Engine
- **Current state:** `scripts/data/environment_reader.py` provides exact audited single-point queries (~2–5 ms per read) directly from verified ZIP archives.
- **What simulations need:** Loading verified environment packages into an in-memory spatial index (heightfield grid + optional building/obstruction BVH) capable of **thousands to millions of elevation lookups and 3D raycasts per second** for terrain-masked radar LOS, optical visibility, UAV obstacle avoidance, and ballistic ground/building impact checks—while preserving the parent package's manifest SHA-256 and local ENU coordinate frame.

### 4.2 4D Atmospheric & Wind Interpolation Layer (Above `D05`/`P06`)
- **Current state:** `P06` reads native ERA5 pressure levels (`850 hPa`, `700 hPa`, etc.) without vertical height conversion or interpolation.
- **What simulations need:** An explicitly derived atmospheric query service `(lat, lon, altitude_m, time) → (wind_u, wind_v, wind_w, air_density, temperature, visibility/dust)` with documented vertical conversion (pressure-to-altitude / ISA fallback), explicit uncertainty bounds, and strict `known-at` vs. valid-time separation so forecast errors and reanalysis never bleed across decision cutoffs.

### 4.3 Three-Biome Reference Environment Suite
Expanding detailed prepared packages beyond Riyadh provides three physically distinct testbeds covering Saudi operational diversity:
1. **Riyadh (Urban Basin — baseline staged):** Dense urban roads, buildings, municipal demand, population exposure.
2. **Eastern Province / Jubail (Industrial Coast — `D06` pilot exists):** Flat coastal terrain, bathymetry, dense maritime (`AIS`/`PortWatch`) and aviation (`OpenSky`) traffic, industrial facilities.
3. **Southern Highlands — Asir / Jazan (Mountain & Border — new release needed):** Severe elevation relief, radar-shadowed valleys, sparse single-corridor road networks, mountain/coastal contrast.

### 4.4 Explicit Derived 3D Building & Obstruction Policy (`D08` Gap)
- **Current state:** Riyadh OSM building footprints lack usable height tags; `GHS-OBAT` provides separate centroid-level modeled height statistics (`median ~10.9 m`).
- **What simulations need:** A versioned, explicitly tagged `synthetic/derived` 3D obstruction layer policy (with declared height-estimation method and error bounds) for urban counter-UAS navigation, street-canyon radar/RF shadowing, and shrapnel shielding—never conflated with surveyed building truth.

### 4.5 Headless Batch/RL Simulation Kernel + Modular Synthetic Sensors
Aligned with [EMAD_ARCHITECTURE_AND_DIRECTION_v2.md](../EMAD_ARCHITECTURE_AND_DIRECTION_v2.md):
- **Decoupled compute vs. rendering:** Keep the physics/kinematic simulation runtime **headless and deterministic**, capable of running far faster than real time (for Monte Carlo shrapnel distributions, parameter sweeps, and RL training), while emitting compact replay artifacts for the Three.js / native 3D viewer and Exercise COP.
- **Pluggable fidelity profiles (`fidelity_profile`):** Support clean progression from fast `kinematic` / 3-DOF ballistic-with-drag-and-wind models up to higher-fidelity specialist models without changing downstream sensor, COP, or evaluation contracts.
- **Reusable Synthetic Sensor Interface:** Standardize sensor modules (terrain-raycasted radar LOS + range/RCS/noise, weather-degraded EO/IR, RF/acoustic, spoofed/delayed ADS-B/AIS) that read `EXERCISE_TRUTH` internally and emit strictly scoped `EXERCISE_OBSERVATION` records for tracking, fusion, and AI decision agents.
- **Benchmarking Harness:** Provide a deterministic step/episode and scoring interface so classical algorithms, optimization solvers, and AI agents can be compared reproducibly against sealed `EXERCISE_TRUTH`.

---

## 5. Strategy for the First Basic Simulation (Non-Throwaway Vertical Slice)

The first simulation should remain **small and easy to experiment with**, while being built directly on **real, permanent EMAD infrastructure** rather than disposable prototype code:

1. **Consume real verified environment packages directly:** Run identically against both the git-committed owned fixture (`testdata/geospatial/owned-fixture-v1/prepared.zip` + `field-era5.json`) for automated regression tests and real local Riyadh prepared releases (`surface` DSM + `buildings`/`roads` + `era5-pressure.json`).
2. **Deliver permanent runtime primitives on day one:** Build the reusable in-memory Terrain/LOS Engine (§4.1) and 3D Wind Field Interpolator (§4.2) as clean, tested modules that every future simulation will import.
3. **Preserve EMAD contracts from the start:** Pin environment manifest SHA-256, seed, fidelity profile, and ENU origin in every run manifest; keep `EXERCISE_TRUTH` separate from synthetic `EXERCISE_OBSERVATION` outputs; emit reproducible run recordings that can be inspected both programmatically (for AI/optimization benchmarking) and visually in 3D.
4. **Prove a real decision/optimization question:** Even in its simplest form, the simulation should allow plugging in at least two strategies (e.g., naive baseline vs. optimizer/AI) and measuring a concrete outcome metric (such as debris-exposure reduction, detection time, or intercept margin) under real terrain and wind data.
