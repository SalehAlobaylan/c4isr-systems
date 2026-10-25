# Implemented map walkthrough

Open [the screenshot gallery](index.html) or [the live application](http://127.0.0.1:5173/map). The [implementation record](../EMAD_MAP_IMPLEMENTATION.md) documents scope, migration, installation and acceptance.

All screenshots show the persistent installation. Unsaved acceptance drafts are labeled and remain in browser memory. The arrival-banner test uses one response-only fixture and never writes it into public evidence.

- [Country selection and simultaneous evidence](01-saudi-situation.png): The selected Saudi polygon scopes the counts and features. Preview selection does not turn other layers off.
- [Country → Makkah → Jeddah study](02-jeddah-study.png): The bounded prepared environment is a study area, not a municipal boundary.
- [The real Systems grid inside the map](03-systems-dock.png): The shared dock has independent scrolling, bottom/side layouts, collapse and full-workspace expansion.
- [Weather evidence without losing geography](04-weather-workbench.png): The same live weather workflow is hosted in the map, including exact source records and history.
- [Actual retained Sentinel-2 acquisitions](05-real-imagery-swipe.png): March 1 and March 31, 2025. The comparison draws only matching retained windows at their own geographic extent.
- [SCL quality overlay over retained RGB](06-quality-mask.png): Amber identifies excluded pixels; transparent areas retain the eligible RGB. SCL is a classification, not a verified cloud observation.
- [Narrow-screen map controls](07-mobile-map.png): Controls and layer visibility remain available on a 390px screen.
- [Expanded Systems on a phone](08-mobile-systems.png): The actual six-domain grid uses the shared expanded workbench.
- [A draft keeps its original inputs](09-pinned-assessment-draft.png): Changing tools or geography does not rewrite an unsaved assessment or its frozen comparison.
- [Measured nearby context](10-nearby-context.png): PostGIS creates a bounded geodesic context buffer. It does not assert administrative or maritime jurisdiction.
- [Protected library beside the 3D scene](11-studio-library-dock.png): Real validated model geometry, search and admission tools; the unfrozen scene draft survives closing and reopening the library.
