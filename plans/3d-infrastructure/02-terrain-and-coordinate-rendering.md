# V02 — Terrain mesh generation and coordinate rendering

[Plan index](../README.md)

**Status** Riyadh terrain v2 reproduced the installed GLB. Aligned seams, raster and polygon masks, and disconnected components passed fixture verification. The viewer supports geographic picking and handoff to Atlas. Independent physical accuracy remains open. [Terrain and coordinate evidence](../../docs/implementation/RIYADH_DURABLE_GEOGRAPHIC_PREVIEW.md#v02-native-terrain-and-coordinates)

**Implementation owner** Geospatial and 3D engineering.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V01 — Browser asset delivery and derivative publication](01-browser-assets-and-publication.md)

## Outcome and baseline

An approved surface appears at the correct position and height in a bounded 3D preview with visible data gaps. Existing geographic packages preserve native elevations and a separate geoid. The current Three.js scene viewer reads a different historical asset contract.

## Features and contracts

Build a GLB surface derivative with an explicit WGS84 origin, ENU coordinates, renderer-axis mapping, applied vertical conversion, mask, extent and source identity. Use the exact pinned geoid for EGM2008 conversion. Apply the conversion once and label the resulting height representation.

Generate triangles only for supported valid cells. Record decimation and display error bounds if a lower-detail mesh is requested. Preserve holes and seams. Use local coordinates for rendering precision and the existing geodetic transform for world positioning.

## Implementation sequence

1. Define deterministic terrain recipes and small fixtures for grid registration, gaps and adjoining tiles.
2. Produce a native-grid mesh for the first Riyadh study area and verify vertex positions through an independent transform.
3. Add normals, bounds and a useful camera fit without assigning physical material properties.
4. Implement a display resolution option only with a declared geometric error measurement.
5. Exercise a second local frame in a bounded Saudi or Middle Eastern study area. Use retained regression meshes to check memory, repeated navigation and context loss across different workloads.
6. Evaluate 3D Tiles or a geospatial renderer only after V08 identifies a workload that needs streamed spatial detail.

## Acceptance

- Selected independent coordinate controls meet the existing numerical transformation tolerance.
- Mesh vertices agree with the declared source-to-frame pipeline and preserve source uncertainty.
- The renderer’s east/north/up orientation and geographic picking agree with Atlas.
- Nodata holes remain holes, and disconnected AOI parts are individually discoverable.
- A missing conversion grid or unsupported datum fails preparation.
- An inferred display simplification is reported with its recipe and error.
- Repeated rebuilds meet the derivative’s declared deterministic identity criterion.

## Rollout and recovery

Introduce a new geographic preview reader behind the existing environment workbench. Reuse renderer utilities where their contracts match. Maintain old Studio fixtures and readers through F10. Failed derivatives can be rebuilt from the unchanged geographic parent.

## Deliverable evidence

Deliver the Riyadh surface derivative first, with transform controls, seam and mask results, and triangle, byte and memory measurements. Record the same evidence for each area added. Physical accuracy remains the source report’s value until independently qualified.

## Implementation and source references

- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [internal/simulation/spatial.go](../../internal/simulation/spatial.go)
- [apps/operator-ui/src/features/simulation/spatial.ts](../../apps/operator-ui/src/features/simulation/spatial.ts)
- [apps/operator-ui/src/features/simulation/SceneViewport.tsx](../../apps/operator-ui/src/features/simulation/SceneViewport.tsx)
- [World Monitor src/components/GlobeMap.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/components/GlobeMap.ts)
