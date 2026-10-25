# EMAD current product scope

**User scope correction: 25 October 2026.** This records the current phase and takes precedence over earlier implementation sequences and recommendations where they conflict. Historical implementation records and immutable evidence remain intact.

## Current objective

Build a useful **map-centered Public Context and Intelligence system for the Middle East and the rest of the world**.

- The Middle East is the priority operating region, not a restriction to Saudi Arabia or selected cities.
- Worldwide navigation, geographic selection, evidence discovery and bounded analytical work belong to the product scope. The interface must state where actual source coverage or detail is unavailable.
- Jeddah is an existing prepared dataset and regression fixture. It must not determine product identity, silently supply unrelated area defaults or constrain the geographic architecture.
- A global basemap is only one layer of worldwide support. Geographic references, source coverage, acquisition, queries, time handling and user workflows must support the broader scope coherently.

## Simulation boundary

Simulation will be discussed and designed separately with the user later. **Current work is limited to preparing reusable geographic environments and their supporting data foundations.**

Environment preparation can include validated terrain/elevation, imagery, roads/buildings, source rights, coordinates and vertical datums, bounded geographic processing, versioned artifacts and portable environment manifests. These should serve map/geospatial work now and allow later reuse.

Do not interpret environment preparation as authorization to develop simulation engines, runtime behavior, scenario/actor authoring, synthetic sensors, exercise tasking, recording/replay or simulation evaluation. Do not select the future simulation architecture or engine on the user's behalf. Existing implementation choices do not settle that later design.

Keep existing code, records and isolation protections unless an explicit migration or removal is agreed. Their existence is historical technical work, not approval to continue their development. The current map product must not depend on exercise execution for its usefulness or acceptance.

## Planning and delivery rules

1. Start from this scope and the [map-first gap report](docs/implementation/MAP_FIRST_GAP_REPORT_2026_10_25.md). Its implementation actions are recommendations for review; writing the report does not execute or approve a new implementation batch.
2. Do not automatically continue the earlier S1–S4 sequence or a parallel simulation pilot.
3. Preserve functioning Go/PostGIS/MapLibre infrastructure, evidence provenance, existing access boundaries and immutable history.
4. Keep public facts, provider reports, derived indicators and human assessments distinct. A country being selectable does not establish source coverage there.
5. Validate workflows across Middle Eastern and worldwide areas. Individual test locations are samples, not product boundaries.
6. Judge progress by correct, repeatable map/analyst outcomes and verified geographic coverage, not module counts, record counts or completion labels.

The older v2 architecture/data documents remain useful design references where compatible. Detailed simulation design and implementation require a later explicit discussion and agreed scope.
