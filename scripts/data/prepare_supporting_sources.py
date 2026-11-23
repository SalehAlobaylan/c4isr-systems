"""Normalize retained, bounded environmental and humanitarian exercise evidence."""

import argparse
from datetime import datetime, timedelta, timezone
from hashlib import sha256
import json
from pathlib import Path
import re
import sys
from urllib.parse import parse_qs, urlsplit
from xml.etree import ElementTree as ET

from prepare_civilian_access import source
from simulation_inputs import instant, number, require, text, utc


def read(path):
    raw = path.read_bytes()
    require(0 < len(raw) <= 3 * 1024 * 1024, "expected a bounded source file")
    return json.loads(raw)


def event(identity, source_id, origin, kind, revision, seen, when, status, label, area, **extra):
    require(when >= seen, "event availability precedes source time")
    return {"id": identity, "sourceId": source_id, "origin": origin, "kind": kind, "revision": revision,
            "observedAt": utc(seen), "availableAt": utc(when), "status": status, "label": label, "area": area, **extra}


def usgs(path, area, revision):
    doc = read(path)
    require(doc["type"] == "Feature" and doc["properties"]["type"] == "earthquake", "expected one USGS earthquake detail")
    props = doc["properties"]
    at = lambda key: datetime.fromtimestamp(number(props[key])/1000, timezone.utc)
    key = text(doc["id"])
    identity = f"usgs-revision-{revision}"
    row = event(key, identity, "usgs:" + key, "earthquake", revision, at("time"), at("updated"), "observed",
                text(props["title"]), area, value=number(props["mag"]), unit=text(props["magType"]), details="Revised catalog event")
    return {"sources": [source(path, identity, "FDSN event detail", "U.S. Geological Survey")], "events": [row]}


def gdacs_key(url):
    parsed = urlsplit(url)
    require(parsed.scheme == "https" and parsed.hostname in {"gdacs.org", "www.gdacs.org"}, "unexpected GDACS issuer")
    params = {k.lower(): v for k,v in parse_qs(parsed.query).items()}
    family, identity = params["eventtype"], params["eventid"]
    require(len(identity) == len(family) == 1 and identity[0].isdigit() and family[0] in {"EQ", "FL"}, "invalid GDACS source key")
    return f"gdacs:{family[0]}:{identity[0]}"


def eonet(path, identity, area, capture):
    matches = [row for row in read(path)["events"] if row["id"] == identity]
    require(len(matches) == 1, "expected exactly one selected EONET event")
    row = matches[0]
    times = sorted(instant(item["date"]) for item in row["geometry"])
    require(times, "EONET geometry dates unavailable")
    upstream = {gdacs_key(item["url"]) for item in row["sources"] if item["id"] == "GDACS"}
    require(len(upstream) == 1, "catalog entry needs an unambiguous original GDACS citation")
    detail = f"Dated source geometry from {utc(times[0])} through {utc(times[-1])}."
    if row["closed"]:
        closed = instant(row["closed"])
        require(closed <= capture, "catalog closure follows retained capture time")
        detail += f" Catalogue closure {utc(closed)}."
    entry = event(identity, "eonet", upstream.pop(), "catalog-entry", 1, times[0], capture,
                  "closed" if row["closed"] else "published", text(row["title"]), area, details=detail)
    return {"sources": [source(path, "eonet", "EONET v3 retained flood catalog", "NASA EONET")], "events": [entry]}


def meteo(path, identity, edition, attribution, variable, label, area, native, kind, available, init, at):
    doc = read(path)
    require(doc.get("utc_offset_seconds") == 0 and not doc.get("error"), "expected a successful UTC hourly archive")
    samples, times = doc["hourly"][variable], doc["hourly"]["time"]
    require(len(times) == len(samples) and len(times) == len(set(times)), "invalid hourly values or repeated instant")
    require((kind == "forecast-run") == (init is not None), "single-run samples need their initialisation time")
    positions = [i for i,t in enumerate(times) if instant(t if t.endswith("Z") else t+"Z") == at]
    require(len(positions) == 1, "named UTC sample is missing from the response")
    value = samples[positions[0]]
    status = "no-coverage" if value is None else "available"
    if value is not None:
        number(value)
    row = {"id": f"{identity}-{at:%Y%m%dT%H%M}", "sourceId": identity, "kind": kind,
           "lane": "decision" if init else "debrief", "variable": label, "area": area, "support": "Provider model at a point",
           "nativeRef": native, "method": "point-model", "reference": "Provider model variable",
           "status": status, "units": text(doc["hourly_units"][variable]), "value": value,
           "samples": 1 if value is not None else 0, "coverage": None, "validFrom": utc(at), "availableAt": utc(available)}
    if init:
        row["initializedAt"] = utc(init)
    return {"sources": [source(path, identity, edition, attribution)], "fields": [row]}


