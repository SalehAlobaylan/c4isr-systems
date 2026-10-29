# Map-first gap analysis against the committed implementation

**Date:** 25 October 2026.  
**Reviewed commit:** `a6d1d22f34692faea31db58e4a6318c539c3321a` — `feat(emad): add public intelligence workflows and map workspaces`.  
**Inputs:** [gap report](MAP_FIRST_GAP_REPORT_2026_10_25.md), [current scope](../../EMAD_CURRENT_SCOPE.md), current source, migrations, tests and retained evaluation evidence.  
**Status:** Analysis and proposed closure criteria. This document does not authorize or implement a delivery batch.

**Follow-up:** The [implementation plan](MAP_FIRST_IMPLEMENTATION_PLAN_2026_10_25.md) and [backlog ledger](MAP_FIRST_BACKLOG_2026_10_25.json) translate this analysis into proposed contracts, migrations, ordered tasks and qualification scenarios.

## Assessment

All 18 gaps remain relevant. The proposed priorities are reasonable: six P0 prerequisites, eleven P1 delivery/qualification gaps and one P2 environment-preparation gap. Their existence does not mean the last delivery produced no useful platform. Its strongest foundations are already present: scoped access, immutable evidence, admitted sources, versioned geography, PostGIS queries and tiles, durable imagery work, canonical review identities, and paired database/object recovery.

The central problem is that these capabilities do not yet share a complete analyst workflow. Different screens can mean different things by “area,” “investigation,” “current,” “available” and “published.” A successful component can therefore contribute to an incorrect or unusable product outcome.

The most immediate confirmed defect is geographic substitution in export. Several other failures are reachable from the source: capture can retain an unrelated AOI, exact geographic IDs can become unavailable after a new edition, and downstream workbenches do not carry a common period contract. Wider performance and independent usability remain unqualified; the evidence does not establish that every broad workload fails.

The recommendation is to repair area/time integrity first, make coverage and task readiness explicit, and complete one durable investigation through independent review and a readable brief. Expand that same workflow across regions. Preserve the existing infrastructure and historical simulation work under the corrected phase boundary.

## Evidence and limits

- The working tree was clean at the start of this review. Findings below concern the committed source rather than an uncommitted implementation branch.
- The [geographic sample](evidence/map-first/geographic-sample.json), [browser walkthrough](../design/product-evaluation/walkthrough.json) and [installation snapshot](../design/product-evaluation/installation-snapshot.json) were read as retained observations from 25 October. Provider health and installed counts were not freshly queried during this analysis.
- Static source tracing confirms implementation behavior where stated. Additional conditions discovered by inspection are identified below; they have not been newly reproduced in a live browser or database.
- `go test ./internal/publicevents ./internal/publicingest ./internal/imageryanalysis ./internal/contextpackage ./internal/assessments` exited successfully. All five package results came from the Go test cache. No new browser, integration, load, fault-injection or independent-user session was run.
- Existing geographic integration and browser tests were inspected. Their coverage is credited below without treating their historical results as new execution evidence.
- No application code, source admission, credentials, running workers, installed content or immutable evidence was changed for this analysis.

## Shared causes

1. **Product scope changed, but startup and ordinary entry points still implement the previous two-pillar product.** Documentation precedence is partly corrected; behavior is not.
2. **Area and period are fragmented.** The map has a region ID, drawn geometry and saved filters; workbenches have their own pinned state; export independently reads browser storage. There is no authoritative scope revision connecting them.
3. **Catalog presence substitutes for task readiness.** A record count, a healthy worker, a source admission and usable evidence for a particular area/time are different facts.
4. **Evidence is durable, but the analyst's case is not.** Jobs, source records and review revisions survive; the question, selected evidence, notes, working scope and handoff do not form a persisted investigation.
5. **Acceptance is narrower than the product promise.** Existing tests demonstrate specific technical paths, often against prepared data and broad development privileges. They cannot establish independent usefulness across regions and operational conditions.

## GAP-01 — Simulation scope drift · P0

