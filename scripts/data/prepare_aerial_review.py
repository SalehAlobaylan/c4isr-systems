"""Prepare bounded, attributable civilian aerial-inspection evidence."""

import argparse
import base64
import csv
from datetime import datetime, time, timedelta, timezone
from hashlib import sha256
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import struct
import sys
from urllib.parse import urlsplit
import zlib

from simulation_inputs import instant, require, utc


def source(path, identity, edition, attribution):
    with path.open("rb") as stream:
        from hashlib import file_digest
        digest = file_digest(stream, "sha256").hexdigest()
    return {"id": identity, "edition": edition, "sha256": digest, "attribution": attribution}


def provider_utc(value):
    parsed = datetime.fromisoformat(value)
    return parsed.replace(tzinfo=timezone.utc) if parsed.tzinfo is None else parsed.astimezone(timezone.utc)


def checked_bounds(value):
    try:
        west, south, east, north = map(float, value.split(","))
    except ValueError as err:
        raise argparse.ArgumentTypeError("expected west,south,east,north") from err
    if not (-180 <= west < east <= 180 and -85 <= south < north <= 85 and
            east-west <= .1 and north-south <= .1):
        raise argparse.ArgumentTypeError("expected a bounded area of up to 0.1 degrees on each axis")
    return west, south, east, north


def png_asset(role, image):
    height, width, channels = image.shape

    def chunk(name, data):
        return struct.pack(">I", len(data)) + name + data + struct.pack(">I", zlib.crc32(name+data))

    settings = struct.pack(">2I5B", width, height, 8, {3: 2, 4: 6}[channels], 0, 0, 0)
    rows = b"".join(b"\x00" + row.tobytes() for row in image)
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", settings) + chunk(b"IDAT", zlib.compress(rows)) + chunk(b"IEND", b"")
    require(len(png) <= 520000, "PNG review preview exceeds its asset budget")
    return {"role": role, "sha256": sha256(png).hexdigest(), "png": base64.b64encode(png).decode("ascii")}