def era5(path, time_at, available, pressure, row, col):
    data = read(path)
    require(data["schema"] == "emad.era5-pressure-subset.v1" and data["crs"] == "EPSG:4326", "expected an ERA5 native pressure subset")
    times, levels = list(map(instant, data["validTimes"])), data["pressureHPa"]
    height, width = len(data["latitudes"]), len(data["longitudes"])
    require(time_at in times and 0 <= row < height and 0 <= col < width, "selected time or native cell outside the retained pressure field")
    variable = data["variables"]["temperature"]
    expected = len(times)*len(levels)*height*width
    require(len(variable["samples"]) == len(data["aboveGroundMask"]) == expected and available >= time_at, "invalid pressure-grid dimensions or capture date")
    status, value = "no-coverage", None
    if pressure in levels:
        index = ((times.index(time_at)*len(levels)+levels.index(pressure))*height+row)*width+col
        status = "available" if data["aboveGroundMask"][index] else "quality-rejected"
        if status == "available":
            value = number(variable["samples"][index])
    identity = "era5-" + sha256(path.read_bytes()).hexdigest()[:12]
    entry = {"id": f"era5-{time_at:%Y%m%dT%H%M}-{pressure}-{row}-{col}", "sourceId": identity, "kind": "reanalysis", "lane": "debrief",
             "variable": "Air temperature", "area": "Central Saudi ERA5 study area", "support": "Native retained pressure grid",
             "nativeRef": f"row-{row} col-{col} {pressure} hPa", "method": "native", "reference": f"{pressure} hPa pressure level",
             "status": status, "units": text(variable["units"]), "value": value, "samples": int(value is not None), "coverage": None,
             "validFrom": utc(time_at), "availableAt": utc(available)}
    return {"sources": [source(path, identity, data["edition"], "Copernicus ERA5 distributed via WeatherBench 2")], "fields": [entry]}


def unhcr(path, year, origin, asylum, capture):
    doc = read(path)
    require(doc["page"] == doc["maxPages"] == 1 and len(doc["items"]) == 1, "expected one complete UNHCR origin/asylum result")
    row = doc["items"][0]
    require((row["year"], row["coo_iso"], row["coa_iso"]) == (year, origin, asylum), "unexpected UNHCR year or country pair")
    identity = f"unhcr-{year}"
    value = row["refugees"]
    require(type(value) is int and value >= 0, "expected the published refugee count")
    entry = {"id": f"{identity}-{origin}-{asylum}", "sourceId": identity, "upstream": "UNHCR Refugee Data Finder | refugees",
             "area": row["coa_name"], "level": "country", "origin": origin, "asylum": asylum, "kind": "stock",
             "definition": "Refugees by origin and asylum at year end", "method": "UNHCR annual population statistics",
             "boundary": asylum, "periodGrain": "year", "from": f"{year}-01-01", "toExclusive": f"{year+1}-01-01",
             "units": "people", "status": "available", "value": value, "availableAt": utc(capture)}
    return {"sources": [source(path, identity, str(year), "UNHCR Refugee Data Finder")], "indicators": [entry]}


