"""Normalize retained civilian regional statistics and historical coastal inputs."""

import argparse
from pathlib import Path, PurePosixPath
import json
import re
import sys
from xml.etree import ElementTree as ET
from zipfile import ZipFile

from prepare_civilian_access import file_member_sha, source
from simulation_inputs import day, instant, require, utc


def indicator(source_id, key, region_code, region, geography, measure, native, unit, scope, channel,
              year, available, value):
    return {"sourceId": source_id, "id": key, "regionCode": region_code, "region": region, "geography": geography,
            "measure": measure, "nativeLabel": native, "unit": unit, "populationScope": scope,
            "channel": channel, "periodFrom": f"{year}-01-01", "periodToExclusive": f"{year+1}-01-01",
            "availableAt": utc(available), "value": value}


def srca_release(path, region):
    raw = path.read_text(encoding="utf-8-sig")
    rows, checks = [], []
    if region == "eastern":
        key, edition = "srca-east", "2025 Eastern Province release, Arabic edition"
        require("red-crescent-eastern-region-emergency-calls-2025" in raw, "expected the 2025 SRCA Eastern article")
        measures = [
            ("calls", "Medical Relocation Centre calls", "إجمالي المكالمات الواردة", "calls", "All centre contacts", "Medical Relocation Centre"),
            ("emergency", "Emergency reports", "بلاغات إسعافية", "reports", "Emergency reports requiring service", "Medical Relocation Centre"),
            ("app", "Asefni emergency reports", "بلاغات تطبيق أسعفني", "reports", "App reports, overlap unknown", "Asefni application"),
            ("field", "Cases handled in the field", "الحالات الإسعافية التي جرى التعامل معها ميدانيًا", "cases", "Cases handled in the field", "SRCA field services"),
            ("transfers", "Hospital transfers", "نقل إلى المستشفيات", "cases", "Cases handled in the field", "SRCA field services"),
            ("scene", "Care at scene", "الرعاية الطبية الإسعافية في موقع البلاغ", "cases", "Cases handled in the field", "SRCA field services"),
        ]
        available = instant("2026-01-08T00:00:00Z")
        patterns = {
            "calls": r"مركز الترحيل الطبي بلغ ([\d,]+) مكالمة", "emergency": r"من بينها ([\d,]+) بلاغًا إسعافيًا",
            "app": r"أسعفني» خلال العام ذاته ([\d,]+) بلاغات", "field": r"ميدانيًا خلال عام 2025م بلغ ([\d,]+) حالة",
            "transfers": r"ونقل ([\d,]+) حالة إلى المستشفيات", "scene": r"موقع البلاغ لـ ([\d,]+) حالة",
        }
        for suffix, measure, native, unit, scope, channel in measures:
            matches = set(re.findall(patterns[suffix], raw))
            require(len(matches) == 1, f"missing or ambiguous Eastern article count: {suffix}")
            value = int(matches.pop().replace(",", ""))
            rows.append(indicator(key, f"{key}-{suffix}", key, "Eastern Province", "SRCA reporting region", measure,
                                  native, unit, scope, channel, 2025, available, value))
        checks.append({"totalId": f"{key}-field", "componentIds": [f"{key}-transfers", f"{key}-scene"]})
    else:
        key, edition = "srca-asir", "2024 Asir annual statistics, SPA N2236149"
        # The official article uses periods as thousands separators.
        measures = [
            ("asir", "Asir", "Emergency reports handled", "بلاغات إسعافية", r"بمنطقة عسير\s+(\d{2}\.\d{3})\s*بلاغات"),
            ("khamis", "Khamis Mushait sector", "Emergency reports handled", "قطاع خميس مشيط الإسعافي", r"خميس مشيط الإسعافي.*?بواقع\s+(\d{2}\.\d{3})"),
            ("abha", "Abha sector", "Emergency reports handled", "قطاع أبها الإسعافي", r"أبها الإسعافي بعدد\s+(\d{2}\.\d{3})"),
            ("tihamah", "Tihamah sector", "Emergency reports handled", "قطاع تهامة", r"تهامة بـ\s*(\d\.\d{3})"),
        ]
        for suffix, locality, measure, native, pattern in measures:
            matches = set(re.findall(pattern, raw, flags=re.DOTALL))
            require(len(matches) == 1, f"missing or ambiguous Asir article count: {suffix}")
            value = int(matches.pop().replace(".", ""))
            rows.append(indicator(key, f"{key}-{suffix}", f"{key}-{suffix}", locality,
                                  "SRCA reporting region" if suffix == "asir" else "Named SRCA sector",
                                  measure, native, "reports", "Emergency reports handled", "SRCA field services via SPA",
                                  2024, instant("2025-01-02T00:00:00Z"), value))
    attribution = "Saudi Red Crescent Authority" if region == "eastern" else "Saudi Red Crescent Authority via Saudi Press Agency"
    return {"sources": [source(path, key, edition, attribution)], "indicators": rows, "reconciliations": checks}


