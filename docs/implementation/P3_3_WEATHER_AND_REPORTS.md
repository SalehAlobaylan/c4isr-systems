# P3.3 - Station weather and selected official reports

Delivery date: 2026-10-21. Implementation and acceptance evidence for the third P3 task. P3.1-P3.2 and migrations 31-32 are preserved. The next task is **S1.1 - Model and environment asset registry**. The broader P3 live-provider exit gate remains open for the previously documented OpenSky/AISStream activation prerequisites.

## Pilot question and product scope

The bounded pilot asks: **Which official earthquake bulletins and station weather contextualize existing USGS/FIRMS evidence in the selected region?** Each enabled source admission must record its own explicit analyst question, selected AOI, provider/product identity, reviewed rights, freshness policy and service identity.

The selected providers are NOAA Aviation Weather Center METAR station reports and GDACS structured earthquake bulletins. The official-report option in P3.3 is implemented; there is no generic news collector or GDELT/UCDP deployment. A future humanitarian/media question needs its own bounded admission and adapter evaluation.

Weather samples and report claims remain useful with simulation disabled. Neither becomes a controllable asset, moving track, classification, impact assertion, synthetic observation or simulator truth. Context export is unchanged: these report/weather records do not enter the imagery-only approved export path automatically.

## Implemented behavior

- Native fixed-endpoint adapters reuse human-reviewed admissions, source-bound service identities, safe HTTPS transport, request/byte quotas, timeouts, durable leases/backoff, exact original bytes, content revisions, source provenance and projection retries. No scheduler starts in the API.
- Every retrieval restarts a bounded current query window. Native snapshots are discontinuous; a successful selected query does not imply complete reception or absence of hazards. Empty METAR HTTP 204 is valid. Malformed/null JSON and over-budget results fail explicitly.
- METAR JSON uses the provider's latitude/longitude bbox order, an at-most-six-hour lookback and at most 400 returned records. Only METAR/SPECI samples are selected. The raw METAR and station point survive normalization. Celsius, hPa, statute miles and explicitly converted knots-to-m/s retain their meanings. Zero remains a measurement; missing values remain null; variable wind has no fixed bearing; visibility with `+` is a lower bound. No route/area interpolation or forecast is inferred.
- The GDACS pilot requests EQ bulletins only, at most seven days and one page of at most 100 records. The returned cited event centroid/interval selects the approved AOI/time window locally. This does not claim coverage of a disaster's footprint. An exactly full page is **PARTIAL / provider_result_limit**, with no claim of catalogue completeness. The next attempt queries a fresh bounded window rather than resuming unstable page ordering.
- GDACS structured bulletin features are the retained source documents. Their original feature bytes include the provider's fields and links. The linked HTML bulletin/attachments are not fetched or archived; the external link can change. Source claims, provider alert level, cited producer/identifier, original time strings, location basis, extraction version and uncertainty are exposed separately. Timezone-free timestamps are interpreted as UTC for selection, with that assumption explicitly shown.
- Immutable context projections use same-source/resource foreign keys and forced scope/source RLS. PostGIS owns read-time spatial selection. Current station samples are chosen by observation time; current report revisions by provider modification time, then receipt time and deterministic evidence ID. Newest revision selection precedes AOI filtering, so an older location cannot reappear after a correction moves outside the view. Historical evidence remains addressable by exact ID.
- GDACS event IDs identify copies/revisions of the same bulletin lineage. Known NEIC/USGS citations share an upstream-source key. Other cited source names are retained; absent event identifiers leave event-level independence unresolved. These are lineage indicators, not independent-source counts or inferred cross-provider event matches. Repeated copies never increase an automatic confidence/corroboration score.
- Report interpretations reuse the existing assessment revision, audit, optimistic-concurrency and independent-human review workflow. Each proposal freezes an exact retained evidence revision, its raw/content hashes, extracted claims and lineage. OPERATOR/ALGORITHM/AI methods and optional AI scores are explicit. Publication requires a different human, a separate analyst confidence/rationale and valid original/current retained-analysis rights. Corrections and later drafts preserve the prior published revision. Weather observations cannot be proposed as source reports.

## Operator view and contracts

Open **Public Context -> Weather and source reports** from the public map. `/weather-reports` provides URL-backed AOI/kind filters, exact evidence selection, retained sample/revision history, original downloads, claim/lineage inspection, proposal creation, draft revision and independent review. Age of the cited observation/event and source-capture health are shown separately. Evidence selection remains pinned when a provider revision changes.

React owns one bounded DOM list (100 current records), one inspector and review controls; no new chart/map renderer or heavy visualization dependency is introduced. Mobile uses the same reading order with wrapping cards and ordinary scrolling. Values and caveats are visible without hover; controls are labeled and keyboard accessible. Loading, empty, partial/failed capture, stale sample, missing permission and failed mutation states remain explicit. Raw provenance/review documents are collapsed on demand. Workspace-scoped queries and canceled callbacks preserve the existing isolation behavior.

New endpoints:

- `GET /api/v1/public-context/records?kind=WEATHER|REPORT&bbox=west,south,east,north`
- `GET /api/v1/public-context/records/{evidenceId}`
- `GET /api/v1/public-context/records/{evidenceId}/history`
- `GET /api/v1/assessments/reports`
- `POST /api/v1/assessments/report-proposals`

