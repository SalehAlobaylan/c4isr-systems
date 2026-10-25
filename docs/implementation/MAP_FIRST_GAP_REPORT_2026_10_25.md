# EMAD map-first product gap report and recommended actions

**Date:** 25 October 2026  
**Status:** Current scope correction recorded; implementation backlog proposed for review.  
**Governing scope:** [EMAD_CURRENT_SCOPE.md](../../EMAD_CURRENT_SCOPE.md).  
**Current objective:** A useful map-centered Public Context and Intelligence product for the **Middle East and the rest of the world**, with **geographic environment preparation only** for future simulation.

## 1. Correcting the direction

Simulation behavior was advanced ahead of the dedicated design conversation the user intends to have. That was a scope-management error. Runtime, actors, sensors, exercise operations, replay and evaluation must not continue as active milestones in this phase. Existing code and evidence should be preserved and clearly separated from the current map product. No deletion, worker shutdown or application change is performed by this report.

The earlier evaluation also made too much of the prepared Jeddah installation as a product direction. Jeddah is a useful historical fixture, not the intended geographic scope. The current product covers the Middle East with access and extensibility across the rest of the world. Future detailed simulation design remains open.

The recommendation is to preserve the working platform and concentrate delivery on a coherent map, meaningful regional/worldwide data coverage and complete analytical workflows. This report supersedes the earlier recommendations to center the product on a Jeddah-only pilot or develop a simulation pilot in parallel.

## 2. What “Middle East plus the world” means

The implementation needs three connected levels:

1. **Worldwide geographic access:** navigation, search, named geographic areas, country/region selection, references, consistent identities and explicit supported map/projection behavior.
2. **Regional intelligence coverage:** declared source footprints and time windows, public observations, imagery discovery, freshness, uncertainty and historical interpretation. Middle Eastern coverage receives delivery priority. The same mechanisms must work elsewhere where suitable sources and permissions exist.
3. **Detailed selected-area work:** bounded acquisition, analysis and geographic preparation for a chosen area. Large selections use summaries/tiles; detailed processing uses explicit smaller areas and durable jobs. Global scope does not require preloading every road, building or raster at full resolution.

Actual coverage may vary by source, licensing, credentials, geography and period. Those differences must be visible and actionable. Worldwide country navigation alone is not complete worldwide intelligence support, and an empty query is not proof of absence.

Use a configurable, versioned Middle East region definition rather than embedding a single country list or bbox throughout the application. Include relevant land, coastal and maritime areas according to the chosen task and source footprints. Retain the dataset's boundary interpretation and attribution; generalized references do not certify jurisdiction.

## 3. Evidence from the current installation

- Global country navigation already exists. A fresh [geographic sample](evidence/map-first/geographic-sample.json) found Saudi Arabia, the UAE, Jordan, Egypt, France, Japan and Brazil. Saudi Arabia returned 13 installed first-order regions; the other sampled countries returned none. This samples the installed catalog, not the world's available data.
- `scripts/data/install_geography.py` explicitly installs world navigation and Saudi subdivisions. Other countries need deeper reference coverage through the same generalized area model.
- `scripts/data/install_local.py` assigns Jeddah to imagery, FIRMS, OpenSky and AIS admissions and a Saudi extent to other installed retrieval profiles. These are bounded installation choices, not the intended limits of the product.
- Home, the Systems reference preview and context-export fallbacks contain Jeddah/Saudi-specific assumptions. An inventory must distinguish intentional fixture definitions from defaults that affect ordinary users.
- The [read-only product snapshot](../design/product-evaluation/installation-snapshot.json) contains 24 reference versions, 19 imagery catalogue revisions, four successful capture records and two distinct acquisition dates. The real imagery assessment is still a proposal; the published record is a development workflow test.
- The [browser reproduction](../design/product-evaluation/walkthrough.json) confirms a scope defect: selecting Saudi Arabia and clicking **Use investigation region** fills the Jeddah rectangle in export. No package was created during that review.
- The current development account has 42 contexts, many from exercise history. That is not a requirement for normal map users and must not define the current product's information architecture.