def moh_beds(path):
    key = "moh-er-beds"
    with ZipFile(path) as workbook:
        def xml(member):
            require(workbook.getinfo(member).file_size < 12_000_000, "workbook member exceeds its input budget")
            return ET.fromstring(workbook.read(member))

        tables = [sheet for sheet in xml("xl/workbook.xml").findall(".//{*}sheet") if sheet.get("name") == "21"]
        require(len(tables) == 1, "missing or ambiguous Table 2-21 worksheet")
        sheet_id = next(value for field, value in tables[0].attrib.items() if field.endswith("}id"))
        relations = {item.get("Id"): item.get("Target") for item in xml("xl/_rels/workbook.xml.rels")}
        target = PurePosixPath(relations[sheet_id])
        member = str(target).removeprefix("/") if target.is_absolute() else str(PurePosixPath("xl") / target)
        require(re.fullmatch(r"xl/worksheets/sheet\d+\.xml", member), "unexpected worksheet path")
        strings = ["".join(t.text or "" for t in item.findall(".//{*}t")) for item in xml("xl/sharedStrings.xml")]
        cells = {}
        for cell in xml(member).findall(".//{*}c"):
            value = cell.findtext("{*}v")
            if value is not None:
                cells[cell.get("r")] = strings[int(value)].strip() if cell.get("t") == "s" else value
    require(cells["A2"] == "ER Beds at MoH Hospitals By Health Region, 2023G" and cells["B3"] == "Table 2-21" and
            cells["B5"] == "Beds with Monitors" and cells["C5"] == "Beds without Monitors" and cells["D5"] == "Total",
            "worksheet layout changed")
    results, checks = [], []
    seen = set()
    columns = {"B": ("monitored", "ER beds with monitors"), "C": ("unmonitored", "ER beds without monitors"), "D": ("total", "Total ER beds")}
    for index in range(6, 27):
        label = cells[f"E{index}"]
        region = "moh-" + re.sub(r"[^a-z0-9]+", "-", label.lower()).strip("-")
        require(region not in seen, "duplicate MOH geography")
        seen.add(region)
        for column, (suffix, measure) in columns.items():
            count = cells.get(f"{column}{index}")
            require(count is None or count.isdigit(), "invalid annual bed count")
            results.append(indicator(key, f"{region}-{suffix}", region, label,
                                     "MOH national total" if label == "Total" else "MOH health region", measure,
                                     f"{cells[f'{column}4']} · {cells[f'A{index}']}", "beds", "MOH hospital emergency departments",
                                     "MOH 2023 Statistical Yearbook, Table 2-21", 2023, instant("2025-02-26T00:00:00Z"),
                                     int(count) if count is not None else None))
        checks.append({"totalId": f"{region}-total", "componentIds": [f"{region}-monitored", f"{region}-unmonitored"]})
    return {"sources": [source(path, key, "2023 Statistical Yearbook, worksheet 21", "Saudi Ministry of Health")],
            "indicators": results, "reconciliations": checks}


