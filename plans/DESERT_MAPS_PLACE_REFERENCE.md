# Desert Maps place names and geographic reference

**Implemented and verified with owned fixtures · 22 November 2026.** Deliver Arabic place search and an optional, attributed reference layer for EMAD’s Atlas and 3D viewer. A civilian exercise can refer to the exact admitted place and edition its participants saw. Native publication, optional thematic display and portable use each require the appropriate source grant. [Current product scope](../EMAD_CURRENT_SCOPE.md)

Shared staging runs migration 61. Live release remains pending publisher permission, an authorized first delivery, and rollout of the matching application build. The regional directory is available as an external reference. [Operator workflow and release status](../docs/data/DESERT_MAPS.md)

## Implementation evidence

- **Source and normalization.** All 13 regional maps are cataloged. The owned intake tests exercise split layers, duplicate names, missing geometry, plausible swapped axes, grant checks, thematic preservation and repeatable IDs. [Regional inventory](../docs/data/DESERT_MAPS.md) · [Normalizer tests](../scripts/data/test_desert_maps.py)
- **Atlas, thematic vectors and terrain.** The Atlas keeps same-name results separately selectable and preserves the study area when a place is selected. A PostGIS test admits and reads an unnamed line. The terrain test exercises an exact source label and an edition-scoped group on the prepared surface. [Atlas browser tests](../apps/operator-ui/tests/e2e/map-first-tasks.spec.ts) · [Terrain browser tests](../apps/operator-ui/tests/e2e/environment-library.spec.ts) · [PostGIS tests](../internal/integration/desert_maps_test.go)
- **Release and civilian handoff.** The named-point workflow stages against a reviewed head. The PostGIS test exercises an interrupted activation, a stale expected head, a correction and rollback. With the caller and source grants verified, online issuance can package an earlier exact place with its original attribution and digests. The offline verifier rejects altered or expired packages. [Handoff issuer](../internal/reference/place_handoff.go) · [Receiver command](../scripts/data/desert_maps_handoff.py)

## Source and first regions

