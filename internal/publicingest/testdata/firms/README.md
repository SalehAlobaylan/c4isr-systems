# FIRMS fixture provenance

`noaa20-published-example.csv` transcribes the first five NOAA-20 rows in NASA's [FIRMS API tutorial](https://firms.modaps.eosdis.nasa.gov/content/academy/data_api/firms_api_use.html), South Asia example for June 4, 2025. The CSV serialization/header are reconstructed for the fixture; this is **not** an exact retained live HTTP response. Source values and product labels are preserved. NASA [data-use guidance](https://www.earthdata.nasa.gov/engage/open-data-services-software/data-use-policy) and [FIRMS reuse guidance](https://forum.earthdata.nasa.gov/viewtopic.php?t=5184) permit appropriate attributed use.

Tests derive explicitly synthetic mutations for high confidence, corrections, malformed data, overlapping acquisition windows, and same-AOI integration. They are not additional observations or independent corroboration. No MAP_KEY is present in fixtures.
