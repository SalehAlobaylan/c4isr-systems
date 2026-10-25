# P2.4 / P3.1 / P3.2: approved context and bounded moving sources

This increment delivers the three tasks after P2.3. It reuses the existing source admission, immutable evidence/outbox, observation/track projections, private artifact storage, assessment review, and frozen simulation contracts. It adds no 3D engine, hardware integration, engagement controls, weather feed, or general news ingestion.

## P2.4: approved context export

The Intelligence workspace now offers AOI, UTC acquisition interval, reference layer, retained subset and published-assessment selection. Selecting a published assessment pins that exact immutable revision, its comparison result and its two prepared inputs, even if a newer draft exists. Additional successful preparations may be selected as background context. Drafts, unreviewed proposals, restricted sources, failed jobs, uncovered AOIs and out-of-interval acquisitions are rejected.

`POST /api/v1/context-packages/preview` creates a source-restricted, creator-bound candidate for ten minutes. It returns the manifest, byte size, hash, rights, accuracy, transform chain, provenance and omissions. It does not create an approved package or offer candidate bytes as an approved download. A changed selection clears the UI preview, including responses arriving after that change.

An authorized human with `context.export` supplies the preview ID, exact preview hash and a 10-2000 character approval statement to the existing `POST /api/v1/context-packages`. Source admission/export rights and retained dependency hashes/availability are checked again. Source/admission and artifact rows are held through the freeze transaction; permissions are refreshed before persistence. A changed admission requires a fresh preview. Service principals cannot approve packages. Reads, lists, previews and frozen packages retain workspace/source isolation.

The established `emad.context.v1` envelope and offline validator remain in use. Reference-only legacy packages keep `reference-snapshot-v1`; enriched packages declare `context-snapshot-v2`. New asset kinds are `imagery`, `analysis`, `assessment` and `derived_geometry`. Every imagery-derived asset retains the current admission and rights snapshot, original producer/rights, scene/evidence IDs, job and artifact IDs/hashes, and the exact published revision where applicable. Acquisition time, WGS84/angular resolution, processing method and limitations remain explicit. The exported availability deadline is bounded by artifact retention and both applicable rights expiry dates.

Only derived finding geometry is intersected with the AOI, using PostGIS. It carries `syntheticDerived: true`, `scenario_context_only`, its source result hash and the published revision. The package also declares derived geometry as synthetic. Reference files and prepared spectral subsets retain their declared extent; there is no implicit full-raster download, resampling or clipping. Published assessments remain interpretations. Public aircraft/vessel positions are never turned into actors or ground truth.

Frozen bundles download through the existing `GET /api/v1/context-packages/{id}/bundle`, with exact asset bytes encoded in JSON. They cannot mutate the original public records. The existing S0 loader consumes enriched bundles without a competing format. Reference-only bundles explicitly declare external dependencies and fail offline reproduction validation. No resolver dereferences their URLs automatically.

Limits: 16 assets, 8 MiB retained asset bytes, 12 MiB serialized candidate/download, 32 active previews/64 MiB and 64 frozen packages/64 MiB per authorized scope view. Vector/elevation inputs retain their existing 512 KiB per-asset limit. The present headless runner has a separate **4 MiB total frozen scenario JSON** budget; preview omissions identify this, and larger exports are not claimed to run in that adapter. An approved copy does not extend an upstream retention/rights deadline. Availability expiry blocks new loading; it does not erase immutable package metadata or implement a general archive purge.

## P3.1: external moving hypotheses

The new `externaltracks` domain writes source-bound public hypotheses through the existing durable publication transaction. Native moving evidence creates ordinary normalized observations with `semantic_kind=moving` and `geometry_role=reported_location`; the underlying position, reported quality and provider claims remain traceable to the exact evidence record.

`source-bound-motion-v1` is deliberately limited:

- Source + entity kind + claimed ICAO24/MMSI identify a candidate segment. There is no cross-provider merge or automatic verified identity.
- A gap over 30 minutes starts a new segment and closes the prior candidate. A report older than all known segments may create a separate historical segment; continuity is not invented.
- Strictly newer reports update state only within a PostGIS geography gate: aircraft 1500 m/s plus 15 km slack, vessels 100 m/s plus 2 km slack. These are coarse rejection gates, not accuracy estimates or platform physics.
- Late/same-time evidence is recorded as `LATE`, linked without rewinding current state. A failed motion gate records `QUARANTINED`; it cannot update the track position or become an accepted track observation.
- Every `APPLIED`, `LATE` and `QUARANTINED` decision retains origin/known times, method, rationale, reported values and evidence/observation links. Decisions are immutable. No automatic reassociation silently rewrites past decisions; a richer correction workflow remains future work.
- Read-time live/stale/lost states use admitted freshness thresholds. Reception older than 30 minutes is closed; publication maintenance also updates stored lifecycle status. Source-disabled/rights-expired conditions and partial/outdated capture health remain separate from position age.

