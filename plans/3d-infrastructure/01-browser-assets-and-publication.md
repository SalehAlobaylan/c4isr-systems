# V01 — Browser asset delivery and derivative publication

[Plan index](../README.md)

**Status** Durable terrain, optical imagery, roads, buildings and water passed isolated Riyadh publication and exact readback. Shared rollout is pending. [Durable publication evidence](../../docs/implementation/RIYADH_DURABLE_GEOGRAPHIC_PREVIEW.md#v01-durable-publication)

**Implementation owner** Backend and geospatial delivery engineering.

**Dependencies** [F01 — Source admission, lineage and retention](../environment-foundation/01-source-admission-and-lineage.md), [F04 — Durable preparation workers and execution transport](../environment-foundation/04-durable-workers-and-transport.md), [F05 — Artifact delivery, integrity and storage lifecycle](../environment-foundation/05-artifact-delivery-and-lifecycle.md)

## Outcome and baseline

A browser can fetch verified displays independently of the environment archive. STAC links the ZIP, manifest and published displays for released environments. Reviewers can inspect staged displays through the scoped display registry. The terrain recipe also runs through the durable worker.

Use the Riyadh reference release for the first durable publication and measure its source, mesh and texture bytes and startup latency. Existing terrain measurements provide a regression baseline for the reader and storage path. [Terrain measurements](../IMPLEMENTATION_PROGRESS.md#native-grid-terrain-results)

## Features and contracts

Define a display derivative manifest containing schema, parent release ID and digest, exact input members, recipe/toolchain digests, local frame, layer semantics, coverage, output paths/digests/lengths, bounds, limits and current policy bindings.

Propose build submission, status, manifest and asset routes under the existing environment domain. Resolve IDs through scoped records and never accept arbitrary storage paths. Extend the STAC projection to advertise authorized derivative assets only after atomic publication. Preserve canonical source and ZIP identities.

Store display manifests and their members as a distinct artifact family linked to the parent release. Reuse the geographic streaming store and reservation accounting. Register explicit media types for GLB, textures and any selected vector-tile format in that family. Existing geographic source format and layer allowlists remain the validation boundary for original inputs. Consumer display reads require a released parent. Existing reviewer grants permit staged qualification where explicitly checked.

## Implementation sequence

1. Specify additive job/output records and generated OpenAPI contracts for display artifacts.
2. Reuse durable worker orchestration with a distinct recipe type and toolchain identity.
3. Stage outputs privately, validate every member and publish the derivative manifest in one fenced transaction.
4. Serve full and range reads through existing authorization and streaming machinery.
5. Add metadata-first loading, cancellation and retry to the client. Bind cached assets to scope revision and exact derivative identity.

## Acceptance

- Duplicate submission or completion yields one usable derivative publication.
- A crash between object upload and manifest commit exposes no partial scene.
- Missing or corrupt members block publication and generate bounded cleanup.
- Parent withdrawal or source revocation blocks subsequent derivative reads and STAC discovery.
- A renderer can load only terrain and inspect its meaning before choosing imagery.
- New derivatives coexist with old geographic packages and their verifier.
- Measured byte accounting includes metadata, meshes, textures, fonts and retries.

## Rollout and recovery

Publish the contract and reader before enabling producers. Start with bounded GLB and texture artifacts for Riyadh. Expand to other bounded Saudi and Middle Eastern areas after their sources and profiles are qualified. Use F05 reconciliation for orphan outputs and F07 for recovery inventory. The existing ZIP remains an independent portable delivery format.

## Deliverable evidence

Retain derivative manifests, publication fault tests, scoped range results, parent-policy checks and per-layer byte ledgers. V02 and V03 provide the actual data transformations, V04 consumes the outputs.

## Implementation and source references

- [internal/geoenv/http.go](../../internal/geoenv/http.go)
- [internal/geoenv/releases.go](../../internal/geoenv/releases.go)
- [internal/geoenv/build.go](../../internal/geoenv/build.go)
- [internal/artifacts/stream.go](../../internal/artifacts/stream.go)
- [api/openapi/environment-library.yaml](../../api/openapi/environment-library.yaml)
- [World Monitor scripts/_bootstrap-payload-budget.mjs](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/scripts/_bootstrap-payload-budget.mjs)
- [World Monitor src/services/bootstrap.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/services/bootstrap.ts)
