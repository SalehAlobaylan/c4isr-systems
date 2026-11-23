# Regional response demand and coastal recovery

The eastern and southern exercises answer different questions. At Jubail, can a logistics planner distinguish a sustained change in recorded port activity from an isolated quiet day? In Asir, how much does a reviewed road closure change civilian service access under different assumptions about demand? Regional statistics constrain the exercises. The operator should be able to change a staffing or arrival assumption and see which result depends on it.

Atlas and the offline worker now deliver the regional service comparison, coastal evidence and held-out PortWatch validation. [Regional delivery, evidence and replay commands](regional-response-delivery.md)

### Saudi Red Crescent, Eastern Province — calibrate the workload mix

Field mapping · [srca-east](normalization.md#srca-east)

**Feature.** A scenario designer compares an authored workload with the region's published numbers for emergency reports, field cases, hospital transfers and care at scene. Show year, geography, unit and reporting channel on every figure. [SRCA 2025 release](https://www.srca.gov.sa/en/news/red-crescent-eastern-region-emergency-calls-2025/)

**Build and test.** Start with a source-cited table for 2025 and two authored shift profiles with the same total demand but different peaks. Keep the profiles labeled as assumptions. The release gives 148,544 emergency reports and 110,718 field cases. Its 71,841 transfers and 38,865 cases treated at scene sum to 110,706. Preserve the difference of 12 without assigning a cause. Keep total calls and app reports in their published categories until their overlap is known. Test reconciliation, then compare wait-time distributions and unfinished cases under the two profiles using the same staffing assumption. Local operational calibration requires time-stamped regional arrivals and staffing records.

### Saudi Red Crescent via Saudi Press Agency, Asir — exercise southern access

Field mapping · [srca-asir](normalization.md#srca-asir)

**Feature.** A planner closes one independently reviewed road in a southern study area and tests access to an exercise service destination under steady and peaked demand. The regional publication reports 71,804 emergency reports handled in 2024. Keep the Arabic measure and its definition alongside the translation. [Official Asir report](https://www.spa.gov.sa/N2236149)

**Build and test.** Transcribe the regional total and any sector figures with their original labels. Check which categories partition the total before aggregating. Select a bounded area only after road coverage, alternative connections and a reviewer are available. Use synthetic demand allocation for the pilot and show how results change across allocations. A closed edge should alter reachability only for affected zones and restore it when reopened. Compare with the Eastern Province figures only when years, geography and workload definitions match. [Road and population plan](01-reports-roads-and-inspections.md)

### Saudi Ministry of Health — regional service-capacity assumptions

Field mapping · [moh-beds](normalization.md#moh-beds)

**Feature.** Display published emergency beds beside editable exercise capacity, so a planner can see why queues change when fewer receiving slots are assumed. The MOH library lists emergency beds by health region in MOH hospitals for 2023. [MOH open-data library](https://www.moh.gov.sa/en/ministry/opendata/pages/open-data-library.aspx)

**Build and test.** Pin the workbook, sheet, reporting year, region code, facility scope and unit. Validate the region crosswalk against the Red Crescent geography and keep incompatible areas separate. Show annual installed counts as reference. Ask the exercise owner to specify hypothetical staffed slots and service durations separately. A change in assumed availability must affect the simulated queue while leaving the published count intact. Leave any unmatched health region or missing cell visible. A field-capacity claim needs facility-specific, dated evidence.

### IMF PortWatch — measure daily activity and recovery at Jubail

Field mapping · [portwatch](normalization.md#portwatch)

The Atlas Environment Library supports reviewed Jubail and Jazan files with a fixed historical weekday baseline. The offline reader also reports when the requested consecutive return criterion was first met. [Atlas workflow](delivery.md#review-a-source-in-the-atlas-environment-library)

**Feature.** Compare each day's observed estimate with the port's own historical distribution. For a supplied disruption interval, report the departure from baseline, duration and first sustained return to the historical range. Show the reference period and how return was defined. PortWatch uses AIS-derived estimates, so an activity change alone cannot identify its cause. [IMF methods](https://www.elibrary.imf.org/view/journals/001/2025/093/article-A001-en.xml) · [PortWatch](https://portwatch.imf.org/)

**Verified example.** A read-only query of the daily port layer for Saudi Arabia on 15 January 2024 returned four port calls for Jubail (port24) and zero for Jazan (port2074). This establishes a historical Jubail record and a useful zero-value test. It leaves the full series, revision history and local logistics interpretation to qualify. [Reproduce the dated provider query](https://services9.arcgis.com/weJ1QsnbMYJlCHdG/arcgis/rest/services/Daily_Ports_Data/FeatureServer/0/query?where=ISO3%3D%27SAU%27%20AND%20year%3D2024%20AND%20month%3D1%20AND%20day%3D15&outFields=portname%2Cportid%2Cyear%2Cmonth%2Cday%2Cportcalls&returnGeometry=false&f=pjson)

**Build and test.** Retrieve one calendar year for the same port ID, check pagination, missing dates, zeros, repeated port-days and retrospective revisions. Begin with total calls. Add shipment estimates only with documented units and coverage. Fit a rolling weekday baseline using only earlier dates and compare with a simple trailing median on held-out weeks. Record error and interval coverage before interpreting anomalies. On fixtures, a zero enters the baseline, an absent day stays unknown and an isolated quiet day fails a sustained-recovery rule. Move to multiple years before claiming seasonal effects. Use archived as-published editions or label the analysis retrospective.

Before computing recovery, fix the comparison window, minimum number of valid preceding weekdays, minimum consecutive in-range days and treatment of missing days. A missing day interrupts the run of consecutive evidence. If the held-out period ends before the criterion is met, report recovery as unresolved.

### AISStream — verify the completeness of a civilian maritime replay

Field mapping · [aisstream](normalization.md#aisstream)

**Feature.** A playback coverage strip shows whether a retained civilian shipping interval contains messages and when collection stopped. Compare historical activity at daily resolution with PortWatch after recording the different definitions. [AISStream subscription documentation](https://aisstream.io/documentation)

**Build and test.** Use the existing adapter under an admitted, authorized recording window or a permitted historical capture. Record receipt intervals, disconnects and duplicate messages. Export coarse activity counts when permitted. A forced disconnect creates an unknown interval and duplicates leave counts stable. PortWatch and AISStream both draw on AIS. A message count measures reception activity and cannot independently validate an estimated port call. [Current coastal and moving-data evidence](../../../docs/implementation/COASTAL_MOVING_AND_READINESS.md)

### Maritime notices and reports — explain what the planner knew

Field mapping · [nga-nav](normalization.md#nga-nav) · [ukmto](normalization.md#ukmto)

- **NGA navigational warnings.** Prepare a dated civil navigation notice about a temporary navigation-aid or service change. Preserve issuer, warning area, identifier, issue time, validity and cancellation. A timeline exercise should remove the notice at the correct point while retaining its earlier version. Check the issuing authority and availability of historical messages for the selected coast. [NGA warning guide](https://msi.nga.mil/api/publications/download?key=16694477%2FSFH00000%2F117chapter3.pdf)
- **UKMTO.** Use a historical public report or update as an attributed annotation for a civil logistics debrief. Capture original issue and update times, separate reported effects from verified service interruptions and preserve any republisher's link to the issuer. A correction should change the later debrief and leave the earlier one intact. Independent port-service records are needed to attribute a change in daily activity. [UKMTO public reports](https://www.ukmto.org/recent-incidents)

### GEBCO, shoreline and port inventory — orient the coastal exercise

Field mapping · [gebco](normalization.md#gebco) · [shoreline](normalization.md#shoreline) · [port-index](normalization.md#port-index)

**Feature.** Show the dated shoreline, offshore depth coverage and the port definition used for the daily series in the existing Jubail 3D area. Mark the cells and source editions behind a displayed depth. Use the World Port Index as an identifier and locality crosswalk where its definitions agree. [EMAD Jubail pilot](../../../docs/implementation/COASTAL_MOVING_AND_READINESS.md) · [World Port Index FAQ](https://msi.nga.mil/api/publications/download?key=16920959%2FSFH00000%2FWPIFAQ.pdf&type=view)

**Build and test.** Reuse the pinned coastal release, preserve horizontal and vertical references and test land, offshore and nodata samples. Check the inventory edition and manually reconcile ambiguous names with the PortWatch port ID. A regional depth or port listing supplies geographic context. Actual berth availability, service interruptions and reopening times need dated operator evidence. Add modeled water levels only for a question that can tolerate their resolution. [Marine fields](04-supporting-sources.md)

### Local response and service records

Field mapping · [local-response](normalization.md#local-response)

Obtain dated, deidentified service observations for the chosen region. Match actual or authored arrivals to staffed receiving slots and first service. A control case distinguishes a slot known to be unavailable from a slot whose status is unknown.

### Historical coastal search observations

Field mapping · [drifter-check](normalization.md#drifter-check)

Pair permitted historical observations of a civilian drifter with sea conditions for the same place and time. Reserve a blind portion for scoring. Report time to verified sighting, area searched and unresolved cases. Access, local coverage and reference quality remain open.

**Handoff.** Deliver the annotated Jubail series and its dated control query, one Asir access comparison, and the three regional demand/capacity tables with their definitions. A reviewer should be able to reproduce the portfolio of results with each manually chosen assumption visible and alter one assumption at a time.
