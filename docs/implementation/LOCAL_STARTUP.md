# Supported populated startup

Run `task local:setup`, then `task dev` and open **http://127.0.0.1:5173/**. Use `task local:doctor` and `task local:stop`. This installs and starts persistent content and scoped workers. The [current operations guide](RECOVERY_OPERATIONS.md) supersedes the manual provisioning instructions below. [Recovery acceptance](DELIVERY_RECOVERY_ACCEPTANCE.md) records the installed workflows and limits.

## Historical startup investigation

2026-10-23. The normal entry is now **Home** at `http://localhost:5173/`. The module bar links directly to the features available in the selected workspace. The existing map is still at `/map`.

## Why the old launch looked unchanged

The development token authenticates `human:operator-01`, which initially has only legacy exercise access. Database migrations do not grant Public Context or protected scene-authoring access, admit sources, import assets, or start workers. Previously `/` redirected straight to the old map, several new routes had no main-navigation link, and the empty legacy run gave no useful introduction to the new product areas.

Home now explains missing Public Context and Studio access. It checks Studio authorization against the real API rather than inferring it from exercise permissions. These UI changes do not grant access or fabricate observations/scenes.

## Resolve a conflicting API port

If another service uses port 8080, stop your EMAD development commands with Ctrl+C, then use a free port in both terminals:

```powershell
# Terminal A
task dev:api C4ISR_DEV_PORT=8081

# Terminal B
task dev:web C4ISR_DEV_PORT=8081
```

The UI remains on port 5173. `C4ISR_DEV_PORT` coordinates the API listener and the HTTP/WebSocket proxy. An explicit `C4ISR_HTTP_ADDR` overrides the listener; if you set it, also set the matching `C4ISR_API_PROXY_TARGET` on `dev:web`. Direct `pnpm dev` launches can set `VITE_API_PROXY_TARGET` to the API's HTTP origin.

Do not stop an unrelated server to free the port. During the reported local launch, Apache answered on IPv4 port 8080 while EMAD answered on IPv6 port 8080. That can produce different behavior depending on which loopback address resolves first.

React StrictMode also cancelled the first in-flight WebSocket upgrade during development effect setup/cleanup. Startup now defers opening the connection until that cleanup has run. Authorization checks, reconnects and actual proxy error reporting remain enabled.

## Explicit local analyst and author access

These commands are an **optional, trusted administrator action against your local development database**. They create a separate human credential with Public Context access, authoring access and exercise access in `default / local`. They do not alter the existing operator's grants, grant simulator-truth access, or enable external retrieval. Change the names/environment for your installation. Run provisioning once; if the identity already exists, use the administrator's `issue`/`rotate` actions rather than recreating it.

```powershell
$env:C4ISR_DATABASE_URL = 'postgres://c4isr:c4isr@localhost:5433/c4isr?sslmode=disable'
New-Item -ItemType Directory -Force private | Out-Null

# Create the named human principal and write its secret only to an ignored file.
go run ./apps/emad-admin -action provision -spec scripts/data/local-analyst.example.json -out private/local-analyst.token

# Assign authoring in the same environment.
$studioGrant = Get-Content scripts/data/simulation-studio-author-grant.example.json -Raw | ConvertFrom-Json
$studioGrant.id = 'local-analyst'
[IO.File]::WriteAllText((Join-Path (Get-Location) 'private/local-studio-grant.json'), ($studioGrant | ConvertTo-Json -Depth 10))
go run ./apps/emad-admin -action grant -spec private/local-studio-grant.json

# Explicitly allow exercise operations across this local environment's runs.
$exerciseGrant = Get-Content scripts/data/native-exercise-operator-grant.example.json -Raw | ConvertFrom-Json
$exerciseGrant.id = 'local-analyst'
$exerciseGrant.resource.runId = '*'
$exerciseGrant.permissions += 'scenarios.control'
[IO.File]::WriteAllText((Join-Path (Get-Location) 'private/local-exercise-grant.json'), ($exerciseGrant | ConvertTo-Json -Depth 10))
go run ./apps/emad-admin -action grant -spec private/local-exercise-grant.json

# Restart the UI using the new credential. Do not paste or print the token.
$env:C4ISR_API_TOKEN = (Get-Content private/local-analyst.token -Raw).Trim()
task dev:web C4ISR_DEV_PORT=8081
```

Keep the API running in terminal A. Restart only terminal B after changing its token. The issued credential defaults to a 24-hour lifetime; expiry requires a new credential. Example credentials/permissions are for explicit local development setup, not a production deployment policy.

## Where the delivered capabilities appear

- **Public Context → Map**: event/reference layers, filters, historical views and evidence drill-down. Configure reviewed source admissions and trusted provider bindings first; see [public ingestion](P1_1_PUBLIC_INGESTION.md) and [USGS/FIRMS acceptance and activation](S0_4_P1_5_6_ADAPTER_AND_PUBLIC_HISTORY.md).
- **Imagery & assessments**: imagery comparison, analysis jobs and analyst review. Private artifact storage, admitted imagery and the analysis worker are separate setup requirements; see [imagery and analysis/review](P2_1_3_IMAGERY_INTELLIGENCE.md).
- **Weather & reports / Moving tracks**: bounded connector records and provenance. Provider credentials/agreements remain required where documented; see [moving feeds](P2_4_P3_1_2_CONTEXT_AND_MOVING.md) and [weather and hazard reports](P3_3_WEATHER_AND_REPORTS.md).
- **Simulation Studio / 3D assets**: model admission, environment/template registration, scene authoring and a georeferenced 3D viewport. A grant opens the module; it does not create a scene. Follow [asset admission](S1_1_MODEL_ASSET_REGISTRY.md) and [Studio/runtime rollout](S1_3_S2_1_STUDIO_AND_NATIVE_RUNTIME.md).
- **Exercise COP / Recording & replay**: native sensor output, exercise operations, recorded output and released after-action reports. These need an authored run and the independent runtime/recording workers; see [perception and tasking](S2_2_S2_3_S3_1_PERCEPTION_AND_TASKING.md) and [recording/evaluation](S3_2_S3_3_S4_1_RECORDING_EVALUATION_PRESETS.md).

The shipped simulation is an incremental, bounded profile with abstract movement, visual context and synthetic sensor behavior. It is not yet the long-term high-fidelity aircraft/terrain/physics environment. Passing those profiles' tests demonstrated those profiles; it did not demonstrate a fully populated operational installation or the completion of future S4 work.

## Verification of this follow-up

- Production UI build passed. All 80 browser regression checks passed, with 54 intentional project skips (including the four new cases' duplicate token-project runs).
- All 16 real-backend production browser checks passed against disposable PostGIS/S3. The first run exposed a test race reading an old document's response after navigation; the test now separates document lifetimes and reads the new response body immediately.
- The local API/Vite path was exercised without HTTP/WebSocket mocks. Before the fix, development startup opened and cancelled one COP upgrade before establishing the next. After the fix, one COP stream reached `realtime.ready`, remained healthy through the 30-second heartbeat, and reported no browser socket errors. Killing an earlier inspection process produced a separate expected connection-reset log; real proxy errors remain visible.
- Desktop and 390px mobile views were inspected; the new page had no document-level horizontal overflow. Local screenshots and raw logs remain ignored under `tmp/runtime-review/`.
- Port-override Task definitions pass dry-run parsing. Documentation links resolve. All 38 migrations, the migration script and the line-ending policy retain their previously protected hashes.

No local access grants or source configuration were changed by this review. The optional provisioning instructions above still require an explicit administrator action. No commit was created.