Existing functional tests provide valuable technical evidence. They do not demonstrate complete regional/worldwide coverage, independent usability or compliance with the newly corrected phase boundary. No new application tests were run merely to write this report.

## 4. Gap register

**P0:** correctness or scope prerequisite for this phase.  
**P1:** core map-product delivery.  
**P2:** supporting environment preparation after its geographic inputs are trustworthy.  
Priorities are proposed ordering, not a claim that implementation has started.

### GAP-01 · Simulation scope drift — P0

**Finding:** The installation and prior plans advanced runtime/exercise features beyond the user's intended current phase.

**Action:** Establish a map-first phase boundary. Exclude simulation execution and exercise-oriented onboarding from the normal map journey and its definition of done. Make ordinary map startup independent of simulation workers. Preserve existing implementation/history separately; do not add new runtime features or choose a future engine.

**Acceptance:** A normal analyst can use the product without selecting an exercise, starting a runtime, opening Studio or understanding simulation partitions. Current delivery tasks contain only map/Public Context work and the allowed environment foundations.

### GAP-02 · Geography hardcoded into product assumptions — P0

**Finding:** Multiple installation policies, previews and fallback areas are tied to Saudi Arabia/Jeddah.

**Action:** Inventory all geography-specific defaults. Move operational areas into named, versioned configuration and selected-area contracts. Keep Jeddah constants only where explicitly identified as fixtures or a selected dataset. Offer a Middle East overview and unrestricted supported world navigation.

**Acceptance:** Switching between supported Middle Eastern and worldwide areas uses the same workflow without editing source code, inheriting a Jeddah bbox or falsely claiming installed local detail.

### GAP-03 · Incorrect area handoff — P0

**Finding:** A selected country can become the Jeddah export rectangle through **Use investigation region**.

**Action:** Give the selected area a stable identity/geometry/version across map, analysis, review, brief and export. Derive supported rectangles explicitly from the actual selection. Where a format or processing limit requires a smaller area, ask the user to choose that subset and explain the restriction. Never substitute a different place.

**Acceptance:** Country, subdivision, named study area, drawn polygon, viewport selection and reopened investigation retain the correct area. Unsupported extents produce a useful narrowing action. No silent geographic substitution occurs.

### GAP-04 · Uneven geographic reference depth — P1

**Finding:** Worldwide country references exist; the installed subdivision/place coverage remains concentrated in Saudi Arabia.

**Action:** Provide a region-agnostic reference pipeline and catalog for administrative areas, settlements, coastlines and appropriate infrastructure. Expand Middle Eastern depth first while supporting worldwide acquisition through the same pipeline. Include source-backed aliases, Arabic/English names where available, retained rights, editions and stated geographic accuracy.

**Acceptance:** Users can find and select supported subdivisions/places in multiple Middle Eastern countries and international areas. Missing depth is labeled and can be requested/imported through supported actions. A place point is never presented as a municipal polygon.

### GAP-05 · Source availability is confused with geographic coverage — P0

**Finding:** A source may be configured/current while the chosen country has no admitted or useful observations. Selecting a country does not change the source's installed footprint.

**Action:** Model source-by-area-by-time coverage explicitly. Support approved regional source profiles and bounded requests beyond the original installation. Keep endpoint validation, source-specific credentials, quotas, rights and immutable admission history. New coverage must not silently broaden existing service grants. Country selection should show coverage and available acquisition actions, not modify ingestion authority by itself.

**Acceptance:** Any selected area explains which sources cover it, the applicable periods, retrieval freshness, missing access and the next supported action. Outside-coverage, permission-gated and genuine zero-result states remain distinct.

### GAP-06 · Current monitoring and historical study are insufficiently separated — P0

**Finding:** The fresh map opens at **World · configured evidence / All retained dates**. Old records can be prominent when someone expects current activity.

