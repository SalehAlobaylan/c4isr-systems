# Simulation input normalization

Reviewed 21 November 2026. Use these mappings when preparing an input for an EMAD exercise. Each row states the smallest identifiable source record, its time and spatial meaning, and the values a consumer can use. Each source plan links to its rows. The mapping names are local to these plans.

## Platform contracts

1. **Acquisition.** For an installed connector, bind the source to a specific admission and profile revision. Record `originalProducer`, `upstreamProvider`, `dataset`, `productVersion`, coverage, retrieval bounds, revision policy and source rights. Check the active source, execution grant and adapter capability for the requested operation. Apply retention, analysis, imagery, export and expiry decisions separately. Record any training authorisation with the reviewed terms. [Admission and evidence types](../../../internal/publicingest/domain.go) · [Adapter capabilities](../../../internal/publicingest/capabilities.go)
2. **Identity and time.** Preserve `providerRecordId`, `providerRevision`, `originRecordKey`, `rawHash`, `contentHash` and `normalizerVersion` where the adapter emits them. Carry publication, revision, effective and observation times at source precision. EMAD's `retrievedAt` is the acquisition instant. For replay, record first availability to each participant separately. A retrospective time assumption needs a named scenario revision. Preserve the source calendar and timezone when deriving UTC. Use inclusive start and exclusive end for derived buckets and document exceptions in the original product. Reimporting the same edition must retain membership. Revisions and withdrawals must be reproducible at an earlier cutoff. [Public ingestion](../../../internal/publicingest/service.go)
3. **Geography and units.** Preserve named places as names until a reviewed boundary crosswalk exists. Carry the boundary edition, CRS, coordinate order, vertical reference, grid registration, mask and native unit where applicable. WGS84 GeoJSON uses longitude first. Keep provider values alongside conversions, including scale, offset and conversion rule. An area label retains its published precision. A raster needs a native reader for each intended variable and band. Display offsets belong to rendering. [Native and display reads](../../../docs/implementation/OFFLINE_ENVIRONMENT_KIT_GUIDE.md)
4. **Evidence classes.** Keep reported statements, measured observations, modeled values, authored exercise inputs and independently reviewed outcomes distinguishable at read time. Tag parent editions and derivation for every join. Preserve raw Arabic spelling and a versioned comparison key. Summaries retain their denominator, period, coverage, and counts of unknown or excluded records. Missing samples, valid zero, partial retrieval and upstream failure require distinct results. Public event and context projections each require specific geometry and timestamps. An unlocated municipal row belongs in the planned table reader. [Public event validation](../../../internal/publicingest/observation.go) · [Context validation](../../../internal/publicingest/context.go) · [Moving evidence](../../../internal/publicingest/moving.go)
5. **Delivery.** Bind geographic inputs to release, area revision, manifest SHA-256, member digest, operation and issued terms. Use `emad.environment-requirement.v1` for a native geographic requirement and record its actual suitability result. A proposed table, notice or evaluation set needs its own versioned reader, identity, time/area rules, rights and known-answer cases before use. Join its immutable edition to the exercise package and prove receipt and offline reconstruction for that representation. Keep restricted evaluation labels out of participant exports. [Delivery status](../../../docs/implementation/SIMULATION_PREPARATION_DELIVERY.md) · [Offline kit](../../../docs/implementation/OFFLINE_ENVIRONMENT_KIT_GUIDE.md)

A literal adapter ID refers to a registered public-ingest adapter for its specific product. A `pilot/` path refers to an existing bounded demonstration. A `design/` path names a proposed representation. A `defer/` path requires a defined use or an access decision before implementation. None of these labels grants admission for a region or operation.

## Road access and inspections

[Feature plan and acceptance cases](01-reports-roads-and-inspections.md)