def reception_coverage(path, identity, edition, attribution):
    doc = json.loads(path.read_bytes())
    rows = []
    seen = set()
    for record in doc["days"]:
        at = day(record["day"])
        available = instant(record["availableAt"])
        start, end = record["window"]
        require(at not in seen and available.date() > at and all(type(v) is int for v in [start, end]) and
                0 <= start < end <= 1440, "invalid retrospective civil reception day or window")
        seen.add(at)
        minutes = set()
        for low, high in record["connected"]:
            require(type(low) is int and type(high) is int and start <= low < high <= end, "invalid connected interval")
            minutes.update(range(low, high))
        ids = set()
        duplicates = 0
        for packet in record["receipts"]:
            require(type(packet["minute"]) is int and packet["minute"] in minutes and
                    re.fullmatch(r"[a-f0-9]{64}", packet["digest"]), "invalid or out-of-coverage receipt checksum")
            if packet["digest"] in ids:
                duplicates += 1
            ids.add(packet["digest"])
        rows.append({"sourceId": identity, "day": at.isoformat(), "availableAt": utc(available),
                     "windowMinutes": end-start, "usableMinutes": len(minutes),
                     "uniqueReports": len(ids) if minutes else None, "duplicates": duplicates if minutes else None,
                     "portWatchCalls": None})
    require(len(rows) <= 180, "reception input exceeds the day budget")
    return {"sources": [source(path, identity, edition, attribution)], "coast": {"coverage": rows}}


def coastal_depth(path, shore, manifest, selected):
    from environment_reader import Reader

    reader = Reader(path, manifest)
    require(reader.manifest["profileId"] == "coastal-context-v1", "expected the pinned coastal environment profile")
    entry = reader.member("coastal-grid.json")
    with ZipFile(path) as prepared:
        require(file_member_sha(prepared, entry["name"]) == entry["sha256"], "coastal grid differs from release manifest")
        payload = json.loads(prepared.read(entry["name"]))
    grid, info = payload["grid"], payload["summary"]
    grid_id, shoreline_id = "gebco-jubail", "natural-earth-shoreline"
    x0, y0, x1, y1 = grid["window"]
    width, height = x1-x0+1, y1-y0+1
    require(width * height == len(grid["elevation"]) == len(grid["tid"]) and
            0 < len(selected) <= 32 and entry["details"]["crs"] == "EPSG:4326", "invalid retained coastal grid")
    shoreline = source(shore, shoreline_id, info["coastVersion"], "Natural Earth coastline")
    require(shoreline["sha256"] == info["coastSha256"], "shoreline differs from the release")
    result = []
    for point in selected:
        row, col = (int(value) for value in point.split(","))
        require(0 <= row < height and 0 <= col < width, "selected cell is outside the retained coastal grid")
        value, tid = grid["elevation"][row*width+col], grid["tid"][row*width+col]
        require(value is None or tid == 0 or value < 0, "nonnegative water cell needs independent review")
        result.append({"cell": f"row-{row}:col-{col}", "state": "nodata" if value is None else "land" if tid == 0 else "offshore",
                       "elevationM": value, "sourceType": "Source type unavailable" if tid is None else f"GEBCO TID {tid}"})
    return {"sources": [source(path, grid_id, grid["edition"], "GEBCO paired elevation and type-identification grids"), shoreline],
            "coast": {"retainedManifestSha256": reader.manifest_sha,
                      "depth": {"gridSourceId": grid_id, "shorelineSourceId": shoreline_id,
                                "horizontalReference": f"EPSG:4326 {grid['registration']}",
                                "verticalReference": grid["datum"], "samples": result}}}


