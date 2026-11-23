# Riyadh reports and access delivery

**The complete authored exercise runs through the Atlas Environment Library.** The selected geographic release, reviewed source admission and exact input SHA-256 bind each replay. The API returns only evidence available by the selected cutoff. An approved preparation stays a pinned local file and can be replayed again after source-rights checks. The endpoint processes submissions for the request.

## Run the authored control

1. Use the [owned exercise package](../../../testdata/simulation-inputs/riyadh-access-exercise.json). The retained-source admission for this tool uses dataset `civilian-road-exercise`, edition `v1`, analysis permission and explicit internal analytical output. Its terms cover the source items actually included in the package. Use the existing source-admission workflow and its normal operator roles. [Intake workflow](../../../internal/geoenv/admission.go)
2. In a visible Environment Library release, open **Riyadh reports and road access replay**. Select the package, fill the registered source and admission IDs, and provide the SHA-256 from the reviewed handoff. Use the evidence cutoff field as UTC.
3. At 06:45 UTC on 21 November 2026 the ticket is closed, the eastbound road is still closed, and the verified detour is 230 m. The scheduled works comparison blocks that detour while the observed route remains visible. The two similar reports about different obstructions are independently marked separate. A fourth report has unresolved location.
4. Pin that cutoff as the comparison. At 06:55 the cancelled permit stops affecting the planned route. By 07:05 the eastbound finding is available and the verified route is 120 m. The westbound edge remains closed. The west entrance is reachable and has a dated restoration task priced in SAR. The east entrance remains uncertain. The population result preserves zero, nodata and the known unallocated balance.

The route table includes verified edge order and shows uncertainty, disconnected destinations and paths outside the reviewed network as separate outcomes. Directions and incoming-edge turn restrictions come from the reviewed graph. When an accepted crosswalk pins a retained OSM feature to the same manifest, **Inspect retained road in 3D** uses that release’s verified road display. Associations suggested by the category and place rule require human review. Session decisions stay local to the chosen cutoff. Recorded evidence and adjudications come from the package. [Replay implementation](../../../internal/geoenv/civilian_access.go) · [Routing and population checks](../../../internal/geoenv/access_paths.go)

## Reproduce and score the exercise

Use the same replay engine from the command line with an already issued file and trusted input digest. For a retained road crosswalk also pass the trusted `--manifest-sha256`. The local tool runs on the received bytes and the recipient applies the package's terms.

```text
go run ./apps/emad-environment replay-access --input testdata/simulation-inputs/riyadh-access-exercise.json --sha256 7e12521d2e243295840606201767e50da1c82ec05750a562ed62ff16fa3b6bf8 --known-at 2026-11-21T06:45:00Z
python scripts/data/evaluate_civilian_access.py testdata/simulation-inputs/riyadh-access-exercise.json testdata/simulation-inputs/riyadh-access-answers.json testdata/simulation-inputs/riyadh-access-submissions.json
```

The evaluator pins the exact case package and requires every run to cover the same cases. It counts premature and unscorable clearances separately, counts assignments beyond each case’s independent allowance, and estimates verified reopening times with unresolved cases censored at the cutoff. An unreached 90th percentile stays unavailable. The authored controls include one deliberately early reopening, one unscorable decision, and one excess dispatch. Keep real answer files and participant submissions separate and accessible only to the reviewers authorized to score the exercise. [Evaluation reader](../../../scripts/data/evaluate_civilian_access.py)

## Prepare a source-backed package

Run the preparer with the pinned [reader dependencies](../../../scripts/data/reader-requirements.txt) for native road and raster operations. Its commands emit JSON on standard output. Use the reviewed local files and retain the acquisition, attribution and rights records.

| Command | Inputs and outputs |
| --- | --- |
| `osm` | The existing prepared ZIP and trusted manifest SHA-256. Checks the road member digest and extracts exact feature IDs, source tags and geometry for review. |
| `overture` | Bounded segment and connector GeoJSON exports from the same pinned edition. Keeps connector topology, native access and turn rules, exclusions and the OSM lineage match candidates. Optional `--osm-candidates` pins the retained road crosswalk. |
| `graph` | Candidate report and a signed-off review record. The review names candidate SHA-256, edition, reviewer, service class, accepted piece directions, turn restrictions, boundary connections and any accepted retained OSM feature IDs. It returns a bounded graph fragment. |
| `municipal` | The pinned Riyadh Q1 JSON and an authored availability schedule listing row ordinals and release times. Exports raw category labels, original receipt times and any published place label. Snapshot ticket status and repeat flags stay out of the replay. |
| `works` | Reviewed UTF-8 CSV with `id,edgeId,roadLabel,revision,availableAt,from,toExclusive,status`. Accepts scheduled and cancelled revisions. Leave unresolved matches blank. Until an authorized historical Balady file is available, use an authored schedule and attribute it accordingly. |
| `population` | Native WorldPop people-per-cell GeoTIFF and reviewed disjoint WGS84 service zones keyed by destination ID. Run once with `area-weighted` and once with `cell-center` when checking boundary sensitivity. Retains native cell identifiers and nodata. |
| `assemble` | Exercise definition and reviewed graph, plus selected municipal, works and population fragments. Appends working records and preserves distinct source identities. The final replay validates graph membership, record revisions, evidence times and population allocation. |

The [authored package](../../../testdata/simulation-inputs/riyadh-access-exercise.json) shows the schema for report, inspection, association and restoration records. Use pseudonymous local IDs and permitted deidentified narratives. For actual field use, a local reviewer approves the road directions, turn restrictions, and service-zone crosswalk and supplies dated access verdicts.

## Source-backed technical evidence

- The verifier read **3,703 OSM roads** from the pinned Riyadh prepared member. Overture edition **2026-11-18.0** returned 11,524 segments and 9,516 connectors over the buffered extraction. The tool derived 13,816 connector pieces and identified 9,680 Overture road records with candidate retained OSM lineage. The same underlying OSM record can contribute to both sources. [Overture schema and attribution](https://docs.overturemaps.org/guides/transportation/)
- The Saudi WorldPop R2025A 2024 raster matched the native WGS84 people-per-cell contract. The two **authored** test-zone estimates were 54.94 and 29.66 with area weighting, and 61.77 and 12.69 with cell-center allocation. These are software controls for boundary sensitivity. [WorldPop product](https://hub.worldpop.org/geodata/summary?id=75294)
- The local source-backed package contains a retained road crosswalk, selected 940 rows, native WorldPop cells and authored access and works conditions. Its pinned SHA-256 is `c6d4ab092af5f5ddb456131435329cb8f8bbabbae1f3de79d94771e0601e910e`. The source exports and package are in the workspace’s private qualification directory. The package references the Riyadh manifest `f338cbd833efa47fbf79c58702883e2e52fcd977987e6ede9e10f113f73fbeb6`.

**Field qualification remains open.** The available Balady guide describes the permit workflow and street-closure filter. A historical Riyadh permit export and independently reviewed Riyadh dispatch and inspection records were unavailable. The replay is established for authored controls and source-backed preparation. Real access, restoration, and dispatch-performance claims require those field records. [Balady excavation guide](https://balady.gov.sa/en/services/11386/guide)