The API exposes bounded `GET /external-tracks`, `/{id}` and `/{id}/associations` under `/api/v1`. Lists accept AOI/source filters and cap at 200 tracks; histories cap at 100 with explicit truncation. Reported identity, classification and affiliation claims remain separate from analyst confidence. Provider quality is not converted to confidence. A registration-country inference is not affiliation, and repeated network snapshots are not independent corroboration.

The UI shows claims, units, provenance, source health, age, decisions and exact-record downloads, with a bookmarked selection and link into the existing MapLibre map. Public map track/history queries and the hypothesis list poll every 15 seconds. No second map renderer or public asset/command controls are introduced. Restrictive source RLS also protects the existing generic track detail/state/history endpoints, including known identifiers. The native correlation prefix is reserved from legacy scripted/manual ingestion. Exercise workflows retain their existing correlation path.

## P3.2: provider choice, protocols and activation

**OpenSky is the selected ADS-B provider.** Its native bounded `/api/states/all` endpoint is implemented; no generic intermediary response is assumed. ADS-B.lol's current public API documentation marks that API deprecated and directs use toward feeder-restricted access, so it was not selected for this increment.

The initial OpenSky adapter uses the documented anonymous current-state endpoint. Requests include only the admitted latitude/longitude box. ICAO address, callsign, inferred registration country and altitude remain provider claims. Position time comes from `time_position`, never `last_contact` or retrieval time. Missing positions are excluded, invalid coordinates/times are rejected, and original state-vector JSON bytes remain downloadable. A snapshot does not establish historical coverage across its lookback interval. Content revisions/deduplication reuse the existing evidence facilities.

Technical anonymous access is **not a license grant**: OpenSky's current terms require a written agreement for operational API use, including internal automated products, and separately restrict commercial use. Obtain the applicable agreement and record approved retention, analysis, use and attribution terms before enabling an admission. No live OpenSky call was made as part of this increment; software acceptance uses explicitly synthetic native-format records. Account/OAuth support is not implemented or claimed.

AISStream uses its fixed `wss://stream.aisstream.io/v0/stream` endpoint. The key stays on the server in `AISSTREAM_API_KEY`, named by trusted registry `authTokenEnv`. The client negotiates permessage-deflate, subscribes within two seconds using **latitude/longitude** bounding corners, requires the provider's compression confirmation, and accepts compressed binary UTF-8 JSON position frames. PositionReport and both standard/extended class-B position messages are normalized. Full provider UTC timestamps are required; unavailable AIS speed/course/heading values remain absent. Knots convert to m/s. MMSI/name/navigation fields remain claims, and position-accuracy flags remain source quality.

Each AIS connection is a bounded 6-10 second capture, with one in-flight stream per fetcher and the existing per-source database lease across processes. A decoded frame is at most 64 KiB; captures retain at most 200 records/1 MiB. Disconnections/record/byte exhaustion are explicit partial results. Accepted records survive a mid-capture disconnect. Empty completed captures retain capture metadata without manufacturing an observation. Credentials echoed by upstream are rejected before retention. There is no durable upstream replay or continuity guarantee; missing messages between connections cannot be reconstructed. Run metadata records capture bounds, compression and discontinuity even when no record arrives.

Provider-independent limits: one page, AOI at most 25 square degrees and at most five minutes of accepted source-time lookback. OpenSky uses at least 60-second spacing and at most 300 requests/day; AIS uses at least 10-second spacing and at most 1000 connections/day. Prefer the more conservative example intervals. OpenSky's documented anonymous credit ceiling is 400/day, with the smallest AOI tier costing one credit; the adapter's smaller cap leaves headroom. Registry/admission budgets, retries, outage backoff and deduplication persist across restarts. Live moving feeds always use a fresh window after a failure/partial result, not a historical cursor.

Each moving source is bounded to 10,000 retained evidence records/64 MiB of evidence JSON and original bytes, and 2000 hypothesis segments. A page crossing the evidence budget rolls back atomically and records `source_storage_budget`. Limits are archival stop limits, not a rolling retention/purge implementation. Source expiry stops new capture/publication; historical evidence remains immutable. Enable only with rights compatible with this retained archive. Providers requiring automatic timed erasure need a separately approved lifecycle implementation before activation. Do not evade quotas/archive limits by duplicating source identities.

AISStream currently documents three connections per account/IP and no durable replay/SLA. Operate one configured worker/source for the initial account and account for other clients sharing that account/IP; this application cannot reserve third-party connections. Review account-specific access and retention terms before activation. No key was present, no live AIS connection was made, and redistribution permission is not inferred from public API visibility.

## Rollout and operation

Apply new migrations **31 and 32** after the already applied version 30:

```sh
task db:migrate
```

31 adds expiring restricted export previews. 32 adds source-restricted external tracks/segments/immutable associations and the supporting indexes. Existing migrations/data were not rewritten. The API checks for the new tables at startup. These migrations were exercised only against disposable acceptance databases during development; your developer database has not been migrated by this change. External evidence rollback is deliberately refused in favor of forward recovery.