**Action:** Define explicit current-monitoring and historical-investigation modes, with visible area/time scope and source-dependent freshness. Preserve observed, received and revision times. Historical imagery belongs to a dated study, not an implied live picture. Define what a useful quiet-state summary says when no qualifying reports exist.

**Acceptance:** A user can explain the displayed period and coverage without opening diagnostics. Stale, historical, current and unknown information remain distinguishable. An empty well-covered query and an unobserved area receive different explanations.

### GAP-07 · Regional/worldwide map scale needs qualification — P1

**Finding:** A clean canvas and country search do not establish that regional/global discovery remains useful under real data volumes and extent limits.

**Action:** Use bounded PostGIS aggregates, vector/raster tiles, scale-dependent visibility, readable legends and deliberate drill-down. Separate broad discovery from detailed processing. Handle maritime areas, date-line crossings and projection/high-latitude limits explicitly. Avoid silent result caps and whole-world detailed downloads.

**Acceptance:** Middle East overview, country, subdivision and local AOI views remain readable and responsive. Complete matching totals and displayed subsets have clear meanings. Large or unsupported requests offer an actionable alternative.

### GAP-08 · Too many competing navigation concepts — P1

**Finding:** Product area, resource, workbench, Systems, inner tabs and technical vocabulary still compete with the analyst's task.

**Action:** Make the map the main working surface for this phase. Use a consistent model: selected area/period → evidence → investigation → finding/assessment → brief/export. Keep and improve the Systems grid as an overview of relevant content, coverage and work requiring attention. Keep the map clean with the existing compact dock and temporary/allocated panels. Place setup and processing details behind relevant actions.

**Acceptance:** A new analyst can locate data, inspect its source, continue an investigation and find its output without learning database partitions or simulation concepts. Labels and selection state remain consistent across entry points.

### GAP-09 · No complete durable investigation spine found — P1

**Finding:** Saved filters/bookmarks and retained evidence objects exist, but the inspected workflow does not provide a coherent persisted analyst case.

**Action:** Add named investigations containing the question, area, period, selected evidence, notes, analytical work, review state and outputs. Reuse existing evidence IDs and versions. Include ownership, resume behavior and an explicit handoff to a reviewer; keep evidence immutable.

**Acceptance:** An analyst can save, restart, reopen and hand off the same investigation with its correct scope and supporting evidence. The investigation is useful across regions, rather than a Jeddah-specific screen preset.

### GAP-10 · Imagery usefulness is demonstrated mainly by a prepared historical pair — P1

**Finding:** Four installed capture records represent two dates and one completed NDVI comparison.

**Action:** Make discovery, local AOI selection, acquisition, quality review, before/after comparison and retained analysis repeatable for other admitted areas and periods. Prefill forms from the actual map selection. Explain clouds/masks, alignment, insufficient coverage, processing failures and valid zero-change results. Reuse durable workers instead of synchronous API processing.

**Acceptance:** The workflow succeeds on additional Middle Eastern and worldwide AOIs where suitable data/access exist, without developer file preparation or source-code edits. Failure states remain honest. An indicator is not automatically published as a real-world conclusion.

### GAP-11 · The primary output is too technical — P1

**Finding:** JSON bundles and manifests are valuable for reproducibility but are not a complete analyst deliverable.

**Action:** Produce a readable brief with the question, area, period, coverage, findings, maps/comparisons, cited evidence, uncertainty, limitations and review status. Retain the machine-readable bundle alongside it. Support independent human review using the existing canonical identity protections.

**Acceptance:** A permitted recipient can understand the brief without reading schemas or job records. Findings link to exact retained evidence. Development-account workflow tests remain distinct from actual independent human review.

### GAP-12 · Development content and working content share normal catalogs — P1

**Finding:** Acceptance scenes, development publication records and old exercise contexts can inflate apparent readiness and distract from actual analytical content.

**Action:** Classify development/acceptance fixtures explicitly and filter normal defaults. Preserve immutable history. Distinguish source revisions, independent acquisitions, usable studies and approved outputs in counts. Do not delete old evidence or rename it into operational content.

