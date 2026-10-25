# P2.1–P2.3: retained imagery, environmental analysis and reviewed assessments

**Status:** P2.1–P2.3 complete, 2026-10-21. Implemented against the two v2 design proposals. This increment adds bounded Public Context intelligence and preserves the existing exercise, source, reference and simulation infrastructure. P2.4 approved context export remains separate; M3 is not yet complete. No 3D engine or live hardware path is introduced.

## Delivered workflow

A reviewed Copernicus source publishes immutable Sentinel-2 L2A catalogue revisions. An authorized user inspects acquisition, geometry, product baseline, cloud/quality, rights, asset references and exact STAC originals. Catalogue discovery alone never makes a scene analysis-ready.

A source manager prepares a small multispectral subset from licensed local B04/B08/SCL files, retains the subset through the authenticated artifact API, and queues validation. An analyst chooses two successful preparations, submits an NDVI comparison, inspects aligned before/after/difference panels and thresholded finding geometry, and downloads the exact result. The processing executable runs independently of HTTP handlers. The analyst records an explicit interpretation proposal; another human reviews and publishes or rejects it with a rationale. Every assessment revision pins the exact job, scenes, artifact hashes, method, quality limitations and actor identity.

The new `/intelligence` workspace is linked from the Public Context panel. Exercise COP remains independent. No scene or finding is converted into an observation, track identity, asset or simulation truth automatically.

## P2.1: bounded discovery and object storage

`cdse-sentinel2-stac-v1` uses the native CDSE STAC 1.1 endpoint and `sentinel-2-l2a` collection. The server owns the endpoint; browser requests cannot supply arbitrary upstream URLs. The adapter accepts only the expected Sentinel-2 L2A product, identity/time/geometry fields and selected `s3://eodata/Sentinel-2/MSI/L2A/…` asset references. It does not fetch asset URLs. Reported scene-wide cloud cover above 30% is excluded and counted; retained scenes still require local mask validation. A scene-wide percentage is not a local clear-pixel guarantee.

Admission limits remain authoritative: one bounded EPSG:4326 AOI, at most 31 days per retrieval window, no more than 25 records/page for this adapter, minimum five-second request spacing, response byte limits, daily budgets, timeout, durable backoff and bounded pages per invocation. The optional `from`/`to` body on `POST /sources/{id}/connector-runs` selects an ordered historical UTC interval no longer than the admitted lookback. Without it the existing rolling-window behavior remains. A partial retrieval resumes its frozen window and opaque vetted token; it cannot switch windows midway. Complete evidence records can publish while the source run is partial, but none is implicitly prepared for analysis.

Original STAC feature bytes and hashes are retained through existing public evidence. Scene metadata is an immutable projection in the same publication transaction. Known object IDs and scene IDs do not bypass source/resource authorization. Lists are bounded to 100 with explicit truncation; spatial filtering uses PostGIS.

New large artifacts use a private path-style S3-compatible port. PostgreSQL owns metadata, source admission, rights, scope, SHA-256, byte count, state and retention. The browser sees an artifact ID, never a bucket key, storage credential or public presigned URL. Object keys bind hashed workspace/environment, PUBLIC_EVIDENCE, source, artifact identity and content digest. The port requires the final key component to equal the exact content hash; concurrent application writes to one key can therefore only contain identical bytes. It checks existing bytes before writing and refuses a different/corrupt existing object. `If-None-Match: *` supplies additional protection on providers that implement conditional PUT. The implementation does not depend on every S3-compatible provider honoring that header.

Objects are 1 byte–16 MiB; each source/resource has at most 1,000 nondeleted objects and 512 MiB, including pending reservations. Reservation request keys are idempotent and conflict if metadata changes. Upload transitions PENDING to READY only after exact size/hash validation and renewed rights/expiry checks. Interrupted uploads can retry the same reservation. Reads validate current source access, rights, expiry, size and hash. There is no incomplete-object fallback.

