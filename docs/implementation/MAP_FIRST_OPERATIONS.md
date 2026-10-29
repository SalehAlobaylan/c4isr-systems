# Map-first operation and upgrade



The default environment-loaded server profile is `C4ISR_PRODUCT_PROFILE=map`. `historical-full` explicitly restores the preserved historical facilities. The ordinary map identity is `local-map-analyst`; `local-source-admin` separately administers source installation and purpose classification. Existing identities and grants are preserved.



## Local operation



With the supported application supervisor stopped, run:



```powershell

task local:setup

task dev

```



Setup applies forward migrations through `00051_catalog_and_imagery_workflows.sql`, builds the public executables, provisions the map identities and retains existing data. Public setup installs the existing admitted reference/provider configuration; it does not choose additional regions or grant broader retrieval authority. Optional providers remain subject to their installed credentials and rights. The supervisor uses the map analyst and starts imagery/preparation/configured public connectors, without runtime, asset or recording workers.



`task local:doctor` checks the map profile. `task local:stop` stops only the registered supervisor; unrelated processes are never killed. Setup refuses to replace binaries or migrate while the configured application ports are occupied. This implementation pass does not shut down a running installation or silently change its active profile.



Historical operation is deliberate:



```powershell

task local:setup:historical

task dev:historical

```



An existing ordinary credential is not broadened into a simulation credential. New map profiles do not require simulation token files or a runtime descriptor. The server refuses exercise resource selection in map mode; historical operation needs its explicit profile and original grants.



## Area, period and saved views



The T04–T06 API/workers/UI require migration `00046_analyst_scope.sql` even if migration 00045 has already been applied. Use the coordinated setup/upgrade above before starting the updated binaries. Readiness checks require the new scope tables. Existing source restrictions still apply to every source retained in a scope, including reference geography; workers need those contributing source grants to process the submitted selection.



The map owns one area and period. Camera movement changes presentation; **Use this view** creates a deliberate viewport selection. Date-line components have separate navigation controls. Beyond ±85° the display restriction is explicit and original WGS84 geometry remains retained. Capture/export require a supported bounded local rectangle inside the exact selected polygon, including its holes.



The time control selects a rolling monitoring preset or a fixed historical interval, a supported revision interpretation and a display timezone. New scope intervals include the start and exclude the end. Existing legacy API interval semantics remain unchanged. Historical imagery outside a monitoring period requires the background-imagery option and remains labeled by acquisition date. An unsupported historical weather/track feed is not reconstructed from its current state.



Workbench drafts either follow the map or visibly pin a scope. **Use current map scope** clears affected scene/pair choices and previews while adopting area and period. Completed work retains its frozen scope. Bookmarks/share links store view presets, not investigations. Invalid, conflicting or inaccessible links show recovery; **Start a new view** deliberately returns to a fresh scope.



Related map families in a batch share a materialized read. Temporary read manifests expire after five minutes and are bounded to 16 active/64 MiB combined per principal/resource, 8 MiB each and at most 5,000 exact event revisions. Narrow a query or use aggregates when a budget is exceeded; let an expired view restart explicitly. A saved citation promotes its bounded result into durable scope evidence before expiry. Expired cursor cleanup never deletes that durable evidence. Changing access invalidates existing cursors and clears protected client caches.



Geographic administrators can admit complete release manifests and atomically activate a per-source release head using the expected previous head. Staged editions do not replace a released hierarchy; saved exact IDs keep their edition. Region profiles pin releases or named study areas. Broader regional preparation is implemented in T08; installing and qualifying additional shared regions remains an explicit administration step. See the [implementation report](MAP_FIRST_T04_T06_IMPLEMENTATION_2026_10_26.md) and [OpenAPI contract](../../api/openapi/analyst-scope.yaml).



## Purpose classification



Authorized source administrators use **Systems → Sources & access → Content purpose review**. The inventory lists only objects permitted by current resource/source and underlying domain grants. It provides unclassified/all-purpose views, exact record identity, receipt/reason, and expected-version updates. New classification never rewrites original evidence, source dates or review status. A development/acceptance assessment cannot be relabeled as a working publication; create a separately reviewed successor.



The installer uses exact reference hashes and imagery/output IDs from retained installation receipts, plus the dated [evaluation inventory](MAP_FIRST_CONTENT_PURPOSE_INVENTORY_2026_10_25.json). It never classifies from place names, scene titles or conclusion text. Backfill is part of the subsequent successful local setup. It can also be reviewed and applied separately against the upgraded API:



```powershell

$env:EMAD_INSTALLER_PROFILE = 'local-source-admin'

.venv-recovery/Scripts/python.exe scripts/data/classify_content.py

.venv-recovery/Scripts/python.exe scripts/data/classify_content.py --apply

```



The first command produces a preview under the private installation data directory. `--apply` appends the classifications. Missing/not-authorized historical IDs remain explicitly unavailable. A receipt hash mismatch or conflicting later human classification stops the backfill; it does not overwrite that decision. Remove a temporary `EMAD_INSTALLER_PROFILE` override after the administrative session if appropriate.



Catalog GET requests accept `content_view=ordinary` by default, including unclassified records. `working`, `unclassified` and `all` require source-administration authority plus each catalog's original read permission. Exact historical reads keep their original authorization. Counts label unclassified content; reviewed working-output totals require explicit `working` classification and canonical review identity.



## Staging and recovery



The delivery composition defaults to the map server and public workers. The map worker image omits runtime/asset/recording executables. To build historical delivery, set `C4ISR_PRODUCT_PROFILE=historical-full` and enable the Compose `historical-full` profile deliberately. API and worker build arguments must use the same profile; rebuild when switching profiles. Provision/copy an appropriate scoped UI credential rather than using an exercise-only credential for a map deployment.



Map backups preserve all existing database/object history. The restore rehearsal selects the profile recorded in private configuration, validates available map references/captures and reproduces installed imagery when the receipt declares it. Historical model/recording checks run only for a historical profile with corresponding receipts. Old backups lacking a profile retain their historical interpretation. Database/object backup still requires the existing coordinated quiescence procedure; complete expanded workload/restore qualification belongs to T22.



This change has isolated migration/startup tests and a Linux image check. Apply a reviewed upgrade during the installation's normal maintenance window; keep its paired backup and original profile configuration for recovery.



## Upgrade for investigations, profiles and durable acquisition (T07–T12)



The latest working tree requires migrations **00047–00050**, in addition to the preceding 00045–00046. The user reported applying earlier migrations; this implementation session did not change the shared database or restart its processes. The [T07–T09 report](MAP_FIRST_T07_T09_IMPLEMENTATION_2026_10_26.md) and [T10–T12 report](MAP_FIRST_T10_T12_IMPLEMENTATION_2026_10_26.md) distinguish isolated verification from installed qualification.



Use the supported coordinated upgrade after the paired backup described above:



```powershell

task local:stop

task local:setup

task dev

```



Review the trusted local provider configuration before setup/start. Setup applies forward migrations, rebuilds executables and reprovisions the maintained map identities, including `acquisitions.read` and `acquisitions.request`. Restart API, connector, imagery, raster and UI processes together. Readiness rejects a schema missing the scheduler tables. Do not leave older workers running during migration or roll back the schema: the cutovers fence old leases/counters/claims/artifact reservations, and retained inputs, immutable admissions and histories remain intact. A fenced capture/imagery attempt needs inspection before an explicit retry. Older in-flight external I/O cannot be undone; its stale completion must not commit.



The connector executable now rotates due, explicitly authorized profile revisions and durable bounded request parts. Existing source-only APIs continue addressing `default`. A new named profile needs a separately reviewed exact-revision service execution grant. Selecting a country or submitting a coverage-review request does not grant authority. Administrative profile approval remains in Sources & access; analyst requests are in Coverage & acquisition.



Trusted connector `accountBudget` fields are `group`, `dailyCost`, `requestCost`, `concurrency` and `minIntervalSeconds`. Use the same opaque group and consistent account limits for endpoints sharing an allowance; request costs may differ by adapter. The local USGS/current and USGS/history bindings share a group. Private `providers.json` may supply reviewed account policies; no API/source metadata can override them. Defaults are conservative installation caps, not assertions of a provider's remaining credit balance. A changed persisted account-policy fingerprint stops new calls until an operator reconciles it; do not erase the cost reservation history to reset a budget.



Unknown sends remain potentially spent until conservative UTC reset. Only an explicitly unsent reservation may be refunded. Expired concurrency/work leases do not refund uncertain provider cost. `GET /sources/{id}/acquisition-metrics` returns the authorized source's attempts, uncertain charges, queue age, active/storage reservations and complete/partial receipt counts, without revealing other sources/account identities. Inspect the request's parts, reason, next attempt and recovery owner before retrying. Keep the same request key for a lost response; use **Start a new acquisition intent** only for a deliberate new request.