def cap(files, captures, area):
    origins, groups, rows = {}, {}, []
    require(len(files) == len(captures) and 0 < len(files) <= 20, "supply one capture per CAP message in issuer order")
    sources = [source(path, f"cap-{index+1}", "CAP 1.2 retained notice", "Issuer in CAP sender field") for index,path in enumerate(files)]
    # The same source identity is used for revisions; individual originals remain pinned above.
    body_hash = sha256("".join(row["sha256"] for row in sources).encode()).hexdigest()
    sources.append({"id": "cap-issuer", "edition": "CAP 1.2 preserved revision sequence", "attribution": "Issuer in retained CAP notices", "sha256": body_hash})
    for index, (file, capture) in enumerate(zip(files, captures)):
        raw = file.read_bytes()
        require(len(raw) <= 262144, "notice exceeds CAP byte budget")
        root = ET.fromstring(raw)
        require(root.tag == "{urn:oasis:names:tc:emergency:cap:1.2}alert", "expected CAP 1.2")
        ident, sender, sent = (text(root.findtext(f"{{*}}{key}")) for key in ("identifier", "sender", "sent"))
        msg_type = text(root.findtext("{*}msgType"))
        require((sender,ident,sent) not in origins and msg_type in {"Alert", "Update", "Cancel"}, "invalid or repeated CAP message")
        at = instant(sent)
        links = (root.findtext("{*}references") or "").split()
        if msg_type == "Alert":
            require(not links, "a new CAP notice must begin its own sequence")
            origin = "cap:" + sha256(f"{sender}:{ident}".encode()).hexdigest()[:24]
        else:
            referenced = [tuple(link.split(",")) for link in links]
            require(referenced and all(len(key) == 3 and key[0] == sender and key in origins and instant(key[2]) <= at for key in referenced),
                    "CAP update needs its earlier issuer references")
            keys = {origins[key] for key in referenced}
            require(len(keys) == 1, "CAP message spans unrelated sequences")
            origin = keys.pop()
        origins[sender,ident,sent] = origin
        groups[origin] = groups.get(origin, 0)+1
        info = root.findall("{*}info")
        languages = {}
        windows = []
        for node in info:
            lang = node.findtext("{*}language", "en-US")
            require(lang not in languages, "CAP record repeats a language block")
            languages[lang] = text(node.findtext("{*}headline") or node.findtext("{*}event"))
            effective, expires = node.findtext("{*}effective"), node.findtext("{*}expires")
            windows.append((instant(effective) if effective else at, instant(expires) if expires else None))
        require(len(set(windows)) <= 1, "translated CAP blocks disagree about validity")
        row = event(origin, "cap-issuer", origin, "public-notice", groups[origin], at, capture,
                    "cancelled" if msg_type == "Cancel" else "published", f"Civil public notice ({msg_type})", area,
                    languages=languages, details=f"Issuer {sender}; identifier {ident}; original cap-{index+1}")
        if windows:
            start, end = windows[0]
            row["effectiveAt"] = utc(start)
            if end is not None:
                row["expiresAt"] = utc(end)
        rows.append(row)
    return {"sources": sources, "events": rows}


def regional_column(path, row):
    """Aggregate reviewed pixels from one versioned regional satellite product."""
    data = read(path)
    target = number(data["reviewAreaM2"])
    minimum = number(data["minQuality"])
    require(target > 0 and 0 <= minimum <= 1 and len(data["pixels"]) <= 50000, "invalid column study area or quality limit")
    seen = set()
    total, accepted, weighted, count = 0, 0, 0, 0
    for pixel in data["pixels"]:
        identity, weight, quality = text(pixel["id"]), number(pixel["areaM2"]), number(pixel["quality"])
        require(identity not in seen and weight > 0 and 0 <= quality <= 1, "invalid or repeated native pixel")
        seen.add(identity)
        total += weight
        if quality > minimum and pixel["value"] is not None:
            weighted += weight * number(pixel["value"])
            accepted += weight
            count += 1
    require(total <= target, "pixel overlaps exceed the declared review area")
    value = weighted/accepted if count else None
    status = "available" if count else "quality-rejected" if seen else "no-coverage"
    return {**row, "kind": "regional-column", "lane": "debrief", "method": "area-summary", "status": status,
            "samples": count, "coverage": accepted/target, "value": value}