Retention is explicit, immutable, no more than 365 days and no later than admitted rights expiry. The UI chooses up to seven days. Metadata and hashes remain after authorized deletion; content reads fail after expiry. `DELETE /artifacts/{id}` is denied before retention ends, and thereafter performs retryable DELETING → DELETED cleanup. No automatic janitor is added; operators invoke authorized cleanup. Source suspension blocks use, while expired-content cleanup remains possible. Configure private bucket access and preserve blobs together with database backups; existing PostgreSQL-only backups do not include new S3 content. Use storage versioning/backups when required by deployment policy. Existing small reference/context artifacts remain compatible in PostgreSQL.

## P2.2: declared environmental baseline

The first use case is **NDVI difference**, version `emad.ndvi-change.v1`, from two retained `emad.multispectral-subset.v1` JSON inputs. This is a reproducible numerical indicator with an explicit human interpretation step. It is not a vegetation-loss classifier, causal assessment or calibrated confidence model.

Each input pins scene revision/hash, UTC acquisition, platform, MGRS tile, orbit, processing baseline, three source-file hashes, preparation method, explicit surface-reflectance unit, red/NIR samples, SCL values and target grid. Source hashes and provenance are preparer assertions; the worker verifies retained subset bytes and catalogue identity, not a fresh upstream raster download. Synthetic inputs remain visibly synthetic in results and assessments.

The initial angular grid has EPSG:4326 longitude/latitude cell edges, north-to-south rows, at most 256×256 cells and 0.000001°–0.001° spacing. The scene footprint must cover it according to PostGIS. Both inputs must have identical grids, source, platform, tile, orbit and processing baseline, ordered acquisition times at most 90 days apart. Mixed sensor/baseline comparisons are rejected. Angular spacing is not a constant metre GSD, equal-area measurement or native 10 m imagery claim.

Only SCL 4/5/6 (vegetation, nonvegetated land, water) are used. No-data, saturated/defective, dark/topographic shadow, cloud shadow, unclassified, cloud, cirrus and snow/ice classes are excluded. Negative/nonfinite/null reflectances and nearly zero NDVI denominators are masked. Common clear coverage must meet the selected fraction (0.1–1); the decline threshold is 0.05–1 NDVI units. The retained result records before/after NDVI, difference, mask counts, coverage, mean difference and thresholded cells. It does not infer meaning from masked pixels. SCL is imperfect; residual cloud, atmosphere, illumination, registration, resampling and seasonal differences remain stated limitations. Same orbit/baseline and a 90-day bound do not remove those effects.

Thresholded cell polygons are normalized with PostGIS `ST_UnaryUnion`. PostGIS build identity is recorded with the output; geometry exceeding 4,096 changed cells fails explicitly instead of silently truncating. Display panels share scales, grid and selected cell, expose numeric text/keyboard inspection and mask labels, and draw the derived boundary over the difference panel. That boundary represents thresholded cells rather than a surveyed environmental boundary.

The queue is PostgreSQL-backed; no broker or mandatory ML service is introduced. Frozen input manifests are at most 64 KiB, with SHA-256 and idempotent request keys. Each source/resource permits 32 active jobs. Statuses are QUEUED, RUNNING, CANCEL_REQUESTED, CANCELED, FAILED and SUCCEEDED. Workers claim with SKIP LOCKED, use a 150-second lease and 120-second processing deadline, and poll cancellation/live credentials every 500 ms. Failed jobs can be explicitly retried up to three total attempts without changing their inputs. Expired leases record an immutable interrupted attempt before authorized recovery. A queued cancellation is immediate; running cancellation cannot publish a result. A revoked worker cannot finalize its lease; another authorized worker can recover it after expiry. Process termination records failure where possible, otherwise lease recovery applies.

The worker records the SHA-256 of its executable, algorithm, parameters, full pinned scene/artifact metadata, attempt and output hash. It rechecks source rights before/after object I/O and before comparison publication. Results retain no longer than either input. Scalar comparison is deterministic for the same bytes/configuration; the acceptance fixture reproduces identical output bytes/SHA with the same build/PostGIS environment. No bitwise promise is made across different preparation/GDAL/PostGIS builds. Retain the worker binary/environment and original subset bytes to repeat a historical computation. Replay of a retained result and rerunning an algorithm are distinct operations.

### Local raster preparation

The offline helper is optional until real local rasters are available. It runs outside the API process and has no provider credentials or remote raster downloading:

