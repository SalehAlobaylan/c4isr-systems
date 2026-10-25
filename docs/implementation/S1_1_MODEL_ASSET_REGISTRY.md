# S1.1 - Model and environment asset registry

Delivery date: 2026-10-22. This increment implements the first S1 task from the active v2 plan. It preserves P3.3 and migrations 1-33. The next task is **S1.2 - Engine evaluation**; this delivery selects no engine and does not claim the S1/M4 scene proof has passed.

## Delivered domain boundaries

- `ModelAsset` is an immutable validated visual version with the original source artifact, checksum, source/import revision, license/admission author, importer version, validator/dependency fingerprint, worker build, material/texture summary, scene bounds, units/axes and fidelity.
- `ActorTemplate` is a separately versioned authoring definition referencing one validated model. Static and abstract waypoint-kinematic behavior declarations are supported. Speed bounds and limitations are recorded; templates do not instantiate an actor or execute behavior.
- `EnvironmentAsset` pins a validated model to the existing S0 frame contract: explicit WGS84 longitude/latitude, resolved ellipsoidal height, versioned ECEF/ENU transform, metre units and orientation. The declared model-axis mapping is glTF X east, Y up, Z south. No terrain is silently reprojected, loaded or certified by registry admission.
- `SimulationActor` is a separate runtime instance reference in the simulation contracts. Runtime instantiation, private world state and actor lifecycle remain S2 work. No new actor, controlled asset, track, observation, command or truth record is created by these registry operations.

The asset pipeline uses the existing private S3-compatible `artifacts.Store` and content-addressed writes. Public imagery admission/metadata remain unchanged: simulation authoring has separate metadata tables and object paths under `EXERCISE_INPUT`. Original authoring sources and externally produced engine variants have separate artifact records, hashes and purposes. Native variants pin a parent model version, inherit its complete license snapshot and identify their engine/build; they remain opaque, unexecuted stored objects. Storing one does not select an EMAD engine or certify compatibility.

## Import and storage lifecycle

An authorized human admits metadata, reviewed terms and a declared SHA-256/byte count. Reservations are idempotent by request key within the exact input resource; reuse with changed content/rights conflicts. Upload verifies the exact bytes, writes privately and rechecks live author access after external I/O. Models then enter the persistent validation queue. Authoring sources/engine variants are stored separately and never executed.

The standalone `apps/emad-assets` Go worker requires an input-bound service principal with `sim.assets.import`. It claims a bounded lease, runs a pinned Node/Khronos validation subprocess, validates embedded image payloads in the worker, stores the validation manifest separately, and atomically publishes the immutable model version, attempt and audit outcome. Worker execution is outside API startup and synchronous HTTP handlers. Revocation after validation prevents publication. Expired leases record an interrupted attempt before recovery; at most three attempts are permitted. Storage/process failures retry within that budget; invalid inputs are rejected with retained diagnostics. Source and manifest downloads recheck access and verify hashes/byte counts.

A validated model ID includes its exact source admission, source checksum, importer, worker build, validator/dependency fingerprint and validation-manifest hash. The manifest's original byte length and hash are retained, so JSONB normalization cannot change its downloaded bytes. New source/import revisions create new immutable records; no existing version is silently replaced. Rejected/failed originals and attempts remain inspectable.

## Supported portable profile and resource budgets

The current profile is deliberately small and explicit: core static glTF 2.0 and GLB, triangle primitives, a selected scene, valid node transforms, embedded buffers and embedded PNG/JPEG images. All external file/network references are rejected; no asset URL is fetched. glTF source and terms URLs are retained as references only. Archives, sparse accessors, compressed extensions, other extensions, skins, animations, cameras and morph targets are rejected by this profile.

Budgets are enforced before/alongside Khronos structural/data validation:

