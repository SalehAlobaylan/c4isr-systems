# Environment foundation implementation evidence

11 November 2026. The local database schema, preparation worker, Environment Library and portable package formats are implemented. The installed release and staging qualification gates remain open.

| Area | Surface samples | Buildings | Roads | Water features | Prepared bytes | Manifest SHA-256 |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Jeddah | 67,081 | 2,090 | 3,436 | 12 | 6,652,344 | `eda2e7169d686fc78b664b5a2bf1539c1c6afd62d4c0718fa708746a04f009e4` |
| Al Ain | 5,184 | 2,474 | 648 | 5 | 1,724,793 | `2d94c54065132750fb9cabed6f2abfd5dbab6834ba92b22b049df3701ec06b8a` |
| Avignon | 5,256 | 2,700 | 521 | 7 | 1,860,470 | `c51488f96ee2c119c1892204cedd0ef72f399983a425bb96a8c365c3c63dfb6d` |

Each package contains ten geographic members and its manifest. The candidate directories under `private/environment-foundation/implementation` contain the prepared archive, its exact recipe, retained inputs, a private rebuild archive, and verification evidence. Each fresh rebuild reproduced the exact manifest. The offline reader verified masks and clipped geometry, and the Go publisher independently checked the resulting archive contract and coordinate controls. Each surface raster preserved all selected samples in these particular windows. Physical accuracy remains unknown. Building omissions in the Jeddah and Al Ain extracts are recorded in the manifests.

The input inventory verified 11 source receipts and 30 retained imagery band windows. The band inventory binds the retained hashes to the source scene dates, parent locations, and receipt hashes. Local PostGIS was backed up before migration 54. Restoring the backup into a disposable database recovered migration 53 and all 24 reference dataset records. The streaming test round-tripped a file above 16 MiB and read an exact byte range from the local private object store.

The assembled API passed the geographic workflow test with real PostGIS and isolated object storage. It exercised reviewed intake, an explicit worker grant, a bounded upload, duplicate submission, worker staging, separate review and release actions, denied raw redistribution, byte ranges, and denial after a source rights revision. The operator browser checks exercised preparation and review at 390×844 and 740×360. The staging Compose overlay passed configuration validation.

The regional navigation manifest passed atomic activation in a disposable PostGIS database. It included all 17 countries with the expected subdivision and place counts, plus global country discovery. Its prepared source manifest is in the private evidence directory.

## Remaining qualification

The private candidates need reviewed source admissions in the selected shared scope. The earlier immutable reference records do not convey geographic output permission. The exact Copernicus DSM, NGA EGM2008 grid, Sentinel-2 windows, and OSM extracts are identified in each candidate recipe. Review retention, geographic output permission, attribution, and any redistribution permission for those specific inputs. Avignon uses a bounded buffered OSM API extract. Jeddah and Al Ain use the retained Geofabrik regional extract. [Copernicus DEM source](https://registry.opendata.aws/copernicus-dem/) [OpenStreetMap terms](https://www.openstreetmap.org/copyright) [OSM map API](https://wiki.openstreetmap.org/wiki/API_v0.6#Retrieving_map_data:_GET_/api/0.6/map)

After intake, build and verify each installed release using its new exact manifest digest. The final gate also requires the active regional navigation release, installed staging load and worker restart evidence, paired recovery of the installed geographic objects, an independent operator walkthrough, and the full 24-hour staging observation. The local backup rehearsal covered database migration recovery. [Operating instructions](ENVIRONMENT_LIBRARY_OPERATIONS.md)
