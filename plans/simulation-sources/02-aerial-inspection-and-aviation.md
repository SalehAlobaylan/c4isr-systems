# Civilian aerial inspection and historical aviation

Give the road reviewer three choices after viewing an image: send a crew, request a clearer view, or record what the image supports. Keep the imagery decision distinct from the field verdict that authorizes reopening. The following sources answer different parts of that workflow.

### Earth Search — find usable before-and-after images

**Feature.** A reviewer selects an area and incident date and sees the earliest usable post-event view alongside a comparable earlier view. Show acquisition time, sensor, pixel size, footprint and usable coverage over the road of interest. A scene-wide cloud percentage can hide a cloud directly over that road. [Earth Search STAC API](https://earth-search.aws.element84.com/v1/api.html)

**Build and test.** Query one supported imagery collection by area and interval, follow pagination, pin item and asset identifiers and check the collection's asset terms. Use the product's quality mask where available. Begin with coverage and visual review. Test a partial intersection, local cloud, a missing asset and a road narrower than the effective resolution. The result should show which area can be interpreted, and offer another inspection when detail is insufficient. Record processing and release time separately from acquisition.

### Copernicus EMS Rapid Mapping — compare event assessments over time

**Feature.** Place the area assessed by a mapping activation beside imagery and field reports. A planner can see where flood extent or a published damage grade supports sending a reviewer and where the activation leaves a gap. [Rapid Mapping API and product metadata](https://mapping.emergency.copernicus.eu/about/how-to-harvest-cems-mapping-data/emergency-response-data/)

**Build and test.** Select one published flood activation with a usable area, product type, delivered version and source imagery dates. Preserve the assessed area and legend for each delineation or grading product. Keep monitoring updates as revisions. At a historical cutoff show only released versions. Exercise one mapped area, one outside it and one where the assessment changes. If the map used the same image that the model saw, record the shared source. Compare with a separately timed field observation for an independent road-access verdict. Use an overseas activation for software validation if the Saudi search returns no suitable product.

### FloodNet — test whether image assistance improves flood inspection

**Feature.** Give a reviewer an optional flooded-road overlay and a way to mark a view inconclusive. Evaluate how often assistance reduces review time and how often it could lead to a flooded road being called clear. FloodNet contains 2,343 post-Harvey aerial images and distinguishes flooded and non-flooded roads. Its labels describe the image and its published geography. [Authors' dataset, split and license](https://github.com/BinaLab/FloodNet-Supervised_v1.0)

**Build and test.** Preserve the published split, class IDs, mask alignment and license. Use training data for development, validation for threshold selection and the untouched test partition for reporting. Check for near-duplicate or adjacent views crossing partitions and disclose residual overlap. Compare unaided review, a simple image baseline and AI-assisted review. Report flooded-road intersection-over-union, false-clear decisions with their denominator, abstentions and time per image. Agree on the verdict rule before scoring. Saudi transfer needs an independent local set grouped by incident or collection site, including unreadable views and dry roads. Local obstruction and reopening labels come from the inspection plan. [Guidance on spatially structured validation](https://doi.org/10.1111/ecog.02881) · [Local inspections](01-reports-roads-and-inspections.md)

### CMU AirLab — benchmark the battery display for a civilian survey

**Feature.** Replay a completed research flight and compare a simulated energy display with measured use. Let a reviewer see where the display drifts or the telemetry becomes insufficient. The authors distinguish 195 flights varying study conditions and 14 ancillary-power or hover recordings. [AirLab experiment](https://theairlab.org/energy-dataset/) · [Archived data](https://doi.org/10.1184/R1/12683453)

**Build and test.** Preserve recording ID, platform, test type, units and elapsed time. Derive a reference in watt-hours from voltage, current and elapsed intervals, handling gaps explicitly. Use a constant average-power estimate fitted on training flights as a baseline. Hold out whole flights and report median and 90th-percentile absolute energy error, sample count and excluded duration by test type. Include a known constant-power fixture to catch time or unit errors. The research aircraft gives a reproducible benchmark. A fleet-specific battery display needs independently recorded flights on that fleet.

### NASA ASRS — build and score civilian handoff exercises

**Feature.** A trainer selects a deidentified UAS narrative about incomplete information or a communication handoff. The operator sees evidence in stages, chooses whether to pause the inspection and records who owns the follow-up. [NASA UAS report collection](https://asrs.arc.nasa.gov/docs/rpsts/uas.pdf) · [ASRS database](https://asrsdbol.arc.nasa.gov/)

**Build and test.** Have a reviewer code the reported sequence, what the narrator could observe and what remains uncertain. Store the source reference with an independently authored exercise and scoring rubric. A second reviewer checks the coding. Score acknowledgment, handoff completion and unresolved follow-ups. Keep later narrative outcomes hidden until debrief. Voluntary reports support case selection. A frequency estimate would require a separate exposure denominator and representative reporting.

### OpenSky and adsb.lol — show historical civil traffic coverage

**Feature.** An exercise designer selects a historical interval and sees aggregate counts of observed civilian traffic, minutes with usable reports and gaps. This supplies context for airport disruption and tests how a reviewer handles stale information.

- **OpenSky.** Read the existing adapter and retained-history path first. Check admission, a permitted historical extract or recorded interval, times and local coverage. Summarize eligible civil reports by time bucket, retain missing intervals and show the oldest observation used in each bucket. A controlled gap and duplicate must leave the expected gap and count in an offline replay. [OpenSky coverage FAQ](https://opensky-network.org/about/faq) · [EMAD moving-data baseline](../../docs/implementation/COASTAL_MOVING_AND_READINESS.md)
- **adsb.lol.** The provider now documents a daily historical archive and ODbL terms. Take one permitted historical civil sample for the same window, apply the same aggregation and compare the number of usable intervals and overlap. Shared receiver inputs limit independence. Add a second importer only if the comparison fills a measured coverage gap or makes the historical exercise reproducible. Saudi coverage for a chosen interval remains to be measured. [Provider archive documentation](https://www.adsb.lol/docs/open-data/historical)

### SANS eAIP, ICAO and FAA — replay published civil airport changes

**Feature.** At each exercise cutoff, show the aeronautical edition and dated public civil airport notices relevant to an airport-service disruption. Let a reviewer distinguish what had been published from what had taken effect.

- **SANS eAIP.** Begin with a dated Saudi edition and an amendment or supplement. Record publication and effective dates, references, permitted extraction and superseded records. Select the right baseline on either side of the edition boundary. [SANS edition index](https://aimss.sans.com.sa/assets/FileManagerFiles/History-en-SA.html)
- **ICAO.** Check product access and Saudi coverage for the exercise window. Retain a civil airport notice, its issuer, validity and replacement or cancellation if available. A later replacement must leave the earlier historical view reproducible. [ICAO API Data Service](https://www.icao.int/api-data-service)
- **FAA NMS.** Evaluate as a distribution alternative for the same notice, subject to approved access and retention. Compare identifiers, text revisions and timing with the issuer before adding an adapter. Receiving the same notice twice must produce one underlying restriction with both delivery records. If historical access is unavailable, use an authored cancellation case for software verification. [FAA NOTAM service](https://nms.aim.faa.gov/)

**Handoff.** Deliver a frozen image pair and quality mask, a reference label set with a documented split, a completed-flight energy comparison, and one handoff drill. The aviation context can ship independently once a historical civil window and its terms are qualified. At debrief the reviewer should be able to trace an inspection suggestion back to a visible image region and explain why a field visit was still needed.