- At most 16 MiB uploaded file; at most 4 MiB JSON; at most four buffers and 16 MiB decoded buffer bytes.
- At most 128 meshes, 256 primitives, 256 nodes/visited instances, 16 scenes, 1,024 accessors/views, and depth 32.
- At most 250,000 rendered vertices and 250,000 rendered triangles. Repeated mesh instances consume the rendered budget rather than counting the shared mesh once.
- At most 64 materials, 32 textures and 16 images. Each image is at most 4,096 by 4,096 pixels; aggregate image pixels are at most 16 Mi. Compressed image bytes are bounded, dimensions are checked before full decoding, and PNG/JPEG payload/CRC/decode failures reject the model. No archive decompression is performed.
- Node validation has a 15-second subprocess deadline, a 256 MiB V8 heap limit and bounded stdout/stderr. Truncated Khronos issue reports reject the import rather than certifying an incompletely reported result. The whole worker invocation has a 45-second context and a 60-second database lease. Image decode runs only in the worker with byte/pixel budgets and deadline checks. These limits are not an assertion that all engine loads have a 256 MiB total-RSS budget.
- Scene-coordinate bounds are finite and limited to 500,000 metres per axis. Bounds come from validated accessor data and the selected scene hierarchy, not an unverified caller box. Frame placement/scene loading and their resource measurements belong to S1.2-S1.3.
- Each input environment admits at most 1,000 source/variant/model upload records and 512 MiB reserved original bytes; at most 1,000 actor templates and 1,000 environment definitions. Validation manifests and at-most-three attempt records are separately bounded. Lists return at most 100 items with a truncation flag; pinned model versions and authoring definitions remain addressable by ID.

This is a bounded profile validator, not a claim to accept every valid glTF extension or provide specialist dynamics. The retained importer/validator versions and warnings expose that scope. Opaque source/native artifacts are attachments only and are never fed into a renderer, shell or interpreter.

## Licensing and reproducibility

Admission records the license ID/version, legal basis, source reference/version, terms URL, attribution, storage/simulation permission and separate redistribution permission. This first append-only registry requires an explicit perpetual source-archive permission; unsupported retention terms require a future retention design. A portable model linked to a retained authoring source preserves that source's complete reviewed license snapshot. Engine variants preserve the parent model's snapshot. Human metadata admission is an explicit review record, not automatic interpretation of upstream legal terms.

Three tiny fixtures are authored by `scripts/data/prepare_model_fixtures.py`: an abstract wing triangle and marker in GLB, and a synthetic ground tile in embedded glTF. `testdata/models/README.md` records their internal fixture-use terms. They contain no third-party binary model inputs, surveyed terrain or physical/sensor performance claims. Their source generator and exact files are retained in the repository. They are suitable for repeatable pipeline testing; licensed production models still require their own admission review.

## UI and API

In an authorized Exercise COP, open **Scenarios -> Simulation asset registry**. The page requests the same workspace/environment's `EXERCISE_INPUT` resource explicitly and verifies its author permission separately. COP access alone grants no access to hidden authoring assets. Current browser entry uses the existing COP shell; input-only principals can use the dedicated API, while a broader Studio shell remains S1.3 work.

The page supports source/license review, checksummed uploads, import-state/attempt inspection, exact source/manifest downloads, validated geometry/material metadata, actor-template authoring and explicit environment-frame registration. Model/upload selection is URL-backed. Workspace query caches and cancellation remain isolated; a public workspace cannot open the authoring view. The display is a bounded responsive DOM list/inspector with labeled native controls, ordinary scrolling and on-demand provenance. It adds no renderer or GPU dependency.

All routes are under `/api/v1/simulation-assets/` and require an explicit input-resource `sim.scenarios.author` grant:

- `GET/POST uploads`, `GET uploads/{id}`, `PUT/GET uploads/{id}/content`, `GET uploads/{id}/attempts`.
- `GET models`, `GET models/{id}`, `GET models/{id}/manifest`.
- `GET/POST templates`, `GET templates/{id}`.
- `GET/POST environments`, `GET environments/{id}`.

Writes additionally require a human principal. There is no HTTP endpoint that lets a caller submit a successful validation result or execute a worker. `EXERCISE_INPUT` is admitted only on these dedicated authoring routes and the existing auth introspection endpoint; ordinary COP/realtime routes still reject input/truth partitions. Forced RLS and same-resource foreign keys protect the tables independently of handler checks. Source/license snapshots and model versions/definitions/attempts are immutable, with scoped audit records for admission, definitions and import completion.

## Rollout