def earthsearch_scene(path, item_id, bounds, area_revision, available, time_basis, assets, metadata_only):
    page = json.loads(path.read_bytes())
    matches = [item for item in page["features"] if item["id"] == item_id]
    require(len(matches) == 1 and matches[0]["collection"] == "sentinel-2-c1-l2a", "expected one pinned Sentinel-2 C1 L2A item")
    item = matches[0]
    acquired = instant(item["properties"]["datetime"])
    require(available >= acquired, "scene availability precedes acquisition")
    if published := item["properties"].get("created"):
        require(available >= instant(published), "scene availability precedes catalog creation")
    identity = "earth-search"
    partial = any(link["rel"] == "next" for link in page.get("links", []))
    candidate = {"id": item_id, "sourceId": identity, "sensorFamily": "Sentinel-2 MSI", "acquiredAt": utc(acquired),
                 "availableAt": utc(available), "timeBasis": time_basis, "areaRevision": area_revision,
                 "bounds": list(bounds), "catalogCoverage": "partial-page" if partial else "supplied-catalog",
                 "grid": "assets-unavailable", "pixelM": 10, "usableFraction": None, "coveredFraction": None, "assets": []}
    fragment = {"sources": [source(path, identity, "sentinel-2-c1-l2a", "Copernicus Sentinel-2 imagery via Element 84 Earth Search")],
                "catalogNextPresent": partial, "scenes": [candidate]}
    if metadata_only:
        return fragment

    import numpy as np
    import rasterio
    from rasterio.features import geometry_mask
    from rasterio.warp import transform_bounds, transform_geom, reproject, Resampling
    from rasterio.windows import Window, from_bounds
    from math import floor, ceil

    def raster_location(key, override):
        if override:
            return str(override.resolve())
        uri = item["assets"][key]["href"]
        url = urlsplit(uri)
        require(url.scheme == "https" and url.netloc == "e84-earth-search-sentinel-data.s3.us-west-2.amazonaws.com" and
                not url.query and not url.fragment and url.path.startswith("/sentinel-2-c1-l2a/") and
                f"/{item_id}/" in url.path, "scene asset is outside the supported Earth Search source")
        return uri

    with rasterio.Env(GDAL_DISABLE_READDIR_ON_OPEN="EMPTY_DIR"):
        with rasterio.open(raster_location("visual", assets[0])) as visual:
            require(visual.count == 3 and all(t == "uint8" for t in visual.dtypes) and visual.crs and visual.crs.is_projected,
                    "expected the native RGB preview in a projected metre frame")
            projected = transform_bounds("EPSG:4326", visual.crs, *bounds)
            wish = from_bounds(*projected, transform=visual.transform)
            left, top = floor(wish.col_off), floor(wish.row_off)
            window = Window(left, top, ceil(wish.col_off + wish.width) - left, ceil(wish.row_off + wish.height) - top)
            require(0 < window.width <= 384 and 0 < window.height <= 384, "select a smaller review area")
            rgb = visual.read(window=window, boundless=True, masked=True)
            frame = visual.window_transform(window)
            crs = visual.crs
            pixel_m = max(abs(frame.a), abs(frame.e))
        with rasterio.open(raster_location("scl", assets[1])) as mask:
            require(mask.count == 1 and mask.crs is not None, "expected a single source scene-classification mask")
            quality = np.zeros(rgb.shape[1:], dtype=np.uint8)
            reproject(rasterio.band(mask, 1), quality, src_transform=mask.transform, src_crs=mask.crs,
                      src_nodata=0, dst_transform=frame, dst_crs=crs, dst_nodata=0,
                      resampling=Resampling.nearest)
    west, south, east, north = bounds
    outline = {"type": "Polygon", "coordinates": [[(west, south), (east, south), (east, north), (west, north), (west, south)]]}
    selected = geometry_mask([transform_geom("EPSG:4326", crs, outline)], out_shape=quality.shape, transform=frame, invert=True)
    require(selected.any(), "no native image pixel centers in the selected review area")
    coverage = selected & (~np.any(np.ma.getmaskarray(rgb), axis=0)) & (quality != 0)
    usable = coverage & np.isin(quality, [4, 5, 6])
    overlay = np.zeros((*quality.shape, 4), dtype=np.uint8)
    overlay[~coverage] = (20, 27, 37, 220)
    overlay[coverage & ~usable] = (255, 153, 56, 190)
    visual_png = png_asset("visual", np.moveaxis(rgb.filled(0), 0, -1))
    quality_png = png_asset("quality-mask", overlay)
    grid = f"{crs}:{list(frame)[:6]}:{quality.shape[1]}x{quality.shape[0]}"
    candidate.update(grid=grid, pixelM=pixel_m, usableFraction=float(usable[selected].mean()),
                     coveredFraction=float(coverage[selected].mean()), assets=[visual_png, quality_png])
    return fragment


