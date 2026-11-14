# V06 — Context overlays, freshness and coverage

[Plan index](../README.md)

**Status** Installed in private staging Atlas and the geographic preview. The staged Riyadh read evaluated its exact area, and the shared status fault cases passed in Chromium fixtures. [Implementation evidence](../IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification)

**Implementation owner** Public data and frontend engineering.

**Dependencies** [F01 — Source admission, lineage and retention](../environment-foundation/01-source-admission-and-lineage.md), [V04 — Shared layer catalog and Atlas-to-3D handoff](04-layer-catalog-and-atlas-handoff.md)

## Outcome and baseline

The map and 3D viewer explain which observations exist for the chosen area/time and whether missing records reflect coverage, age or retrieval failure. Current Atlas reads already preserve scoped snapshots, coverage receipts and some display states.

## Features and contracts

Compose a layer status from independent dimensions for retrieval health, observation age, admitted spatial/temporal coverage, query completeness and display reduction. Preserve source-specific freshness policy. A recent request can return old observations, and a complete query can return no records.

Reuse source scheduling, reservations and backfill. Canonical cache identity includes AOI, time interpretation, source admission, filters and authorized scope revision. Persist protected browser data only under an explicit permitted cache policy with bounded lifetime and invalidation.

## Implementation sequence

1. Inventory existing state fields and define one shared status model for Atlas and 3D.
2. Project current receipts and retained records into that model without inferring provider completeness.
3. Add inspector explanations and a deliberate acquisition/refresh action using the existing scheduler.
4. Implement bounded fetch retries and cancellation. Preserve last-known values only with their original time and stale state.
5. Instrument source failure, content age and display-cap metrics separately.
6. Add authorized live context overlays with their own timestamps alongside frozen geography.

## Acceptance

- Empty, partial, out-of-coverage, stale, unavailable and display-limited cases have distinct readable results.
- A provider failure cannot become a successful empty dataset.
- A successful poll with old content shows the old observation time.
- Rapid AOI changes cancel obsolete work and cannot repaint another area.
- Revocation invalidates subsequent reads and reusable protected cache entries.
- Display counts expose sampling/aggregation and preserve access to authorized retained details.
- Backfill and live refresh respect the existing shared account quota and queue fairness.

## Rollout and recovery

Adopt the status projection first in current Atlas, then the new renderer. Preserve existing filters and receipts. If a new provider adapter fails, retain prior eligible observations with explicit status and suspend only that acquisition path.

## Deliverable evidence

Deliver the status contract, source-specific examples, cache isolation results and fault fixtures. World Monitor’s imagery error-to-empty fallback is a behavior to correct in our adaptation.

## Implementation and source references

- [internal/analystscope/reads.go](../../internal/analystscope/reads.go)
- [internal/publicingest/receipts.go](../../internal/publicingest/receipts.go)
- [internal/publicingest/scheduler.go](../../internal/publicingest/scheduler.go)
- [internal/coverage/service.go](../../internal/coverage/service.go)
- [apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx)
- [World Monitor src/services/health-freshness.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/services/health-freshness.ts)
- [World Monitor src/utils/circuit-breaker.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/utils/circuit-breaker.ts)
- [World Monitor server/worldmonitor/imagery/v1/search-imagery.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/server/worldmonitor/imagery/v1/search-imagery.ts)
