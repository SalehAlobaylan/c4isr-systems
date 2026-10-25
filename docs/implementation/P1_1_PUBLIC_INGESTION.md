# P1.1 — Public source admission and durable evidence

Current installations also include [P1.2–P1.4](P1_2_4_PUBLIC_CONTEXT.md) and [S0.1–S0.3](S0_1_3_SIMULATION_FOUNDATIONS.md), and require migrations through **25**. The version-22 rollout below records this increment's original baseline.

Implemented 2026-10-21 against the accepted F0/M0 foundation. This increment supplies public ingestion infrastructure in the existing modular monolith. Provider-specific USGS/FIRMS adapters, semantic observation routing, GIS imports, and the full evidence inspection UI retain their later task IDs. No live provider is enabled by this change.

## Source admission and authority

An existing `external` source in `PUBLIC_EVIDENCE` receives a reviewed, immutable admission version. It records the original producer, upstream provider, dataset/product version, connector location and service principal, revision policy, rights, coverage description, AOI, freshness thresholds, and retrieval budgets. The server records the human reviewer and approval time. A changed producer/dataset/product requires a new source identity. A changed rights document requires a new rights ID, including when returning to an older admission's ID.

Review requires a human principal with `operational.read`, `sources.read`, and `sources.manage` in that public resource. Execution requires a separate service principal with `operational.read`, `sources.read`, and `connectors.execute`, restricted to the source's returned canonical ID. The enabled admission validates that service's live grants. Human callers cannot execute a connector even if provisioned the execution permission. Service callers cannot approve an admission. The existing human role matrix gains no execution authority.

The admitted service ID, resource, source, and server location must agree. Current credentials/grants are checked again after external I/O and before evidence/publication writes. Revocation prevents a pending fetch from committing. A disabled/expired admission prevents new retrieval and pending publication; previously retained evidence keeps its historical rights snapshot. Rights expiry does not implement deletion or a general artifact retention scheduler. Analysis/export flags record reviewed eligibility; they do not grant new analysis or export capabilities.

## Retrieval configuration and adapter contract

`C4ISR_PUBLIC_CONNECTORS_FILE` names a server-owned JSON array. An unset value supplies an empty registry, with no polling or network activity. Invalid configuration fails startup. A deployment operator replaces the placeholder source ID with the canonical ID returned by source registration and reviews the real provider location and terms before enabling its admission. Example shape:

```json
[
  {
    "id": "reviewed-provider-page",
    "sourceId": "RETURNED_CANONICAL_SOURCE_ID",
    "resource": {
      "workspaceId": "public-context",
      "environmentId": "world",
      "partition": "PUBLIC_EVIDENCE"
    },
    "url": "https://provider.example.invalid/emad-page",
    "adapter": "emad-json-page-v1",
    "authTokenEnv": "EMAD_REVIEWED_PROVIDER_TOKEN",
    "referenceOrigins": ["https://provider.example.invalid"],
    "maxRequestsPerDay": 100
  }
]
```

The example is not a usable public feed or a provider approval. Omit `authTokenEnv` for an admitted unauthenticated endpoint. A named credential must exist in the server environment; only the environment variable name appears in configuration. Provider credentials and endpoint selection never come from admission input. Reference links are not fetched automatically.

The initial adapter sends a bounded HTTPS GET to the fixed endpoint with `bbox=west,south,east,north`, `from`, `to`, `limit`, and opaque `checkpoint` parameters. Optional provider authentication uses a server-side bearer header. It expects one JSON page containing `records`, `checkpoint`, `hasMore`, `coverageFrom`, and `coverageTo`. Each record carries `providerRecordId`, `providerRevision` when using provider versions, `referenceUrl`, `originalRecord`, and optional `observedAt`/`providerUpdatedAt`. The page contract lives in `internal/publicingest/domain.go`. The adapter identity is versioned; it is not an assumption that USGS or FIRMS natively serves this envelope. Their native format adapters belong to P1.4/P1.5.

For pagination, `hasMore=true` requires an advancing nonempty cursor. The query time bounds remain frozen, at PostgreSQL microsecond precision, until that query completes, including across failed or interrupted invocations. A final checkpoint is a reusable provider watermark for the next query. A native provider adapter must translate its cursor/watermark semantics accordingly. Coverage must be a nonempty interval within the frozen query and declared lookback; it describes provider query coverage, not verified completeness of the physical world.

