"""Offline readers and analyses for reviewed civilian simulation inputs."""

from collections import Counter, defaultdict
import csv
from datetime import date, datetime, timedelta, timezone
from hashlib import sha256
import heapq
import io
import json
import math
from statistics import median


MUNICIPAL_FIELDS = {
    "category": "نوع البلاغ",
    "place": "الموقع",
    "status": "حالة البلاغ",
    "closure": "زمن الإغلاق",
    "receipt": "تاريخ الاستلام",
    "repeat": "تكرار",
}


def require(ok, message):
    if not ok:
        raise ValueError(message)


def instant(value):
    require(isinstance(value, str) and "T" in value, "expected a timestamp with a UTC offset")
    parsed = datetime.fromisoformat(value)
    require(parsed.utcoffset() is not None, "timestamp needs a UTC offset")
    return parsed.astimezone(timezone.utc)


def utc(value):
    return value.isoformat().replace("+00:00", "Z")


def day(value):
    parsed = date.fromisoformat(value)
    require(parsed.isoformat() == value, "expected YYYY-MM-DD")
    return parsed


def text(value):
    require(isinstance(value, str) and bool(value.strip()), "expected a nonempty text field")
    return value


def number(value):
    require(type(value) in (int, float) and math.isfinite(value), "expected a finite number")
    return value


def municipal(raw, include_rows=False):
    """Read the published six-column JSON as a retrospective workload."""
    document = json.loads(raw)
    require(isinstance(document, list), "municipal JSON must contain a list of rows")
    digest = sha256(raw).hexdigest()
    counts = Counter()
    days = Counter()
    unresolved = 0
    rows = []
    for ordinal, row in enumerate(document, 1):
        require(isinstance(row, dict) and set(row) == set(MUNICIPAL_FIELDS.values()),
                f"unexpected municipal columns in row {ordinal}")
        category, place, status, closure, receipt, repeat = (row[k] for k in MUNICIPAL_FIELDS.values())
        key = " ".join(text(category).split())
        require(type(receipt) is int and type(closure) is int, f"invalid numbers in row {ordinal}")
        at = datetime(1970, 1, 1, tzinfo=timezone.utc) + timedelta(milliseconds=receipt)
        label = place if isinstance(place, str) and place.strip() else None
        status_key = text(status).strip()
        repeat_key = text(repeat).strip()
        counts[key] += 1
        days[at.date().isoformat()] += 1
        unresolved += label is None
        if include_rows:
            rows.append({
                "id": f"{digest}:{ordinal}",
                "receiptAt": utc(at),
                "categoryKey": key,
                "reportedPlaceLabel": label,
                "locationResolution": "unresolved",
                "snapshot": {
                    "status": {"تم التنفيذ": "completed", "جاري العمل": "in-progress"}.get(status_key),
                    "closureValue": closure,
                    "closureUnit": None,
                    "repeatFlag": {"مكرر": True, "غير مكرر": False}.get(repeat_key),
                },
                "raw": row,
            })
    output = {
        "editionSha256": digest, "rowCount": len(document), "categoryCounts": dict(counts),
        "utcDayCounts": dict(days), "rowsWithoutPlaceLabel": unresolved,
        "temporalMeaning": "retrospective-export", "availabilityAt": None,
    }
    if include_rows:
        output["rows"] = rows
    return output


def visible_revision_key(row):
    return instant(row["effectiveAt"]), instant(row["availableAt"]), row["revision"]


