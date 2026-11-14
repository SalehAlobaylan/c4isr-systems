# Map-first plan: gaps, analysis, tasks and audit

> **Status:** Active governing plan for the map-first phase. T01–T21 are implemented; **T22–T24 remain IN_PROGRESS and the release is NOT_QUALIFIED** — see the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation) and the [records index](README.md).

## Contents

- [Gap report and recommended actions (25 October 2026)](#gap-report)
- [Gap analysis against the committed implementation](#gap-analysis)
- [Implementation plan: tasks, acceptance, traceability](#implementation-plan)
- [Plan completeness and hardening audit](#plan-audit)

---
<a id="gap-report"></a>

## Gap report and recommended actions (25 October 2026)

**Date:** 25 October 2026\
**Status:** Current scope correction recorded; implementation backlog proposed for review.\
**Governing scope:** [EMAD_CURRENT_SCOPE.md](../../EMAD_CURRENT_SCOPE.md).\
**Current objective:** A useful map-centered Public Context and Intelligence product for the **Middle East and the rest of the world**, with **geographic environment preparation only** for future simulation.

**Follow-up planning:** The [code-backed gap analysis](#gap-analysis) and [implementation plan](#implementation-plan) develop these recommendations into proposed solutions, dependencies and acceptance checks. The [backlog ledger](MAP_FIRST_BACKLOG_2026_10_25.json) records planned work, not completed implementation.

### 1. Correcting the direction

Simulation behavior was advanced ahead of the dedicated design conversation the user intends to have. That was a scope-management error. Runtime, actors, sensors, exercise operations, replay and evaluation must not continue as active milestones in this phase. Existing code and evidence should be preserved and clearly separated from the current map product. No deletion, worker shutdown or application change is performed by this report.

The earlier evaluation also made too much of the prepared Jeddah installation as a product direction. Jeddah is a useful historical fixture, not the intended geographic scope. The current product covers the Middle East with access and extensibility across the rest of the world. Future detailed simulation design remains open.

The recommendation is to preserve the working platform and concentrate delivery on a coherent map, meaningful regional/worldwide data coverage and complete analytical workflows. This report supersedes the earlier recommendations to center the product on a Jeddah-only pilot or develop a simulation pilot in parallel.

### 2. What “Middle East plus the world” means

The implementation needs three connected levels:

1. **Worldwide geographic access:** navigation, search, named geographic areas, country/region selection, references, consistent identities and explicit supported map/projection behavior.
2. **Regional intelligence coverage:** declared source footprints and time windows, public observations, imagery discovery, freshness, uncertainty and historical interpretation. Middle Eastern coverage receives delivery priority. The same mechanisms must work elsewhere where suitable sources and permissions exist.
3. **Detailed selected-area work:** bounded acquisition, analysis and geographic preparation for a chosen area. Large selections use summaries/tiles; detailed processing uses explicit smaller areas and durable jobs. Global scope does not require preloading every road, building or raster at full resolution.

Actual coverage may vary by source, licensing, credentials, geography and period. Those differences must be visible and actionable. Worldwide country navigation alone is not complete worldwide intelligence support, and an empty query is not proof of absence.

Use a configurable, versioned Middle East region definition rather than embedding a single country list or bbox throughout the application. Include relevant land, coastal and maritime areas according to the chosen task and source footprints. Retain the dataset's boundary interpretation and attribution; generalized references do not certify jurisdiction.

### 3. Evidence from the current installation

- Global country navigation already exists. A fresh [geographic sample](evidence/map-first/geographic-sample.json) found Saudi Arabia, the UAE, Jordan, Egypt, France, Japan and Brazil. Saudi Arabia returned 13 installed first-order regions; the other sampled countries returned none. This samples the installed catalog, not the world's available data.
- `scripts/data/install_geography.py` explicitly installs world navigation and Saudi subdivisions. Other countries need deeper reference coverage through the same generalized area model.
- `scripts/data/install_local.py` assigns Jeddah to imagery, FIRMS, OpenSky and AIS admissions and a Saudi extent to other installed retrieval profiles. These are bounded installation choices, not the intended limits of the product.
- Home, the Systems reference preview and context-export fallbacks contain Jeddah/Saudi-specific assumptions. An inventory must distinguish intentional fixture definitions from defaults that affect ordinary users.
- The [read-only product snapshot](../design/product-evaluation/installation-snapshot.json) contains 24 reference versions, 19 imagery catalogue revisions, four successful capture records and two distinct acquisition dates. The real imagery assessment is still a proposal; the published record is a development workflow test.
- The [browser reproduction](../design/product-evaluation/walkthrough.json) confirms a scope defect: selecting Saudi Arabia and clicking **Use investigation region** fills the Jeddah rectangle in export. No package was created during that review.
- The current development account has 42 contexts, many from exercise history. That is not a requirement for normal map users and must not define the current product's information architecture.

Existing functional tests provide valuable technical evidence. They do not demonstrate complete regional/worldwide coverage, independent usability or compliance with the newly corrected phase boundary. No new application tests were run merely to write this report.

### 4. Gap register

**P0:** correctness or scope prerequisite for this phase.\
**P1:** core map-product delivery.\
**P2:** supporting environment preparation after its geographic inputs are trustworthy.\
Priorities are proposed ordering, not a claim that implementation has started.

#### GAP-01 · Simulation scope drift — P0

**Finding:** The installation and prior plans advanced runtime/exercise features beyond the user's intended current phase.

**Action:** Establish a map-first phase boundary. Exclude simulation execution and exercise-oriented onboarding from the normal map journey and its definition of done. Make ordinary map startup independent of simulation workers. Preserve existing implementation/history separately; do not add new runtime features or choose a future engine.

**Acceptance:** A normal analyst can use the product without selecting an exercise, starting a runtime, opening Studio or understanding simulation partitions. Current delivery tasks contain only map/Public Context work and the allowed environment foundations.

#### GAP-02 · Geography hardcoded into product assumptions — P0

**Finding:** Multiple installation policies, previews and fallback areas are tied to Saudi Arabia/Jeddah.

**Action:** Inventory all geography-specific defaults. Move operational areas into named, versioned configuration and selected-area contracts. Keep Jeddah constants only where explicitly identified as fixtures or a selected dataset. Offer a Middle East overview and unrestricted supported world navigation.

**Acceptance:** Switching between supported Middle Eastern and worldwide areas uses the same workflow without editing source code, inheriting a Jeddah bbox or falsely claiming installed local detail.

#### GAP-03 · Incorrect area handoff — P0

**Finding:** A selected country can become the Jeddah export rectangle through **Use investigation region**.

**Action:** Give the selected area a stable identity/geometry/version across map, analysis, review, brief and export. Derive supported rectangles explicitly from the actual selection. Where a format or processing limit requires a smaller area, ask the user to choose that subset and explain the restriction. Never substitute a different place.

**Acceptance:** Country, subdivision, named study area, drawn polygon, viewport selection and reopened investigation retain the correct area. Unsupported extents produce a useful narrowing action. No silent geographic substitution occurs.

#### GAP-04 · Uneven geographic reference depth — P1

**Finding:** Worldwide country references exist; the installed subdivision/place coverage remains concentrated in Saudi Arabia.

**Action:** Provide a region-agnostic reference pipeline and catalog for administrative areas, settlements, coastlines and appropriate infrastructure. Expand Middle Eastern depth first while supporting worldwide acquisition through the same pipeline. Include source-backed aliases, Arabic/English names where available, retained rights, editions and stated geographic accuracy.

**Acceptance:** Users can find and select supported subdivisions/places in multiple Middle Eastern countries and international areas. Missing depth is labeled and can be requested/imported through supported actions. A place point is never presented as a municipal polygon.

#### GAP-05 · Source availability is confused with geographic coverage — P0

**Finding:** A source may be configured/current while the chosen country has no admitted or useful observations. Selecting a country does not change the source's installed footprint.

**Action:** Model source-by-area-by-time coverage explicitly. Support approved regional source profiles and bounded requests beyond the original installation. Keep endpoint validation, source-specific credentials, quotas, rights and immutable admission history. New coverage must not silently broaden existing service grants. Country selection should show coverage and available acquisition actions, not modify ingestion authority by itself.

**Acceptance:** Any selected area explains which sources cover it, the applicable periods, retrieval freshness, missing access and the next supported action. Outside-coverage, permission-gated and genuine zero-result states remain distinct.

#### GAP-06 · Current monitoring and historical study are insufficiently separated — P0

**Finding:** The fresh map opens at **World · configured evidence / All retained dates**. Old records can be prominent when someone expects current activity.

**Action:** Define explicit current-monitoring and historical-investigation modes, with visible area/time scope and source-dependent freshness. Preserve observed, received and revision times. Historical imagery belongs to a dated study, not an implied live picture. Define what a useful quiet-state summary says when no qualifying reports exist.

**Acceptance:** A user can explain the displayed period and coverage without opening diagnostics. Stale, historical, current and unknown information remain distinguishable. An empty well-covered query and an unobserved area receive different explanations.

#### GAP-07 · Regional/worldwide map scale needs qualification — P1

**Finding:** A clean canvas and country search do not establish that regional/global discovery remains useful under real data volumes and extent limits.

**Action:** Use bounded PostGIS aggregates, vector/raster tiles, scale-dependent visibility, readable legends and deliberate drill-down. Separate broad discovery from detailed processing. Handle maritime areas, date-line crossings and projection/high-latitude limits explicitly. Avoid silent result caps and whole-world detailed downloads.

**Acceptance:** Middle East overview, country, subdivision and local AOI views remain readable and responsive. Complete matching totals and displayed subsets have clear meanings. Large or unsupported requests offer an actionable alternative.

#### GAP-08 · Too many competing navigation concepts — P1

**Finding:** Product area, resource, workbench, Systems, inner tabs and technical vocabulary still compete with the analyst's task.

**Action:** Make the map the main working surface for this phase. Use a consistent model: selected area/period → evidence → investigation → finding/assessment → brief/export. Keep and improve the Systems grid as an overview of relevant content, coverage and work requiring attention. Keep the map clean with the existing compact dock and temporary/allocated panels. Place setup and processing details behind relevant actions.

**Acceptance:** A new analyst can locate data, inspect its source, continue an investigation and find its output without learning database partitions or simulation concepts. Labels and selection state remain consistent across entry points.

#### GAP-09 · No complete durable investigation spine found — P1

**Finding:** Saved filters/bookmarks and retained evidence objects exist, but the inspected workflow does not provide a coherent persisted analyst case.

**Action:** Add named investigations containing the question, area, period, selected evidence, notes, analytical work, review state and outputs. Reuse existing evidence IDs and versions. Include ownership, resume behavior and an explicit handoff to a reviewer; keep evidence immutable.

**Acceptance:** An analyst can save, restart, reopen and hand off the same investigation with its correct scope and supporting evidence. The investigation is useful across regions, rather than a Jeddah-specific screen preset.

#### GAP-10 · Imagery usefulness is demonstrated mainly by a prepared historical pair — P1

**Finding:** Four installed capture records represent two dates and one completed NDVI comparison.

**Action:** Make discovery, local AOI selection, acquisition, quality review, before/after comparison and retained analysis repeatable for other admitted areas and periods. Prefill forms from the actual map selection. Explain clouds/masks, alignment, insufficient coverage, processing failures and valid zero-change results. Reuse durable workers instead of synchronous API processing.

**Acceptance:** The workflow succeeds on additional Middle Eastern and worldwide AOIs where suitable data/access exist, without developer file preparation or source-code edits. Failure states remain honest. An indicator is not automatically published as a real-world conclusion.

#### GAP-11 · The primary output is too technical — P1

**Finding:** JSON bundles and manifests are valuable for reproducibility but are not a complete analyst deliverable.

**Action:** Produce a readable brief with the question, area, period, coverage, findings, maps/comparisons, cited evidence, uncertainty, limitations and review status. Retain the machine-readable bundle alongside it. Support independent human review using the existing canonical identity protections.

**Acceptance:** A permitted recipient can understand the brief without reading schemas or job records. Findings link to exact retained evidence. Development-account workflow tests remain distinct from actual independent human review.

#### GAP-12 · Development content and working content share normal catalogs — P1

**Finding:** Acceptance scenes, development publication records and old exercise contexts can inflate apparent readiness and distract from actual analytical content.

**Action:** Classify development/acceptance fixtures explicitly and filter normal defaults. Preserve immutable history. Distinguish source revisions, independent acquisitions, usable studies and approved outputs in counts. Do not delete old evidence or rename it into operational content.

**Acceptance:** Ordinary map users see relevant public content and their investigations, with no exercise-history requirement. Support/admin views can still inspect retained development evidence with its original meaning.

#### GAP-13 · Worker health does not answer task readiness — P1

**Finding:** Ready workers and available-feature counts can coexist with stale feeds, unavailable acquisition and unobserved selected areas.

**Action:** Present readiness for the current area/question: usable evidence, available dates, coverage limitations, provider state, queued work and recovery action. Keep private credential/configuration detail in administration. Support interruptions, retries, deduplication, quota pacing and explicit external prerequisites.

**Acceptance:** Users can tell whether their task can finish and what is missing. A stalled provider is actionable; a successful empty retrieval does not become a false failure or an absence claim. Restricted feeds remain gated rather than simulated.

#### GAP-14 · Wider workloads and unattended operation need acceptance — P1

**Finding:** Existing bounded profiles do not establish a worldwide, multi-area operational envelope.

**Action:** Define map/analysis budgets, cancellation and pagination behavior, realistic data-density loads and an unattended regional monitoring profile. Verify startup/restart, worker loss, storage failure, credential expiry and paired data/object restore. Record hardware, query/interaction latency and data volume with results; reuse working recovery infrastructure.

**Acceptance:** Declared regional/worldwide workloads meet agreed limits and recover predictably. Changing area cancels obsolete requests, prevents stale content from appearing in the new area and preserves proper access boundaries.

#### GAP-15 · International and role-focused usability is not yet qualified — P1

**Finding:** The current broad development account does not represent every analyst/reviewer experience, and worldwide scope brings naming, language, units and time interpretation requirements.

**Action:** Provide role-led starting points and progressively disclosed technical controls. Validate Arabic/English place search, mixed-script rendering, clear local/UTC timestamps, supported coordinate/datum entry, keyboard/focus, readable contrast and narrow screens. Do not assume a country selection implies an account or permission change.

**Acceptance:** Target users can identify the current area, period, source meaning and available actions. Important place aliases work. Normal tasks do not require token-file paths, raw identifiers or unexplained datum conversion.

#### GAP-16 · Environment preparation must become portable and region-agnostic — P2

**Finding:** The current detailed prepared environment is near Jeddah; it must not stand in for the global preparation capability.

**Action:** Generalize bounded geographic preparation: selected area → permitted terrain/imagery/vectors → cropped/versioned inputs → validated environment manifest/artifacts. Record CRS, units, input/output vertical datums, transformation grids and their area of use, source dates/rights, estimated building properties, omissions and size/LOD budgets. Retain parent bytes and transformation lineage. Heavy preparation stays outside the synchronous Go API.

**Acceptance:** An additional supported area can be prepared without changing application code. Artifacts are reusable by future tools, correctly georeferenced and truthful about fidelity. This gate contains no actors, simulation clocks, behaviors, synthetic sensors, exercise start or replay.

#### GAP-17 · Product usefulness has not been independently demonstrated — P1

**Finding:** Passing implementation tests does not prove an analyst can obtain a useful result without developer guidance.

**Action:** Observe independent target users performing map discovery, current/historical inspection, investigation, review and brief/export tasks. Test multiple Middle Eastern and worldwide areas, including missing coverage and quiet periods. Measure correctness, assistance required, completion and output usefulness against the user's present workflow.

**Acceptance:** Users complete the agreed tasks and correctly explain coverage/uncertainty. Any wrong-area or wrong-period interpretation is a blocking finding. Record observed results rather than assigning a speculative completion percentage.

#### GAP-18 · Documentation and approval state can restart the wrong roadmap — P0

**Finding:** Older documents and completion summaries still emphasize broad C2 or implemented simulation milestones. They can be mistaken for current authorization.

**Action:** Put the current scope above historical plans. Track implementation, installation, functional verification, geographic coverage and user validation separately. Retain previous results as history; require a later dedicated simulation discussion before opening a simulation implementation sequence.

**Acceptance:** “Next task” cannot automatically mean another simulation increment. The current product definition, backlog and acceptance criteria consistently say map/Public Context, Middle East plus world, and environment preparation only.

### 5. Recommended delivery sequence

These are outcome gates, not dates or automatic execution instructions. Reliability, isolation and accessibility checks accompany every stage.

#### MAP-0 · Correct the scope and ordinary product surface

Addresses GAP-01, GAP-12 and GAP-18. Adopt the current scope, reconcile the active product brief, identify fixture-only content and make normal map use/startup independent of simulation. Preserve old code/history and remove its status as an active phase requirement.

**Exit:** A normal user enters the current map product without exercise/runtime choices or development content determining the workflow. Historical facilities remain separately identifiable.

#### MAP-1 · Establish the global area model and correct every handoff

Addresses GAP-02, GAP-03 and the contracts behind GAP-04/07/09. Make countries, subdivisions, named areas, drawn areas and viewports use a consistent identity/geometry/version model. Define the durable investigation scope and explicit large-area/subset rules early. Fix the export substitution immediately.

**Exit:** Area and period survive navigation, analysis, review and export correctly across regional and worldwide samples.

#### MAP-2 · Expand reference depth and explicit source coverage

Addresses GAP-04, GAP-05 and GAP-13. Install/enable regional reference depth through a generalized pipeline, expose provider coverage by area/time, and support bounded approved source profiles. Middle Eastern coverage is prioritized; worldwide acquisition uses the same mechanisms rather than a separate city-specific implementation.

**Exit:** Users can discover what is available, missing or obtainable in any supported selection. Global navigation is no longer confused with universal live observation coverage.

#### MAP-3 · Deliver a useful map-centered daily workflow

Addresses GAP-06, GAP-07, GAP-08 and GAP-15. Establish current/historical modes, meaningful quiet states, scalable layers, clear evidence inspection and consistent actions. Preserve the clean map and Systems grid. Make user tasks, rather than module names, determine the interaction.

**Exit:** Users can answer what is reported for an area/period, how fresh it is, what supports it and what remains unknown.

#### MAP-4 · Complete repeatable investigation, imagery and reporting

Addresses GAP-09, GAP-10 and GAP-11. Persist investigations, support further real acquisitions, review quality and interpretations, hand off to a reviewer, and produce a readable brief with a reproducibility bundle.

**Exit:** The same complete workflow succeeds in multiple Middle Eastern and worldwide areas where inputs/access exist. It does not rely on the prepared Jeddah pair or developer intervention.

#### MAP-5 · Generalize environment preparation only

Addresses GAP-16. Reuse the now-trustworthy geographic acquisition and versioning pipeline to prepare portable environment artifacts. This can proceed alongside late MAP-4 work once its input contracts are stable.

**Exit:** A validated environment package is produced for another supported area. Nothing in this gate executes a simulated world or commits the user to a future simulation design.

#### MAP-6 · Qualify the phase through real use

Addresses GAP-14 and GAP-17 and closes the earlier gates. Run independent analyst/reviewer sessions, unattended monitoring/recovery and declared map workloads. Resolve the resulting task failures before expanding scope.

**Exit:** The map/Public Context workflow is measurably useful, geographically honest and repeatable. Simulation discussion is a separate later decision, not the next automatic implementation gate.

### 6. Geographic acceptance coverage

The acceptance matrix must sample the following conditions. Named test sites are qualification examples, not geographic limits of the product:

- Several Middle Eastern countries and subdivisions, rather than Saudi Arabia alone.
- A cross-border regional view and a relevant coastal/maritime area with truthful provider coverage.
- Multiple areas outside the Middle East on different continents.
- A country-level selection narrowed deliberately to a supported analysis AOI.
- Named, drawn, viewport and restored-investigation areas with identical meaning across downstream actions.
- A date-line-crossing selection and explicit behavior for high latitudes/projection limits.
- Current observations, a historical investigation, a genuine zero-result period, missing coverage, a stale source and a credential/permission gate.
- A fresh imagery acquisition and a prepared environment outside the existing Jeddah fixture where supported inputs are available.

Every sampled journey should preserve area, period, source attribution, rights, version and uncertainty. Tests must verify that the implementation is region-agnostic, not merely add another fixed list of demo cities.

### 7. Preserve and defer

**Preserve/reuse:** Go modular domains, PostGIS geometry operations, MapLibre and the compact dock, Systems grid, source/evidence history, confidence/provenance distinctions, review identity safeguards, durable acquisition/analysis jobs, private object storage, backups and portable geographic artifacts.

**Defer from the current phase:** simulation engines and engine selection, runtime enhancements, actors and behaviors, synthetic sensor modeling, exercise-controlled missions/commands, simulation recording/replay, evaluator features and further S4 expansion. Existing versions remain historical assets subject to a later agreed design.

The next success criterion is a useful map and a trusted analyst outcome across the intended region/world scope. Environment readiness supports that work now and preserves future options; simulation execution is not part of the current deliverable.

<a id="gap-analysis"></a>

## Gap analysis against the committed implementation

**Date:** 25 October 2026.\
**Reviewed commit:** `a6d1d22f34692faea31db58e4a6318c539c3321a` — `feat(emad): add public intelligence workflows and map workspaces`.\
**Inputs:** [gap report](#gap-report), [current scope](../../EMAD_CURRENT_SCOPE.md), current source, migrations, tests and retained evaluation evidence.\
**Status:** Analysis and proposed closure criteria. This document does not authorize or implement a delivery batch.

**Follow-up:** The [implementation plan](#implementation-plan) and [backlog ledger](MAP_FIRST_BACKLOG_2026_10_25.json) translate this analysis into proposed contracts, migrations, ordered tasks and qualification scenarios.

### Assessment

All 18 gaps remain relevant. The proposed priorities are reasonable: six P0 prerequisites, eleven P1 delivery/qualification gaps and one P2 environment-preparation gap. Their existence does not mean the last delivery produced no useful platform. Its strongest foundations are already present: scoped access, immutable evidence, admitted sources, versioned geography, PostGIS queries and tiles, durable imagery work, canonical review identities, and paired database/object recovery.

The central problem is that these capabilities do not yet share a complete analyst workflow. Different screens can mean different things by “area,” “investigation,” “current,” “available” and “published.” A successful component can therefore contribute to an incorrect or unusable product outcome.

The most immediate confirmed defect is geographic substitution in export. Several other failures are reachable from the source: capture can retain an unrelated AOI, exact geographic IDs can become unavailable after a new edition, and downstream workbenches do not carry a common period contract. Wider performance and independent usability remain unqualified; the evidence does not establish that every broad workload fails.

The recommendation is to repair area/time integrity first, make coverage and task readiness explicit, and complete one durable investigation through independent review and a readable brief. Expand that same workflow across regions. Preserve the existing infrastructure and historical simulation work under the corrected phase boundary.

### Evidence and limits

- The working tree was clean at the start of this review. Findings below concern the committed source rather than an uncommitted implementation branch.
- The [geographic sample](evidence/map-first/geographic-sample.json), [browser walkthrough](../design/product-evaluation/walkthrough.json) and [installation snapshot](../design/product-evaluation/installation-snapshot.json) were read as retained observations from 25 October. Provider health and installed counts were not freshly queried during this analysis.
- Static source tracing confirms implementation behavior where stated. Additional conditions discovered by inspection are identified below; they have not been newly reproduced in a live browser or database.
- `go test ./internal/publicevents ./internal/publicingest ./internal/imageryanalysis ./internal/contextpackage ./internal/assessments` exited successfully. All five package results came from the Go test cache. No new browser, integration, load, fault-injection or independent-user session was run.
- Existing geographic integration and browser tests were inspected. Their coverage is credited below without treating their historical results as new execution evidence.
- No application code, source admission, credentials, running workers, installed content or immutable evidence was changed for this analysis.

### Shared causes

1. **Product scope changed, but startup and ordinary entry points still implement the previous two-pillar product.** Documentation precedence is partly corrected; behavior is not.
2. **Area and period are fragmented.** The map has a region ID, drawn geometry and saved filters; workbenches have their own pinned state; export independently reads browser storage. There is no authoritative scope revision connecting them.
3. **Catalog presence substitutes for task readiness.** A record count, a healthy worker, a source admission and usable evidence for a particular area/time are different facts.
4. **Evidence is durable, but the analyst's case is not.** Jobs, source records and review revisions survive; the question, selected evidence, notes, working scope and handoff do not form a persisted investigation.
5. **Acceptance is narrower than the product promise.** Existing tests demonstrate specific technical paths, often against prepared data and broad development privileges. They cannot establish independent usefulness across regions and operational conditions.

### GAP-01 — Simulation scope drift · P0

**Assessment: confirmed in startup and normal navigation; public-data isolation already exists.**

[Home](../../apps/operator-ui/src/routes/home.tsx#L38) still asks users to choose between public evidence and a simulated world. The [map chooser](../../apps/operator-ui/src/features/workspace/MapWorkbenchChooser.tsx#L16) includes simulation/replay. [Local setup and startup](../../scripts/ops/local.py#L164) build/describe the runtime and launch asset, recording and runtime workers alongside map services.

This is delivery-profile coupling, not proof that public queries intrinsically require an exercise to run. The supervisor already restarts failed workers separately, and resource boundaries distinguish public evidence from exercise state. Those protections should be retained.

**Closure:** a map startup profile containing the required API, storage, connectors and imagery services; an ordinary map-first entry journey; separately identifiable access to historical facilities. Missing simulation executables, grants or worker readiness must not prevent this profile from starting or being considered ready.

**Proof:** start the declared map profile without simulation services, using an analyst with only the intended public permissions, and complete discovery, investigation and output tasks. This establishes independence more reliably than hiding a navigation label.

### GAP-02 — Geography hardcoded into product assumptions · P0

**Assessment: confirmed; distinguish legitimate fixtures from ordinary defaults.**

The relevant inventory includes [source installation policy](../../scripts/data/install_local.py#L77), [Systems fallback and preview](../../apps/operator-ui/src/features/workspace/SystemsWorkspace.tsx#L12), [export fallback](../../apps/operator-ui/src/features/imagery/ContextExport.tsx#L19), [capture defaults](../../apps/operator-ui/src/features/imagery/CaptureWorkbench.tsx#L18), [initial map camera](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L46), and [geography preparation](../../scripts/data/prepare_geography.py#L17). The camera is a presentation default; export/capture defaults can alter the work itself and take precedence in remediation.

The source uses one prepared installation as both sample data and implicit product configuration. No configurable, versioned Middle East definition was found in the inspected application/installer paths.

**Closure:** named region configuration with retained version/meaning, explicit installed dataset footprints, and selected-area parameters for operational actions. Preserve Jeddah fixtures and their original labels. A regional overview must describe the chosen region definition, including relevant coastal/maritime areas, rather than implying one politically universal boundary.

**Proof:** switch to a Middle Eastern and a non-Middle-Eastern area through the same flow; inspect every derived query, draft AOI and preview. No source edit or unrelated location default should be required. Depends on the GAP-03 scope contract.

### GAP-03 — Incorrect area handoff · P0

**Assessment: confirmed defect, with a broader persistence problem.**

[Country selection](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L88) stores/navigates a `region` ID. [Public filtering](../../apps/operator-ui/src/features/public-context/usePublicContext.ts#L29) correctly uses `region_id` or geometry instead of the saved bbox. But [ContextExport](../../apps/operator-ui/src/features/imagery/ContextExport.tsx#L18) reads only `investigation.filters.bbox` from a separate preference and supplies Jeddah when that value is empty. [IntelligencePage](../../apps/operator-ui/src/routes/intelligence.tsx) passes it a published revision, not the selected geographic scope. This explains the retained browser reproduction.

There is a second source-confirmed failure mode: [Geography](../../internal/reference/geography.go#L138) chooses the latest row for each logical region before filtering by exact ID. After admitting a newer edition with the same region key, lookup of a saved older ID yields no feature even though its immutable geometry remains stored. Saved investigations therefore need a version-resolution rule, not just a string ID.

**Closure:** one explicit area reference containing logical identity, exact edition/revision, geometry or a resolvable geometry hash, source meaning and derived bounds. Named, drawn and viewport areas need the same downstream contract. An analysis subset must record its parent scope and be deliberately selected. Bbox-only formats must explain their representation and limits.

**Proof:** all six area forms in the report survive map → analysis → review → export → reopen, including after a newer geography edition is installed. Missing, inaccessible, too-large or unsupported geometry must produce a blocking/narrowing state, never another location. Pinned workbench inputs may remain pinned, provided their actual area is explicit.

### GAP-04 — Uneven geographic reference depth · P1

**Assessment: confirmed installation gap; the generalized storage/admission foundation exists.**

[install_geography.py](../../scripts/data/install_geography.py#L25) installs worldwide country navigation, then filters subdivisions to `SAU` and places to the Saudi geometry. The [retained sample](evidence/map-first/geographic-sample.json) supports this: 13 Saudi first-order regions and none returned for the six other sampled countries.

[Geographic editions](../../internal/reference/geography.go#L24) already retain source, rights, hashes, geometry, aliases and edition metadata. The map already treats a place point as a location rather than inventing a municipal polygon. Those are valuable completed foundations.

**Closure:** parameterized reference acquisition/import, a versioned regional selection profile, explicit available/missing detail and a supported request/import action. Preserve source-backed Arabic/English aliases and edition/accuracy attribution. Do not claim the generalized country layer supplies every worldwide administrative unit or territory.

**Proof:** several countries and subdivisions plus international places, using the same pipeline. Include unavailable detail and point-only places. Expansion also needs geographic-result pagination: the backend returns at most 300 regions, while the inspected map consumers do not paginate those lists. Fix exact-edition resolution under GAP-03 before using new editions as saved case references.

### GAP-05 — Source availability versus geographic coverage · P0

**Assessment: confirmed missing composition; admission and health are already modeled separately.**

[Admission policy](../../internal/publicingest/domain.go#L52) retains an AOI, lookback, freshness, quotas, rights and connector identity. [Ingestion health](../../internal/publicingest/repository.go#L435) tracks successful retrieval windows and explicitly avoids an absence claim. The [coverage map family](../../internal/reference/map.go#L173) renders admitted query extents with the same qualification. This is more than a simple configured/unconfigured source flag.

What is missing is the answer for **this selected area, period and task**: footprint intersection, relevant retained evidence, actual retrieval completeness, freshness, permission/credential gates and permitted next action. A successful empty query remains only an empty provider query unless observational completeness is independently justified.

There is also a multi-area design constraint: [admitting a new policy](../../internal/publicingest/repository.go#L60) replaces the active state for a source and resets checkpoint/coverage. Reusing that mutation on every country selection would disrupt monitoring. Decide how independent approved retrieval profiles share provider identity and account-level quotas without overwriting each other's state.

**Closure/proof:** an area/time coverage result that distinguishes outside footprint, partial/unknown coverage, gated access, stale retrieval, successful empty retrieval and usable evidence. Country selection remains read-only with respect to ingestion authority. Exercise each state with scoped identities and retained admission history. GAP-06 and GAP-13 depend on this.

### GAP-06 — Current monitoring versus historical study · P0

**Assessment: confirmed semantic gap; historical event semantics already exist.**

[Default filters](../../apps/operator-ui/src/features/public-context/filters.ts#L8) use `latest` without an observation interval. Here `latest` means the latest retained revision of a record, not a recently observed event. The map accordingly displays “All retained dates.” Existing `known_at` and `effective_at` semantics are valuable and should not be collapsed into a generic date filter.

The [map API](../../internal/reference/map.go#L144) honestly returns `historical_unsupported` for unsupported families. However, for ordinary latest-view requests it [clears observation bounds for imagery and coverage](../../internal/reference/map.go#L202). Old captures can therefore remain under a recent-period header. Separately dated context is a valid feature, but its exception must be visible in the main layer/scope language. Workbench scope does not currently carry the same temporal fields as map filtering.

**Closure:** explicit monitoring/historical mode, a declared rolling monitoring window, visible timezone and freshness, and separate observation, acquisition, provider-revision, receipt/publication and analysis times. Unsupported historical data remains unavailable. Static historical context must be visibly identified as outside the monitoring window or intentionally included.

**Proof:** late arrivals, corrected/withdrawn reports, stale sources, historical imagery, timezone boundaries and empty queries. Verify the period at every downstream handoff. Define a coverage-aware quiet-state explanation with GAP-05 before treating “no results” as a useful monitoring outcome.

### GAP-07 — Regional/worldwide scale · P1

**Assessment: qualification and completeness gap, not absence of scaling work.**

The implementation already provides [event aggregates](../../internal/publicevents/map.go#L16), [other-family aggregates and reference vector tiles](../../internal/reference/map.go#L208), complete matching totals for map families, and separate detail queries. [Geographic integration tests](../../internal/integration/geographic_map_test.go) inspect exact scopes, history, aggregates above 2,000 records, tile access and isolation.

The remaining problem is the declared workload envelope: density, geometry complexity, tile size, query cost, concurrency, readability and actionable drill-down. Limits are inconsistent across surfaces. Geographic lists stop at 300; [captures stop at 100](../../internal/imagery/captures.go#L95) and their [HTTP response](../../internal/imagery/capture_http.go#L21) exposes only items, without total/truncation or paging.

[Study geometry validation](../../internal/publicevents/geometry.go#L11) rejects date-line crossings and coordinates beyond ±85°, and limits polygon complexity/extent. These are explicit restrictions, not evidence of silently supported global geometry. The user needs an actual split/narrow alternative, while broad country geometry and local processing retain different budgets.

**Closure/proof:** a declared workload for regional overview, country, subdivision and local detail; complete totals/subset semantics on every catalog; measured latency and bounded cancellation; maritime/date-line/high-latitude cases. Do not lift existing limits globally without replacing their resource protection. Begin these checks during MAP-1/2 rather than waiting until MAP-6.

### GAP-08 — Competing navigation concepts · P1

**Assessment: confirmed workflow gap despite substantial map/dock improvements.**

The compact map dock, [Systems grid](../../apps/operator-ui/src/features/workspace/SystemsWorkspace.tsx) and [persistent workbench host](../../apps/operator-ui/src/features/workspace/MapWorkbenches.tsx#L12) already provide reusable working surfaces. Replacing them wholesale would discard useful work.

The remaining vocabulary reflects implementation modules: workspaces, workbenches, inner imagery tabs, technical context packages and a simulation entry inside the ordinary map chooser. More consequentially, the pinned workbench and current map can have different scopes. The existing “Use current map area” action acknowledges that distinction, but export bypasses the host entirely.

**Closure:** make the task progression visible: area/period → evidence → investigation → finding/review → brief/export. Keep Systems for relevant content, coverage and work needing attention. State whether a view follows the current selection or a pinned investigation revision; preserve drafts without silently rebinding them.

**Proof:** a new analyst can find evidence, inspect provenance, resume a case and retrieve its output using task language. Test map, Systems, direct links and back/forward navigation. GAP-01 and GAP-03 establish the phase and scope behavior; GAP-09 supplies the durable object navigation should follow.

### GAP-09 — Durable investigation · P1

**Assessment: confirmed missing product aggregate; durable evidence should be reused.**

The [workspace preference hook](../../apps/operator-ui/src/features/workspace/preferences.ts#L5) stores device-local settings. [Map bookmarks](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L65) retain view settings and links, not a case. [Mounted workbenches](../../apps/operator-ui/src/features/workspace/MapWorkbenches.tsx#L12) retain in-memory drafts while switching tools. Review revisions and analysis jobs are durable, but no investigation aggregate containing question, owner, scope, evidence selection, notes, handoff and outputs was found in the inspected API/migration paths.

**Closure:** a server-persisted investigation with explicit identity, owner/access, versioned area and period, exact evidence references, notes/analysis links, review assignments and output revisions. Store evidence references rather than editable copies. Use revision/conflict checks for simultaneous edits and define what a recipient can access after handoff.

**Proof:** save, close the browser, reopen on another authorized session/device, resume and hand off the same case. Include a concurrent edit and a revoked/inaccessible evidence reference. Browser preference retention alone does not meet this gate.

Define the investigation identity and scope contract in MAP-1, then build the smallest usable case early enough to drive MAP-3 workflows. Full reporting can remain MAP-4.

### GAP-10 — Repeatable imagery usefulness · P1

**Assessment: repeatability gap; acquisition and analysis are real implemented capabilities.**

[Capture jobs](../../internal/imagery/captures.go#L110) validate scene coverage and rights, freeze input, deduplicate by request key, enforce budgets and use expiring worker leases. The [raster worker](../../scripts/data/capture_worker.py) performs processing outside the synchronous API. [NDVI comparison](../../internal/imageryanalysis/ndvi.go#L135) checks aligned grids, dates, platform/tile/orbit/baseline and common clear pixels. Valid zero-change output is distinct from unsuitable inputs.

The user workflow still defaults to Jeddah, and [CaptureWorkbench](../../apps/operator-ui/src/features/imagery/CaptureWorkbench.tsx#L17) requests workspace-wide captures even though the API accepts region/geometry. Its comparison defaults select the earliest/latest retained captures, which need not belong to the current area. Changing to an oversized AOI leaves an earlier capture AOI in place. Backend validation can reject unsuitable inputs, but it cannot infer which area the analyst intended.

Automatic capture currently supports the admitted Earth Search Sentinel-2 Collection 1 path; the presence of other catalog metadata is not equivalent acquisition support. The retained installation has two distinct acquisition dates, not four independent temporal samples.

**Closure/proof:** selected-area/time discovery, deliberate local subset, supported acquisition, quality review, compatible-pair selection, retained analysis and human interpretation on additional areas. Show unsupported providers, clouds, alignment failure, empty discovery, zero change and retry states honestly. Reuse existing jobs; do not implement another synchronous raster pipeline. Depends on GAP-03/05/06 and later GAP-09/11.

### GAP-11 — Analyst deliverable · P1

**Assessment: confirmed output gap; reproducibility and review protections already exist.**

[ContextExport](../../apps/operator-ui/src/features/imagery/ContextExport.tsx#L43) downloads JSON. The [bundle contract](../../internal/contextpackage/domain.go#L76) is built around AOI, interval, assets, transformations, rights and approval. It is useful reproducibility infrastructure, but it does not constitute a question-led analytical brief.

[Review identity validation](../../internal/assessments/review_identity.go#L42) requires distinct resolved registered human identities. Preserve it. The development publication in the snapshot explicitly says it demonstrates account/workflow separation, not review by two actual people.

There is a further design constraint: current package rights use `AllowSimulationExport`. A readable brief, distribution of raw imagery and a future environment package are different output purposes. The brief must evaluate applicable output rights explicitly; neither renaming this flag nor copying all retained bytes establishes permission for a different use.

**Closure/proof:** a versioned readable brief with question, exact area/period, coverage, findings, cited evidence, maps/comparisons, uncertainty, limitations and review status, accompanied by a permitted reproducibility bundle. A recipient can understand it and resolve exact references. Define draft versus approved brief behavior and recipient access. Depends on investigation, scope, coverage and review contracts.

### GAP-12 — Development and working content · P1

**Assessment: confirmed default-catalog contamination; not evidence of partition failure.**

The [snapshot](../design/product-evaluation/installation-snapshot.json) contains 41 exercise-observation contexts, acceptance scenes and a published development workflow test. [Home](../../apps/operator-ui/src/routes/home.tsx#L9) and [readiness](../../internal/platform/readiness/http.go#L113) count authorized installed records. Imagery review lists are workspace-wide. Text in a conclusion or a scene name is not a reliable content classification.

Some synthetic indicators and explicit warnings already exist. They cannot substitute for a consistent purpose/provenance classification: real imagery used in a development acceptance test is still real source data, while its test publication is not an operationally reviewed conclusion.

**Closure:** classify content purpose separately from whether inputs are synthetic or real. Apply ordinary catalog/count defaults accordingly, with support/admin access to retained history. An append-only classification sidecar or equivalent versioned metadata can preserve immutable records without rewriting their meaning.

**Proof:** an ordinary analyst sees relevant source content and investigations; development publications and old exercise contexts do not inflate approved-output or usable-study counts. Counts distinguish source versions, distinct acquisitions, compatible studies and approved outputs. Coordinate with GAP-01/13 and retain exact historical identities.

### GAP-13 — Task readiness · P1

**Assessment: confirmed mismatch between platform readiness and analyst readiness.**

[Readiness](../../internal/platform/readiness/http.go#L113) marks a feature `AVAILABLE` when its scoped table count is positive; workers are `READY` based on recent heartbeats. It has no selected AOI, period or question parameter. A successful storage sentinel read does not validate every retained artifact. [SourceStatus](../../apps/operator-ui/src/features/public-context/SourceStatus.tsx) adds provider health and next actions, but is still source-oriented.

Consequently, four successful captures anywhere can make imagery available even when none can answer the current question. A healthy worker may have no permitted provider access or compatible inputs. Conversely, a quiet successful retrieval should not look broken.

**Closure:** keep operational readiness, but add task eligibility: evidence/coverage for the selected scope, supported operation, compatible dates/inputs, current rights and access, queue capacity, relevant worker/storage availability, and an understandable next action. Preserve distinct unavailable, unsupported, pending, stale, empty and ready states.

**Proof:** a healthy stack with no selected-area data; data present but processing unavailable; expired provider access; stalled work; a valid empty result; and recovery. The task summary must reflect each accurately without exposing private credentials. Depends on GAP-05/06/10 and the investigation question.

### GAP-14 — Wider workloads and unattended operation · P1

**Assessment: a qualification gap; recovery foundations have meaningful existing evidence.**

The [recovery acceptance](HISTORICAL_RECOVERY.md#recovery-acceptance) documents worker/lease recovery, storage failure, credential expiry, interrupted setup and paired restore with NDVI reproduction. Its performance measurements are explicitly a bounded exercise profile, including a virtual hour rather than a full wall-clock monitoring soak. They cannot establish regional map performance or multi-area provider scheduling.

Area-change handling is also partly implemented: [map queries](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L97) pass cancellation signals and key results by filters; [workspace switching](../../apps/operator-ui/src/components/WorkspaceProvider.tsx#L21) aborts requests and clears caches. Do not report these protections as absent. Hidden workbenches remain mounted and several queries poll, so navigation history can increase background workload; this needs measurement.

Concrete growth limits include four active captures and [a 1,000-row total capture budget](../../internal/imagery/captures.go#L166). Because the latter counts retained jobs rather than only pending work, reaching it stops new acquisition unless a deliberate archive/quota design is added. UI UUIDs also mean retrying an uncertain submission with a new key can create another job despite backend idempotency support.

**Closure/proof:** agree hardware, data density, concurrent analysts, area count, provider quotas, retention, latency and recovery targets; measure the map workflow against them. Cover rapid area changes, cancellation, lost responses, worker/storage/credential faults and restore of investigations plus artifacts. Begin budget and fault criteria early; final unattended qualification belongs to MAP-6.

### GAP-15 — International and role-focused usability · P1

**Assessment: partially supported, insufficiently qualified.**

Geographic aliases include Arabic source names, and the [geographic integration test](../../internal/integration/geographic_map_test.go#L46) includes Arabic search. Existing browser tests exercise keyboard geometry and narrow layouts. These are useful foundations, not evidence of a complete localized analyst workflow.

[The document language](../../apps/operator-ui/index.html#L6) is English. [Common time formatting](../../apps/operator-ui/src/lib/format.ts#L3) uses local browser time with no timezone label and sometimes no year, while [dateLabel](../../apps/operator-ui/src/features/workspace/preferences.ts#L17) omits time-of-day. The coordinate entry expects longitude/latitude, whereas other presentation helpers display latitude/longitude. The labels must make this ordering unmistakable across regions.

**Closure/proof:** test intended analyst and reviewer grants separately from the broad development account; Arabic/English aliases and mixed-script text; declared timezone/year; supported coordinate order, CRS/datum and units; keyboard/focus and readable narrow layouts. Missing permissions should explain available actions without presenting exercise setup or credential-file paths as routine analyst work.

Full Arabic application localization is a product decision distinct from Arabic place search. Do not quietly treat one as proof of the other. Wrong-area/time interpretation blocks acceptance even when the layout itself renders correctly.

### GAP-16 — Portable geographic environment preparation · P2

**Assessment: real preparation exists, but the implementation is tied to one area.**

[prepare_geography.py](../../scripts/data/prepare_geography.py#L157) uses a fixed DEM tile, Jeddah bounds and origin; it retains the geoid transformation and validates a round trip. [prepare_environment.py](../../scripts/data/prepare_environment.py#L12) uses those constants in texture coordinates, terrain lookup, output naming and manifest construction. It also generates actor-model fixtures at the end, which should not be part of a new environment-only task boundary.

The existing manifest already records source hashes, frame, input/output vertical datum and limitations. Preserve those facts. A mathematically successful round trip checks transform consistency, not terrain accuracy or suitability of a transformation grid for a different region.

**Closure:** an area-parameterized durable preparation request: permitted source selection → retained/cropped inputs → validated geographic artifacts and manifest. Resolve DEM coverage, local frame, horizontal/vertical transformations and area of use for the selected area; record estimated building properties, omitted geometry, dates/rights, size/LOD budgets and parent-byte lineage.

**Proof:** prepare another supported area without application-code edits; independently check georeferencing and transformation applicability, missing tiles, omissions, integrity and reusable output. This gate contains no actors, runtime clocks, synthetic sensors, exercise start or replay. Depends on trustworthy GAP-03/04/05/10 foundations.

### GAP-17 — Independent usefulness · P1

**Assessment: absent qualification evidence; no justified completion percentage.**

The inspected evidence comprises automated tests, developer walkthroughs and an account-separation workflow test. It does not establish independent target users completing the intended map-first task. For example, the [imagery browser journey](../../apps/operator-ui/tests/recovery/workspace-redesign.spec.ts#L67) confirms a prepared comparison, proposal and export screen, but does not assert that a newly selected country's geometry and period survive into a useful deliverable.

**Closure:** observe analysts and reviewers working from a stated question, without developer preparation beyond declared normal setup. Include multiple Middle Eastern and international areas, no-data/quiet states, denied access, a fresh supported acquisition, reopening a case, review and reading the resulting brief.

Record completion, assistance, scope errors, evidence/uncertainty comprehension and output usefulness against their present workflow. Two accounts operated by one developer do not demonstrate independent human review. Any wrong-area or wrong-period interpretation is blocking.

Start short formative sessions after the first corrected workflow. MAP-6 should qualify and close the phase, not provide the first encounter with real users. Exact participants and success thresholds require a product decision before claiming this gap closed.

### GAP-18 — Documentation and approval state · P0

**Assessment: partly corrected, still inconsistent.**

[Current scope](../../EMAD_CURRENT_SCOPE.md), [README](../../README.md#L3), [PRODUCT](../../PRODUCT.md#L3) and the [v2 plan header](../../EMAD_IMPLEMENTATION_PLAN_v2.md#L3) already establish the correction. It would be inaccurate to say no phase boundary has been recorded.

However, adjacent text still calls v2 the forward roadmap, describes a current two-pillar implementation, and carries broad completion language. The active product brief mostly remains the historical C2 document. Readers can follow conflicting statements without an explicit active backlog/status vocabulary.

**Closure:** a concise current map-first product brief and active backlog, with historical plans clearly subordinate. For each outcome distinguish implemented, installed, functionally verified, geographically covered and independently user-validated. Keep evidence dated and tied to its tested version/profile. A completed historical simulation increment must never generate the next current task automatically.

**Proof:** trace “what should we do next?” from README, product brief, implementation plan and delivery status; each leads to the same current phase and open acceptance gates. No deletion of old reports or retroactive reinterpretation of their evidence is necessary.

### Refinements to the proposed delivery sequence

Keep MAP-0 through MAP-6 as outcome gates, with these dependency adjustments:

1. **Contain the geographic substitution immediately within MAP-0/1.** Remove implicit unrelated-area behavior across export, capture and Systems together. Invalid/missing scope should stop or request a subset. This should not wait for complete global reference installation.
2. **Define area, period and investigation identity together in MAP-1.** Include exact-edition reads, temporal semantics, parent/subset relationships, permitted access, pinned drafts and a clear revision-change policy. A shared bbox helper alone will not close the issue.
3. **Separate reference expansion from acquisition authority in MAP-2.** More countries in the navigation catalog must not mutate connector admissions. Resolve multi-area profile identity and account-level quota sharing before enabling concurrent regional monitoring.
4. **Build a minimal persisted investigation during the first daily-workflow increment.** Its complete review/brief functionality can finish in MAP-4. Otherwise MAP-3 will keep calling browser filters an investigation and later require another workflow refactor.
5. **Treat time consistency, fixtures, accessibility and reliability as exit criteria at every gate.** Establish budgets and fault cases early, then run the final unattended profile at MAP-6. Begin independent formative testing before the final gate.
6. **Run MAP-5 only after geographic input contracts are stable.** Its output is a portable environment package. It does not reopen the deferred simulation sequence.

The core dependency is: **correct scope → explicit coverage/time meaning → durable investigation → repeatable acquisition/analysis → reviewed brief → qualified use**. Regional reference expansion supplies this flow; environment preparation reuses its geographic foundations.

### Implementation decisions to settle before broadening the backlog

- **Region configuration:** the initial versioned Middle East definition, its geographic meaning and source-backed membership/geometry; examples used for qualification remain samples rather than limits.
- **Time contract:** monitoring windows by source family, historical modes, timezone display and treatment of separately dated background imagery.
- **Scope persistence:** exact pinned geometry versus latest-version discovery, explicit subset selection and what happens when referenced geometry becomes inaccessible.
- **Investigation lifecycle:** ownership, collaborators/reviewer handoff, conflict handling, revisions, retention and recipient access to evidence/outputs.
- **Acquisition profiles:** independent area/time requests, durable idempotency, shared provider quotas, admission changes and permitted reuse of retained inputs.
- **Output purpose:** readable brief format, draft/approved behavior, permissible redistribution and its distinction from raw-data/environment export.
- **Qualification envelope:** target users, representative regions and conditions, data volumes, concurrency, latency/recovery limits and unattended duration.

These decisions constrain implementation; they do not require choosing a simulation engine or expanding runtime scope.

### First acceptance scenarios worth making authoritative

1. Select a country, open export and imagery, and verify that both retain the actual selection or demand an explicit local subset. Repeat for subdivision, named study, drawn polygon, viewport and restored case.
2. Save a case against geographic edition A, admit edition B, and reopen A with its original meaning. A later edit may deliberately choose B; the old output remains reproducible.
3. Change area/time while requests are slow and multiple workbenches have drafts. No stale response may masquerade as the new selection; pinned drafts remain clearly labeled and do not mutate silently.
4. Select an unobserved area with healthy workers. Report missing coverage, not readiness to answer the question and not a factual absence. Contrast with a successful zero-result retrieval whose limitations are stated.
5. Use monitoring and historical modes with late/corrected reports and old imagery. The user can explain which dates apply to each visible layer and to the brief.
6. Acquire supported imagery outside the prepared fixture, review quality, compare compatible inputs, save the investigation, hand it to an independent reviewer and produce a readable, cited result. Include a valid zero-change result and an unsuitable-input failure.
7. Exercise catalog pagination/budgets, dense regional discovery, date-line/high-latitude handling, provider quota/credential failure, worker loss and paired restore. Restore the investigation-to-evidence links as well as the bytes.

The immediate delivery target should be a correctly scoped, coverage-aware analyst journey that produces a durable result. New regions and larger workloads should qualify that same journey rather than introduce more prepared demonstrations.

<a id="implementation-plan"></a>

## Implementation plan: tasks, acceptance, traceability

**Prepared:** 25 October 2026.  
**Source baseline:** `a6d1d22f34692faea31db58e4a6318c539c3321a`.  
**Governing scope:** [EMAD current scope](../../EMAD_CURRENT_SCOPE.md).  
**Problem statements:** [18-gap report](#gap-report) and [code-backed analysis](#gap-analysis).\
**Status (updated 11 November 2026):** T01–T21 are implemented with scoped verification — see the [first-batch report](MAP_FIRST_DELIVERY_LOG.md#t01-t03), [T04–T06 report](MAP_FIRST_DELIVERY_LOG.md#t04-t06), [T07–T09 report](MAP_FIRST_DELIVERY_LOG.md#t07-t09), [T10–T12 report](MAP_FIRST_DELIVERY_LOG.md#t10-t12), [T13–T15 report](MAP_FIRST_DELIVERY_LOG.md#t13-t15), [T16–T18 report](MAP_FIRST_DELIVERY_LOG.md#t16-t18), [T19–T21 report](MAP_FIRST_DELIVERY_LOG.md#t19-t21) and [T22–T24 report](MAP_FIRST_DELIVERY_LOG.md#t22-t24). **T22–T24 remain IN_PROGRESS and the release is NOT_QUALIFIED**: the [29 October product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation) measured failed dense-workload latency budgets, an unresolved read-capacity blocker and no observed sessions. The status line recorded at preparation time was: *"T01–T12 are implemented … T13–T24 remain planned. … This work adds 00047–00050 and did not migrate/restart/deploy the shared installation."* Shared installation, geographic qualification and independent use retain separate evidence states; the full phase is not qualified. The forward work of the current preparation phase is tracked separately in [ENVIRONMENT_FOUNDATION_PLAN_2026_11_11.md](ENVIRONMENT_FOUNDATION_PLAN_2026_11_11.md).

**Navigation:** [Technical solutions](#2-technical-decisions-and-solutions) · [Migration strategy](#3-data-and-api-change-strategy) · [24 implementation tasks](#4-ordered-implementation-backlog) · [Acceptance scenarios](#5-acceptance-scenarios) · [Qualification budgets](#6-qualification-matrix-and-initial-budgets) · [All 18 gaps mapped](#7-gap-to-delivery-traceability).

**Second-pass audit:** [Plan hardening audit](#plan-audit) records the omissions found and their resolutions. The backlog now includes atomic requirements, hardening checks and acceptance status. Run `python scripts/ops/check_map_first_plan.py` to check these relationships. Passing that check establishes planning consistency, not implemented behavior or exhaustive correctness.

### 1. Intended result

An analyst can select an area and period, understand actual coverage, investigate a question using retained evidence, acquire supported additional inputs, record findings, hand the case to an independent reviewer, and produce a readable brief with exact citations and a permitted reproducibility bundle. The same workflow works across the Middle East and supported worldwide areas. Missing inputs and access have useful, truthful outcomes.

The map remains the principal working surface. Preserve the compact dock, Systems overview, Go modular domains, PostGIS, MapLibre, immutable evidence, scoped permissions, durable workers and private object storage. Current environment work produces reusable geographic artifacts only. Existing simulation implementation/history stays separately accessible; this plan adds no actors, runtime behavior, sensors, replay, engine selection or exercise milestones.

#### Delivery rules

- Correctness containment comes first. A missing or unsupported area must never become Jeddah or another previous selection.
- Resolve shared contracts once and use them across the map, Systems, jobs, investigations, review and output.
- Keep broad discovery separate from bounded acquisition/processing. Global selection does not authorize a global download.
- Preserve source bytes and historical migrations. Add new contracts, metadata and forward migrations without silently rewriting old evidence.
- Deliver small vertical increments, with each task's implementation, installation, functional checks, geographic qualification and user validation recorded separately.
- Numerical budgets and working defaults below are **proposed qualification targets**, not measured results or previously agreed service guarantees. Freeze the qualification profile before running the final gate; record any later revision and its reason.

### 2. Technical decisions and solutions

#### D01 — A single versioned area contract

Introduce an `AreaRevision` owned by a new small `internal/areas` domain. Keep `internal/reference` responsible for admitted reference datasets and geographic editions. The new domain owns analyst selections and resolution, not provider ingestion or account partitions.

An area revision contains:

- Identity, revision ID and schema/normalization version; resource scope; display name; kind (`world`, administrative, named study, drawn, viewport or composite).
- Exact WGS84 geometry or a resolvable immutable geometry artifact; hash of its canonical retained representation; source attribution and geographic meaning.
- For administrative selections: source namespace, logical region key, exact edition/feature ID and geographic release ID. Discovery can choose a newer release; saved work continues resolving its exact version.
- Derived bounds **per component**, geodesic area, and declared projection/display restrictions. Bounds are indexes and display aids, never replacements for polygon meaning.
- Parent area revision and explicit subset relationship when narrowing. Preserve the original request geometry and any documented normalization; do not silently simplify the analytical boundary.

`World` is an explicit discovery scope rather than an invented giant local AOI. A place point is a navigation target until the analyst draws/selects an area or deliberately creates a labeled buffer. A viewport becomes an area only through “Use this view”; camera movement otherwise changes presentation.

Resolving an exploratory scope is read-only and can return a validated transient fingerprint. Saving an investigation or submitting a job persists the exact area/period snapshot transactionally. Merely panning, searching or following a link does not create cases or acquisition work.

**Resolution changes:** make exact-ID geographic reads independent of latest-edition discovery. Namespace logical identity by source, and resolve parent/child hierarchy within a declared release. Add an atomic geographic release manifest referencing chunked editions so a partially imported update does not mix new and old subdivision data. Keep old IDs readable subject to current access.

**Geometry edge cases:** normalize a supported date-line-crossing selection into bounded Polygon/MultiPolygon components and retain one parent identity. Broad query counts deduplicate evidence IDs across components. Local raster jobs select one component/subset or an explicit bounded batch; they never use a near-worldwide rectangle across the date line. PostGIS provides split/wrap primitives, but the chosen normalization algorithm must be verified with holes, islands and seam-adjacent geometries. [PostGIS ST_WrapX](https://postgis.net/docs/ST_WrapX.html)

Retain geographic identity and tabular evidence access at high latitudes. Keep the initial map's ±85° display limit explicit, with an actionable explanation; do not claim a polar renderer. A limited display projection must not change retained geographic coordinates or silently discard an area's off-map part.

#### D02 — One period contract, several explicitly named clocks

Introduce a `TemporalScope` beside the area contract and combine them into a versioned `ScopeSnapshot`. It records mode, selected observation/acquisition interval, revision-view mode, as-of time, timezone preference and evaluation time.

- **Monitoring:** proposed default is the last 24 hours for event discovery. Actual freshness is source-specific and uses admitted policy/validity, not the 24-hour window. A weather report or moving observation does not become current merely by falling inside that window.
- **Historical study:** explicit fixed interval; retain `latest retained revision`, `known_at` and `effective_at` as separate supported interpretations. Unsupported family/history combinations return a typed unsupported result.
- Resolve rolling time **once per query batch on the server**. A saved monitoring preset can roll; a cited result, investigation checkpoint, submitted job and brief use a fixed evaluated interval and timestamp.
- For imagery, use acquisition time. For observations, use observation/validity time. Retain receipt, provider revision, publication and computation timestamps as distinct fields. Do not reuse “latest” to mean “live.”
- Adopt explicit start-inclusive/end-exclusive intervals in the new scope API. Legacy endpoints retain their existing semantics through a tested adapter; do not silently change old saved queries.
- Historical background imagery is opt-in during monitoring and visibly labeled with acquisition dates and “outside monitoring period” when applicable. It does not contribute to current-observation counts.

The UI shows timezone/offset and year where ambiguity is possible. UTC is always available. The default display timezone comes from the user's preference/browser, rather than assuming every region uses Riyadh time.

**Read consistency:** a fixed interval alone does not freeze the records in it. Introduce a bounded `ReadSnapshot` distinct from `ScopeSnapshot`, containing an authorized query fingerprint, stable result membership/order, exact revision IDs, coverage receipt revisions and an expiry. For initial delivery, materialize paged result manifests under one repeatable-read transaction with current authorization; retain only bounded rows/aggregates with explicit drill-down. Do not keep a database transaction open across HTTP pages. Related family summaries in one batch share this snapshot; independently refreshed layers show their own evaluation times. A timestamp cutoff or maximum sequence alone is insufficient because an earlier-started transaction can commit later. Keep the existing documented `known_at` meaning; do not relabel its publication timestamp as physical commit time. Recheck current access on every page; a changed authorization revision invalidates the snapshot instead of leaking or silently changing totals. Expired cursors explicitly restart the view. Saving/citing work promotes the selected exact references and necessary aggregate manifest to durable evidence; monitoring refresh creates a new snapshot.

#### D03 — One client scope owner and explicit pinned work

Create an `AnalystScopeProvider` and typed scope/query adapters. Replace independent operational reads of the `investigation` localStorage preference in `ContextExport`, Systems and capture. Browser preferences retain presentation settings; server cases retain analytical work.

Every map-family request, Systems action, capture form and export receives the same `ScopeSnapshot` or an explicit pinned case revision. Responses echo the resolved scope hash and query evaluation time. Include scope, authorization revision, source filters and temporal semantics in query keys. Accept results only for the active request generation.

Workbench behavior has two explicit modes: follow current map selection, or pinned investigation/draft. “Use current map scope” updates area **and period** after presenting the affected draft fields. Completed jobs, retained evidence and approved outputs never rebind. Changing geography clears invalid scene/pair choices and invalidates export previews.

Pass cancellation signals through all relevant API methods, including geography, scene/capture discovery and artifact previews. Preserve workspace-change abort/cache clearing. A hidden retained workbench may keep a draft, but suspends ordinary polling; a shared job subscription can monitor active work once. TanStack's cancellation behavior requires consuming its signal at the transport boundary. [TanStack Query cancellation](https://tanstack.com/query/latest/docs/framework/react/guides/query-cancellation)

Legacy URL/bookmark import resolves an exact region first, then drawn geometry, then an explicit bbox. Reject conflicting scopes. Invalid or inaccessible saved scope opens a recovery state with no operational submission; it does not reset to the world or another place. Offer deliberate “Start a new view.”

Authorization revisions are server-derived from effective source grants, resource access and case membership; a client cache key is not enforcement. On logout, identity/resource change or an access-denied refresh, cancel subscriptions and clear protected query caches, previews and blob URLs. Unsent draft recovery is opt-in, keyed to the canonical identity and resource, and never displayed to the next signed-in account. Preserve explicitly acknowledged server saves through these transitions.

#### D04 — Durable investigations and findings

Add `internal/investigations` with a mutable head pointing to immutable revisions, following the existing review-head pattern. Persist title/question, owner, members, exact scope snapshot, selected evidence references, notes, findings, analysis links, handoff state and output references.

Suggested tables: `investigations`, `investigation_revisions`, `investigation_members`, `investigation_evidence`, `investigation_findings`, `investigation_finding_revisions`, `investigation_review_events` and a required-source dependency projection. Use composite scope-qualified relationships. Notes/findings are authored content; source records remain immutable and are cited by ID, revision and hash where available.

Use expected head revision/ETag checks, returning a conflict without overwriting another edit. Debounced autosave reports saving/saved/conflict/error accurately and flushes on explicit save. Offer device-local unsent draft recovery only when clearly labeled and scoped to the current identity. Server save is the durability guarantee.

A case progresses through draft, active, review requested, changes requested, approved and archived. New edits create a new unapproved revision. Existing source-specific assessments can be linked as evidence; do not force a multisource finding into the existing single-source assessment schema.

**Access design:** membership never grants source permission. Use canonical registered human identity for ownership/contributor/reviewer identity and the current principal's live grants for every access. Add narrowly scoped investigation permissions and transaction-local principal/human identity binding to `db.BindTx`, backed by RLS and service checks. Workers receive explicit job-scoped access.

For the initial release, require access to **all sources contributing to the readable case and retained case history**. This avoids revealing restricted facts through titles, notes, derived maps or approved briefs. Adding a restricted source can make a case unavailable to a previous member; explain that generically without disclosing hidden identifiers. Retain source dependencies when evidence is removed from a later draft. A deliberately sanitized release is a separate future object, not automatic redaction of mixed-source prose.

**Lifecycle contract:** each case revision references exact finding revisions, never mutable finding heads. Authors may draft/edit/submit; an eligible assigned reviewer may request changes or approve the frozen revision; the owner may archive or transfer ownership to an eligible member. Reopening an archive creates a new unapproved revision. Edits during review supersede the pending request. Membership and ownership changes are audited, require live permissions, and do not rewrite authorship or prior approval. Membership alone does not make someone a contributor; content edits do. Removing the last owner requires an explicit eligible successor or administrative recovery, without granting administrators blanket source access. Archival is not evidence deletion. Case mutation, dependency registration and head comparison commit in one transaction so an accepted save cannot point to half-written evidence or findings.

Keep review events, output/job backlinks and current validity notices in append-only relations to a case revision, outside its authored content digest. Recording a brief produced from revision X must not create revision Y and invalidate that same brief automatically. Changes to the question, scope, selected evidence, notes or findings do create a new content revision. A new analysis result is linked operationally first and enters authored content only when deliberately adopted.

#### D05 — Configurable regional references

Create versioned region profiles with a human-readable definition, source-backed administrative members, optional explicitly drawn coastal/maritime components, source editions and hashes. A profile influences navigation and reference installation only; it grants no acquisition authority.

Proposed initial working preset: Gulf states, Yemen, Iraq, Iran, the Levant, Egypt, Türkiye and Cyprus, plus explicitly labeled relevant surrounding waters. Resolve members through the installed source's identities and names; preserve that dataset's boundary interpretation. Treat this as a configurable working overview, not a universal or jurisdictional definition of “Middle East.” Worldwide discovery remains available independently.

Parameterize `install_geography.py` and preparation inputs by profile/selected area. Retain country navigation, then expand subdivision/place depth across the configured region and international qualification samples. Use source IDs rather than row offsets for stable place keys. Preserve authoritative aliases as supplied; add normalized search indexes without altering displayed names. Show reference availability, scale/accuracy, edition and an import/request action.

Use chunked imports within existing admission size/feature budgets and activate only a fully validated release manifest. Separate administrative navigation from detailed vector/elevation datasets. Large roads/buildings imports run as bounded worker jobs; no whole-world detailed preload.

#### D06 — Several retrieval profiles under the same source identity

Keep provider/product `source_id` stable to preserve evidence identity and deduplication. Add named `retrieval_profiles`, immutable profile revisions pointing to approved `source_admissions`, and per-profile state for cursor, lease, retry, coverage and scheduling. Admission version numbers can remain monotonic per source; each profile head selects its own approved version.

Map each existing source state to a `default` profile with its exact current admission, footprint, checkpoint and quotas. Preserve immutable historical records. Associate historical admissions with that legacy profile without changing their bytes. Old source-only APIs address the default profile explicitly during compatibility; new APIs require profile identity where selection matters.

New profiles require reviewed area/time/product policy and explicit execution authorization for the intended service principal. Record grants to approved profile revisions, or an equivalent bounded versioned execution envelope. Source-level grants alone must not silently acquire authority over new geographic extents. Country selection only reads coverage and available actions.

**Required compatibility refactor:** replace `Artifacts.Admission(sourceID)` as the universal current-policy resolver. Artifact reservation, capture, imagery workers, context preview/freeze and download must resolve the input's original admission/profile plus its currently applicable approved rights and source suspension state. New reservations carry explicit profile/admission identity. Unknown or ambiguous mappings stop processing. No unrelated default profile may supply the rights for an artifact.

Retain evidence uniqueness by source/provider-record/provider-revision. Add retrieval-occurrence links from profile/run to an existing evidence ID, so overlapping profiles preserve successful retrieval/coverage history without duplicating evidence or independent-observation counts. A conflicting body for the same upstream revision is recorded as a provider inconsistency, not overwritten.

#### D07 — Durable requests and shared provider budgets

Add durable acquisition requests that reference an investigation/scope, exact profile revision, permitted purpose, interval, bounded parts and a client request key. An analyst can submit within an already approved execution envelope; requests beyond it become an administrative coverage request. No browser request accepts arbitrary provider URLs or credential paths.

A new opaque provider-account quota group in trusted connector configuration binds all profiles/endpoints using the same allowance. Reserve provider-specific request/credit cost atomically before network I/O; apply account-level concurrency/pacing, retry-after and outage backoff in addition to stricter profile limits. Unknown remaining credits are not zero or unlimited. Budget conservative unknown-cost requests according to the approved adapter policy.

Use the existing worker executables and lease patterns. Schedule profiles fairly, with bounded backfill separate from current polling. Expired/replaced leases cannot commit. Resume only the same frozen query/cursor. Keep an intent's idempotency key across retries and lost responses; use a deliberate new key only for a new acquisition. Cancellation stops unstarted parts and prevents stale completion; already retained bytes remain traceable.

Keep active-job and storage-byte budgets. Replace lifetime capture-row ceilings with paginated history and an explicit storage/retention policy. Do not delete retained history to regain queue capacity. When storage is full, pause new acquisition with a recovery action. Preserve stricter provider-specific limits until a separately reviewed policy changes them.

Persist quota reservations before network I/O with attempt identity, cost estimate, reset/window semantics and reconciliation status. If a process dies after sending but before recording the response, treat the charge as potentially spent until provider accounting or conservative expiry permits reconciliation; never refund an unknown call automatically. Idempotency binds resource, operation, authorized requester and canonical frozen input hash: a reused key with different input conflicts. Job creation, intent registration and any outbox/queue row commit atomically. Reserve estimated storage for concurrent work and reconcile actual bytes; reconcile orphan uploads without deleting referenced or still-retained objects. Use bounded retry/backoff with jitter and an operator-visible terminal state. These rules apply to acquisition, brief rendering and environment preparation as well as imagery.

#### D08 — Coverage and task readiness as a query result

Add `internal/coverage`, composing authorized profiles, receipts, reference releases, retained evidence, rights, job state and relevant worker/storage checks for a resolved scope. Keep `/platform/readiness` for operational health.

Do not compress independent facts into one green status. Per source/family return:

- Support/access: supported, unsupported, admitted, external prerequisite required; disclose only provider catalog information the user may see.
- Spatial relation: none, partial, full query-footprint overlap or unknown; observation completeness separately remains unknown unless justified.
- Temporal retrieval: requested/evaluated interval, completed and missing windows, latest successful retrieval and evidence time, source freshness.
- Data result: usable retained records, successful empty retrieval, no retrieval, partial retrieval, or unavailable query. Missing metadata is not proof of no observations.
- Task eligibility: can inspect, can request acquisition, needs smaller AOI, needs compatible dates, waiting/failed work, missing review, storage/capacity gate; typed next actions.

Coverage for old evidence uses historical profile receipts, not just today's admitted footprint. Union coverage over profile windows/components without double counting, and qualify approximation or unknown completeness. Permission-filter results before counts; never leak hidden source names, footprints or job IDs through readiness.

Maintain an adapter capability table for USGS, FIRMS, Earth Search, CDSE/STAC, METAR, GDACS and moving feeds, plus reference/DEM imports. Record supported discovery, retention, historical queries, capture/analysis, geometry/time limits, credentials and rights per installed adapter version. This is an inventory of supported operations, not a promise to add historical acquisition to every feed. A completed retrieval receipt records attempted geometry/window, pages, provider truncation, rejected records, publication success and gaps. Exhausted page budgets, rejected normalization or unpublished retained responses cannot establish complete coverage or a successful empty interval. Reference-release availability remains a separate dimension.

#### D09 — Useful map behavior and complete catalogs

Keep the current MapLibre canvas and Systems grid. Put area, mode, period and coverage in the main scope strip; route evidence selection, case creation, review and output through consistent actions. A read-only analyst, author and reviewer see role-appropriate starting actions without changing account authority when selecting a country.

Continue PostGIS aggregates and MVT. Use exact scoped counts and explicit aggregate/individual representation, with zoom-to-drill-down and server-paged records. Apply stable cursor pagination to geographic search, scenes, captures, jobs, findings and case lists; bind cursors to scope/query/access revision. All responses distinguish total, returned, representation and continuation. If an exact expensive count is pending, label it pending rather than silently approximating it.

Reference display geometry may be simplified or tile-clipped independently of exact selection geometry. MVT encoding can collapse geometry dimensions; that output must never become authoritative analytical geometry. [PostGIS ST_AsMVTGeom](https://postgis.net/docs/ST_AsMVTGeom.html)

Set query deadlines and response/geometry budgets; preserve the selected scope when offering a narrower view. Use scoped spatial/time indexes and inspect query plans before adding summary tables. Aggregates must retain revision/withdrawal/time and access semantics. Date-line component totals count one evidence record once.

Support Arabic/English aliases, mixed-script text isolation, local/UTC labels, explicit longitude/latitude fields and WGS84 entry first. Other coordinate/datum inputs either use a declared supported conversion with provenance or return an actionable unsupported state. Use existing Arabic font assets. Validate map-label shaping against the installed MapLibre **5.24.0** source; newer documentation describes different RTL behavior. Use a pinned, locally served compatible RTL solution if required, rather than assuming current upstream behavior applies or requiring an unrelated major upgrade. [MapLibre RTL API/version caveat](https://maplibre.org/maplibre-gl-js/docs/API/functions/setRTLTextPlugin/)

Full translation of all legacy screens is not necessary to qualify Arabic place search and mixed-script map work. Extract labels in new/current map workflows so later localization is practical; record English UI support and Arabic-search support separately.

#### D10 — Repeatable imagery from discovery through interpretation

Reuse existing scene retention, capture/preparation workers, analysis jobs, masks and exact comparison validation. Add area **and acquisition-time** filtering to scene/capture queries and pass the full scope to capture/compare.

Before submission show the selected investigation area, proposed subset and actual provider scene footprint together. Require a deliberate local rectangle inside the selected area and scene; if a rectangle cannot represent the polygon faithfully, offer a smaller inscribed rectangle or a supported masked-polygon operation, explaining the choice. Keep the current 0.1°-per-side capture bound initially. Never prefill another area when a selected country is too large.

Only offer automatic capture for supported/admitted provider paths; a CDSE catalog result does not imply the Earth Search raster path can capture it. The durable discovery request must retrieve suitable metadata for the requested area/period without developer file preparation. Supported imports remain a secondary explicit action.

The pair chooser first filters by area/subset and acquisition order, then reports existing grid/platform/tile/orbit/baseline/time compatibility rules and clear-pixel quality. Do not silently relax those scientific constraints. Unsupported alignment requires a separately versioned, validated preprocessing method; initial delivery can honestly request another compatible pair. Valid zero-change analysis remains successful and can produce a finding with appropriate limits.

#### D11 — Independent review and readable outputs

Findings cite exact immutable evidence, analyses and assessment revisions. A review binds the exact case/finding revision and content manifest. Preserve the existing canonical-human protections; a reviewer/publisher must be distinct from every contributing human author of that reviewed content. Services cannot perform human review. Reviewer edits become a new authored revision that needs another independent reviewer. Account aliases do not establish independence.

Add purpose-specific rights decisions for internal analytical use, citation, included imagery, raw-data distribution and geographic-environment export. Preserve old `AllowSimulationExport` semantics for legacy bundles; do not reinterpret it as general brief-distribution permission. Unknown permission blocks the affected inclusion/export with a specific administrative action. Rights remain source-backed and time/versioned; this plan itself grants none.

Existing artifacts have one `source_id` and an imagery-specific kind/media allowlist. Add a composite derived-output manifest/storage path with all source dependencies and investigation ACL, rather than assigning a multisource brief to an arbitrary source. Reuse private object storage and integrity checks. Check current access, retention and applicable rights before and after external object I/O.

The first readable deliverable is an authenticated HTML brief plus a self-contained downloadable HTML package where included content may be retained/distributed. It contains question, area map, period/mode, coverage, findings, citations, comparisons, uncertainty, omissions, review status and reproduction manifest. Provide print styles for a practical human deliverable. PDF can be added through an isolated renderer using the same frozen manifest; it is not required to invent a new report data model.

Freeze case/evidence/scope, coverage-evaluation timestamp, chart/map inputs, style/renderer versions and allowed assets. Generate retained map/comparison images from those inputs; no hidden live-map screenshot or unreviewed third-party tile fetch during rendering. Render outside synchronous API requests with leases, cancellation and durable attempts. A changed case invalidates the draft's approval eligibility; an existing approved output remains pinned and is labeled superseded when appropriate. Output approval and rights to distribute are separate checks.

**Review/render protocol:** build the frozen content manifest and render a draft first. Reviewers inspect those exact figures, text and omissions. Approval records the content digest, case revision, contributor set and renderer/assets, using an expected-revision transaction. The final approval label is a separate attestation referencing the reviewed digest; adding that label cannot trigger new figure generation. Renderer/input changes create a new unapproved output. Before rendering, committing, approving and serving, recheck dependencies and rights; a race with edits, revocation or expiry stops eligibility. Authorize draft reading separately from approved distribution.

Render source text, names and analyst notes as escaped text or a deliberately restricted markup subset. The downloaded HTML is script-free, contains no active forms or embedded executables, and uses validated links and bounded local assets. The authenticated reader uses the normal protected application shell. Test script/URL injection and path traversal in included asset names; prohibit undeclared renderer network access. Retain visible legends, units, acquisition dates and source attribution in every frozen figure, including mixed-script content.

#### D12 — Portable environment preparation only

Parameterize geographic preparation by area revision, supported input source versions, horizontal CRS, input/output vertical datum, local origin and declared size/LOD limits. Reuse Python geospatial tooling and retained-parent storage. Separate fixture-model generation from the geographic preparation entry point.

A durable environment job resolves source coverage/rights, acquires bounded DEM/imagery/vectors, crops/reprojects with explicit transformations, validates, and publishes an immutable geographic package. Retain original bytes, normalized data, source dates/rights, transformation grids and hashes, grid area of use, estimated heights/widths, omissions and processing versions.

For a rectangular raster/grid, record nodata/mask outside the actual selected polygon. Do not represent omitted buildings/roads as complete coverage. Validate vertical conversion applicability with independent control values where available; round-trip success alone is insufficient. Output a neutral manifest plus permitted terrain/vector/imagery files and optional visual GLB, all georeferenced. Environment generation does not require an exercise partition or actor-model worker.

#### D13 — Purpose classification and map-only operation

Add append-only content-purpose metadata with `working`, `development`, `acceptance` and `unclassified`, separate from real/synthetic provenance and from review state. Initial classification uses verified installation receipts and known record IDs. Do not infer purpose solely from a name or conclusion string. Ordinary catalogs exclude known development/acceptance items, expose an unclassified-content review action, and do not count unclassified publications as approved working outputs. Support views retain access to original history.

Add explicit local and staging `map`/`historical-full` startup profiles. The map profile requires API/database/object store and enabled public connectors/imagery services, without simulation credentials, runtime descriptors, asset/recording workers or Studio onboarding. Preserve old schema/history and the existing isolation rules. Existing running services are not stopped by a documentation change; the new profile applies on a deliberate subsequent startup.

#### D14 — Evidence meaning and later corrections

Carry explicit source/reference fact, provider report, derived indicator and human finding/assessment labels through map inspection, aggregates, cases and briefs. Preserve provider quality separately from analytical confidence, geometry roles separately from display points/buffers, and measured values with units/methods. A thermal report is not automatically a confirmed fire; a derived change indicator is not a human-approved conclusion. Keep conflicting reports visible with their separate provenance instead of silently merging them into certainty.

Track reverse dependencies from evidence revisions to findings, cases and outputs. A provider correction, withdrawal, integrity failure or loss of required inputs creates a current validity notice on affected work. Check that status on open and before review/export; use a bounded repairable projection rather than assuming a missed notification is harmless. Keep the original reviewed content and historical approval event immutable. The current view distinguishes “approved at revision X” from “support changed; reassessment needed”; new distribution requiring current support blocks until reviewed again. A historical export may state the original cutoff and current limitation explicitly under its permitted purpose. Never substitute today's evidence into yesterday's approved figure.

#### D15 — Retention and reproducibility boundaries

Preserve immutable identity/provenance and original bytes for their permitted retention, not indefinitely by assertion. Model each dependency's availability, allowed retention and derived-output disposition. Use the strictest applicable inclusion/retention rule unless a source-backed grant expressly permits a longer-lived derived output. Warn about approaching expiry when it affects an active case; never extend a source retention date just because a case cites it.

Reuse the existing artifact deletion states for permitted expiry/deletion and extend them to composite assets. Preserve authorized tombstones, hashes, dependency links and a typed unavailable reason; failed deletion retries safely. Distinguish expired/deleted, access denied, corrupt and temporarily unavailable inputs. Metadata retention itself follows applicable policy. Archived cases and approved outputs may become only partially reproducible; show that limitation and block unsupported re-analysis or regeneration. Restore verification treats a policy-valid tombstone as expected, while a missing unexpired required object is a failure. Do not restore expired bytes into ordinary readable service or overwrite a historical output to conceal loss.

### 3. Data and API change strategy

These are proposed names to make the work concrete. Final migration numbers are allocated in merge order **after 00044**; do not reserve or edit existing migrations.

| Additive migration group | Main records | Compatibility rule |
| --- | --- | --- |
| Area/release | Area identities/revisions, region profiles, geographic release manifests/heads | Old feature IDs and edition bytes remain resolvable; no automatic geometry upgrade |
| Classification | Purpose revisions and current-purpose projection | No mutation of original evidence/conclusion text |
| Investigation | Cases, revisions, members, evidence links, findings, review events, source dependencies | Browser bookmarks import as view presets or explicit new drafts; never invent a question or review |
| Retrieval | Profile heads/revisions/state, execution grants, requests/attempts/occurrences, shared quota ledger | Existing source becomes one exact default profile; no footprint/grant expansion |
| Outputs | Purpose rights decisions, composite output manifests/jobs/reviews | Legacy context bundles retain schema and rights behavior; multisource output has all dependencies |
| Environment | Preparation requests/attempts/manifests and composite artifacts | Old Jeddah packages remain historical and reproducible |
| Read/validity lifecycle | Bounded read snapshots, exact result manifests, reverse dependencies, validity notices and authorized tombstones | Existing time meanings and retained identities remain intact; ephemeral read snapshots confer no durable access |

Proposed API additions, under the current `/api/v1` base:

- Implemented in T04–T06: `POST /analyst-scope/resolve` → validated area/period snapshot and operation capabilities; `GET /areas/revisions/{id}` → exact authorized revision. `POST /analyst-scope/freeze` transactionally persists a scope; existing job/export submissions use the same service inside their own transaction.
- Implemented in T05: `POST /analyst-scope/read` and `GET /analyst-scope/reads/{id}` share materialized membership across family summaries/pages and publish expiry, query fingerprint and evaluation time. Initial bounds: five minutes, 16 active manifests and 64 MiB combined per principal/resource, 8 MiB per manifest, at most 5,000 exact event revisions. These are correctness/storage bounds; later workload qualification remains required.
- Implemented in T04: `GET /geography/regions?id={exactId}` bypasses latest discovery. Release/source/edition selectors and permission-gated `/geography/releases` and `/geography/profiles` pin complete hierarchies. Expanded search/cursor installation remains T08.
- Investigation create/list/read, conditional revision update, evidence attachment, finding revisions, member/handoff and review-action routes under `/investigations`.
- `POST /coverage/query` → per-source/family coverage and task eligibility for a resolved scope; never changes ingestion.
- Profile administration under `/sources/{id}/profiles`; durable `/acquisition-requests` for approved requests and separately recorded requests for expanded coverage.
- Existing imagery endpoints gain explicit scope/profile/admission and time-aware discovery through backward-compatible fields or a versioned request schema. Capture history gains cursor/total metadata.
- `/brief-jobs` and `/environment-preparation-jobs` create frozen durable work; output download resolves the full dependency/rights checks.

For each route, update OpenAPI, generated TypeScript, permission catalogs, route-policy classification, audit redaction and scoped query helpers in the same implementation increment. New public workflows must not obtain an implicit internal-projection bypass. API schema presence alone is not permission provisioning or installed data readiness.

#### Upgrade and rollback procedure

1. Record baseline versions/counts/hashes and create a verified paired database/object backup using existing operations tooling. Quiesce all relevant writers, including new render/environment workers and retention cleanup, and verify the quiescence; checking API ports alone does not establish it. Record the maintenance interval and failure recovery procedure.
2. Apply additive migrations, backfill derived mappings/classification from verified records, and verify scoped relationships. Unknown mappings remain explicitly unresolved.
3. Deploy readers compatible with old/new contracts behind feature flags. Keep new writers disabled until compatibility tests pass.
4. For retrieval cutover, drain or fence legacy leases, copy default-profile state transactionally, switch one scheduler owner, then enable new profile writers. Old and new workers must not both own the same source/profile.
5. Validate old IDs, artifacts, context packages, bookmarks and source permissions after restart. Enable each new flow only after its acceptance slice passes.
6. Roll back UI/features and disable new writers while retaining additive schema and immutable history. A rollback binary must either understand the new policy/state or run read-only with affected workers disabled; never fall back to an old source-default rights decision. Use forward fixes or isolated restore rehearsal for data faults, not destructive down migrations. Rehearse the actual previous binary against the migrated schema with new-profile records present, including disabled feature flags, rollback, forward recovery and read-only behavior.

### 4. Ordered implementation backlog

Task details, acceptance scenarios and complete gap traceability follow below. Tasks may be split into small commits/PRs, but a contract migration must keep schema, authorization, API and compatibility behavior together. No task is complete merely because its code compiles.

#### T01 — Contain wrong-area submissions immediately

**Gate:** MAP-1, urgent containment before broader refactoring. **Depends on:** none. **Gaps:** 02, 03, 10. **Acceptance:** A02.

Change `ContextExport`, `CaptureWorkbench`, Systems and their callers to use explicit selected/pinned scope inputs. Remove operational Jeddah fallbacks. An absent, unresolved, too-large or unrepresentable selection disables submission and provides a local-subset action. Invalidate incompatible capture scenes/pairs and previews on scope change. Preserve intentionally pinned completed inputs and make their scope visible.

Add regression tests for the recorded Saudi-country → Jeddah-export failure and the large-area → old-capture-AOI failure. This task prevents substitution; complete versioned scope persistence arrives in T04–T07. It can ship before the new data model.

#### T02 — Map-only startup and ordinary entry

**Gate:** MAP-0. **Depends on:** none. **Gaps:** 01, 08, 18. **Acceptance:** A01.

Implement D13 in `scripts/ops/local.py`, `apps/emad-local`, `Taskfile.yml`, staging composition and readiness expectations. Audit `internal/app/app.go` and server configuration for profile-specific initialization, background activity and clock/readiness dependencies; the public profile uses wall time and does not activate exercise execution. Provision a map analyst without exercise grants. Do not require simulation token files, binary description or worker startup for the map profile. Keep full historical operation explicitly selectable.

Update Home, default-resource restoration, header navigation and the map chooser so normal startup opens Public Context. Test fresh setup and an upgraded populated installation. A saved exercise resource must not override the selected map profile. Preserve database isolation and historical routes; no data deletion or live worker shutdown belongs to this change.

Include packaging, Docker image entry points, readiness dashboards, `scripts/ops/check_delivery_package.py` and `scripts/ops/paired_backup.py` in the profile inventory. The existing package checker explicitly invokes runtime/asset/recording binaries, so changing startup alone cannot close this task. Qualify the declared Windows local and Linux staging delivery paths, dependencies, pinned fonts/styles/RTL assets and installation/repair instructions; mark any other platform unqualified. Optional external basemap failure must leave selected geometry, evidence lists and source attribution usable with a visible degraded state.

#### T03 — Classify content and correct visible counts

**Gate:** MAP-0. **Depends on:** none. **Gaps:** 12, 18. **Acceptance:** A14.

Add purpose metadata and normal/admin catalog filtering under D13. Inventory verified fixture/test IDs from installation receipts and acceptance evidence; backfill classifications without changing source/review bytes. Distinguish catalog revisions, distinct acquisitions, compatible studies and reviewed working outputs in Home/Systems/readiness. Unclassified content remains actionable, not silently promoted or destroyed.

Add the current map-first brief/backlog links to active documentation while preserving historical status records. Later T24 records final qualification rather than reusing historical completion labels.

#### T04 — Versioned areas, exact geography and component geometry

**Gate:** MAP-1. **Depends on:** T01. **Gaps:** 02, 03, 04, 07. **Acceptance:** A02, A03, A08.

Implement D01 in the area domain, geographic resolution API, schema and OpenAPI. Split exact retained reads from latest discovery. Add source-qualified logical identity, pinned releases/parents, region-profile manifests and explicit world/point/area distinctions. Implement and verify date-line component normalization and high-latitude display capability results.

Separate canonical selection geometry from display simplification and local processing budgets. Exact area revisions are immutable. Tests cover replacing a geography edition, malformed/conflicting scopes, multipart islands, holes, source restrictions, date-line counts and deliberate subsets.

#### T05 — Shared temporal scope and query evaluation

**Gate:** MAP-1. **Depends on:** none. **Gaps:** 03, 06, 15. **Acceptance:** A04, A22.

Implement D02 in a shared Go contract, API validation and TypeScript types/adapters. Preserve known-at/effective-at distinctions and legacy interval compatibility. Define operation/family support explicitly; use one server-evaluated time per monitoring query batch. Include source-validity/freshness metadata without changing observation timestamps.

Test late arrivals, withdrawals, year/timezone boundaries, monitoring preset versus frozen result, and unsupported history. A period must be serializable into investigations and jobs before new consumers are built.

Implement the D02 read-snapshot contract and bounded storage/expiry policy, including publication races. Materialized membership and stable sort order are part of the API contract before cursor consumers are enabled.

#### T06 — Connect every user-facing handoff

**Gate:** MAP-1. **Depends on:** T04, T05. **Gaps:** 02, 03, 06, 08. **Acceptance:** A02, A04, A09.

Implement D03 across `PublicAtlasWorkspace`, `WorkbenchContext`, `MapWorkbenches`, `usePublicContext`, Systems, imagery/review/export callers, URL parsing and bookmarks. Use one active scope; support deliberate pinned drafts. Remove obsolete storage reads and invalid-scope fallthrough. Pass request abort signals and scope hashes through the API wrapper.

Browser tests traverse map → Systems → imagery → review/export and back/forward for all area forms and periods, including a scope changed while an old response is delayed. Legacy links either resolve faithfully or show a recovery action.

#### T07 — Minimal durable investigation

**Gate:** MAP-1 foundation / MAP-4 early delivery. **Depends on:** T04, T05. **Gaps:** 09, 03, 08. **Acceptance:** A10, A12, A23.

Implement case identity, question, pinned scope, ownership/membership, revision heads and evidence references from D04. Add permission/route policy and transaction-local identity binding with RLS tests. Save, list, reopen and resume from another authorized browser session. Handle revision conflicts and source-access loss without leaking case contents.

Keep this slice small: it establishes actual persisted work before the daily workflow expands. Rich findings, review and briefs follow in T16–T19. Existing bookmarks remain view presets until explicitly turned into a case.

#### T08 — Generalized regional reference installation

**Gate:** MAP-2. **Depends on:** T04. **Gaps:** 02, 04, 15. **Acceptance:** A08, A15.

Implement D05 in `install_geography.py`, reference preparation and region-profile configuration. Install the proposed regional depth through source-backed, resumable chunked imports and activate complete releases atomically. Add country/subdivision/place search pagination, aliases, unavailable-depth labels and request/import actions.

Exercise the same machinery on at least four Middle Eastern country selections and three international sample areas. No new country-specific branches in application code. A second source's feature with a similar name/key must not overwrite another source's logical identity.

#### T09 — Retrieval-profile model and rights compatibility

**Gate:** MAP-2. **Depends on:** T04, T05. **Gaps:** 05, 13, 14. **Acceptance:** A05, A06, A18.

Implement D06 in public ingestion, source admissions and artifact/imagery/context rights resolution. Split into reviewed increments: additive profile records and exact legacy backfill; explicit profile-aware policy resolver; default-profile API compatibility; fenced scheduler cutover. Keep old and new policy enforcement consistent during rollout.

Enumerate every caller of `Artifacts.Admission` and every source-state join. Tests prove that an artifact from profile A cannot obtain profile B's rights, a disabled source/profile cannot complete new processing, old immutable admissions remain readable, and new profiles receive no execution grant automatically.

#### T10 — Durable acquisition scheduling and provider quotas

**Gate:** MAP-2. **Depends on:** T09. **Gaps:** 05, 10, 13, 14. **Acceptance:** A05, A06, A17.

Implement D07 using existing connectors and job/lease patterns. Add explicit acquisition/coverage-request APIs, stable intent keys, profile scheduling, shared provider-account budgets, retrieval occurrence links, cancellation and actionable retry status. Bind all provider requests to trusted connector configuration.

Test two overlapping profiles and two sources sharing a provider account: independent cursors/coverage, deduplicated evidence, shared quota/pacing and no stale-lease commits. Simulate a response lost after successful submission and verify retry returns the same request/job. Add active/storage reservations and uncertain-charge reconciliation here; keep the existing lifetime history cap until T13 installs paging and activates its safe replacement. Ship queue age, quota, attempt, storage and coverage metrics with these workers, before final soak qualification.

#### T11 — Area/time coverage and task readiness

**Gate:** MAP-2. **Depends on:** T03, T06, T08, T10. **Gaps:** 05, 13, 06. **Acceptance:** A07, A14.

Implement D08, then expose it in the map scope strip, Systems and relevant acquisition/export actions. Keep operational worker health separately inspectable. Return typed next actions and distinguish support, permission, spatial overlap, retrieval completeness, freshness and data result.

Test empty versus unqueried, partial versus full query footprint, stale versus unavailable, historical receipt versus current admission, gated credentials and healthy workers with unusable input. No UI should infer task readiness from a workspace-wide count.

Implement the adapter capability/receipt matrix in D08. A capped provider response, rejected record or retained-but-unpublished page must not become a complete or successfully empty interval. Correlate UI next actions with authorized durable request IDs and a recorded recovery owner.

#### T12 — Monitoring, historical study and useful quiet states

**Gate:** MAP-3. **Depends on:** T05, T06, T11. **Gaps:** 06, 08, 13. **Acceptance:** A04, A07, A21.

Implement the two visible temporal modes and their family-specific adapters in map/event/context/track/imagery queries. Apply acquisition-time filters to study imagery; separately label opt-in background context. Show coverage-aware quiet summaries without claiming factual absence. Keep current-only feeds unavailable when the historical query cannot be answered.

Verify that source failure does not replace successful sibling results with zeros, and that frozen historical results do not refresh into a different meaning. Make the evaluated interval visible in evidence inspection and saved investigation checkpoints.

Preserve the D14 evidence/quality/geometry distinctions in every layer, inspector and aggregate. Connect corrected/withdrawn evidence to a visible validity status without turning a provider report into an independently established fact.

#### T13 — Scale, paging and request lifecycle

**Gate:** MAP-3. **Depends on:** T04, T05, T06, T10. **Gaps:** 07, 14. **Acceptance:** A09, A17, A22.

Implement D09 catalog pagination, complete counts/aggregate semantics, query deadlines, scale-dependent reference display and density-aware drill-down. Inspect PostGIS execution plans at the proposed loads. Preserve exact history/revision selection in all aggregate paths.

Suspend hidden-workbench polling, deduplicate job monitoring and abort obsolete reads. Make storage/processing limits actionable. Test more than 300 geography results, 100 capture results and 2,000 map records, plus date-line duplicates, multipart geometry, mixed densities and rapid scope changes. Test acquisition beyond 1,000 retained historical capture rows using isolated data without bypassing active/storage quotas.

Cursor pages, counts and drill-down use D02 read snapshots. Test a late commit, correction and grant change between pages. T10's active/storage guards must be installed before removing the lifetime cap; this dependency is a completion gate, while query profiling can start earlier.

#### T14 — Analyst navigation, roles and international usability

**Gate:** MAP-3. **Depends on:** T02, T06, T07, T11. **Gaps:** 08, 15. **Acceptance:** A15, A19 formative.

Refine the existing map/dock/Systems around evidence, investigations, review and outputs. Add role-led entry points and progressively disclosed technical details. Implement explicit timezone/year/coordinate labels, normalized source-backed Arabic/English search and mixed-script rendering. Verify installed-version map-label shaping and locally packaged fonts/assets.

Exercise keyboard/focus, contrast, 320/390px widths, short landscape, zoomed text and screen-reader names for the current workflows. Use separate analyst, reviewer, source administrator and restricted-read accounts; do not rely on the broad development identity. Begin observed user sessions here and feed defects back into the active tasks.

#### T15 — Repeatable imagery acquisition and comparison

**Gate:** MAP-4. **Depends on:** T06, T07, T10, T11, T12, T13. **Gaps:** 10, 03, 13. **Acceptance:** A11.

Implement D10 in discovery/capture UI, scene/capture queries and workers. Prefill only supported subsets from the actual scope. Persist request keys, link jobs to case revisions, filter compatible capture pairs and expose quality limitations before analysis. Acquisition remains provider-specific and admission-controlled.

Complete fresh supported acquisitions for an additional Middle Eastern area and an international area. Include cloud/mask rejection, partial scene coverage, alignment mismatch, failed/retried capture, cancellation and valid zero change. A pipeline fixture is useful technical evidence but does not count as fresh real acquisition.

#### T16 — Complete case evidence, notes and findings

**Gate:** MAP-4. **Depends on:** T07, T12. **Gaps:** 09, 08, 11. **Acceptance:** A10, A12, A21, A23.

Complete D04 with evidence selection, notes, multisource findings, analysis links, revision comparison and durable autosave/recovery. Resolve exact retained revisions instead of today's latest objects. Derive required-source dependencies for human-written content as well as images and jobs.

Test interrupted save/reopen, two editors, evidence expiry/access revocation and removed-but-historically-cited evidence. Raw evidence is never edited through a case form. A changed scope creates a new case revision and flags incompatible analysis rather than relabeling it.

Implement D04 lifecycle permissions and exact finding revision links, D14 reverse-dependency validity notices, and D15 unavailable-input handling. Test owner departure/transfer, archive/reopen and case access after a required source becomes unavailable; permissions never arise from a recovery action.

#### T17 — Handoff and independent review

**Gate:** MAP-4. **Depends on:** T16. **Gaps:** 09, 11, 17. **Acceptance:** A12, A19, A24.

Implement reviewer assignment/inbox, review-requested/changes-requested/approved events and the exact-revision checks in D11. Require live case/source permissions and canonical human separation from all contributors. Add an explicit disagreement/rework loop and preserve prior review history.

Test same-human aliases, service identities, reviewer edits, stale expected revision, revoked grants and a real independent-review session. Automated accounts demonstrate enforcement only; do not label that as independent human validation.

#### T18 — Output rights and composite artifact foundations

**Gate:** MAP-4. **Depends on:** T09, T16. **Gaps:** 11, 12, 16. **Acceptance:** A13, A18, A23, A24.

Implement purpose-specific rights decisions and composite output manifests/access under D11. Support authorized references when embedded bytes cannot be included, clearly declaring external dependencies. Reuse private object storage while adding appropriate MIME/kind handling and all-source dependency checks.

Test restricted source mixtures, expired retention, source suspension during I/O, unclassified/development findings, and legacy bundle compatibility. Fail closed on an unknown output permission. Do not fabricate review, redistribute raw assets implicitly or change old `AllowSimulationExport` meaning.

Implement D15 retention/tombstone rules for composite outputs and the D11 review/render attestation contract. Content-purpose reclassification is audited and never converts a development publication into independently reviewed working content; a working successor needs its own permitted review and retains its lineage.

#### T19 — Readable, reproducible analyst briefs

**Gate:** MAP-4. **Depends on:** T11, T15, T17, T18. **Gaps:** 11, 10, 09. **Acceptance:** A13, A19, A21, A24.

Build frozen brief manifests, durable rendering, reader view, printable HTML and permitted self-contained download. Include exact citations and retained maps/comparisons, coverage, omissions and review state. Offer the compatible machine-readable evidence bundle alongside it.

Review and export bind the same frozen content. Render changes affecting content/figures require renewed review; approval metadata must remain tied to its reviewed manifest. Test a recipient opening the result without developer assistance, with and without online evidence access, and verify no live fetch silently changes a historical output.

Execute the draft-render → exact-content review → attestation → distribution protocol. Include changed evidence, injected markup, unsafe asset paths, renderer failure and approval/edit races. The review service T17 exposes manifest-bound review operations; this task integrates and qualifies the rendered output.

#### T20 — Generalize geographic preparation inputs

**Gate:** MAP-5. **Depends on:** T04, T08, T10, T18. **Gaps:** 16, 02, 04. **Acceptance:** A16.

Refactor geography/environment scripts into area-parameterized operations under D12. Resolve DEM tiles, imagery, permitted vectors, local frame and transformations from frozen inputs. Separate fixture model generation, fixed output names and Jeddah UV/terrain indexing from reusable preparation.

Preserve original source bytes, hashes, source rights/dates and transformation lineage. Verify source coverage and vertical-grid area of use before conversion. Retain the existing Jeddah path as an explicit regression fixture.

#### T21 — Durable portable environment package

**Gate:** MAP-5. **Depends on:** T20. **Gaps:** 16, 14. **Acceptance:** A16, A17.

Add environment preparation requests/leases/attempts, bounded resource estimation and validation. Publish georeferenced terrain/vector/imagery artifacts, neutral manifest and optional visual GLB through composite output storage. Validate no-data masks, approximations, source omissions, geometry/LOD/byte budgets and reproducibility.

Prepare another supported area through the product flow without application-code edits. Validate in an independent reader or coordinate-check harness. No actor setup, runtime start or replay is part of this task's test or definition of done.

Before processing, record a qualification profile with output horizontal/vertical tolerances, control-point method and provenance, source resolution/uncertainty, pixels/features/triangles/bytes, temporary disk/RAM limits and maximum attempt duration. Derive tolerances from admitted source accuracy and intended geographic use, not from successful round trips. Reject unsupported CRS/grid area-of-use, missing required DEM tiles or impossible resource estimates before consuming a full job; retained failures explain omissions. A visually plausible mesh alone cannot pass this task.

#### T22 — Workload, unattended operation and recovery qualification

**Gate:** MAP-6; harness/budget work begins during earlier tasks. **Depends on:** T02, T10, T13, T15, T19, T21. **Gaps:** 14, 07, 13. **Acceptance:** A09, A17, A18, A22, A23, A24.

Create isolated synthetic load profiles clearly classified as acceptance content. Measure the agreed map/query/render budgets, simultaneous users and multi-profile schedules. Run the unattended wall-clock monitoring profile with queue, memory, storage and freshness telemetry.

Rehearse restart, worker loss, stale leases, storage loss, quota/credential exhaustion, lost responses, source revocation and paired restore. Verify investigations, geographic releases, profile mappings, canonical review identity, briefs and environment dependencies as well as object hashes. Record hardware, dataset size and cold/warm conditions. Fix failed tasks; do not convert failures into looser unlabeled limits.

Extend the actual backup/restore tooling for every new writer/object family and authorized tombstones; rehearse deletion/expiry interruptions and real rollback compatibility. Use metrics introduced with the affected tasks, with correlation IDs and bounded labels that exclude case prose and credentials. Publish per-operation request count, error/timeout rate and p50/p95/p99; an average or a success-only percentile cannot satisfy a latency gate.

#### T23 — Independent analyst/reviewer qualification

**Gate:** MAP-6, with early sessions after T14. **Depends on:** T14, T19, T21, T22. **Gaps:** 17, 15, 08, 11. **Acceptance:** A19, A20.

Observe target users on the geographic/state matrix below. Compare completion, assistance and result usefulness with their present workflow. Require correct area/period and coverage/uncertainty explanations. Include a reviewer distinct from case contributors.

Record observed task outcomes, corrections and retest evidence. Participant availability is an external gate; automation cannot substitute for it. Early formative sessions can proceed before every late task is finished, but the final qualification uses the complete declared release candidate.

#### T24 — Active product documentation and release evidence

**Gate:** MAP-0 initial direction / MAP-6 final closure. **Depends on:** T03, T22, T23. **Gaps:** 18, 01, 12, 17. **Acceptance:** A20.

Reconcile README, PRODUCT, current scope links, operations and the active backlog around the map-first phase. Clearly label old plans as historical. Document supported map/coordinate/time behavior, installed geographic/source profiles, known gates and the map-only startup path.

Publish a release ledger linking each gap/task to exact source revision, installation profile and evidence. Distinguish implementation, installation, functional verification, geographic coverage and user validation. All P0 checks and mandatory acceptance scenarios must pass before claiming the phase delivered. Deferred simulation remains a separate later design decision.

#### Execution order and useful delivery checkpoints

1. **Immediate correction:** T01, then T02/T03. Wrong-area actions stop; the normal product and its counts follow the corrected phase.
2. **Trustworthy scope and saved work:** T04/T05 → T06/T07. This is the first complete contract checkpoint and enables real formative feedback.
3. **Geographic/source expansion:** T08 and T09 → T10 → T11. References expand without changing ingestion authority; several profiles operate independently.
4. **Daily map workflow:** T12/T13/T14. Monitoring/historical meaning, scalable discovery, role-focused navigation and international input are coherent.
5. **Complete analyst outcome:** T15/T16 → T17/T18 → T19. Reopenable investigation, repeatable imagery, independent review and readable brief work together.
6. **Environment preparation:** T20 → T21, once required inputs and output-rights contracts exist. It may overlap late reporting work, respecting dependencies.
7. **Qualification:** T22 → final T23 → T24. Formative T23 sessions still start after T14; closing user sessions use the operationally qualified release candidate. The earlier gates are closed by measured outcomes, not another feature list.

T05, reference preparation, test harness design and early user sessions can progress when their dependencies permit. This ordering is a dependency plan, not an instruction to launch parallel agents or to implement all tasks in one change.

```mermaid
flowchart LR
  A[Contain wrong-area actions] --> B[Versioned area and period]
  B --> C[Shared map scope and durable case]
  B --> D[References and approved retrieval profiles]
  D --> E[Coverage and task readiness]
  C --> F[Daily map workflow]
  E --> F
  F --> G[Imagery, findings and review]
  G --> H[Readable brief]
  D --> I[Portable geographic preparation]
  H --> J[Workload and independent-use qualification]
  I --> J
```

The map-only startup and content-purpose work apply across the chain. Rights and access checks remain attached to every dependent operation.

### 5. Acceptance scenarios

These scenario IDs are the stable references used by tasks and the backlog ledger. Add tests where they exercise meaningful behavior; do not use snapshot counts or tests mirroring implementation as a substitute for workflow proof.

#### A01 — Independent map startup

Fresh and upgraded map profiles start without runtime/recording/asset executables or simulation grants. An ordinary analyst enters Public Context and completes discovery/save. Missing optional simulation facilities do not mark the map task unavailable. Historical facilities still require their original explicit access.

#### A02 — Area integrity across every handoff

Country, subdivision, named study, drawn polygon, viewport and reopened investigation preserve exact area meaning through map, Systems, capture, analysis, review and export. A too-large country demands deliberate narrowing. An existing pinned draft stays labeled with its own scope. Missing geometry and unsupported shapes cannot submit another place. Inspect request bodies and retained output manifests, not only visible labels.

#### A03 — Immutable geography across updates

Save a case against edition A, install release B, and reopen A with identical retained geometry/hash. Latest discovery may show B. Check old parent hierarchy, partially staged releases, mixed providers, deleted/renamed upstream places and access revocation. No current release silently rewrites a historical output.

#### A04 — Time interpretation

Use current monitoring, fixed historical intervals, known-at and effective-at with late/corrected/withdrawn reports. Test rolling preset evaluation, frozen checkpoints, midnight/year boundaries and different user timezones. A historical capture outside monitoring time is omitted or explicitly background context. Unsupported historical feeds show unsupported, not a present-day reconstruction.

#### A05 — Selection does not change authority

Switch countries and create study areas without any admission/grant mutation. An approved bounded acquisition executes; an out-of-envelope request enters the coverage-request workflow. An unapproved profile/revision cannot be claimed by an existing source-granted worker. A normal analyst is not prompted for raw token-file paths.

#### A06 — Multi-profile retrieval without corruption

Run overlapping profiles concurrently: independent cursors, leases, periods and coverage; one retained provider revision where identities match; occurrence history for both runs. Exhaust shared account quota and verify all affected requests pace fairly. Retry after response loss using the same intent key. A stale lease, changed profile or rights suspension cannot publish new completion.

#### A07 — Coverage, emptiness and readiness

Demonstrate outside footprint, partial/unknown coverage, missing retrieval, successful zero-result retrieval, stale data, missing credentials, denied access, provider failure and healthy workers with no compatible inputs. Every case has a truthful summary and supported next action. No result asserts factual absence merely because the retained count is zero. Forbidden source details do not leak through totals or readiness.

#### A08 — Reference depth and worldwide geometry

Find subdivisions/places in several Middle Eastern countries and international areas via the same importer/search. Test Arabic/English aliases, point-only places, coastal/maritime study areas, cross-border selection, date-line components and explicit high-latitude display restrictions. Imports resume and activate atomically; detail not installed remains labeled and requestable.

#### A09 — Scale and stale-response control

Use catalog sizes above former result caps and dense map queries above the aggregate threshold. All totals/subsets/cursors have accurate meanings. Verify component deduplication, aggregate drill-down and detail retrieval. Change area/period rapidly under delayed responses: obsolete data cannot replace the active view, server work cancels within budget, and hidden workbenches do not accumulate polling.

#### A10 — Durable case and conflict behavior

Save a question, scope, evidence, note and analysis link; restart and reopen through another authorized device/session. Test interrupted saves, visible unsent drafts, two simultaneous editors and expected-revision conflicts. No autosave claims success before persistence. Editing a case does not change immutable evidence or relabel prior jobs.

#### A11 — Fresh imagery and honest analytical outcomes

From the selected case, discover admitted scene metadata, choose a valid local subset, acquire real windows, inspect masks, select a compatible pair and retain analysis. Repeat on an additional Middle Eastern and an international AOI where supported data exist. Exercise cloud/coverage/alignment failures, expired access, interruption, retry, deduplication, cancellation and valid zero change. Count distinct acquisition dates separately from capture attempts and catalog revisions.

#### A12 — Case access and independent review

Test ownership/membership plus effective source grants through API and non-owner database role. Include direct joins, pooled-connection reuse, service workers, a member losing a source grant and restricted case titles/notes. Reviewer assignment never grants source access. Same-human aliases, contributing authors and service identities cannot independently approve; new authored revisions invalidate approval eligibility. Real independent review is separately recorded under A19.

#### A13 — Useful brief and controlled output

Generate/read a question-led brief with area/period, coverage, maps/comparison, exact citations, uncertainty and review state. Verify manifests/asset hashes and current dependency access before download. Test denied byte inclusion with permitted references, no external dependency masquerading as offline-complete output, legacy bundles, changed case revisions and expired rights. Export authority is checked before distribution; downloaded copies are not claimed to support remote revocation.

#### A14 — Development content retains its meaning

Known acceptance/development records are excluded from ordinary working counts/defaults. Real source bytes used by tests remain identified as real, while their test publications remain test publications. Unclassified records are visible through an explicit review path. Administrators can inspect retained history, with original IDs and immutable content intact.

#### A15 — International and role-focused interaction

Use separate analyst, reviewer, source administrator and restricted-read accounts. Verify source-backed Arabic/English aliases, mixed-script labels/notes, correct map shaping, visible timezone/year, explicit coordinate order and supported datum/units. Complete core tasks with keyboard and at 320px/390px and short-landscape sizes; verify focus restoration, text zoom, contrast and readable errors. Record English application-language support separately from Arabic place search and content support.

#### A16 — Portable geographic preparation

Prepare an additional supported area using the same public workflow. Verify CRS/frame/origin, vertical datum/grid applicability, independent control values, parent hashes, source dates/rights, nodata, estimated properties, omitted geometry, budgets and reconstruction. Read the output independently of Studio. Actors, synthetic sensing and runtime execution are absent from the preparation journey and acceptance criteria.

#### A17 — Faults, capacity and unattended use

Run the declared wall-clock profile and capture request/job/worker/coverage telemetry. Rehearse API/worker restart, abandoned lease, object store failure, provider backoff, credential expiry, full storage quota and lost responses. Test history growth beyond legacy caps without relaxing active/storage/provider limits. Recovery preserves exact scope, attempts and evidence meaning, and stale workers cannot commit.

#### A18 — Upgrade, restore and access preservation

Upgrade an isolated populated installation through all forward migrations. Confirm old source/evidence/scene/capture/package/area IDs and still-permitted original bytes are unchanged; legacy admission mappings remain exact. Restore the paired database/object backup into isolated services and validate every required object/dependency, including policy-valid tombstones under D15. Reproduce an analysis with retained permitted inputs, read a case/brief and validate member/source restrictions after restore. Rehearse the actual rollback binary and forward recovery with new profile records present. A successful storage probe alone does not satisfy this check.

#### A19 — Observed independent usefulness

At least three target analysts and one independent reviewer are the proposed initial sample, with the reviewer separate from the case contributors. Start formative sessions during T14; use the final release candidate for the closing sessions. Ask users to discover, explain coverage/time, save/resume, analyze, hand off and read a brief, including no-data/gated cases. Record assistance and wrong interpretations rather than coaching them through silently.

Proposed exit: no wrong-area or wrong-period outcomes; all participants correctly explain coverage versus absence and result/review status; at least 80% of assigned core task attempts complete without developer assistance, with counts and failures reported. Every critical failed journey is repaired and retested. This small sample establishes a bounded usability result, not statistical proof for all users.

#### A20 — Honest release ledger

Every task and gap has links to its implementation revision, installed profile, functional evidence, geographic samples and user-validation status. P0 failures block release. An external prerequisite can remain documented as gated, but cannot be labeled as a successful live capability. All mandatory journeys in the declared release profile pass. Future simulation design is explicitly outside the next automatic task sequence.

#### A21 — Evidence semantics and changed support

Carry a provider report, source/reference fact, derived indicator and human assessment from map to brief with their distinct quality/confidence and geometry meanings. Correct, withdraw or invalidate cited evidence after a finding is approved. The exact historical content stays pinned; current views show the support-change notice and reassessment action. New current-purpose approval/distribution cannot silently proceed on invalidated support. Test conflicting reports and a missed/rebuilt dependency projection.

#### A22 — Stable reads under concurrent publication

Page a result while another transaction publishes, corrects or withdraws matching evidence, including a transaction begun before the read snapshot but committed afterward. Counts, order and exact revisions stay consistent within the snapshot; monitoring refresh deliberately creates a new result. Test concurrent grant changes, cursor tampering/wrong scope, snapshot expiry and resource limits. No long-lived database transaction or unrestricted result cache is required for pagination.

#### A23 — Lifecycle, retention and lost access

Archive/reopen, transfer ownership, remove a member, switch accounts with an unsent draft, and expire a referenced input while a job is running. Case revisions retain exact finding versions; protected content never appears under the next account. Exercise interrupted artifact deletion and restore of permitted tombstones. Missing unexpired dependencies fail validation; expired dependencies show unavailable/partial-reproduction status and never receive an invented retention extension. Repeat for composite briefs and environment packages.

#### A24 — Review/render and durable publication races

Render a draft, edit the case or change the renderer, and attempt approval of the previous manifest. Only the exact inspected digest can receive an attestation; altered content needs a new review. Race approval, rights revocation, worker cancellation and object upload completion. A crash/lost response creates neither a second intent nor a published object without its database record. Include script/URL injection and unsafe embedded paths; a self-contained brief performs no undeclared fetch and executes no included source text.

Recording output backlinks, review events or validity notices does not mutate the reviewed authored revision or invalidate itself; adopting new evidence or editing a finding does.

### 6. Qualification matrix and initial budgets

#### Geographic and state sample

- **Regional detail:** proposed Saudi Arabia, UAE, Jordan and Egypt, including at least one supported subdivision and place in each. These are test samples, not the complete regional definition.
- **International:** proposed France, Japan and Brazil, spanning different regions and geometry/data conditions. Include a supported local acquisition outside the Middle East.
- **Cross-border/maritime:** one regional view spanning multiple administrative areas and one coastal/water study polygon; record actual provider coverage and source boundary meanings.
- **Geometry extremes:** a date-line-crossing study and a high-latitude selection; verify the explicit supported behavior instead of pretending the initial map projection is universal.
- **Selection forms:** country, subdivision, named study, drawn polygon, viewport and restored investigation, plus a point-only place that requires explicit area selection.
- **Evidence states:** current, historical, corrected/late, withdrawn, successful empty, unobserved, partial, stale, missing credentials, denied access and processing failure.
- **Real workflows:** a fresh imagery study in an additional Middle Eastern AOI and an international AOI; one new environment package outside the prepared fixture; independent case handoff and brief reading.

Choose acquisition dates after checking actual admitted catalog/quality; do not promise a fixed clear pair before discovery. Unsupported inputs are legitimate negative tests, but they cannot replace all required successful real journeys. A restricted optional feed can remain gated if the declared product and acceptance claims say so explicitly.

#### Proposed workload profile v1

Freeze exact fixtures, hardware, software versions, provider budgets and test duration before measurement. The following is the starting qualification proposal:

- **Read load:** 100,000 retained report revisions, 50,000 detailed reference features with realistic polygon complexity, 10,000 weather/moving records and 1,000 catalog/capture-history entries distributed over at least ten areas. Use isolated acceptance data with varied spatial density and revision history; do not inject synthetic public facts into ordinary working catalogs.
- **Concurrency:** five simultaneous analyst sessions plus ten scheduled retrieval profiles; retain the initial maximum of four active raster captures. Respect stricter provider limits even when that reduces throughput.
- **Latency:** scoped search/coverage p95 ≤1 second; map family/tile requests p95 ≤2 seconds; first useful map after selection ≤3 seconds on the recorded local qualification network. Record cold and warm results separately and distinguish provider acquisition time from interactive query latency.
- **Responsiveness:** ordinary controls respond within 100 ms on the recorded browser/hardware; no obsolete query results displayed after a scope change. Cancel client fetches promptly and bound canceled server reads within 5 seconds. Prevent hidden workbench history from increasing active poll counts.
- **Payloads:** initial detailed GeoJSON response target ≤2 MiB, individual reference tile target ≤512 KiB; enforce totals/representation semantics when switching to aggregates. Existing local raster size/geometry limits remain until measured expansion is justified.
- **Durability:** zero acknowledged case edits lost; no duplicate job for the same intent; no stale-lease completion; all restored required and still-permitted objects hash-verified, with policy-valid tombstones explicitly reconciled.
- **Soak:** 24 actual wall-clock hours with monitoring/retries and at least one planned worker restart. Capture queue delay, provider freshness, memory/storage growth, failures and recovery. A simulated or virtual hour is not equivalent.
- **Recovery:** worker retry/lease recovery within 5 minutes under the declared profile; object/provider recovery produces visible state transitions without data substitution. Measure RTO/RPO from the backup procedure instead of inventing a universal guarantee; retain the actual last consistent backup time.

These values may be revised after baseline measurement and explicit review of tradeoffs. Publish both the original and revised profiles; never mark an unmet target passed by changing its definition after the run.

### 7. Gap-to-delivery traceability

| Gap | Priority | Main implementation tasks | Closure evidence |
| --- | --- | --- | --- |
| GAP-01 Simulation scope drift | P0 | T02, T24 | A01, A20 |
| GAP-02 Hardcoded geography | P0 | T01, T04, T06, T08, T20 | A02, A08, A16 |
| GAP-03 Incorrect area handoff | P0 | T01, T04–T07, T15 | A02, A03, A04, A10, A22 |
| GAP-04 Uneven reference depth | P1 | T04, T08, T20 | A03, A08, A16 |
| GAP-05 Availability versus coverage | P0 | T09–T11 | A05, A06, A07 |
| GAP-06 Monitoring versus history | P0 | T05, T06, T11, T12 | A04, A07, A21, A22 |
| GAP-07 Regional/world scale | P1 | T04, T13, T22 | A08, A09, A17, A22 |
| GAP-08 Competing navigation | P1 | T02, T06, T07, T12, T14, T16, T23 | A02, A10, A15, A19 |
| GAP-09 Durable investigation | P1 | T07, T16, T17, T19 | A10, A12, A13, A19, A21, A23, A24 |
| GAP-10 Repeatable imagery | P1 | T01, T10, T15, T19 | A02, A06, A11, A13 |
| GAP-11 Analyst brief/output | P1 | T16–T19, T23 | A12, A13, A19, A21, A23, A24 |
| GAP-12 Development content | P1 | T03, T18, T24 | A14, A20 |
| GAP-13 Task readiness | P1 | T09–T12, T15, T22 | A06, A07, A17 |
| GAP-14 Workloads/unattended operation | P1 | T09, T10, T13, T21, T22 | A09, A17, A18, A22, A23, A24 |
| GAP-15 International/role usability | P1 | T05, T08, T14, T23 | A04, A08, A15, A19 |
| GAP-16 Portable environment preparation | P2 | T18, T20, T21 | A13, A16, A18, A23 |
| GAP-17 Independent usefulness | P1 | T17, T23, T24 | A19, A20 |
| GAP-18 Documentation/approval state | P0 | T02, T03, T24 | A01, A14, A20 |

### 8. Risks and bounded decisions

**Highest implementation risk: profile-aware rights and scheduler cutover.** Source state currently has one active admission and artifact consumers depend on it. Implement T09 before enabling new profiles, exhaustively inventory policy consumers, and fence old workers during cutover. A UI-only profile selector would be incomplete.

**Highest correctness risk: partial scope migration.** Moving only export to the new area model leaves capture, Systems, bookmarks or pinned drafts inconsistent. T06 must close the full handoff matrix. Geometry and time are server-validated, not trusted merely because TypeScript types match.

**Highest access risk: multisource authored content.** Case prose and rendered images can reveal a restricted source even if individual evidence APIs deny it. Enforce whole-case/output dependency access initially, with explicit worker job authorization. Reuse canonical review identity and prevent reviewer edits from retaining independence.

**Geographic expansion risk: mixed releases and assumed fidelity.** Admit complete reference releases, retain original editions and source boundary meanings, and treat generalized reference polygons as qualified references. Preparing a second DEM region also requires suitable transformation grids and controls, not just a new bbox.

**Operational risk: larger history exhausts old finite budgets.** Replace lifetime-row ceilings with pagination and explicit storage/retention policy, keeping admission limits and exact history. Add bytes/queue/age diagnostics before increasing workloads; full disks should stop new work predictably.

**Scope risk: solving every gap by adding a new platform.** Reuse existing modules, workers, database, storage and map. Introduce only the domain records needed for scopes, cases, profiles and outputs. A generic workflow engine, new simulation architecture or unrelated mapping-library major upgrade is not a prerequisite.

#### Defaults adopted for planning

The plan recommends an English initial interface with Arabic/English place search and mixed-script correctness; explicit local/UTC display; 24-hour event monitoring with source-specific freshness; server-persisted cases; independent case-revision review; readable HTML/print output first; and the versioned working regional preset described in D05. These are concrete starting choices, not hidden dependencies on a future planning exercise.

Before qualification, record the target analysts/reviewer, accepted task list, exact installed source profiles and final workload profile. Provider credentials/terms, additional coverage approvals and actual human participation are external prerequisites for their dependent acceptance cases. Continue independent implementation/testing while those are unavailable, but leave the corresponding live/usefulness results unqualified. Do not create placeholder operational evidence or mark them complete.

### 9. Verification and evidence procedure

Each implementation task should attach its change-specific proof plus the applicable checks already supported by [Taskfile](../../Taskfile.yml):

- Changed Go packages and focused integration cases, then the existing assembled PostGIS/S3 integration gate when their contracts are affected.
- TypeScript typecheck/production build, regenerated OpenAPI client reproducibility, permission/route-policy consistency and relevant real-backend browser journeys.
- Migration-on-populated-data, RLS/source/case access and original-byte preservation for schema changes. Existing canonical identity and public/exercise isolation regressions continue to apply.
- Keyboard/narrow-layout and scope/time browser checks for affected user actions. Add failure or race cases where they address an actual behavior risk.
- Source installation/real-acquisition, output rendering, environment verification and independent sessions as separately dated evidence; fixtures cannot satisfy these by themselves.

Store sanitized evidence under `docs/implementation/evidence/map-first/` with task/scenario ID, commit/build, configuration/profile hash, source/area versions, execution date, hardware where relevant, expected/actual result and limitations. Keep tokens, private provider responses and restricted evidence out of committed test reports. Link permitted artifacts instead of copying confidential content into diagnostics.

Maintain the [machine-readable backlog](MAP_FIRST_BACKLOG_2026_10_25.json) alongside this plan. Its initial task status is `PLANNED`; each task has separate implementation, installation, functional, geographic and user-validation evidence arrays. Populate those with actual results, never speculative percentages.

**Planning-pass verification:** source/API/schema inspection and checks of task dependencies, gap coverage, acceptance IDs and document links. No new application tests, migrations, data installation, provider activation or user sessions were performed merely to prepare this plan.

### 10. Closure rules and maintained planning harness

The [audit](#plan-audit) and backlog's atomic requirements make coverage reviewable below the gap-heading level. The validator checks IDs, task/scenario mappings, reciprocal gap coverage, dependency ordering/cycles, source-document fingerprints, and local links. Its failure-injection mode checks that omitted coverage, cycles, mismatches and unsupported completion claims are actually rejected. Neither mode verifies that the proposed algorithms work; that remains the implementation and acceptance work.

```powershell
python scripts/ops/check_map_first_plan.py
python scripts/ops/check_map_first_plan.py --self-test
```

Use these closure rules throughout execution:

1. Establish an accountable implementation owner and evidence reviewer for each active task, and the named participants/operators for external prerequisites. The ledger records role requirements now; do not fabricate named assignees or dates. Record unresolved prerequisites as `NOT_CONFIRMED` or `BLOCKED`, not completed capabilities.
2. Early gates close their applicable scenario assertions only. For example, MAP-0 proves startup/discovery in A01; saving a case additionally needs T07. T17 can finish its manifest-bound review service before T19 supplies rendered briefs. Final A01/A24 results require the integrated journey. Partial task evidence must state the exact assertion it exercises.
3. Final release qualification uses one immutable candidate identity: commit/build, migrations, configuration, source/profile/area versions, renderer/assets, permission model and workload profile. Evidence records expected/actual outcomes, run time, environment and applicable assertions. Implementation, installation, functional, geographic and independent-user proof remain separate. Explicitly justified `NOT_APPLICABLE` is allowed for a task/evidence dimension, never for a mandatory end-to-end scenario.
4. All 24 acceptance scenarios are mandatory at phase closure, including their required successful real journeys. An optional feed can remain gated; it cannot replace required acquisitions, independent people or an environment package. If one is blocked, release a clearly scoped increment if useful, but do not close its gap or claim all 18 solved.
5. Wrong area/period, access leaks, lost acknowledged saves, invalid review, broken provenance or mandatory recovery failures block closure. Numeric targets, geographic/environment tolerances and participant tasks are frozen before qualification. Report failures, timeouts and sample sizes alongside latency results; mark flaky or unavailable checks unresolved.
6. Record the preflight baseline, then T22 operational qualification and final T23 user qualification against that candidate. A subsequent fix invalidates affected evidence: rerun impacted functional/geographic/user assertions, and repeat the soak when scheduling, resource use, persistence or recovery changed. Unrelated evidence can be carried forward only with a recorded impact rationale. Dates alone do not establish that evidence describes the release candidate.
7. Keep an operator runbook for quota/credential gates, storage exhaustion, expired dependencies, stalled jobs, backup quiescence and rollback. Each diagnostic links a recovery action to an authorized role. Metrics are shipped with the related feature, rather than added only for T22.

No planning-only audit can establish that nothing remains undiscovered. This revision resolves the identified design omissions and provides explicit tests and closure rules for the remaining implementation uncertainty.

<a id="plan-audit"></a>

## Plan completeness and hardening audit

> **Status:** Historical audit of the plan as written on 25 October 2026. The plan was executed afterwards: T01–T21 are implemented, T22–T24 remain IN_PROGRESS. Its findings remain traceability history. See the [records index](README.md).

**Date:** 25 October 2026.\
**Scope:** Second-pass challenge of the [implementation plan](#implementation-plan), against the [18-gap report](#gap-report), [governing scope](../../EMAD_CURRENT_SCOPE.md), code and delivery tooling.\
**Implementation baseline:** `a6d1d22f34692faea31db58e4a6318c539c3321a`.\
**Outcome:** Twelve design/closure weaknesses were identified and addressed in the plan. All 18 gaps now have 54 explicit requirement statements, mapped to the existing 24 implementation tasks and 24 acceptance scenarios. These are specified solutions, not verified product fixes.

### What the first validation established, and what it missed

The first pass correctly checked task counts, all-gap coverage, dependency cycles and local links. That could still pass with an incomplete design: a gap can have a task ID without specifying concurrent-read behavior, retention consequences or a review/publication race. The second pass challenged the contracts and execution sequence directly, including existing recovery and packaging code.

This was a direct repository audit. No live provider activity, application migration, operational deletion, deployment, user session or application acceptance run occurred. The added Python harness checks planning artifacts only.

### Findings and resolutions

| Check | Weakness in the earlier plan | Concrete hardening | Implementation / proof |
| --- | --- | --- | --- |
| H01 | Freezing an area/interval and using a cursor did not freeze membership; late commits/corrections could change totals or page contents. | D02 defines bounded materialized read snapshots, exact revision membership, expiry and live access checks, distinct from scope and historical time semantics. | T05, T13, T22; A22 plus A04/A09 |
| H02 | Provenance was preserved, but fact/report/indicator/assessment distinctions and later correction impact were not an explicit end-to-end contract. | D14 carries semantic/quality/geometry distinctions and reverse-dependency validity notices; original approved content remains immutable. | T12, T16, T19; A21 |
| H03 | “Keep original bytes/history” was underspecified when rights expire or bytes are legitimately deleted. | D15 defines permitted retention, composite dependency disposition, tombstones, partially reproducible cases and restore expectations. | T16, T18, T22; A18/A23 |
| H04 | Case states existed, but transition authority, owner departure, exact finding revisions and cross-account draft recovery were incomplete. | D03/D04 define lifecycle permissions, atomic saves, ownership recovery, archival semantics and protected cache/draft handling. | T06, T07, T16; A10/A12/A23 |
| H05 | Frozen output/review were required without a complete render-before-review protocol or handling of the final approval label and output backlinks. | D11 defines draft rendering, content-digest approval and a separate attestation; operational backlinks do not revise their own approved content. Edits/renderer changes invalidate eligibility. Escaping, script-free downloads and bounded assets cover the new HTML surface. | T17–T19; A13/A24 |
| H06 | General coverage states did not force per-adapter operation support or account for rejected/unpublished/truncated retrievals. | D08 requires an adapter capability inventory and receipts that distinguish retrieval from publication and complete from capped windows. | T10–T12; A06/A07 |
| H07 | Durable jobs and shared quotas lacked explicit accounting for crashes after sending a billable request or concurrent storage reservations. | D07 specifies conservative uncertain charges, atomic intent/job registration, storage reservation/reconciliation and bounded terminal retries. | T10, T15, T19, T21, T22; A06/A17/A24 |
| H08 | Map startup independence could pass while packaging and recovery still required simulation executables or missed new writers. | T02 includes package checking and platform paths; upgrade/T22 inventory and quiesce all writers including retention/render/environment jobs. | T02, T22; A01/A18 |
| H09 | T10 could remove the history cap before T13 paging; final user qualification could precede operational qualification. | T10 installs guardrails and retains the cap; T13 depends on T10 and replaces it with paging. Final T23 depends on T22; early formative sessions remain permitted. | T10, T13, T22, T23; A09/A17/A19 |
| H10 | Gate evidence could describe different builds, or mandatory external journeys could be relabeled optional to claim all gaps closed. | Section 10 binds qualification to a candidate, defines evidence invalidation/retest and requires all 24 scenarios. Missing humans/real inputs remain open prerequisites. | T22–T24; A18–A20 |
| H11 | Portable output checks named accuracy and LOD without requiring frozen tolerances or processing resource limits. | T21 requires source-supported accuracy/control criteria and pixel/feature/mesh/disk/RAM/time budgets before execution. | T20/T21; A16/A17 |
| H12 | The successful one-off consistency check was not retained and could not prevent subsequent backlog drift. | Added the maintained validator, source fingerprints, atomic requirements, acceptance records and failure-injection checks. | Planning harness now; T24 maintains it; A20 closes product evidence |

### Code evidence that constrained the changes

- [Public-event queries](../../internal/publicevents/query.go) document publication-time `known_at`, select latest revisions before spatial filters, and currently use `LIMIT/OFFSET`. The new snapshot design must preserve the first two behaviors while handling publication between HTTP reads. A timestamp or maximum sequence is not a commit-consistent snapshot.
- [Public-event projection](../../internal/publicevents/projector.go) already resolves related alerts on provider correction/withdrawal. New case/finding/brief dependencies need equivalent explicit impact handling; this audit does not claim correction support is entirely absent.
- [Artifact lifecycle](../../internal/artifacts/service.go) already blocks use after retention and implements `DELETING`/`DELETED` with retry. Composite output/case design must accommodate this existing behavior rather than promise permanent availability.
- [Scoped database binding](../../internal/platform/db/scoped.go) currently binds resource and source IDs. Principal/canonical-human binding and case RLS remain required new work; a client cache key or membership row cannot substitute for enforcement.
- [Capture jobs](../../internal/imagery/captures.go) already compare request input, fence leases and enforce active/history budgets. New intent/account/storage contracts extend these protections; removing the history cap before the replacement is ready would regress bounded operation.
- [Connector state](../../internal/publicingest/repository.go) has source-level checkpoints, leases and quota counters. New shared-account reservations must coexist with per-profile state and retain uncertainty after a network-side effect.
- [Delivery-package validation](../../scripts/ops/check_delivery_package.py) explicitly runs asset/recording workers and runtime description. It must become profile-aware alongside startup.
- [Paired backup](../../scripts/ops/paired_backup.py) relies on a quiescent system and checks a declared set of restored objects/workflows. New workers and object families must join that inventory; an API-port check alone is not writer fencing.

### Coverage of the original report

The machine-readable [backlog](MAP_FIRST_BACKLOG_2026_10_25.json) is authoritative for the 54 individual requirement mappings. Each gap has three separately reviewable obligations:

| Gap | Obligations checked | Additional challenge |
| --- | --- | --- |
| GAP-01 | Ordinary analyst entry; startup/package independence; preserved historical isolation | H08 |
| GAP-02 | Geography-default inventory; versioned region/world scope; identical selection workflow | Fixture constants must stay explicit |
| GAP-03 | Six area forms and period handoff; deliberate subset; exact revisions across races | H01 |
| GAP-04 | Generic reference pipeline; aliases/rights/accuracy/point meaning; regional/worldwide depth | Partial imports cannot activate mixed releases |
| GAP-05 | Area/time coverage states; approved bounded profiles; no implicit grants | H06/H07 |
| GAP-06 | Monitoring/history; distinct clocks; quiet/unobserved/background meaning | H01/H02 |
| GAP-07 | Aggregates/tiles/drill-down; accurate counts/cursors; geometry extremes | H01/H09 |
| GAP-08 | Map/dock/Systems journey; role actions and durable work; observed navigation | H04 |
| GAP-09 | Complete case content; lifecycle/conflicts/handoff; restart/access/immutable output | H02–H05 |
| GAP-10 | Fresh scoped imagery; honest quality/failure/zero-change; regional/worldwide repetition | H06/H07; indicators remain indicators |
| GAP-11 | Readable content; exact permitted reproducibility; independent review/recipient usefulness | H02/H03/H05 |
| GAP-12 | Purpose versus provenance; ordinary defaults; truthful counts and retained support history | Reclassification cannot manufacture human review |
| GAP-13 | Input/task readiness; recoverable provider/queue gates; durable honest outcomes | H06/H07 |
| GAP-14 | Measured workload; cancellation/history/capacity; faults and complete restore | H01/H03/H07–H10 |
| GAP-15 | Real role grants; international meaning; keyboard/narrow/error usability | H04 and degraded external basemap |
| GAP-16 | Bounded durable preparation; rights/coordinates/controls/lineage; portable non-simulation output | H03/H11 |
| GAP-17 | Independent users; observed usefulness/assistance; blocking interpretation failures | H09/H10 |
| GAP-18 | Current scope precedence; separate versioned evidence; honest mandatory closure | H10/H12 |

### Validation and its limits

Run from the repository root:

```powershell
python scripts/ops/check_map_first_plan.py
python scripts/ops/check_map_first_plan.py --self-test
```

The harness validates plan/backlog titles and mappings, reciprocal gap records, requirement coverage, hardening references, scenario coverage, dependency cycles and ordering, source fingerprints and local document links. It rejects unsubstantiated completed/qualified status. The self-test injects broken relationships and false-completion states in memory and expects rejection; it does not modify the plan.

The source fingerprints force a new review if the governing scope or original report changes. They are not proof that every natural-language requirement was interpreted correctly. Likewise, a requirement with a valid task/scenario link still needs human semantic review and actual implementation proof. The audit's 12 weaknesses are resolved **in the design**; runtime performance, acquisition success, geographic accuracy, access enforcement and usefulness remain unverified until their acceptance work runs.

**Recorded planning checkpoint, before implementation authorization:** the consistency check passed, and all 12 deliberately injected faults were rejected. At that checkpoint every implementation task was `PLANNED` and the release was `NOT_QUALIFIED`. The later [T01–T03 record](MAP_FIRST_DELIVERY_LOG.md#t01-t03), [T04–T06 record](MAP_FIRST_DELIVERY_LOG.md#t04-t06) and backlog now carry scoped task evidence. T07–T24 remain planned, all full scenarios retain their qualification gates, and planning-harness success itself never populates application acceptance evidence.

The plan is substantially stronger after this pass. Claiming absolute completeness before implementation and independent qualification would exceed the evidence.

Implementation update, 27 October 2026: the ledger now records T01–T18 as IMPLEMENTED with scoped evidence. T19–T24 remain planned; all full acceptance scenarios and independent-human qualification retain their gates. See [T16–T18 record](MAP_FIRST_DELIVERY_LOG.md#t16-t18).

T19–T21 update, 27 October 2026: readable briefs and portable geographic preparation have scoped implementation evidence in [the T19–T21 record](MAP_FIRST_DELIVERY_LOG.md#t19-t21). T01–T21 are IMPLEMENTED, T22–T24 remain planned, and full acceptance/release/observed-human gates remain open.


### T22–T24 qualification tooling, 27 October 2026

The authorized final batch adds workload samples, bounded operation histograms, expanded paired inventory/retention checks, observed-session record validation and active release traceability. T22–T24 remain IN_PROGRESS; all full acceptance and release gates retain their state. See [implementation and remaining gates](MAP_FIRST_DELIVERY_LOG.md#t22-t24) and [verification](evidence/map-first/t22-t24/verification.json). The harness still checks planning consistency only.