**Assessment: confirmed in startup and normal navigation; public-data isolation already exists.**

[Home](../../apps/operator-ui/src/routes/home.tsx#L38) still asks users to choose between public evidence and a simulated world. The [map chooser](../../apps/operator-ui/src/features/workspace/MapWorkbenchChooser.tsx#L16) includes simulation/replay. [Local setup and startup](../../scripts/ops/local.py#L164) build/describe the runtime and launch asset, recording and runtime workers alongside map services.

This is delivery-profile coupling, not proof that public queries intrinsically require an exercise to run. The supervisor already restarts failed workers separately, and resource boundaries distinguish public evidence from exercise state. Those protections should be retained.

**Closure:** a map startup profile containing the required API, storage, connectors and imagery services; an ordinary map-first entry journey; separately identifiable access to historical facilities. Missing simulation executables, grants or worker readiness must not prevent this profile from starting or being considered ready.

**Proof:** start the declared map profile without simulation services, using an analyst with only the intended public permissions, and complete discovery, investigation and output tasks. This establishes independence more reliably than hiding a navigation label.

## GAP-02 — Geography hardcoded into product assumptions · P0

**Assessment: confirmed; distinguish legitimate fixtures from ordinary defaults.**

The relevant inventory includes [source installation policy](../../scripts/data/install_local.py#L77), [Systems fallback and preview](../../apps/operator-ui/src/features/workspace/SystemsWorkspace.tsx#L12), [export fallback](../../apps/operator-ui/src/features/imagery/ContextExport.tsx#L19), [capture defaults](../../apps/operator-ui/src/features/imagery/CaptureWorkbench.tsx#L18), [initial map camera](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L46), and [geography preparation](../../scripts/data/prepare_geography.py#L17). The camera is a presentation default; export/capture defaults can alter the work itself and take precedence in remediation.

The source uses one prepared installation as both sample data and implicit product configuration. No configurable, versioned Middle East definition was found in the inspected application/installer paths.

**Closure:** named region configuration with retained version/meaning, explicit installed dataset footprints, and selected-area parameters for operational actions. Preserve Jeddah fixtures and their original labels. A regional overview must describe the chosen region definition, including relevant coastal/maritime areas, rather than implying one politically universal boundary.

**Proof:** switch to a Middle Eastern and a non-Middle-Eastern area through the same flow; inspect every derived query, draft AOI and preview. No source edit or unrelated location default should be required. Depends on the GAP-03 scope contract.

## GAP-03 — Incorrect area handoff · P0

**Assessment: confirmed defect, with a broader persistence problem.**

[Country selection](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L88) stores/navigates a `region` ID. [Public filtering](../../apps/operator-ui/src/features/public-context/usePublicContext.ts#L29) correctly uses `region_id` or geometry instead of the saved bbox. But [ContextExport](../../apps/operator-ui/src/features/imagery/ContextExport.tsx#L18) reads only `investigation.filters.bbox` from a separate preference and supplies Jeddah when that value is empty. [IntelligencePage](../../apps/operator-ui/src/routes/intelligence.tsx) passes it a published revision, not the selected geographic scope. This explains the retained browser reproduction.

There is a second source-confirmed failure mode: [Geography](../../internal/reference/geography.go#L138) chooses the latest row for each logical region before filtering by exact ID. After admitting a newer edition with the same region key, lookup of a saved older ID yields no feature even though its immutable geometry remains stored. Saved investigations therefore need a version-resolution rule, not just a string ID.

**Closure:** one explicit area reference containing logical identity, exact edition/revision, geometry or a resolvable geometry hash, source meaning and derived bounds. Named, drawn and viewport areas need the same downstream contract. An analysis subset must record its parent scope and be deliberately selected. Bbox-only formats must explain their representation and limits.

**Proof:** all six area forms in the report survive map → analysis → review → export → reopen, including after a newer geography edition is installed. Missing, inaccessible, too-large or unsupported geometry must produce a blocking/narrowing state, never another location. Pinned workbench inputs may remain pinned, provided their actual area is explicit.

## GAP-04 — Uneven geographic reference depth · P1

**Assessment: confirmed installation gap; the generalized storage/admission foundation exists.**

[install_geography.py](../../scripts/data/install_geography.py#L25) installs worldwide country navigation, then filters subdivisions to `SAU` and places to the Saudi geometry. The [retained sample](evidence/map-first/geographic-sample.json) supports this: 13 Saudi first-order regions and none returned for the six other sampled countries.

[Geographic editions](../../internal/reference/geography.go#L24) already retain source, rights, hashes, geometry, aliases and edition metadata. The map already treats a place point as a location rather than inventing a municipal polygon. Those are valuable completed foundations.

**Closure:** parameterized reference acquisition/import, a versioned regional selection profile, explicit available/missing detail and a supported request/import action. Preserve source-backed Arabic/English aliases and edition/accuracy attribution. Do not claim the generalized country layer supplies every worldwide administrative unit or territory.

**Proof:** several countries and subdivisions plus international places, using the same pipeline. Include unavailable detail and point-only places. Expansion also needs geographic-result pagination: the backend returns at most 300 regions, while the inspected map consumers do not paginate those lists. Fix exact-edition resolution under GAP-03 before using new editions as saved case references.

## GAP-05 — Source availability versus geographic coverage · P0

**Assessment: confirmed missing composition; admission and health are already modeled separately.**

[Admission policy](../../internal/publicingest/domain.go#L52) retains an AOI, lookback, freshness, quotas, rights and connector identity. [Ingestion health](../../internal/publicingest/repository.go#L435) tracks successful retrieval windows and explicitly avoids an absence claim. The [coverage map family](../../internal/reference/map.go#L173) renders admitted query extents with the same qualification. This is more than a simple configured/unconfigured source flag.

What is missing is the answer for **this selected area, period and task**: footprint intersection, relevant retained evidence, actual retrieval completeness, freshness, permission/credential gates and permitted next action. A successful empty query remains only an empty provider query unless observational completeness is independently justified.

There is also a multi-area design constraint: [admitting a new policy](../../internal/publicingest/repository.go#L60) replaces the active state for a source and resets checkpoint/coverage. Reusing that mutation on every country selection would disrupt monitoring. Decide how independent approved retrieval profiles share provider identity and account-level quotas without overwriting each other's state.

**Closure/proof:** an area/time coverage result that distinguishes outside footprint, partial/unknown coverage, gated access, stale retrieval, successful empty retrieval and usable evidence. Country selection remains read-only with respect to ingestion authority. Exercise each state with scoped identities and retained admission history. GAP-06 and GAP-13 depend on this.

## GAP-06 — Current monitoring versus historical study · P0

**Assessment: confirmed semantic gap; historical event semantics already exist.**

[Default filters](../../apps/operator-ui/src/features/public-context/filters.ts#L8) use `latest` without an observation interval. Here `latest` means the latest retained revision of a record, not a recently observed event. The map accordingly displays “All retained dates.” Existing `known_at` and `effective_at` semantics are valuable and should not be collapsed into a generic date filter.

The [map API](../../internal/reference/map.go#L144) honestly returns `historical_unsupported` for unsupported families. However, for ordinary latest-view requests it [clears observation bounds for imagery and coverage](../../internal/reference/map.go#L202). Old captures can therefore remain under a recent-period header. Separately dated context is a valid feature, but its exception must be visible in the main layer/scope language. Workbench scope does not currently carry the same temporal fields as map filtering.

**Closure:** explicit monitoring/historical mode, a declared rolling monitoring window, visible timezone and freshness, and separate observation, acquisition, provider-revision, receipt/publication and analysis times. Unsupported historical data remains unavailable. Static historical context must be visibly identified as outside the monitoring window or intentionally included.

**Proof:** late arrivals, corrected/withdrawn reports, stale sources, historical imagery, timezone boundaries and empty queries. Verify the period at every downstream handoff. Define a coverage-aware quiet-state explanation with GAP-05 before treating “no results” as a useful monitoring outcome.

## GAP-07 — Regional/worldwide scale · P1

**Assessment: qualification and completeness gap, not absence of scaling work.**

The implementation already provides [event aggregates](../../internal/publicevents/map.go#L16), [other-family aggregates and reference vector tiles](../../internal/reference/map.go#L208), complete matching totals for map families, and separate detail queries. [Geographic integration tests](../../internal/integration/geographic_map_test.go) inspect exact scopes, history, aggregates above 2,000 records, tile access and isolation.

The remaining problem is the declared workload envelope: density, geometry complexity, tile size, query cost, concurrency, readability and actionable drill-down. Limits are inconsistent across surfaces. Geographic lists stop at 300; [captures stop at 100](../../internal/imagery/captures.go#L95) and their [HTTP response](../../internal/imagery/capture_http.go#L21) exposes only items, without total/truncation or paging.

[Study geometry validation](../../internal/publicevents/geometry.go#L11) rejects date-line crossings and coordinates beyond ±85°, and limits polygon complexity/extent. These are explicit restrictions, not evidence of silently supported global geometry. The user needs an actual split/narrow alternative, while broad country geometry and local processing retain different budgets.

**Closure/proof:** a declared workload for regional overview, country, subdivision and local detail; complete totals/subset semantics on every catalog; measured latency and bounded cancellation; maritime/date-line/high-latitude cases. Do not lift existing limits globally without replacing their resource protection. Begin these checks during MAP-1/2 rather than waiting until MAP-6.

## GAP-08 — Competing navigation concepts · P1

**Assessment: confirmed workflow gap despite substantial map/dock improvements.**

The compact map dock, [Systems grid](../../apps/operator-ui/src/features/workspace/SystemsWorkspace.tsx) and [persistent workbench host](../../apps/operator-ui/src/features/workspace/MapWorkbenches.tsx#L12) already provide reusable working surfaces. Replacing them wholesale would discard useful work.

The remaining vocabulary reflects implementation modules: workspaces, workbenches, inner imagery tabs, technical context packages and a simulation entry inside the ordinary map chooser. More consequentially, the pinned workbench and current map can have different scopes. The existing “Use current map area” action acknowledges that distinction, but export bypasses the host entirely.

**Closure:** make the task progression visible: area/period → evidence → investigation → finding/review → brief/export. Keep Systems for relevant content, coverage and work needing attention. State whether a view follows the current selection or a pinned investigation revision; preserve drafts without silently rebinding them.

**Proof:** a new analyst can find evidence, inspect provenance, resume a case and retrieve its output using task language. Test map, Systems, direct links and back/forward navigation. GAP-01 and GAP-03 establish the phase and scope behavior; GAP-09 supplies the durable object navigation should follow.

## GAP-09 — Durable investigation · P1

**Assessment: confirmed missing product aggregate; durable evidence should be reused.**

The [workspace preference hook](../../apps/operator-ui/src/features/workspace/preferences.ts#L5) stores device-local settings. [Map bookmarks](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L65) retain view settings and links, not a case. [Mounted workbenches](../../apps/operator-ui/src/features/workspace/MapWorkbenches.tsx#L12) retain in-memory drafts while switching tools. Review revisions and analysis jobs are durable, but no investigation aggregate containing question, owner, scope, evidence selection, notes, handoff and outputs was found in the inspected API/migration paths.

**Closure:** a server-persisted investigation with explicit identity, owner/access, versioned area and period, exact evidence references, notes/analysis links, review assignments and output revisions. Store evidence references rather than editable copies. Use revision/conflict checks for simultaneous edits and define what a recipient can access after handoff.

**Proof:** save, close the browser, reopen on another authorized session/device, resume and hand off the same case. Include a concurrent edit and a revoked/inaccessible evidence reference. Browser preference retention alone does not meet this gate.

Define the investigation identity and scope contract in MAP-1, then build the smallest usable case early enough to drive MAP-3 workflows. Full reporting can remain MAP-4.

## GAP-10 — Repeatable imagery usefulness · P1

**Assessment: repeatability gap; acquisition and analysis are real implemented capabilities.**

[Capture jobs](../../internal/imagery/captures.go#L110) validate scene coverage and rights, freeze input, deduplicate by request key, enforce budgets and use expiring worker leases. The [raster worker](../../scripts/data/capture_worker.py) performs processing outside the synchronous API. [NDVI comparison](../../internal/imageryanalysis/ndvi.go#L135) checks aligned grids, dates, platform/tile/orbit/baseline and common clear pixels. Valid zero-change output is distinct from unsuitable inputs.

The user workflow still defaults to Jeddah, and [CaptureWorkbench](../../apps/operator-ui/src/features/imagery/CaptureWorkbench.tsx#L17) requests workspace-wide captures even though the API accepts region/geometry. Its comparison defaults select the earliest/latest retained captures, which need not belong to the current area. Changing to an oversized AOI leaves an earlier capture AOI in place. Backend validation can reject unsuitable inputs, but it cannot infer which area the analyst intended.

Automatic capture currently supports the admitted Earth Search Sentinel-2 Collection 1 path; the presence of other catalog metadata is not equivalent acquisition support. The retained installation has two distinct acquisition dates, not four independent temporal samples.

**Closure/proof:** selected-area/time discovery, deliberate local subset, supported acquisition, quality review, compatible-pair selection, retained analysis and human interpretation on additional areas. Show unsupported providers, clouds, alignment failure, empty discovery, zero change and retry states honestly. Reuse existing jobs; do not implement another synchronous raster pipeline. Depends on GAP-03/05/06 and later GAP-09/11.

## GAP-11 — Analyst deliverable · P1

**Assessment: confirmed output gap; reproducibility and review protections already exist.**

[ContextExport](../../apps/operator-ui/src/features/imagery/ContextExport.tsx#L43) downloads JSON. The [bundle contract](../../internal/contextpackage/domain.go#L76) is built around AOI, interval, assets, transformations, rights and approval. It is useful reproducibility infrastructure, but it does not constitute a question-led analytical brief.

[Review identity validation](../../internal/assessments/review_identity.go#L42) requires distinct resolved registered human identities. Preserve it. The development publication in the snapshot explicitly says it demonstrates account/workflow separation, not review by two actual people.

There is a further design constraint: current package rights use `AllowSimulationExport`. A readable brief, distribution of raw imagery and a future environment package are different output purposes. The brief must evaluate applicable output rights explicitly; neither renaming this flag nor copying all retained bytes establishes permission for a different use.

**Closure/proof:** a versioned readable brief with question, exact area/period, coverage, findings, cited evidence, maps/comparisons, uncertainty, limitations and review status, accompanied by a permitted reproducibility bundle. A recipient can understand it and resolve exact references. Define draft versus approved brief behavior and recipient access. Depends on investigation, scope, coverage and review contracts.

## GAP-12 — Development and working content · P1

**Assessment: confirmed default-catalog contamination; not evidence of partition failure.**

The [snapshot](../design/product-evaluation/installation-snapshot.json) contains 41 exercise-observation contexts, acceptance scenes and a published development workflow test. [Home](../../apps/operator-ui/src/routes/home.tsx#L9) and [readiness](../../internal/platform/readiness/http.go#L113) count authorized installed records. Imagery review lists are workspace-wide. Text in a conclusion or a scene name is not a reliable content classification.

Some synthetic indicators and explicit warnings already exist. They cannot substitute for a consistent purpose/provenance classification: real imagery used in a development acceptance test is still real source data, while its test publication is not an operationally reviewed conclusion.

**Closure:** classify content purpose separately from whether inputs are synthetic or real. Apply ordinary catalog/count defaults accordingly, with support/admin access to retained history. An append-only classification sidecar or equivalent versioned metadata can preserve immutable records without rewriting their meaning.

**Proof:** an ordinary analyst sees relevant source content and investigations; development publications and old exercise contexts do not inflate approved-output or usable-study counts. Counts distinguish source versions, distinct acquisitions, compatible studies and approved outputs. Coordinate with GAP-01/13 and retain exact historical identities.

## GAP-13 — Task readiness · P1

**Assessment: confirmed mismatch between platform readiness and analyst readiness.**

[Readiness](../../internal/platform/readiness/http.go#L113) marks a feature `AVAILABLE` when its scoped table count is positive; workers are `READY` based on recent heartbeats. It has no selected AOI, period or question parameter. A successful storage sentinel read does not validate every retained artifact. [SourceStatus](../../apps/operator-ui/src/features/public-context/SourceStatus.tsx) adds provider health and next actions, but is still source-oriented.

Consequently, four successful captures anywhere can make imagery available even when none can answer the current question. A healthy worker may have no permitted provider access or compatible inputs. Conversely, a quiet successful retrieval should not look broken.

**Closure:** keep operational readiness, but add task eligibility: evidence/coverage for the selected scope, supported operation, compatible dates/inputs, current rights and access, queue capacity, relevant worker/storage availability, and an understandable next action. Preserve distinct unavailable, unsupported, pending, stale, empty and ready states.

**Proof:** a healthy stack with no selected-area data; data present but processing unavailable; expired provider access; stalled work; a valid empty result; and recovery. The task summary must reflect each accurately without exposing private credentials. Depends on GAP-05/06/10 and the investigation question.

## GAP-14 — Wider workloads and unattended operation · P1

**Assessment: a qualification gap; recovery foundations have meaningful existing evidence.**

The [recovery acceptance](DELIVERY_RECOVERY_ACCEPTANCE.md) documents worker/lease recovery, storage failure, credential expiry, interrupted setup and paired restore with NDVI reproduction. Its performance measurements are explicitly a bounded exercise profile, including a virtual hour rather than a full wall-clock monitoring soak. They cannot establish regional map performance or multi-area provider scheduling.

Area-change handling is also partly implemented: [map queries](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx#L97) pass cancellation signals and key results by filters; [workspace switching](../../apps/operator-ui/src/components/WorkspaceProvider.tsx#L21) aborts requests and clears caches. Do not report these protections as absent. Hidden workbenches remain mounted and several queries poll, so navigation history can increase background workload; this needs measurement.

Concrete growth limits include four active captures and [a 1,000-row total capture budget](../../internal/imagery/captures.go#L166). Because the latter counts retained jobs rather than only pending work, reaching it stops new acquisition unless a deliberate archive/quota design is added. UI UUIDs also mean retrying an uncertain submission with a new key can create another job despite backend idempotency support.

**Closure/proof:** agree hardware, data density, concurrent analysts, area count, provider quotas, retention, latency and recovery targets; measure the map workflow against them. Cover rapid area changes, cancellation, lost responses, worker/storage/credential faults and restore of investigations plus artifacts. Begin budget and fault criteria early; final unattended qualification belongs to MAP-6.

## GAP-15 — International and role-focused usability · P1

**Assessment: partially supported, insufficiently qualified.**

Geographic aliases include Arabic source names, and the [geographic integration test](../../internal/integration/geographic_map_test.go#L46) includes Arabic search. Existing browser tests exercise keyboard geometry and narrow layouts. These are useful foundations, not evidence of a complete localized analyst workflow.

[The document language](../../apps/operator-ui/index.html#L6) is English. [Common time formatting](../../apps/operator-ui/src/lib/format.ts#L3) uses local browser time with no timezone label and sometimes no year, while [dateLabel](../../apps/operator-ui/src/features/workspace/preferences.ts#L17) omits time-of-day. The coordinate entry expects longitude/latitude, whereas other presentation helpers display latitude/longitude. The labels must make this ordering unmistakable across regions.

**Closure/proof:** test intended analyst and reviewer grants separately from the broad development account; Arabic/English aliases and mixed-script text; declared timezone/year; supported coordinate order, CRS/datum and units; keyboard/focus and readable narrow layouts. Missing permissions should explain available actions without presenting exercise setup or credential-file paths as routine analyst work.

Full Arabic application localization is a product decision distinct from Arabic place search. Do not quietly treat one as proof of the other. Wrong-area/time interpretation blocks acceptance even when the layout itself renders correctly.

## GAP-16 — Portable geographic environment preparation · P2

**Assessment: real preparation exists, but the implementation is tied to one area.**

[prepare_geography.py](../../scripts/data/prepare_geography.py#L157) uses a fixed DEM tile, Jeddah bounds and origin; it retains the geoid transformation and validates a round trip. [prepare_environment.py](../../scripts/data/prepare_environment.py#L12) uses those constants in texture coordinates, terrain lookup, output naming and manifest construction. It also generates actor-model fixtures at the end, which should not be part of a new environment-only task boundary.

The existing manifest already records source hashes, frame, input/output vertical datum and limitations. Preserve those facts. A mathematically successful round trip checks transform consistency, not terrain accuracy or suitability of a transformation grid for a different region.

**Closure:** an area-parameterized durable preparation request: permitted source selection → retained/cropped inputs → validated geographic artifacts and manifest. Resolve DEM coverage, local frame, horizontal/vertical transformations and area of use for the selected area; record estimated building properties, omitted geometry, dates/rights, size/LOD budgets and parent-byte lineage.

**Proof:** prepare another supported area without application-code edits; independently check georeferencing and transformation applicability, missing tiles, omissions, integrity and reusable output. This gate contains no actors, runtime clocks, synthetic sensors, exercise start or replay. Depends on trustworthy GAP-03/04/05/10 foundations.

## GAP-17 — Independent usefulness · P1

**Assessment: absent qualification evidence; no justified completion percentage.**

The inspected evidence comprises automated tests, developer walkthroughs and an account-separation workflow test. It does not establish independent target users completing the intended map-first task. For example, the [imagery browser journey](../../apps/operator-ui/tests/recovery/workspace-redesign.spec.ts#L67) confirms a prepared comparison, proposal and export screen, but does not assert that a newly selected country's geometry and period survive into a useful deliverable.

**Closure:** observe analysts and reviewers working from a stated question, without developer preparation beyond declared normal setup. Include multiple Middle Eastern and international areas, no-data/quiet states, denied access, a fresh supported acquisition, reopening a case, review and reading the resulting brief.

Record completion, assistance, scope errors, evidence/uncertainty comprehension and output usefulness against their present workflow. Two accounts operated by one developer do not demonstrate independent human review. Any wrong-area or wrong-period interpretation is blocking.

Start short formative sessions after the first corrected workflow. MAP-6 should qualify and close the phase, not provide the first encounter with real users. Exact participants and success thresholds require a product decision before claiming this gap closed.

## GAP-18 — Documentation and approval state · P0

**Assessment: partly corrected, still inconsistent.**

[Current scope](../../EMAD_CURRENT_SCOPE.md), [README](../../README.md#L3), [PRODUCT](../../PRODUCT.md#L3) and the [v2 plan header](../../EMAD_IMPLEMENTATION_PLAN_v2.md#L3) already establish the correction. It would be inaccurate to say no phase boundary has been recorded.

However, adjacent text still calls v2 the forward roadmap, describes a current two-pillar implementation, and carries broad completion language. The active product brief mostly remains the historical C2 document. Readers can follow conflicting statements without an explicit active backlog/status vocabulary.

**Closure:** a concise current map-first product brief and active backlog, with historical plans clearly subordinate. For each outcome distinguish implemented, installed, functionally verified, geographically covered and independently user-validated. Keep evidence dated and tied to its tested version/profile. A completed historical simulation increment must never generate the next current task automatically.

**Proof:** trace “what should we do next?” from README, product brief, implementation plan and delivery status; each leads to the same current phase and open acceptance gates. No deletion of old reports or retroactive reinterpretation of their evidence is necessary.

## Refinements to the proposed delivery sequence

Keep MAP-0 through MAP-6 as outcome gates, with these dependency adjustments:

1. **Contain the geographic substitution immediately within MAP-0/1.** Remove implicit unrelated-area behavior across export, capture and Systems together. Invalid/missing scope should stop or request a subset. This should not wait for complete global reference installation.
2. **Define area, period and investigation identity together in MAP-1.** Include exact-edition reads, temporal semantics, parent/subset relationships, permitted access, pinned drafts and a clear revision-change policy. A shared bbox helper alone will not close the issue.
3. **Separate reference expansion from acquisition authority in MAP-2.** More countries in the navigation catalog must not mutate connector admissions. Resolve multi-area profile identity and account-level quota sharing before enabling concurrent regional monitoring.
4. **Build a minimal persisted investigation during the first daily-workflow increment.** Its complete review/brief functionality can finish in MAP-4. Otherwise MAP-3 will keep calling browser filters an investigation and later require another workflow refactor.
5. **Treat time consistency, fixtures, accessibility and reliability as exit criteria at every gate.** Establish budgets and fault cases early, then run the final unattended profile at MAP-6. Begin independent formative testing before the final gate.
6. **Run MAP-5 only after geographic input contracts are stable.** Its output is a portable environment package. It does not reopen the deferred simulation sequence.

The core dependency is: **correct scope → explicit coverage/time meaning → durable investigation → repeatable acquisition/analysis → reviewed brief → qualified use**. Regional reference expansion supplies this flow; environment preparation reuses its geographic foundations.

## Implementation decisions to settle before broadening the backlog

- **Region configuration:** the initial versioned Middle East definition, its geographic meaning and source-backed membership/geometry; examples used for qualification remain samples rather than limits.
- **Time contract:** monitoring windows by source family, historical modes, timezone display and treatment of separately dated background imagery.
- **Scope persistence:** exact pinned geometry versus latest-version discovery, explicit subset selection and what happens when referenced geometry becomes inaccessible.
- **Investigation lifecycle:** ownership, collaborators/reviewer handoff, conflict handling, revisions, retention and recipient access to evidence/outputs.
- **Acquisition profiles:** independent area/time requests, durable idempotency, shared provider quotas, admission changes and permitted reuse of retained inputs.
- **Output purpose:** readable brief format, draft/approved behavior, permissible redistribution and its distinction from raw-data/environment export.
- **Qualification envelope:** target users, representative regions and conditions, data volumes, concurrency, latency/recovery limits and unattended duration.

These decisions constrain implementation; they do not require choosing a simulation engine or expanding runtime scope.

## First acceptance scenarios worth making authoritative

1. Select a country, open export and imagery, and verify that both retain the actual selection or demand an explicit local subset. Repeat for subdivision, named study, drawn polygon, viewport and restored case.
2. Save a case against geographic edition A, admit edition B, and reopen A with its original meaning. A later edit may deliberately choose B; the old output remains reproducible.
3. Change area/time while requests are slow and multiple workbenches have drafts. No stale response may masquerade as the new selection; pinned drafts remain clearly labeled and do not mutate silently.
4. Select an unobserved area with healthy workers. Report missing coverage, not readiness to answer the question and not a factual absence. Contrast with a successful zero-result retrieval whose limitations are stated.
5. Use monitoring and historical modes with late/corrected reports and old imagery. The user can explain which dates apply to each visible layer and to the brief.
6. Acquire supported imagery outside the prepared fixture, review quality, compare compatible inputs, save the investigation, hand it to an independent reviewer and produce a readable, cited result. Include a valid zero-change result and an unsuitable-input failure.
7. Exercise catalog pagination/budgets, dense regional discovery, date-line/high-latitude handling, provider quota/credential failure, worker loss and paired restore. Restore the investigation-to-evidence links as well as the bytes.

The immediate delivery target should be a correctly scoped, coverage-aware analyst journey that produces a durable result. New regions and larger workloads should qualify that same journey rather than introduce more prepared demonstrations.
