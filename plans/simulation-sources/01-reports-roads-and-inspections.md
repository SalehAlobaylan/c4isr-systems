# Riyadh reports, road access and verified inspections

Deliver these inputs as one exercise package. Each is also useful on the map by itself. The practical dependency is the connection between a report, a possible road segment and an independently timed inspection. Some municipal reports can only be located to an area. Preserve that uncertainty throughout the exercise.

### Riyadh Municipality 940 — Arabic report queue and workload replay

**Use.** Show a coordinator which categories are arriving, where reports cluster at the published location precision and which might refer to an already open case. Let reviewers accept or reject a suggested association and inspect the original Arabic label. Include ordinary pothole and construction-waste demand alongside exercise-authored debris reports. [Q1 municipal publication](https://alriyadh.gov.sa/ar/data-sets/940-service-requests/940-requests-group-q1-2026)

**Build.** Pin the Q1 JSON and normalize whitespace and category aliases in a separate dictionary. Preserve source spelling, raw receipt value, interpreted UTC instant, location label, status, closure field and repeat flag. Identify a row within an edition by source digest and ordinal. Track possible cross-edition matches separately. Import an identical file twice and require identical membership. Begin with counts by day and category. Before deriving hour-of-day or turnaround distributions, confirm source timezone, duration units and whether an in-progress case can carry a closure value.

**Prove.** Reproduce 6,820 JSON records, 524 in the cited asphalt/pothole category and 301 in the cited construction-waste category against the pinned digest. Flag the 13,640-row CSV as a different representation. Never sum both exports. In a manually labeled sample, count false merges and missed repeats by category. Review false merges before enabling suggestions for dispatch. Generalized locations can support workload estimates even if no road can be assigned. [Recorded file checks](../../docs/data/WORLDMONITOR_SOURCE_REVIEW_2026_11_17.md#what-the-riyadh-940-files-actually-contain)

### Retained OpenStreetMap and Overture — accessible routes under closures

**Use.** A planner selects a road segment and sees which destinations retain access if it closes. Show the connected alternative, disconnected destinations and paths that leave the study area. Start with path distance. Time estimates require separate evidence about travel speeds.

**Build.** Use retained OSM features for the reference view. Prepare one matching Overture transportation edition with segments, connectors, direction and applicable access/turn restrictions. Include a buffer around the study area to reduce false dead ends. Preserve source IDs and uncertain matches through clipping. Validate vehicle access for the intended civilian service class. Overture includes OSM-derived content, so agreement between them alone cannot verify a junction. Pin source attribution and ODbL terms. [Overture guide](https://docs.overturemaps.org/guides/transportation/) · [Segment schema](https://docs.overturemaps.org/schema/reference/transportation/segment/)

**Prove.** A reviewer supplies expected paths for a one-way link, an at-grade junction, a bridge crossing without a junction and an alternative route outside the clipped core. Close and reopen one reviewed edge. Expected reachability must change and recover without editing the source network. Preserve “outside known network” as a distinct outcome. [Existing topology fixture](../../docs/implementation/SIMULATION_PREPARATION_DELIVERY.md)

### Balady excavation permits — planned works calendar

**Use.** During a simulated closure, show whether a proposed detour already has scheduled works. Let the reviewer see the permit interval, closure description, match confidence and subsequent field observations.

**Build.** The published guide documents a street-closure filter. Establish an authorized historical sample, provider identifiers and coverage for one municipality. Parse planned dates, road reference, status and amendments as available. Attach ambiguous geometry to a review list. A permit creates a planned constraint. A dated inspection or other authorized observation establishes actual access. If there is no export, exercise the same interface with a clearly authored works schedule. [Balady excavation guide](https://balady.gov.sa/en/services/11386/guide)

**Prove.** A future permit leaves today's route available. A revised date changes only the later view. A cancelled permit and an independently observed closure can coexist. A reviewer can identify exactly which record caused a detour.

### Local civilian inspections — evidence for closing and reopening a road

**Use.** Give the dispatcher a history of reported obstruction, inspected condition, partial access and verified reopening. This is the missing source for judging an AI suggestion or comparing recovery procedures.

**Build.** Begin with adjudicated exercise fixtures. For a local pilot, collect permitted observations on ordinary obstructions or maintenance cases. Record segment or location uncertainty, inspection time, first available time, access by direction, reviewer, optional permitted image, confidence and revision history. Include clear roads, partial obstructions, unreadable images and contradictory reports. Keep an independent reviewer for evaluation labels. Minimize identifying material in field media. Model separate times for verification, dispatch, arrival and restoration. [FHWA milestone definitions](https://ops.fhwa.dot.gov/tim/preparedness/tim/performance_measures.htm)

**Prove.** Replay a case where a municipal ticket closes while the road is still obstructed, then a second inspection restores one direction only. At each cutoff the user sees only evidence available then. A disputed verdict remains disputed until resolved. An inspection fixture supports logic checks while local verdicts support claims about field accuracy.

### WorldPop — population served by an accessible area

**Use.** Estimate the resident population in areas whose access to a defined civilian service changes under a closure. Show before/after estimates, uncovered cells, input year and assumptions about which road access point serves each zone.

**Build.** Select a Saudi counts-per-cell edition and record the exact asset's terms and year. Define disjoint service zones using the reviewed network. Aggregate on the native grid, documenting the treatment of partially intersecting cells. Where the road-to-zone association is uncertain, show the range under plausible assignments. [Saudi product and release](https://hub.worldpop.org/geodata/summary?id=75294)

**Prove.** On a small known grid, conserve the population sum across disjoint zones within documented rounding, leave nodata unknown and prevent overlapping closures from double-counting residents. Compare a boundary-sensitive zone with a second aggregation rule. Describe the result as modeled residential access, with daytime occupancy and actual service use unresolved.
