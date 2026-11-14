# Implementation sequence and shared contracts

[Plan index](README.md)

The existing environment library is the starting point. Preserve its verified behavior while implementing the gaps identified by the two reviews. Parallel work here means independent implementation branches or work items, with shared contract changes integrated in dependency order.

Dependencies identify contracts and results required for integrated acceptance. Teams can prepare schemas, source samples, UI fixtures and processing prototypes before every upstream system is fully accepted. Optional datasets and a separate-host rollout qualify their own declared targets.

## Increment 1 — Reconcile the foundation

Start F10 with a clean-checkout and contract baseline. F01 inventories retained sources and rights. F02 freezes the supported geographic meanings and F05 validates storage behavior. Prepare F03 review evidence while reviewers complete actual decisions. Implement F04’s remaining execution transport and F08’s metrics.

F06 makes independent verification repeatable. F07 rehearses the paired restore with the current inventory. F09 assembles foundation acceptance after its dependencies have evidence. The existing 24-hour observation must be read and evaluated as it actually ran. A new qualifying run needs a worker restart within its own observed window.

Prepare the first Riyadh reference release through F01–F03. Save a bounded Riyadh study area as a revision, inventory the available surface, geoid, dated imagery and vector inputs, and record coverage and rights per layer. Reuse retained inputs where their edition and coverage match. Acquire and admit the missing required inputs, then prepare, verify and stage the package through the normal workflow. Use a new intake profile revision to pin the area and source decisions.

**Exit** Document release, access, recovery, reproducibility and operational results for the installed candidate, including the Riyadh release and the pinned F09 workload. Verify Saudi country and region navigation and Middle East overview coverage separately from detailed releases. Existing fixtures can support upgrade and viewer development while Riyadh inputs and human review are pending.

## Increment 2 — Deliver one geographic preview end to end

Finish durable V01 display jobs on the existing terrain publication contract. Use the first Riyadh reference release to qualify V02 terrain and V03 optional imagery and vector display. Complete the V04 Environment Library entry and Atlas return path for the same release. Use the retained Saudi and Middle East reference layers for the first V05 local navigation mode.

Staged reviewer testing can validate the pipeline. Independent consumer acceptance uses a genuinely released candidate. A lost response, cancelled build, corrupt output and withdrawn source must each be exercised through the normal endpoints.

**Exit** One user can discover the exact Riyadh release, inspect its limits, open terrain, request available imagery or vectors, select a feature and return to Atlas with the same identity.

## Increment 3 — Complete resilient viewing and data handoff

Apply V06’s coverage/freshness states to existing source overlays. V07 materializes immutable geographic/observation bundles with explicit time interpretation. Finish V05’s local basemap dependencies and authorized portable package.

Use V08 to measure the Riyadh workflow on named hardware. Check Saudi navigation and coverage at national scale, then exercise regional discovery and a second detailed frame in a bounded Middle Eastern area with suitable sources. Existing fixtures can cover specific load and geometry regressions. Fix measured bottlenecks before selecting an additional renderer or tiling stack.

**Exit** The assembled viewer passes performance, access, compact-layout, disconnected-use and independent-consumer acceptance.

## Increment 4 — Enrich declared capabilities

Qualify D01 land cover, D02 ground terrain and D08 infrastructure dimensions for Riyadh first where suitable sources exist. Prioritize additional Saudi areas, then expand across the Middle East according to coverage and user workflows. Source and profile changes depend on the shared F01/F02/V03 contracts.

Activate D03 radar, D04 additional hazards, D05 weather fields, D06 bathymetry and D07 moving/orbital context when an actual consumer need is recorded. Required fields are use, AOI, interval, layer, units/reference frame, maximum resource cost, rights and a source-backed quality criterion. Availability checks and real sample acquisition precede implementation claims.

**Exit** Each activated layer has retained parents, a qualified representation, an inspected display and a portable verification result. A failed acquisition or unsuitable source remains an explicit capability gap.

