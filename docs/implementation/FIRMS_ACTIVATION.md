# FIRMS local activation

2026-10-23. The user-supplied map key is installed in ignored, access-controlled `private/local/firms.key`. The provider configuration references that file. OpenSky configuration and all existing content were preserved. No key is included in browser configuration, source, screenshots or acceptance evidence. The supported paired backup includes the FIRMS key file.

NASA accepted the key and returned **HTTP 200** through the existing native NOAA-20 VIIRS adapter. The first installed day-page produced **zero detections** within the bounded Jeddah area. Its run is **PARTIAL / page_budget** because the initial frozen three-day lookback spans multiple UTC calendar days. It is not a completed regional query or a finding that fires are absent. The worker retains its cursor and resumes the remaining pages under the existing hourly pacing and local limit of 24 requests/day. No pacing, cursor or quota was reset to force completion.

The independent live assembled-API check completed its bounded rolling-day query successfully against NASA, using disposable PostGIS and actual responses. That test uses longitude 39–40 and latitude 21–22; it does not populate or broaden the persistent Jeddah source. The installed source retains longitude 39.144–39.216 and latitude 21.464–21.536. [NASA's Area API documentation](https://firms.modaps.eosdis.nasa.gov/api/area/) defines the calendar-day queries.

The interface now explains initial day-page backfill and displays the next scheduled attempt. Complete source coverage remains unknown until all pages of the frozen window finish. Reported thermal anomalies remain provider detections with acquisition time, confidence and provenance; they are not automatically confirmed fires, perimeters or intelligence assessments.

Open **Public Context → Map → Provider coverage and activation → NASA FIRMS · NOAA-20 thermal detections**. The [redacted acceptance evidence](evidence/firms/ACCEPTANCE.json) contains the actual installed capture and scoped source status. The [source-card screenshot](evidence/firms/source-status.png) was taken from the running installation.

Verification passed for native FIRMS decoding and credential isolation, the authorized live assembled check, the FIRMS/USGS evidence-history regression, a partial-backfill coverage regression, the production UI build and the installed browser source card. The key was also checked for absence from Git-visible files and the browser page. AISStream remains separately gated.
