# EMAD product evaluation

> **User scope correction, 25 October 2026:** The current phase is map/Public Context for the **Middle East and the rest of the world**, with **environment preparation only** for future simulation. The earlier recommendations below to center a pilot on Jeddah or continue a simulation pilot in parallel are superseded by the [map-first gap report](../implementation/MAP_FIRST_PLAN.md#gap-report) and [current scope](../../EMAD_CURRENT_SCOPE.md). Observed evidence remains a historical record; simulation design is a later discussion.

25 October 2026. Expert product review of the running installation, current UI/code and retained acceptance evidence. This is an assessment, not a new approved implementation plan. No application behavior or installed evidence was changed for this review.

## Verdict

EMAD has a credible technical foundation and several real, bounded workflows. Its current maturity is an engineering prototype with usable pilot capabilities. It is not yet a coherent, validated everyday intelligence product or a sophisticated simulation environment.

There is something worth pursuing. The strongest current value is retaining geographically grounded evidence, reproducing an analysis, reviewing an interpretation and reusing versioned geography in a separate exercise. The product still makes people assemble that value from modules. Technical completion has been used too freely as a proxy for product completion.

The answer to “is it confusing?” is **yes**. Recent navigation work improves discovery and screen space. It does not finish the underlying information architecture, analyst workflow or product focus. Another round of decorative UI changes would have diminishing value until those issues are addressed.

## Evidence and limits of this review

- [Installation snapshot](product-evaluation/installation-snapshot.json): read-only authenticated API inspection on 25 October, including scoped Saudi queries and ordinary navigation state.
- [Browser walkthrough](product-evaluation/walkthrough.json): normal Home, default Map, geographic selection and context-export handoff. [Home](product-evaluation/home.png), [Map](product-evaluation/default-map.png), [scope mismatch](product-evaluation/export-context.png).
- Current implementations of Home, Public Atlas, workbench navigation, imagery analysis, context export, Studio and scoped authorization.
- [Recovery acceptance](../implementation/HISTORICAL_RECOVERY.md#recovery-acceptance), [one virtual-hour measurement](../implementation/evidence/recovery/one-hour-accepted-run.json) and [graphics measurement](../implementation/evidence/recovery/graphics-acceptance.json).

The recent implementation runs passed 84 frontend checks, all 34 relevant live checks after targeted timing/locator corrections, and 16 production real-backend browser scenarios. Those demonstrate specified behavior and important isolation properties. They do not establish that independent users understand the product, need its output or can complete valuable work unaided. I found no recorded independent usability study or pilot-outcome evidence in the inspected materials. Research outside the repository may exist.

## What it actually provides

### Public Context & Intelligence

Demonstrated capabilities include geographic browsing and source inspection, retained provider observations, source/retrieval history, real satellite windows and masks, deterministic NDVI comparison, reviewable assessments and validated context bundles. These are substantial building blocks for an evidence-based geospatial analysis workflow.

The installed content is much narrower than the menu implies:

- 24 reference dataset versions. That is a version inventory, not 24 complete independent geographic products.
- 19 imagery catalogue revisions, four successful capture records and **two distinct acquisition dates: 1 and 31 March 2025**. Four records do not mean four independent imagery observations.
- One successful NDVI-change job and two successful preparation jobs.
- One real imagery assessment remains **PROPOSED**. The published assessment is explicitly a **DEVELOPMENT WORKFLOW TEST**; it proves account/workflow behavior, not an independently reviewed intelligence conclusion by two actual people.
- The exact Saudi polygon currently returns five retained earthquake reports, 26 station-weather records, two imagery acquisitions and one derived finding. Thermal detections, official bulletins, aircraft and vessels return zero in that query.
- USGS, station weather, FIRMS, OpenSky and the Copernicus catalogue report current retrieval health at the snapshot. GDACS is stale, Earth Search reports unavailable, and AIS is not admitted. Historical/static-source health requires separate interpretation from a live feed.

Zero results are not proof of absence, broken integration or complete regional coverage. Successful retrieval is also not proof that the current user has useful observations for the chosen task. The 3,194 retained provider records reported by readiness measure stored evidence, not 3,194 distinct actionable discoveries.

**Useful today:** an experienced analyst can inspect selected regional evidence, examine the retained Jeddah before/after imagery, reproduce the indicator and prepare an interpretation with its inputs and limitations. This is useful for a guided historical study or technical pilot.

**Not established:** a repeatable, independently usable daily workflow that discovers an important issue, determines relevance, combines supporting and conflicting evidence, coordinates human review and delivers a clear brief to a recipient. General OSINT research, multi-source reasoning and AI interpretation should not be inferred from the presence of an Intelligence menu. The demonstrated imagery calculation is a specific deterministic indicator.

### 3D Modeling & Simulation

The native simulation module is real: prepared Jeddah geography, validated visual models, scene authoring, independent runtime launch, virtual clocks, synthetic perception, controlled virtual tasking, recording, replay and privileged evaluation all have implementation and bounded acceptance evidence. Protecting truth from operators is valuable and should remain intact.

The current installation has two frozen scenes, including an acceptance-test scene, and seven validated model records. The recovery scope describes six recognizable project-authored model types plus existing assets. The prepared real environment is concentrated in an approximately 8 km × 8 km Jeddah area. Its surface elevation and estimated building dimensions have declared limitations.

**Useful today:** engineering integration exercises, controlled demonstrations of perception/tasking/replay, and an expert-led pilot with declared abstract behavior and a clear training objective.

**Not established:** realistic vehicle/flight behavior, calibrated sensing, operationally validated training outcomes, broad environment authoring, large-world streaming or production-scale multiuser exercises. These are different levels of product capability; several are explicitly deferred S4 work, not failures to implement the agreed bounded recovery.

The retained one-hour result represents one hour of **virtual** time at 20× acceleration with four actors and two sensors, taking approximately 644 wall-clock seconds. It reports no run failures, but approximately 90.4% of observations arrived over the metric's 1,000-virtual-ms lateness threshold; all four observed actors were associated with more than one recorded track. These diagnostics need explanation against the intended sensor/correlation profile and acceptance tolerances. They do not establish a normal-speed failure, nor should a passing run be equated with validated timing or tracking quality. Tasking has separate acceptance evidence; this duration profile's command metrics are undefined because their denominators are zero.

The populated graphics sample is about 3.6 seconds of explicit software rendering. It cannot establish a hardware performance envelope or long-session usability. This is a reason to qualify the actual workload, not to replace Three.js speculatively.

## Why it remains confusing

1. **Navigation is still ahead of the user’s task model.** Public Context/Simulation, workspace/environment/run, Systems, the map, workbenches and inner analysis tabs are different levels of state. Their engineering meanings are valid, but a user still needs to learn too many concepts before answering a question.
2. **The same activity has several names and entry points.** Assessments, Review desk and Assessment review lead into related work. Systems, its standalone route and its map-hosted view add another layer. A cleaner chooser reduces clutter but does not resolve the hierarchy.
3. **“Investigation” mostly means view state.** I found saved filters, local device preferences, bookmarks, URLs and retained assessment/package objects. I did not find a first-class analyst case that coherently owns the question, area, period, selected evidence, notes, analytical work, review state and final brief.
4. **Time and coverage are too much work to interpret.** The fresh default map is “World · configured evidence” with “All retained dates.” This is a legitimate historical view, but weak as the first answer to “what is happening now?” Global reference navigation does not imply global feed coverage. A source being current, a record being recent and a region being adequately observed are separate questions.
5. **Development and acceptance artifacts compete with normal content.** The current development account sees 42 contexts, including old/undated exercise contexts. That does not mean every restricted production user would see 42. Nevertheless, development-publication records and an acceptance scene appear in the installed working catalog. Counts and green readiness states can overstate how much useful analyst content exists.
6. **The default experience serves a broad development identity.** Guards and separate author/operator views already exist. The remaining need is a role-led starting experience: an analyst continues an investigation; an author continues a scenario; an operator opens an assigned exercise; a reviewer sees work awaiting review.
7. **The output is too technical.** A reproducible JSON context bundle is useful infrastructure. It does not replace a readable intelligence brief or an understandable exercise report with a question, conclusion, map, evidence, limitations and review history.

## A concrete blocking workflow defect

In a fresh browser, I selected **Saudi Arabia**, opened **Context packages** from the map and clicked **Use investigation region**. The export AOI became:

`39.144,21.464,39.216,21.536`

That is the fixed Jeddah rectangle, not the selected country. The implementation reads a saved bbox and falls back to Jeddah; the country polygon selected through the map host is not propagated into this action. The [recorded walkthrough](product-evaluation/walkthrough.json) and [screenshot](product-evaluation/export-context.png) preserve the reproduction.

No package was approved or created during this review. Server validation may reject incompatible selections, but the region-use action itself is misleading. It must be corrected before a broader pilot. The product must explicitly explain any supported rectangle conversion or size restriction rather than substituting a different region.

## The most important product gaps

**P0 — Trustworthy context across the complete journey.** Fix the export region handoff and test country, named study area, drawn polygon, viewport and saved-case paths. Area, time, source coverage and retained input versions must stay coherent as a user moves from map to analysis to review to output.

**P0 — One target user and one repeatable outcome per pilot.** “A sophisticated intelligence and simulation platform” is a vision. It does not specify who returns tomorrow or which deliverable makes their work better. Define the initial analyst question, recipient and success criterion. Keep both product pillars, with a focused, independently useful workflow in each.

**P1 — A durable investigation and readable brief.** Introduce the missing case spine, evidence selection/notes, review handoff and final readable output. Reuse the existing evidence, assessment and bundle contracts. Preserve technical manifests as an accompanying reproducibility artifact.

**P1 — Useful quiet-state and coverage behavior.** Start from a declared area and period; distinguish current monitoring, retained history and unavailable coverage. A quiet but well-observed region should produce a useful status explanation. A poorly observed region should say what is unknown. Adding irrelevant markers to make the map busy would reduce its value.

**P1 — Curated working content and lifecycle.** Keep immutable development evidence, but classify/filter it out of ordinary defaults. Give exercises a purpose, owner and understandable operator-facing label; separate reusable scenarios from execution history and archived recordings. Keep Systems as a meaningful overview with task-specific readiness and next actions.

**P1 — Repeatability beyond the prepared installation.** Demonstrate a second area/date range and an end-to-end fresh acquisition without developer intervention. Validate outages, recovery and source freshness over an unattended pilot. A ready worker and an available feature count do not establish that the current analytical question is answerable.

**P1 — Measured simulation usefulness.** Define one training/experiment objective, the modeled assumptions, acceptable timing/correlation error and an after-action conclusion. Explain the accelerated benchmark diagnostics. Measure a realistic populated session on declared hardware and normal/accelerated clocks. Let a real operator complete the exercise and interpret its result.

**Validation gap — Independent users and demand.** Automated tests cannot supply this evidence. Observe target users attempting tasks without a developer explaining partitions, hashes, datums or module history. Record task completion, context errors, help requests, review quality and whether the resulting output is useful to its intended recipient. Commercial demand and willingness to adopt remain unproven by this repository.

## Recommended next sequence

1. Correct the scope handoff and audit equivalent area/time transitions. Reconcile the current product definition; `PRODUCT.md` still contains extensive superseded C2-first positioning beneath its historical banner.
2. Define an analyst pilot that matches actual content. A defensible **Jeddah before/after environmental-change investigation** is the strongest immediately demonstrable candidate: named question → explicit AOI/period → quality/coverage review → retained comparison → human interpretation → independent review → readable brief. NDVI remains an indicator, not proof of cause or vegetation loss. Keep the previously selected natural-hazard context pilot explicit rather than implying that unrelated source counts answer it.
3. Make that workflow a durable guided case, then repeat it with a new AOI/date range. Decide its usability gates before implementation. Suggested gates include no wrong-region handoffs, clear handling of absent coverage, traceable evidence in the output, and successful independent review without developer assistance.
4. Continue a focused native simulation pilot in parallel: prepare/choose an environment → author a purposeful scenario → operate permitted contacts/assets → replay → explain performance against the exercise objective. Keep fidelity claims bounded until validated.
5. Observe independent analyst, reviewer, scenario-author and exercise-operator sessions. Expand providers, scene scale and simulation fidelity in response to demonstrated task needs.

Do not rebuild the platform or assume that more modules, an engine switch or another broad redesign will establish product value. Preserve the infrastructure that now works and make the existing capabilities produce a small number of clear, repeatable, trusted outcomes.