## Shared identities and consistency

Use the existing scope and source authorization model for records, counts, assets and caches. A proposed derivative is identified by parent release/digest, profile/recipe/toolchain revisions and exact input members. Physical bytes have their own SHA-256 and length. Idempotency keys are scoped to the caller’s authorized operation, and a repeated key with changed content returns conflict.

Retain the current geographic build state machine. Derivative jobs use an explicit job/recipe discriminator so an older worker cannot claim unsupported work. Publication validates all outputs under the current live lease and commits one visible manifest. If database commit follows object upload unsuccessfully, reconciliation owns the unreferenced objects.

A parent can have multiple derivatives. Their eligibility depends on parent state, effective source rights and output policy. Every derivative delivery route checks that relationship. Policy changes govern subsequent online reads. Portable exports carry their own issued rights and retention terms.

## Proposed API additions

These routes are planning proposals to add to the existing OpenAPI source and generated client. Confirm naming against the current router during implementation.

- V01 submits a display build with `POST /api/v1/environment-releases/{id}/display-builds`, supplying expected manifest digest, recipe/profile revision and request key. Return a durable job identity and status location. Use the shared build/attempt mechanism.
- V01 reads `GET /api/v1/environment-displays/{id}`, `/manifest` and `/assets/{assetId}/content`. Return parent identity, layer metadata and authorized full/range content. The asset ID resolves through the derivative manifest.
- F04 claims work with `POST /api/v1/environment-worker/claim`. Heartbeat, input/output transfer, failure and completion use job-specific worker routes under `/api/v1/environment-worker/builds/{id}` with an authenticated service and lease proof. Responses declare toolchain and lease deadline.
- V07 adds a versioned environment handoff to the existing context-package API. Resolve its existing routes first and extend the shared service. Submit exact release/member identities and time predicates, then retrieve the immutable result by ID.

For all additions, define schema limits, required permissions, 401/403 behavior, unavailable-record semantics, 409 identity/state conflicts, 422 unsupported input, quota responses and retry policy in OpenAPI. Secrets and credentials never enter URLs or manifests. Malformed IDs and unsupported formats fail before expensive work.

## Shared compatibility and deployment

Use additive migrations, versioned producers and compatible readers. Deploy schema and API before workers, readers before new representation producers, and UI after the required capabilities are available. Drain or fence old worker attempts during incompatible processing changes.

Existing package and Studio readers remain regression fixtures. Preserve original source hashes and review evidence. F10 covers clean builds, generated contracts and changed-path regression checks. F07 establishes the actual rollback or forward-recovery behavior for each installed binary.

## Engineering targets and decisions

Foundation budgets are inherited from the original plan and recorded in F09. V08 adds proposed 3D viewer budgets, which require named hardware, browser, network and cache conditions before measurement. Dataset accuracy thresholds come from the relevant product and independent controls. Numerical transform tolerance is separately defined in F02.

The first implementation adopts the current storage and renderer stack. A larger object count, larger area, slower client or new layer may justify PMTiles, 3D Tiles or another renderer. Document the failing workload, options measured and dependency/operational cost before changing that decision.

Record API and data engineering, frontend, operations, quality and reviewer ownership for each increment. The plan names roles. A commissioning record names the actual people and supplied infrastructure. A separate host, provider credentials, purchased data or a human review is an explicit delivery input where required, while independent engineering work continues.

## Evidence and review

For each increment, check normal entry points and preserve accepted older data. Cover a happy path, denied scope, corrupt dependency, retry/partial failure and the capability-specific boundary cases listed in its plan. Re-run broader tests when shared code changes justify them. Store evidence in the repository’s established evidence structure and private payload storage.

F09 answers whether the geographic preparation service is accepted. V08 answers whether the map and 3D inspection workflow is accepted. Each D-plan answers whether its particular layer and intended use are qualified.