Keep the existing private artifact store and separate imagery worker configured for enriched export. Grant `context.export` explicitly to the intended public human approver, plus source access and `assessments.read` for reviewed assessment export. The supervisor role ceiling now permits this capability; existing stored grants still determine authority. Exercise authors additionally need their existing scenario-author/control permissions. No automatic review or simulation control grant is added.

For moving feeds:

1. Register separate external public sources and copy their canonical IDs into the templates.
2. Review rights/access, retention compatibility, attribution, geography and budgets. The `opensky-admission.example.json` and `aisstream-admission.example.json` templates are deliberately **disabled**, with analysis/raw-retention permissions false and replacement instructions for legal basis.
3. Copy only configured locations from `moving-connectors.example.json` to the private `C4ISR_PUBLIC_CONNECTORS_FILE`. An AIS location requires its server key even before a capture starts. Do not put keys in URLs, source metadata, browser configuration or committed files.
4. Provision exact-source service identities using the existing administrator CLI and corresponding principal example. Record a human-approved admission with the matching source, dataset, identity, rights and location. Enable only after the provider requirements are satisfied.
5. Run the separate connector executable with the same database/registry environment as the API:

```sh
go run ./apps/emad-admin -action provision -spec private/opensky-principal.json -out private/opensky.token
go run ./apps/emad-connectors -token-file private/opensky.token -source REGISTERED_SOURCE_ID -once
# Omit -once for bounded captures with persistent pacing, jitter and backoff.
```

Use a separate scoped credential/process for AISStream. The API starts no feed scheduler automatically. Stop with Ctrl+C/SIGTERM. The worker stops when a source reaches its archive budget. `-once` prints status/evidence count without payloads/keys and returns failure for failed/interrupted captures. Inspect source health/runs for partial results and pending publication; `POST /api/v1/sources/{id}/publications/retry` with the exact-source worker credential retries up to 1000 durable projections without another provider request. Refresh expired/revoked worker credentials through the existing provisioning workflow. Never grant these workers exercise control or truth permissions.

## Acceptance evidence

The real PostGIS/S3 imagery workflow now proves M3: recorded scene metadata -> explicitly synthetic retained pixels -> deterministic comparison -> independent human publication -> bounded preview -> approval -> exact bundle download/offline integrity -> the existing S0 headless loader/run. It verifies that a newer draft is not exported, public inputs are not changed, and invalid intervals/AOIs, unreviewed revisions, changed hashes, expired previews, changed rights, unauthorized sources and reference-only offline claims fail.

Two native moving integration cases verify duplicate/late/jump/gap behavior, a full 200-record page without a permanent publication deficit, immutable associations, no assets/commands/classifications, generic-endpoint source isolation, vessel normalization, partial/disconnected capture, outage recovery, empty capture, freshness, disabled source and the atomic 10,000-record stop limit. A local compressed WebSocket peer verifies the actual AIS transport/subscription/binary decode path, budgets, confirmation/error handling and credential-echo rejection. Neither synthetic fixture grants real provider access.

Six real browser workflows pass against the production UI, assembled Go API, disposable PostGIS and private S3-compatible storage. These include comparison/review/export/download, moving-track claims/history/originals/URL/map selection, desktop/mobile access, public reference evidence and the original scenario/audit/truth-revocation paths. The full PostGIS/S3 suite passed **48 top-level cases plus 24 subcases (72 total)**; four opt-in live/browser cases skipped in that run. The separate real-browser gate passed all six workflows, and the mocked browser suite passed 74 checks with 50 intentional project skips. Go tests, vet, Linux race checks, the UI build, OpenAPI lint and generated-code checks passed. Final focused reruns cover the complete imagery export and the full 200-record moving page after the last boundary/projection changes.

```sh
go test ./...
go vet ./...
C4ISR_TEST_INTEGRATION=1 go test -count=1 ./internal/integration/...
pnpm --dir apps/operator-ui build
pnpm --dir apps/operator-ui contracts:generate
pnpm --package=@redocly/cli dlx redocly lint api/openapi/openapi.yaml
task test:browser:integration
```

P3.3 (weather and selected reports) is the next task. The P3 workstream/live-provider exit gate is not declared complete by local protocol acceptance; live activation still depends on the provider agreement/key and approved admission.

## Primary references checked on 2026-10-21

- [OpenSky native REST/state-vector contract and quotas](https://openskynetwork.github.io/opensky-api/rest.html).
- [OpenSky use terms and operational agreement requirement](https://opensky-network.org/about/terms-of-use).
- [ADS-B.lol public API deprecation](https://www.adsb.lol/docs/open-data/api/) and [feeder-only replacement access](https://www.adsb.lol/docs/feeders-only/re-api/).
- [AISStream subscription, compression, position schemas and limits](https://aisstream.io/documentation).
