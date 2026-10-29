# Local access failure correction — 29 October 2026

Earlier evaluation-server checks did not establish access through the user's regular startup. At this correction, neither the temporary UI on port 5198 nor its API on port 8097 was listening. A retained browser tab could therefore display `Failed to fetch`.

The running regular UI on `localhost:5173` also used the legacy `dev-operator-token`, which the running database-authenticated API rejected with HTTP 401. The maintained `local-map-analyst` credential authenticated successfully against that same API. Both failures were reproduced before changing the regular entry.

## Corrections

- Development-only Vite middleware supplies the maintained local identity through a non-cached, same-origin `/config.js`. Bare development startup and the legacy default use this entry. Explicit token configurations, deliberately empty test credentials and production builds retain their own authentication behavior. Credential delivery checks loopback host and peer addresses.
- `task dev:web` declares automatic local entry unless an explicit account token is configured. The full supervisor preserves its supplied profile identity.
- `scripts/ops/local_web_entry.mjs` connects the bookmarked port 5198 to the regular UI and its existing API routing. It has no evaluation timeout and launches no additional API. `task local:web:entry` starts it for an already running UI; `task dev` includes it when the port is free. Normal application processes must remain running; this is not an operating-system startup service.
- Local connection failures now explain that the API is unreachable and offer Reconnect. Account switching remains optional; a network failure no longer presents a mandatory credential form in automatic local mode.

## Verification

Installed Chrome and Brave each opened `http://127.0.0.1:5198/map`, received HTTP 200 for the current identity, and opened Investigations without a credential form. A deliberately interrupted identity request reproduced a failed fetch; Reconnect restored the workspace after removing the interruption. Production TypeScript/build checks passed, as did four applicable existing product-entry checks; four duplicate project cases are intentionally skipped.

Private screenshots and redacted observations: `private/qualification/installed-evaluation-2026-10-29/current-access-verification.json`, `current-access-chrome.png` and `current-access-brave.png`. These evaluator-operated checks do not prove an independent user's completed session. Read-capacity and dense-workload issues remain separate from this access correction.