Resource capacity defaults are 8 GiB retained/reserved bytes and 32 active reservations, with stricter existing source/domain limits retained. Raster claims reserve 64 MiB and renew with the job lease. Storage-full pauses new work and requires administrative storage/retention reconciliation. Retained evidence and PENDING uploads count toward usage. An orphan object is not automatically deleted; first reconcile it against all pending/retained references. History is not deleted to recover queue slots. Migration 00051 removes lifetime row/count ceilings and introduces saved history pagination; the active-slot and byte budgets remain enforced.



For regional reference preparation, run the existing installer with a reviewed JSON region profile, or use `scripts/data/regional_geography.py --prepare-only <manifest>` to prepare without application API calls. Staging never activates a partial release; retry the same chunks and expected release head. The default Middle East profile is configuration, and a seven-country source-backed sample was verified only in isolated PostGIS. This session did not install those releases into the shared system. General roads/DEM preparation remains T20.



Coverage uses historical receipt footprints and windows, with publication gaps, caps and rejected records preserved. A publication retry can reconcile an original receipt's outbox gap without another provider fetch; the original receipt remains immutable. Missing or empty records do not establish factual absence. Historical results keep their bounded materialized membership; explicit refresh evaluates a new result. Expired read cursors need a new read. Current provider withdrawal notices remain separate from the saved historical evidence.



## T13–T15 saved catalogs and imagery



Upgrade the API, capture worker and preparation worker together with forward migration **00051** using the existing supervised upgrade procedure. The user's preceding migration run is assumed through 00050, without independent shared-schema verification. Existing capture/job identifiers and retained evidence are preserved. The new intent ledger applies to newly submitted requests; it does not reconstruct old request keys from identifiers.



Catalog discovery uses `POST /api/v1/catalog/reads`. Keep the scope, filters, limit, snapshot ID and opaque cursor together. Totals belong to the saved read. Changed visibility/authorization, expiry or a mismatched cursor requires a new read. Explicit same-query refresh atomically releases the replaced ephemeral catalog; it never deletes durable content. Oversized reads need a smaller area, period or source. Compatibility imagery lists disclose their ceiling and use an unknown total when truncated.



Map aggregate drill-down uses `/api/v1/analyst-scope/reads/{id}/records`; do not reconstruct membership from a live offset query. The snapshot privately retains up to 100,000 ordered report revision IDs within the shared 8 MiB/read and 64 MiB/principal/resource budgets. Counts and reported geometry refer to that membership. Current withdrawal/supersession annotations are separate.



For imagery, inspect the selected boundary, provider footprint and suggested subset before deliberately adopting a rectangle. Use the comparison preflight before analysis. Preserve the original key/input for a lost response; **Start a new capture intent** explicitly creates another request. Case links identify exact saved revisions, retain current human membership checks and do not grant source or worker authority. An advanced case head does not rewrite a completed intent.



The local live-pixel qualification runner is opt-in: `python scripts/data/qualify_imagery.py --output <isolated-directory>` in the recovery Python environment. It makes anonymous public provider reads and retains historical STAC/windows/receipts. It does not approve production profiles or publish findings. Offline pairs can be checked with `go run ./scripts/ops/check_imagery_pair.go -before <subset.json> -after <subset.json> -out <result.json>`. Keep those outputs separate from complete application acceptance and independent review.



See [T13–T15 verification](MAP_FIRST_T13_T15_IMPLEMENTATION_2026_10_26.md) and the [observed-session protocol](MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md). Larger serial workload results do not qualify interactive p95, five-user concurrency or a 24-hour soak. The session's host-memory interruption required restoring the existing local PostgreSQL container; its accepting-connections/healthy state was checked, without applying a shared migration.



## T16–T18 case review and composite foundations



Apply forward migration **00052_case_workflows_and_outputs.sql** with the updated API/UI through the normal supervised upgrade. The user reported preceding migrations, assumed through 00051. This increment does not migrate or deploy the shared installation. Local provisioning now includes a distinct `local-map-reviewer` human profile; source-administrator and analyst aliases of the same human remain ineligible to review each other's work.



Preserve case save request keys and the original input when retrying a lost response. A recovered save returns its original immutable revision; do not replace its expected revision or silently resubmit it against a newer head. Resolve an actual conflict by reviewing the current case and deliberately preparing a successor. Device recovery remains scoped to the original canonical human/resource and confers no source rights.



Before review, inspect source/purpose/retention/scope notices and add an already authorized independent reviewer as a member. Review assignment grants no source access. Changes requests remain available when support has changed; approval requires current support. Revisions authored by a reviewer make that person a contributor who cannot approve the successor. Output approval requires READY bytes and binds the exact case/manifest/render. Real independent-human sessions still require observation under the T14 protocol.



