"""Review the simulation-source plans and optionally check independent provider samples."""
import argparse
from collections import Counter
from datetime import datetime, timezone
from hashlib import sha256
import json
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[2]
DIRECTORY = ROOT / "plans/archive/simulation-sources"
PLANS = (
    "01-reports-roads-and-inspections.md",
    "02-aerial-inspection-and-aviation.md",
    "03-regional-response-and-coast.md",
    "04-supporting-sources.md",
)
# Each installed mapping is specific to the product and the registered Go adapter.
ADAPTERS = {
    "aisstream": "AISAdapter", "earth-search": "EarthSearchAdapter",
    "eonet": "EONETAdapter", "firms": "FIRMSAdapter", "gdacs": "GDACSAdapter",
    "metar": "METARAdapter", "opensky": "OpenSkyAdapter", "usgs": "USGSAdapter",
}
PLANNED = {"aggregate", "benchmark", "field", "grid", "inspection", "notice", "reference", "table", "vector"}
PILOTS = {
    "osm-roads": ("pilot/vector", "testdata/geospatial/owned-fixture-v1/topology.json"),
    "gebco": ("pilot/grid", "scripts/data/environment_coastal.py"),
    "shoreline": ("pilot/vector", "scripts/data/coastal_pilot.py"),
    "noaa20": ("pilot/reference", "scripts/data/civil_orbit_context.py"),
    "era5": ("pilot/field", "testdata/geospatial/owned-fixture-v1/field-era5.json"),
}
PATHS = {
    "design": PLANNED,
    "defer": {"aggregate", "display", "reference"},
}
REFERENCE = re.compile(r"\[([a-z0-9-]+)\]\(normalization\.md#([a-z0-9-]+)\)")
Q1_SHA256 = "b3619c468a0e91c06d718ed7421b7f18deb5c7d78535f54046f7f807b8b60a21"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def read(path):
    return path.read_text(encoding="utf-8")


def registered_adapters():
    directory = ROOT / "internal/publicingest"
    inventory = read(directory / "capabilities.go").split("func CapabilityInventory()", 1)[1]
    members = re.search(r"range \[\]string\{([^}]+)\}", inventory)
    require(members, "cannot locate the registered adapter inventory")
    symbols = set(re.findall(r"\b\w+Adapter\b", members[1]))
    constants = {}
    for path in directory.glob("*.go"):
        constants.update(re.findall(r'^const (\w+Adapter) = "([^"]+)"', read(path), re.M))
    result = {}
    for name, symbol in ADAPTERS.items():
        require(symbol in symbols and symbol in constants, f"{name}: adapter {symbol} missing from inventory")
        result[name] = constants[symbol]
    return result


def dictionary(document, adapters):
    owner = None
    inputs = {}
    for line in document.splitlines():
        if line.startswith("[Feature plan and acceptance cases]"):
            match = re.fullmatch(r"\[Feature plan and acceptance cases\]\(([^)]+)\)", line)
            require(match and match[1] in PLANS, f"unknown feature plan in {line}")
            owner = match[1]
        if not line.startswith("| ") or line.startswith(("| Input |", "| ---")):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        require(owner and len(cells) == 5 and all(cells), f"incomplete normalization row: {line}")
        anchor = re.fullmatch(r'<a id="([a-z0-9-]+)"></a>\1', cells[0])
        require(anchor, f"invalid input identity: {cells[0]}")
        name = anchor[1]
        require(name not in inputs, f"duplicate mapping: {name}")
        route = cells[1].strip("`")
        if name in adapters:
            require(route == adapters[name], f"{name}: expected registered adapter {adapters[name]}")
        elif name in PILOTS:
            expected, evidence = PILOTS[name]
            require(route == expected and (ROOT / evidence).is_file(), f"{name}: pilot mapping or evidence missing")
        else:
            stage, _, kind = route.partition("/")
            require(kind in PATHS.get(stage, set()), f"{name}: unknown integration path {route}")
        inputs[name] = owner
    require(adapters.keys() <= inputs.keys(), "missing a planned installed-adapter mapping")
    require(PILOTS.keys() <= inputs.keys(), "missing a bounded pilot mapping")
    return inputs


def mapped_inputs(filename, document):
    if filename == PLANS[-1]:
        # Each supporting-source table row is one acceptance case.
        groups = []
        for line in document.splitlines():
            if line.startswith("| ") and not line.startswith(("| Source |", "| ---")):
                cells = [cell.strip() for cell in line.strip("|").split("|")]
                require(len(cells) == 3 and all(cells), f"{filename}: incomplete source/feature/acceptance row")
                groups.append(cells[0])
    else:
        groups = []
        for section in re.split(r"^### ", document, flags=re.M)[1:]:
            title, _, body = section.partition("\n")
            mappings = re.findall("^Field mapping · (.+)$", body, re.M)
            require(len(mappings) == 1, f"{filename}: {title} needs one field mapping line")
            groups.extend(mappings)
    require(groups, f"{filename}: missing source sections")
    result = []
    for group in groups:
        references = REFERENCE.findall(group)
        require(references and all(a == b for a, b in references), f"{filename}: invalid mapping in {group}")
        result.extend(a for a, b in references)
    return result