def cems_assessment(path, vectors, activation, product_id, bounds):
    from shapely.geometry import box, shape
    from shapely import from_wkt

    doc = json.loads(path.read_bytes())
    require(doc.get("count") == 1 and not doc.get("next") and len(doc["results"]) == 1,
            "expected a complete single CEMS activation response")
    event = doc["results"][0]
    require(event["code"] == activation and event["category"] == "Flood" and event["sensitive"] is False,
            "expected the requested public flood product")
    matches = [item for area in event["aois"] for item in area["products"] if item["id"] == product_id]
    require(len(matches) == 1, "missing or ambiguous flood product")
    product = matches[0]
    version = product["version"]
    require(version["statusCode"] == "F" and version["deliveryTime"] and product["type"] in {"DEL", "GRA"},
            "select a finished delineation or grading product")
    extent = from_wkt(product["extent"])
    require(extent.is_valid and extent.geom_type in {"Polygon", "MultiPolygon"}, "invalid CEMS assessment extent")
    review = box(*bounds)
    overlap = extent.intersection(review).area
    coverage = "inside" if extent.covers(review) else "partial" if overlap > 0 else "outside"
    layer = json.loads(vectors.read_bytes())
    legacy_crs = layer.get("crs", {}).get("properties", {}).get("name")
    require(layer["type"] == "FeatureCollection" and legacy_crs in {None, "urn:ogc:def:crs:OGC:1.3:CRS84"},
            "expected a reviewed WGS84 observed-event GeoJSON layer")
    declared_layers = {Path(entry["json"]).stem for entry in product["layers"] if entry.get("json") and
                       (product["type"] == "GRA" or "observedEvent" in entry["name"])}
    require(layer.get("name") in declared_layers, "vector layer identity does not match the provider's product metadata")
    require(len(layer["features"]) <= 10000, "select a bounded product layer")
    affected, classes = False, set()
    for feature in layer["features"]:
        geom = shape(feature["geometry"])
        require(geom.is_valid and not geom.is_empty and geom.geom_type in {"Polygon", "MultiPolygon"},
                "invalid observed-event polygon")
        if geom.intersection(review).area > 0:
            affected = True
            props = feature["properties"]
            value = props.get("notation") or props.get("obj_desc") or props.get("event_type")
            if value:
                classes.add(value)
    identity = f"cems-{activation}"
    return {"sources": [source(path, identity, activation, "European Union, Copernicus EMS Rapid Mapping"),
                        source(vectors, f"{identity}-{product_id}-v{version['number']}-observed", str(version["uuid"]),
                               "European Union, Copernicus EMS observed-event layer")],
            "assessments": [{"id": f"{activation}-{product_id}", "sourceId": identity,
                             "label": f"{product.get('aoiName') or activation}, " +
                                      (f"monitoring update {product['monitoringNumber']}" if product["monitoring"] else "initial assessment"),
                             "kind": "delineation" if product["type"] == "DEL" else "grading",
                             "version": version["number"], "availableAt": utc(provider_utc(version["deliveryTime"])),
                             "sceneRefs": [entry["uuid"] for entry in product["images"]],
                             "sourceImageTimes": [utc(provider_utc(entry["acquisitionTime"])) for entry in product["images"]],
                             "coverage": coverage, "affected": affected if coverage == "inside" else None,
                             "classes": sorted(classes), "assessedAreaWkt": product["extent"]}]}


class IssueTable(HTMLParser):
    """Collect visible table cells from the public SANS issue index."""
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.rows = []
        self.row = None
        self.cell = None

    def handle_starttag(self, tag, attrs):
        if tag == "tr":
            self.row = []
        elif tag == "td" and self.row is not None:
            self.cell = []

    def handle_data(self, data):
        if self.cell is not None:
            self.cell.append(data)

    def handle_endtag(self, tag):
        if tag == "td" and self.cell is not None:
            self.row.append(" ".join(" ".join(self.cell).split()))
            self.cell = None
        if tag == "tr" and self.row is not None:
            self.rows.append(self.row)
            self.row = None


def sans_editions(path):
    table = IssueTable()
    table.feed(path.read_text(encoding="utf-8-sig", errors="strict"))
    notices = []
    identity = "sans-index"
    seen = set()
    months = {month: i for i, month in enumerate(
        "JAN FEB MAR APR MAY JUN JUL AUG SEP OCT NOV DEC".split(), 1)}

    def midnight(value):
        day, month, year = value.upper().split()
        # A date-only notice is released at the end of its stated UTC day for
        # this retrospective use case. UTC midnight is a comparison convention.
        return datetime.combine(datetime(int(year), months[month], int(day)).date(), time(), timezone.utc)

    for row in table.rows:
        if len(row) < 3:
            continue
        match = re.match(r"(?:AIRAC )?AIP AMDT (\d{1,2}/\d\d)", row[2].upper())
        if not match:
            continue
        effective, published = midnight(row[0]), midnight(row[1])
        require(published <= effective, "SANS edition publication follows the effective date")
        identity_tag = "sans-aip-" + match[1].replace("/", "-")
        if identity_tag in seen:
            continue
        seen.add(identity_tag)
        notices.append({"id": identity_tag, "sourceId": identity, "deliveryId": identity_tag, "revision": 1,
                        "kind": "aip-edition", "publisher": "SANS", "title": f"AIP amendment {match[1]}",
                        "publishedAt": utc(published), "availableAt": utc(published + timedelta(days=1)),
                        "effectiveAt": utc(effective), "expiresAt": None, "status": "active"})
    require(notices, "no dated eAIP amendments found in the supplied index")
    return {"sources": [source(path, identity, "historic-issue-index", "Saudi Air Navigation Services")],
            "timeBasis": "dates-only; available at the end of the published UTC date", "notices": notices}