```sh
python -m venv .venv-imagery
# Activate this environment using your shell's normal procedure.
python -m pip install -r scripts/data/imagery-preparation-requirements.txt
python scripts/data/prepare_imagery_subset.py --scene scene.json --red B04_10m.jp2 --nir B08_10m.jp2 --scl SCL_20m.jp2 --metadata MTD_MSIL2A.xml --west 39.16 --north 21.49 --step-lng 0.0002 --step-lat 0.0002 --width 128 --height 128 --output subset.json
```

Download `scene.json` using the catalogue's exact scene manifest button. Obtain permitted product files through your licensed provider account. CDSE S3 raster downloads need account-generated credentials; none were configured during acceptance. They are different from EMAD's own artifact-store credentials. Do not put either credential in frontend config.

The helper accepts bounded local TIFF/JP2 single-band rasters (512 MiB/file; at most 125 million source cells), reads windows through GDAL reprojection rather than loading full scenes, and caps threads/warp/cache memory. It uses average band resampling and nearest SCL, preserves DN=0 as no-data and requires product XML with a matching processing baseline, explicit BOA quantification and explicit B04/B08 offsets. Reflectance is `(DN + BOA_ADD_OFFSET) / BOA_QUANTIFICATION_VALUE`; it never silently assumes a zero offset. Rasterio/GDAL versions, metadata hash, radiometry and source-file hashes are retained in the output. Network projection lookup is disabled. Identical local fixture inputs produce identical subset bytes. Run it with ordinary OS resource/process limits for production workloads. The first helper supports a deliberately narrow explicit-offset product format; unsupported older metadata must be handled by an explicitly versioned preparer rather than guessed.

## P2.3: interpretation and independent review

New imagery findings extend the existing assessments module with source-bound roots and immutable revisions. Legacy assessment rows keep their existing behavior. Review states are DRAFT, PROPOSED, IN_REVIEW, PUBLISHED and REJECTED. Initial proposals are PROPOSED. A revision of a published/rejected/draft/proposed finding starts a new DRAFT; submitting it creates PROPOSED. An independent human moves it through IN_REVIEW to PUBLISHED or REJECTED. The author cannot review their own proposal. Review actions require a rationale; publication additionally requires analyst confidence and its separate rationale, plus still-usable exact evidence. Concurrent stale actions fail with 409.

Source STAC evidence, numeric algorithm outputs and human interpretation remain separate. A proposal explicitly says OPERATOR, ALGORITHM or AI. Optional AI model score is a separate field and cannot substitute for analyst confidence. A reviewer assigns confidence explicitly. Publication never creates an authoritative observation, changes track identity or converts synthetic pixels into public measurements. The shared upstream collection is displayed, so two temporal scenes are not presented as independent corroborating reports.

Each action appends both a revision and a scoped/source-filtered audit entry atomically. Revisions capture author, action actor, times, rationale, evidence, method and quality limitations. The original author remains attached through review; a new authored draft receives its own author. A later draft or rejection preserves the last published revision pointer. Legacy GET/list responses and incident assessment attachments include explicit review state and revision pointers and use the latest reviewed-workflow projection. Incident relation choices also label that state and link to review history. They do not describe a new draft as published. History lists are bounded to the latest 100; `GET /assessments/{id}/revisions/{revisionID}` retrieves a pinned older version directly. Publication provenance remains inspectable if source bytes later expire, but expired content is unavailable and cannot support a new publication until permitted evidence is retained again.

## Rollout and activation

Apply **00028_imagery_artifacts.sql**, **00029_imagery_analysis_jobs.sql** and **00030_reviewed_imagery_assessments.sql** after the user's already-applied version 27, using the established `task db:migrate` procedure. These are additive FORCE-RLS tables and assessment fields/policies; no existing observation, source, reference or scenario evidence is rewritten. Destructive down migration is refused; recover forward. Implementation tests only migrate disposable databases. Startup requires these new tables.

