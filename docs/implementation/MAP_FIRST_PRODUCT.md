# Map-first product: brief and latest qualification result

> **Status:** Active product statement and the latest qualification result. The release is **NOT QUALIFIED**. See the [records index](README.md).

## Contents

- [Current product brief](#product-brief)
- [29 October 2026 product evaluation](#29-october-2026-product-evaluation)

---
<a id="product-brief"></a>

## Current product brief

> **Status:** Active product statement. T01–T21 are implemented; T22–T24 remain IN_PROGRESS and the release is **NOT QUALIFIED** per the [29 October product evaluation](#29-october-2026-product-evaluation). See the [records index](README.md).

The current product helps an analyst select an area, inspect retained public evidence and its source meaning, acquire supported bounded inputs, investigate a question and produce a reviewed analytical result. Middle Eastern coverage has priority; supported worldwide workflows use the same contracts. Actual detail, dates, rights and provider access vary and must remain explicit.

The [current scope](../../EMAD_CURRENT_SCOPE.md) governs this work. The [implementation plan](MAP_FIRST_PLAN.md#implementation-plan) and [backlog](MAP_FIRST_BACKLOG_2026_10_25.json) define delivery. Existing simulation code/history remains available through the explicit historical profile; current environment work prepares geography only.

### Implemented batches

- **T01:** capture/export use explicitly selected or visibly pinned areas. Missing, unresolved, invalid or unsupported geometry cannot borrow an unrelated rectangle. A supported local rectangle is required by the existing rectangular operations.
- **T02:** the default deployed/local profile is `map`. Its analyst has Public Context grants and its supervisor starts public workers. Historical authoring/runtime/recording are selected through `historical-full`. Saved exercise defaults do not override map entry.
- **T03:** content purpose (`working`, `development`, `acceptance`, `unclassified`) is append-only metadata. It is separate from real/synthetic provenance, scientific quality and human review. Ordinary catalogs exclude known development/acceptance records; unclassified records remain explicit and are reviewed by authorized source administrators.
- **T04:** immutable areas retain exact geometry, edition/release provenance, component boundaries and explicit display limits. Complete geographic releases activate atomically while old exact selections remain readable.
- **T05:** one temporal contract distinguishes monitoring, fixed historical studies, revision interpretation and each source's clock/support. Bounded materialized reads retain stable membership through publication races, with current-access checks and explicit expiry.
- **T06:** map, Systems, imagery, review and export share one area/period or an explicitly pinned draft. URLs/bookmarks preserve that selection; invalid scopes recover visibly and obsolete requests cannot populate a new draft.

- **T07–T12:** server-owned investigations, regional reference editions/releases, separately approved retrieval profiles, durable requests/shared provider budgets, scope-specific coverage and monitoring/history.
- **T13–T18:** bounded stable paging, international input/role workflows, imagery intents and study preflight, immutable findings, canonical independent review and composite output rights/retention.
- **T19–T21:** durable author-bound delivery workers, frozen readable HTML/offline approval bundles, retained native geographic normalization and qualified neutral terrain/vector/imagery packages.
- **T22–T24:** repeatable isolated workload samples, per-operation request histograms, expanded paired recovery checks, observed-session validation and release traceability tooling. Qualification itself is in progress.

See [T19–T21 evidence and boundaries](MAP_FIRST_DELIVERY_LOG.md#t19-t21), [qualification work](MAP_FIRST_DELIVERY_LOG.md#t22-t24), [release ledger](evidence/map-first/t22-t24-review/release-ledger.json), the [29 October product evaluation](#29-october-2026-product-evaluation) and [operations](MAP_FIRST_OPERATIONS.md#map-profile). Coordinated API/UI/workers require forward migrations through **00053**. The user reports applying migrations; shared startup/schema/source installation were not independently verified. No additional migration is introduced by T22–T24.

Coordinates use signed longitude first, WGS84, with Arabic/English digit entry; fixed intervals retain timezone/UTC meaning and use an exclusive end. Exact polygons preserve holes, multipart/date-line components and immutable release identity; Web Mercator/high-latitude display limits remain explicit. Geographic preparation supports the bounded profile in the operations guide, rather than every projection/datum. Source profiles, details, acquisition rights and actual date coverage are independently administered. The Middle East regional preset and seven-country test samples do not prove those releases are installed in this user's database.

### Useful progress

Count reference/catalog revisions, successful capture records, distinct acquisition timestamps, distinct completed input/method comparisons and reviewed working assessments separately. A repeated capture or development publication cannot manufacture an independent acquisition or approved working output. Counts describe retained content; selected-area task readiness uses T11 coverage and reports gaps explicitly.

Full workload/24-hour operation, actual rollback compatibility, complete real geographic journeys and observed independent users remain open. All mandatory acceptance scenarios retain their current ledger state. Simulation milestones do not resume automatically.

<a id="29-october-2026-product-evaluation"></a>

## 29 October 2026 product evaluation

**Release status: NOT_QUALIFIED. T22–T24 remain IN_PROGRESS.**

The retained installation supports part of the intended workflow, but it is not yet reliably usable or qualified for analytical delivery. Passing automated checks does not resolve the product findings below. Work crossed the original one-hour evaluation window; this is not a claim that all qualification was completed within an hour.

### Actual workflow and outcome

The evaluator operated the production UI against the retained database and private artifact storage, without mocked API responses. Local map profiles first needed reconciliation with the supported `emad-local -profile map` command: migrations alone had left the maintained legacy analyst without investigation and output permissions. Automatic local analyst entry now works in the evaluation launcher; Account remains available and backend authorization remains enabled. This is trusted loopback access, not public anonymous access or a completed deployment-wide authentication change.

The installation returned 24 reference versions, 177 country boundaries, 3,706 retained evidence revisions, 20 imagery scenes, four captures, two acquisition dates and one comparison. These counts establish retained material, not useful coverage for a particular question. Reference, scene and capture inputs remain unclassified for content purpose; there are no reviewed working outputs.

A deliberate one-kilometre area centred at longitude 46.5, latitude 24.5 was selected. The clearly labelled draft `[Evaluation] Retained evidence near Riyadh` was saved and reopened. Its internal brief rendered through one explicitly executed author-worker attempt and opened in the frozen reader. Saved geometry and rendered bounds agreed. The brief contained no included evidence or findings and correctly reported coverage as unknown. It establishes durable save/render behavior, not a useful completed analysis. Independent review and approved distribution were not performed.

During navigation and reopening, the UI reported **“active read capacity exhausted”** and could not refresh the investigation list. The saved case and output remained durable. Multiple evaluation browser contexts were present, so a single-browser reproduction and exact cause remain unresolved. Read diagnostics showed retained active manifests across catalog families. View replacement, unmount, reload and expiry handling need investigation; increasing quotas would not demonstrate a lifecycle fix.

### Findings and required outcomes

1. **Read lifecycle blocks ordinary work.** Fix and evaluate opening, switching, saving and reopening views across realistic sessions. Successful recovery must preserve the case and restore the list without requiring the analyst to understand snapshot capacity.
2. **Dense workload responsiveness fails.** With 100,000 report revisions, 50,000 detailed reference features, 10,000 weather records and 1,000 history entries coexisting, five analyst identities, ten areas and ten schedules, the fourth full-volume run failed frozen targets. Cold map p95 was 2,854.619 ms against 2,000 ms; warm geography p95 was 1,079.928 ms against 1,000 ms; warm weather requests reached approximately 5,000 ms/timeouts. Investigate actual query and contention costs, retain failures, and repeat the same workload. An experimental weather rewrite worsened performance in the fifth run and was reverted.
3. **Real analytical usefulness is unproven.** Classify retained inputs with appropriate source rights, establish area/time coverage, select suitable real imagery and terrain, and complete evidence-backed studies through independent review. An empty brief or a successful synthetic fixture cannot establish this outcome.
4. **Workflow still requires technical assistance.** Membership and evidence inclusion expose raw identifiers; environment qualification requires JSON. Observe whether analysts can discover colleagues, select evidence and complete tasks without developer intervention. Replace these technical inputs where they obstruct the intended user.
5. **Coordinate presentation is ambiguous.** The footer retained 43.3000/24.7000 while the deliberate study and frozen brief correctly used 46.5/24.5. Clarify pointer versus study coordinates; no incorrect saved geometry was demonstrated.
6. **Operational startup remains incomplete in this evaluation.** One delivery attempt was run explicitly. No continuous shared worker supervisor or provider acquisition was started, so future queued briefs and live refresh cannot be assumed to run automatically. Evaluate the normal assembled deployment and recovery procedure.

The geographic catalog's omission of exact study geometry was corrected. The final broad integration selection passed after this fix and after reverting the experimental weather rewrite. This supports correctness of covered contracts; it does not close the performance or user-workflow findings.

### Evidence and limits

Machine-readable observations, run references, exact artifact identities and verification results are in [evaluation.json](evidence/map-first/qualification-2026-10-29/evaluation.json). Full-volume attempts retain their own source/profile records and raw failures; the fourth run represents the final runtime query design. Existing browser checks and build checks passed. No additional tests are needed merely to increase the passing count.

No independent participant observer record has been received. The automated role-entry checks and evaluator-operated journey are not human usability sessions. The 24-hour soak, complete real imagery/environment journeys, nominated rollback-binary rehearsal and full installed backup-CLI rehearsal remain incomplete. Paired recovery integration evidence does not substitute for those installation exercises. Raster concurrency and actual browser latency under the full-volume workload were not established.

Shared changes were supported local map-profile provisioning and one labelled evaluation case/internal brief. No shared migrations or provider fetches were run. No human approval was fabricated.

### Next evaluation sequence

First resolve the navigation/read-capacity blocker and establish normal worker startup. Then repeat ordinary analyst journeys on the actual installation and the unchanged dense workload, retaining failures and assistance required. Complete real evidence-backed studies and independent observed sessions. Finally execute the long-running and deployment recovery gates. Release qualification requires these outcomes together; implementation completion alone is insufficient.
