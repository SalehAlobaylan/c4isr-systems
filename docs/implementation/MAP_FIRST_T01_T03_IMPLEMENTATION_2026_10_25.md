# T01–T03 implementation and verification

**Prepared:** 25 October 2026. **Handoff updated:** 26 October 2026 (Asia/Riyadh).  
**Authorization:** Implement the first three tasks of the [map-first plan](MAP_FIRST_IMPLEMENTATION_PLAN_2026_10_25.md).  
**Source:** Working tree based on `a6d1d22f34692faea31db58e4a6318c539c3321a`; no commit or deployment is implied.  
**State:** T01–T03 are implemented with the scoped checks below. The [backlog](MAP_FIRST_BACKLOG_2026_10_25.json) retains separate implementation, installation, functional, geographic and user evidence. The full phase remains unqualified.

## T01: Area integrity

[Operation-area validation](../../apps/operator-ui/src/features/workspace/operationArea.ts) now receives exact selected/pinned geometry from the map/workbench. [Export](../../apps/operator-ui/src/features/imagery/ContextExport.tsx) no longer reads a separate localStorage investigation or supplies Jeddah. [Capture](../../apps/operator-ui/src/features/imagery/CaptureWorkbench.tsx) starts with a valid selected local rectangle or empty required coordinates. Systems no longer supplies a fixture when its selected scope is missing.

- A rectangular operation accepts an explicit rectangle faithfully representable from the current selection. Irregular/multipart/invalid/unresolved selections demand a local rectangle; their bbox is not substituted as analytical geometry.
- Export enforces its existing 400-square-degree bound and capture its existing 0.1-degree-per-side bound. A manual subset must stay inside its selected rectangle; existing server scene/admission/rights checks remain in force.
- Switching to the current map area clears stale capture scene/pair choices, incompatible comparison choices and export previews/selections. Pending preview responses cannot populate a different area. Minimized workbenches retain visibly pinned drafts until explicitly changed; frozen jobs/packages retain their original inputs.
- A named area's asynchronous resolution completes that same pinned scope. Invalid shared geometry cannot borrow a saved rectangle. Unsupported operations expose a map-area selection action.
- Capture/history and prepared comparison choices use the selected footprint; intentionally inspected exact completed records keep their original lineage.

The full versioned geometry/time/case contracts remain T04–T07. This containment increment deliberately requires a supported local rectangle for the existing rectangular operations.

## T02: Map-only operation

Environment-loaded configuration defaults to `C4ISR_PRODUCT_PROFILE=map`. The local lifecycle, staging composition and image build support `map` and explicit `historical-full` profiles. Map setup provisions `local-map-analyst` with Public Context permissions and a separate `local-source-admin`; it does not require development exercise/reviewer identities or simulation worker credentials. Existing principals/grants/history are preserved. Historical provisioning remains explicit.

The map supervisor starts the API, UI, imagery/preparation and configured public connectors. It does not describe or launch a runtime or start asset/recording workers. The API does not initialize scenario engines or the exercise freshness monitor in map mode, uses wall time, rejects exercise resource selection, and exposes Public Context navigation resources. Environment-loaded servers require the forward schema, including the new purpose table.

Home, header and workbench/environment choosers use the deployment profile. Map entry ignores saved exercise defaults and does not query simulation onboarding. Explicit historical routes remain available under the historical profile and their original grants. A basemap outage leaves selection geometry and evidence/export controls usable with a visible map-source notice.

The packaging checker and paired-restore script are profile-aware. Map restore does not require simulation receipts; old backups without a recorded profile retain historical interpretation. An installed imagery receipt still triggers reproduction checks. Full expanded recovery/soak qualification remains T22.

## T03: Purpose and truthful counts

[Migration 00045](../../db/migrations/00045_content_purpose.sql) adds append-only purpose revisions with scope/source RLS, actor, reason, exact entity identity, receipt reference and expected-version conflict handling. Original source/review bytes are not rewritten. [Content-purpose administration](../../internal/contentpurpose/service.go) requires source management, the underlying catalog's read permission, and current source access; classification requires a human principal.

Ordinary reference/scene/capture/job/assessment/package catalogs and relevant map layers exclude classified development/acceptance content. Exact authorized history stays available, with support catalog views behind administration permissions. Unclassified records remain explicit; they cannot inflate reviewed working-publication totals. Metadata is bound transaction-locally, preserving pooled-connection isolation.

Counts distinguish retained reference versions, catalog revisions, successful capture records, distinct acquisition timestamps, distinct completed comparisons by frozen inputs/method, and published working assessments with canonical human separation. Systems labels bounded client subsets as **shown** counts. Provider quality, real/synthetic provenance, content purpose and human review remain separate concepts. A development/acceptance assessment cannot be relabeled into a working publication; a successor needs its own permitted review.

[Receipt backfill](../../scripts/data/classify_content.py) verifies original reference hashes and capture/preparation identity links, uses exact receipt/evaluation IDs, rejects conflicting later classifications, and reports unavailable IDs. Its default is a reviewable preview; successful subsequent setup applies the explicit inventory through the upgraded API. The [seed inventory](MAP_FIRST_CONTENT_PURPOSE_INVENTORY_2026_10_25.json) identifies two dated development assessments. Remaining unknown records require source-administrator review; no names or conclusion phrases are used to guess their purpose.

The active [product brief](MAP_FIRST_PRODUCT_BRIEF.md), README, PRODUCT and [operations](MAP_FIRST_OPERATIONS.md) now point to the current map-first scope and backlog.

## Verification performed

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

## Browser evidence

These screenshots use isolated mocked catalog/geometry responses, not a qualification of installed Saudi or Paris coverage:

- [Map-first Home](evidence/map-first/t01-t03/map-entry.png).
- [Capture draft after explicitly selecting the country scope](evidence/map-first/t01-t03/capture-scope-cleared.png). The synthetic country fixture is a rectangle solely to exercise the handoff/size gate; it does not represent an authoritative national boundary.

## Installation and remaining qualification

The shared running installation was not stopped, restarted, migrated or backfilled during this code pass. Apply migration/profile/identity changes with the [normal setup and upgrade procedure](MAP_FIRST_OPERATIONS.md), using the installation's coordinated backup/maintenance process. Isolated startup/migration evidence does not establish that deployment occurred in the shared installation.

T01–T03 are **IMPLEMENTED**, rather than fully closed outcome gates. Full scenario A01 also includes future case save/resume; A02 eventually spans case/review/brief contracts; A14's installed inventory and final release evidence remain separately tracked. No gap is declared closed by this batch. T04 and later tasks remain planned. Regional coverage, full recovery/soak and independent usefulness retain their later qualification gates.