1. Apply **migration 34** after the already applied migration 33: `task db:migrate`. Startup checks the new registry tables. This delivery did not migrate the user's database; automated migration acceptance uses disposable PostGIS. Existing migrations and the migration shell script are preserved. Recovery is forward-only.
2. Reuse the private object-store configuration documented for P2 imagery. Configure the same `C4ISR_ARTIFACT_S3_*` credentials on the independent asset worker; no public bucket or public signed URL is introduced.
3. Run `pnpm install --frozen-lockfile` to install the exact pinned Khronos validator package. The worker needs Node 20+ and the trusted `apps/emad-asset-validator/validate.mjs` wrapper with its installed dependencies. Its fingerprint includes the wrapper, package metadata and validator JavaScript bundles.
4. Adapt `scripts/data/model-asset-author-grant.example.json` to the intended existing human principal and input environment. Add that explicit grant with the existing control-plane command: `go run ./apps/emad-admin -action grant -spec <author-grant.json>`. It grants no runtime truth. Browser entry also needs the existing authorized Exercise COP session in that workspace/environment.
5. Adapt `scripts/data/model-asset-worker.example.json` to the same input environment and provision a separate service credential: `go run ./apps/emad-admin -action provision -spec <worker.json> -out <private-token-file>`. Store/rotate that private credential using the established administration workflow.
6. Start validation explicitly: `go run ./apps/emad-assets -token-file <private-token-file> -validator-script apps/emad-asset-validator/validate.mjs`. Add `-once` for at most one claimed job. API startup never launches this process. Missing dependencies, storage failure and process failure remain observable in bounded attempt history; repair the worker configuration before admitting a new import revision after a terminal attempt budget.
7. The accepted scene/context, engine comparison and first renderer remain S1.2-S1.3. Asset admission does not modify frozen S0 scenarios or add an unreviewed public-to-simulation import path.

## Validation evidence

Logs and screenshots are retained in `tmp/s1-1/`:

- `validator-tests.log`: 6 native profile checks covering GLB/embedded glTF, actual scene transforms, false accessor bounds, external resources/path traversal, invalid indices, unsupported extensions/archives, mesh instance budgets and texture metadata bounds.
- Go tests exercise admission rights/profile limits, bounded subprocess output, actual image payload validation and cancellation. The final `api-final.log` reruns both asset acceptance workflows after the strict issue-report limit was added.
- Database/API acceptance covers real S3 uploads/downloads, deduplication, exact validation manifests, source/license immutability, independent original/native variants, actor/environment definitions, invalid geometry, explicit frames, no operational instantiation, cross-environment/partition denial, worker revocation and expired-lease recovery without duplicate model versions.
- `acceptance-browser.log` and the final `browser-final.log`: **8 production-UI/backend browser workflows passed**, including the new import-to-template/environment path, checksum-preserving downloads, pinned URL/reload, public-workspace denial and all previous imagery/public-context/moving/workspace flows. Desktop/mobile screenshots: `asset-registry-desktop.png`, `asset-registry-mobile.png`.
- Full PostGIS regression: **76 passed cases** (52 top-level tests and 24 subcases); 4 opt-in skips are the 3 live provider smokes and the separately executed browser gate. No failures. Log: `integration-final.jsonl`.
- Existing mocked browser suite: **74 passed / 50 intentional project skips**. Log: `browser-mocked.log`.
- `go test ./...`, `go vet ./...`, affected-package Linux race tests, UI typecheck/production build and OpenAPI lint passed. SQLC/TypeScript regeneration stability, deterministic fixture regeneration, UTF-8/JSON/Go formatting and `git diff --check` also passed. Migrations 1-33, the user’s `.gitattributes` and `scripts/ops/migrate.sh` match their pre-task hashes. The UI build retains the existing large-chunk advisory.
- Environment: Go 1.25.0 Windows/amd64, Node 24.13.0, pnpm 10.14.0; disposable PostGIS 17/3.5 and the existing pinned Garage S3 fixture. No GPU or engine performance claim is made.

## Primary format references

- [Khronos glTF 2.0 specification](https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html).
- [Khronos glTF Validator](https://github.com/KhronosGroup/glTF-Validator); pinned package `gltf-validator@2.0.0-dev.3.10` and its dependency integrity are recorded in `pnpm-lock.yaml`.