**Acceptance:** Ordinary map users see relevant public content and their investigations, with no exercise-history requirement. Support/admin views can still inspect retained development evidence with its original meaning.

### GAP-13 · Worker health does not answer task readiness — P1

**Finding:** Ready workers and available-feature counts can coexist with stale feeds, unavailable acquisition and unobserved selected areas.

**Action:** Present readiness for the current area/question: usable evidence, available dates, coverage limitations, provider state, queued work and recovery action. Keep private credential/configuration detail in administration. Support interruptions, retries, deduplication, quota pacing and explicit external prerequisites.

**Acceptance:** Users can tell whether their task can finish and what is missing. A stalled provider is actionable; a successful empty retrieval does not become a false failure or an absence claim. Restricted feeds remain gated rather than simulated.

### GAP-14 · Wider workloads and unattended operation need acceptance — P1

**Finding:** Existing bounded profiles do not establish a worldwide, multi-area operational envelope.

**Action:** Define map/analysis budgets, cancellation and pagination behavior, realistic data-density loads and an unattended regional monitoring profile. Verify startup/restart, worker loss, storage failure, credential expiry and paired data/object restore. Record hardware, query/interaction latency and data volume with results; reuse working recovery infrastructure.

**Acceptance:** Declared regional/worldwide workloads meet agreed limits and recover predictably. Changing area cancels obsolete requests, prevents stale content from appearing in the new area and preserves proper access boundaries.

### GAP-15 · International and role-focused usability is not yet qualified — P1

**Finding:** The current broad development account does not represent every analyst/reviewer experience, and worldwide scope brings naming, language, units and time interpretation requirements.

**Action:** Provide role-led starting points and progressively disclosed technical controls. Validate Arabic/English place search, mixed-script rendering, clear local/UTC timestamps, supported coordinate/datum entry, keyboard/focus, readable contrast and narrow screens. Do not assume a country selection implies an account or permission change.

**Acceptance:** Target users can identify the current area, period, source meaning and available actions. Important place aliases work. Normal tasks do not require token-file paths, raw identifiers or unexplained datum conversion.

### GAP-16 · Environment preparation must become portable and region-agnostic — P2

**Finding:** The current detailed prepared environment is near Jeddah; it must not stand in for the global preparation capability.

**Action:** Generalize bounded geographic preparation: selected area → permitted terrain/imagery/vectors → cropped/versioned inputs → validated environment manifest/artifacts. Record CRS, units, input/output vertical datums, transformation grids and their area of use, source dates/rights, estimated building properties, omissions and size/LOD budgets. Retain parent bytes and transformation lineage. Heavy preparation stays outside the synchronous Go API.

**Acceptance:** An additional supported area can be prepared without changing application code. Artifacts are reusable by future tools, correctly georeferenced and truthful about fidelity. This gate contains no actors, simulation clocks, behaviors, synthetic sensors, exercise start or replay.

### GAP-17 · Product usefulness has not been independently demonstrated — P1

**Finding:** Passing implementation tests does not prove an analyst can obtain a useful result without developer guidance.

**Action:** Observe independent target users performing map discovery, current/historical inspection, investigation, review and brief/export tasks. Test multiple Middle Eastern and worldwide areas, including missing coverage and quiet periods. Measure correctness, assistance required, completion and output usefulness against the user's present workflow.

**Acceptance:** Users complete the agreed tasks and correctly explain coverage/uncertainty. Any wrong-area or wrong-period interpretation is a blocking finding. Record observed results rather than assigning a speculative completion percentage.

### GAP-18 · Documentation and approval state can restart the wrong roadmap — P0

**Finding:** Older documents and completion summaries still emphasize broad C2 or implemented simulation milestones. They can be mistaken for current authorization.

**Action:** Put the current scope above historical plans. Track implementation, installation, functional verification, geographic coverage and user validation separately. Retain previous results as history; require a later dedicated simulation discussion before opening a simulation implementation sequence.

