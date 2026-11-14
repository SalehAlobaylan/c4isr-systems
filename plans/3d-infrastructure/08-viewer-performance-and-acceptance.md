# V08 — Viewer performance, accessibility and acceptance

[Plan index](../README.md)

**Status** Private staging passed desktop and compact workstation budgets and 30 Atlas-to-3D round trips. Physical mobile and an independent receiving participant remain open. [Implementation evidence](../IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification)

**Implementation owner** Frontend performance and quality engineering.

**Dependencies** [V04 — Shared layer catalog and Atlas-to-3D handoff](04-layer-catalog-and-atlas-handoff.md), [V05 — Local basemaps and offline viewing packages](05-local-basemaps-and-offline-packages.md), [V06 — Context overlays, freshness and coverage](06-context-overlays-and-freshness.md), [V07 — Frozen environment and observation handoff](07-frozen-environment-context-handoff.md), [F08 — Operations, metrics, alerts and commissioning](../environment-foundation/08-operations-metrics-and-commissioning.md)

## Outcome and baseline

The receiving team can reliably inspect the Riyadh environment on declared hardware and navigate Saudi Arabia and the broader Middle East. The workflow includes a compact screen and WebGL failure. Existing 0.178-second release previews and browser checks measure the library UI. The 3D path requires its own measurements.

## Features and contracts

Measure cold and warm time to usable terrain, all requested bytes by layer, request failures, p95 active-navigation frame time, decoded bytes, GPU resource counts and available memory measurements. Include retries and failures in the result.

Adopt proposed initial engineering targets of usable terrain within 3 seconds and p95 frame time within 33.3 ms on the named desktop baseline, and 5 seconds/50 ms on the named compact baseline. Record CPU/GPU/RAM, browser, viewport, network shaping and cache state before testing. These are proposed qualification budgets, and any change needs a dated rationale before the acceptance run.

## Implementation sequence

1. Establish the hardware and repeatable camera/inspection path, including local-network and disconnected modes.
2. Use Riyadh for the first terrain, imagery, vector and Atlas round trip. Verify national and regional navigation, then exercise a second local frame in a bounded Saudi or Middle Eastern study area. Use retained regression fixtures to exercise additional geometry and load cases.
3. Optimize within the existing renderer using demand loading, viewport clipping, cancellation and resource disposal.
4. Test 30 map-to-3D round trips and inspect for monotonically growing resource counts or retained content.
5. Exercise keyboard access, 390px portrait, 740px landscape, context loss and permission changes.
6. Evaluate deck.gl or 3D Tiles/Cesium only with a recorded failing workload and a measured comparison.

## Acceptance

- The Riyadh release and each detailed area selected for rollout meet the recorded time and frame budgets on the nominated baselines.
- Selecting a Saudi region or another Middle Eastern area preserves its area identity and shows the available layers and coverage for that selection.
- Terrain loads before optional imagery, and loading failures preserve a useful navigable view.
- Feature selection and source explanations remain available at compact sizes and through keyboard controls.
- Owned geometries, textures, workers and listeners return to the expected baseline after teardown.
- A failed WebGL renderer offers authorized list/map access.
- Display reductions are disclosed and retain stable feature identity.
- The receiving participant completes the whole inspection workflow.

## Rollout and recovery

Record a bounded initial rollout and compare real measurements to qualification. A renderer feature can be disabled independently while data remains readable through approved existing paths. Record any unmet device class explicitly.

## Deliverable evidence

Deliver performance traces, byte ledgers, lifecycle evidence and a real consumer walkthrough. Choose any additional renderer through the measured follow-up comparison.

## Implementation and source references

- [apps/operator-ui/src/features/workspace/AtlasCanvas.tsx](../../apps/operator-ui/src/features/workspace/AtlasCanvas.tsx)
- [apps/operator-ui/src/features/simulation/SceneViewport.tsx](../../apps/operator-ui/src/features/simulation/SceneViewport.tsx)
- [apps/operator-ui/tests/e2e/environment-library.spec.ts](../../apps/operator-ui/tests/e2e/environment-library.spec.ts)
- [World Monitor src/utils/globe-marker-budget.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/utils/globe-marker-budget.ts)
- [World Monitor scripts/_bootstrap-payload-budget.mjs](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/scripts/_bootstrap-payload-budget.mjs)
- [World Monitor src/components/MapContainer.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/components/MapContainer.ts)