def road_access(document, cutoff, origin):
    """Compare distances through a reviewed directed civilian road graph."""
    text(document["graphEdition"])
    nodes = {text(node) for node in document["nodes"]}
    require(len(nodes) == len(document["nodes"]) and origin in nodes, "invalid graph nodes or origin")
    edges = {}
    for edge in document["edges"]:
        identifier = text(edge["id"])
        require(identifier not in edges and edge["from"] in nodes and edge["to"] in nodes,
                f"duplicate edge or unknown endpoint: {identifier}")
        require(number(edge["distanceM"]) > 0, f"invalid distance: {identifier}")
        edges[identifier] = edge

    active = {}
    seen = set()
    for assessment in document["inspections"]:
        identifier = text(assessment["edgeId"])
        effective, available, revision = visible_revision_key(assessment)
        verdict = assessment["access"]
        evidence = text(assessment["evidenceId"])
        require(type(revision) is int and revision >= 1, "invalid inspection revision")
        require(effective <= available, "inspection availability precedes inspection")
        require(identifier in edges and verdict in {"open", "closed", "unknown"},
                "invalid inspected edge or access")
        require((identifier, effective, available, revision) not in seen, "ambiguous inspection revision")
        seen.add((identifier, effective, available, revision))
        if effective > cutoff or available > cutoff:
            continue
        entry = ((effective, available, revision), verdict, evidence)
        if identifier not in active or entry[0] > active[identifier][0]:
            active[identifier] = entry

    admissible, possible = defaultdict(list), defaultdict(list)
    access = {}
    for identifier, edge in edges.items():
        entry = active.get(identifier)
        verdict, evidence = entry[1:] if entry else ("unknown", None)
        step = (edge["to"], edge["distanceM"])
        if verdict == "open":
            admissible[edge["from"]].append(step)
        if verdict != "closed":
            possible[edge["from"]].append(step)
        access[identifier] = {"access": verdict, "evidenceId": evidence}

    verified = distances(admissible, origin)
    maybe = distances(possible, origin)
    result = {}
    for destination in document["destinations"]:
        require(destination in nodes and destination not in result, "invalid or repeated destination")
        outcome = "reachable" if destination in verified else "unknown" if destination in maybe else "unreachable-in-graph"
        result[destination] = {"outcome": outcome, "verifiedDistanceM": verified.get(destination)}
    return {"graphEdition": document["graphEdition"], "cutoff": utc(cutoff), "origin": origin,
            "destinations": result, "edges": access}


def distances(graph, origin):
    distances_by_node = {origin: 0}
    queue = [(0, origin)]
    while queue:
        distance, node = heapq.heappop(queue)
        if distance != distances_by_node[node]:
            continue
        for destination, length in graph[node]:
            candidate = distance + length
            if candidate < distances_by_node.get(destination, math.inf):
                distances_by_node[destination] = candidate
                heapq.heappush(queue, (candidate, destination))
    return distances_by_node


def population_access(document):
    """Aggregate reviewed native-cell allocations to disjoint service zones."""
    text(document["gridEdition"])
    seen = set()
    values = defaultdict(list)
    unknown = Counter()
    outside = []
    for cell in document["cells"]:
        identifier = text(cell["id"])
        require(identifier not in seen, f"duplicate population cell: {identifier}")
        seen.add(identifier)
        count = cell["people"]
        if count is not None:
            require(number(count) >= 0, "population must be nonnegative")
        allocations = cell["zoneFractions"]
        weights = []
        for zone, weight in allocations.items():
            text(zone)
            require(0 < number(weight) <= 1, "zone fraction must be in (0,1]")
            weights.append(weight)
            if count is None:
                unknown[zone] += 1
            else:
                values[zone].append(count * weight)
        fraction = math.fsum(weights)
        require(fraction <= 1 + 1e-12, f"overallocated population cell: {identifier}")
        if count is not None:
            outside.append(count * max(0, 1 - fraction))
    return {"gridEdition": document["gridEdition"], "unit": "people",
            "knownUnallocatedPeople": math.fsum(outside),
            "zones": {z: {"knownPeople": math.fsum(values[z]) if z in values else None,
                          "unknownCellCount": unknown[z]} for z in sorted(values.keys() | unknown.keys())}}


def population_raster(raw, allocation_raw):
    """Read selected cells on the original WorldPop-style count grid."""
    from numpy.ma import is_masked
    from rasterio.io import MemoryFile
    from rasterio.windows import Window

    allocation = json.loads(allocation_raw)
    text(allocation["gridEdition"])
    require(allocation["unit"] == "people-per-cell", "population raster unit must be people-per-cell")
    cells = []
    with MemoryFile(raw) as file, file.open() as raster:
        require(raster.driver == "GTiff" and raster.count == 1 and raster.crs is not None,
                "expected a single-band GeoTIFF with a declared CRS")
        require(raster.scales == (1.0,) and raster.offsets == (0.0,), "population band must use native counts")
        grid = {"crs": raster.crs.to_string(), "width": raster.width, "height": raster.height,
                "transform": list(tuple(raster.transform)[:6])}
        require(allocation["grid"] == grid, "population allocation grid differs from raster")
        for cell in allocation["cells"]:
            row, column = cell["row"], cell["column"]
            require(type(row) is int and type(column) is int and
                    0 <= row < raster.height and 0 <= column < raster.width, "population cell outside native grid")
            value = raster.read(1, window=Window(column, row, 1, 1), masked=True)[0, 0]
            cells.append({"id": f"{row}:{column}",
                          "people": None if is_masked(value) else float(value),
                          "zoneFractions": cell["zoneFractions"]})
    result = population_access({"gridEdition": allocation["gridEdition"], "cells": cells})
    result.update({"grid": grid, "allocationSha256": sha256(allocation_raw).hexdigest()})
    return result