| Input | Integration path | Record identity and grain | Time and geography | Values, units and limits |
| --- | --- | --- | --- | --- |
| <a id="riyadh-940"></a>riyadh-940 | `design/table` | File SHA-256 and row ordinal within the edition | Receipt in Unix milliseconds to UTC. Keep the source district label. | Preserve the six Arabic fields. Normalize whitespace for matching only. Closure units, exact position and narrative remain unknown. |
| <a id="osm-roads"></a>osm-roads | `pilot/vector` | Edition, source way and clipped part | Native WGS84 feature and graph nodes in the release area | Keep direction, shared node IDs and grade separation. Measure route distance in metres after frame conversion. |
| <a id="overture-roads"></a>overture-roads | `design/vector` | Release, segment ID and connector ID | Edition date and bounded transport network with a buffer | Retain connector relationships, access, direction and turn constraints. Validate crosswalks with a reviewed junction. |
| <a id="balady-works"></a>balady-works | `design/notice` | Municipality, permit and amendment | Planned interval and road reference at published precision | Store planned restriction, cancellation and review status. Historical export and observed closure are separate dependencies. |
| <a id="local-inspections"></a>local-inspections | `design/inspection` | Case, inspection revision and independent reviewer reference | Inspected time, available time, segment edition and direction | Record access as open, partial, closed or unknown with evidence. Keep disputed labels and missing verdicts. |
| <a id="local-reports"></a>local-reports | `design/table` | Pseudonymous case, report and evidence revision | Report time, receipt time and stated location precision | Collect permitted Arabic narratives, clarification requests, dispatch/arrival times and reviewer duplicate links for held-out cases. |
| <a id="worldpop"></a>worldpop | `design/grid` | Release, Saudi asset, band and native cell | Population reference year, native grid and declared CRS | People per cell. Preserve fractional counts and mask, conserve totals when allocating split cells. |
| <a id="owned-building"></a>owned-building | `design/inspection` | Building edition, entrance, task and inspection revision | Local frame, inspection and availability times | Track access by entrance, quantities and costs. Keep occupancy assumptions and verified re-entry separate. |

## Aerial inspection and historical aviation

[Feature plan and acceptance cases](02-aerial-inspection-and-aviation.md)

| Input | Integration path | Record identity and grain | Time and geography | Values, units and limits |
| --- | --- | --- | --- | --- |
| <a id="earth-search"></a>earth-search | `earthsearch-sentinel2-c1-v1` | Collection, STAC item, asset and processing edition | Sensing time, release time if available, footprint and native asset grid | Catalog metadata enters Scene. Reflectance needs the asset scale, offset, nodata, band and local quality mask. |
| <a id="cems"></a>cems | `design/vector` | Activation, area of interest, product, version and feature | Event, source imagery and publication times, assessed extent | Keep product legend and class. Areas outside the assessment stay unknown. |
| <a id="floodnet"></a>floodnet | `design/benchmark` | Dataset version, split, image and mask ID | Pixel coordinates in a paired image and label mask | Keep native integer class IDs and ignore labels. Independently tag evaluation geography and near duplicates. |
| <a id="airlab"></a>airlab | `design/benchmark` | Dataset version, recording and sample | Elapsed seconds within each civilian research recording | Preserve volts, amps and gap duration. Compute Wh only over supported intervals. Keep ancillary recordings separate. |
| <a id="asrs"></a>asrs | `design/benchmark` | Report accession, extract edition and authored drill ID | Reported date precision and sequence available to the operator | Keep narrative attribution, coded handoff events and withheld debrief answer. Exposure denominator unknown. |
| <a id="opensky"></a>opensky | `opensky-states-v1` | Provider identity claim and observation time in retained window | Observation time, snapshot time and capture coverage | Moving aircraft evidence. Aggregate civil activity under documented inclusion criteria and report unclassified observations separately. |
| <a id="adsb-history"></a>adsb-history | `design/aggregate` | Archive day, edition and civil reporting bucket | UTC window and study area for a historical extract | Aggregate distinct eligible civil reports. Log input gaps, shared origins and incomplete identification. |
| <a id="sans-eaip"></a>sans-eaip | `design/notice` | Issuer, edition and amendment | Published and effective dates, civil airport scope | Retain baseline edition and supersession. Mark unmatched airport references. |
| <a id="icao-notam"></a>icao-notam | `design/notice` | Issuer, notice series, year and number | Issued, effective and expiry times, published scope | Keep replacement and cancellation references, original text and unresolved schedules. |
| <a id="faa-nms"></a>faa-nms | `design/notice` | Issuer notice key plus FAA delivery revision | Delivery time and source validity interval | Link delivery copies to the issuer record. Preserve late receipt and missing messages. |
| <a id="survey-log"></a>survey-log | `design/benchmark` | Exercise, task, recording and coverage revision | Assignment, collection and receipt times with reviewed footprint | Area in square metres. Track missed and duplicate work and withhold evaluation labels. |