Review source terms for each intended output purpose and approve a new rights/admission/reference version through source administration. `rights.outputPermissions` accepts explicit booleans for `internalAnalysis`, `citation`, `includedImagery`, `rawDistribution`, `geographicEnvironment`, and `metadataRetention`; absent/null means unknown and blocks output. Preserve the immutable original rights identity and use a new rights identity when terms change. New permissive current terms cannot retroactively widen unknown/denied original terms. Do not infer these permissions from `allowSimulationExport`. The rights API is available now; readable brief rendering remains T19.



Composite bytes use the configured private artifact store, under a separate composite namespace. Configure private storage before staging. Freeze a manifest, retain the exact bounded JSON/PNG rendering, obtain independent exact-render approval, and only then request approved distribution. Default foundation ceilings are 8 MiB per render and 4096 per PNG side and 4 megapixels in total; frozen reservations share the resource storage budget. Existing draft bytes remain pinned to their original case revision when the case advances; their superseded status precludes current distribution.



Reconcile expired outputs with `POST /api/v1/outputs/{id}/expire` or the output panel. Failed deletion leaves DELETE_PENDING and counts against capacity until a successful retry. Successful reconciliation retains only a minimal tombstone plus hashes/audit identities. Do not extend the frozen retention or restore expired bytes; a new output needs currently eligible evidence and a new independent review. Audit pending reservations/orphaned private objects after interrupted staging before production rollout; no unattended reaper is claimed by this foundation. See [T16–T18 implementation](MAP_FIRST_T16_T18_IMPLEMENTATION_2026_10_27.md).



## T19–T21 durable delivery and geographic preparation



Coordinate migration **00053_delivery_jobs.sql** with the updated API/UI and `emad-delivery` binary. The user reported migrations, without independent shared-schema inspection. This increment does not deploy or migrate the shared application. Preserve pending/finished job inputs, leases, attempt events and upload records during upgrade; forward recovery preserves immutable history.



The case delivery panel queues a readable brief or geographic package. Draft reading is separate from approved distribution. Inspect the retained output, select **Use exact rendered output for review**, assign an already authorized independent reviewer, and obtain approval for that exact manifest/render. The approved brief bundle includes frozen HTML, machine metadata, and readable/machine approval attestations. Opening it makes no live evidence/tile request. A case edit or changed support blocks current distribution; older permitted drafts remain pinned and labeled historical.



Run `emad-delivery -token-file <private-original-author-credential>` outside HTTP, or use the updated supervised local startup. It processes only that canonical creator's visible jobs and retains current source/case checks. Automation confers no review permission. Aliases are the same contributor. Local startup uses `local-map-analyst.token`; other creators need separately scoped worker instances. The optional staging Compose `case-delivery` profile uses `delivery-author.token`; supply that credential deliberately rather than giving a service every user's case access.



Workers use 150-second leases, at most three attempts, bounded retry delay/jitter and expired-lease recovery. Canceled jobs cannot publish. Retryable storage/deadline failures wait before another attempt; validation or support failures require correction/reassessment. A first upload binds its candidate hash/private key. Changing bytes requires a new output. Upload plans/completions support orphan reconciliation; uncertain writes keep deletion pending until their bounded deadline. Idle workers reconcile expired author-owned outputs with a per-row delay; explicit expiry remains available to authorized case users. Never delete referenced/unexpired inputs or restore expired bytes as ordinary recovery. Prolonged unattended and restore qualification remains T22.



Normalize retained geographic data with `.venv-recovery/Scripts/python.exe scripts/data/prepare_portable_geography.py --spec <spec.json> --output <new-private-directory>`. The spec names the exact WGS84 selection, height unit `metres`, grid size, local DEM paths/hashes/source identities/retrieval references, reviewed reference metadata/rights/accuracy, and optional vector parents. The tool performs no download or admission, preserves native parent files/receipts, and writes `prepared-references.json` for normal source administration. Mixed-source DEM mosaics are rejected; do not hide several providers under one source identity. Use an admitted complete bounded mosaic for the current preparation profile.



Approve exact source-backed accuracy and purpose-specific output rights before geographic processing. Missing accuracy, unknown permissions, incomplete DEM coverage or missing EGM2008 undulation grid cannot be repaired by increasing a tolerance until the job passes. Retain source/control provenance and select tolerances for the intended geographic use. Original unknown/denied rights cannot be widened by permissive current terms alone. Old references/artifacts can need a newly reviewed source version; changing a case citation grants nothing.