def paired_images(document, event, cutoff, min_usable):
    """Choose a reviewed pair with the same sensor and comparison area."""
    require(0 < min_usable <= 1 and event <= cutoff, "invalid image coverage threshold or time order")
    area = text(document["studyAreaRevision"])
    require(len(set(document["requiredAssets"])) == len(document["requiredAssets"]) > 0,
            "required assets must be a nonempty list of unique names")
    candidates = defaultdict(list)
    identifiers = set()
    rejected = Counter()
    for scene in document["scenes"]:
        identifier = text(scene["id"])
        require(identifier not in identifiers, "duplicate scene ID")
        identifiers.add(identifier)
        acquired = instant(scene["acquiredAt"])
        available = instant(scene["availableAt"]) if scene.get("availableAt") else None
        if available is None or available > cutoff:
            continue
        require(acquired <= available, "image availability precedes acquisition")
        if scene.get("reviewedAreaRevision") != area:
            rejected["review-area-mismatch"] += 1
            continue
        usable = scene.get("usableFraction")
        if usable is None:
            rejected["usable-area-unknown"] += 1
            continue
        require(0 <= number(usable) <= 1, "invalid usable image fraction")
        if not set(document["requiredAssets"]) <= set(scene["verifiedAssets"]):
            rejected["required-asset-unverified"] += 1
        elif usable < min_usable:
            rejected["usable-area-below-threshold"] += 1
        else:
            candidates[text(scene["sensor"])].append((acquired, identifier))

    pairs = []
    for entries in candidates.values():
        before = [item for item in entries if item[0] < event]
        after = [item for item in entries if item[0] >= event]
        if before and after:
            latest_before = max(before)
            earliest_after = min(after)
            pairs.append((earliest_after, latest_before))
    if not pairs:
        return {"outcome": "needs-another-observation", "studyAreaRevision": area, "excluded": dict(rejected)}
    after, before = min(pairs)
    return {"outcome": "paired", "studyAreaRevision": area, "before": before[1], "after": after[1],
            "excluded": dict(rejected)}


def airlab_energy(raw, max_gap_seconds, *, member_flight=None):
    """Integrate measured power in one completed research log."""
    require(number(max_gap_seconds) > 0, "maximum gap must be positive")
    reader = csv.DictReader(io.StringIO(raw.decode("utf-8-sig"), newline=""))
    required = {"time", "battery_voltage", "battery_current"}
    if member_flight is None:
        required.add("flight")
    else:
        member_flight = text(member_flight)
    require(reader.fieldnames and required <= set(reader.fieldnames)
            and len(reader.fieldnames) == len(set(reader.fieldnames)), "invalid AirLab column header")
    energy = []
    elapsed = []
    gaps = []
    previous = None
    flight = None
    for row in reader:
        identifier = text(row.get("flight", member_flight))
        require(member_flight is None or identifier == member_flight, "AirLab member disagrees with the row flight identifier")
        require(flight is None or identifier == flight, "expected one AirLab flight")
        flight = identifier
        at = number(float(row["time"]))
        voltage, current = row["battery_voltage"], row["battery_current"]
        watts = None
        if voltage and current:
            volts, amps = number(float(voltage)), number(float(current))
            require(volts >= 0 and (member_flight is not None or amps >= 0), "negative battery voltage or current")
            watts = number(volts * amps)
        if previous is not None:
            prior_at, prior_watts = previous
            duration = at - prior_at
            require(duration > 0, "AirLab time must increase")
            if duration <= max_gap_seconds and watts is not None and prior_watts is not None:
                energy.append(duration * (watts / 2 + prior_watts / 2) / 3600)
                elapsed.append(duration)
            else:
                gaps.append(duration)
        previous = (at, watts)
    require(flight is not None and (elapsed or gaps), "need at least two AirLab samples")
    return {"flight": flight, "measuredWh": math.fsum(energy) if elapsed else None,
            "includedSeconds": math.fsum(elapsed), "excludedSeconds": math.fsum(gaps),
            "maxGapSeconds": max_gap_seconds}



