# S1.2 — Rendering engine decision

Accepted 2026-10-22 for the bounded S1 scene and S2 presentation boundary. This supersedes the earlier statement that no initial renderer has been selected.

EMAD uses **Three.js 0.180.0** for its first integrated Studio viewport. A separate, compiled **Go `emad-runtime` executable** owns the initial headless abstract kinematics. Browser scene objects are presentation state; they never become authoritative actor truth. The existing YAML adapter remains supported. The rendering choice does not select a specialist physics engine or commit later native GPU workloads to a browser.

## Requirements and selection scope

The initial workload is a local georeferenced scene with at most 16 abstract actors, eight validated models, 500,000 model vertices/triangles, 32 MiB of source geometry, 32 million decoded texture pixels, 4,096 terrain samples and a 10 km radius. glTF Y-up maps to ENU east/up/south in the renderer. S0 WGS84/ECEF/ENU transformations and runtime positions use float64; shader buffers use local float32 coordinates. Acceptance requires under 2 mm of sampled local float32 quantization error, not a claim of 2 mm terrain accuracy or a complete bound on rasterization error.

The renderer must integrate with React without owning routing, identity or application state; load the admitted self-contained glTF profile; support picking, camera controls and a 2D/DOM alternative; work on Windows/Linux browsers; retain offline model bytes and license notices; and dispose GPU resources when a scene or authorization scope changes. Headless execution cannot require a browser, graphics driver or display server. Behavior code is compiled project code rather than arbitrary uploaded scripts. Native rendering/physics may later be a separate adapter to these same contracts; native WebView embedding is not implemented by this increment.

Prototype budgets on the declared machine are under 1 s to the shader-ready first frame from local fixture data, under 200 ms for the fixed 120-frame batch, and under 256 MiB end-of-test RSS / 3 s CPU for each no-GPU scene test. These are small-scene admission checks, not production or maximum-library performance guarantees. The runtime publishes at 2 Hz while its local clock loop runs at 20 Hz; it has no GPU dependency.

## Candidates

Three.js and Babylon.js 8.32.0 were the runnable shortlist. Both use JavaScript/TypeScript, can parse the existing portable assets, integrate directly into the present browser stack, and offer permissive distribution licenses. Babylon provides a built-in no-GPU NullEngine and more engine systems. Three exposes a smaller renderer/scene graph; EMAD must own authoring, lifecycle and later physics/sensor adapters. Babylon's additional features are valuable when a concrete workload needs them, but are not used to imply simulation fidelity here.

Godot, Unity and Unreal were reviewed as alternatives, **not benchmarked**. Godot supports headless servers and native desktop delivery, with distinct large-world and web-export constraints. Unity and Unreal introduce separate editor/export toolchains and project-specific licensing/distribution considerations. They remain plausible candidates for future native GPU or physics requirements. No installed/editor builds, performance numbers, or rejection on technical capability are claimed for those three. This decision compares the two adapters that meet the current embedded-web workload; it does not declare a universal winner among game engines.

## Measured evidence

The reproducible harness is `apps/operator-ui/scripts/evaluate-engines.mjs`, with shared data in `engine-evaluation/fixture.mjs`. It builds a standalone comparison, creates five alternating fresh browser contexts for each candidate in one Chromium process, loads the exact ground tile, abstract wing and marker files, adds the same synthetic terrain, checks the same coordinate fixtures, and submits 120 identical position updates. It waits for shader readiness before the measured batch. Both candidates must render **13 triangles in four draws**. Node tests additionally load the same assets/terrain and execute 20,000 identical 50 ms waypoint steps without a GPU. Asset hashes and every individual sample are retained in the JSON evidence.

Machine: Windows 11 build 26200, AMD Ryzen 9 7900X / 24 logical CPUs, 33,479,524,352 bytes RAM; Node 24.13.0 and Chromium 153.0.8010.12. Hardware samples identify ANGLE / NVIDIA GeForce RTX 4070 Ti SUPER / Direct3D11, driver 32.0.16.1656. Integrated AMD graphics are present but were not the measured renderer.