Only credential-free HTTPS endpoints on port 443 are allowed. Production fetching rejects redirects, proxy discovery, private/loopback/link-local/metadata and reserved destinations, including unsafe resolved addresses. DNS addresses are checked and the checked address is dialed directly while TLS authenticates the original hostname. Responses have header, byte, timeout, encoding, and strict JSON limits. Reference URLs must use the endpoint origin or an explicitly allowed origin and cannot carry userinfo, query strings, or fragments. No raster download, archive extraction, Python worker, broker, or background daemon is introduced.

## Execution and recovery

Execution is an explicit, bounded worker invocation through the API. A scheduler may repeat it using the admitted service credential and source; the server does not start an autonomous polling loop. Admission policy limits each invocation to 1–10 pages, 1–1,000 records per page, at most 4 MiB per response, 1–10 seconds per fetch, and 30 seconds of total work. Original records are limited to 256 KiB. AOIs are finite WGS84 rectangles of at most 400 square degrees; split dateline crossings. Lookback is at most 31 days. The server manifest also caps requests/day per source binding.

Before any outbound request, a transaction reserves its daily quota and minimum request interval. Failures still consume quota. UTC daily budgets, outage backoff, provider `Retry-After`, checkpoints, and a 45-second execution lease persist across API/process restarts. Re-admission preserves quota and next-attempt constraints. Expired leases and admissions changed during execution produce an interrupted run history; competing invocations cannot share an active lease.

Each page commits immutable evidence, a publication receipt, its checkpoint, and run counts in one transaction. Duplicate source/record/revision keys compare normalized content hashes. An unchanged retry increments duplicate counts; a reused revision with changed content fails the page and preserves the prior checkpoint. New provider revisions retain older evidence.

The catalog is a separate projection. Publication inserts catalog visibility and acknowledges its receipt atomically, using the stable evidence ID. A projection failure leaves durable evidence and its checkpoint intact. The pending count is visible even if the catalog is still empty. `publications/retry` replays pending receipts without provider I/O; repeating it cannot duplicate catalog entries. Retrieval failure backoff is persisted; publication retry is explicitly invoked and does not introduce a second polling service.

Run states are `RUNNING`, `SUCCEEDED`, `PARTIAL`, `FAILED`, and `INTERRUPTED`. A successfully created execution returns HTTP 201 even when its run result is failed/partial; callers must inspect `status` and `failureCode`. HTTP 409 rejects an active lease, deferred invocation, or changed/disabled admission. Quota exhaustion records a partial run without fetching and defers further attempts to the next UTC day. Sanitized fixed failure codes avoid persisting provider error bodies or credentials.

## Evidence identity and provenance

Provider IDs/revisions are trimmed, case-sensitive, UTF-8 strings without control characters, limited to 512 bytes. Evidence IDs hash the JSON array `[sourceId, providerRecordId, providerRevision]`; source IDs already have canonical ownership. An `originRecordKey` hashes `[originalProducer, dataset, productVersion, providerRecordId, providerRevision]`, allowing a later analyst to recognize repeated upstream evidence across different providers/connectors. This key is an attribution aid, not automatic correlation or independent corroboration. Providers that rewrite IDs need an explicit translation in their adapter.

`hashVersion=emad-json-v1` identifies the current normalization: decode one UTF-8 JSON value with Go `UseNumber`, then serialize it with sorted object keys and preserved number lexemes. `contentHash` is SHA-256 of that result. Whitespace and object key order do not change it; `1`, `1.0`, and `1e0` remain different. This is not RFC 8785 canonicalization. For datasets without provider versions, `providerRevision` is `sha256:<contentHash>`.

`rawHash` is SHA-256 of the exact original record JSON bytes. When rights permit retention, those bytes live in `bytea` alongside the evidence document; JSONB cannot preserve their original formatting. The `/original` endpoint returns those exact bytes for hash verification. A JSON detail response embeds the parsed record and may normalize formatting, so it is not the byte-verification download. Reference-only admissions retain hashes, source/product/rights metadata, the permitted reference, retrieval time and HTTP validators, without retaining original bytes. Evidence links to the immutable admission and connector run that first retained it. Lists omit originals and are capped at 100 records; details enforce the same resource/source boundary.

## API and operator behavior

All paths below are relative to `/api/v1/sources/{canonicalSourceId}` and use the existing authenticated resource headers/default binding:

- `POST /admission` saves a reviewed version; `GET /admission` returns the current one.
- `GET /admissions/{admissionId}` resolves the original source/rights snapshot referenced by evidence.
- `GET /health` returns provider coverage, timestamps, next attempt, failure code, and pending publication count.
- `POST /connector-runs` executes the admitted service; `GET /connector-runs` lists its history.
- `GET /evidence` lists catalog revisions; `GET /evidence/{evidenceId}` returns a durable detail/reference, including a pending revision when its ID is already known.
- `GET /evidence/{evidenceId}/original` downloads retained bytes, or returns 404 for reference-only evidence.
- `POST /publications/retry` publishes pending receipts, up to 1,000 per bounded invocation, without refetching.