def runway_weather(path, area, station, capture):
    data = read(path)
    require(isinstance(data, list) and 0 < len(data) <= 128, "expected a bounded METAR archive")
    fields = []
    seen = set()
    for report in data:
        raw_time = report["obsTime"]
        require(report["icaoId"] == station and type(raw_time) is int and raw_time not in seen, "different station or duplicate METAR hour")
        seen.add(raw_time)
        at = datetime.fromtimestamp(raw_time, timezone.utc)
        require(at <= capture, "METAR observation follows the archive capture")
        value = report.get("temp")
        if value is not None:
            number(value)
        fields.append({"id": f"metar-{station}-{raw_time}", "sourceId": "awc-metar", "kind": "station-observation", "lane": "decision",
                       "variable": "Station air temperature", "area": area, "support": f"{station} station",
                       "nativeRef": station, "method": "reported", "reference": "METAR station observation", "units": "°C",
                       "status": "available" if value is not None else "no-coverage", "samples": int(value is not None),
                       "coverage": None, "value": value, "validFrom": utc(at), "availableAt": utc(capture)})
    return {"sources": [source(path, "awc-metar", "Retained AWC METAR JSON", "NOAA Aviation Weather Center")], "fields": fields}


def terminal_forecast(path, area, station, capture):
    data = read(path)
    require(isinstance(data, list) and len(data) == 1 and data[0]["icaoId"] == station, "expected one archived TAF at the requested station")
    report = data[0]
    issue = instant(report["issueTime"])
    require(issue <= capture, "TAF follows the archive capture")

    def at(value):
        require(type(value) is int, "TAF validity needs a provider epoch")
        return datetime.fromtimestamp(value, timezone.utc)

    fields = []
    for index, group in enumerate(report["fcsts"]):
        start, end = at(group["timeFrom"]), at(group["timeTo"])
        value = group.get("visib")
        require(issue <= start < end and (value is None or isinstance(value, (str, int, float))), "invalid TAF time group")
        fields.append({"id": f"taf-{station}-{index}", "sourceId": "awc-taf", "kind": "station-forecast", "lane": "decision",
                       "variable": "Visibility category", "area": area, "support": f"{station} station",
                       "nativeRef": f"{station} forecast group {index+1}", "method": "reported",
                       "reference": f"TAF encoded visibility {value}" if value is not None else "TAF visibility unavailable",
                       "units": "provider category", "status": "reported" if value is not None else "no-coverage", "value": None,
                       "samples": None, "coverage": None, "initializedAt": utc(issue), "availableAt": utc(capture),
                       "validFrom": utc(start), "validTo": utc(end)})
    return {"sources": [source(path, "awc-taf", "Retained AWC TAF JSON", "NOAA Aviation Weather Center")], "fields": fields}


def gdacs(path, identity, area, capture):
    doc = read(path)
    candidates = doc["features"] if doc.get("type") == "FeatureCollection" else [doc]
    items = [item["properties"] for item in candidates if str(item.get("properties", {}).get("eventid")) == identity]
    require(items and len(items) <= 40, "missing or unbounded GDACS event records")
    out = []
    seen = set()
    for props in items:
        start = text(props["fromdate"])
        at = instant(start if start.endswith("Z") or "+" in start[10:] else start+"Z")
        episode = str(props["episodeid"])
        family = props["eventtype"]
        require(episode.isdigit() and episode not in seen and family in {"EQ", "FL"}, "invalid or duplicate GDACS bulletin episode")
        seen.add(episode)
        origin = f"gdacs:{family}:{identity}"
        out.append(event(f"{origin}:{episode}", "gdacs", origin, "disaster-bulletin", 1, at, capture, "estimated",
                         f"GDACS {text(props['alertlevel'])} bulletin", area, details=f"Issuer estimate · episode {episode}"))
    return {"sources": [source(path, "gdacs", "GDACS retained bulletin JSON", "GDACS")], "events": out}


def reviewed_indicators(path, spec):
    """Read a steward-reviewed HAPI or DTM extract alongside its coverage decision."""
    receipt = read(spec)
    edition, provider = text(receipt["edition"]), receipt["provider"]
    require(provider in {"hdx-hapi", "iom-dtm"}, "unsupported humanitarian provider")
    identity = text(receipt["sourceId"])
    data = read(path)
    require(data.get("status") in {"complete", "source-error"} and isinstance(data["rows"], list), "invalid reviewed humanitarian extract")
    seen, rows = set(), []
    for scope in receipt["scopes"]:
        key = text(scope["id"])
        found = [row for row in data["rows"] if row["scopeId"] == key]
        require(key not in seen and len(found) <= 1 and (data["status"] != "source-error" or not found), "invalid coverage key or duplicate measurement")
        seen.add(key)
        value = found[0]["value"] if found else None
        if value is not None:
            require(number(value) >= 0, "invalid humanitarian indicator value")
        status = "source-error" if data["status"] == "source-error" else "available" if value is not None else "no-coverage"
        rows.append({**scope, "sourceId": identity, "value": value, "status": status})
    require(all(row["scopeId"] in seen for row in data["rows"]), "extract includes an unreviewed reporting scope")
    return {"sources": [source(path, identity, edition, provider), source(spec, identity+"-scope", edition, "Reviewed indicator definitions and coverage")], "indicators": rows}