## Regional demand and coastal recovery

[Feature plan and acceptance cases](03-regional-response-and-coast.md)

| Input | Integration path | Record identity and grain | Time and geography | Values, units and limits |
| --- | --- | --- | --- | --- |
| <a id="srca-east"></a>srca-east | `design/aggregate` | Edition, region, reporting year, measure and channel | Eastern Province, 2025 annual reporting period | Counts of calls, field cases or transfers. Preserve published breakdown and residual. Overlapping channels stay separate. |
| <a id="srca-asir"></a>srca-asir | `design/aggregate` | Originating SRCA release, region, sector and measure | Asir, 2024 annual reporting period | Count reports handled under the Arabic measure. Preserve sector coverage and publisher lineage. |
| <a id="moh-beds"></a>moh-beds | `design/aggregate` | Workbook edition, sheet, health region and measure | Reporting year and MOH hospital geography | Beds as published. Staffing and availability are independently dated assumptions. |
| <a id="portwatch"></a>portwatch | `design/aggregate` | Edition, portid, calendar date and metric | Provider day at the catalog port, preserve date precision | portcalls is a count. Explicit zero, missing port-day and failed request differ. Keep shipment estimates in their documented units. |
| <a id="aisstream"></a>aisstream | `aisstream-position-v1` | Provider identity claim and observation instant | Original/receipt time basis and retained capture interval | Moving vessel evidence. Preserve speedMps, nullable fields and receiver gaps before aggregate replay. |
| <a id="nga-nav"></a>nga-nav | `design/notice` | Issuing authority, series, year and warning number | Warning area and issue, validity and cancellation times | Retain civil service status as reported and link updates. |
| <a id="ukmto"></a>ukmto | `design/notice` | Issuer reference and bulletin revision | Report time, update time and stated civil logistics scope | Keep source assertions and corrections. Independent operator records establish effect. |
| <a id="gebco"></a>gebco | `pilot/grid` | Edition, native cell and source-type band | Native grid and vertical reference of the Jubail pilot | Signed elevation in metres with separate land and source-type masks. Keep the native sign. |
| <a id="shoreline"></a>shoreline | `pilot/vector` | Natural Earth edition, source geometry and clipped part | Shoreline vintage and generalized bounds | Record source scale and clipping. A generalized coast cannot resolve berth boundaries. |
| <a id="port-index"></a>port-index | `design/table` | WPI edition and port identifier | Published locality and inventory edition | Crosswalk to PortWatch with reviewed match status. Leave ambiguous names unresolved. |
| <a id="local-response"></a>local-response | `design/table` | Service area, shift, workload measure or service event and revision | Local calendar, UTC interval and verified service area | Staff, beds, arrivals, wait and service duration retain their units. Preserve the year on published regional counts. |
| <a id="drifter-check"></a>drifter-check | `design/benchmark` | Dataset edition, exercise object and observation revision | Dated position, coordinate reference and availability time | Retain location uncertainty and gaps. Withhold future observations from the participant package. |

## Conditional sources

[Feature plan and acceptance cases](04-supporting-sources.md)

