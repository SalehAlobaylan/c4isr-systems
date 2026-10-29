# T07–T09 implementation and verification

Prepared 26 October 2026, Asia/Riyadh. The user authorized the next three tasks after T04–T06, then requested T10–T12 while their verification was underway. This report records T07–T09; the [following report](MAP_FIRST_T10_T12_IMPLEMENTATION_2026_10_26.md) records the continuation. Both describe the uncommitted working tree based on `a6d1d22f34692faea31db58e4a6318c539c3321a`.

Implementation and isolated verification do not establish shared deployment, complete acceptance scenarios or independent human qualification. The [verification record](evidence/map-first/t07-t09/verification.json) identifies its scope and the final combined checks.

## T07: Durable investigations

[Investigations](../../internal/investigations/service.go) now retain authored revisions, exact evaluated area/period, analytical question, notes, case state and exact evidence references on the server. Source evidence remains immutable. Bookmarks remain view presets. The UI offers explicit server save, resume, conflict-preserving editing and deliberate reload; browser-local storage does not retain operational case drafts.

Membership uses canonical registered human identities. Owners control membership, transfer, archive and reopen. Membership has its own compare-and-swap version, independent of the authored revision head. A transfer verifies the successor's eligible permissions; an alias is not another human. Immutable membership events retain the audit trail.

Every case revision and history read requires all cumulative source dependencies, including evidence removed from the latest draft and reference geography nested in a scope or job. Source revocation cannot expose an old title/history through another readable revision. Workers cannot obtain human case membership. The browser clears an already displayed case when membership/source access becomes unavailable.

The map restores the saved canonical selection outline as well as the query geometry when a drawn case resumes through its persisted area revision. Rich finding/review/brief authoring remains T16–T19. Later corrections now appear as separate current-validity notices, added in T12; they do not rewrite the saved evidence.

[Migration 00047](../../db/migrations/00047_investigations.sql) installs case heads, immutable revisions/membership events and resource/source/human RLS. The [contract](../../api/openapi/investigations.yaml) and browser exercise cover saving in one browser and resuming in another. That protocol fixture is not an independent-user session.

## T08: Source-backed regional geography

[Regional preparation](../../scripts/data/regional_geography.py) accepts a reviewed region profile. It builds worldwide country navigation and selected-country ADM1/place depth from actual Natural Earth source archives, preserves source identities and aliases, and adds explicitly labeled study areas from configuration. The [default Middle East profile](../../scripts/data/region_profiles/middle-east-v1.json) includes 17 configured source countries; its definition is explicit and does not assert jurisdiction or maritime detail.

The 50m country source includes Bahrain, which the previous 110m country archive omitted. Source country identity `PSX` is retained for the configured Palestinian feature. ADM1 means the installed source's administrative interpretation: the France sample contains departments, not a newly invented regional hierarchy. Names and source geometry remain retained; search normalization and display do not rewrite analytical boundaries. Natural Earth's published [terms](https://www.naturalearthdata.com/about/terms-of-use/) place its data in the public domain and qualify its accuracy/completeness; those receipts are preserved with the prepared source.

Preparation sorts stable source IDs, validates selected countries, splits bounded chunks, fingerprints source receipts and reviewed rights, and supports `--prepare-only`. Installation stages all chunks, retries by content identity, and activates a complete per-source release using the expected head. Staged first-install chunks do not become discoverable through the legacy no-head path. Exact historical feature IDs remain readable. The installer persists the configured countries and study-area members in the region profile.

The source-backed isolated test prepared and admitted four real chunks for Saudi Arabia, UAE, Egypt, Turkey, France, Japan and the USA. It verified source-qualified editions, exact selection, retained ADM1/place depth, accent search, same-name features from different sources, retry safety and atomic activation. The source depth and preparation digest are in the verification record. This is broader than a fabricated seven-box fixture; it does not qualify each country's real analyst workflows, terrain, roads, imagery or jurisdictional accuracy.

[Migration 00048](../../db/migrations/00048_geographic_catalog.sql) adds normalized accent/alias search and durable reference-depth requests. Search exposes continuation, the UI retains previous pages, and availability names the exact source/release depth. Analysts can request an import; administrators can verify resolution only against a compatible installed release. A request does not download data or grant ingestion authority. Generalized roads/DEM preparation remains T20.

## T09: Profile-bound acquisition authority

[Migration 00049](../../db/migrations/00049_retrieval_profiles.sql) maps each old admission to the `default` profile without changing its document, cursor, frozen query, quota, retry history or backoff. Existing running connector leases are fenced. A security-invoker compatibility view retains source-only default reads/writes while new authenticated server code selects a profile. Named profiles keep independent state and immutable approved revisions; a new profile receives no execution grant automatically.

The [profile policy resolver](../../internal/publicingest/profiles.go) resolves the input's original admission/profile and intersects its rights with that same profile's current approved rights and source state. Artifacts, raster workers, captures, comparisons, export preview/freeze and downloads use that resolver. Explicit original-admission dependencies propagate through derived artifacts. Unknown/mismatched mappings fail closed. Immutable original admissions remain readable as provenance; a different profile cannot supply broader rights. Policy locks also cover the final processing/publication commit after external I/O.

Execution grants identify the exact profile revision and eligible service identity. Every begin, request reservation, page commit and finish rechecks current authority. An older policy-unaware reader can read legacy/default inputs but cannot process named-profile raster inputs through a universal default-policy decision. The [caller inventory](MAP_FIRST_PROFILE_POLICY_CALLERS_2026_10_26.json) enumerates policy calls and source-state queries, including intentionally default-only health/legacy maintenance paths.

Successful overlapping retrievals retain occurrence links to the same source/provider record/revision. They do not create additional independent evidence. Conflicting bytes for an upstream revision fail explicitly. Source-wide pacing/quota checks prevent profiles multiplying a source allowance; T10 adds durable account-wide reservations across sources/endpoints.

The Sources UI separates approving a profile from granting its execution. It displays reviewed bounds, lookback, purpose and exact revision. Selecting a country changes neither approval nor grants.

## Verification and installation boundary

Real isolated PostGIS/private S3 checks cover immutable history, membership and save conflicts, old-schema populated migration, overlapping profile deduplication, original-profile rights, stale leases, source suspension and revocation during an upload. Browser protocol checks cover shared resume, conflict retention, membership loss, reference request retry, paging and explicit profile grants. Python preparation tests cover stable identities, bounds, resumable staging and receipt-sensitive versions.

The final assembled suite also runs T10–T12 and prior native provider/imagery paths. See the machine-readable verification records for actual commands and results. The reports do not reinterpret all acceptance scenarios as PASS.

The user reported applying migrations preceding this work. This session did not apply 00047–00050, admit a release into the shared installation, restart shared workers or deploy. Use the coordinated upgrade in [operations](MAP_FIRST_OPERATIONS.md), with the latest schema/API/workers/UI together. Keep old processes stopped during the forward cutover; downgrading these migrations is intentionally rejected. Independent users, installed geographic qualification, real account-cost policies and unattended soak/recovery remain separate gates.