The product supports one bounded component (within ±85°, at most 2° per side), a WGS84 DEM mosaic, optional admitted EGM2008 undulation grid, vectors and qualified retained multispectral subsets. Imagery must be an exactly cited `imagery_subset` with declared horizontal accuracy and georeferencing. Source acquisition date/quality classes and outside/nodata masks remain explicit. Set the case's background-imagery option deliberately if an older acquisition is intended. Arbitrary PNG/TIFF files are not inferred to have a valid geographic frame. Unsupported CRS/datum/grid profiles fail before a full job; optional GLB is omitted.



Inspect the neutral package's instructions/manifest and verify independently with `.venv-recovery/Scripts/python.exe scripts/data/check_portable_environment.py <package.zip>`. It checks PROJ coordinates, controls, GEOS polygon containment, masks, hashes and triangle indices without an application/runtime session. This is not a surveyed-accuracy certificate or live rights check. Native GeoTIFF/OSM parents may remain external dependencies, identified by the retained receipts; preserve their permitted bytes separately. Geographic processing requires no actor setup, model-generation worker, runtime or replay. The historical Jeddah CLI is explicitly `prepare_environment.py --jeddah-fixture`; model fixtures remain in the historical simulation installer.



See [T19–T21 implementation and boundaries](MAP_FIRST_T19_T21_IMPLEMENTATION_2026_10_27.md). Full acceptance, real qualified terrain/control data, observed independent recipients/users, mixed-load timing and long-running/recovery qualification remain open.





## T22–T24 qualification and release



The preceding sections describe their dated increments. Current coordinated installation requires migrations through **00053** and the matching API/UI/public workers. The user reports applying migrations; shared schema, active binary versions and installed regional/source profiles were not independently inspected. T22–T24 introduce no additional migration.



Run a repeatable isolated sample with the recovery Python environment:



```powershell

.venv-recovery/Scripts/python.exe scripts/ops/qualify_map_first.py sample --output private/qualification/sample-001 --seconds 30

```



Use a new output directory for each run. Docker/Testcontainers start only disposable services. The command freezes the profile/source/hardware and preserves failures, errors/timeouts, per-operation cold/warm percentiles and heap/database/queue/retrieval-age samples. `--seconds 86400` measures a real 24-hour **reduced** sample; it still does not close the full dataset, browser, process restart, RSS/object-growth or independent-user gates. Do not run it in the shared working database or widen provider limits to make a sample pass. See the [frozen proposed profile](../../scripts/ops/map_first_qualification_profile.json) and [implementation/remaining qualification](MAP_FIRST_T22_T24_IMPLEMENTATION_2026_10_27.md).



For paired backup, stop the supported supervisor and every standalone API/connector/capture/imagery/delivery/retention writer, preserve quiescence, then explicitly confirm:



```powershell

task local:backup QUIESCED=true

task local:restore:test

```



The v2 inventory checks all public table contents and security definitions, including all new case/review/profile/job/upload families; required private bytes must match. It rejects changing snapshots and omits known expired/tombstoned bytes. Restore repeats retention checks before writing private objects. Old v1 backups remain supported but cannot satisfy expanded recovery qualification. No backup/restore was run against the shared installation in this increment. Retain the original upgrade backup and exact nominated rollback binary; only an actual isolated rollback/forward rehearsal can close that compatibility gate.



An appointed observer records real sessions in a copy of the [T23 template](MAP_FIRST_T23_OBSERVED_SESSIONS.template.json), pinned to the exact [candidate ledger](evidence/map-first/t22-t24/release-ledger.json) source fingerprint. Validate records with `qualify_map_first.py sessions <record.json> --release-fingerprint <hash> --output <assessment.json>`. Keep credentials and personal case text out of retained evidence. Record completeness cannot establish that a session really occurred. A reviewer must be a distinct canonical human from every case contributor. Actual target-user and assistive-technology observations remain NOT_RUN.



Regenerate traceability with `qualify_map_first.py ledger --output <new-ledger.json>`. This command assembles all task/gap/evidence states and hashes, and never awards release qualification. T22–T24 remain IN_PROGRESS; all mandatory full acceptance, P0, operational, real-geography and user-validation gates must be closed explicitly before calling the phase delivered.



Before starting qualification, use the candidate in the [pre-qualification review](MAP_FIRST_T22_T24_REVIEW_2026_10_27.md). Its fixes supersede the earlier tool/evidence fingerprint. Keep `workload.jsonl` beside its original `frozen-input.json` when running the standalone summarizer; current budgets must not replace frozen run targets. Even legacy backups now derive current expiry/tombstone policy from the isolated restored database. Fill the session template's `workflowChecks` entries from actual observations, with a full candidate fingerprint and consistent canonical participant identity.