def thermal_overpasses(path, area):
    data = read(path)
    require(data["product"] == "NOAA-20 VIIRS" and 0 < len(data["windows"]) <= 60, "expected a bounded reviewed NOAA-20 FIRMS subset")
    rows = []
    for overpass in data["windows"]:
        key = text(overpass["id"])
        status = overpass["status"]
        require(status in {"observed", "missing", "source-error"}, "unknown thermal overpass coverage")
        extent = text(overpass["reviewedExtent"])
        detail = f"{data['product']} · {text(data['archiveType'])} · {extent}"
        if status == "observed":
            require(0 < number(overpass["pixelM"]) <= 5000, "invalid thermal native pixel support")
            detail += f" · {overpass['pixelM']} m pixel · {text(overpass['confidence'])} confidence"
        else:
            detail += f" · {text(overpass['reason'])}"
        rows.append(event(key, "firms-reviewed", "firms:"+key, "thermal-sample", 1, instant(overpass["at"]),
                          instant(overpass["availableAt"]), status, "Reviewed thermal overpass", area, details=detail))
    return {"sources": [source(path, "firms-reviewed", text(data["archiveType"]), "NASA FIRMS NOAA-20 VIIRS")], "events": rows}


def water_tracks(path, area):
    data = read(path)
    require(data["product"] == "ATL13" and 0 < len(data["samples"]) <= 150, "expected reviewed ATL13 inland water rows")
    rows = []
    for track in data["samples"]:
        status = track["status"]
        require(status in {"available", "no-coverage", "quality-rejected"}, "unknown along-track status")
        value = track.get("heightM")
        qa = text(track["qualityBasis"])
        require(status != "available" or (value is not None and track.get("qualityPassed") is True), "water height requires a passed quality review")
        rows.append({"id": text(track["id"]), "sourceId": "atl13-reviewed", "kind": "along-track", "lane": "debrief", "variable": "Water surface height",
                     "area": area, "support": "Reviewed inland water body", "nativeRef": f"{text(track['nativeRef'])} · {qa}", "method": "native",
                     "reference": text(track["heightReference"]), "units": "m", "value": number(value) if status == "available" else None,
                     "status": status, "samples": track.get("samples"), "coverage": None, "validFrom": utc(instant(track["at"])),
                     "availableAt": utc(instant(track["availableAt"]))})
    return {"sources": [source(path, "atl13-reviewed", text(data["edition"]), "NASA ICESat-2 ATL13 via NSIDC")], "fields": rows}


