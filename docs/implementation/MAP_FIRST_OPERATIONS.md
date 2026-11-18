# Operations: map profile, startup, upgrade and recovery

> **Status:** Active operations guide for the map profile (`C4ISR_PRODUCT_PROFILE=map`). The map-first phase is **NOT QUALIFIED**; T22–T24 remain IN_PROGRESS. See the [records index](README.md).

## Contents

- [Map profile: operation, upgrade, workers and profiles](#map-profile)
- [Historical startup investigation](#historical-startup)
- [Historical recovery operations guide](#historical-recovery-operations)

---
<a id="map-profile"></a>

## Map profile: operation, upgrade, workers and profiles

> **Status:** Active operations guide for the map profile (`C4ISR_PRODUCT_PROFILE=map`). The map-first phase is **NOT QUALIFIED**; T22–T24 remain IN_PROGRESS and the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation) records the open failures. See the [records index](README.md).



The default environment-loaded server profile is `C4ISR_PRODUCT_PROFILE=map`. `historical-full` explicitly restores the preserved historical facilities. The ordinary map identity is `local-map-analyst`; `local-source-admin` separately administers source installation and purpose classification. Existing identities and grants are preserved.



### Local operation



Start Docker and stop any existing development supervisor with `task local:stop`. Run `task local:setup` for a new installation or after source changes that require new migrations or Go binaries. Once setup succeeds, run `task dev` and open the [development map](http://127.0.0.1:5173/map).



```powershell

task local:setup

task dev

```



Setup waits for PostgreSQL, applies forward migrations from the checkout, builds the public executables, provisions map identities and preserves existing data. The current checkout includes migration `00059_geographic_storage_lifecycle.sql`. Public setup installs the admitted reference and provider configuration under existing grants. The default supervisor uses the map analyst and starts the public imagery, preparation, delivery and configured connector workers.



`task dev` starts the local infrastructure and waits for PostgreSQL before launching the app. `task local:doctor` checks the map profile. `task local:stop` stops the registered supervisor and preserves the database and object store. Setup requires the configured application ports to be free before migrating or replacing binaries.



Historical operation is deliberate:



```powershell

task local:setup:historical

task dev:historical

```



An existing ordinary credential is not broadened into a simulation credential. New map profiles do not require simulation token files or a runtime descriptor. The server refuses exercise resource selection in map mode; historical operation needs its explicit profile and original grants.



### Area, period and saved views



The T04–T06 API/workers/UI require migration `00046_analyst_scope.sql` even if migration 00045 has already been applied. Use the coordinated setup/upgrade above before starting the updated binaries. Readiness checks require the new scope tables. Existing source restrictions still apply to every source retained in a scope, including reference geography; workers need those contributing source grants to process the submitted selection.



The map owns one area and period. Camera movement changes presentation; **Use this view** creates a deliberate viewport selection. Date-line components have separate navigation controls. Beyond ±85° the display restriction is explicit and original WGS84 geometry remains retained. Capture/export require a supported bounded local rectangle inside the exact selected polygon, including its holes.



The time control selects a rolling monitoring preset or a fixed historical interval, a supported revision interpretation and a display timezone. New scope intervals include the start and exclude the end. Existing legacy API interval semantics remain unchanged. Historical imagery outside a monitoring period requires the background-imagery option and remains labeled by acquisition date. An unsupported historical weather/track feed is not reconstructed from its current state.



Workbench drafts either follow the map or visibly pin a scope. **Use current map scope** clears affected scene/pair choices and previews while adopting area and period. Completed work retains its frozen scope. Bookmarks/share links store view presets, not investigations. Invalid, conflicting or inaccessible links show recovery; **Start a new view** deliberately returns to a fresh scope.



Related map families in a batch share a materialized read. Temporary read manifests expire after five minutes and are bounded to 16 active/64 MiB combined per principal/resource, 8 MiB each and at most 5,000 exact event revisions. Narrow a query or use aggregates when a budget is exceeded; let an expired view restart explicitly. A saved citation promotes its bounded result into durable scope evidence before expiry. Expired cursor cleanup never deletes that durable evidence. Changing access invalidates existing cursors and clears protected client caches.



Geographic administrators can admit complete release manifests and atomically activate a per-source release head using the expected previous head. Staged editions do not replace a released hierarchy; saved exact IDs keep their edition. Region profiles pin releases or named study areas. Broader regional preparation is implemented in T08; installing and qualifying additional shared regions remains an explicit administration step. See the [implementation report](MAP_FIRST_DELIVERY_LOG.md#t04-t06) and [OpenAPI contract](../../api/openapi/analyst-scope.yaml).



### Purpose classification



Authorized source administrators use **Systems → Sources & access → Content purpose review**. The inventory lists only objects permitted by current resource/source and underlying domain grants. It provides unclassified/all-purpose views, exact record identity, receipt/reason, and expected-version updates. New classification never rewrites original evidence, source dates or review status. A development/acceptance assessment cannot be relabeled as a working publication; create a separately reviewed successor.



The installer uses exact reference hashes and imagery/output IDs from retained installation receipts, plus the dated [evaluation inventory](MAP_FIRST_CONTENT_PURPOSE_INVENTORY_2026_10_25.json). It never classifies from place names, scene titles or conclusion text. Backfill is part of the subsequent successful local setup. It can also be reviewed and applied separately against the upgraded API:



```powershell

$env:EMAD_INSTALLER_PROFILE = 'local-source-admin'

.venv-recovery/Scripts/python.exe scripts/data/classify_content.py

.venv-recovery/Scripts/python.exe scripts/data/classify_content.py --apply

```



The first command produces a preview under the private installation data directory. `--apply` appends the classifications. Missing/not-authorized historical IDs remain explicitly unavailable. A receipt hash mismatch or conflicting later human classification stops the backfill; it does not overwrite that decision. Remove a temporary `EMAD_INSTALLER_PROFILE` override after the administrative session if appropriate.



Catalog GET requests accept `content_view=ordinary` by default, including unclassified records. `working`, `unclassified` and `all` require source-administration authority plus each catalog's original read permission. Exact historical reads keep their original authorization. Counts label unclassified content; reviewed working-output totals require explicit `working` classification and canonical review identity.



### Staging and recovery



The delivery composition defaults to the map server and public workers. The map worker image omits runtime/asset/recording executables. To build historical delivery, set `C4ISR_PRODUCT_PROFILE=historical-full` and enable the Compose `historical-full` profile deliberately. API and worker build arguments must use the same profile; rebuild when switching profiles. Provision/copy an appropriate scoped UI credential rather than using an exercise-only credential for a map deployment.



Map backups preserve all existing database/object history. The restore rehearsal selects the profile recorded in private configuration, validates available map references/captures and reproduces installed imagery when the receipt declares it. Historical model/recording checks run only for a historical profile with corresponding receipts. Old backups lacking a profile retain their historical interpretation. Database/object backup still requires the existing coordinated quiescence procedure; complete expanded workload/restore qualification belongs to T22.



This change has isolated migration/startup tests and a Linux image check. Apply a reviewed upgrade during the installation's normal maintenance window; keep its paired backup and original profile configuration for recovery.



### Upgrade for investigations, profiles and durable acquisition (T07–T12)



The latest working tree requires migrations **00047–00050**, in addition to the preceding 00045–00046. The user reported applying earlier migrations; this implementation session did not change the shared database or restart its processes. The [T07–T09 report](MAP_FIRST_DELIVERY_LOG.md#t07-t09) and [T10–T12 report](MAP_FIRST_DELIVERY_LOG.md#t10-t12) distinguish isolated verification from installed qualification.



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



### T13–T15 saved catalogs and imagery



Upgrade the API, capture worker and preparation worker together with forward migration **00051** using the existing supervised upgrade procedure. The user's preceding migration run is assumed through 00050, without independent shared-schema verification. Existing capture/job identifiers and retained evidence are preserved. The new intent ledger applies to newly submitted requests; it does not reconstruct old request keys from identifiers.



Catalog discovery uses `POST /api/v1/catalog/reads`. Keep the scope, filters, limit, snapshot ID and opaque cursor together. Totals belong to the saved read. Changed visibility/authorization, expiry or a mismatched cursor requires a new read. Explicit same-query refresh atomically releases the replaced ephemeral catalog; it never deletes durable content. Oversized reads need a smaller area, period or source. Compatibility imagery lists disclose their ceiling and use an unknown total when truncated.



Map aggregate drill-down uses `/api/v1/analyst-scope/reads/{id}/records`; do not reconstruct membership from a live offset query. The snapshot privately retains up to 100,000 ordered report revision IDs within the shared 8 MiB/read and 64 MiB/principal/resource budgets. Counts and reported geometry refer to that membership. Current withdrawal/supersession annotations are separate.



For imagery, inspect the selected boundary, provider footprint and suggested subset before deliberately adopting a rectangle. Use the comparison preflight before analysis. Preserve the original key/input for a lost response; **Start a new capture intent** explicitly creates another request. Case links identify exact saved revisions, retain current human membership checks and do not grant source or worker authority. An advanced case head does not rewrite a completed intent.



The local live-pixel qualification runner is opt-in: `python scripts/data/qualify_imagery.py --output <isolated-directory>` in the recovery Python environment. It makes anonymous public provider reads and retains historical STAC/windows/receipts. It does not approve production profiles or publish findings. Offline pairs can be checked with `go run ./scripts/ops/check_imagery_pair.go -before <subset.json> -after <subset.json> -out <result.json>`. Keep those outputs separate from complete application acceptance and independent review.



See [T13–T15 verification](MAP_FIRST_DELIVERY_LOG.md#t13-t15) and the [observed-session protocol](MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md). Larger serial workload results do not qualify interactive p95, five-user concurrency or a 24-hour soak. The session's host-memory interruption required restoring the existing local PostgreSQL container; its accepting-connections/healthy state was checked, without applying a shared migration.



### T16–T18 case review and composite foundations



Apply forward migration **00052_case_workflows_and_outputs.sql** with the updated API/UI through the normal supervised upgrade. The user reported preceding migrations, assumed through 00051. This increment does not migrate or deploy the shared installation. Local provisioning now includes a distinct `local-map-reviewer` human profile; source-administrator and analyst aliases of the same human remain ineligible to review each other's work.



Preserve case save request keys and the original input when retrying a lost response. A recovered save returns its original immutable revision; do not replace its expected revision or silently resubmit it against a newer head. Resolve an actual conflict by reviewing the current case and deliberately preparing a successor. Device recovery remains scoped to the original canonical human/resource and confers no source rights.



Before review, inspect source/purpose/retention/scope notices and add an already authorized independent reviewer as a member. Review assignment grants no source access. Changes requests remain available when support has changed; approval requires current support. Revisions authored by a reviewer make that person a contributor who cannot approve the successor. Output approval requires READY bytes and binds the exact case/manifest/render. Real independent-human sessions still require observation under the T14 protocol.



Review source terms for each intended output purpose and approve a new rights/admission/reference version through source administration. `rights.outputPermissions` accepts explicit booleans for `internalAnalysis`, `citation`, `includedImagery`, `rawDistribution`, `geographicEnvironment`, and `metadataRetention`; absent/null means unknown and blocks output. Preserve the immutable original rights identity and use a new rights identity when terms change. New permissive current terms cannot retroactively widen unknown/denied original terms. Do not infer these permissions from `allowSimulationExport`. The rights API is available now; readable brief rendering remains T19.



Composite bytes use the configured private artifact store, under a separate composite namespace. Configure private storage before staging. Freeze a manifest, retain the exact bounded JSON/PNG rendering, obtain independent exact-render approval, and only then request approved distribution. Default foundation ceilings are 8 MiB per render and 4096 per PNG side and 4 megapixels in total; frozen reservations share the resource storage budget. Existing draft bytes remain pinned to their original case revision when the case advances; their superseded status precludes current distribution.



Reconcile expired outputs with `POST /api/v1/outputs/{id}/expire` or the output panel. Failed deletion leaves DELETE_PENDING and counts against capacity until a successful retry. Successful reconciliation retains only a minimal tombstone plus hashes/audit identities. Do not extend the frozen retention or restore expired bytes; a new output needs currently eligible evidence and a new independent review. Audit pending reservations/orphaned private objects after interrupted staging before production rollout; no unattended reaper is claimed by this foundation. See [T16–T18 implementation](MAP_FIRST_DELIVERY_LOG.md#t16-t18).



### T19–T21 durable delivery and geographic preparation



Coordinate migration **00053_delivery_jobs.sql** with the updated API/UI and `emad-delivery` binary. The user reported migrations, without independent shared-schema inspection. This increment does not deploy or migrate the shared application. Preserve pending/finished job inputs, leases, attempt events and upload records during upgrade; forward recovery preserves immutable history.



The case delivery panel queues a readable brief or geographic package. Draft reading is separate from approved distribution. Inspect the retained output, select **Use exact rendered output for review**, assign an already authorized independent reviewer, and obtain approval for that exact manifest/render. The approved brief bundle includes frozen HTML, machine metadata, and readable/machine approval attestations. Opening it makes no live evidence/tile request. A case edit or changed support blocks current distribution; older permitted drafts remain pinned and labeled historical.



Run `emad-delivery -token-file <private-original-author-credential>` outside HTTP, or use the updated supervised local startup. It processes only that canonical creator's visible jobs and retains current source/case checks. Automation confers no review permission. Aliases are the same contributor. Local startup uses `local-map-analyst.token`; other creators need separately scoped worker instances. The optional staging Compose `case-delivery` profile uses `delivery-author.token`; supply that credential deliberately rather than giving a service every user's case access.



Workers use 150-second leases, at most three attempts, bounded retry delay/jitter and expired-lease recovery. Canceled jobs cannot publish. Retryable storage/deadline failures wait before another attempt; validation or support failures require correction/reassessment. A first upload binds its candidate hash/private key. Changing bytes requires a new output. Upload plans/completions support orphan reconciliation; uncertain writes keep deletion pending until their bounded deadline. Idle workers reconcile expired author-owned outputs with a per-row delay; explicit expiry remains available to authorized case users. Never delete referenced/unexpired inputs or restore expired bytes as ordinary recovery. Prolonged unattended and restore qualification remains T22.



Normalize retained geographic data with `.venv-recovery/Scripts/python.exe scripts/data/prepare_portable_geography.py --spec <spec.json> --output <new-private-directory>`. The spec names the exact WGS84 selection, height unit `metres`, grid size, local DEM paths/hashes/source identities/retrieval references, reviewed reference metadata/rights/accuracy, and optional vector parents. The tool performs no download or admission, preserves native parent files/receipts, and writes `prepared-references.json` for normal source administration. Mixed-source DEM mosaics are rejected; do not hide several providers under one source identity. Use an admitted complete bounded mosaic for the current preparation profile.



Approve exact source-backed accuracy and purpose-specific output rights before geographic processing. Missing accuracy, unknown permissions, incomplete DEM coverage or missing EGM2008 undulation grid cannot be repaired by increasing a tolerance until the job passes. Retain source/control provenance and select tolerances for the intended geographic use. Original unknown/denied rights cannot be widened by permissive current terms alone. Old references/artifacts can need a newly reviewed source version; changing a case citation grants nothing.



The product supports one bounded component (within ±85°, at most 2° per side), a WGS84 DEM mosaic, optional admitted EGM2008 undulation grid, vectors and qualified retained multispectral subsets. Imagery must be an exactly cited `imagery_subset` with declared horizontal accuracy and georeferencing. Source acquisition date/quality classes and outside/nodata masks remain explicit. Set the case's background-imagery option deliberately if an older acquisition is intended. Arbitrary PNG/TIFF files are not inferred to have a valid geographic frame. Unsupported CRS/datum/grid profiles fail before a full job; optional GLB is omitted.



Inspect the neutral package's instructions/manifest and verify independently with `.venv-recovery/Scripts/python.exe scripts/data/check_portable_environment.py <package.zip>`. It checks PROJ coordinates, controls, GEOS polygon containment, masks, hashes and triangle indices without an application/runtime session. This is not a surveyed-accuracy certificate or live rights check. Native GeoTIFF/OSM parents may remain external dependencies, identified by the retained receipts; preserve their permitted bytes separately. Geographic processing requires no actor setup, model-generation worker, runtime or replay. The historical Jeddah CLI is explicitly `prepare_environment.py --jeddah-fixture`; model fixtures remain in the historical simulation installer.



See [T19–T21 implementation and boundaries](MAP_FIRST_DELIVERY_LOG.md#t19-t21). Full acceptance, real qualified terrain/control data, observed independent recipients/users, mixed-load timing and long-running/recovery qualification remain open.





### T22–T24 qualification and release



The preceding sections describe their dated increments. Current coordinated installation requires migrations through **00053** and the matching API/UI/public workers. The user reports applying migrations; shared schema, active binary versions and installed regional/source profiles were not independently inspected. T22–T24 introduce no additional migration.



Run a repeatable isolated sample with the recovery Python environment:



```powershell

.venv-recovery/Scripts/python.exe scripts/ops/qualify_map_first.py sample --output private/qualification/sample-001 --seconds 30

```



Use a new output directory for each run. Docker/Testcontainers start only disposable services. The command freezes the profile/source/hardware and preserves failures, errors/timeouts, per-operation cold/warm percentiles and heap/database/queue/retrieval-age samples. `--seconds 86400` measures a real 24-hour **reduced** sample; it still does not close the full dataset, browser, process restart, RSS/object-growth or independent-user gates. Do not run it in the shared working database or widen provider limits to make a sample pass. See the [frozen proposed profile](../../scripts/ops/map_first_qualification_profile.json) and [implementation/remaining qualification](MAP_FIRST_DELIVERY_LOG.md#t22-t24).



For paired backup, stop the supported supervisor and every standalone API/connector/capture/imagery/delivery/retention writer, preserve quiescence, then explicitly confirm:



```powershell

task local:backup QUIESCED=true

task local:restore:test

```



The v2 inventory checks all public table contents and security definitions, including all new case/review/profile/job/upload families; required private bytes must match. It rejects changing snapshots and omits known expired/tombstoned bytes. Restore repeats retention checks before writing private objects. Old v1 backups remain supported but cannot satisfy expanded recovery qualification. No backup/restore was run against the shared installation in this increment. Retain the original upgrade backup and exact nominated rollback binary; only an actual isolated rollback/forward rehearsal can close that compatibility gate.



An appointed observer records real sessions in a copy of the [T23 template](MAP_FIRST_T23_OBSERVED_SESSIONS.template.json), pinned to the exact [candidate ledger](evidence/map-first/t22-t24-review/release-ledger.json) source fingerprint, following the [T23 observed-session protocol](MAP_FIRST_T23_OBSERVED_SESSION_PROTOCOL.md). Validate records with `qualify_map_first.py sessions <record.json> --release-fingerprint <hash> --output <assessment.json>`. Keep credentials and personal case text out of retained evidence. Record completeness cannot establish that a session really occurred. A reviewer must be a distinct canonical human from every case contributor. Actual target-user and assistive-technology observations remain NOT_RUN.

The [T23 observed-session protocol](MAP_FIRST_T23_OBSERVED_SESSION_PROTOCOL.md) is published and extends the [T14 protocol](MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md) with the frozen-brief, geographic-package, case-handoff and offline-recipient journeys, the `workflowChecks` matrix that `qualify_map_first.py sessions` enforces and the per-task attempt/assistance accounting that A19's exit measurement needs. A recorded FAIL or BLOCKED journey is evidence: the assessment reports it and measures the unassisted rate against A19's 80% threshold; it never awards qualification and A19/T23 stay NOT_RUN. The retained ledger fingerprint is pinned to an older source revision; regenerate the ledger against the candidate being observed and pin the session record to that fingerprint.



Regenerate traceability with `qualify_map_first.py ledger --output <new-ledger.json>`. This command assembles all task/gap/evidence states and hashes, and never awards release qualification. T22–T24 remain IN_PROGRESS; all mandatory full acceptance, P0, operational, real-geography and user-validation gates must be closed explicitly before calling the phase delivered.

Exercise the declared recovery faults with `qualify_recovery.py --output <result.json>` (or `task map:qualify:recovery OUTPUT=<path>`). It runs each of the profile's twelve `requiredRecovery` rehearsals in isolation and records `PASS`, `FAIL` or `NOT_RUN` per item. A rehearsal that could not run is `NOT_RUN` with the missing instrument named — never a pass. `--list` prints what each rehearsal does and which check backs it. Feed the result to the workload run with `qualify_map_first.py sample --recovery <result.json>` so the recovery gates carry the measured outcome instead of `NOT_RUN`.

Rehearse rollback compatibility with `qualify_rollback.py --output <result.json> --binary <installed-binary> --manifest <manifest.json>`. It refuses to substitute the retained source archive under `private/qualification/` for an installed binary, and records `NOT_RUN` with the reason when nothing has been nominated. It verifies that cases, geographic releases, profile mappings, canonical review identity, briefs and environment dependencies survive with matching object hashes.

A measured budget outside its target fails the run. An unmeasured budget is reported as `NOT_MEASURED` with the reason and is never assumed to pass; the same applies to the soak, concurrency and recovery gates.



Before starting qualification, use the candidate in the [pre-qualification review](MAP_FIRST_DELIVERY_LOG.md#t22-t24-review). Its fixes supersede the earlier tool/evidence fingerprint. Keep `workload.jsonl` beside its original `frozen-input.json` when running the standalone summarizer; current budgets must not replace frozen run targets. Even legacy backups now derive current expiry/tombstone policy from the isolated restored database. Fill the session template's `workflowChecks` entries from actual observations, with a full candidate fingerprint and consistent canonical participant identity.

<a id="historical-startup"></a>

## Historical startup investigation

> **Historical record — startup investigation.** This is the historical manual provisioning investigation for the pre-map-first installation. The supported current startup is `task local:setup` then `task dev`, documented in [MAP_FIRST_OPERATIONS.md](#map-profile). Its account set and migration level below are superseded. See the [records index](README.md).

Run `task local:setup`, then `task dev` and open **http://127.0.0.1:5173/**. Use `task local:doctor` and `task local:stop`. This installs and starts persistent content and scoped workers. The [current operations guide](#historical-recovery-operations) supersedes the manual provisioning instructions below. [Recovery acceptance](HISTORICAL_RECOVERY.md#recovery-acceptance) records the installed workflows and limits.

### Historical startup investigation

2026-10-23. The normal entry is now **Home** at `http://localhost:5173/`. The module bar links directly to the features available in the selected workspace. The existing map is still at `/map`.

### Why the old launch looked unchanged

The development token authenticates `human:operator-01`, which initially has only legacy exercise access. Database migrations do not grant Public Context or protected scene-authoring access, admit sources, import assets, or start workers. Previously `/` redirected straight to the old map, several new routes had no main-navigation link, and the empty legacy run gave no useful introduction to the new product areas.

Home now explains missing Public Context and Studio access. It checks Studio authorization against the real API rather than inferring it from exercise permissions. These UI changes do not grant access or fabricate observations/scenes.

### Resolve a conflicting API port

If another service uses port 8080, stop your EMAD development commands with Ctrl+C, then use a free port in both terminals:

```powershell
# Terminal A
task dev:api C4ISR_DEV_PORT=8081

# Terminal B
task dev:web C4ISR_DEV_PORT=8081
```

The UI remains on port 5173. `C4ISR_DEV_PORT` coordinates the API listener and the HTTP/WebSocket proxy. An explicit `C4ISR_HTTP_ADDR` overrides the listener; if you set it, also set the matching `C4ISR_API_PROXY_TARGET` on `dev:web`. Direct `pnpm dev` launches can set `VITE_API_PROXY_TARGET` to the API's HTTP origin.

Do not stop an unrelated server to free the port. During the reported local launch, Apache answered on IPv4 port 8080 while EMAD answered on IPv6 port 8080. That can produce different behavior depending on which loopback address resolves first.

React StrictMode also cancelled the first in-flight WebSocket upgrade during development effect setup/cleanup. Startup now defers opening the connection until that cleanup has run. Authorization checks, reconnects and actual proxy error reporting remain enabled.

### Explicit local analyst and author access

These commands are an **optional, trusted administrator action against your local development database**. They create a separate human credential with Public Context access, authoring access and exercise access in `default / local`. They do not alter the existing operator's grants, grant simulator-truth access, or enable external retrieval. Change the names/environment for your installation. Run provisioning once; if the identity already exists, use the administrator's `issue`/`rotate` actions rather than recreating it.

```powershell
$env:C4ISR_DATABASE_URL = 'postgres://c4isr:c4isr@localhost:5433/c4isr?sslmode=disable'
New-Item -ItemType Directory -Force private | Out-Null

# Create the named human principal and write its secret only to an ignored file.
go run ./apps/emad-admin -action provision -spec scripts/data/local-analyst.example.json -out private/local-analyst.token

# Assign authoring in the same environment.
$studioGrant = Get-Content scripts/data/simulation-studio-author-grant.example.json -Raw | ConvertFrom-Json
$studioGrant.id = 'local-analyst'
[IO.File]::WriteAllText((Join-Path (Get-Location) 'private/local-studio-grant.json'), ($studioGrant | ConvertTo-Json -Depth 10))
go run ./apps/emad-admin -action grant -spec private/local-studio-grant.json

# Explicitly allow exercise operations across this local environment's runs.
$exerciseGrant = Get-Content scripts/data/native-exercise-operator-grant.example.json -Raw | ConvertFrom-Json
$exerciseGrant.id = 'local-analyst'
$exerciseGrant.resource.runId = '*'
$exerciseGrant.permissions += 'scenarios.control'
[IO.File]::WriteAllText((Join-Path (Get-Location) 'private/local-exercise-grant.json'), ($exerciseGrant | ConvertTo-Json -Depth 10))
go run ./apps/emad-admin -action grant -spec private/local-exercise-grant.json

# Restart the UI using the new credential. Do not paste or print the token.
$env:C4ISR_API_TOKEN = (Get-Content private/local-analyst.token -Raw).Trim()
task dev:web C4ISR_DEV_PORT=8081
```

Keep the API running in terminal A. Restart only terminal B after changing its token. The issued credential defaults to a 24-hour lifetime; expiry requires a new credential. Example credentials/permissions are for explicit local development setup, not a production deployment policy.

### Where the delivered capabilities appear

- **Public Context → Map**: event/reference layers, filters, historical views and evidence drill-down. Configure reviewed source admissions and trusted provider bindings first; see [public ingestion](HISTORICAL_DELIVERY.md#p1-1) and [USGS/FIRMS acceptance and activation](HISTORICAL_DELIVERY.md#s0-4-p1-5-6).
- **Imagery & assessments**: imagery comparison, analysis jobs and analyst review. Private artifact storage, admitted imagery and the analysis worker are separate setup requirements; see [imagery and analysis/review](HISTORICAL_DELIVERY.md#p2-1-3).
- **Weather & reports / Moving tracks**: bounded connector records and provenance. Provider credentials/agreements remain required where documented; see [moving feeds](HISTORICAL_DELIVERY.md#p2-4-p3-1-2) and [weather and hazard reports](HISTORICAL_DELIVERY.md#p3-3).
- **Simulation Studio / 3D assets**: model admission, environment/template registration, scene authoring and a georeferenced 3D viewport. A grant opens the module; it does not create a scene. Follow [asset admission](HISTORICAL_DELIVERY.md#s1-1) and [Studio/runtime rollout](HISTORICAL_DELIVERY.md#s1-3-s2-1).
- **Exercise COP / Recording & replay**: native sensor output, exercise operations, recorded output and released after-action reports. These need an authored run and the independent runtime/recording workers; see [perception and tasking](HISTORICAL_DELIVERY.md#s2-2-s2-3-s3-1) and [recording/evaluation](HISTORICAL_DELIVERY.md#s3-2-s3-3-s4-1).

The shipped simulation is an incremental, bounded profile with abstract movement, visual context and synthetic sensor behavior. It is not yet the long-term high-fidelity aircraft/terrain/physics environment. Passing those profiles' tests demonstrated those profiles; it did not demonstrate a fully populated operational installation or the completion of future S4 work.

### Verification of this follow-up

- Production UI build passed. All 80 browser regression checks passed, with 54 intentional project skips (including the four new cases' duplicate token-project runs).
- All 16 real-backend production browser checks passed against disposable PostGIS/S3. The first run exposed a test race reading an old document's response after navigation; the test now separates document lifetimes and reads the new response body immediately.
- The local API/Vite path was exercised without HTTP/WebSocket mocks. Before the fix, development startup opened and cancelled one COP upgrade before establishing the next. After the fix, one COP stream reached `realtime.ready`, remained healthy through the 30-second heartbeat, and reported no browser socket errors. Killing an earlier inspection process produced a separate expected connection-reset log; real proxy errors remain visible.
- Desktop and 390px mobile views were inspected; the new page had no document-level horizontal overflow. Local screenshots and raw logs remain ignored under `tmp/runtime-review/`.
- Port-override Task definitions pass dry-run parsing. Documentation links resolve. All migrations, the migration script and the line-ending policy retain their previously protected hashes.

No local access grants or source configuration were changed by this review. The optional provisioning instructions above still require an explicit administrator action. No commit was created.

<a id="historical-recovery-operations"></a>

## Historical recovery operations guide

> **Historical record — bounded delivery recovery, 23–24 October 2026.** Retained as traceability history. It describes the **pre-map-first** installation profile: its migration level, worker set and account list are superseded by the map profile. The current map profile is documented in [MAP_FIRST_OPERATIONS.md](#map-profile). See the [records index](README.md).

Use the persistent local profile from the repository root in PowerShell:

```powershell
task local:setup
task dev
```

Open **http://127.0.0.1:5173/**. Setup uses the existing PostgreSQL database on port 5433, applies forward migrations through **00053**, provisions exact development profiles, installs retained real content and configures private Garage storage. It preserves the database, volumes, credentials and existing grants. Repeating setup verifies and reconciles its resources. It stops on conflicting identifiers, changed grants or an occupied application port. Never use `db:reset` for this installation.

Go, Task, pnpm, Python 3.12 and a running Docker Desktop are prerequisites. Setup creates `.venv-recovery` with pinned raster dependencies. The first content installation downloads roughly half a gigabyte of geographic sources and bounded satellite windows; retained sources and receipts allow subsequent runs to resume. An external download failure leaves the checkpoint available for retry and does not replace real inputs with fixtures.

`task dev` starts the API, Vite and the workers for the **active profile**. Under the default `map` profile that is the API, Vite, imagery preparation, imagery analysis and the configured public connectors, plus the delivery worker — the admitted native runtime, raster asset validation and recording workers start only under the explicit `historical-full` profile. Application traffic uses explicit IPv4 addresses: API `127.0.0.1:8081`, browser `127.0.0.1:5173`. Change the two ports in ignored `private/local/config.json` only while stopped. It contains secrets: do not publish its contents. The older separate `dev:api` / `dev:web` commands remain available for low-level development; they do not assemble this installation.

```powershell
task local:doctor
task local:stop
```

Doctor checks scoped credentials, storage, installed features and worker readiness. Stop shuts down the supervisor's own application processes while leaving PostgreSQL, Garage and content intact. Logs are in `private/local/logs`. Worker failures restart with bounded backoff; a lost native execution lease closes that exercise as failed and archives its received output. It does not silently recreate its private world.

### Accounts and the two workflows

The default local session depends on the active profile. Under the default `map` profile it is `local-map-analyst`, a Public Context analyst; `local-source-admin` separately administers source installation and content purpose. The `historical-full` profile instead opens `local-analyst`, a development analyst and scene author, with Home showing the two pillars, Public Context opening the Saudi map and Simulation Studio opening the frozen Jeddah environment with an automatic Start.

Other account credentials are in ignored `private/local/credentials/<account>.token`. Open **Account**, paste the credential into **Account credential**, then **Open account**. Switching reloads the application, closing sockets and removing the previous account's in-memory caches. Never paste credentials into a URL or a checked-in file.

Under the `map` profile the installed identities are:

- `local-map-analyst`: the ordinary map analyst (Public Context read, investigation and delivery grants).
- `local-map-reviewer`: the map review account. It requires an independent human; a second credential for the same person is not independent review.
- `local-source-admin`: source installation and content-purpose administration.

Under the `historical-full` profile the following additional development identities are installed:

- `local-reviewer`: independently registered development reviewer for review/export testing.
- `local-operator`: exercise observations, controlled virtual tasking and ordinary replay; no authoring or simulator truth.
- `local-evaluator`: explicit authoring and truth/evaluation access for this development environment's runs.
- `local-author-alias`: same registered human as the analyst; used to prove review/publication/rejection denial.
- `worker-*`: separate source or partition-bound service identities; never human reviewers.

These profiles are clearly labeled development identities. A workflow executed by two accounts does not establish review by two actual people. The real retained imagery assessment remains proposed for human review. The separately published **DEVELOPMENT WORKFLOW TEST** record must not be treated as an intelligence conclusion.

Credentials expire after 30 days. Setup never overwrites an existing credential. Doctor reports expiry. Use the trusted `emad-admin` issue/rotate workflow to create a new credential in a new private file, then deliberately replace the affected local profile file; restart its process. Production identity provisioning and grants require the deployment's own policy.

### Restricted source activation

**Current local provider status:** The user-supplied [OpenSky OAuth client](HISTORICAL_RECOVERY.md#opensky-activation) and [FIRMS map key](HISTORICAL_RECOVERY.md#firms-activation) are installed and their bounded live checks passed. FIRMS is completing its initial lookback through paced day pages. AIS remains separately gated. The instructions below remain available for a fresh installation.

Public map source cards distinguish retained geography, configured retrieval, last successful retrieval, stale coverage and missing prerequisites. A successful retrieval with zero Saudi records differs from an unavailable provider. Historical USGS and March 2025 imagery remain labeled by their actual dates.

FIRMS, OpenSky and AISStream are gated until the applicable key or use permission is supplied. Create ignored `private/local/providers.json` with only the bindings you have approved. Paths below are examples of private files, not supplied keys:

```json
{
  "firms": {"keyFile": "private/local/firms.key"},
  "opensky": {
    "permissionConfirmed": true,
    "authenticatedRequired": true,
    "oauthCredentialsFile": "private/local/opensky-client.json"
  },
  "ais": {"permissionConfirmed": true, "keyFile": "private/local/aisstream.key"}
}
```

The OpenSky file accepts the provider's `clientId` / `clientSecret` (or `client_id` / `client_secret`) JSON. The native adapter obtains and refreshes short-lived bearer tokens using OpenSky's fixed TLS token endpoint; tokens stay in memory. Anonymous bounded retrieval is possible only when permission is confirmed and authenticated access is not required. Use and retention permission must be resolved against the provider's terms. See the [official OpenSky REST documentation](https://openskynetwork.github.io/opensky-api/rest.html), [FIRMS API](https://firms.modaps.eosdis.nasa.gov/api/) and [AISStream documentation](https://aisstream.io/documentation).

After editing private bindings, stop, run setup and start again. Missing/invalid optional files remain gated; they do not block unrelated workers. Setup reconciles its own admitted source status. Requests retain bounded Saudi coverage, quotas, pacing and freshness policy. A provider rejecting a syntactically valid credential appears as a provider-access failure, not an empty result. No restricted feed is represented by synthetic public observations.

### Paired backup and isolated restore

```powershell
task local:stop
task local:backup QUIESCED=true
task local:restore:test
task dev
```

Stop every writer, including standalone delivery/connector/capture/imagery/retention workers, before explicitly confirming `QUIESCED=true`. Backup creates a consistent database dump, permitted private objects, source/installation receipts and private configuration. The current v2 format verifies unchanged public table/security-definition inventories and required object hashes before publishing its completion manifest. Known expired/tombstoned object keys are excluded and retention is checked again during restore. See [current map qualification/recovery limits](MAP_FIRST_DELIVERY_LOG.md#t22-t24). Backups remain under ignored `private/local/backups`; they contain credentials and must remain private. Copy the complete directory to protected storage for off-machine recovery. `private/local/last-backup.json` identifies the latest accepted backup.

Restore testing uses new randomly named PostgreSQL and Garage containers/volumes and ephemeral loopback ports. It validates every byte, opens restored data through a separate API, reproduces NDVI from the restored inputs and reads retained models, geography and recordings. It removes only those temporary containers/volumes, leaving the active installation unchanged. It does not replace a production disaster-recovery procedure or restore over the original database.

### Staging packaging

`deployments/Dockerfile.workers` builds the API and independent workers with a pinned raster toolchain and model validator. Apply `deployments/staging/delivery.yml` after the existing staging composition. It adds persistent private Garage storage and imagery, asset, raster, native-runtime and recording workers. Provide scoped credentials via its named secret files. One connector instance takes one exact-source service credential; add instances for each deliberately admitted source.

The runtime build admitted by the API comes from the same image. The runtime shares the API network namespace for loopback control. Private S3 HTTP is explicitly enabled only inside the staging network; production rejects that exception and requires HTTPS. The non-root/read-only worker image and composed configuration have been exercised locally. No external staging environment or production credentials were deployed by this recovery.
