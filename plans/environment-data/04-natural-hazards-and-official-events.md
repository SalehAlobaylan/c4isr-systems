# D04 — Natural hazards and official event context

[Plan index](../README.md)

**Status** Historical flood-report adapter and frozen revision handling implemented, 13 November 2026. The tested Riyadh query returned no flood reports. Regional source qualification does not authorize a live schedule or export. [Pilot evidence](../../docs/implementation/RIYADH_OBSERVATION_PILOT.md#d04-historical-provider-reports).

**Implementation owner** Public data integration engineering.

**Dependencies** [F01 — Source admission, lineage and retention](../environment-foundation/01-source-admission-and-lineage.md), [V06 — Context overlays, freshness and coverage](../3d-infrastructure/06-context-overlays-and-freshness.md)

## Outcome and activation

A user can inspect attributed natural-event reports for the environment’s area/time, with source geometry and coverage meaning preserved. Existing adapters already provide USGS, GDACS and FIRMS. Activate an additional EONET or official warning source only for a documented coverage gap.

## Features and contracts

Keep provider event ID/revision, observed or effective interval, publication/retrieval times, source references, geometry type, uncertainty and lifecycle state. EONET supplies dated event geometries and citations. Preserve multiple dated geometries when an event changes over time.

Add reversible cross-source associations using explicit references and constrained matching. Retain each original statement. Expose corrections, withdrawals and provider query limitations.

## Implementation sequence

1. Compare current retained coverage with the proposed provider for one AOI and interval.
2. Select categories with additional useful information and document overlap.
3. Implement the adapter with existing bounded acquisition, quota and receipt mechanisms.
4. Normalize geometry and time without inventing a full affected-area polygon from a point.
5. Add association review and provenance inspection to the existing event layers.
6. Include selected approved revisions in V07 packages where retention permits.

## Acceptance

- The pilot demonstrates an identified coverage addition or records that no addition was found.
- Duplicate reports remain independently traceable even when grouped for display.
- Corrections and withdrawals preserve historical versions and alter current presentation appropriately.
- Partial paging, provider outages and complete empty queries remain distinguishable.
- Reported footprints and point uncertainty survive export and 3D placement.
- Reopening a frozen package retains its exact selected event revisions.

## Rollout and recovery

Start with a bounded source profile. Suspend the new adapter independently if its semantics or coverage fail validation. Existing provider records remain accessible under their current policies.

## Deliverable evidence

Deliver the coverage comparison, recorded fixtures for changed/withdrawn events, receipts, geometry checks and a consumer inspection example.

## Implementation and source references

- [internal/publicingest/observation.go](../../internal/publicingest/observation.go)
- [internal/publicingest/gdacs.go](../../internal/publicingest/gdacs.go)
- [internal/publicingest/firms.go](../../internal/publicingest/firms.go)
- [internal/publicingest/usgs.go](../../internal/publicingest/usgs.go)
- [World Monitor docs/data-sources.mdx](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/docs/data-sources.mdx)
- [NASA EONET API](https://eonet.gsfc.nasa.gov/docs/v3)