Medians from [the hardware run](evidence/S1_2_ENGINE_BENCHMARK_HARDWARE.json):

- Three.js: **49.5 ms** shader-ready load; **5.5 ms** wall time and **1.350 ms** GPU elapsed time for the 120-frame batch; **54.68 ms** Chromium renderer task time for the whole measured page. No-GPU test: **45.36 ms** load, **93 ms** process CPU, **74.41 MiB** end RSS.
- Babylon.js: **357.2 ms** shader-ready load; **15.0 ms** wall time and **6.409 ms** GPU elapsed time for the same batch; **118.59 ms** Chromium renderer task time. No-GPU test: **1,979.25 ms** load, **2,046 ms** process CPU, **163.36 MiB** end RSS.
- Both: maximum sampled local float32 quantization error **0.000580 m** and headless waypoint end-position error **0 m** for the declared fixture. The native runtime's separate spatial/kinematic tests verify S0 alignment and state at intermediate virtual instants.

The [software-renderer run](evidence/S1_2_ENGINE_BENCHMARK.json) identifies SwiftShader explicitly. Its GPU timer is unavailable and recorded as null. Browser heap measurements are privacy-rounded, omit driver allocations and are not peak memory; RSS is an end sample. GPU figures are elapsed query time for this batch, not device utilization percentages. Contexts are fresh, while process/driver caches may remain warm. Other host activity, tiny geometry, material defaults and import/toolchain scope limit extrapolation. This is sufficient to choose the current bounded adapter, not to forecast advanced terrain, imagery, animation, physical simulation or large model libraries.

## Decision and consequences

Both pass the prototype budgets. Three's measured load, process footprint and current integration cost favor the small renderer plus explicit EMAD contracts. Studio ships a lazy viewport chunk of about 601 kB minified / 154 kB gzip in this build; Babylon remains an evaluation-only development dependency and is excluded from normal Vite entry discovery. There is no continuous render loop for a static authored scene; camera/selection/resize changes redraw it. DPR is capped at 1.5, model bytes are hash-checked before parsing, and exact properties plus a 2D overview remain available.

EMAD owns a narrow render adapter, not a replacement general-purpose game engine. A new physics, native embedding, high-rate sensing, animation, global terrain/3D Tiles or large-world workload must first define fidelity and performance requirements, compare appropriate engines on that workload, and record a compatible adapter/migration decision. Preserve frozen versions, portable model sources and truth/perception separation during that work.

## Reproduction and primary references

After `pnpm install --frozen-lockfile` and installing Playwright Chromium, run `node apps/operator-ui/scripts/evaluate-engines.mjs`. On Windows, set `EMAD_BENCHMARK_GPU=1` for the Direct3D11 hardware run. The default portable run identifies the actual software/device renderer and does not silently label it hardware. CI runs `evaluate-headless.mjs three` and `evaluate-headless.mjs babylon`; benchmark timings are evidence, not flaky CI thresholds across machines.

- [Three.js GLTFLoader](https://threejs.org/docs/pages/GLTFLoader.html), [WebGLRenderer](https://threejs.org/docs/pages/WebGLRenderer.html), and [MIT license](https://threejs.org/license/).
- [Babylon server-side / NullEngine documentation](https://github.com/BabylonJS/Documentation/blob/master/content/setup/support/serverSide.md) and [Babylon.js source/license](https://github.com/BabylonJS/Babylon.js).
- [Godot large-world coordinates](https://docs.godotengine.org/en/stable/tutorials/physics/large_world_coordinates.html), [headless command-line support](https://docs.godotengine.org/en/stable/tutorials/editor/command_line_tutorial.html), [web export](https://docs.godotengine.org/en/stable/tutorials/export/exporting_for_web.html), and [MIT license](https://godotengine.org/license/).
- [Unity Editor Software Terms](https://unity.com/legal/editor-terms-of-service/software) and [Unreal licensing](https://www.unrealengine.com/license). These are project/licensing research inputs, not legal or financial advice or assertions of EMAD's eligibility for a particular tier.