Provision a private S3-compatible bucket yourself; API startup never creates it. Configure the API and worker with the same endpoint/bucket/region and mounted secrets listed in `.env.example`. TLS is required outside development/test. Endpoint/bucket/access/secret must be supplied together; leaving all absent keeps existing functions available while artifact operations report storage unavailable. Storage credentials need the selected private bucket's get/put/delete access and must not be shared with browsers.

Build and provision the independent worker:

```sh
go build -o bin/emad-imagery ./apps/emad-imagery
go run ./apps/emad-admin -action provision -spec private/imagery-worker.json -out private/imagery-worker.token -lifetime 24h
# Use provisioned DB authentication; no bootstrap human token is needed.
# Set C4ISR_DATABASE_AUTH=true and load the database/S3 config in this process.
./bin/emad-imagery -token-file private/imagery-worker.token
# Add -once to process at most one queued job.
```

On Windows, build `bin/emad-imagery.exe` and run `.\bin\emad-imagery.exe -token-file private/imagery-worker.token` in PowerShell. Keep the built binary with the records needed to reproduce a job.

Worker spec (replace the registered source and actual resource binding):

```json
{"id":"imagery-worker-region","kind":"service","name":"Regional imagery processor","resource":{"workspaceId":"public-context","environmentId":"world","partition":"PUBLIC_EVIDENCE"},"permissions":["operational.read","sources.read","imagery.jobs.execute"],"sourceIds":["REGISTERED_SOURCE_ID"]}
```

Use a distinct connector service with `operational.read`, `sources.read`, `connectors.execute`. Merge `scripts/data/cdse-connectors.example.json` into private server-owned connector config and set `C4ISR_PUBLIC_CONNECTORS_FILE`. No credentials are needed for public catalogue discovery and no provider schedule starts automatically. The connector does not receive the worker execution capability.

An authorized human admits the source with dataset `sentinel-2-l2a`, productVersion `stac-1.1-l2a`, revisionPolicy `content_hash`, producer/provider, exact connector/principal, reviewed rights/attribution, selected AOI and policy. Suitable bounded example values: 31-day lookback, 3,600-second fresh period, 7,200-second unavailable period, minimum 5-second spacing, 10-second timeout, one page/invocation, 10 records/page, 1 MiB response and 100 requests/day. Approve rights that permit the intended retention and analysis; an adapter's availability is not a rights decision. For historical discovery, send an explicit interval such as `{"from":"2025-02-01T00:00:00Z","to":"2025-02-28T23:59:59Z"}` with the connector credential. Retry partial retrieval with the same window; inspect source health and publication status.

Human grants: `sources.read` inspects catalogue/jobs/artifacts; `sources.manage` also retains and cleans up subsets; `imagery.jobs.submit` submits/cancels/retries; `assessments.read` inspects findings; `assessments.create` authors/submits; `assessments.review` reviews another person's proposal. Existing analyst/supervisor role permissions are extended compatibly, but trusted provisioning must still grant the public resource explicitly. Reviewers must be separate human identities. The worker execution capability is service-only and is not a human role default.

## Verification evidence

Executed checks pass with Go 1.25, Node 24/pnpm 10, Chromium, Python 3.11/Rasterio 1.4.4/NumPy 2.2.6, actual disposable Garage 2.4.1 and PostgreSQL 17/PostGIS 3.5:

- Go tests and vet; canonical formatting; stable sqlc/TypeScript generation; Redocly validation; production TypeScript/Vite build; tracked whitespace checks.
- Linux race detector for incidents, assessments, imagery analysis, public ingestion, access, HTTP and realtime; artifact/imagery packages compile under the detector and exercise their database/I/O paths in integration acceptance.
- Two offline georeferenced raster preparation cases covering explicit offsets, masks/no-data, repeated bytes and invalid metadata/bounds.
- 74 mocked browser tests pass, with 50 intentional project skips. Mocked socket shutdown messages remain non-failing.
- Five production-browser flows pass through real Go/PostGIS/S3: binary subset upload/validation/comparison/download/independent publication; empty/invalid/unavailable imagery views; existing public history/originals; scenario-to-command/audit across workspaces; and truth/revocation isolation. Automatic response checks confirm zero API 5xx reaches the browser. Backend cancellation/shutdown diagnostics remain non-failing. Desktop comparison geometry and narrow-screen review/history screenshots were visually inspected.
- Final complete PostGIS regression passes **46 top-level tests and 24 subtests**, with zero failures, in **270.522 seconds**. Four intentional opt-in skips cover live CDSE, FIRMS, USGS and the separately executed browser gate. Native CDSE live acceptance and all five browser flows passed separately. Focused imagery and legacy scenario-to-command/incident/audit acceptance also pass after the compatibility projection change.

 Raw local logs/screenshots are ignored under `tmp/p2/`; reproducible source fixtures live under `testdata/imagery/`.

