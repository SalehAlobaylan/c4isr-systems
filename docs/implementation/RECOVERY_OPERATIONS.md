# Running the recovered EMAD installation

Use the persistent local profile from the repository root in PowerShell:

```powershell
task local:setup
task dev
```

Open **http://127.0.0.1:5173/**. Setup uses the existing PostgreSQL database on port 5433, applies forward migrations through 43, provisions exact development profiles, installs retained real content and configures private Garage storage. It preserves the database, volumes, credentials and existing grants. Repeating setup verifies and reconciles its resources. It stops on conflicting identifiers, changed grants or an occupied application port. Never use `db:reset` for this installation.

Go, Task, pnpm, Python 3.12 and a running Docker Desktop are prerequisites. Setup creates `.venv-recovery` with pinned raster dependencies. The first content installation downloads roughly half a gigabyte of geographic sources and bounded satellite windows; retained sources and receipts allow subsequent runs to resume. An external download failure leaves the checkpoint available for retry and does not replace real inputs with fixtures.

`task dev` starts the API, Vite, the admitted native runtime, raster preparation, imagery analysis, asset validation, recording and configured source workers. Application traffic uses explicit IPv4 addresses: API `127.0.0.1:8081`, browser `127.0.0.1:5173`. Change the two ports in ignored `private/local/config.json` only while stopped. It contains secrets: do not publish its contents. The older separate `dev:api` / `dev:web` commands remain available for low-level development; they do not assemble this installation.

```powershell
task local:doctor
task local:stop
```

Doctor checks scoped credentials, storage, installed features and worker readiness. Stop shuts down the supervisor's own application processes while leaving PostgreSQL, Garage and content intact. Logs are in `private/local/logs`. Worker failures restart with bounded backoff; a lost native execution lease closes that exercise as failed and archives its received output. It does not silently recreate its private world.

## Accounts and the two workflows

The default local session is `local-analyst`, a development analyst and scene author. Home shows actual installed content and links to the two pillars. Public Context opens the Saudi map, layer controls, source status, historical evidence and imagery. Simulation Studio opens the frozen Jeddah environment, authoring controls and Start. Start queues the installed executable automatically; no build hash or manual runtime command is needed.

Other account credentials are in ignored `private/local/credentials/<account>.token`. Open **Account**, paste the credential into **Account credential**, then **Open account**. Switching reloads the application, closing sockets and removing the previous account's in-memory caches. Never paste credentials into a URL or a checked-in file.

- `local-reviewer`: independently registered development reviewer for review/export testing.
- `local-operator`: exercise observations, controlled virtual tasking and ordinary replay; no authoring or simulator truth.
- `local-evaluator`: explicit authoring and truth/evaluation access for this development environment's runs.
- `local-author-alias`: same registered human as the analyst; used to prove review/publication/rejection denial.
- `worker-*`: separate source or partition-bound service identities; never human reviewers.

These profiles are clearly labeled development identities. A workflow executed by two accounts does not establish review by two actual people. The real retained imagery assessment remains proposed for human review. The separately published **DEVELOPMENT WORKFLOW TEST** record must not be treated as an intelligence conclusion.

Credentials expire after 30 days. Setup never overwrites an existing credential. Doctor reports expiry. Use the trusted `emad-admin` issue/rotate workflow to create a new credential in a new private file, then deliberately replace the affected local profile file; restart its process. Production identity provisioning and grants require the deployment's own policy.

## Restricted source activation

**Current local provider status:** The user-supplied [OpenSky OAuth client](OPENSKY_ACTIVATION.md) and [FIRMS map key](FIRMS_ACTIVATION.md) are installed and their bounded live checks passed. FIRMS is completing its initial lookback through paced day pages. AIS remains separately gated. The instructions below remain available for a fresh installation.

Public map source cards distinguish retained geography, configured retrieval, last successful retrieval, stale coverage and missing prerequisites. A successful retrieval with zero Saudi records differs from an unavailable provider. Historical USGS and March 2025 imagery remain labeled by their actual dates.

FIRMS, OpenSky and AISStream are gated until the applicable key or use permission is supplied. Create ignored `private/local/providers.json` with only the bindings you have approved. Paths below are examples of private files, not supplied keys:

```json
{
  "firms": {"keyFile": "private/local/firms.key"},
  "opensky": {
    "permissionConfirmed": true,
    "authenticatedRequired": true,
    "oauthCredentialsFile": "private/local/opensky-client.json"
  },
  "ais": {"permissionConfirmed": true, "keyFile": "private/local/aisstream.key"}
}
```

The OpenSky file accepts the provider's `clientId` / `clientSecret` (or `client_id` / `client_secret`) JSON. The native adapter obtains and refreshes short-lived bearer tokens using OpenSky's fixed TLS token endpoint; tokens stay in memory. Anonymous bounded retrieval is possible only when permission is confirmed and authenticated access is not required. Use and retention permission must be resolved against the provider's terms. See the [official OpenSky REST documentation](https://openskynetwork.github.io/opensky-api/rest.html), [FIRMS API](https://firms.modaps.eosdis.nasa.gov/api/) and [AISStream documentation](https://aisstream.io/documentation).

After editing private bindings, stop, run setup and start again. Missing/invalid optional files remain gated; they do not block unrelated workers. Setup reconciles its own admitted source status. Requests retain bounded Saudi coverage, quotas, pacing and freshness policy. A provider rejecting a syntactically valid credential appears as a provider-access failure, not an empty result. No restricted feed is represented by synthetic public observations.

## Paired backup and isolated restore

```powershell
task local:stop
task local:backup QUIESCED=true
task local:restore:test
task dev
```

Stop every writer, including standalone delivery/connector/capture/imagery/retention workers, before explicitly confirming `QUIESCED=true`. Backup creates a consistent database dump, permitted private objects, source/installation receipts and private configuration. The current v2 format verifies unchanged public table/security-definition inventories and required object hashes before publishing its completion manifest. Known expired/tombstoned object keys are excluded and retention is checked again during restore. See [current map qualification/recovery limits](MAP_FIRST_T22_T24_IMPLEMENTATION_2026_10_27.md). Backups remain under ignored `private/local/backups`; they contain credentials and must remain private. Copy the complete directory to protected storage for off-machine recovery. `private/local/last-backup.json` identifies the latest accepted backup.

Restore testing uses new randomly named PostgreSQL and Garage containers/volumes and ephemeral loopback ports. It validates every byte, opens restored data through a separate API, reproduces NDVI from the restored inputs and reads retained models, geography and recordings. It removes only those temporary containers/volumes, leaving the active installation unchanged. It does not replace a production disaster-recovery procedure or restore over the original database.

## Staging packaging

`deployments/Dockerfile.workers` builds the API and independent workers with a pinned raster toolchain and model validator. Apply `deployments/staging/delivery.yml` after the existing staging composition. It adds persistent private Garage storage and imagery, asset, raster, native-runtime and recording workers. Provide scoped credentials via its named secret files. One connector instance takes one exact-source service credential; add instances for each deliberately admitted source.

The runtime build admitted by the API comes from the same image. The runtime shares the API network namespace for loopback control. Private S3 HTTP is explicitly enabled only inside the staging network; production rejects that exception and requires HTTPS. The non-root/read-only worker image and composed configuration have been exercised locally. No external staging environment or production credentials were deployed by this recovery.