def reviewed_csv(path, identity, edition, attribution, action):
    with path.open(encoding="utf-8-sig", newline="") as stream:
        reader = csv.DictReader(stream)
        if action == "traffic":
            columns = {"day", "availableAt", "civilCount", "unclassified", "usableHours", "oldestAt"}
        else:
            columns = {"id", "deliveryId", "revision", "publisher", "title", "publishedAt",
                       "availableAt", "effectiveAt", "expiresAt", "status"}
        require(len(reader.fieldnames or []) == len(columns) and set(reader.fieldnames or []) == columns,
                "incorrect columns for the bounded historical extract")
        rows = []
        for row in reader:
            require(None not in row and all(value is not None for value in row.values()), "malformed extract row")
            row["sourceId"] = identity
            row["availableAt"] = utc(instant(row["availableAt"]))
            if action == "traffic":
                row["usableHours"] = int(row["usableHours"])
                for key in ("civilCount", "unclassified"):
                    row[key] = int(row[key]) if row[key] else None
                row["oldestAt"] = utc(instant(row["oldestAt"])) if row["oldestAt"] else None
            else:
                row.update(kind="service-notice", revision=int(row["revision"]))
                for key in ("publishedAt", "effectiveAt"):
                    row[key] = utc(instant(row[key]))
                row["expiresAt"] = utc(instant(row["expiresAt"])) if row["expiresAt"] else None
            rows.append(row)
    return {"sources": [source(path, identity, edition, attribution)],
            "traffic" if action == "traffic" else "notices": rows}


def energy_summary(path, available):
    receipt = json.loads(path.read_bytes())
    require(receipt["schema"] == "emad.civilian-aerial-evaluation.v1" and
            receipt["result"]["scope"] == "retrospective-completed-recordings", "expected a completed-flight benchmark receipt")
    identity = "airlab-energy"
    fields = ("platform", "testType", "trainingFlights", "heldOutFlights", "scoredFlights", "medianAbsoluteErrorWh",
              "p90AbsoluteErrorWh", "excludedHeldOutSeconds")
    groups = [{**{key: group[key] for key in fields}, "availableAt": utc(available), "sourceId": identity}
              for group in receipt["result"]["groups"]]
    return {"sources": [source(path, identity, receipt["definitionSha256"], "EMAD retrospective evaluation of CMU AirLab research data")],
            "energy": groups}


def attach_overlay(path, predictions, item, class_id, attribution, available):
    import numpy as np
    import rasterio

    fragment = json.loads(path.read_bytes())
    matching = [row for row in fragment["scenes"] if row["id"] == item]
    require(len(matching) == 1, "expected one matching review scene")
    scene = matching[0]
    require(available >= instant(scene["availableAt"]), "overlay release precedes scene release")
    with rasterio.open(predictions) as dataset:
        grid = f"{dataset.crs}:{list(dataset.transform)[:6]}:{dataset.width}x{dataset.height}"
        require(dataset.count == 1 and dataset.dtypes[0].startswith(("uint", "int")) and dataset.crs and
                grid == scene["grid"] and dataset.width <= 384 and dataset.height <= 384,
                "prediction must be a class raster on the identical preview grid")
        flagged = dataset.read(1, masked=True) == class_id
    overlay = np.zeros((*flagged.shape, 4), dtype=np.uint8)
    overlay[flagged.filled(False)] = (0, 157, 255, 160)
    require(all(asset["role"] != "flood-overlay" for asset in scene["assets"]), "scene already has a suggested overlay")
    scene["assets"].append({**png_asset("flood-overlay", overlay), "availableAt": utc(available)})
    fragment["sources"].append(source(predictions, f"flood-overlay-{item}", "reviewed-class-raster", attribution))
    return fragment


