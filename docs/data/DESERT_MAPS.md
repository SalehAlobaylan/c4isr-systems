# Desert Maps source and operator workflow

All 13 links currently have external-reference status. An EMAD source steward must register the source and approve the regional scope and delivery terms before native publication. A public viewer link supplies attribution and a place to browse the publisher’s map. [Regional directory](https://sites.google.com/view/desertmaps/home) · [Publisher’s rights notice](https://touristmaps.store/ar/p/KbyxD)

Shared staging applied migrations 60 and 61 on 22 November 2026 after paired backup verification. The API health check and all three worker heartbeats passed. The three pinned environment manifests and prepared digests matched. The matching Desert Maps build still requires rollout.

- [Riyadh](https://www.google.com/maps/d/viewer?mid=1zDvUx5fza1RvGTDpoDrx2YeMRrmvbNKe)
- [Qassim](https://www.google.com/maps/d/viewer?mid=1uife5mhtb6LyfjWfsSSEk6wuq-R1Nkcq)
- [Makkah](https://www.google.com/maps/d/viewer?mid=1RAERgQNPG6IWwKkK3svbmQNOh8moerCW)
- [Hail](https://www.google.com/maps/d/viewer?mid=18fBeGiAoVbLTGNcJoXR2NQUpuaInXepi)
- [Northern Borders](https://www.google.com/maps/d/viewer?mid=1Ca79X7Y82zn1FWqZlJlgtxaCc4VSD1_X)
- [Madinah](https://www.google.com/maps/d/viewer?mid=1nB9BEpJwKrBUpNZl0r81Nw8dbLfQhUiw)
- [Asir](https://www.google.com/maps/d/viewer?mid=1Ph5YSXGpngGB61eOnlD-5u0LdtBsQN-N)
- [Al Bahah](https://www.google.com/maps/d/viewer?mid=1_Mwi6VATi5tWg-UoFsxnVEjdSbiHXNnU)
- [Tabuk](https://www.google.com/maps/d/viewer?mid=1ex2Fz9YoPgxm5W0WiEJ7GU6HYDhSnVdJ)
- [Al Jawf](https://www.google.com/maps/d/viewer?mid=1HqaIi2Hp02lQelti20ago_cgtA7fMUcs)
- [Najran](https://www.google.com/maps/d/viewer?mid=1WsNswtQtjCouYxJC_7RsdIvwoM_5K30C)
- [Eastern Province](https://www.google.com/maps/d/viewer?mid=1SPQZ7GkcngMUjwMhYqrw-ANgddZrY7y6)
- [Jazan](https://www.google.com/maps/d/viewer?mid=12vYaqBx9aijU5r-cIR7NUwNv-xAHafaw)

## Prepare a delivery

Use the geospatial Python environment with Shapely 2.2. Normalize only a file you own or have authority to retain and analyze. The current intake format is a reviewed JSON envelope. It identifies the publisher, edition, retrieval timestamp with offset, CRS, completeness and each map, layer and entry. Flattening a licensed publisher export into this envelope is a steward-reviewed preparation step. Provider entry IDs are unique within a layer. Preserve the upstream spelling and full two-dimensional GeoJSON geometry. A WGS84 delivery uses `OGC:CRS84` and longitude first. Check a publisher control point before declaring the order when both numbers could be valid in either order.

- Envelope `schema` is `emad.desert-maps-delivery.v1`. Include `publisher`, `crs`, `version`, `retrievedAt`, `complete` and `maps`.
- Each map has `id`, `region` and `layers`. Each layer has `id`, the source `title`, `entryCount` and `entries`.
- A reviewed `controlPoint` can pin a map’s `layerId`, `entryId` and expected longitude/latitude `coordinates`. A disagreement stops import. A rights record with `mapIds` limits the edition to those maps.
- Entries can provide `id`, `name`, `description`, `verifiedAt` and GeoJSON `geometry`. Missing attributes remain unknown. A steward can add `reviewedAliases` with a `name` and exact `reviewRef` for each accepted association.
- The grant JSON records `id`, `legalBasis`, HTTPS `termsUrl`, `attribution` and optional `expiresAt`. Preparation requires `retainRaw` and `allowAnalysis`. Publishing points requires `allowAtlasDisplay`. Set `allow3DDisplay` only where the terms permit terrain display. Preparing thematic imports requires `allowThematicDisplay`. The civilian handoff functions require `allowOfflineDistribution` and `allowCivilianExercise`.

```powershell
.venv-recovery/Scripts/python.exe scripts/data/prepare_desert_maps.py delivery.json --rights grant.json --output normalized.json
```

The result contains a reconciled layer inventory, per-entry dispositions and the thematic feature collection. The CLI can compare a candidate with a prior normalized file using `--previous`. A missing record needs review. Confirm withdrawals with the publisher before delivering a replacement edition. Keep source and normalized files in access-controlled storage.

## Publish named points

Review the normalized output, confirm the approved source and covered regions, and get the current geography release head. Use the existing source-admin API profile to stage batches and activate them together. Supply an empty expected head only for the first release. Subsequent releases require `--previous` with the last active normalized file. If the comparison finds missing keys, confirm the withdrawals or edition-local identity changes and supply that exact reviewed comparison with `--reviewed-removals`. A partial delivery cannot activate. Existing immutable releases remain separately addressable while their terms allow access.

```powershell
.venv-recovery/Scripts/python.exe scripts/data/prepare_desert_maps.py delivery.json --rights grant.json --output normalized.json --publish-source SOURCE_ID --expected-head CURRENT_RELEASE_ID --previous active-normalized.json
```

The named-place workflow requires database migration 61 and the matching API. In the Atlas, select an admitted point to focus the map and see its source. Where terrain display is permitted, the result supplies a place reference. Enter it under **Reference place** in an open terrain preview to inspect the same retained entry. Its Arabic label sits on the rendered terrain. Source height remains unknown. **Show nearby places from this edition** reads the terrain area and the chosen place release together. It shows up to 24 related labels and keeps an accessible list when labels overlap.

## Publish optional thematic vectors

Generate the separate reference import requests with the approved thematic grant. Each request includes its exact edition, digest and source attribution. Review and submit the requests to `POST /api/v1/reference-datasets` using the same registered source and a human source steward. Select the intended region and edition in **Map layers → Reference geography** after all of its chunks are admitted. Click a rendered drawing to inspect its category, attribution and exact source entry ID. Theme imports use the reference catalog’s current storage and tile limits. Older retained thematic versions stay separately selectable.

```powershell
.venv-recovery/Scripts/python.exe scripts/data/prepare_desert_maps.py delivery.json --rights grant.json --output normalized.json --thematic-source SOURCE_ID --thematic-requests vectors.json
```

## Civilian reference handoff

The online issuer checks the caller’s export permission, exact admitted place and release, and the source’s civilian-use and offline distribution grant recorded at admission. It binds the release, edition and member digests and the attribution. The download is a portable place reference that can accompany an environment kit or a separately authored fictional bulletin.

```powershell
.venv-recovery/Scripts/python.exe scripts/data/desert_maps_handoff.py issue --release RELEASE_ID --feature FEATURE_ID --expires-at EXPIRY_UTC --output place.json
.venv-recovery/Scripts/python.exe scripts/data/desert_maps_handoff.py verify place.json --sha256 TRUSTED_DIGEST
```

Send the displayed digest through a channel separate from the package. Use a configured credential profile with `sources.read` and `context.export`. Existing editions require readmission if the publisher later grants handoff rights. Earlier exact editions remain issuable only while their recorded terms permit the operation. [Integration plan](../../plans/DESERT_MAPS_PLACE_REFERENCE.md)
