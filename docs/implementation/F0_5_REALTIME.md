# F0.5 — Realtime authorization and metadata protection

**Date:** 2026-10-20 (Asia/Riyadh).  
**Status:** Complete for F0.5.  
**Plan:** [EMAD Implementation Plan v2](../../EMAD_IMPLEMENTATION_PLAN_v2.md).  
**Scope:** Complete F0.5 after the [principal/ownership/query work](F0_2_4_IMPLEMENTATION.md). The full F0 gate remains open until F0.6 introduces explicit workspaces, scoped query caches, and the representative real-backend browser workflow.

## Ordinary COP delivery

`internal/realtime` publishes a version **2** envelope with `id`, `type`, `version`, `occurredAt`, `resource`, and explicitly mapped `data`. The resource is taken from authenticated application context, not event payloads. Only `PUBLIC_EVIDENCE` and `EXERCISE_OBSERVATION` are ordinary COP resources. Input, truth, unscoped events, unknown topics, and the internal area-rule event are not ordinary subscriptions. A privileged truth grant does not enable this socket for truth. No privileged truth/input socket is implemented; those event families remain reserved for separately authorized delivery.

Every connection requires `operational.read` plus at least one permitted topic. The explicit topic catalogue in `realtime/policy.go` applies the corresponding domain read permission. Geofence track events additionally require track visibility; freshness messages check the entity kind. Observation, source, and telemetry events also apply the live source allowlist. Source-bound subscribers cannot subscribe to mixed-source track projections through the COP stream. A new domain event is private until its transport mapping and subscription policy are added deliberately.

Clients may request a comma-separated `topics` list; an unknown or unauthorized requested topic rejects the upgrade and records a sanitized denial. Omitting `topics` selects the current permitted catalogue. New grants do not silently expand an already-selected topic set. Removed permissions and source restrictions take effect during delivery and periodic refresh.

Workspace/environment/partition/run may be selected with existing headers or WebSocket query parameters `workspaceId`, `environmentId`, `partition`, `runId`. Headers take precedence. Selection must match existing live grants. Explicit selections stay pinned. The documented legacy default-resource behavior remains available when no selector is supplied.

The hub checks live authority before enqueueing and again immediately before writing each queued frame. Queue entries retain their immutable resource, topic policy, source, and serialized bytes. Resource changes cannot relabel old frames. Credential expiry/revocation, principal disablement, and grant changes are reloaded through the identity store. Idle connections refresh every second. Invalid sessions receive close code **1008**; changes to permitted topics/source restrictions or the selected default resource require resynchronization. A frame already handed to the network cannot be recalled; checks do not claim atomicity with concurrent credential updates after the last check.

Each connection has at most **256** queued frames, each no more than **64 KiB**. Writes and authorization lookups have a **5-second** bound; keepalive runs every **30 seconds**. Overflow closes the affected connection with **1013** so it can obtain a fresh snapshot. Closure signalling does not wait for the peer inside synchronous domain publication. The close handshake is completed by the connection handler before its read context is cancelled.

## Reconnect and browser behavior

`realtime.ready` is the first frame. `realtime.resync` signals a resource or authorization change. Both carry the authorized resource, current permitted topics, and `resyncRequired: true`. They are control messages, not evidence. There is no durable cursor or claim that disconnected clients receive every event. Consumers must obtain authorized API snapshots after connection, reconnect, a resync signal, or a suspected gap. Events are notifications that invalidate those snapshots; they are never the authoritative state store.

The React provider accepts version 2 ordinary-resource frames only after a ready/resync binding, discards frames for another resource, and refreshes queries from the API. It resets queries on resync/resource changes and policy closure. Network/overflow closures use the existing bounded exponential reconnect backoff with jitter. A **1008** policy closure stops automatic retries and leaves the connection disconnected. Full identity/workspace-aware query-key and selection management remains F0.6.

The browser offers `emad.realtime.v2` and `emad.auth.<URI-encoded-token>` WebSocket subprotocols. The server consumes the authentication offer and selects only `emad.realtime.v2`; it never echoes the credential. The URL no longer contains the browser credential. Legacy `access_token` query authentication remains compatible, but the outer request wrapper consumes it and removes it from URL/RequestURI before HTTP instrumentation and logging. Credential protocol offers are similarly removed before instrumentation. Existing HTTP bearer authentication remains supported.

## Exercise disclosure policy

Existing domain HTTP DTOs and the explicit realtime mapper remain the transport boundary. `ResponseDisclosure` and `internal/platform/presentation` apply the same exercise policy to detail/list/ingress, history, audit, relationships, and replay responses. This policy does not alter stored observations or public evidence:

- Exercise observation payloads are not ordinary COP fields. Numeric quality/uncertainty values and declared Boolean attributes remain visible. Extensible registration metadata and quality maps reject unknown/nested values and unverified reference strings, including strings disguised under an otherwise familiar key such as `sourceId`.
- Correlation `trackHint` and track `externalRef` are empty in exercise responses. Track primary IDs remain the visible hypothesis identity. Legacy assessment subjects/evidence using logical track references are resolved to the canonical authorized track ID before encoding.
- Source and area display names use ordinary exercise labels. Alert prose preserves supported measured classification/hedging and spatial evidence without copying private resource names. Controlled exercise asset identifiers and names remain visible under their existing permissions; those assets are deliberately operated by the exercise user. Approved source IDs are operational provenance, not hidden actor mappings. Publishers must not use those declared visible fields as covert truth channels.
- Internally generated audit/rule reference maps expose only declared operational references and measured values. Observation references use public aliases. Arbitrary input metadata does not become trusted merely because its key resembles a domain ID. Raw truth, actor mappings, and unknown metadata remain outside ordinary responses.
- Raw action/failure diagnostic text is replaced in ordinary exercise transport. HTTP logs use route patterns rather than supplied object paths; HTTP internal-error logs and error spans retain categories rather than exception text. Selected runner/track logging also removes action names, raw observation identifiers, and exception text. Stored diagnostic evidence remains retained for its authorized purpose.
- Scenario definitions and scheduled-event inspection require `sim.scenarios.author` at the application service boundary. Run status remains available to scoped `scenarios.read` users, while definition names, seeds, namespaces, and scheduled action names are author-only fields. Scenario realtime events carry lifecycle status/run identity, never the seed or definition. This does not create a truth reader or grant evaluation access.

Schema-declared operator-authored assessments and controlled-operation inputs remain subject to their existing domain permissions and validation. The presentation policy is not a classifier for malicious text covertly encoded into otherwise permitted measurements or operator-authored prose. Future synthetic sensor adapters must publish observation schemas that exclude sealed actor mappings by construction.

## Durable observation references and rollout

Migration **21**, `00021_observation_public_references.sql`, adds `observations.public_id`. It issues a random UUID-based `view_obs_` reference for every existing and new observation and indexes it within scope. This reference is stable and immutable. It is not a public hash of a predictable scenario name, so an operator cannot test name guesses against a deterministic alias. Original primary IDs, evidence links, payloads, row counts, and ownership are preserved; public-evidence HTTP IDs remain unchanged.

Exercise rendering resolves references in batches through `db.Scoped`. Detail lookup, idempotent retries using a returned reference, assessment subjects/evidence, incident observation links, and observation-specific audit/assessment filters resolve back to the stored identity under the selected scope. A copied alias cannot resolve in another run/workspace/source restriction. A rejected input with no stored observation has no published evidence identifier. Original storage IDs remain accepted for compatible authorized callers; ordinary responses return the public reference. The `view_obs_` format is reserved for server publication references.

Apply the existing migrations through **21 before starting the updated server**. Startup now verifies the publication-reference column. Existing binaries using their generated explicit column lists can coexist with the additive column during a coordinated rollout, but the updated API and browser should be released together for the v2 stream and disclosure contract. Removing already-published references would break evidence consumers, so migration 21 refuses automatic Down; use an explicit recovery plan and verified backup/restore procedure.

The existing development database was not migrated or changed by this task. Tests use disposable PostGIS containers. No real public provider or native 3D engine is enabled.

## Verification

All required checks passed; local raw logs are in ignored `tmp/f0-5/`.

- `go test -count=1 ./...` and `go vet ./...` passed. The new presentation, queued-delivery, source-policy, reserved-partition, and credential-redaction tests are retained in the repository.
- The complete opt-in PostGIS integration suite passed **33 tests in 195.651 seconds**, including migration 21, the populated legacy ownership fixture, and the original scenario/alert/incident/mission/command/audit/replay workflows. Following the final input-map allowlist refinement, the affected integration selection passed again in **32.099 seconds**. The random-reference storage/immutability assertion also passed in a focused check.
- The race suite passed in the cached Linux Go 1.25.14 container with repository and module cache mounted read-only. CI and Taskfile now include the realtime and presentation packages. Native Windows CGO remains disabled.
- Frontend production build, regenerated OpenAPI TypeScript types, and Redocly validation passed. The baseline Vite large-bundle warning remains; OpenAPI validation has no new warnings. Protobuf was not changed by F0.5.
- Existing Playwright projects passed **65 tests with 49 intentional skips** using two workers and the baseline temporary ports 25173/25174. These tests mock API responses; they do not replace the F0.6 real-backend browser gate. The scoped socket, disclosure, and reconnect cases above use the real assembled API and PostGIS.
- `sqlc generate -f db/sqlc.yaml` completed and the generated observation model/query scans include the new column. Canonical Go formatting, UTF-8 source validation, and diff whitespace checks passed.

The first regression run identified documented wire-visibility assumptions in older tests (raw correlation names and storage IDs), and an over-filtered measured overlap. The implementation preserves measured overlap, classified/hedged alert meaning, stable evidence traversal, and compatible idempotent retries while removing the private values. No failing check is accepted as completion.

New tests inspect serialized envelopes and exercise HTTP bytes containing hidden-identity fixtures, rather than relying on the UI to hide fields. They cover simultaneous public/two-run delivery, explicit unauthorized/unknown/input/truth topics, source restrictions, idle revocation and policy close codes, reconnect/snapshot handshakes, authorization of queued frames, resource switches, overflow, missing context, reserved partitions even with truth authority, random-reference stability/immutability, evidence-link resolution, and preservation of raw public/stored evidence. The existing exercise acceptance and replay/failure/spatial cases remain required.
