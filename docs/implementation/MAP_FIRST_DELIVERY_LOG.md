# Map-first delivery log: implementation and qualification records

> **Status:** Active delivery record. Each section below is the implementation record for one task group. The phase remains **NOT QUALIFIED**; T22–T24 are still IN_PROGRESS. See the [records index](README.md).

## Contents

- [T01–T03: area containment, map-only startup, content purpose](#t01-t03)
- [T04–T06: versioned areas, temporal scope, shared handoff](#t04-t06)
- [T07–T09: investigations, regional reference, retrieval profiles](#t07-t09)
- [T10–T12: acquisition, coverage and task readiness](#t10-t12)
- [T13–T15 design contract](#t13-t15-design)
- [T13–T15: paging, role workflows, imagery intents](#t13-t15)
- [T16–T18: evidence, findings, independent review](#t16-t18)
- [T19–T21: readable briefs and portable geography](#t19-t21)
- [T22–T24: operational qualification and release evidence](#t22-t24)
- [T22–T24 pre-qualification review](#t22-t24-review)
- [Local access failure correction (29 October 2026)](#access-fix)

---
<a id="t01-t03"></a>

## T01–T03: area containment, map-only startup, content purpose

**Prepared:** 25 October 2026. **Handoff updated:** 26 October 2026 (Asia/Riyadh).\
**Authorization:** Implement the first three tasks of the [map-first plan](MAP_FIRST_PLAN.md#implementation-plan).\
**Source:** Working tree based on `a6d1d22f34692faea31db58e4a6318c539c3321a`; no commit or deployment is implied.\
**State:** T01–T03 are implemented with the scoped checks below. The [backlog](MAP_FIRST_BACKLOG_2026_10_25.json) retains separate implementation, installation, functional, geographic and user evidence. The full phase remains unqualified.

### T01: Area integrity

[Operation-area validation](../../apps/operator-ui/src/features/workspace/operationArea.ts) now receives exact selected/pinned geometry from the map/workbench. [Export](../../apps/operator-ui/src/features/imagery/ContextExport.tsx) no longer reads a separate localStorage investigation or supplies Jeddah. [Capture](../../apps/operator-ui/src/features/imagery/CaptureWorkbench.tsx) starts with a valid selected local rectangle or empty required coordinates. Systems no longer supplies a fixture when its selected scope is missing.

- A rectangular operation accepts an explicit rectangle faithfully representable from the current selection. Irregular/multipart/invalid/unresolved selections demand a local rectangle; their bbox is not substituted as analytical geometry.
- Export enforces its existing 400-square-degree bound and capture its existing 0.1-degree-per-side bound. A manual subset must stay inside its selected rectangle; existing server scene/admission/rights checks remain in force.
- Switching to the current map area clears stale capture scene/pair choices, incompatible comparison choices and export previews/selections. Pending preview responses cannot populate a different area. Minimized workbenches retain visibly pinned drafts until explicitly changed; frozen jobs/packages retain their original inputs.
- A named area's asynchronous resolution completes that same pinned scope. Invalid shared geometry cannot borrow a saved rectangle. Unsupported operations expose a map-area selection action.
- Capture/history and prepared comparison choices use the selected footprint; intentionally inspected exact completed records keep their original lineage.

The full versioned geometry/time/case contracts remain T04–T07. This containment increment deliberately requires a supported local rectangle for the existing rectangular operations.

### T02: Map-only operation

Environment-loaded configuration defaults to `C4ISR_PRODUCT_PROFILE=map`. The local lifecycle, staging composition and image build support `map` and explicit `historical-full` profiles. Map setup provisions `local-map-analyst` with Public Context permissions and a separate `local-source-admin`; it does not require development exercise/reviewer identities or simulation worker credentials. Existing principals/grants/history are preserved. Historical provisioning remains explicit.

The map supervisor starts the API, UI, imagery/preparation and configured public connectors. It does not describe or launch a runtime or start asset/recording workers. The API does not initialize scenario engines or the exercise freshness monitor in map mode, uses wall time, rejects exercise resource selection, and exposes Public Context navigation resources. Environment-loaded servers require the forward schema, including the new purpose table.

Home, header and workbench/environment choosers use the deployment profile. Map entry ignores saved exercise defaults and does not query simulation onboarding. Explicit historical routes remain available under the historical profile and their original grants. A basemap outage leaves selection geometry and evidence/export controls usable with a visible map-source notice.

The packaging checker and paired-restore script are profile-aware. Map restore does not require simulation receipts; old backups without a recorded profile retain historical interpretation. An installed imagery receipt still triggers reproduction checks. Full expanded recovery/soak qualification remains T22.

### T03: Purpose and truthful counts

[Migration 00045](../../db/migrations/00045_content_purpose.sql) adds append-only purpose revisions with scope/source RLS, actor, reason, exact entity identity, receipt reference and expected-version conflict handling. Original source/review bytes are not rewritten. [Content-purpose administration](../../internal/contentpurpose/service.go) requires source management, the underlying catalog's read permission, and current source access; classification requires a human principal.

Ordinary reference/scene/capture/job/assessment/package catalogs and relevant map layers exclude classified development/acceptance content. Exact authorized history stays available, with support catalog views behind administration permissions. Unclassified records remain explicit; they cannot inflate reviewed working-publication totals. Metadata is bound transaction-locally, preserving pooled-connection isolation.

Counts distinguish retained reference versions, catalog revisions, successful capture records, distinct acquisition timestamps, distinct completed comparisons by frozen inputs/method, and published working assessments with canonical human separation. Systems labels bounded client subsets as **shown** counts. Provider quality, real/synthetic provenance, content purpose and human review remain separate concepts. A development/acceptance assessment cannot be relabeled into a working publication; a successor needs its own permitted review.

[Receipt backfill](../../scripts/data/classify_content.py) verifies original reference hashes and capture/preparation identity links, uses exact receipt/evaluation IDs, rejects conflicting later classifications, and reports unavailable IDs. Its default is a reviewable preview; successful subsequent setup applies the explicit inventory through the upgraded API. The [seed inventory](MAP_FIRST_CONTENT_PURPOSE_INVENTORY_2026_10_25.json) identifies two dated development assessments. Remaining unknown records require source-administrator review; no names or conclusion phrases are used to guess their purpose.

The active [product brief](MAP_FIRST_PRODUCT.md#product-brief), README, PRODUCT and [operations](MAP_FIRST_OPERATIONS.md#map-profile) now point to the current map-first scope and backlog.

### Verification performed

- `go test ./...` passed; Docker integration cases are separately enabled and listed below.
- `go vet ./...` and repository Go formatting checks passed.
- UI typecheck, regenerated OpenAPI client and production build passed. The existing Vite large-chunk advisory remains; bundle optimization is outside this correctness batch.
- **Seven new browser cases** passed: map-only entry; country extent and irregular-country rejection; clearing a mounted capture draft; exact Paris-area export coordinates; classification administration; invalid shared geometry; and external basemap outage.
- **Eight existing entry/navigation browser cases** passed, including historical author-only entry and retained workspace/chooser behavior.
- **Two new assembled PostGIS integration cases** passed. They cover fresh and upgraded provisioning/default selection, real Linux-image API startup without a runtime descriptor, preserved historical grants, append-only purpose metadata, authorization/RLS, exact bytes, stale writes, pagination totals, capture/date/study separation and development-versus-working publication counts. Counter fixtures do not assert actual review by independent people.
- **Three existing assembled PostGIS/S3 cases** passed: geographic history/aggregates/isolation, retained imagery/independent-identity enforcement/context export, and real reference/terrain bytes and isolation.
- **Four Python backfill cases** passed: preview-only behavior, exact verified-ID application without changing originals, hash mismatch rejection, and refusal to overwrite conflicting human classification.
- Staging configuration passed in both profiles: default map excludes runtime/asset/recording services; explicit historical-full includes them with the matching API/build profile.
- The Linux map delivery image built and its dependencies/inventory were checked. Runtime, asset and recording executables are absent. The same image served a real authenticated map session against an isolated migrated PostGIS database.

The [machine-readable verification record](evidence/map-first/t01-t03/verification.json) records source hashes, artifact identity, checks and limits. Browser API fixtures are used for the geometry/presentation tests; assembled service tests use real PostGIS and, where applicable, private S3 fixtures. No live provider acquisition, new regional source admission or independent-human session was performed.

### Browser evidence

These screenshots use isolated mocked catalog/geometry responses, not a qualification of installed Saudi or Paris coverage:

- [Map-first Home](evidence/map-first/t01-t03/map-entry.png).
- [Capture draft after explicitly selecting the country scope](evidence/map-first/t01-t03/capture-scope-cleared.png). The synthetic country fixture is a rectangle solely to exercise the handoff/size gate; it does not represent an authoritative national boundary.

### Installation and remaining qualification

The shared running installation was not stopped, restarted, migrated or backfilled during this code pass. Apply migration/profile/identity changes with the [normal setup and upgrade procedure](MAP_FIRST_OPERATIONS.md#map-profile), using the installation's coordinated backup/maintenance process. Isolated startup/migration evidence does not establish that deployment occurred in the shared installation.

T01–T03 are **IMPLEMENTED**, rather than fully closed outcome gates. Full scenario A01 also includes future case save/resume; A02 eventually spans case/review/brief contracts; A14's installed inventory and final release evidence remain separately tracked. No gap is declared closed by this batch. T04 and later tasks remain planned. Regional coverage, full recovery/soak and independent usefulness retain their later qualification gates.

<a id="t04-t06"></a>

## T04–T06: versioned areas, temporal scope, shared handoff

**Prepared:** 26 October 2026 (Asia/Riyadh).\
**Authorization:** Implement the next three tasks after T01–T03 in the [map-first plan](MAP_FIRST_PLAN.md#implementation-plan). The user reports applying the preceding migrations.\
**Source:** Uncommitted working tree based on `a6d1d22f34692faea31db58e4a6318c539c3321a`.\
**State:** T04–T06 are implemented with the checks below. At the time of this increment T07–T24 remained planned; T07–T21 were subsequently implemented and **T22–T24 remain IN_PROGRESS**, so the phase and full acceptance scenarios remain unqualified. See the [records index](README.md) and the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation).

### T04: Versioned areas and exact geography

The new [area domain](../../internal/areas/service.go) owns transient selections and immutable persisted area revisions. Revisions retain the original request geometry, canonical geometry bytes and hash, source-qualified edition/release identity, parent revision, per-component bounds/geodesic area and display capability. Resolution is read-only. Capture/comparison submission and approved export persist the selected area and evaluated period in the same transaction as their durable work.

- Exact geographic IDs remain readable after newer editions are staged or activated. Discovery uses a per-source atomic release head; a release admits a complete, validated edition hierarchy. Duplicate logical keys, missing parents, hierarchy cycles and conflicting selectors fail explicitly. Legacy catalogs without releases remain readable.
- Immutable region profiles pin released reference features or explicitly named study areas. Their hashes preserve exact decimal values. Numeric precision above JavaScript's safe-integer range is retained in server fingerprints; old unscoped context-package identity rules remain compatible.
- World is a discovery scope. A place point is a navigation target. Neither becomes an invented local operation rectangle. Panning changes the camera; **Use this view** deliberately creates a viewport area.
- Date-line crossings are unwrapped, validated and split into bounded Polygon/MultiPolygon components. Holes and islands retain their meaning. Normalization uses [PostGIS ST_WrapX](https://postgis.net/docs/ST_WrapX.html) and [ST_Normalize](https://postgis.net/docs/ST_Normalize.html); a topology round-trip check rejects an unrepresentable serialization. Original geometry bytes remain separate from canonical/display geometry.
- Exact geometry reaches map queries, catalogs and local-subset validation. Component bounds support navigation; they do not replace an analytical polygon. Server `ST_Covers` checks reject processing across holes or outside the selected area. The existing capture/export size limits still apply.
- Geometry beyond ±85° remains retained and queryable, with an explicit map-display restriction. Component navigation avoids fitting a date-line selection to a near-worldwide extent. This does not add a polar renderer.

[Migration 00046](../../db/migrations/00046_analyst_scope.sql) adds release manifests/heads, immutable areas/profiles/scopes and temporary read manifests. Current resource/source RLS also protects scope provenance embedded in captures, jobs, assessment evidence and context packages. Losing the reference-geography grant cannot expose that source through an otherwise readable imagery job. Existing queue budgets still count jobs whose nested provenance is hidden from the caller.

### T05: Shared time and stable reads

[TemporalScope](../../internal/platform/queryscope/temporal.go) provides monitoring presets, fixed historical intervals, timezone, revision interpretation and explicit `[from,to)` boundaries. One server evaluation timestamp resolves every family in a query batch. Jobs and exports receive a fixed evaluated interval; a bookmarked monitoring preset may continue rolling. Browser display includes year, timezone and offset, with UTC available.

The new contract preserves `known_at` and `effective_at` as distinct event-history interpretations. Families without historical reconstruction return typed support metadata and an explicit unsupported state. Acquisition time filters imagery/capture catalogs. Monitoring background imagery is opt-in and labeled outside the selected period. Weather validity/current-feed state stays separate from observation time. Legacy direct endpoints retain their original inclusive-end behavior; imported bounded legacy periods advance to the next representable PostgreSQL microsecond through an explicit adapter, preserving records at the former inclusive endpoint.

[Read snapshots](../../internal/analystscope/reads.go) materialize a bounded result under one repeatable-read transaction, including stable membership/order, exact event revisions, family summaries and retained connector/admission receipts. Pages use stored results rather than an open transaction or a timestamp cutoff. A publication transaction that starts before a read and commits afterward cannot change that read's later pages. Later corrections and withdrawals appear in a new read.

Temporary manifests expire after five minutes. Each principal/resource can hold at most 16 manifests, each at most 8 MiB, with a 64 MiB combined budget. Exact event materialization is bounded at 5,000 rows; larger results require narrowing or explicit aggregates. Expiry creation, cleanup, page access and citation checks use the database clock consistently. Every page rechecks the owner, resource and current authorization revision. Expired/changed-access cursors require an explicit restart. Saving a cited result copies the bounded manifest into durable immutable scope evidence before expiry; subsequent cursor cleanup does not delete that evidence.

These receipts describe retained connector/admission state. They do not establish complete provider coverage or factual absence. Full workload, soak, recovery and all-family qualification remain later gates.

### T06: One scope through the user journey

[AnalystScopeProvider](../../apps/operator-ui/src/features/workspace/AnalystScopeProvider.tsx) owns the active map area and period. Systems, imagery discovery, capture, comparison, review and export use it or an explicitly pinned workbench snapshot. Operational reads of the old `investigation` browser preference are removed.

Pinned drafts visibly retain their area and period. **Use current map scope** explains and clears affected scene/pair choices and export previews before adopting both. Completed jobs and packages retain their submitted scope. Hidden retained workbenches suspend ordinary polling. Existing context packages now preserve their approved durable scope through preview, approval and offline bundle round-trip.

URLs, bookmarks and browser history carry the exact geographic identity or original drawn geometry and temporal selection. Conflicting, malformed, unavailable or inaccessible saved selections open recovery with no operational fallback. **Start a new view** is deliberate. Bookmarks remain view presets; durable investigations are T07.

Requests include scope, authorization and relevant source/temporal filters in their cache/query identity. Catalogs echo the resolved scope hash and evaluation timestamp; mismatched responses cannot populate a draft. Cancellation reaches geography, map queries, catalogs, detail/history reads and artifact previews. Late responses cannot replace a newer selection. Existing identity/resource cleanup also handles denied scoped reads by clearing protected caches/previews.

The [OpenAPI scope contract](../../api/openapi/analyst-scope.yaml) and regenerated TypeScript client describe area/scope resolution, durable freezing, read snapshots/pages, release/profile operations and optional scope fields on existing work. Actual routes are under `/api/v1/areas`, `/api/v1/analyst-scope`, `/api/v1/geography/releases` and `/api/v1/geography/profiles`.

### Verification performed

- `go test ./...`, `go vet ./...` and Go formatting checks pass. Default Go integration tests skip Docker scenarios unless explicitly enabled.
- UI typecheck and production build pass. Regenerating the OpenAPI client produces identical bytes. The existing Vite large-chunk advisory remains.
- **15 new browser cases** cover country, subdivision, named study, drawn polygon/hole, explicit viewport and date-line-component handoffs through Systems → imagery → review/export; period adoption; bookmarks/back/forward; invalid-link recovery; hidden polling; hash mismatch rejection; high-latitude display; denial of scope resolution/read refresh clears protected work until reconnect; and cancellation of a delayed obsolete response.
- **15 prior browser cases** cover the first batch, product entry and navigation. Browser APIs are isolated protocol fixtures; they do not prove PostGIS topology or installed coverage.
- **Three new real PostGIS cases** verify exact editions/releases/parents and original bytes, invalid geometry/subsets and holes, date-line event counts, immutable profiles/areas and source isolation; exclusive time boundaries and legacy compatibility; actual late-commit publication/outbox failure, corrections/withdrawals, stable pages, expiry/promotion, authorization changes and storage slots; frozen native-decoded capture inputs, time/containment failures, atomic writes and hidden-provenance queue limits.
- **Seven prior assembled integration cases** pass: map provisioning/Linux startup; purpose RLS/count/history; geographic history/aggregates/isolation; imagery worker reproducibility/independent-identity checks and scoped S3 export/offline round-trip; public-event correction/spatial semantics; native FIRMS/USGS bitemporal ingestion; and reference/terrain bytes/isolation. The Linux map image also resolves a real authenticated monitoring scope using `Asia/Riyadh` without a runtime descriptor.
- The map Linux image builds successfully. Python capture-worker syntax and four prior classification/backfill tests pass using the existing recovery environment. System Python lacks the optional geospatial dependencies; it is not the supported backfill environment.
- The planning harness and its 12 injected-fault checks pass. This establishes ledger consistency, not full application acceptance.

The [verification record](evidence/map-first/t04-t06/verification.json) records commands, test identities, source hashes, image identity and limitations. A [bookmark screenshot](evidence/map-first/t04-t06/bookmarked-scope.png) shows an isolated synthetic geographic selection and fixed historical period; it is presentation evidence only.

### Installation and remaining qualification

The user's preceding migration completion is recorded as reported installation progress. This pass did not inspect, migrate, restart or deploy the shared installation. **The new `00046_analyst_scope.sql` migration is required before running the updated API/workers/UI.** Follow the [normal setup/upgrade procedure](MAP_FIRST_OPERATIONS.md#map-profile), including the paired backup and coordinated maintenance process. The API readiness schema check now requires the new scope tables. Existing source grants must cover every source in a submitted scope, including geographic-reference sources needed by workers.

Additive schema and immutable evidence remain on rollback; use a forward fix or the established recovery procedure rather than destructive down migration. Previous scope-unaware workers must stay disabled for scope-bearing work.

T04–T06 are **IMPLEMENTED** with scoped evidence. Full A02/A03/A04/A08/A09/A22 scenarios remain `NOT_RUN` with partial assertions because they also span later case, reference-depth, scale and release qualification. No geographic gap, coverage matrix or independent-human usefulness gate is declared closed. T07 investigations, T08 regional installation and all subsequent tasks retain their planned scope.

<a id="t07-t09"></a>

## T07–T09: investigations, regional reference, retrieval profiles

> **Status:** T07–T09 implemented with scoped verification. The full phase remains unqualified; T22–T24 are still IN_PROGRESS. See the [records index](README.md) and the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation).

Prepared 26 October 2026, Asia/Riyadh. The user authorized the next three tasks after T04–T06, then requested T10–T12 while their verification was underway. This report records T07–T09; the [following report](#t10-t12) records the continuation. Both describe the uncommitted working tree based on `a6d1d22f34692faea31db58e4a6318c539c3321a`.

Implementation and isolated verification do not establish shared deployment, complete acceptance scenarios or independent human qualification. The [verification record](evidence/map-first/t07-t09/verification.json) identifies its scope and the final combined checks.

### T07: Durable investigations

[Investigations](../../internal/investigations/service.go) now retain authored revisions, exact evaluated area/period, analytical question, notes, case state and exact evidence references on the server. Source evidence remains immutable. Bookmarks remain view presets. The UI offers explicit server save, resume, conflict-preserving editing and deliberate reload; browser-local storage does not retain operational case drafts.

Membership uses canonical registered human identities. Owners control membership, transfer, archive and reopen. Membership has its own compare-and-swap version, independent of the authored revision head. A transfer verifies the successor's eligible permissions; an alias is not another human. Immutable membership events retain the audit trail.

Every case revision and history read requires all cumulative source dependencies, including evidence removed from the latest draft and reference geography nested in a scope or job. Source revocation cannot expose an old title/history through another readable revision. Workers cannot obtain human case membership. The browser clears an already displayed case when membership/source access becomes unavailable.

The map restores the saved canonical selection outline as well as the query geometry when a drawn case resumes through its persisted area revision. Rich finding/review/brief authoring remains T16–T19. Later corrections now appear as separate current-validity notices, added in T12; they do not rewrite the saved evidence.

[Migration 00047](../../db/migrations/00047_investigations.sql) installs case heads, immutable revisions/membership events and resource/source/human RLS. The [contract](../../api/openapi/investigations.yaml) and browser exercise cover saving in one browser and resuming in another. That protocol fixture is not an independent-user session.

### T08: Source-backed regional geography

[Regional preparation](../../scripts/data/regional_geography.py) accepts a reviewed region profile. It builds worldwide country navigation and selected-country ADM1/place depth from actual Natural Earth source archives, preserves source identities and aliases, and adds explicitly labeled study areas from configuration. The [default Middle East profile](../../scripts/data/region_profiles/middle-east-v1.json) includes 17 configured source countries; its definition is explicit and does not assert jurisdiction or maritime detail.

The 50m country source includes Bahrain, which the previous 110m country archive omitted. Source country identity `PSX` is retained for the configured Palestinian feature. ADM1 means the installed source's administrative interpretation: the France sample contains departments, not a newly invented regional hierarchy. Names and source geometry remain retained; search normalization and display do not rewrite analytical boundaries. Natural Earth's published [terms](https://www.naturalearthdata.com/about/terms-of-use/) place its data in the public domain and qualify its accuracy/completeness; those receipts are preserved with the prepared source.

Preparation sorts stable source IDs, validates selected countries, splits bounded chunks, fingerprints source receipts and reviewed rights, and supports `--prepare-only`. Installation stages all chunks, retries by content identity, and activates a complete per-source release using the expected head. Staged first-install chunks do not become discoverable through the legacy no-head path. Exact historical feature IDs remain readable. The installer persists the configured countries and study-area members in the region profile.

The source-backed isolated test prepared and admitted four real chunks for Saudi Arabia, UAE, Egypt, Turkey, France, Japan and the USA. It verified source-qualified editions, exact selection, retained ADM1/place depth, accent search, same-name features from different sources, retry safety and atomic activation. The source depth and preparation digest are in the verification record. This is broader than a fabricated seven-box fixture; it does not qualify each country's real analyst workflows, terrain, roads, imagery or jurisdictional accuracy.

[Migration 00048](../../db/migrations/00048_geographic_catalog.sql) adds normalized accent/alias search and durable reference-depth requests. Search exposes continuation, the UI retains previous pages, and availability names the exact source/release depth. Analysts can request an import; administrators can verify resolution only against a compatible installed release. A request does not download data or grant ingestion authority. Generalized roads/DEM preparation remains T20.

### T09: Profile-bound acquisition authority

[Migration 00049](../../db/migrations/00049_retrieval_profiles.sql) maps each old admission to the `default` profile without changing its document, cursor, frozen query, quota, retry history or backoff. Existing running connector leases are fenced. A security-invoker compatibility view retains source-only default reads/writes while new authenticated server code selects a profile. Named profiles keep independent state and immutable approved revisions; a new profile receives no execution grant automatically.

The [profile policy resolver](../../internal/publicingest/profiles.go) resolves the input's original admission/profile and intersects its rights with that same profile's current approved rights and source state. Artifacts, raster workers, captures, comparisons, export preview/freeze and downloads use that resolver. Explicit original-admission dependencies propagate through derived artifacts. Unknown/mismatched mappings fail closed. Immutable original admissions remain readable as provenance; a different profile cannot supply broader rights. Policy locks also cover the final processing/publication commit after external I/O.

Execution grants identify the exact profile revision and eligible service identity. Every begin, request reservation, page commit and finish rechecks current authority. An older policy-unaware reader can read legacy/default inputs but cannot process named-profile raster inputs through a universal default-policy decision. The [caller inventory](MAP_FIRST_PROFILE_POLICY_CALLERS_2026_10_26.json) enumerates policy calls and source-state queries, including intentionally default-only health/legacy maintenance paths.

Successful overlapping retrievals retain occurrence links to the same source/provider record/revision. They do not create additional independent evidence. Conflicting bytes for an upstream revision fail explicitly. Source-wide pacing/quota checks prevent profiles multiplying a source allowance; T10 adds durable account-wide reservations across sources/endpoints.

The Sources UI separates approving a profile from granting its execution. It displays reviewed bounds, lookback, purpose and exact revision. Selecting a country changes neither approval nor grants.

### Verification and installation boundary

Real isolated PostGIS/private S3 checks cover immutable history, membership and save conflicts, old-schema populated migration, overlapping profile deduplication, original-profile rights, stale leases, source suspension and revocation during an upload. Browser protocol checks cover shared resume, conflict retention, membership loss, reference request retry, paging and explicit profile grants. Python preparation tests cover stable identities, bounds, resumable staging and receipt-sensitive versions.

The final assembled suite also runs T10–T12 and prior native provider/imagery paths. See the machine-readable verification records for actual commands and results. The reports do not reinterpret all acceptance scenarios as PASS.

The user reported applying migrations preceding this work. This session did not apply 00047–00050, admit a release into the shared installation, restart shared workers or deploy. Use the coordinated upgrade in [operations](MAP_FIRST_OPERATIONS.md#map-profile), with the latest schema/API/workers/UI together. Keep old processes stopped during the forward cutover; downgrading these migrations is intentionally rejected. Independent users, installed geographic qualification, real account-cost policies and unattended soak/recovery remain separate gates.

<a id="t10-t12"></a>

## T10–T12: acquisition, coverage and task readiness

> **Status:** T10–T12 implemented with scoped verification. The full phase remains unqualified; T22–T24 are still IN_PROGRESS. See the [records index](README.md) and the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation).

Prepared 26 October 2026, Asia/Riyadh. The user requested the next three tasks while T07–T09 verification was underway, then asked to continue. This batch implements T10–T12 in the existing working tree. The [preceding report](#t07-t09) records durable cases, regional references and profile authority.

The [verification record](evidence/map-first/t10-t12/verification.json) identifies the source fingerprint, commands, assembled image and scoped evidence. IMPLEMENTED describes source and scoped checks. The full acceptance scenarios, shared installation and phase qualification remain separate.

### T10: Durable acquisition and shared budgets

[Acquisition requests](../../internal/acquisition/service.go) retain a registered human's intent, exact scope snapshot, original purpose, exact profile revision and stable request key. Canonical input identity is scoped to resource and requester. An identical retry returns the same committed request and frozen scope, including after a lost response or advancing monitoring clock. A different input with the same key conflicts. Scope persistence, intent and bounded queue parts commit together.

Only requests within an enabled, approved profile envelope and explicit execution grant enter execution. A request outside the footprint, rights or supported operation becomes `COVERAGE_REVIEW`; it creates no execution authority. Current-only adapters keep their approved current polling and cannot replay a frozen historical window. There is no browser-supplied provider URL or credential path. A request may reference an authorized investigation without modifying its saved revision.

Requests split into at most 64 area-component/time parts. The [existing connector executable](../../apps/emad-connectors/main.go) rotates due authorized profiles and performs bounded queue work alongside current polling. Profile state pins cursor, query interval, footprint and request owner. An unfinished request cannot lend its cursor to another intent or default polling. Claims, original revision and live authority are rechecked at provider reservation, page commit and completion. Cancellation fences active/unstarted work while retaining bytes, occurrences and charges. Bounded attempts, exponential backoff/jitter, next-attempt time, terminal state and recovery owner are inspectable. A deliberate new intent creates a new key; a retry of the same submission keeps its key.

Trusted connector configuration can bind several endpoints/sources to one opaque `accountBudget` group. Group daily allowance, pacing and concurrency are consistent across bindings; adapter request costs may differ. The local USGS and USGS-history bindings now share an allowance. Reservations commit before external connector I/O. Unknown sends remain charged until conservative UTC reset, even after the concurrency lease expires. Known responses reconcile metadata without refunding estimated costs. A reservation explicitly proven unsent may be released. Provider-reported remaining credits can tighten the gate; missing credits remain unknown. Stricter per-source and per-profile quotas also remain enforced. Changing a live account policy fingerprint requires deliberate operator reconciliation, rather than silently resetting a spent allowance.

[Capacity guards](../../internal/platform/capacity/capacity.go) serialize an 8 GiB resource retained-byte budget and at most 32 concurrent reservations. Connector work reserves estimated bytes before I/O; actual page commits and artifact reservations recheck usage across concurrent and hidden-source work without exposing hidden counts. Raster capture claims also reserve 64 MiB, renew while their lease is live and settle on completion. PENDING artifacts count their bytes, including a failed upload whose object might exist. Expired work releases a reservation, never deletes evidence or referenced objects. Existing stricter source artifact/raw-evidence and lifetime history limits remain; paging and safe lifetime-cap replacement are T13.

`GET /sources/{id}/acquisition-metrics` exposes durable source-authorized queue age, attempts, quota reservations, uncertain charges, storage reservations and coverage receipts. It returns no account group, credential or another source's totals. Operational health remains separately inspectable. These metrics are not a claim of a completed unattended soak.

The account model qualifies configured connector calls. Anonymous raster byte-window retrieval retains its existing fixed-host, size, integrity and lease controls; no provider credit schedule is inferred for that endpoint. Real account-specific costs, storage retention, orphan-object reconciliation and long-running raster/provider behavior require the installed-policy and T15/T22 qualification gates. No retained object is automatically deleted as part of this change. Brief rendering/environment-preparation executors arrive with their later tasks and must use these intent/capacity contracts.

### T11: Coverage for the selected area and interval

[Coverage queries](../../internal/coverage/service.go) resolve the selected scope and compose authorized profiles, exact historical receipts, retained records, reference depth, rights, queued work and storage state. The [contract](../../api/openapi/acquisition-coverage.yaml) keeps support/access, spatial overlap, temporal retrieval, freshness, data result, observation completeness and next actions separate. A healthy worker or workspace-wide row count cannot establish task readiness.

Completed retrieval receipts retain attempted footprint/window, adapter version/capability, pages, rejected records, provider caps, retained occurrences and publication gaps. A capped response, rejected normalization, exhausted page budget or pending publication cannot establish a complete or successfully empty interval. Successful overlapping profiles refer to existing evidence identities. Receipt footprints are unioned across profiles for each temporal slab, so adjoining successful queries can cover a selection without double-counting observations. Today's approved AOI never substitutes for an earlier receipt's footprint. The response bounds receipt/source work and explicitly labels receipt truncation. Publication recovery can reconcile an original receipt’s gap without refetching provider bytes; the response preserves the originally recorded gap alongside the current publication annotation. Freshness measures successful retrieval, not a later failed attempt.

`no_retrieval`, `partial_retrieval`, `successful_empty_retrieval`, `usable_retained_records` and `unavailable_query` have different meanings. Observation completeness remains unknown. Permission filtering precedes names, counts, footprints and job/request IDs. Source-restricted users cannot infer hidden source identities through the coverage response or quota metrics. A reference release's depth remains separate from live observation coverage.

The installed capability inventory covers USGS, FIRMS, Earth Search, CDSE/STAC, METAR, GDACS JSON/RSS, OpenSky, AIS and reference/DEM import operations. It describes actual installed adapter contracts, credentials, retention/rights, historical support and limits; it does not add historical acquisition to current-only feeds.

The map scope strip and Systems now offer **Coverage & acquisition**. Capture/export workbenches expose the same scope-specific evaluation while retaining their operation-specific server validation. Typed actions identify an approved profile revision, a saved request and a recovery owner where applicable. Submission explicitly saves an intent; it does not execute a provider call in the browser. Failed submission retries retain the same frozen payload/key.

### T12: Monitoring and historical study

The shared temporal contract remains the basis for every query. Monitoring presets roll through one evaluated interval. While a bounded read is retained, historical map/catalog reads disable ordinary polling, focus, reconnect and remount refresh, and keep membership until explicit refresh or a deliberate new query/scope. Expired server cursors require a new read. Camera changes and layer toggles do not silently replace a historical map read. Explicit refresh evaluates a new retained result for the same fixed interval. Current-only feeds return unsupported historical states. Background imagery remains opt-in and labeled by the existing family adapter.

Each family in a map read now has its own database savepoint. A real SQL or permission failure rolls back that family without discarding successful siblings. The response serializes an unavailable/unsupported count as `null`, with a typed error and recovery owner. Paging an unavailable family requires recovery rather than returning an invented zero. Shared input/scope validation still fails the request when it is invalid.

Quiet summaries use the selected area's coverage: unqueried, partial, successfully empty, unsupported and unavailable states have distinct wording. Empty results never assert factual absence. The evaluated interval appears in map/evidence inspection and saved investigation checkpoints.

Current provider validity is a response annotation, separate from a retained revision or historical interpretation. Inspectors and reopened cases can show a current, superseded or withdrawn provider report without altering its original bytes. Aggregates retain selected-revision counts and separate withdrawal/supersession annotations. Provider quality, reported geometry and derived findings keep their existing meanings; the UI does not convert a report into independently verified fact. Full finding dependency/review/brief workflows remain T16–T19.

### Verification and upgrade

Focused real PostGIS tests exercise lost-response idempotency, frozen cursor ownership, outside-envelope review, cancellation during provider I/O, two sources sharing one account, uncertain-charge expiry, source-isolated metrics, storage exhaustion, adjacent receipt unions, capped/rejected responses, historical footprints after profile relocation, gated credentials, SQL-family failure, immutable historical membership and current withdrawal notices on saved evidence. The broader assembled suite retains native adapter, original-rights, private S3, imagery, export, reference, identity and migration coverage.

Browser protocol tests exercise frozen acquisition retry without provider execution, historical focus/reconnect stability with explicit refresh, and unknown failed-family counts alongside successful siblings. The complete combined browser suite includes the preceding map/scope/case/profile regressions. These fixtures do not prove real provider coverage or independent-user usability.

[Migration 00050](../../db/migrations/00050_acquisition_scheduler.sql) is required in addition to 00047–00049. It fences outstanding connector/raster/imagery attempts, preserves retained inputs/history, installs requests/parts/receipts and private budget/capacity ledgers, and prevents older worker counters/job claims/artifact reservations bypassing the new policy. Requests and their frozen inputs are immutable apart from operational status/recovery fields. Readiness requires the scheduler tables.

Stop the existing installation before migrating; rebuild API, workers and UI together, reprovision map identities for `acquisitions.read`/`acquisitions.request`, then start and inspect source/account policies. Back up first and use forward recovery. This session did not migrate, restart, admit new shared data or deploy the user's installation. See [operations](MAP_FIRST_OPERATIONS.md#map-profile) for the supported commands. T13–T15 are the next planned tasks; installed account/geographic/user qualification and T22 soak/recovery remain open.

<a id="t13-t15-design"></a>

## T13–T15 design contract

> **Status:** Historical design note preceding the T13–T15 increment. The implemented outcome is recorded in [MAP_FIRST_T13_T15_IMPLEMENTATION_2026_10_26.md](#t13-t15). See the [records index](README.md).

This increment retains one MapLibre canvas with the existing WGS84 source layers, quiet basemap, canonical selected area and explicit monitoring/historical period. Server-owned snapshots provide exact membership/order/counts and opaque continuations. Both map and catalog reads share the existing 16-slot, 8 MiB/read, 64 MiB/principal/resource budget and five-minute expiry; no transaction remains open between pages. Expiry, changed source/case access or a mismatched query requires an explicit new read.

The map's geographic job is lookup and spatial inspection. Locator/aggregate marks and outlines stay stable in screen space; real scene/subset/reference shapes remain map-space geometry. Aggregates mean retained selected provider revisions, not independent incidents or verified absence. Individual-record drill-down uses the same materialized read, including corrections/publication membership at evaluation; current provider validity is separately annotated at a page read. A paged text list provides the keyboard/non-map path. Display simplification never becomes submitted analysis geometry.

Map read manifests retain compact ordered immutable report IDs privately; the initial response does not transmit those lists. Catalogs cover source-qualified geography, scenes, captures, jobs, assessment heads and investigations. They retain bounded immutable response documents; permission/dependency visibility is rechecked before returning a page or count. Technical status detail is disclosed on demand. Hidden workbenches stop ordinary polling and obsolete reads abort; query keys retain scope/access identity. Oversized reads offer a smaller area/period/source instead of inventing an approximate total.

The existing imagery canvas gains a selected-area/actual-scene/proposed-subset preview. Automatic capture remains confined to the supported provider path and the 0.1-degree-per-side bound. A suggested local rectangle must be explicitly adopted and pass exact polygon/scene containment. Server preflight applies the existing grid/platform/tile/orbit/baseline/date and clear-pixel rules before comparison; zero change remains valid. Durable intent identity binds resource, operation, registered requester and canonical original input before evaluating rolling time. Case links name exact revisions and never grant processing authority.

The geospatial specialist owns aggregate/reference/subset encodings; accessibility guidance owns keyboard, focus, contrast and narrow-screen continuation. These passes are local: no delegation is authorized. The palette keeps neutral reference context, existing family colors, and distinct text/outline selection and failure states; color alone never conveys status. No generated graphics or replacement renderer are needed.

Role-led actions use effective permissions for analyst, reviewer, source-administrator and restricted-reader accounts. English application text and Arabic/source-content support are reported separately. Labels keep explicit timezone/year and longitude/latitude/WGS84 meaning; unsupported coordinate systems receive a recovery message. Source names/notes use bidi isolation. RTL shaping is checked against installed MapLibre 5.24.0 and packaged locally with pinned assets rather than an unrelated upgrade.

Portrait 320/390px, short landscape and enlarged text keep the scope visible, panels scrollable and keyboard targets reachable. Controls provide accessible names, focus restoration, explicit loading/partial/unknown states and manual alternatives to map gestures. QA combines actual PostGIS snapshot/capacity/EXPLAIN checks, private S3/scientific-pipeline tests, browser role/keyboard/mobile checks and visual inspection. Real additional-area acquisitions and observed human sessions retain separate evidence states; synthetic pipeline or browser fixtures cannot satisfy them.

<a id="t13-t15"></a>

## T13–T15: paging, role workflows, imagery intents

> **Status:** T13–T15 implemented with scoped verification. The full phase remains unqualified; T22–T24 are still IN_PROGRESS. See the [records index](README.md) and the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation).

T13, T14 and T15 add saved catalog paging, role-led international map workflows, and repeatable imagery intent handling. They are **IMPLEMENTED**, subject to the verification boundaries below. No task, complete acceptance scenario, release candidate or observed-user session is marked qualified by this increment.

### T13: saved reads and bounded history

`POST /api/v1/catalog/reads` serves geography, scenes, captures, jobs, findings and investigations. A repeatable-read transaction freezes ordered identities and response documents. Pages return exact total/returned counts, representation, evaluation/expiry times and opaque continuation. The cursor binds owner, resource, original scope/filter/page size and authorization revision. Current source, content-purpose and case visibility are checked before returning either a page or its count. Later arrivals and review-head changes do not join an existing read.

Reads share 16 slots, 8 MiB per manifest, 64 MiB per principal/resource and a five-minute expiry. Explicit same-query catalog refresh atomically replaces the prior ephemeral read. It cannot delete another user's read or durable evidence. Oversized reads require a smaller area, period or source. Client requests abort when obsolete; SQL statements have a 12-second deadline and catalog/map HTTP reads have a 15-second deadline. Expired catalog controls require an explicit refresh.

Geographic catalog shapes are simplified for display and tagged `displayOnly`; analytical selection resolves the original source-qualified feature identity. Geographic search, subdivisions, scenes, captures, jobs, findings and cases have continuation controls. Legacy imagery lists disclose their first-page ceiling, returned count and saved-catalog recovery path; a truncated list has an unknown total.

Aggregate maps retain ordered immutable report revision IDs privately in the same manifest. Display/count/membership are evaluated in one SQL query. The initial map response does not transmit the membership list. `GET /analyst-scope/reads/{id}/records` pages original revisions, optionally within one original aggregate cell. It never selects new latest heads. Current provider validity is a separately dated annotation. Representative locations remain display locators, distinct from reported geometry. Individual reports are accessible through a keyboard list, including records within an aggregate.

The obsolete lifetime ceilings of 1,000 capture-history rows, 1,000 artifacts and 10,000 raw records are removed. Active captures remain limited to four; source job slots, provider quotas, 512 MiB/source artifact bytes, 64 MiB/source raw bytes and resource capacity reservations remain enforced. No history or orphan object is deleted to recover capacity.

Catalog snapshots do not poll. Selected active job/capture detail queries share identity-based query keys, stop on terminal states and suspend while their workbench is hidden. Systems and imagery discovery share catalog query keys. Exact area/period and source access stay in the cache identity.

### T14: navigation, input and renderer behavior

The workbench chooser offers investigations and coverage directly and orders permitted analyst, reviewer and source-administrator actions using effective permissions. Systems remains available for evidence and operational inspection. One MapLibre canvas is retained across workbench changes. An explicit area link fits its selected geometry unless the link carries an explicit camera state.

Signed longitude/latitude input has explicit WGS84 order, accepts Arabic/Persian digits and Arabic separators, and rejects unsupported formats and ranges. Capture coordinates identify longitude and latitude without fixed E/N hemisphere assumptions. Period labels identify a Gregorian year, IANA timezone and exclusive end; scene dates identify UTC. Arabic/source names use bidi isolation; authored notes accept automatic direction.

The installed MapLibre version is **5.24.0**. RTL plugin **0.3.0** is packaged locally with its complete upstream license. Tests check worker registration and actual Arabic shaping/mixed-direction output. Area names use DOM labels with the packaged IBM Plex Arabic font; this is not a claim that every WebGL symbol layer uses a local Arabic glyph atlas. No font service is required. See the [MapLibre RTL API](https://maplibre.org/maplibre-gl-js/docs/API/functions/setRTLTextPlugin/) and the pinned plugin's [source documentation](https://github.com/mapbox/mapbox-gl-rtl-text/blob/v0.3.0/README.md) and [license](https://github.com/mapbox/mapbox-gl-rtl-text/blob/v0.3.0/LICENSE.md).

Keyboard names/focus restoration, 320/390px portrait, short landscape and 200% text checks are recorded in browser evidence. Narrow/short screens permit vertical scrolling so scope and toolbar cannot consume the draft's whole viewport. Renderer initialization failure leaves the exact scope and workbench forms available with an explicit recovery notice.

The interface remains English with Arabic/source-content support. Independent observed users have **not** been substituted with browser fixtures. The session protocol in `MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md` remains NOT_RUN until actual users and an observer participate.

### T15: original intent and retained imagery

`POST /imagery/capture-proposal` returns the actual scene footprint, proposed rectangle, exact containment outcome and a suggested inscribed rectangle. The selected area, purple scene outline and amber subset remain separate. Adopting a suggestion is deliberate. The server repeats exact `ST_Covers` checks against holes/multipart geometry and the retained scene at submission. The supported automatic path remains EarthSearch Sentinel-2 Collection 1, at most 0.1° per side. CDSE catalog metadata does not imply an available raster-capture path.

`POST /imagery/comparison-check` reads the retained original subsets and uses the scientific comparison rules: grid, platform, tile, orbit, processing baseline, chronological acquisitions within 90 days, selected period/area, original profile rights, eligible reflectance/SCL masks and minimum common clear coverage. It returns usable-cell counts and recovery reasons. Zero-change output remains valid.

An immutable intent ledger binds resource, operation, canonical human (or authorized processing service), original request key and canonical **unevaluated** input. A retry finds this ledger before evaluating a rolling period or current case head. It returns the original job; changed input with the same key conflicts. Credential rotation for the same human preserves identity; a different human receives a separate intent. Entity creation, frozen scope and intent commit atomically.

Optional case links identify an exact saved case revision. New explicit links require current membership/edit authority and the current saved head. Capture-to-preparation inheritance preserves the original revision without giving processing services case text or new source permissions. Analysis preserves matching parent links; incompatible case links require an explicit valid choice. Human reads of linked work retain case membership checks, including catalog counts after membership loss. Existing captures/jobs keep their identifiers; only newly submitted intents enter the new requester namespace.

Fresh anonymous provider retrieval retained two real scene pairs: **Al Ain, UAE** (1/11 June 2025) and **Avignon, France** (9/19 June 2025). Both passed the existing offline NDVI algorithm with 16,384 and 15,684 jointly eligible cells respectively, on 128×128 grids. Raw windows, STAC responses, masks, previews, hashes and ETag receipts are retained under `private/qualification-t13-t15`; the shareable receipt is `evidence/map-first/t13-t15/live-imagery.json`. These are freshly retrieved **historical acquisitions**, not current observations or verified vegetation loss. This isolated live pixel/offline comparison evidence does not claim production admission, a complete live application journey, independent review or publication.

### Verification and remaining qualification

The machine-readable result and exact source/image fingerprints are in `evidence/map-first/t13-t15/verification.json`. Final checks passed: **33 assembled integration tests, 49 browser tests, all Go package tests, Go vet, generated contracts, the production UI build and the planning harness**. The implementation record was finalized on 27 October 2026 (Asia/Riyadh). Tests use isolated PostGIS/private S3, the compiled Linux API image, scoped browser fixtures and fresh public raster windows. Screenshots and PostGIS EXPLAIN outputs accompany that record. Existing browser tests initially regenerated preview files at their historical output paths; their earlier JSON hashes were not rewritten and should not be treated as current proofs. Those test outputs now target this increment’s directory, which holds the newly fingerprinted captures.

The larger synthetic workload has 100,000 report revisions, 50,000 detailed reference polygons and a separate 1,105-row capture history. It checks exact membership/counts, later changes, opaque cursor isolation, history beyond former ceilings and exact analytical geometry. On the recorded serial run the combined aggregate read took 5.48 seconds and the first individual page 4.06 seconds. These are single-run observations, not p95 measurements; they exceed the proposed interactive latency targets. Five simultaneous users, ten scheduled profiles, mixed 10,000-row weather/track load, full dateline/density workload combinations and sustained qualification remain T22 work. The preserved SQL plans make that follow-up measurable.

Windows commit-memory/GPU pressure interrupted an earlier parallel verification attempt. Final checks were serialized; renderer failure handling was also verified explicitly. This is not a successful soak or recovery rehearsal.

The user reported applying the preceding migrations, assumed through **00050**, without independent inspection of the shared database. This batch introduces **00051_catalog_and_imagery_workflows.sql**. It has been applied only in isolated tests. The shared application was not migrated, restarted or deployed. A host memory interruption stopped the pre-existing local PostgreSQL container; the same container was started again and `pg_isready`/Docker health were verified. No shared migration was run as part of that recovery. Coordinate the normal application/worker upgrade with migration 00051; rolling back by deleting immutable history is unsupported.

T16–T24, complete A01–A24 journeys, independent users and release qualification remain open. T13–T15 are not marked COMPLETE.

<a id="t16-t18"></a>

## T16–T18: evidence, findings, independent review

> **Status:** T16–T18 implemented with scoped verification. The full phase remains unqualified; T22–T24 are still IN_PROGRESS. See the [records index](README.md) and the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation).

This increment implements case findings and save recovery (T16), independent case review (T17), and composite-output rights/storage foundations (T18). These changes belong to the combined uncommitted working tree. Task closure and full application qualification remain separate from scoped automated checks.

### T16: findings, exact evidence and recovery

Cases select exact provider reports, completed analyses, captures, assessment revisions, reference editions and retained artifacts. The server resolves hashes and source lineage again when saving. An analysis citation freezes both its input and completed result; historical `job` links remain readable but block review/output until the exact result is adopted. Evidence is never edited by the case service.

Finding revisions contain authored conclusions, confidence/rationale, uncertainty, purpose, exact citations, the original scope hash, canonical author and previous finding revision. Unchanged findings retain their revision and scope when the case area changes; a notice requires reassessment. Changed conclusions create immutable successors. Unclassified/development/acceptance inputs cannot silently become working findings. Retained artifacts now have an explicit purpose classification path through the existing source-administration catalog.

Case history retains cumulative source dependencies even when current citations/findings are removed. Typed lineage includes original admissions and secondary artifact admissions. Reverse dependency records are indexed; current support notices are recomputed on case open, review and output eligibility, so missed notifications cannot conceal an unavailable input. Current source state, purpose, policy, retention and provider-support notices stay outside the immutable authored content.

Save intents bind the original input to resource, canonical human and request key. Case revision, findings, dependencies and intent commit together. A lost-response retry returns the original revision even after another editor advances the case. Changed payloads with the same key conflict. Live grants are checked before commit and recovery. New revisions still compare the expected case head and finding revision.

The browser persists an account/resource/case-scoped unsent intent and autosaves valid changes after a pause. A reopened recovery copy is submitted to the server for current authorization and conflict checks before its content is displayed. The operator can discard it. Device storage is a recovery buffer, not authoritative case storage or a source grant. Browser profiles/devices need their normal access controls. History comparison shows notes, question, scope, exact citations and authored finding fields. Existing ownership transfer/archive/reopen rules remain in force.

### T17: exact-content independent handoff

Review requests, changes requests, approvals and supersession events are append-only. Requests bind the current case revision and content digest, expected prior workflow event, assigned canonical reviewer and substantive reason. A new content revision supersedes pending review; prior approvals remain historical. The review inbox contains only currently pending, assigned and visible cases.

The reviewer must already be a case member with live review/read grants covering every cumulative source. Credentials for the same human do not establish independence. All case-history authors, finding authors, cited assessment authors, and any output creator/render contributor are excluded. Services cannot perform human review. A reviewer who edits becomes a contributor to the successor. Grant/credential validity is checked at commit; original admission policy heads are locked for positive review decisions. Source problems block approval but permit a request for changes.

Output review requires an existing READY render. The immutable attestation binds case revision, manifest digest, output identity and exact rendered SHA-256. Approval does not rewrite content or rerender bytes. Independent-human observed sessions remain **NOT_RUN**; distinct test credentials prove protocol behavior only.

### T18: explicit rights, frozen manifests and private bytes

Optional `rights.outputPermissions` fields are tri-state: `internalAnalysis`, `citation`, `includedImagery`, `rawDistribution`, `geographicEnvironment`, and `metadataRetention`. Missing/null permissions fail closed. Original and current policy permissions intersect. Admission lineage stays within the original retrieval profile; independent reference imports use their original and current dataset versions. Legacy `allowSimulationExport` remains insufficient for general analytical output or brief distribution. Existing simulation-package permissions are unchanged.

An output freezes the complete case revision, all historical source-policy dependencies, exact selected assets, explicit source uses/omissions, renderer/style versions, intended purpose and retention. Cumulative prose dependencies cannot be removed to avoid output rights. Assets must be explicitly cited, retained, permitted and within their original retention. External references and declared omissions allow a permitted citation without embedding disallowed bytes. All-source case membership/RLS applies; source administration is not blanket case access.

Frozen manifests reserve resource storage capacity, including PostgreSQL JSON document bytes and the bounded rendered-byte reservation. Original/effective rights, current policy identities and evaluation times are recorded in each dependency decision. Content is immutable after staging. Private reads verify exact byte count/hash and recheck live identity, all-source access, original/current rights and retention after I/O. Staging repeats live creation checks after I/O. Distribution additionally requires the exact independent render attestation and distribution grant. Authorized historical draft artifacts remain pinned and labeled superseded; they cannot be distributed as current approved work.

This foundation accepts bounded JSON render plans and PNG figures, at most 8 MiB, with PNG limits of 4096 per side and 4 megapixels in total. It does **not** implement T19's HTML brief renderer. An uploaded rendering is explicitly declared and must itself be reviewed. No automatic screenshot, external network rendering, HTML execution or zip packaging is introduced here.

Expiry uses DELETE_PENDING followed by private-object deletion and a minimal TOMBSTONED manifest containing only case revision ID, content digest and tombstone flag. Deletion failures remain retryable; expired bytes cannot be restaged. Frozen retention cannot be extended by citation. IDs, hashes and append-only audit events remain. Reconciliation is an explicit authorized API/UI operation in this foundation; unattended scheduling and full production recovery/soak qualification are not asserted by these tests.

### Verification and installation boundary

The final commands, results, logs and source fingerprints are recorded under `evidence/map-first/t16-t18/verification.json`. Final checks pass for **39 unique assembled integration cases**, **53 unique browser cases**, all Go package tests, Go vet, generated contracts, the production UI build and final Linux-image startup. Integration evidence combines the broad regression with focused reruns: one alias fixture initially lacked its required source-read grant, was corrected, and passes in the final workflow run. The final composite/shared-quota rerun verifies manifest-byte capacity accounting. Browser evidence combines 52 regression cases with a final 10-case workflow run, including the new save-race case. Test fixtures use isolated migrated PostGIS and private stores. Browser fixtures establish UI/protocol behavior; they do not establish observed independent human review or real publication.

The user reported applying the preceding migrations, assumed through **00051**. This increment adds **00052_case_workflows_and_outputs.sql**. Only isolated test databases are migrated by this work. Apply 00052 through the normal installation process before running the updated application. No shared application migration, deployment, restart, commit or publication is performed by this increment. Rollback must preserve immutable case/review/output history; use forward recovery.

T19–T24, observed independent users, complete A01–A24 journeys and release qualification remain open. T16–T18 are not marked COMPLETE.

<a id="t19-t21"></a>

## T19–T21: readable briefs and portable geography

> **Status:** T19–T21 implemented with scoped verification. The full phase remains unqualified; T22–T24 are still IN_PROGRESS. See the [records index](README.md) and the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation).

This increment implements readable briefs (T19), area-parameterized geographic normalization (T20), and durable geographic preparation/packages (T21). Source and scoped automated evidence are recorded separately from full acceptance. No task, independent-user journey or release candidate is marked COMPLETE/qualified by this increment.

### T19: frozen readable output

The case delivery panel creates an immutable output and a durable job atomically. An owner-only intent ledger binds resource, canonical human, request key, case path and original input before evaluating current renderer defaults. Changed input with the same key conflicts; recovery returns the original job. Device recovery preserves the original submission across reopening and rechecks live case/source grants. It never grants new access or displays recovered source content without authorization.

The independent `emad-delivery` process renders outside HTTP. It uses the original creator's current human credential and case/source grants, with no new case membership or review authority. One worker instance handles its credential's canonical author; deploy additional author-bound instances for other authors. Aliases remain the same contributor. Leases, immutable attempt events, bounded retry/backoff/jitter, stale-attempt fencing, cancellation and lease-expiry recovery survive process interruption. Attempts are capped at three; failures and recovery actions remain visible.

The brief freezes the case, exact citations, evidence meaning, provider quality, derived comparison counts, overlapping source-retrieval receipts, cutoff, omissions, rights decisions and renderer/style identity. Coverage is bounded to 64 intersecting-period/footprint receipts and explicitly discloses truncation. It never treats retrieval completion as complete study-area coverage or absence of phenomena. Human confidence remains separate from provider quality and derived indicators.

The Go renderer produces escaped, script-free, printable HTML with a schematic outline of the exact frozen study boundary. The outline uses angular WGS84 coordinates, is north-up, and disclaims distance/legal-boundary interpretation. Explicitly declared retained PNGs are validated, hash-checked and embedded locally. No basemap, live screenshot, remote font or evidence fetch occurs during rendering or opening. Source data/notes cannot introduce HTML, script, forms, URLs or arbitrary file paths. Print styles retain the readable report while omitting the expanded machine-readable manifest from the printed body.

The authenticated application embeds the retained HTML in a sandboxed reader. Draft reading and approved distribution use separate checks. Independent review binds the existing case revision, manifest and exact rendered SHA-256; it never rewrites the brief. The approved bundle contains the same `brief.html`, `manifest.json`, `approval.json`, and a readable `approval.html`. Its approval label is separate from frozen content and explicitly historical. Source originals not permitted for inclusion remain declared external dependencies. Existing context-package export permissions/formats are preserved.

### T20: parameterized retained inputs

`prepare_portable_geography.py --spec ... --output ...` accepts an explicit polygon, reviewed metadata/accuracy/rights, hash-pinned local GeoTIFF tiles and optional vectors. It performs no download or source admission. One normalized DEM mosaic uses one source identity; mixed providers must be prepared as separate admitted inputs rather than assigned an arbitrary source identity. Missing coverage, wrong hashes, unknown vertical datum/units or unsupported CRS applicability fail explicitly.

Native DEM/vector bytes are retained in the private preparation directory. Structured parent metadata preserves every tile's hash, bytes, source identity, reference, retrieval date, CRS and datum. The tool uses a bounded 2–64-node sampling grid with independent row/column counts, explicit selection/nodata masks and exact polygon/vector intersection. Horizontal transformation uses installed PROJ operations with network disabled and no ballpark substitution. Vertical conversion is not silently performed during normalization. The product consumes the resulting exact admitted reference versions; running this tool does not approve rights or working purpose.

The historical Jeddah surface path remains an explicitly named regression fixture. Actor model generation was separated into `build_fixture_models()` and is invoked only by the historical simulation installer. The generic geographic entry point has no Jeddah constants, fixed UV indexing, actor generation, exercise partition, runtime start or replay dependency.

### T21: durable geographic packages

The product flow selects exactly cited geographic references, an optional undulation grid, optional qualified retained multispectral subsets, an explicit local origin and a qualification profile. The supported initial profile is a single normalized component within ±85° latitude, at most two degrees per side, with one complete admitted DEM mosaic, optional EGM2008 undulation grid and vector editions. Other CRS/vertical/grid configurations are rejected, rather than silently approximated. Multiple native DEM tiles can be normalized into that bounded mosaic before admission.

Before processing, source-backed horizontal/vertical uncertainty, independent control values/provenance, intended method/use, pixels/features/triangles/bytes, working-memory estimates, temporary-space bounds and maximum processing duration are recorded. Missing source accuracy, incomplete grid/area-of-use coverage, incompatible datum or impossible estimates roll back without consuming a full job/output reservation. Tolerances cannot claim accuracy tighter than admitted sources. EGM2008 conversion uses explicit `h = H + N`, qualified with independent controls; successful round trips alone are insufficient.

Core limits are 16 exact reference datasets, 131,072 total declared raster nodes/pixels, 10,000 features, 131,072 triangles, 8 MiB retained output, 128 MiB estimated working memory, and 120 seconds per attempt. Polygon/raster work has an additional fixed complexity ceiling. Current processing uses in-memory buffers and no temporary raster workspace; native-parent normalization has a separate 256 MiB retained-parent ceiling. These are bounds/estimates, not measured RSS, p95 or soak qualification.

The worker pins geospatial implementation/version, applies exact PostGIS vector intersection from frozen source bytes, and emits a neutral ZIP: georeferenced ENU/WGS84 terrain mesh, nodata/selection masks, clipped GeoJSON, explicitly qualified multispectral grids/masks, original normalized inputs, instructions and a provenance/qualification manifest. It records original/effective rights, dates, hashes, frame, vertical method, residuals, budgets and omissions. Multispectral source bytes, pixel-edge registration, acquisition dates, reflectance units and quality classes are preserved. Unknown imagery accuracy blocks that inclusion. Native raw GeoTIFF/OSM parents can remain external dependencies, identified by retained receipts; their bytes remain in the normalization directory. A visual GLB is optional and deliberately omitted from this profile.

Terrain cells/imagery pixels crossing holes or omitted boundaries are conservatively excluded. Nodata and source omissions are reported; no complete road/building coverage, inferred heights, cloud correction, collision/navigation suitability or engineering accuracy is asserted. The offline `check_portable_environment.py` reader uses a separate PROJ/GEOS implementation to verify coordinates, independent controls, masks, polygon containment, triangle indices and file hashes. It is a coordinate/geometry check, not live authorization, cryptographic authentication, observed survey or production qualification.

### Integrity, retention and operation

All delivery objects retain the composite all-source case ACL and original/current purpose-specific rights checks. Access, support and retention are checked before processing and again after private I/O before publication. Cancellation/stale lease and output publication commit atomically. A render attempt binds its candidate hash/private key, preventing changed bytes and orphan fan-out under one reservation. Upload plans/completions are retained before/after I/O; uncertain writes keep deletion pending until their bounded deadline. Explicit cleanup and idle author-bound workers reconcile expired committed and uncommitted objects, retain minimal tombstones, and retry failed deletion. Cleanup never restores expired inputs. Storage accounting includes frozen JSON manifest bytes and rendered-byte reservations.

Local supervised startup now builds/starts `emad-delivery` with `local-map-analyst.token`. Staging has an opt-in `case-delivery` profile using a separately supplied author credential. It confers no review permission. Apply the normal coordinated API/UI/worker upgrade; source bindings and rights still require source administration. Do not start actor/model/runtime workers to generate a geographic package.

### Verification and qualification boundary

Commands, results, source/image fingerprints and artifacts are in `evidence/map-first/t19-t21/verification.json`. Backend checks use isolated migrated PostGIS/private storage; browser tests use scoped protocol fixtures plus actual Go-rendered HTML. Geographic product-flow fixtures exercise Saudi and French coordinate frames through the same API, with separate PROJ/GEOS reader checks and qualified synthetic imagery. Normalization tests exercise multiple native tiles, rectangular grids, holes, preserved hashes and rejection paths.

The geographic grids and control values in this increment's automated tests are synthetic protocol/algorithm evidence, not real terrain acquisition or surveyed control. Real source-backed terrain accuracy, observed independent recipients/users, complete A01–A24 journeys, mixed/concurrent workloads, prolonged unattended operation and recovery rehearsals remain qualification work. No legacy Jeddah fixture or previous live imagery evidence is relabeled as satisfying those gates.

Migration **00053_delivery_jobs.sql** is required. The user reported migrations, without independent shared-schema verification. This work migrates only disposable test databases and does not deploy/restart the shared application. During verification, the pre-existing PostgreSQL container was found exited (255) while Garage had recently restarted; the underlying cause was not independently established; the same container was started and connection health checked. No shared migration was performed during that restoration.

T22–T24 and release qualification remain open. Full acceptance and observed-human gates retain their ledger state.

<a id="t22-t24"></a>

## T22–T24: operational qualification and release evidence

The qualification tooling and active documentation are implemented. **T22, T23 and T24 remain IN_PROGRESS; the release is NOT_QUALIFIED.** This report separates executable checks from qualification that still needs real elapsed time, installed inputs and appointed people. No new migration is introduced. The user reports applying the preceding migrations through 00053; shared installation was not independently inspected or restarted.

### T22: repeatable measurement and paired recovery

`scripts/ops/map_first_qualification_profile.json` freezes the original proposed MAP-6 dataset, concurrency, timing, payload, recovery, geographic/state and wall-clock targets. It also declares a reduced operational sample separately. These proposed targets are not silently promoted to agreed service guarantees. A run records its profile bytes/hash, baseline and changed-source hashes, hardware, duration, software version, dataset sizes and fixed synthetic provider budgets **before** measurement. Source changes invalidate that run's sample result.

`qualify_map_first.py sample --output <new-directory> --seconds 30` creates disposable PostGIS services and an assembled Go HTTP server. The sample seeds 1,000 explicitly acceptance-classified capture-history records across ten area labels, creates five separate canonical analyst identities and ten actually claimed retrieval profiles, and measures simultaneous scoped resolution/catalog reads plus scheduled retrieval. Ordinary catalogs still exclude the acceptance records; the authorized sample explicitly requests all-purpose discovery. No synthetic public facts are injected into the shared installation. Ten area labels on history records are not ten qualified geographic datasets.

The sample retains every request's bounded operation, duration, outcome, timeout, status, response bytes and correlation ID. It stores no bearer token, URL/query, case prose or provider-account key. Cold means the first request in each session; warm means subsequent requests, **not** an OS/database-cache purge. The summary reports request counts, errors/timeouts and their rates, p50/p95/p99 using nearest rank over all completed attempts, including failures/timeouts. It detects missing operations, incomplete runs, invalid durations and inconsistent failure totals. A slow failed request cannot disappear into success-only percentiles. The longer sample uses actual monotonic elapsed time; 86,400 seconds is not simulated.

Telemetry records application heap, goroutines, database size, delivery queue count/oldest age and time since the oldest successful retrieval. Retrieval age is not source observation freshness. These are useful diagnostic samples, not process/container RSS, queue-age/freshness or retained-object growth qualification. The reduced sample does not seed the proposed 100,000 reports, 50,000 detailed vector features or 10,000 weather/moving records, exercise four concurrent raster captures, or measure browser first useful map/control responsiveness. Earlier scale/browser evidence remains separately scoped. Increasing its duration alone cannot qualify those missing gates.

Production `/metrics` now exposes cumulative `c4isr_http_operation_duration_seconds` histograms, with fixed buckets and bounded method/operation/status-class labels. Errors remain in the distributions. Existing exact HTTP status/request counters remain available. Compute quantiles across **all** status classes for an operation; filtering to 2xx would hide failures. Resource identifiers, case titles and credentials cannot create labels. Existing aggregate metrics remain compatible. Histogram estimates and the sample's exact nearest-rank quantiles are distinct measurements.

The paired backup tool now writes v2 manifests with hashes for every ordinary public table, plus policy/grant/access-function/immutability-trigger definitions. This automatically includes new case revisions, dependencies, findings, canonical review identities, release/profile heads, admission mappings, intents, reservations, jobs, attempts and upload plans; it does not maintain an incomplete hand-picked family list. Dump/copy completion requires unchanged inventories before/after, and every current required artifact/composite key must have its exact recorded hash/size. Inventory files and raw keys remain private.

All writers must be stopped, including standalone delivery, connector, capture, imagery and retention processes. Backup requires explicit `--quiesced` (or `task local:backup QUIESCED=true`), a stopped supported supervisor and free API/UI ports. The tool never stops an application on the user's behalf. These checks support the coordinated procedure; they cannot prove the absence of arbitrary external writers. Maintenance must remain quiesced until completion. Failed backup directories lack a completion manifest.

Known expired/deleted/DELETE_PENDING/tombstoned object keys are omitted from new backups and from restoration, unless the same key is independently required by a still-permitted current object. Restore reevaluates retention at its current wall time and around each object write. Known expired capture/preparation inputs are reported as unavailable; analysis reproduction is explicitly NOT_RUN_POLICY_EXPIRED instead of restoring their bytes. Pending uploads remain optional; forward reconciliation retains their audit history and never restores expired bytes. The backup itself can contain private database/source history and must remain protected. Unknown historical object families keep their existing domain policy; no blanket deletion rule is inferred for them.

Restore compares every inventoried table and security definition **before** migrations/application startup, restores permitted private objects into isolated storage and hash-checks them, then runs the existing domain replay. Older v1 backups remain readable but explicitly lack expanded-inventory qualification. A new assembled integration test executes a real `pg_dump`/`pg_restore` into a new isolated database and a second private S3 server. It verifies the exact approved brief bundle, case identity, canonical access restrictions, original intent, job state and attempt history without reprovisioning grants. This synthetic populated-case regression does not qualify every installed geographic/environment/imagery dependency or an actual older rollback binary.

The first workload fixture mistakenly used 120-second source pacing while attempting ten schedules at one-second spacing; it retained nine failures. The corrected isolated profile explicitly declares one-second synthetic source pacing, a 10,000-request daily ceiling and a slower 12-second steady scheduler cadence. No pacing clock or spent allowance is reset during measurement. Initial recovery tests also caught a database-name-dependent grant fingerprint and an incorrect expected-size argument in the test's S3 read; both were corrected before final verification. Failed run logs remain distinct from the final evidence.

### T23: observed analyst/reviewer qualification

Use the [T23 observed analyst/reviewer session protocol](MAP_FIRST_T23_OBSERVED_SESSION_PROTOCOL.md), which carries the [T14 observed-session protocol](MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md) forward as its foundation and adds what T14 never contained: the frozen-brief, geographic-package, case-handoff and offline-recipient journeys; an explicit description of **every** entry of the `workflowChecks` matrix that the validator enforces, so an observer and the validator work to the same instrument; and the per-task attempt/assistance accounting that A19's exit measurement needs. T14 now points to the T23 protocol instead of diverging silently from it. The machine-readable [session template](MAP_FIRST_T23_OBSERVED_SESSIONS.template.json) starts NOT_RUN with no participants. Fill a separate record from actual consented observations; do not turn the template or automated fixtures into observed evidence.

Pin the record to the exact candidate fingerprint in the release ledger. Record baseline workflow/time, completion/time, assistance, usefulness, the participant's area/period and coverage/uncertainty explanation, corrections/retests and hashed redacted evidence. Include analyst, independent reviewer, source administrator and restricted reader; the reviewer must be a different canonical human from all case contributors. Cover the geographic, selection-form and evidence-state matrix in the qualification profile, including the real regional/international imagery, new geographic package, handoff and offline recipient tasks. Record keyboard, 320/390px portrait, short landscape, 200% text and an actual assistive-technology observation. Missing provider capability remains a recorded gap rather than an invented successful journey.

`qualify_map_first.py sessions <record.json> --release-fingerprint <hash> --output <assessment.json>` checks record completeness, timezones/durations, consent, canonical contributor separation, the participant sample (at least three target analysts and one independent reviewer, the reviewer distinct from every case contributor and from every observed analyst), all required roles/matrix entries, per-task attempt and assistance counts, exact local evidence hashes and the reviewer-independence and assistive-technology assertions. A missing/synthetic session fails. A session whose journey outcome is FAIL or BLOCKED is **recorded and reported, not rejected**: the assessment counts outcomes, lists the unresolved journeys, measures the unassisted rate of assigned core task attempts against A19's 80% threshold and reports each A19 exit condition (no wrong-area or wrong-period outcome, coverage-versus-absence and result/review-state explained, every critical failed journey repaired and retested) with its observed value. The assessment always carries `qualification: NOT_QUALIFIED` and `a19Status: NOT_RUN`: this validates records and measures a reported exit metric, it cannot establish that a human observation truly occurred and it never awards qualification. The appointed observer and evidence reviewer remain responsible for their attestations. No actual participants were observed in this increment; A19/A20 remain NOT_RUN.

### T24: active documentation and release traceability

README, PRODUCT, the product brief and operations now point to the complete implemented feature sequence and qualification boundaries. The active installation requires coordinated API/UI/public workers through migration 00053. Map-only startup and signed longitude-first/Arabic digit/time semantics remain documented; geographic selection, installed source/profile coverage and qualified detailed geography remain distinct. Older exercise instructions remain visibly historical; simulation design does not resume.

`qualify_map_first.py ledger --output <ledger.json>` assembles all 24 tasks, 18 gaps, mandatory acceptance and 12 hardening checks from the governed backlog, preserving implementation/installation/functional/geographic/user evidence separately. It pins the candidate source/profile identity and lists every unclosed task/scenario. It is deliberately unable to award release qualification; closure requires named owners/reviewers, all P0 and mandatory acceptance and the governing plan's evidence rules. A new source change requires a new candidate and relevant retests/session evidence.

### Verification and remaining closure

Exact commands/results/source fingerprints are in [verification.json](evidence/map-first/t22-t24/verification.json). The [release ledger](evidence/map-first/t22-t24-review/release-ledger.json) is the reviewable handoff, not a delivered-phase claim. No shared migration, deployment, backup, retention action or application restart was executed by this increment.

The later [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation) is the most recent qualification result and supersedes the qualification view in this record: it measured failed dense-workload budgets, an unresolved read-capacity blocker and no observed sessions. Both ledgers and this record's evidence are pinned to older source fingerprints; regenerate the ledger against a clean revision before citing it as release evidence.

Outstanding closure: freeze an appointed release/installation/profile with owners/reviewers; run the complete declared mixed dataset and browser budgets; run 24 real wall-clock hours with process/worker restart and queue/freshness/RSS/object-growth telemetry; rehearse all listed recovery faults, full populated dependency replay and the actual nominated rollback binary/forward recovery; acquire/qualify the required real geographic journeys; observe independent target users and resolve/retest issues. The existing A01–A24 and P0 gates must be adjudicated with integrated evidence before the phase can be called delivered.


### Pre-qualification review

The subsequent [focused review](#t22-t24-review) found and fixed source-pinning, sample/budget validation, legacy-retention, object-count, observed-workflow and interrupted-run defects. Use its new candidate/evidence for future qualification; this increment's original evidence remains historical. Standalone summaries now require the original sibling `frozen-input.json`, and all restores derive current retention policy even when the backup lacks a persisted inventory. Observed records now include the explicit `workflowChecks` profile/template field. Task/release qualification states remain unchanged.

<a id="t22-t24-review"></a>

## T22–T24 pre-qualification review

> **Status:** Historical pre-qualification review of the qualification tooling, 27 October 2026. Its candidate ledger is superseded for future qualification; the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation) is the latest qualification result. See the [records index](README.md).

Reviewed the qualification runner, recovery inventory/backup/restore paths, observed-session validator, HTTP operation metrics, relevant tests and release claims. This is a focused correctness/regression review of T22–T24, not an exhaustive repository audit or completion of the operational/user gates.

Seven issues were found and fixed before qualification:

1. **P1 — Candidate fingerprints omitted staged changes and deletions.** The earlier inventory read index-relative modified/untracked paths. A staged-only edit could therefore be absent even though HEAD did not describe its bytes. Source capture now uses all changes relative to HEAD plus untracked paths, with explicit deletion records and NUL-delimited paths. A regression checks that staged bytes alter the fingerprint and deleted paths remain represented.
2. **P2 — Incomplete/contradictory samples could receive a sample PASS.** The summarizer required an end marker but did not verify the advertised duration or reject successful timeout flags. It now checks finite positive elapsed/advertised duration, frozen requested duration, strict outcomes, profile-first/completion-last ordering and consistent timeout/status flags. The release remains NOT_QUALIFIED regardless of a sample result.
3. **P2 — Standalone summaries used the current budget file.** Reassessing an old sample after changing targets could silently change its outcome. Standalone summaries now require the original sibling `frozen-input.json`, use its budgets/duration and record the canonical budget content hash. The runner checks its own frozen requested duration too. A regression loosens the current file and confirms that the original target still fails.
4. **P1 — Legacy v1 restores skipped retention policy.** v1 archives lacked a persisted inventory, so the previous restore path copied their objects without the new expiry/tombstone policy. Every restore now derives an inventory from the isolated restored database. v2 also compares its persisted fingerprint; v1 enforces current retention while remaining explicitly unqualified for expanded historical inventory comparison.
5. **P2 — Restored-object counts could become negative.** The excluded-key set can contain expired keys already absent from a new backup. Subtracting that set's size from the backup object count was incorrect. Counts/bytes now use actual manifest entries, and excluded-key counts are reported separately from objects actually omitted.
6. **P2 — Observed records lacked explicit real-workflow/accessibility coverage.** Geography/state labels alone could pass record completeness without the required imagery, geographic-package, handoff, offline brief and accessibility tasks. The profile/template/validator now require those workflow entries, a full candidate fingerprint and consistent participant-to-canonical-human mapping. This remains record validation, not proof that observations actually happened; no real sessions were performed.
7. **P2 — Interrupted/timed-out workload runners could lose their final failure result and leave their test process running.** The runner now owns a process group/tree, stops only that invocation on timeout/interruption, and records a failing result even when samples are incomplete/malformed. The timeout regression runs and stops a real owned process. Docker Testcontainers remain responsible for their disposable-container lifecycle; no shared service is selected for shutdown.

Files changed: `scripts/ops/qualify_map_first.py`, `recovery_inventory.py`, `paired_backup.py`, their qualification tests, the proposed qualification profile and observed-session template. No Go runtime/API behavior, migration, shared installation or source rights were changed in this review.

### Verification and handoff

Exact source identity, commands and results are in [review verification](evidence/map-first/t22-t24-review/verification.json), with the revised [candidate ledger](evidence/map-first/t22-t24-review/release-ledger.json). Earlier T22–T24 evidence is preserved as historical; its source fingerprint is superseded for future qualification by this review's candidate. Prior tests remain evidence of their own recorded code, not fresh acceptance of this candidate.

The focused checks cover regression/negative cases, a fresh five-identity/ten-profile reduced workload, and a real isolated database/private-object restore with the original brief approval/intent/access restrictions. The restore test exercises the common inventory and assembled product; it does not qualify the complete installed backup CLI, every populated geographic/environment dependency, all recovery faults or the nominated older binary. Those remain explicit T22 gates.

No further concrete correctness issue was identified in the reviewed paths after these fixes and scoped checks. T22–T24 remain IN_PROGRESS and the release remains NOT_QUALIFIED. Full workload/browser budgets, 24 actual hours with planned restart and growth/freshness telemetry, complete real geographic journeys and observed independent users are still required.


At the final handoff check, the Docker API was unavailable. Current shared-container health could not be verified; Docker must be available before repeating isolated qualification. The earlier successful test logs are retained. The cause was not established and no shared infrastructure was restarted by this review.


The subsequent final recheck found Docker available, `c4isr-postgres` running and healthy, and `emad-garage` running. The earlier unavailable result is retained separately; no infrastructure restart was performed by this review. See `docker-handoff-final.json` in the review evidence. Shared application deployment/source-profile verification remains a qualification gate.

<a id="access-fix"></a>

## Local access failure correction (29 October 2026)

> **Status:** Active correction record for automatic local analyst entry, applied before the product evaluation on the same date. See the [records index](README.md).

Earlier evaluation-server checks did not establish access through the user's regular startup. At this correction, neither the temporary UI on port 5198 nor its API on port 8097 was listening. A retained browser tab could therefore display `Failed to fetch`.

The running regular UI on `localhost:5173` also used the legacy `dev-operator-token`, which the running database-authenticated API rejected with HTTP 401. The maintained `local-map-analyst` credential authenticated successfully against that same API. Both failures were reproduced before changing the regular entry.

### Corrections

- Development-only Vite middleware supplies the maintained local identity through a non-cached, same-origin `/config.js`. Bare development startup and the legacy default use this entry. Explicit token configurations, deliberately empty test credentials and production builds retain their own authentication behavior. Credential delivery checks loopback host and peer addresses.
- `task dev:web` declares automatic local entry unless an explicit account token is configured. The full supervisor preserves its supplied profile identity.
- `scripts/ops/local_web_entry.mjs` connects the bookmarked port 5198 to the regular UI and its existing API routing. It has no evaluation timeout and launches no additional API. `task local:web:entry` starts it for an already running UI; `task dev` includes it when the port is free. Normal application processes must remain running; this is not an operating-system startup service.
- Local connection failures now explain that the API is unreachable and offer Reconnect. Account switching remains optional; a network failure no longer presents a mandatory credential form in automatic local mode.

### Verification

Installed Chrome and Brave each opened `http://127.0.0.1:5198/map`, received HTTP 200 for the current identity, and opened Investigations without a credential form. A deliberately interrupted identity request reproduced a failed fetch; Reconnect restored the workspace after removing the interruption. Production TypeScript/build checks passed, as did four applicable existing product-entry checks; four duplicate project cases are intentionally skipped.

Private screenshots and redacted observations: `private/qualification/installed-evaluation-2026-10-29/current-access-verification.json`, `current-access-chrome.png` and `current-access-brave.png`. These evaluator-operated checks do not prove an independent user's completed session. Read-capacity and dense-workload issues remain separate from this access correction.