| Input | Integration path | Record identity and grain | Time and geography | Values, units and limits |
| --- | --- | --- | --- | --- |
| <a id="usgs"></a>usgs | `usgs-earthquakes-v1` | Event ID and provider revision | Event origin, update time and reported point | ReportedObservation with magnitude type, depth unit, quality and withdrawal. |
| <a id="firms"></a>firms | `firms-viirs-noaa20-v1` | Sensor and detection identity from the installed decoder | UTC acquisition time and reported pixel location | ReportedObservation, native confidence class and footprint dimensions where supplied. Missing acquisition stays unknown. |
| <a id="gdacs"></a>gdacs | `gdacs-eq-bulletins-v1` | Earthquake bulletin and episode revision | Provider time basis and cited location | Context REPORT, preserve alert level, uncertain extent and common lineage. |
| <a id="eonet"></a>eonet | `nasa-eonet-floods-v1` | EONET event and revision with upstream references | Dated geometries and reported temporal precision | Context REPORT. Preserve source references and distinguish page cap from empty result. |
| <a id="noaa20"></a>noaa20 | `pilot/reference` | Civil satellite catalog ID and archived element epoch | UTC epoch and documented retrospective pilot interval | Computed position is modeled context. An image requires a dated scene asset. |
| <a id="atl13"></a>atl13 | `design/reference` | Product version, granule, beam and sample | Along-track sample time and native vertical reference | Water height and quality flags. Do not interpolate across unrelated tracks or water bodies. |
| <a id="s5p"></a>s5p | `design/grid` | Product, processor version, orbit and pixel | Product interval and native footprint | Keep named atmospheric column, native column unit and product-specific quality mask. |
| <a id="radar-outages"></a>radar-outages | `design/notice` | Provider annotation ID and update | Provider geographic/network scope, interval and issue time | Keep outage classification and unknown end time. Local reporting delays are authored separately. |
| <a id="gpsjam"></a>gpsjam | `defer/aggregate` | Archive edition, daily cell and provider method | Published daily grid and coverage | Retain quality class and sample support if available. Archive permission and usable observations remain open. |
| <a id="hapi"></a>hapi | `design/aggregate` | Upstream resource, admin code, indicator and period | Reference period and boundary edition | Retain numerator, denominator and unit when defined, plus availability by area. |
| <a id="unhcr"></a>unhcr | `design/aggregate` | Edition, country roles, population group and measure | Stock date or flow period and published geographic level | People or movements as defined. Keep stock and flow separate. |
| <a id="dtm"></a>dtm | `design/aggregate` | Operation, round, area and indicator | Collection interval and assessment boundary edition | Keep unit, method and coverage break. Omitted locations stay unknown. |
| <a id="infrastructure"></a>infrastructure | `defer/reference` | Original inventory edition and provider feature ID | Published scope and edition date | Use broad service context. Fictional exercise dependencies need their own IDs and provenance. |
| <a id="meteo-runs"></a>meteo-runs | `design/field` | Model, run, variable, level and valid instant | Model cell, initialization, validity and first availability | Keep native units, height/level and forecast horizon. |
| <a id="meteo-history"></a>meteo-history | `design/field` | Archive product, model, variable, cell and time | Historical weather or stitched forecast at declared spatial support | Record retrospective or stitched semantics and interpolation. Preserve source units and missing hours. |
| <a id="era5"></a>era5 | `pilot/field` | Edition, native grid index, variable, pressure and valid time | Declared pressure levels in hPa and UTC reanalysis times | Read native unit and mask. Below-ground, off-grid and unavailable time are distinct reader results. |
| <a id="metar"></a>metar | `noaa-metar-v1` | Station, report type and observation time | Station position and report time | Context WEATHER. Preserve raw report, m/s wind, statute-mile visibility and lower bounds. |
| <a id="taf"></a>taf | `design/notice` | Station, issuance and forecast revision | Terminal validity and change intervals | Keep forecast groups, units and unknown periods. |
| <a id="sigmet"></a>sigmet | `design/notice` | Issuer, sequence and validity interval | Reported region, validity and release | Preserve stated scope, phenomenon and cancellation. |
| <a id="ncm-cap"></a>ncm-cap | `design/notice` | CAP sender, identifier and sent time | Issue, effective, onset and expiry, referenced area | WMO supplies issuer discovery. Retain NCM language blocks, references, updates and cancellations. |
| <a id="cams"></a>cams | `design/field` | Model, run, variable and native cell | Global model grid, valid time and issue time | Keep dust concentration and optical depth as separate variables with their own units. |
| <a id="marine"></a>marine | `design/field` | Model, variable, cell and run where available | Offshore grid, valid time and variable reference level | Preserve wave, water-level or current unit and wet-cell mask. Local comparison requires observations. |
| <a id="rainviewer"></a>rainviewer | `defer/display` | Product, tile coordinates and image time | Tile bounds and past radar image interval | Visual color values only. Demonstrate regional coverage and permitted use before integration. |