def attach_port_calls(path, activity, port, available):
    context, page = (json.loads(item.read_bytes()) for item in (path, activity))
    require(port in {"port24", "port2074"} and "error" not in page and not page.get("exceededTransferLimit"),
            "expected a complete single-port daily export")
    counts = {}
    for feature in page["features"]:
        row = feature["attributes"]
        date = day(f"{row['year']:04d}-{row['month']:02d}-{row['day']:02d}").isoformat()
        count = row["portcalls"]
        require(row["portid"] == port and date not in counts and type(count) is int and count >= 0,
                "invalid, repeated or mixed port day")
        counts[date] = count
    identity = "imf-portwatch-" + port
    context["sources"].append(source(activity, identity, "daily-ports-v1, retained retrospective export", "IMF PortWatch"))
    require(context["coast"].get("portWatchId", port) == port, "port activity differs from the reviewed coastal area")
    context["coast"]["portWatchId"] = port
    for row in context["coast"]["coverage"]:
        if row["day"] in counts:
            require(available.date() > day(row["day"]), "port file time precedes the completed day")
            row.update(portWatchCalls=counts[row["day"]], portWatchSourceId=identity,
                       portWatchAvailableAt=utc(available))
    return context


def assemble(path, fragments):
    body = json.loads(path.read_bytes())
    require(body["schema"] == "emad.regional-response-review.v1", "expected a regional review definition")
    by_id = {entry["id"]: entry for entry in body.get("sources", [])}
    require(len(by_id) == len(body.get("sources", [])), "duplicate source identity")
    for fragment in fragments:
        data = json.loads(fragment.read_bytes())
        for entry in data["sources"]:
            identity = entry["id"]
            require(identity not in by_id or by_id[identity] == entry, "fragment changes a retained source")
            by_id[identity] = entry
        for field in ("indicators", "reconciliations"):
            if field in data:
                body[field] = body.get(field, []) + data[field]
        if "coast" in data:
            require("coast" in body, "supply a coastal definition before its evidence")
            for field, value in data["coast"].items():
                current = body["coast"].get(field)
                if field in {"coverage", "notices"}:
                    body["coast"][field] = (current or []) + value
                else:
                    require(current is None or current == value, "fragment replaces a reviewed geographic reference")
                    body["coast"][field] = value
    body["sources"] = list(by_id.values())
    return body


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    srca = sub.add_parser("srca")
    srca.add_argument("input", type=Path)
    srca.add_argument("--region", choices=["eastern", "asir"], required=True)
    sub.add_parser("moh-beds").add_argument("input", type=Path)
    reception = sub.add_parser("reception")
    reception.add_argument("input", type=Path)
    reception.add_argument("--source-id", required=True)
    reception.add_argument("--edition", required=True)
    reception.add_argument("--attribution", required=True)
    depth = sub.add_parser("coastal-depth")
    depth.add_argument("input", type=Path)
    depth.add_argument("shoreline", type=Path)
    depth.add_argument("--manifest-sha256", required=True)
    depth.add_argument("--cell", action="append", required=True, help="local row,column in the prepared grid")
    port = sub.add_parser("attach-portwatch")
    port.add_argument("input", type=Path, help="reviewed reception fragment")
    port.add_argument("activity", type=Path)
    port.add_argument("--port", required=True)
    port.add_argument("--available-at", type=instant, required=True)
    bundle = sub.add_parser("assemble")
    bundle.add_argument("input", type=Path)
    bundle.add_argument("fragments", type=Path, nargs="+")
    args = parser.parse_args()
    handlers = {"srca": lambda: srca_release(args.input, args.region), "moh-beds": lambda: moh_beds(args.input),
                "reception": lambda: reception_coverage(args.input, args.source_id, args.edition, args.attribution),
                "coastal-depth": lambda: coastal_depth(args.input, args.shoreline, args.manifest_sha256, args.cell),
                "attach-portwatch": lambda: attach_port_calls(args.input, args.activity, args.port, args.available_at),
                "assemble": lambda: assemble(args.input, args.fragments)}
    print(json.dumps(handlers[args.command](), sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, KeyError, IndexError, AttributeError, StopIteration, ET.ParseError) as err:
        sys.exit(f"Regional input refused: {err}")