def port_activity(page, port, start, end, minimum_history, consecutive):
    """Build a daily series with a weekday baseline fixed before the study window."""
    require(0 < (end - start).days <= 3660, "expected a bounded, nonempty port-day interval")
    require(minimum_history >= 2 and consecutive >= 1, "invalid baseline or recovery settings")
    require("error" not in page and not page.get("exceededTransferLimit"), "failed or partial port response")
    counts = {}
    for feature in page["features"]:
        row = feature["attributes"]
        if row["portid"] != port:
            continue
        when = date(row["year"], row["month"], row["day"])
        count = row["portcalls"]
        require(when not in counts, f"duplicate port-day: {when}")
        require(type(count) is int and count >= 0, f"invalid port count: {when}")
        counts[when] = count
    baseline = defaultdict(list)
    for when, count in counts.items():
        if when < start:
            baseline[when.weekday()].append(count)

    daily = []
    below_seen = False
    run = 0
    recovered = None
    for offset in range((end - start).days):
        when = start + timedelta(days=offset)
        history = baseline[when.weekday()]
        count = counts.get(when)
        typical = median(history) if len(history) >= minimum_history else None
        interval = [min(history), max(history)] if typical is not None else None
        if count is None or interval is None:
            status = "missing-port-day" if count is None else "insufficient-history"
        elif count < interval[0]:
            status = "below-history"
            below_seen = True
        elif count > interval[1]:
            status = "above-history"
        else:
            status = "within-history"
        run = run + 1 if below_seen and status == "within-history" else 0
        if run == consecutive and recovered is None:
            recovered = when
        daily.append({"date": when.isoformat(), "portCalls": count, "status": status,
                      "weekdayBaselineMedian": typical, "weekdayBaselineRange": interval,
                      "weekdayBaselineDays": len(history),
                      "departureFromMedian": count - typical if count is not None and typical is not None else None})
    return {"portId": port, "studyFrom": start.isoformat(), "studyToExclusive": end.isoformat(),
            "firstSustainedReturnConfirmedOn": recovered.isoformat() if recovered else None,
            "belowBaselineObserved": below_seen, "daily": daily}


def regional_indicators(document):
    """Preserve the grain and denominator of reviewed regional measures."""
    rows = {}
    for record in document["indicators"]:
        key = text(record["id"])
        require(key not in rows, "duplicate regional measure ID")
        for field in ("edition", "region", "measure", "unit", "populationScope"):
            text(record[field])
        require(day(record["periodFrom"]) < day(record["periodToExclusive"]), "invalid reporting period")
        value = record["value"]
        require(value is None or (type(value) is int and value >= 0), "invalid regional count")
        rows[key] = record
    checks = []
    for check in document["reconciliations"]:
        names = [check["totalId"], *check["componentIds"]]
        require(len(names) >= 2 and len(names) == len(set(names)), "invalid reconciliation membership")
        total, *components = [rows[key] for key in names]
        grain = ("edition", "region", "unit", "periodFrom", "periodToExclusive", "populationScope")
        require(all(all(row[field] == total[field] for field in grain) for row in components),
                "reconciliation mixes reporting scopes")
        values = [row["value"] for row in components]
        component_sum = None if None in values else sum(values)
        residual = total["value"] - component_sum if total["value"] is not None and component_sum is not None else None
        checks.append({"totalId": check["totalId"], "componentIds": check["componentIds"],
                       "componentSum": component_sum, "unallocatedResidual": residual})
    return {"indicators": list(rows.values()), "reconciliations": checks}


def notices_at(document, cutoff):
    """Read reviewed civil service notices as a participant knew them."""
    selected = {}
    seen = {}
    for item in document["notices"]:
        key = text(item["noticeId"])
        evidence = text(item["evidenceId"])
        issued, available = instant(item["issuedAt"]), instant(item["availableAt"])
        effective = instant(item["effectiveAt"])
        expires = instant(item["expiresAt"]) if item.get("expiresAt") else None
        require(issued <= available and (expires is None or effective < expires), "invalid notice timeline")
        revision = item["revision"]
        status = item["status"]
        require(type(revision) is int and revision >= 1 and status in {"active", "cancelled"},
                "invalid notice revision or status")
        identity = (key, revision)
        # Republishing the same issuer revision adds a delivery path.
        payload = (issued, effective, expires, status, text(item["summary"]))
        require(identity not in seen or seen[identity] == payload, "conflicting copies of issuer notice")
        seen[identity] = payload
        if available <= cutoff and effective <= cutoff:
            prior = selected.get(key)
            if prior is None or revision > prior[0]:
                selected[key] = revision, payload, {evidence}
            elif revision == prior[0]:
                prior[2].add(evidence)

    visible = {}
    for key, (revision, (_, _, expires, status, summary), identifiers) in selected.items():
        if status == "active" and expires is not None and expires <= cutoff:
            status = "expired"
        visible[key] = {"revision": revision, "status": status, "summary": summary, "evidenceIds": sorted(identifiers)}
    return {"cutoff": utc(cutoff), "notices": visible}