`TestP2CatalogueS3WorkerReproducibilityAndIndependentReview` uses an actual disposable S3-compatible Garage 2.4.1 server and PostgreSQL 17/PostGIS 3.5. It verifies recorded native STAC metadata, partial-window recovery, pending uploads, hash/size checks, exact object bytes, repeat output hashes, PostGIS finding geometry, queued/running cancellation, dead-process lease recovery, storage-failure retry, withdrawn analysis rights, reverse acquisition rejection, independent AI review with separate score/confidence, draft/rejection history with preserved publication, atomic audit, incident review projections, retention/deletion, source/workspace/partition isolation, revoked-worker recovery and queue capacity shared with retries. It confirms zero observations/tracks/assets were created. The fixture's spectral pixels are intentionally synthetic. It makes no claim of observed vegetation change.

The opt-in native live catalogue test passed on 2026-10-21: one bounded public request retained four scene revisions in a disposable database. No raw raster was downloaded, no live raster analysis was executed, no provider schedule or developer service was enabled. The two recorded eligible scene metadata features have retained retrieval URL/hash in `testdata/imagery/retrieval.json`. Offline preparation tests use generated local georeferenced raster fixtures, not observed spectral measurements.

Commands:

```sh
go test ./...
go vet ./...
C4ISR_TEST_INTEGRATION=1 go test -count=1 ./internal/integration/...
C4ISR_TEST_INTEGRATION=1 C4ISR_TEST_CDSE_LIVE=1 go test -count=1 -run TestCDSEAuthorizedLiveCatalogueSmoke -v ./internal/integration
python -m unittest discover -s scripts/data -p test_prepare_imagery_subset.py -v
pnpm --dir apps/operator-ui build
pnpm --dir apps/operator-ui contracts:generate
pnpm --package=@redocly/cli dlx redocly lint api/openapi/openapi.yaml
task test:browser:integration
```

The live test is deliberately opt-in so ordinary CI does not depend on provider availability. Real browser acceptance uses recorded public metadata, synthetic retained pixels, the production Go API, independent test worker, real S3/PostGIS and separate reviewer credentials. Ordinary mocked browser suites continue to cover existing views/workspace behavior.

## Primary references checked during implementation

- [CDSE native STAC](https://documentation.dataspace.copernicus.eu/APIs/STAC.html) and [account-bound S3 raster access](https://documentation.dataspace.copernicus.eu/APIs/S3.html).
- [Copernicus terms](https://dataspace.copernicus.eu/terms-and-conditions) and [Sentinel legal notice](https://sentinels.copernicus.eu/documents/247904/690755/Sentinel_Data_Legal_Notice).
- [Sentinel-2 processing and BOA offset](https://sentiwiki.copernicus.eu/web/s2-processing), [L2A scene classification](https://docs.sentinel-hub.com/api/latest/data/sentinel-2-l2a/) and [Rasterio reprojection](https://rasterio.readthedocs.io/en/stable/topics/reproject.html).
- [S3 conditional PutObject](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html) and [Garage S3 compatibility](https://garagehq.deuxfleurs.fr/documentation/reference-manual/s3-compatibility/).

Next: **P2.4**, approved public-to-simulation context export through the existing frozen manifest/trust boundary. This increment does not approve or export imagery into exercise input automatically.

Subsequent delivery: **P2.4 is now complete** through [approved frozen context export](P2_4_P3_1_2_CONTEXT_AND_MOVING.md), with an M3 headless acceptance run using synthetic spectral fixtures. P3.1-P3.2 are also implemented; P3.3 is next. The original P2.1-P2.3 acceptance scope above remains unchanged.
