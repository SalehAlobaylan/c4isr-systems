# OpenSky local activation

2026-10-23. The user supplied the Standard API client `devsaleh-api-client` for EMAD. Its secret was copied into ignored, access-controlled `private/local/opensky-client.json`; provider configuration references that private file. No secret or OAuth bearer token is included in this report, browser configuration or checked-in source. The supported paired backup includes this private client file.

The installed source completed an authenticated bounded capture with **HTTP 200**, **3,999 credits remaining** reported by `X-Rate-Limit-Remaining`, and **zero new evidence records**. These are results of that snapshot, not a statement about the current balance or absence of aircraft. The redacted [activation evidence](evidence/opensky/ACCEPTANCE.json) records the run, geographic bounds, source status, verification and source-file hashes.

The supplied Standard account allowance is **4,000 credits per day**. It is not used as a remaining balance. OpenSky documents separate credit buckets for states, tracks and flights; this integration queries only bounded `/states/all`. A box of at most 25 square degrees costs one credit per request. [Official OpenSky API documentation](https://openskynetwork.github.io/opensky-api/rest.html)

The existing pilot coverage and pacing were preserved:

- Longitude **39.144–39.216**, latitude **21.464–21.536**, around coastal Jeddah.
- One bounded snapshot every **15 minutes**, capped locally at **96 requests/day**. The local request budget is separate from the provider's credit allowance.
- At most 200 returned vectors and 1 MiB per capture; no global query or flight-history retrieval.
- Empty snapshots remain empty. Outdated contacts and source coverage retain their existing freshness semantics between snapshots; this is not a continuous live reception feed.

OAuth tokens remain in process memory and refresh before expiry. The adapter records only a validated numeric remaining-credit header, preserves a reported zero, and leaves absent/invalid balances unknown. Credit exhaustion honors OpenSky's `X-Rate-Limit-Retry-After-Seconds` without bypassing local pacing. Failed 429 responses retain the zero balance and retry information through the ordinary run receipt.

Open **Public Context → Map → Provider coverage and activation → External aircraft observations · OpenSky** to see the configured source, retrieval history and last-reported states balance. Public moving tracks remain non-controllable observations. FIRMS and AISStream still require their separate prerequisites.

Verification passed for the actual installed source card, Go unit checks, generated frontend contracts, production frontend build and focused assembled PostGIS integration tests. The integration cases include durable zero-credit failure, provider retry, unknown balance, moving-source isolation and connector revocation. The [earlier recovery acceptance](DELIVERY_RECOVERY_ACCEPTANCE.md) remains evidence for its earlier snapshot; this report records the subsequent OpenSky configuration and code changes.