Existing assessment revision/review/submit endpoints accept report proposals through the same workflow. Imagery proposal/list behavior remains compatible and reports are excluded from the imagery list. OpenAPI and generated TypeScript include typed station/report records, admission questions and report evidence snapshots.

Reads require `sources.read`; assessment reads/authorship/review use `assessments.read`, `assessments.create`, `assessments.review`. Execution remains `connectors.execute` for a source-bound service principal. Human source admission remains `sources.manage`. All new operations require PUBLIC_EVIDENCE and enforce resource/source visibility.

## Rollout

1. Apply **migration 33** after 32: `task db:migrate`. Startup explicitly checks the new projection table. Migrations 1-32 are unchanged; no developer database migration was run by this task. Recovery is forward-only because evidence/review history is immutable.
2. Register a separate external source for METAR and GDACS in the intended Public Context resource. Replace the source IDs in `scripts/data/metar-principal.example.json`, `gdacs-principal.example.json` and `weather-report-connectors.example.json` with the actual registered IDs. Provision the service principals using the existing `apps/emad-admin` workflow.
3. Merge the locations into the server-owned `C4ISR_PUBLIC_CONNECTORS_FILE` manifest. Both endpoints are fixed HTTPS services without credentials. The generic HTTP transport sends an EMAD User-Agent, permits no redirects/proxy/private-network targets, bounds response size and sanitizes retrieval failures.
4. Review the disabled `metar-admission.example.json` and `gdacs-admission.example.json` templates. Record the relevant original producers, upstream rights/attribution and retained-analysis permissions. Change placeholder rights IDs/terms and enable only after admission review. NWS public-domain content does not automatically license every third-party station contribution. GDACS automatic alerts require their source/terms review and are not expert-validated facts.
5. Both pilot admissions require retained originals and analysis rights, one page, at most 1 MiB/response, at most 96 requests/day and a meaningful analyst question. Minimum pacing is 300 seconds for METAR and 900 seconds for GDACS; examples use 900 seconds for both. General bounded AOI/timeout constraints still apply. Quotas are source/location-bound. Admission changes interrupt active leases as before.
6. Run the existing `apps/emad-connectors` executable with its source, scope, token-file and optional `-once` flags. Disabled admissions perform no collection. There is no automatic live activation or permanent credential written by this delivery.
7. Each native source stops at the existing archive budget of 10,000 evidence records or 64 MiB. Publication replay handles a full native batch independently of retrieval. Retention is append-only; admit only archive-compatible rights. A purge/retention migration or changed terms requires a separately documented design. These limits intentionally support bounded pilots, not continuous global archives.

## Validation

Acceptance uses explicitly synthetic native-format station and bulletin records, disposable PostGIS, the assembled Go API and the production UI. No permanent live feed is enabled and no live weather/hazard condition is asserted by fixture results.

- Native adapter tests: station units, nulls versus zero, variable wind, censored visibility, timestamp/position selection, malformed/unsupported samples, report cited times and shared lineage, full-page partial results, exact fixed registry endpoints, provider-specific request parameters and METAR 204.
- Database/API acceptance: deduplication, corrected samples, immutable histories, report evidence pinning, independent review, concurrency conflicts, missing publication confidence, revoked rights, published pointers surviving revisions, source/workspace/partition isolation and zero generated tracks/assets/observations/classifications.
- Real production-UI browser gate: **7 workflows passed**, including weather/report downloads, pinned URL/reload, explicit uncertainty, a second human publishing a report assessment, desktop/mobile captures and all existing imagery/moving/workspace workflows. Screenshots: `tmp/p3-3/weather-desktop.png`, `report-review-desktop.png`, `report-mobile.png`.
- Full PostGIS regression: **74 cases passed** (50 top-level tests and 24 subcases); 4 opt-in skips: the 3 live provider smokes and browser gate, which was run separately. Log: `tmp/p3-3/integration-final.jsonl`.
- Existing mocked browser suite: **74 passed / 50 intentional project skips**. Log: `tmp/p3-3/browser-mocked.log`.
- `go test ./...`, `go vet ./...`, Linux `go test -race` for affected ingestion/review/access/export packages, UI typecheck/production build, OpenAPI lint, SQLC/TypeScript regeneration stability, UTF-8/JSON/Go formatting and `git diff --check` passed. Build retains its existing large-bundle advisory.
- After final blank-question validation and review-refresh/UTC display adjustments, the two P3.3 API acceptance tests and all 7 real browser workflows were rerun. Log: `tmp/p3-3/final-targeted-browser.log`. A final API-only pass also verifies that report payloads are rejected by the imagery-only proposal route (`tmp/p3-3/final-api.log`).

## Provider references

Provider documentation was checked during implementation; these references describe protocol/terms, not proof of deployment authorization:

- [NOAA AWC Data API](https://aviationweather.gov/data/api/) and [official OpenAPI](https://aviationweather.gov/data/schema/openapi.yaml).
- [NWS use and disclaimer](https://www.weather.gov/disclaimer).
- [GDACS official API schema](https://www.gdacs.org/gdacsapi/swagger/v1/swagger.json) and [terms of use](https://www.gdacs.org/About/termofuse.aspx).

A small read-only GDACS metadata probe confirmed the native FeatureCollection shape, cited NEIC origin, timezone-free timestamps and parameter schema; it was not an admitted collector run. Reproducible tests use synthetic native records rather than live conditions.