def floodnet_scores(document):
    """Score a held-out binary road review from reviewed pixel-pair counts."""
    groups = set(document["developmentGroups"])
    cases = set()
    pixels = Counter()
    verdicts = Counter()
    for row in document["cases"]:
        identifier = text(row["id"])
        require(identifier not in cases and text(row["collectionGroup"]) not in groups,
                "duplicate evaluation case or development group overlap")
        cases.add(identifier)
        result = row["reviewVerdict"]
        require(result in {"clear", "flooded", "inconclusive"} and type(row["floodedRoad"]) is bool,
                "invalid human-reviewed verdict")
        for key in ("intersection", "union", "scoredPixels"):
            require(type(row[key]) is int and row[key] >= 0, "invalid pixel count")
        require(row["intersection"] <= row["union"] <= row["scoredPixels"], "inconsistent pixel count")
        pixels.update({key: row[key] for key in ("intersection", "union", "scoredPixels")})
        verdicts["inconclusive"] += result == "inconclusive"
        verdicts["actuallyFlooded"] += row["floodedRoad"]
        verdicts["falseClear"] += row["floodedRoad"] and result == "clear"
    require(cases, "need held-out evaluation cases")
    return {"cases": len(cases), "inconclusive": verdicts["inconclusive"],
            "falseClear": verdicts["falseClear"], "actuallyFlooded": verdicts["actuallyFlooded"],
            "scoredPixels": pixels["scoredPixels"],
            "floodedRoadIoU": pixels["intersection"] / pixels["union"] if pixels["union"] else None}



def delivery_replay(document, cutoff):
    """Compare recorded or exercise-authored civil report delivery to two desks."""
    rows = document["messages"]
    identifiers = set()
    outcomes = {}
    outstanding = 0
    for row in rows:
        identifier = text(row["id"])
        require(identifier not in identifiers, "duplicate report message ID")
        identifiers.add(identifier)
        desk = text(row["recipient"])
        sent = instant(row["sentAt"])
        received = instant(row["receivedAt"]) if row.get("receivedAt") else None
        acknowledged = instant(row["acknowledgedAt"]) if row.get("acknowledgedAt") else None
        require(received is None or sent <= received, "receipt precedes sending")
        require(acknowledged is None or (received is not None and received <= acknowledged),
                "acknowledgment precedes receipt")
        if sent > cutoff:
            continue
        summary = outcomes.setdefault(desk, {"delivered": [], "awaitingDelivery": 0, "awaitingAcknowledgment": 0})
        if received is None or received > cutoff:
            summary["awaitingDelivery"] += 1
            outstanding += 1
            continue
        confirmed = acknowledged is not None and acknowledged <= cutoff
        summary["delivered"].append({"id": identifier, "receivedAt": utc(received),
                                     "lagSeconds": (received - sent).total_seconds(),
                                     "acknowledged": confirmed})
        summary["awaitingAcknowledgment"] += not confirmed
    for result in outcomes.values():
        result["delivered"].sort(key=lambda row: (instant(row["receivedAt"]), row["id"]))
    return {"cutoff": utc(cutoff), "outstandingDeliveries": outstanding, "recipients": outcomes}