Provision identities with the existing local `apps/emad-admin` tool. A service spec uses `kind: "service"`, an exact public resource, the three execution permissions above, and `sourceIds: ["RETURNED_CANONICAL_SOURCE_ID"]`. Provision the reviewer separately as a human linked to an existing operator. `go run ./apps/emad-admin -action provision -spec <spec.json> -out <new-secret-file> -lifetime 24h` uses trusted database control-plane access and writes the token only to the named new file. See [F0 provisioning and rollout](F0_2_4_IMPLEMENTATION.md) for credential rotation and resource grants.

`GET /sources` and source detail now include an additive ingestion summary for public external sources. States distinguish `not_admitted`, `unknown`, `current`, `stale`, `unavailable`, and `disabled`. A completed query can become stale/unavailable by age; old resumed coverage cannot become fresh simply because its execution completed now. A later outage preserves previously successful coverage and evidence. The Systems page refreshes source health every 15 seconds and shows provider coverage and pending publication separately from registration status. Missing records never prove absence. The existing exercise inventory remains compatible.

## Persistence and rollout

Apply migration **00022_public_source_ingestion.sql** before starting the updated server. Startup checks for the new evidence storage. It adds six tables: admissions, connector state, runs, evidence, publication receipts, and catalog entries. All use restricted-role FORCE RLS with public-resource and source policies, composite ownership foreign keys, and scoped transactions. Admission/evidence writes are append-only; ordinary database access cannot update their originals. The migration refuses an automatic destructive Down operation. Existing source rows and exercise data are preserved. The developer database was not migrated or populated during implementation; verification uses disposable PostGIS databases.

Deploy with `C4ISR_PUBLIC_CONNECTORS_FILE` unset first. Select an actual provider adapter, source identity, region, current terms, rights, quota, and service credential before adding a trusted location and enabling an admission. Save an admission with `enabled=false` to suspend future retrieval/publication while preserving retained provenance. This task does not activate USGS/FIRMS or promote any record into an observation, track, assessment, exercise input, or ground truth. P1.2 owns semantic event/area/moving routing.

## Verification

Focused unit tests cover public destination rules, trusted location/source bindings, server credentials, bounded HTTP decoding/redirects/timeouts, normalized hashes and revisions, reference-only retention, and shared-origin identity. Five real API/PostGIS ingestion tests cover retries and revisions, atomic checkpoint recovery, publication failure/replay, concurrent leases, credential revocation during I/O, resource/source isolation, initial outages, persistent daily quota, historical rights, exact original hashes, and reference-only downloads. New browser coverage verifies that provider outages and pending evidence remain visible independently of active source registration.

Verification completed on Windows with native Go 1.25, Node 24/pnpm 10, disposable PostgreSQL 17/PostGIS 3.5 containers, and Chromium. All public provider inputs were controlled fixtures; no real credentials or public feed were activated.

- `go test ./...` and `go vet ./...` passed. Docker/browser tests are intentionally opt-in and were also run explicitly.
- The full PostGIS suite passed **39 top-level tests and 24 subtests in 236.678 seconds**, including all five new public-ingestion tests. Its browser gate was skipped in this invocation and executed separately below.
- Linux Go 1.25 race checks passed for `publicingest`, `access`, and `httpx`; `sources` compiled with no standalone test files. CI/Taskfile race targets now include the new ingestion module.
- The complete mocked browser suite passed **68 tests with 50 intentional project skips**, including outage/pending-publication coverage under both token configurations. The occupied default Vite port required the existing ignored alternate-port configuration; tests ran on ports 25173/25174.
- Both production-UI/real-Go/PostGIS workspace compatibility flows passed: **2 browser tests**, 10.3 seconds of browser execution, 19.751 seconds for the Go invocation including fixtures. These preserve scenario-to-command/audit behavior and truth/revocation isolation.
- Production TypeScript/Vite build, regenerated OpenAPI types, Redocly validation, sqlc generation, canonical Go formatting, and whitespace checks passed. The existing Vite large-chunk advisory remains. Mocked socket shutdowns emitted Vite proxy cancellation messages without test failures.

Local raw logs are ignored `tmp/p1-1-*` files, including `full-integration.log`, `browser.log`, `real-browser.log`, `race.log`, `go-test.log`, `vet.log`, `ui-build.log`, and `openapi.log`. The plan marks only P1.1 complete; M1 still requires P1.2–P1.6 and real provider acceptance.
