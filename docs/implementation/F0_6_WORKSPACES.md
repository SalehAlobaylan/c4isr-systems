# F0.6 — COP workspaces and compatibility acceptance

Implemented 2026-10-21 against the v2 architecture and data strategy. This closes the implemented F0/M0 foundation gate, together with the retained F0.1–F0.5 checks. P1 source admission and S0 simulation contracts remain future work; this task does not enable a public connector or select a 3D engine.

## Workspace behavior

The existing React app has explicit **Public Context** and **Exercise COP** navigation. The persistent context bar shows workspace, environment, run (or public context), source mode, clock basis, and live versus historical replay. Public evidence uses UTC evidence timestamps. The existing exercise adapter exposes UTC evidence timestamps alongside the run's virtual clock; this is not the richer S0 clock contract.

`GET /api/v1/auth/me` adds `resources` and `accessRevision`. The resource catalog is computed from live grants and existing scopes, including explicitly granted wildcard runs. It requires `operational.read` in each offered resource and excludes exercise input and truth even for a principal with explicit truth authority. It publishes no credential identifiers, token hashes, source allowlists, or grant rows. The opaque access fingerprint changes on grant and source-policy changes.

The UI first authenticates, then validates a previously selected resource against that identity's authorized catalog. Only the resource tuple is optionally remembered in local storage, under the principal ID. API credentials are still supplied through the established runtime/build configuration; changing configured identity requires a page reload. No login or identity administration product was added. A missing or unauthorized identity renders an access/reconnect screen without mounting domain views.

Every mounted workspace captures an API instance, resource, abort controller, query client, and query-key prefix containing principal ID, resource tuple, access revision, and session generation. Ordinary HTTP calls send explicit `X-EMAD-*` resource headers. Realtime connections use the same tuple in their URL and retain the existing credential subprotocol transport. There is no token in the URL or cache key.

Switching clears the previous client and mutation cache, cancels queries and network work, closes the socket, clears entity selection/drawing/replay/toasts, and navigates to the map without carrying selected object IDs. Late socket callbacks and mutation callbacks check the captured lifetime. Queries without the current capability remain disabled. Revalidation runs every five seconds and on focus; server authorization is checked on every request and stream delivery independently of that interval. Credential revocation or realtime authorization resynchronization removes the workspace. Reconnect authenticates again. Permission or source-policy changes detected by session revalidation create a fresh workspace lifetime.

Scenario controls remain in Exercise COP. Definitions and event queries require author capability; ordinary readers retain scoped run status. Starting/restarting a scenario explicitly selects the returned run. Assets/scenarios routes and mission/command tabs are unavailable in Public Context. Public tracks stay observational records.

## Compatibility and rollout

The new identity fields are additive. Existing REST methods and paths, source/observation/track IDs, evidence traversal, and configured legacy operator credentials retain the documented F0.2–F0.5 compatibility behavior. Clients omitting selection headers still use their server-bound credential default; starting a scenario updates that default. The browser now pins every mounted context explicitly, so another client changing the credential default cannot move its active COP. Historical run selection is also explicit.

Apply migrations through **21**, then deploy the matching API and UI. F0.6 introduces no migration. Existing development data was not changed: integration and browser gates use disposable PostGIS containers. Public workspace navigation becomes available only after a trusted administrator grants that principal a public resource with the required read permissions. Labels, local storage, URL parameters, and frontend visibility never grant authority.

The existing realtime v2 and observation presentation changes remain documented in [F0.5](F0_5_REALTIME.md). The broader ownership migration and legacy quarantine remain documented in [F0.2–F0.4](F0_2_4_IMPLEMENTATION.md). Future native runtime, imagery, artifact storage, and provider adapters must enforce the same boundaries when introduced.

## Acceptance evidence

`internal/integration/workspace_browser_test.go` serves the built production UI and the assembled Go application from one test origin, using a freshly migrated PostGIS database. Playwright does not mock domain HTTP endpoints or WebSockets in this gate. The fixture-only credential-revocation route exists solely inside that test server and is absent from the application.

The real browser verifies scenario start and explicit run selection; geofence-derived alert acknowledgement; linked incident and mission creation; virtual asset command issue and automatic simulated completion; audit inspection; public-context switching; public tracks; inaccessible exercise detail in public scope; return to the authorized exercise; opaque observation references and hidden-actor fixture absence for an ordinary operator; and revocation removing the UI. It also checks resource headers and run-pinned, credential-free WebSocket URLs.

The negative matrix is retained in `workspace_catalog_test.go`, `scope_test.go`, `principal_realtime_test.go`, and `realtime_boundaries_test.go`:

- Workspace, environment, run, and unknown-action denials; public assets/commands denied.
- Input/truth excluded from catalog and ordinary transports, including explicitly truth-authorized callers.
- Cross-scope details, lists, counts, histories, relations, spatial reads, attachments, writes, and commands denied or scoped empty as appropriate.
- Explicit unauthorized subscriptions denied/audited; simultaneous public and two-run delivery; source policy, reconnect, queued-delivery authorization, revocation, overflow, and disclosure fixtures.
- Ownership backfill preserves known graphs, quarantines unknown legacy ownership, and enforces row policies and relation guards.
- The browser lifecycle regression delays old exercise requests, switches during replay with an entity selected, releases those responses, and verifies public data, cleared selection/replay, and retained public scope after reload. Existing mocked design, inspection, and operator-workflow suites remain in place.

Run the gate with `task test:browser:integration`, or build the UI and run:

```powershell
pnpm --dir apps/operator-ui exec playwright install chromium
pnpm --dir apps/operator-ui build
$env:C4ISR_TEST_INTEGRATION = '1'
$env:C4ISR_TEST_BROWSER = '1'
go test -count=1 -run TestWorkspaceBrowserCompatibilityGate -v ./internal/integration
```

Plain Go tests intentionally skip Docker/browser gates. `task test:integration` exercises the complete database suite. CI installs Chromium and runs the real-backend gate explicitly after building the UI and running the existing browser suites.

Verification: **34 PostGIS integration tests and 20 subtests passed in 204.742 seconds**; **65 existing browser tests passed with 49 intentional project skips**, plus the new workspace lifecycle regression. Both real-backend production browser flows passed (7.5 seconds of browser execution; 12.47 seconds including the disposable fixture). Go tests and vet; complete PostGIS integration suite; browser compatibility gate; existing Playwright suites plus the workspace lifecycle regression; access/HTTP/realtime race tests in Linux Go 1.25; production TypeScript/Vite build; OpenAPI generation/lint; canonical Go formatting and whitespace checks. Local raw logs and the public-workspace inspection image are in ignored `tmp/f0-6-*` files. The existing Vite large-bundle advisory remains.