**Acceptance:** “Next task” cannot automatically mean another simulation increment. The current product definition, backlog and acceptance criteria consistently say map/Public Context, Middle East plus world, and environment preparation only.

## 5. Recommended delivery sequence

These are outcome gates, not dates or automatic execution instructions. Reliability, isolation and accessibility checks accompany every stage.

### MAP-0 · Correct the scope and ordinary product surface

Addresses GAP-01, GAP-12 and GAP-18. Adopt the current scope, reconcile the active product brief, identify fixture-only content and make normal map use/startup independent of simulation. Preserve old code/history and remove its status as an active phase requirement.

**Exit:** A normal user enters the current map product without exercise/runtime choices or development content determining the workflow. Historical facilities remain separately identifiable.

### MAP-1 · Establish the global area model and correct every handoff

Addresses GAP-02, GAP-03 and the contracts behind GAP-04/07/09. Make countries, subdivisions, named areas, drawn areas and viewports use a consistent identity/geometry/version model. Define the durable investigation scope and explicit large-area/subset rules early. Fix the export substitution immediately.

**Exit:** Area and period survive navigation, analysis, review and export correctly across regional and worldwide samples.

### MAP-2 · Expand reference depth and explicit source coverage

Addresses GAP-04, GAP-05 and GAP-13. Install/enable regional reference depth through a generalized pipeline, expose provider coverage by area/time, and support bounded approved source profiles. Middle Eastern coverage is prioritized; worldwide acquisition uses the same mechanisms rather than a separate city-specific implementation.

**Exit:** Users can discover what is available, missing or obtainable in any supported selection. Global navigation is no longer confused with universal live observation coverage.

### MAP-3 · Deliver a useful map-centered daily workflow

Addresses GAP-06, GAP-07, GAP-08 and GAP-15. Establish current/historical modes, meaningful quiet states, scalable layers, clear evidence inspection and consistent actions. Preserve the clean map and Systems grid. Make user tasks, rather than module names, determine the interaction.

**Exit:** Users can answer what is reported for an area/period, how fresh it is, what supports it and what remains unknown.

### MAP-4 · Complete repeatable investigation, imagery and reporting

Addresses GAP-09, GAP-10 and GAP-11. Persist investigations, support further real acquisitions, review quality and interpretations, hand off to a reviewer, and produce a readable brief with a reproducibility bundle.

**Exit:** The same complete workflow succeeds in multiple Middle Eastern and worldwide areas where inputs/access exist. It does not rely on the prepared Jeddah pair or developer intervention.

### MAP-5 · Generalize environment preparation only

Addresses GAP-16. Reuse the now-trustworthy geographic acquisition and versioning pipeline to prepare portable environment artifacts. This can proceed alongside late MAP-4 work once its input contracts are stable.

**Exit:** A validated environment package is produced for another supported area. Nothing in this gate executes a simulated world or commits the user to a future simulation design.

### MAP-6 · Qualify the phase through real use

Addresses GAP-14 and GAP-17 and closes the earlier gates. Run independent analyst/reviewer sessions, unattended monitoring/recovery and declared map workloads. Resolve the resulting task failures before expanding scope.

**Exit:** The map/Public Context workflow is measurably useful, geographically honest and repeatable. Simulation discussion is a separate later decision, not the next automatic implementation gate.

## 6. Geographic acceptance coverage

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

## 7. Preserve and defer

**Preserve/reuse:** Go modular domains, PostGIS geometry operations, MapLibre and the compact dock, Systems grid, source/evidence history, confidence/provenance distinctions, review identity safeguards, durable acquisition/analysis jobs, private object storage, backups and portable geographic artifacts.

**Defer from the current phase:** simulation engines and engine selection, runtime enhancements, actors and behaviors, synthetic sensor modeling, exercise-controlled missions/commands, simulation recording/replay, evaluator features and further S4 expansion. Existing versions remain historical assets subject to a later agreed design.

The next success criterion is a useful map and a trusted analyst outcome across the intended region/world scope. Environment readiness supports that work now and preserves future options; simulation execution is not part of the current deliverable.