def assemble(path, fragments):
    out = json.loads(path.read_bytes())
    require(out["schema"] == "emad.civilian-aerial-review.v1", "expected a civilian aerial exercise definition")
    sources = {entry["id"]: entry for entry in out.get("sources", [])}
    require(len(sources) == len(out.get("sources", [])), "exercise contains repeated source identity")
    for fragment in fragments:
        data = json.loads(fragment.read_bytes())
        for item in data["sources"]:
            identity = item["id"]
            require(identity not in sources or sources[identity] == item, "fragment changes a pinned source")
            sources[identity] = item
        for key in ("scenes", "assessments", "briefing", "traffic", "notices", "energy"):
            if key in data:
                out[key] = out.get(key, []) + data[key]
    out["sources"] = list(sources.values())
    return out


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    scene = sub.add_parser("earthsearch")
    scene.add_argument("input", type=Path)
    scene.add_argument("--item", required=True)
    scene.add_argument("--bounds", required=True, type=checked_bounds)
    scene.add_argument("--area-revision", required=True)
    scene.add_argument("--available-at", type=instant, required=True)
    scene.add_argument("--time-basis", required=True)
    scene.add_argument("--visual", type=Path)
    scene.add_argument("--scl", type=Path)
    scene.add_argument("--metadata-only", action="store_true", help="retain the catalog candidate with unavailable assets")
    cems = sub.add_parser("cems")
    cems.add_argument("input", type=Path)
    cems.add_argument("vectors", type=Path)
    cems.add_argument("--activation", required=True)
    cems.add_argument("--product", type=int, required=True)
    cems.add_argument("--bounds", type=checked_bounds, required=True)
    sub.add_parser("sans-index").add_argument("input", type=Path)
    energy = sub.add_parser("energy-summary")
    energy.add_argument("input", type=Path)
    energy.add_argument("--available-at", type=instant, required=True)
    overlay = sub.add_parser("attach-overlay")
    overlay.add_argument("input", type=Path)
    overlay.add_argument("predictions", type=Path)
    overlay.add_argument("--item", required=True)
    overlay.add_argument("--flooded-road-class", type=int, required=True)
    overlay.add_argument("--attribution", required=True)
    overlay.add_argument("--available-at", type=instant, required=True)
    for action in ("traffic", "service-notice"):
        extract = sub.add_parser(action)
        extract.add_argument("input", type=Path)
        extract.add_argument("--source-id", required=True)
        extract.add_argument("--edition", required=True)
        extract.add_argument("--attribution", required=True)
    bundle = sub.add_parser("assemble")
    bundle.add_argument("input", type=Path)
    bundle.add_argument("fragments", type=Path, nargs="+")
    args = parser.parse_args()
    handlers = {
        "earthsearch": lambda: earthsearch_scene(args.input, args.item, args.bounds, args.area_revision,
                                                  args.available_at, args.time_basis, (args.visual, args.scl), args.metadata_only),
        "cems": lambda: cems_assessment(args.input, args.vectors, args.activation, args.product, args.bounds),
        "sans-index": lambda: sans_editions(args.input),
        "energy-summary": lambda: energy_summary(args.input, args.available_at),
        "attach-overlay": lambda: attach_overlay(args.input, args.predictions, args.item, args.flooded_road_class,
                                                  args.attribution, args.available_at),
        "assemble": lambda: assemble(args.input, args.fragments),
    }
    if args.action in {"traffic", "service-notice"}:
        print(json.dumps(reviewed_csv(args.input, args.source_id, args.edition, args.attribution, args.action),
                         ensure_ascii=True, sort_keys=True, allow_nan=False))
        return
    print(json.dumps(handlers[args.action](), ensure_ascii=True, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, KeyError, AttributeError, ImportError) as err:
        sys.exit(f"Aerial review input refused: {err}")