def regional_queue(document, cutoff):
    """Compare authored single-pool staffing profiles against the same workload."""
    start = instant(document["from"])
    require(start < cutoff, "queue cutoff must follow start")
    arrivals = []
    seen = set()
    for record in document["arrivals"]:
        identifier = text(record["id"])
        arrival = instant(record["arrivedAt"])
        duration = number(record["serviceSeconds"])
        require(identifier not in seen and arrival >= start and duration > 0, "invalid or repeated arrival")
        seen.add(identifier)
        if arrival < cutoff:
            arrivals.append((arrival, identifier, duration))
    arrivals.sort()
    profiles = {}
    for profile in document["profiles"]:
        key = text(profile["id"])
        staffed = profile["staffedSlots"]
        require(key not in profiles and type(staffed) is int and 0 <= staffed <= 1024,
                "invalid or repeated staffing profile")
        next_free = [start] * staffed
        waits = []
        completed = 0
        for arrival, _, duration in arrivals:
            if not next_free or next_free[0] >= cutoff:
                break
            begins = max(arrival, next_free[0])
            finishes = begins + timedelta(seconds=duration)
            heapq.heapreplace(next_free, finishes)
            waits.append((begins - arrival).total_seconds())
            completed += finishes <= cutoff
        ranked = sorted(waits)
        profiles[key] = {"staffedSlots": staffed, "started": len(waits), "completed": completed,
                         "awaitingService": len(arrivals) - len(waits),
                         "inServiceAtCutoff": len(waits) - completed,
                         "medianStartedWaitSeconds": median(waits) if waits else None,
                         "p90StartedWaitSeconds": ranked[math.ceil(.9 * len(waits)) - 1] if waits else None}
    return {"cutoff": utc(cutoff), "arrivals": len(arrivals), "assumption": "authored-constant-staffing",
            "profiles": profiles}


def cems_products(document, activation, cutoff):
    """Summarize delivered public flood-assessment metadata available by a cutoff."""
    require(not document.get("next") and document.get("count") == 1 and len(document["results"]) == 1,
            "expected one complete activation response")
    item = document["results"][0]
    require(item["code"] == activation and item["sensitive"] is False and item["category"] == "Flood",
            "expected the requested public flood activation")

    def provider_time(value):
        # The provider defines these fields as UTC, including timestamps without a suffix.
        parsed = datetime.fromisoformat(text(value))
        return parsed.replace(tzinfo=timezone.utc) if parsed.tzinfo is None else parsed.astimezone(timezone.utc)

    products = []
    seen = set()
    for area in item["aois"]:
        for product in area["products"]:
            version = product.get("version") or {}
            if product["type"] not in {"DEL", "GRA"} or version.get("statusCode") != "F" or not version.get("deliveryTime"):
                continue
            delivered = provider_time(version["deliveryTime"])
            if delivered > cutoff:
                continue
            require(product["activationCode"] == activation and product["aoiNumber"] == area["number"],
                    "product disagrees with activation or area")
            require(type(version["number"]) is int and version["number"] > 0, "invalid CEMS version number")
            identity = text(version["uuid"])
            require(identity not in seen, "duplicate CEMS product version")
            seen.add(identity)
            imagery = [{"sourceImageId": text(image["uuid"]),
                        "sensor": image.get("sensorName"),
                        "acquisitionAt": utc(provider_time(image["acquisitionTime"])) if image.get("acquisitionTime") else None}
                       for image in product["images"]]
            products.append({
                "productId": product["id"], "type": product["type"], "areaNumber": area["number"],
                "areaName": area["name"], "areaExtentWkt": area["extent"], "productExtentWkt": product["extent"],
                "versionId": identity, "versionNumber": version["number"], "deliveredAt": utc(delivered),
                "monitoringNumber": product["monitoringNumber"], "sourceImages": imagery,
            })
    return {"activation": activation, "cutoff": utc(cutoff), "versionCoverage": "versions-in-supplied-response",
            "products": sorted(products, key=lambda row: (instant(row["deliveredAt"]), row["versionId"]))}


def radar_outages(document, country, start, end):
    """Read coarse historical Internet outage annotations from a retained response."""
    require(document.get("success") is True and not document.get("errors"),
            "expected a successful Radar annotations response")
    require(start < end and len(country) == 2 and country.isascii() and country.isalpha(),
            "expected a country code and an ordered observation window")
    seen = set()
    reported = []
    for item in document["result"]["annotations"]:
        if item["eventType"] != "OUTAGE" or country not in item["locations"]:
            continue
        identifier = text(item["id"])
        require(identifier not in seen, "duplicate Radar annotation")
        seen.add(identifier)
        began = instant(item["startDate"])
        ended = instant(item["endDate"]) if item.get("endDate") else None
        require(ended is None or began <= ended, "Radar annotation ends before it starts")
        if began >= end or (ended is not None and ended <= start):
            continue
        outage = item["outage"]
        reported.append({"id": identifier, "startAt": utc(began), "endAt": utc(ended) if ended else None,
                         "scope": text(item["scope"]), "providerType": text(outage["outageType"])})
    return {"country": country, "from": utc(start), "toExclusive": utc(end), "availabilityAt": None,
            "coverage": "annotations-in-supplied-response",
            "annotations": sorted(reported, key=lambda row: (instant(row["startAt"]), row["id"]))}
