# T04–T06 implementation and verification

**Prepared:** 26 October 2026 (Asia/Riyadh).  
**Authorization:** Implement the next three tasks after T01–T03 in the [map-first plan](MAP_FIRST_IMPLEMENTATION_PLAN_2026_10_25.md). The user reports applying the preceding migrations.  
**Source:** Uncommitted working tree based on `a6d1d22f34692faea31db58e4a6318c539c3321a`.  
**State:** T04–T06 are implemented with the checks below. T07–T24 remain planned; the phase and full acceptance scenarios remain unqualified.

## T04: Versioned areas and exact geography

The new [area domain](../../internal/areas/service.go) owns transient selections and immutable persisted area revisions. Revisions retain the original request geometry, canonical geometry bytes and hash, source-qualified edition/release identity, parent revision, per-component bounds/geodesic area and display capability. Resolution is read-only. Capture/comparison submission and approved export persist the selected area and evaluated period in the same transaction as their durable work.

- Exact geographic IDs remain readable after newer editions are staged or activated. Discovery uses a per-source atomic release head; a release admits a complete, validated edition hierarchy. Duplicate logical keys, missing parents, hierarchy cycles and conflicting selectors fail explicitly. Legacy catalogs without releases remain readable.
- Immutable region profiles pin released reference features or explicitly named study areas. Their hashes preserve exact decimal values. Numeric precision above JavaScript's safe-integer range is retained in server fingerprints; old unscoped context-package identity rules remain compatible.
- World is a discovery scope. A place point is a navigation target. Neither becomes an invented local operation rectangle. Panning changes the camera; **Use this view** deliberately creates a viewport area.
- Date-line crossings are unwrapped, validated and split into bounded Polygon/MultiPolygon components. Holes and islands retain their meaning. Normalization uses [PostGIS ST_WrapX](https://postgis.net/docs/ST_WrapX.html) and [ST_Normalize](https://postgis.net/docs/ST_Normalize.html); a topology round-trip check rejects an unrepresentable serialization. Original geometry bytes remain separate from canonical/display geometry.
- Exact geometry reaches map queries, catalogs and local-subset validation. Component bounds support navigation; they do not replace an analytical polygon. Server `ST_Covers` checks reject processing across holes or outside the selected area. The existing capture/export size limits still apply.
- Geometry beyond ±85° remains retained and queryable, with an explicit map-display restriction. Component navigation avoids fitting a date-line selection to a near-worldwide extent. This does not add a polar renderer.

[Migration 00046](../../db/migrations/00046_analyst_scope.sql) adds release manifests/heads, immutable areas/profiles/scopes and temporary read manifests. Current resource/source RLS also protects scope provenance embedded in captures, jobs, assessment evidence and context packages. Losing the reference-geography grant cannot expose that source through an otherwise readable imagery job. Existing queue budgets still count jobs whose nested provenance is hidden from the caller.

## T05: Shared time and stable reads

[TemporalScope](../../internal/platform/queryscope/temporal.go) provides monitoring presets, fixed historical intervals, timezone, revision interpretation and explicit `[from,to)` boundaries. One server evaluation timestamp resolves every family in a query batch. Jobs and exports receive a fixed evaluated interval; a bookmarked monitoring preset may continue rolling. Browser display includes year, timezone and offset, with UTC available.

The new contract preserves `known_at` and `effective_at` as distinct event-history interpretations. Families without historical reconstruction return typed support metadata and an explicit unsupported state. Acquisition time filters imagery/capture catalogs. Monitoring background imagery is opt-in and labeled outside the selected period. Weather validity/current-feed state stays separate from observation time. Legacy direct endpoints retain their original inclusive-end behavior; imported bounded legacy periods advance to the next representable PostgreSQL microsecond through an explicit adapter, preserving records at the former inclusive endpoint.

[Read snapshots](../../internal/analystscope/reads.go) materialize a bounded result under one repeatable-read transaction, including stable membership/order, exact event revisions, family summaries and retained connector/admission receipts. Pages use stored results rather than an open transaction or a timestamp cutoff. A publication transaction that starts before a read and commits afterward cannot change that read's later pages. Later corrections and withdrawals appear in a new read.

Temporary manifests expire after five minutes. Each principal/resource can hold at most 16 manifests, each at most 8 MiB, with a 64 MiB combined budget. Exact event materialization is bounded at 5,000 rows; larger results require narrowing or explicit aggregates. Expiry creation, cleanup, page access and citation checks use the database clock consistently. Every page rechecks the owner, resource and current authorization revision. Expired/changed-access cursors require an explicit restart. Saving a cited result copies the bounded manifest into durable immutable scope evidence before expiry; subsequent cursor cleanup does not delete that evidence.

These receipts describe retained connector/admission state. They do not establish complete provider coverage or factual absence. Full workload, soak, recovery and all-family qualification remain later gates.

## T06: One scope through the user journey

[AnalystScopeProvider](../../apps/operator-ui/src/features/workspace/AnalystScopeProvider.tsx) owns the active map area and period. Systems, imagery discovery, capture, comparison, review and export use it or an explicitly pinned workbench snapshot. Operational reads of the old `investigation` browser preference are removed.

Pinned drafts visibly retain their area and period. **Use current map scope** explains and clears affected scene/pair choices and export previews before adopting both. Completed jobs and packages retain their submitted scope. Hidden retained workbenches suspend ordinary polling. Existing context packages now preserve their approved durable scope through preview, approval and offline bundle round-trip.

URLs, bookmarks and browser history carry the exact geographic identity or original drawn geometry and temporal selection. Conflicting, malformed, unavailable or inaccessible saved selections open recovery with no operational fallback. **Start a new view** is deliberate. Bookmarks remain view presets; durable investigations are T07.

Requests include scope, authorization and relevant source/temporal filters in their cache/query identity. Catalogs echo the resolved scope hash and evaluation timestamp; mismatched responses cannot populate a draft. Cancellation reaches geography, map queries, catalogs, detail/history reads and artifact previews. Late responses cannot replace a newer selection. Existing identity/resource cleanup also handles denied scoped reads by clearing protected caches/previews.

The [OpenAPI scope contract](../../api/openapi/analyst-scope.yaml) and regenerated TypeScript client describe area/scope resolution, durable freezing, read snapshots/pages, release/profile operations and optional scope fields on existing work. Actual routes are under `/api/v1/areas`, `/api/v1/analyst-scope`, `/api/v1/geography/releases` and `/api/v1/geography/profiles`.

## Verification performed

- `go test ./...`, `go vet ./...` and Go formatting checks pass. Default Go integration tests skip Docker scenarios unless explicitly enabled.
- UI typecheck and production build pass. Regenerating the OpenAPI client produces identical bytes. The existing Vite large-chunk advisory remains.
- **15 new browser cases** cover country, subdivision, named study, drawn polygon/hole, explicit viewport and date-line-component handoffs through Systems → imagery → review/export; period adoption; bookmarks/back/forward; invalid-link recovery; hidden polling; hash mismatch rejection; high-latitude display; denial of scope resolution/read refresh clears protected work until reconnect; and cancellation of a delayed obsolete response.
- **15 prior browser cases** cover the first batch, product entry and navigation. Browser APIs are isolated protocol fixtures; they do not prove PostGIS topology or installed coverage.
- **Three new real PostGIS cases** verify exact editions/releases/parents and original bytes, invalid geometry/subsets and holes, date-line event counts, immutable profiles/areas and source isolation; exclusive time boundaries and legacy compatibility; actual late-commit publication/outbox failure, corrections/withdrawals, stable pages, expiry/promotion, authorization changes and storage slots; frozen native-decoded capture inputs, time/containment failures, atomic writes and hidden-provenance queue limits.
- **Seven prior assembled integration cases** pass: map provisioning/Linux startup; purpose RLS/count/history; geographic history/aggregates/isolation; imagery worker reproducibility/independent-identity checks and scoped S3 export/offline round-trip; public-event correction/spatial semantics; native FIRMS/USGS bitemporal ingestion; and reference/terrain bytes/isolation. The Linux map image also resolves a real authenticated monitoring scope using `Asia/Riyadh` without a runtime descriptor.
- The map Linux image builds successfully. Python capture-worker syntax and four prior classification/backfill tests pass using the existing recovery environment. System Python lacks the optional geospatial dependencies; it is not the supported backfill environment.
- The planning harness and its 12 injected-fault checks pass. This establishes ledger consistency, not full application acceptance.

The [verification record](evidence/map-first/t04-t06/verification.json) records commands, test identities, source hashes, image identity and limitations. A [bookmark screenshot](evidence/map-first/t04-t06/bookmarked-scope.png) shows an isolated synthetic geographic selection and fixed historical period; it is presentation evidence only.

## Installation and remaining qualification

The user's preceding migration completion is recorded as reported installation progress. This pass did not inspect, migrate, restart or deploy the shared installation. **The new `00046_analyst_scope.sql` migration is required before running the updated API/workers/UI.** Follow the [normal setup/upgrade procedure](MAP_FIRST_OPERATIONS.md), including the paired backup and coordinated maintenance process. The API readiness schema check now requires the new scope tables. Existing source grants must cover every source in a submitted scope, including geographic-reference sources needed by workers.

Additive schema and immutable evidence remain on rollback; use a forward fix or the established recovery procedure rather than destructive down migration. Previous scope-unaware workers must stay disabled for scope-bearing work.

T04–T06 are **IMPLEMENTED** with scoped evidence. Full A02/A03/A04/A08/A09/A22 scenarios remain `NOT_RUN` with partial assertions because they also span later case, reference-depth, scale and release qualification. No geographic gap, coverage matrix or independent-human usefulness gate is declared closed. T07 investigations, T08 regional installation and all subsequent tasks retain their planned scope.
