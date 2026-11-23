# Simulation data and feature delivery plan

Archived on 22 November 2026. The delivery records describe the implemented workflows and the evidence still needed for live qualification. [Active plan index](../../README.md) · [Remaining platform and evidence work](delivery.md#remaining-platform-and-evidence-work)

Reviewed 21 November 2026. The first data package supports a Riyadh dispatcher deciding which reports need an inspection and when a road can reopen. A second exercise uses civilian drone imagery to prioritize visits. Eastern coastal logistics and southern access disruptions use their own regional records.

Atlas can review Riyadh municipal workload, historical PortWatch activity, Copernicus flood-mapping products and historical Internet outage annotations through the selected release. It also replays road access, civilian aerial inspection, regional service comparisons, coastal evidence and supporting sources. [Implemented operations, tests and remaining work](delivery.md) · [Aerial review delivery](aerial-review-delivery.md) · [Regional response and coast delivery](regional-response-delivery.md) · [Supporting evidence delivery](supporting-sources-delivery.md)

| Build order | Source plans | First reviewable result | Dependency and proposed lead |
| --- | --- | --- | --- |
| 1 | [Municipal reports, roads, inspections and population](01-reports-roads-and-inspections.md) | Arabic report queue, two alternative road states and a traceable reopening decision | Data engineering can start the 940 and road imports together. A municipal reviewer supplies or adjudicates the inspection cases. |
| 2 | [Aerial evidence, drone logs and civil aviation](02-aerial-inspection-and-aviation.md) | A reviewer sees what imagery actually covers and can request a second inspection. An evaluator can compare predictions with hidden labels. | Imagery engineering can start on public research benchmarks during step 1. Local performance claims require local cases. Civil aviation history has separate access requirements. |
| 3 | [Regional response demand and maritime activity](03-regional-response-and-coast.md) | Asir workload comparison and a historical Jubail activity timeline | Regional data analyst checks reporting boundaries, periods and coverage. PortWatch can proceed independently of access to vessel reports. |
| By exercise | [Supporting source plans and deferred leads](04-supporting-sources.md) | One extra layer changes a named decision, such as postponing an inspection or explaining a delayed report | Exercise owner specifies the place, interval and decision. Weather and warning feeds are optional. |

**Delivery scope.** Each plan describes a proposed feature and how to demonstrate it. During infrastructure preparation, deliver the qualified data, offline reads, review cases and map inspection features. Dispatch comparisons, synthetic closures and AI evaluation belong to the later simulation phase. The [existing assessment](../../../docs/data/WORLDMONITOR_SOURCE_REVIEW_2026_11_17.md) distinguishes catalog leads from Saudi and research extensions. The [delivery record](../../../docs/implementation/SIMULATION_PREPARATION_DELIVERY.md) identifies the existing suitability checks, native reader, topology fixtures, field reads and offline kit.

## From provider data to something a user can inspect

World Monitor is the primary discovery reference. For normalization and acceptance, follow each entry through to the original provider and EMAD's registered product or proposed reader. [World Monitor catalog](https://www.worldmonitor.app/docs/data-sources) · [EMAD normalization dictionary](normalization.md)

1. A data steward records the source edition, geographic and temporal coverage, acquisition method, attribution, and permissions for each intended use, including retention, redistribution and model training where relevant. Start with one file, product or historical window. Keep the original digest and request parameters with the result. Use an owned fixture for software verification if provider access is pending.
2. Select the source's integration path in the field dictionary. For installed adapters, check the admitted product and retrieval limits. Proposed tables, notices, image labels and inspection records need a reader for their own record types. For a retrospective bulk file with unknown publication history, declare a hypothetical release schedule for the exercise. [Normalization dictionary](normalization.md) · [Adapter capabilities](../../../internal/publicingest/capabilities.go)
3. For native geographic layers, bind the selected members to a release and verify the offline kit under its issued terms. For each additional representation, demonstrate its reader against known answers, pin the input edition and test receipt before claiming a combined offline replay. The proposed Atlas view should show source, age, precision and missing evidence alongside each value. [Offline kit guide](../../../docs/implementation/OFFLINE_ENVIRONMENT_KIT_GUIDE.md)
4. For eastern or southern use, repeat the coverage check against the new area and date. Record which local fields were measured, imported as historical context or authored for training. The regional comparison must be readable even when a source is missing from one area.

## First exercise and evaluation

For the Riyadh trial, cross ordinary versus AI-assisted review with existing reports versus one additional clarification or image. Record the time and staff effort of obtaining that observation. Keep a simple rule as a separately scored baseline. Freeze the same initial state, evidence availability, crew schedule, random seed where used and adjudicated answers across matched conditions. Record the participant's decision time and the evidence IDs visible then. The evaluator should be able to reconstruct a decision without exposing later verdicts to the participant.

In the 3D viewer, highlight a report's area only when its published location resolves to a reviewed boundary. An unresolved report remains visible in the queue. Selecting a reviewed inspection should highlight its road segment and show the evidence available at the chosen time. Scrubbing across a partial reopening should change the accessible destinations and retain the still-closed direction. In a debris-response exercise, compare ground inspection alone with aerial imagery used to prioritize a visit. Measure verified reopening time, unnecessary visits and premature clearances on the same cases.

Use one reviewed junction, an alternative access route, one repeated report, one unrelated report with similar wording, and one closure that can disconnect part of the study area. Run twice on the same clock, changing only the availability of the alternative route. A ticket may finish before a crew verifies the road. A later inspection can reverse an earlier conclusion. A receiver should reconstruct both histories from the package.

Compare manual triage, an Arabic category/time/location rule, and AI-assisted suggestions. Keep the same report arrivals, available crews, inspection durations and adjudicated outcomes in every run. Counterbalance operator order or use matched cases to reduce learning effects. Hide repeat labels, final statuses, later imagery and inspection verdicts where those facts would have been unavailable at decision time. Record operator overrides. Synthetic cases can demonstrate workflow correctness. Estimate field performance only on separately adjudicated local cases.

| Measure | Definition for this exercise |
| --- | --- |
| Time to verified reopening | From first recordable awareness to verified restoration, with the direction and vehicle class specified. Report the cohort size, observation horizon and count still unresolved. Use time-to-event estimates for censored cases and mark an unreached quantile unestimable. Identify partial reopenings separately. |
| Premature reopening | Count reopenings contradicted by the independent inspection verdict, and report the number of reopening decisions examined. Report missing verdicts separately. |
| Duplicate dispatch | Count avoidable additional assignments to the same adjudicated obstruction. Exclude deliberate second inspections. Also count distinct incidents merged by mistake. |
| Accessibility | Compare reachable destinations and reviewed road distance before closure, during closure and after reopening. Report unknowns caused by clipping and missing network data. |
| Operator burden | Measure suggestions reviewed, manual corrections and time spent per resolved case. Report by case type and geography alongside errors. |

The U.S. Federal Highway Administration measures road clearance from awareness to lanes becoming available and distinguishes this from the last responder leaving. Adopt that distinction when defining local milestones. Fix pass criteria for field performance with the exercise owner before seeing held-out results. Use exact expected events for the controlled software cases. [FHWA incident measures](https://ops.fhwa.dot.gov/tim/preparedness/tim/performance_measures.htm)

**First handoff.** Deliver one manifest, a source limitations sheet, independently written expected answers and a read-only replay example. The receiver must be able to explain why two similar reports stayed separate, which edge closed, what evidence reopened it and what remained unknown. Production admission, real Saudi traffic history, a historical Balady export and Saudi inspection labels each need their own evidence.

## Data still needed to qualify the exercises

| Exercise and decision | Evidence already identified | Missing input, owner and closure test |
| --- | --- | --- |
| Riyadh dispatch and reopening | Verified Q1 arrival rows and a bounded road release | Municipal reviewer pairs permitted Arabic reports, candidate segments, direction, crew assignments, inspection and verified access times. A second reviewer adjudicates separate incidents, repeats, partial clearance and unresolved cases. |
| Image-assisted and cooperative civilian inspection | Dated scenes, overseas flood labels and single-platform research recordings | Imagery and fleet reviewers provide local scene masks, equipment and clock metadata, task assignments, overlapping and missed surveyed areas, actual collection intervals and independently reviewed image usability. Split by incident or site. Score missed tasks and inconclusive images as well as completed visits. |
| Asir service access | Annual regional workload figures | Road reviewer verifies a bounded southern network, destination and a dated closure. Regional analyst supplies comparable demand, travel times and staffed slots or records each as an exercise assumption. Reproduce a connected detour and a genuinely unreachable destination. |
| Jubail logistics and civil search | One historical port-day control and a bounded static coastal pilot | Logistics analyst checks daily series completeness and independently dated service interruptions. For a civil search evaluation, the coastal reviewer also needs time-aligned historical currents, wind and independent object/drifter observations with uncertainty. Reserve observations for scoring and count search area, missed recoveries and time to first verified sighting. |
| Service restoration and building re-entry | A dated area and building footprints can locate a reviewed case | Facilities reviewer supplies an owned or permitted example with entrance identifiers, baseline photos, inspection decisions, task dependencies and timestamps. For recovery cost or displacement also obtain dated quantities, price basis, occupancy assumptions and restoration evidence. Reproduce partial reopening and unresolved entrances independently. |
| Delayed reports and false alarms | Source lineage, public notices and conditional regional context | Exercise reviewer records message revisions, delivery and acknowledgment per recipient, independently resolved and unresolved cases, and examples with no incident. Replaying a delayed correction must reproduce different knowledge at the two desks. |

## Run the plan harness

From the repository root, run the first command for documentation and mapping checks. The project task also runs harness regression cases, Go ingestion and geographic tests, and the Python suitability suite using the local fixtures.

```text
python scripts/ops/check_simulation_source_plans.py
task test:simulation-source-plans
```

Use `--riyadh-q1 PATH` to check a local copy of the pinned municipal JSON, and `--portwatch-day PATH` for the dated provider response linked in the coastal plan. Both are optional, read-only inputs. The harness reports which provider samples it actually checked. For qualification, record the input digest, reader version, target region and interval, baseline result and unresolved evidence with the relevant exercise. [Normalization dictionary](normalization.md) · [Harness](../../../scripts/ops/check_simulation_source_plans.py)

On 21 November the review covered 54 inputs. Eight map to installed public-ingest adapters, five to bounded pilots, 38 to proposed mappings and three to deferred sources. The municipal file check reproduced the pinned Q1 digest and the district-field exception. The dated PortWatch check reproduced four Jubail calls and zero Jazan calls. A single port-day does not establish year-round completeness. [Municipal controls](01-reports-roads-and-inspections.md) · [PortWatch control query](03-regional-response-and-coast.md)