def check_links(path, document):
    for target in re.findall(r"\[[^\]\n]*\]\(([^)\n]+)\)", document):
        url = urlsplit(target)
        if url.scheme:
            continue
        destination = (path.parent / unquote(url.path)).resolve() if url.path else path
        require(destination.is_file(), f"{path.name}: missing {target}")
        if url.fragment:
            contents = read(destination)
            anchors = set(re.findall(r'<a id="([^"]+)">', contents))
            for heading in re.findall(r"^#+ (.+)$", contents, re.M):
                anchors.add(re.sub(r"[^\w -]", "", heading.lower()).replace(" ", "-"))
            require(unquote(url.fragment) in anchors, f"{path.name}: missing anchor {target}")


def check_plans():
    documents = {p.name: read(p) for p in DIRECTORY.glob("*.md")}
    require(set(PLANS) == {p for p in documents if re.match(r"^\d+-", p)}, "numbered plan inventory changed")
    inputs = dictionary(documents["normalization.md"], registered_adapters())
    for filename in PLANS:
        require(f"]({filename})" in documents["README.md"], f"index omits {filename}")
        linked = mapped_inputs(filename, documents[filename])
        expected = {name for name, owner in inputs.items() if owner == filename}
        require(len(linked) == len(set(linked)), f"{filename}: repeated primary mapping")
        require(set(linked) == expected, f"{filename}: unmatched inputs {sorted(set(linked) ^ expected)}")
    for filename, document in documents.items():
        check_links(DIRECTORY / filename, document)
    print(f"Plan coverage passed: {len(PLANS)} plans, {len(inputs)} input mappings, {len(ADAPTERS)} registered adapter mappings.", flush=True)


def check_riyadh(raw):
    require(sha256(raw).hexdigest() == Q1_SHA256, "Riyadh Q1 edition digest differs")
    rows = json.loads(raw)
    fields = {"نوع البلاغ", "الموقع", "حالة البلاغ", "زمن الإغلاق", "تاريخ الاستلام", "تكرار"}
    require(len(rows) == 6820 and all(set(row) == fields for row in rows), "Riyadh row count or fields differ")
    categories = Counter(" ".join(row["نوع البلاغ"].split()) for row in rows)
    require(categories["طرق ملاحظات اسفلت وحفر فى الطريق"] == 524, "asphalt category differs")
    require(categories["القطاعات و البلديات مخلفات بناء لمبنى تحت الإنشاء"] == 301, "construction waste category differs")
    require(sum(isinstance(row["الموقع"], str) and bool(row["الموقع"].strip()) for row in rows) == 6819,
            "text location count differs")
    require(sum(row["الموقع"] == 0 for row in rows) == 1, "unresolved numeric location count differs")
    require(sum(row["تكرار"].strip() == "مكرر" for row in rows) == 3160, "repeat flag count differs")
    require(sum(row["حالة البلاغ"].strip() == "جاري العمل" for row in rows) == 92, "in-progress count differs")
    dates = [datetime.fromtimestamp(row["تاريخ الاستلام"] / 1000, tz=timezone.utc).date() for row in rows]
    require((min(dates).isoformat(), max(dates).isoformat()) == ("2026-01-01", "2026-03-31"), "receipt range differs")


def check_ports(raw):
    page = json.loads(raw)
    require("error" not in page and not page.get("exceededTransferLimit"), "failed or partial PortWatch query")
    expected = {"port24": ("Jubail", 4), "port2074": ("Jazan", 0)}
    found = {}
    for feature in page["features"]:
        row = feature["attributes"]
        port = row["portid"]
        require((row["year"], row["month"], row["day"]) == (2024, 1, 15), "unexpected port-day")
        require(port not in found, f"duplicate port-day for {port}")
        value = row["portcalls"]
        require(type(value) is int and value >= 0, f"{port}: invalid port call count")
        found[port] = (row["portname"], value)
    for port, control in expected.items():
        require(found.get(port) == control, f"{port}: missing or changed control row")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform-tests", action="store_true")
    parser.add_argument("--riyadh-q1", type=Path)
    parser.add_argument("--portwatch-day", type=Path)
    args = parser.parse_args()
    check_plans()
    for path, name, check in [(args.riyadh_q1, "Riyadh Q1", check_riyadh),
                              (args.portwatch_day, "PortWatch day", check_ports)]:
        if path is None:
            print(f"{name} provider sample: not supplied")
        else:
            raw = path.read_bytes()
            check(raw)
            print(f"{name} provider sample passed: sha256={sha256(raw).hexdigest()}")
    if args.platform_tests:
        subprocess.run([sys.executable, "-m", "unittest", "test_check_simulation_source_plans"],
                       cwd=ROOT / "scripts/ops", check=True)
        subprocess.run([sys.executable, "-m", "unittest", "test_environment_suitability", "test_simulation_inputs",
                        "test_evaluate_civilian_access", "test_evaluate_aerial_review", "test_regional_response_inputs", "test_supporting_sources"],
                       cwd=ROOT / "scripts/data", check=True)
        subprocess.run(["go", "test", "-count=1", "-short", "-skip", "TestOptionalGDACSRSSCapture",
                        "./internal/publicingest", "./internal/geoenv"], cwd=ROOT, check=True)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, KeyError, subprocess.CalledProcessError) as exc:
        sys.exit(f"Simulation source check failed: {exc}")
