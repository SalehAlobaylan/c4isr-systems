# Source expansion for future simulation environments

First reviewed 17 November 2026. Revised 18 November after inspecting Riyadh Municipality's public 940 attachments and reviewing sources for civilian inspection, mobility and recovery. World Monitor is EMAD's primary discovery reference. Its transport, imagery, population, aviation and maritime leads are supplemented below with specific Saudi publications and civilian research datasets. [World Monitor catalog](https://www.worldmonitor.app/docs/data-sources) · [Provider inventory](https://www.worldmonitor.app/docs/source-attribution)

## Recommendation

The [source and feature delivery plans](../../plans/archive/simulation-sources/README.md) turn the shortlist into proposed features, implementation steps and acceptance cases. Two follow-up checks confirmed a published adsb.lol daily archive and a historical Jubail port-call record. Saudi civil air coverage and completeness of the Jubail series still need interval-specific checks. [Archive documentation](https://www.adsb.lol/docs/open-data/historical) · [PortWatch sample and plan](../../plans/archive/simulation-sources/03-regional-response-and-coast.md)

**Start with the decision to dispatch an inspection crew and reopen a road after a debris report.** A useful first Riyadh package combines municipal report patterns, a connected road network, a dated population layer and a small set of independently reviewed inspection outcomes. The exercise can compare a manual queue, rules that flag repeated reports and an AI-assisted queue. Measure time to verified reopening, duplicate dispatches and premature reopenings on the same cases. Clearance requires evidence from an inspection. A municipal ticket can close for other reasons.

For civilian drones, prioritize measured flight logs, incident narratives, labeled inspection imagery and a permitted historical civil traffic window. For the eastern coast, add regional response demand and maritime activity. For a southern highland area, prioritize road connectivity and evidence of actual closures. Weather and official warnings enter an exercise when a specific condition changes the decision being tested.

## Data for incident response and civilian drone exercises

| Question | Addition and relationship to World Monitor | First use and qualification |
| --- | --- | --- |
| How many road and debris reports arrive, repeat and close? | **Saudi extension.** Riyadh Municipality's quarterly 940 service records contain categories, location labels, status, receipt time, a closure field and a repeat flag. | Build an Arabic report queue using category frequencies and receipt times. Preserve the publisher's repeat flag. A reviewer must establish whether two reports concern the same obstruction. [Q1 2026 municipal publication](https://alriyadh.gov.sa/ar/data-sets/940-service-requests/940-requests-group-q1-2026) |
| Which routes remain connected when a road closes? | **Transport extension.** World Monitor lists Canadian 511 feeds. For Saudi roads, compare EMAD's retained OSM features with Overture segments and connectors. Balady's excavation service documents a street-closure status. | Test one connected detour and one disconnected area. Preserve one-way restrictions, bridge crossings and clipped boundaries. Overture shares OSM ancestry and publishes transportation under ODbL. Balady describes a permit workflow. Historical exports and evidence of actual closure/reopening still need checking. [Overture transportation](https://docs.overturemaps.org/guides/transportation/) · [Balady guide](https://balady.gov.sa/en/services/11386/guide) |
| Can a drone inspection help verify disrupted access? | **Imagery extension.** World Monitor lists Earth Search. Copernicus EMS Rapid Mapping publishes event-specific delineation and damage products. FloodNet supplies civilian drone imagery with labeled flooded and clear roads after Hurricane Harvey. | Pair source observations with assessment time and visibility. Use FloodNet's held-out labels to evaluate flood inspection. Saudi debris inspection needs a separate local reference set with a reviewer, inspection time, passability verdict and reopening evidence. [Copernicus EMS products](https://mapping.emergency.copernicus.eu/about/how-to-harvest-cems-mapping-data/emergency-response-data/) · [FloodNet data and license](https://github.com/BinaLab/FloodNet-Supervised_v1.0) |
| Does a simulated civilian inspection flight reproduce measured battery use? | **Research extension.** A Carnegie Mellon study recorded voltage, current and other measurements over 209 flights of one delivery quadcopter. NASA ASRS separately publishes deidentified civilian drone safety narratives. | Use held-out whole flights to check energy estimates on the recorded platform. Use narratives to select handoff or lost-contact exercises. Local aircraft performance needs local measurements. Voluntary reporting cannot establish failure rates. [Flight study](https://pmc.ncbi.nlm.nih.gov/articles/PMC8213793/) · [NASA UAS reports](https://asrs.arc.nasa.gov/docs/rpsts/uas.pdf) |
| What level of civilian service demand should the exercise represent? | **Saudi extension to population context.** The Ministry of Health lists emergency beds by health region. The Red Crescent reports Eastern Province calls, field responses and transfers. Saudi Press Agency reports the authority's Asir annual figures by sector. | Use the published totals to check regional workload assumptions. Keep health-region definitions, year and denominator with each figure. Shift staffing, individual facility availability and local travel times require separate records. [MOH data library](https://www.moh.gov.sa/en/ministry/opendata/pages/open-data-library.aspx) · [Eastern Province, 2025](https://www.srca.gov.sa/en/news/red-crescent-eastern-region-emergency-calls-2025/) · [Asir, 2024](https://www.spa.gov.sa/N2236149) |
| How does disruption change coastal activity over several days? | **Direct catalog lead.** IMF PortWatch derives daily port calls and shipment estimates from AIS. EMAD already has a Jubail shoreline/bathymetry pilot and an AISStream adapter. | Compare a port's daily activity with its own baseline in a coastal supply exercise. Check the port identity, missing days and reuse terms. Use a separately admitted AIS window when individual civilian vessel observations are needed. [IMF methods](https://www.elibrary.imf.org/view/journals/001/2025/093/article-A001-en.xml) · [Jubail pilot](../implementation/COASTAL_MOVING_AND_READINESS.md) |

### What the Riyadh 940 files actually contain

On 18 November, both Q1 attachments were parsed in memory from the municipal publication. The **JSON contains 6,820 records**. Decoding the receipt field as Unix milliseconds yields dates from 1 January through 31 March 2026 UTC. The dataset includes 524 asphalt/pothole reports and 301 construction-waste reports associated with buildings under construction. These categories provide examples of ordinary demand competing for a crew's attention. A shrapnel exercise would introduce its own clearly marked reports and independently adjudicated outcomes. [Riyadh Municipality Q1 publication](https://alriyadh.gov.sa/ar/data-sets/940-service-requests/940-requests-group-q1-2026)

The CSV attachment on the same page has **13,640 data rows**. Its first 6,820 match the JSON when compared as a multiset on type, location, status, closure field and repeat flag, after trimming whitespace. Some CSV receipt values appear as minute/second fragments. Use the dated JSON for the first replay candidate and resolve the extra CSV rows before deriving rates. The six fields provide no stable incident ID or coordinate pair. Keep location labels at their published granularity and confirm closure units before estimating service time. For reproducibility, the SHA-256 digests observed were `b3619c468a0e91c06d718ed7421b7f18deb5c7d78535f54046f7f807b8b60a21` (JSON) and `9dbad257469e1d481f8d54b38ade3eadaa87c6e739c661cfc90bcc09959978a8` (CSV).

The first package can therefore use measured report arrivals alongside explicitly scripted obstructions on reviewed road segments. Keep the field report, inspector's verdict and simulated road state as distinct records. This makes a false reopening or a missed obstruction observable when comparing procedures. The existing offline reader, topology fixture and suitability assessments provide starting points for packaging and checking this data. The fixture establishes software behavior. Saudi road connectivity and independently verified inspection outcomes still need local checks. [18 November delivery record](../implementation/SIMULATION_PREPARATION_DELIVERY.md)

## Existing capabilities and the incremental opportunity

The [adapter inventory](../../internal/publicingest/capabilities.go), [implementation record](../../plans/IMPLEMENTATION_PROGRESS.md) and [coastal/moving evidence](../implementation/COASTAL_MOVING_AND_READINESS.md) establish the following baseline. An implemented adapter does not imply that a source is active or that a selected region has retained observations.

| Existing capability | Evidence and current limit | Useful next addition |
| --- | --- | --- |
| Riyadh terrain, modeled ground, land cover, imagery and vector features | Building heights and road widths remain unknown; local physical accuracy is unqualified. [Enrichment evidence](../implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md) | Population exposure, verified facility attributes, connected roads and independent terrain/building controls |
| ERA5 pressure-field pilot | One retained central-Saudi slice from 23 October 2022, with wind and temperature at two pressure levels. [Pilot evidence](../implementation/RIYADH_OBSERVATION_PILOT.md) | Time sequences, additional relevant levels/variables, native-grid access and explicit forecast/reanalysis semantics |
| METAR | Current-context station adapter; no general historical-weather contract | TAF and international SIGMET products, where actual Saudi station/region coverage is demonstrated |
| OpenSky and AISStream | Current aircraft snapshots and vessel stream adapters; moving history reads records EMAD has retained. The last documented staging audit had zero admitted moving observations | Admit and measure one real civilian traffic window; assess alternative providers only against demonstrated gaps |
| USGS, FIRMS, GDACS and EONET | Earthquakes, thermal detections, earthquake bulletins and a historical flood-report adapter; these are not a general physical hazard model | Broaden justified event categories and add official warning lifecycles; retain upstream relationships |
| Jubail coastal and NOAA-20 orbital pilots | Static bathymetry/shoreline and bounded civil orbital context | Marine fields for the coastal use case; additional orbital work only for a defined civil observation question |

## Candidate shortlist

The first package draws on the incident-response sources above and the WorldPop row below. Civil aviation sources take priority for the drone and airspace exercise. Priority 2 indicates a specific exercise dependency. Priority 3 indicates background material or an unresolved source lead.

Discovery labels distinguish **catalog** entries, **inventory** hosts, and **extensions** found by following a provider or investigating a Saudi-specific gap. A catalog mention does not establish that World Monitor currently operates every related provider product.

| Candidate | Discovery and EMAD delta | Simulation value | Priority and principal condition |
| --- | --- | --- | --- |
| Open-Meteo forecast, historical weather and individual forecast runs | Catalog provider with additional products | Weather-sensitive civilian drone and access scenarios | **2.** Select the model, height reference and access terms when the exercise requires a weather field |
| Aviation Weather Center TAF and international SIGMET | Extension of the existing METAR provider | Terminal forecasts and aviation weather advisories | **2.** Use for civil airspace exercises with a weather decision |
| WMO warning discovery and Saudi NCM CAP | Catalog lead and WMO-listed Saudi authority | Warning revisions and cancellations | **2.** Check feed availability and retention when an exercise involves warning delivery |
| WorldPop Saudi population grids | Catalog freshness-tracker mention; exact Saudi products checked at WorldPop | Estimated population exposed to a supplied hazard footprint, evacuation demand and recovery planning | **1:** pin product/year, respect modeled uncertainty and establish exact asset terms |
| OpenSky or adsb.lol civilian aircraft reports | Catalog lead and existing OpenSky adapter | Historical civilian traffic, observations arriving late and gaps in reports | **Civil aviation priority.** Check Saudi coverage, retention and historical access for one window |
| ICAO and FAA NOTAM distribution, Saudi SANS eAIP | Catalog distributors and Saudi authority | Effective airport restrictions and civil airspace context | **Civil aviation priority.** Check effective editions and access for the traffic window |
| CAMS dust/aerosols through Open-Meteo | Provider extension beyond the catalog's climate product | Dust and reduced-visibility scenario conditions | **2:** global-product resolution applies in Saudi Arabia; sensor effects need separate models |
| Open-Meteo marine fields | Provider extension | Coastal rescue, drifting-object context and weather-dependent access | **2:** local grid coverage, vertical reference and nearshore suitability |
| GPSJAM | Catalog | Context for navigation-degradation and incomplete-observation scenarios | **2:** derived daily report, unresolved bulk/history/export access; no causal interference claim |
| Cloudflare Radar outages | Catalog | Internet-service disruption timelines and degraded reporting workflows | **2:** read token, permitted reuse and country/network coverage |
| AISStream, IMF PortWatch, NGA warnings and UKMTO reports | Catalog/linked maritime domain; AISStream already implemented | Vessel context, port disruption and maritime warning timelines | **2:** separate live positions, aggregates and reports; verify each source's geography and rights |
| USGS / FIRMS / GDACS / EONET extensions | Catalog; existing integrations and pilots | Environmental-event context and historical incident selection | **2:** extend specific products/categories, preserve shared upstream lineage |
| HDX/HAPI, UNHCR and IOM DTM | Catalog humanitarian leads | Regional humanitarian context and response-demand indicators | **3:** dataset-specific geography and dates; Saudi local facility coverage is unverified |
| Cable, port and other infrastructure listings | Catalog | Broad service-dependency and restoration context | **3:** original asset provenance, operational dependencies and local detail are still needed |

## Provider findings that affect the choice

### 1. Atmospheric inputs for weather-sensitive exercises

[Open-Meteo's forecast documentation](https://open-meteo.com/en/docs) exposes wind, gusts, temperature, precipitation, visibility and other variables, with availability varying by model. For EMAD, a bounded time series would add substantially more than the current isolated weather slice. Select the model and retain grid/elevation semantics instead of treating every response coordinate as an independent local measurement.

Treat coordinate-query responses as prepared series. A consumer requiring native three-dimensional atmospheric fields needs the corresponding producer artifacts and vertical-coordinate metadata; collecting many point queries does not establish that representation. Preserve the existing native ERA5 acquisition route for uses that need it.

Its [Historical Forecast API](https://open-meteo.com/en/docs/historical-forecast-api) stitches early forecast hours into a continuous series. That is useful historical model context, but does not preserve a complete forecast as it was available to an operator. The [Single Runs API](https://open-meteo.com/en/docs/single-runs-api) selects an initialization time and full forecast horizon. For decision replay, preserve initialization, valid time and actual availability separately. Later reanalysis must not become information the operator supposedly knew earlier.

The provider's [terms](https://open-meteo.com/en/terms) distinguish the free non-commercial service from paid commercial access. Document the chosen service arrangement and attribution before admission. No paid plan or account was created for this assessment.

[Aviation Weather Center](https://aviationweather.gov/data/api/) documents worldwide METAR, TAF and SIGMET products. Extend the existing provider family rather than creating an unrelated weather pipeline. Worldwide product scope still requires checks for the selected Saudi stations, reporting windows and message completeness. A station sample, terminal forecast and regional warning remain different evidence types.

### 2. Official Saudi weather warnings: a concrete local lead

The [WMO Register of Alerting Authorities entry for Saudi Arabia](https://alertingauthority.wmo.int/authorities.php?recId=140) lists Arabic and English NCM CAP URLs. This provides a more relevant starting point than importing US or Canadian warning coverage into a Saudi scenario.

The Arabic NCM endpoint was not accessible through the research browser. The directory entry verifies a discovery lead, not a working connector. The next acquisition check should establish a successful sample, publisher identity, feed format, geographic coverage and permitted retention. A future CAP adapter should preserve identifiers, issue/effective/expiry times, geometry, language, updates and cancellations. A missing feed or expired alert must remain distinguishable from a successful empty query.

### 3. Population exposure: a major missing layer

WorldPop publishes specific Saudi population products. A [Saudi 2024 constrained product record](https://hub.worldpop.org/geodata/summary?id=75294) describes estimates in people per grid cell at approximately 100 m at the equator, within the R2025A release, and identifies that release as an alpha product. The [population catalog](https://hub.worldpop.org/project/categories?id=3) explains the available product families. Choose a year and constrained/unconstrained method deliberately; preserve counts, nodata and cell areas when resampling.

The [WorldPop API description](https://api.worldpop.org/v3/about) identifies a CC BY 4.0 offering. Product pages also distinguish datasets with ODbL-derived inputs. Bind permission and attribution to the selected asset and release rather than assuming all WorldPop products have identical terms.

This would support population-exposure estimates for an independently supplied hazard footprint. Residential population does not establish moment-to-moment occupancy, people outdoors, shelter availability or injury outcomes. National demographic totals and displacement counts cannot substitute for local population grids. Emergency facilities and their capacities require their own sources.

### 4. Aircraft and airspace: finish real coverage, then add products

Use the existing OpenSky implementation as the initial comparison point. [OpenSky's FAQ](https://opensky-network.org/about/faq) explains coverage and aircraft-equipment gaps. Our moving-history API is a reader for retained observations, not proof that the provider can backfill any requested historical period.

adsb.lol documents a daily historical archive under ODbL 1.0. This gives the civilian traffic exercise a reproducible acquisition route to evaluate alongside OpenSky. Check coverage on one historical Saudi interval and review archive reuse terms. The live API has a separate access path. [Historical archive](https://www.adsb.lol/docs/open-data/historical) · [API access](https://www.adsb.lol/request)

[ICAO's API Data Service](https://www.icao.int/api-data-service) and [FAA NMS](https://nms.aim.faa.gov/) are NOTAM access candidates; FAA identifies an API access-request process. The [SANS published eAIP index](https://aimss.sans.com.sa/assets/FileManagerFiles/History-en-SA.html) is the Saudi-specific reference lead for aeronautical editions and supplements. Data extraction, machine access and redistribution rights remain to be established. AIP baseline information and time-limited NOTAMs should be stored separately, with effective times and supersession. Altitude references must remain explicit.

Airport delay statistics may support transport-disruption scenarios but do not describe complete airspace restrictions. Civil ADS-B tracks do not establish missile tracks, small-drone coverage or authoritative object identity.

### 5. Dust and marine fields: valuable, conditional extensions

[Open-Meteo's air-quality product](https://open-meteo.com/en/docs/air-quality-api) includes dust and aerosol optical depth. Its documentation distinguishes the European product from the global CAMS atmospheric-composition forecast, listed at 0.4 degrees, approximately 45 km, with native three-hourly data. The European approximately 11 km figure must not be applied to Saudi dust coverage. Keep any temporal interpolation explicit. These fields provide regional atmospheric context; local visibility and camera behavior require additional validation.

The [marine API](https://open-meteo.com/en/docs/marine-weather-api) offers waves, sea-level and current variables. Its documentation describes approximately 8 km modeled tides/currents and limited coastal accuracy. These are candidates for extending the Jubail environment, subject to a cell-coverage and reference-level check. Static GEBCO depth plus a regional marine forecast does not establish a validated harbor-flow model. Historical windows and individual-run availability must be checked for the selected marine product rather than inherited from the atmospheric API.

### 6. Disruption context: retain the evidence's actual meaning

[GPSJAM's FAQ](https://gpsjam.org/faq) describes 24-hour aggregates of aircraft-reported navigation accuracy. It explicitly says degraded accuracy does not establish its cause. It currently names both airplanes.live and ADS-B Exchange, which is more specific than the catalog's ADS-B Exchange summary. Missing cells can reflect missing aircraft or receivers. Retain the aggregation window, counts and coverage; do not translate the map into a local RF field, a confirmed jammer location or a guaranteed drone-navigation failure. A documented permission and bulk/history-access route remains unresolved.

[Cloudflare's Radar API](https://developers.cloudflare.com/api/go/resources/radar/subresources/annotations/) supplies outage/anomaly annotations, and its [access guide](https://developers.cloudflare.com/radar/get-started/first-request/) requires a token with Radar read permission. This can supply historical context for delayed reporting or service-outage scenarios. Internet measurements are not direct evidence about radio links, power circuits or an individual facility's availability. Product terms and useful Saudi country/network detail require an admission check.

## Other catalog leads and reasons to defer them

- **Maritime context.** AISStream documents a keyed geographic subscription. PortWatch provides daily aggregate activity. The World Monitor maritime guide traces UKMTO reports to a republisher. Record the issuing organization, transport and reporting time for each retained product. Check actual coverage, dates and reuse terms for the selected coastal area. [AISStream](https://aisstream.io/documentation) · [IMF methods](https://www.elibrary.imf.org/view/journals/001/2025/093/article-A001-en.xml) · [World Monitor maritime guide](https://www.worldmonitor.app/docs/maritime-intelligence)
- **Hazards:** reuse the existing USGS/FIRMS/GDACS/EONET capabilities. EMAD's EONET pilot already records GDACS upstream references, so two feeds can describe the same underlying evidence. A hotspot is not a fire perimeter; a reported flood is not a water-depth field. New physical flood questions need rainfall, drainage, ground and validation data beyond event catalogs.
- **Humanitarian data:** [HDX HAPI documentation](https://hdx-hapi.readthedocs.io/en/latest/) describes standardized indicators with country and administrative-level coverage that varies by dataset. Its focus includes countries with humanitarian response plans; do not assume Saudi local coverage. UNHCR and IOM data can support regional context while retaining their source dates and geographic units.
- **RainViewer:** the host appears in World Monitor's attribution inventory, but the provider's [transition notice](https://www.rainviewer.com/api/transition-faq.html) records discontinued nowcasts, satellite IR and the radar database API, plus restricted remaining services. Defer a new dependency until current access and actual Saudi radar coverage are demonstrated. Rendered radar tiles are not automatically quantitative precipitation inputs.
- **Reservoir levels and Sentinel-5P:** the catalog identifies water-level and atmospheric-column products. They are conditional leads for a specific environmental question; neither supplies near-real-time flash-flood depths or street-level smoke conditions by itself. Product, latency and local validation checks are still needed before prioritization.
- **Infrastructure listings:** cable/port locations can support broad context, but a location or map line does not establish ownership, operational state, connectivity, capacity or physical accuracy. Obtain the original dataset and required operational attributes before representing restoration dependencies.
- **News, conflict event databases, prediction markets and composite risk scores:** retain as optional analyst context or incident-discovery leads. Publication activity and dashboard scoring rules are not physical parameters, event truth or validated probabilities for a simulation. Financial feeds, military watchlists and broad strategic inventories do not address the immediate environmental-data gaps.

## Applying the additions to the simulation ideas

| Future question | Most useful source additions | What still needs independent work |
| --- | --- | --- |
| Civilian drone inspection | Measured civilian flight logs, safety narratives, labeled aerial images and dated civilian traffic | Local aircraft and inspection benchmarks, representative cases and known missing observations |
| Debris/shrapnel reports and recovery | Municipal report patterns, independently reviewed inspection outcomes, road closures and population estimates | Local verification of each closure and clearance, plus separately identified scenario assumptions |
| Civilian recovery after a reported missile event | Supplied incident report, time-stamped access decisions, regional service demand and restoration records | Independent evidence for affected areas, observed service interruptions and reopening times |
| Mixed civilian airspace and incomplete observations | Retained aircraft reports, airspace information, forecast versions and source-quality metadata | Sensor/observation assumptions, identity uncertainty and representative ground truth |
| Southern road access after flooding | Connected roads, damage-mapping products where an activation exists and Asir response-demand statistics | Select a local area, verify passability against inspections and qualify local map coverage |
| Eastern coastal response and disruption | Jubail geography, historical civilian AIS where permitted, PortWatch daily activity and regional demand | Check the port series and nearshore coverage. Local operating limits and response times need independent evidence |

Choose bounded areas independently: retain Riyadh as the reference; evaluate the existing Jubail area for the eastern coastal branch; select a specific southern area according to its question and available data. A southern mountain area and a southern coastal area need different inputs. No equivalent detailed southern release is established by the reviewed records.

## Acquisition sequence and reviewable evidence

1. **Define requirements using P01.** Name the question, area, time window, variables, tolerable uncertainty and intended use. Distinguish reported context, modeled environmental fields, static reference data and later synthetic inputs.
2. **Run a metadata/access preflight.** Confirm the original provider, selected edition/model, Saudi footprint, credentials, current terms, quotas and permitted retention, analysis, derived use and export. Inspect exact product metadata before downloading a large dataset. Existing source admission remains authoritative.
3. **Prepare the Riyadh access and inspection package.** Use the dated 940 JSON after checking reuse terms. Add one reviewed road network and a dated WorldPop aggregate. Attach inspection evidence where available and mark exercise-authored road closures and verdicts as synthetic. Check whether a receiver can reproduce the report queue, reachability changes and outstanding inspections.
4. **Qualify one traffic source and airspace package.** If a provider offers only a current stream, begin permitted retention and use that recorded window. Do not promise arbitrary backfill. Preserve gaps, uncertain associations, timestamps and restriction revisions.
5. **Load and verify the package.** Use the delivered P03 native reader and P05 consumer kit where they support the selected representations. Document any missing reader for reports, road events or inspection outcomes. The P06 field reader is available for optional environmental inputs. [Delivery record](../implementation/SIMULATION_PREPARATION_DELIVERY.md)
6. **Validate the selected workload under P04/P08.** Check control samples, spatial boundaries, partial responses, time cutoffs, corrections, cancellations and offline reopening. Compare eligible field values with independent observations where possible, without treating station and model values as equivalent. Add a conditional source family only when a scenario requires it.

Each candidate should produce a small evidence record containing: discovery URL; original producer; product/version; exact request and retrieval time; observation/issue/valid times; area and coverage result; native resolution and datum; documented access/rights; raw and derived digests; completeness/gaps; intended use; validation limits; and the reason to adopt, defer or reject it. Later API failure must remain distinct from a verified empty result.

AI and conventional workflows can use the same frozen inputs. Include any AI-training use in source admission and withhold later inspection verdicts from the inputs available at dispatch time.

## Remaining physical-data gaps

This catalog substantially broadens environmental and incident context. It does not close the existing gaps in surveyed building dimensions, structural properties, independently validated terrain, local turbulence, complete road/facility networks, occupancy or response capacity. Nor does it establish validated physical behavior for the future simulations. For debris work in particular, credible exposure context and credible physical consequences require different evidence.

For the first Riyadh package, a reviewer should be able to reproduce the municipal counts, identify which road links were checked, and trace each reopening decision to a dated inspection or to a clearly labeled exercise assumption.