The 22 November review counted **34,558 entries in 107 layers across 13 regional viewers**. Of these, 277 lacked a populated Arabic name field. Only 16 exposed a populated Arabic description field in the viewer attributes inspected. These are counts of published entries, including repeated labels and drawings. Use each subsequent publisher delivery’s own manifest as its acceptance baseline. The directory reserves rights and the linked store requires permission to copy or publish its maps. [Regional directory](https://sites.google.com/view/desertmaps/home) · [Publisher’s rights notice](https://touristmaps.store/ar/p/KbyxD)

| First region | Why it belongs in the first release | Regression to catch |
| --- | --- | --- |
| Riyadh | Existing detailed EMAD environment. The viewer has 6,046 entries and splits one water family into two layers. | Both layers survive grouping. Their 2,155 entries in the reviewed snapshot can be reconciled to the two original layers. |
| Eastern Province | There were 1,506 local-name entries and 1,421 distinct trimmed name strings in that layer. | Same-name candidates remain individually selectable and preserve their source spellings. |
| Jazan | Two secondary-name layers have identical titles. They contained 18 and 50 entries. | Both layers survive ingestion with distinct identities. The display family can report their combined 68 entries. |

## What each family contributes

| Publisher family | Entries reviewed | Platform representation and permitted interpretation |
| --- | ---: | --- |
| Primary names | 8,225 | Searchable place labels. Preserve the publisher’s priority. Assign a settlement subtype only when the entry or a reviewed source establishes it. |
| Local names | 8,082 | Independently searchable local toponyms. A reviewer can associate one with an official feature or keep it as a distinct place. |
| Water-related features | 8,279 | Hydrographic reference labels and supplied shapes. A dated water observation or an authored exercise extent has its own source and time interval. |
| Named water sources | 1,474 | Preserve the published name and supported subtype. Availability, potability and visitor access each require independent evidence. |
| Mountain and upland features | 5,562 | Preserve the anchor and broad landform class. The 3D viewer can place the label on a terrain surface. Surveyed height requires its own measurement. |
| Sand terrain | 1,170 | Names of sand features for landscape identification. Preserve the supplied geometry and any explicit subtype. |
| Desert tracks | 1,242 | Draw the supplied sketch or marker as a publisher reference. Connectivity, current passability and access remain unknown unless independently established. |
| Boundary drawings | 356 | Display an attributed outline. Use an admitted administrative boundary edition for containment and reporting. |
| Heritage | 164 | Display approved names at the permitted precision. Official designation, site footprint and visitor status each need an identified source. |
| Publisher hazard labels | 4 | Show as publisher notes with their date or an explicit unknown date. Official warnings retain their own authority and effective interval. |

## Normalized record

The unit of import is **one entry in one layer of one delivered map edition**. A search result, 3D label, reviewed alias and portable reference can all point back to that entry. Keep the fields below with the normalized member or its immutable edition manifest. Reuse EMAD’s existing acquisition, rights and time conventions. [Platform normalization rules](archive/simulation-sources/normalization.md)

| Field group | What to retain or derive |
| --- | --- |
| Source and identity | Source ID, publisher and original producer as supplied, map ID, layer ID and raw title, provider entry ID, delivery version, receipt, source digest, mapping version. Use a stable provider ID within its declared scope. If absent, use an edition-scoped entry key based on exact file identity and record position. Preserve cross-edition continuity only with evidence. |
| Names and classes | Original name, language, optional description, raw category, mapped family and optional evidenced subtype. Preserve an unfamiliar category as unmapped for review. Keep the source spelling, a derived Arabic search key and any reviewed aliases separately. |
| Geometry | Original geometry and CRS, normalized WGS84 longitude/latitude geometry, and its meaning as an anchor, drawn line or supplied area. Track a derived display anchor separately. Preserve missing dimensions and unknown accuracy. Check axis order, bounds, validity and empty shapes before publication. |
| Region and matches | Keep the region named by the publisher. Record administrative containment only against a pinned boundary edition. A match to another gazetteer carries the external feature ID, edition, decision, reasoning and review date. Ambiguous and unmatched are valid outcomes. GEOSA’s public classification workbook can inform terminology. Individual matches require a record-level source. [GEOSA open data](https://geosa.gov.sa/en/Pages/Open_data.aspx) |
| Time, rights and disposition | Separate publisher revision, observation or verification date when supplied, and EMAD retrieval time. Record rights by operation, attribution and expiry. Count each input once as published, retained for review, or excluded with a reason. Track search and display eligibility separately so a feature visible in both is counted once. |

For example, a fictional fixture containing two entries named **وادي المثال** should return two candidates with their original layers and regions. A search for **وَادِي المِثَال** should also find them. Selecting one pins its source entry. A dated fictional bulletin can refer to that selection while keeping its own author, timestamp and extent. The source record continues to describe the place.

## Delivery sequence and acceptance

### 1. Establish source rights and a delivery format

The **source steward** catalogs the 13 regional viewer links as external references. Request a versioned export directly from the publisher, with its region and layer inventory, identifiers, geometry conventions, completeness statement and correction or withdrawal process. Record permissions for raw retention, normalization and indexing, 2D/3D display, civilian training, offline distribution, attribution and expiry. Use owned fixtures for engineering while the decision is pending. [Publisher contact](https://touristmaps.store/ar/p/lWOBe) · [EMAD rights fields](../internal/publicingest/domain.go)

**Exit evidence.** A reviewer can see which operations are approved for which regions. An external reference opens its viewer and reports that EMAD holds no admitted edition. A display-only grant lacking the retention and analysis rights required by the native admission path leaves the source in external-reference mode. Offline delivery requires an explicit grant for that operation.

### 2. Build and qualify the normalizer

**Data engineering** implements one importer for the agreed publisher format. Preserve layer identity before mapping titles to the ten families. Emit a manifest and a reconciliation report per map and layer, including missing names, unsupported classes, invalid geometry and reasons for exclusions. Keep the original geometry at its supplied precision. A valid unnamed drawing can qualify for thematic display while it remains ineligible for named-place search.

**Exit evidence.** A two-edition owned fixture covers the three regional regressions above, an unnamed drawing, unknown category, missing geometry, swapped axes detected against a known control point, duplicate labels and a corrected name. Reprocessing identical input produces the same feature keys, normalized digest and counts. Reordering a delivery without stable provider IDs yields new edition-local identities and explicit continuity decisions. Each input has one disposition, and the sum matches the delivered manifest.

### 3. Release named points and Arabic search

The **geography backend and Atlas UI teams** publish eligible named points through the current `kind=place` admission. Check the catalog’s 200-byte name limit using UTF-8. Retain overlength originals with a review outcome. Use deterministic batches within **5,000 features and 8 MiB per edition**. Stage all batches under the source and activate their exact editions together. Use a parent key only when it resolves within that same-source release. Keep official geography from another source as a separate cross-reference. [Admission constraints](../internal/reference/geography.go) · [Release validation and activation](../internal/reference/releases.go)

Extend the existing name-and-alias search with a tested, versioned Arabic comparison rule for diacritics, tatweel and common alef variants. Preserve meaningful letter distinctions in the stored names. Rank exact source spellings first, then reviewed aliases and folded candidates. Show region, family, source and match basis. Let users choose among equal candidates. A point selection focuses the location while administrative area selection stays explicit. [Search normalization](../db/migrations/00048_geographic_catalog.sql) · [Atlas search](../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx)

**Exit evidence.** Exact queries surface every exact match ahead of folded-only results. The fixture demonstrates spelling variants, two real alternatives and no match. Keyboard and right-to-left results open the intended feature in the atlas. Missing grants, unavailable retrieval and a successful query with zero results produce distinguishable states. Existing geography searches still pass. An interrupted multi-batch import leaves the previous release visible.

### 4. Publish thematic drawings and 3D labels

The **environment data and viewer teams** add a versioned vector member and display recipe for authorized thematic points, lines and polygons. Retain all parts and identify the parent member and feature in derived display output. The current place catalog requires a name and point geometry for places. Environment display recipes are explicitly registered. Extend that system for the thematic layer. [Geography admission](../internal/reference/geography.go) · [Display recipes](../internal/geoenv/display_layers.go)

**Exit evidence.** A line remains a line, an unnamed drawing is inspectable by source reference, and a publisher boundary cannot become an administrative selection. A named feature selected in 2D opens the same source feature in 3D. Inspecting a terrain-anchored label identifies the terrain edition and leaves unknown source height unset. Check dense Arabic labels at regional and local zoom, attribution, mobile selection and the accessible list. A missing thematic layer leaves other atlas layers usable.

### 5. Qualify refresh, expansion and civilian handoff

The **release operator** compares each new publisher manifest with the last complete delivery, recording additions, corrections, explicit withdrawals and unresolved matches. Preserve the last complete visible release on partial retrieval. Stage a replacement against the expected active release. Roll back by selecting the previous complete release only while its terms still permit use. Exercise rights expiry through search, feature detail, display, caches and package issuance. Apply any retention or deletion obligations to retained material.

Qualify Riyadh, Eastern Province and Jazan first. Expand region by region across the remaining ten maps when coverage, rights and reconciliation are recorded for each. Show regional status individually. For portable civilian use, package only authorized members with their release, manifest and member digests, normalizer version, attribution, intended operation and expiry. [Source policy and retention](../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md) · [Offline receiver guide](../docs/implementation/OFFLINE_ENVIRONMENT_KIT_GUIDE.md)

**Exit evidence.** A rehearsal covers retry, partial retrieval, confirmed withdrawal, concurrent activation, rollback and rights expiry. An independent receiver resolves the selected fixture place by its pinned identity after a later spelling correction, verifies its original attribution and opens the separately authored fictional bulletin. An expired or unauthorized package yields an explicit access outcome. This verifies the data handoff for future civilian exercises within the environment-preparation scope.

## Release decisions

- **Start now with fixtures and external references.** The first engineering deliverable is the owned two-edition fixture with independently specified expected records, search results and exclusions.
- **Release native search** when the source steward records the necessary rights, the three pilot regions reconcile, and the search and refresh checks pass for the named-point release.
- **Release thematic display and portable use separately.** Record the actual region, edition, permitted operation, evidence and reviewer for each decision. Call coverage national only when all 13 regions have evidence for the specific capability being claimed.