def assemble(definition, fragments):
    body = read(definition)
    require(body["schema"] == "emad.supporting-source-review.v1", "expected supporting exercise definition")
    source_list = {item["id"]: item for item in body.get("sources", [])}
    require(len(source_list) == len(body.get("sources", [])), "repeated source pin")
    for path in fragments:
        data = read(path)
        for row in data["sources"]:
            require(row["id"] not in source_list or row == source_list[row["id"]], "fragment changes a retained source")
            source_list[row["id"]] = row
        for key in ("events", "fields", "indicators", "comparisons"):
            if key in data:
                body[key] = body.get(key, []) + data[key]
    body["sources"] = list(source_list.values())
    return body


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    quake = commands.add_parser("usgs")
    quake.add_argument("input", type=Path)
    quake.add_argument("--area", required=True)
    quake.add_argument("--revision", type=int, default=1)
    eon = commands.add_parser("eonet")
    eon.add_argument("input", type=Path)
    eon.add_argument("--id", required=True)
    eon.add_argument("--area", required=True)
    eon.add_argument("--capture", type=instant, required=True)
    weather = commands.add_parser("meteo")
    weather.add_argument("input", type=Path)
    for field in ("identity", "edition", "attribution", "variable", "label", "area", "native"):
        weather.add_argument("--" + field, required=True)
    weather.add_argument("--kind", choices=["forecast-run", "historical-estimate", "dust-model", "marine-model"], required=True)
    weather.add_argument("--available", type=instant, required=True)
    weather.add_argument("--at", type=instant, required=True)
    weather.add_argument("--init", type=instant)
    era = commands.add_parser("era5")
    era.add_argument("input", type=Path)
    era.add_argument("--at", type=instant, required=True)
    era.add_argument("--available", type=instant, required=True)
    for field in ("pressure", "row", "col"):
        era.add_argument("--" + field, type=int, required=True)
    population = commands.add_parser("unhcr")
    population.add_argument("input", type=Path)
    population.add_argument("--year", required=True, type=int)
    population.add_argument("--origin", required=True)
    population.add_argument("--asylum", required=True)
    population.add_argument("--capture", required=True, type=instant)
    cap_cmd = commands.add_parser("cap")
    cap_cmd.add_argument("files", type=Path, nargs="+")
    cap_cmd.add_argument("--capture", required=True, type=instant, action="append")
    cap_cmd.add_argument("--area", required=True)
    column = commands.add_parser("regional-column")
    column.add_argument("input", type=Path)
    column.add_argument("definition", type=Path)
    for product in ("metar", "taf"):
        aviation = commands.add_parser(product)
        aviation.add_argument("input", type=Path)
        aviation.add_argument("--area", required=True)
        aviation.add_argument("--station", required=True)
        aviation.add_argument("--capture", required=True, type=instant)
    bulletins = commands.add_parser("gdacs")
    bulletins.add_argument("input", type=Path)
    bulletins.add_argument("--area", required=True)
    bulletins.add_argument("--id", required=True)
    bulletins.add_argument("--capture", required=True, type=instant)
    indicators = commands.add_parser("reviewed-indicators")
    indicators.add_argument("input", type=Path)
    indicators.add_argument("definition", type=Path)
    for action in ("thermal-overpasses", "water-tracks"):
        archive = commands.add_parser(action)
        archive.add_argument("input", type=Path)
        archive.add_argument("--area", required=True)
    bundle = commands.add_parser("assemble")
    bundle.add_argument("input", type=Path)
    bundle.add_argument("fragments", type=Path, nargs="+")
    args = parser.parse_args()
    handlers = {
        "usgs": lambda: usgs(args.input, args.area, args.revision),
        "eonet": lambda: eonet(args.input, args.id, args.area, args.capture),
        "meteo": lambda: meteo(args.input, args.identity, args.edition, args.attribution, args.variable, args.label, args.area,
                                args.native, args.kind, args.available, args.init, args.at),
        "era5": lambda: era5(args.input, args.at, args.available, args.pressure, args.row, args.col),
        "unhcr": lambda: unhcr(args.input, args.year, args.origin, args.asylum, args.capture),
        "cap": lambda: cap(args.files, args.capture, args.area),
        "regional-column": lambda: {"sources": [source(args.input, "s5p-reviewed", "Reviewed native pixel extract", "Copernicus Sentinel-5P"),
                                               source(args.definition, "s5p-review-scope", "Reviewed regional column definition", "Exercise data steward")],
                                     "fields": [regional_column(args.input, {**read(args.definition), "sourceId": "s5p-reviewed"})]},
        "metar": lambda: runway_weather(args.input, args.area, args.station, args.capture),
        "taf": lambda: terminal_forecast(args.input, args.area, args.station, args.capture),
        "gdacs": lambda: gdacs(args.input, args.id, args.area, args.capture),
        "reviewed-indicators": lambda: reviewed_indicators(args.input, args.definition),
        "thermal-overpasses": lambda: thermal_overpasses(args.input, args.area),
        "water-tracks": lambda: water_tracks(args.input, args.area),
        "assemble": lambda: assemble(args.input, args.fragments),
    }
    print(json.dumps(handlers[args.command](), ensure_ascii=True, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, IndexError, KeyError, AttributeError, ET.ParseError) as err:
        sys.exit(f"Supporting source refused: {err}")
