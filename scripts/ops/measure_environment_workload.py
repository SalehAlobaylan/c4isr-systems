"""Measure the offline consumer path against one prepared package.

The workload follows what a receiver actually does: verify the archive, open it,
read scattered native cells, look up one feature and select a bounded area. It
reports p50/p95 wall time, peak resident memory, bytes and cleanup so a budget
can be agreed from evidence rather than a metadata benchmark.
"""
import argparse
import ctypes
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import statistics
import sys
import tempfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))

import environment_library as base
import environment_reader as reader

SCHEMA = "emad.environment-workload.v1"
DEFAULT_PACKAGE = Path(__file__).resolve().parents[2] / "testdata" / "geospatial" / "owned-fixture-v1"
DEFAULT_BUDGETS = {"verifyP95Ms": 500, "kitVerifyP95Ms": 2500, "inventoryP95Ms": 300,
                   "sampleP95Ms": 150, "pointP95Ms": 150, "featureP95Ms": 400,
                   "boundedFeaturesP95Ms": 300, "fieldP95Ms": 100, "peakRssBytes": 768 << 20}


def peak_rss():
    if os.name == "nt":
        class Counters(ctypes.Structure):
            _fields_ = [("cb", ctypes.c_ulong), ("PageFaultCount", ctypes.c_ulong),
                        ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
                        ("QuotaPeakPagedPoolUsage", ctypes.c_size_t),
                        ("QuotaPagedPoolUsage", ctypes.c_size_t),
                        ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t),
                        ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                        ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t)]
        counters = Counters()
        counters.cb = ctypes.sizeof(counters)
        getter = ctypes.windll.psapi.GetProcessMemoryInfo
        getter.argtypes = [ctypes.c_void_p, ctypes.POINTER(Counters), ctypes.c_ulong]
        getter.restype = ctypes.c_int
        process = ctypes.windll.kernel32.GetCurrentProcess
        process.restype = ctypes.c_void_p
        if getter(process(), ctypes.byref(counters), counters.cb):
            return int(counters.PeakWorkingSetSize)
        return 0
    import resource
    return int(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss) * 1024


def timed(function, iterations, warmup):
    for _ in range(warmup):
        function()
    samples = []
    for _ in range(iterations):
        start = time.perf_counter()
        function()
        samples.append((time.perf_counter() - start) * 1000)
    ordered = sorted(samples)
    return {"count": len(samples), "p50Ms": round(statistics.median(ordered), 3),
            "p95Ms": round(ordered[min(len(ordered) - 1, int(len(ordered) * .95))], 3),
            "maxMs": round(ordered[-1], 3)}


def measure(package, iterations=15, warmup=2, kit=False):
    package = Path(package)
    fixture = package.parent if package.name == "prepared.zip" else package
    package = package if package.is_file() else package / "prepared.zip"
    measurements = {}
    cleanup = {}
    kit_identity = None
    with tempfile.TemporaryDirectory() as location:
        probe = Path(location) / "probe.bin"
        probe.write_bytes(b"cleanup probe")
        cleanup_before = len(list(Path(location).iterdir()))
        probe.unlink()
        cleanup["removedProbe"] = cleanup_before == 1 and not probe.exists()
        source_package = package
        if kit:
            import environment_kit
            seed = reader.Reader(package, topology=fixture / "topology.json")
            prepared_sha, prepared_bytes = base.file_digest(package)
            document = environment_kit.inventory_document(
                {"id": fixture.name, "manifestSha256": seed.manifest_sha, "manifest": seed.manifest},
                [], {"sha256": prepared_sha, "bytes": prepared_bytes},
                purpose="native-inspection", terms=None)
            directory = Path(location) / "kit"
            environment_kit.publish(directory, document, package)
            inventory_sha = environment_kit.digest(document)
            kit_identity = {"inventorySha256": inventory_sha, "purpose": "native-inspection",
                            "displays": 0}
            measurements["kitVerify"] = timed(
                lambda: environment_kit.verify(directory, inventory_sha, None, True),
                max(1, iterations // 5), 1)
            source_package = directory / "prepared.zip"
        source = reader.Reader(source_package, topology=fixture / "topology.json")
        manifest = source.manifest
        measurements["verify"] = timed(lambda: base.verify(source_package), max(1, iterations // 5), 0)
        measurements["inventory"] = timed(lambda: source.inventory(), iterations, warmup)
        measurements["sample"] = timed(
            lambda: source.sample("raster-00-00.tif", 10, 10), iterations, warmup)
        measurements["point"] = timed(
            lambda: source.point("surface", 46.73, 24.7055, component=0), iterations, warmup)
        measurements["feature"] = timed(
            lambda: source.feature("roads", "osm-way-1001"), iterations, warmup)
        measurements["boundedFeatures"] = timed(
            lambda: source.features("roads", [46.70, 24.70, 46.76, 24.76], 100), iterations, warmup)
        measurements["field"] = timed(
            lambda: reader.field_read(fixture / "field-era5.json", "temperature", 850,
                                      "2022-10-23T12:00:00Z", 46.5, 25.25), iterations, warmup)
        return {
            "schema": SCHEMA,
            "fixture": manifest.get("profileId"),
            "manifestSha256": source.manifest_sha,
            "packageBytes": package.stat().st_size,
            "memberBytes": sum(int(item["bytes"]) for item in manifest["members"]),
            "iterations": iterations,
            "warmup": warmup,
            "kit": kit_identity,
            "operations": measurements,
            "peakRssBytes": peak_rss(),
            "cleanup": cleanup,
            "budgets": DEFAULT_BUDGETS,
        }


def assess(result):
    failures = []
    mapping = {"verify": "verifyP95Ms", "kitVerify": "kitVerifyP95Ms", "inventory": "inventoryP95Ms",
               "sample": "sampleP95Ms", "point": "pointP95Ms", "feature": "featureP95Ms",
               "boundedFeatures": "boundedFeaturesP95Ms", "field": "fieldP95Ms"}
    for operation, budget in mapping.items():
        if operation not in result["operations"]:
            continue
        if result["operations"][operation]["p95Ms"] > result["budgets"][budget]:
            failures.append({"operation": operation, "budget": budget,
                             "observedMs": result["operations"][operation]["p95Ms"],
                             "allowedMs": result["budgets"][budget]})
    if result["peakRssBytes"] > result["budgets"]["peakRssBytes"]:
        failures.append({"operation": "peakRss", "observedBytes": result["peakRssBytes"],
                         "allowedBytes": result["budgets"]["peakRssBytes"]})
    if not all(result["cleanup"].values()):
        failures.append({"operation": "cleanup", "observed": result["cleanup"]})
    result["status"] = "PASS" if not failures else "FAIL"
    result["failures"] = failures
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--package", type=Path, default=DEFAULT_PACKAGE)
    parser.add_argument("--iterations", type=int, default=15)
    parser.add_argument("--warmup", type=int, default=2)
    parser.add_argument("--measured-at", default=datetime.now(timezone.utc).isoformat())
    parser.add_argument("--output", type=Path)
    parser.add_argument("--kit", action="store_true",
                        help="Assemble a kit in temporary storage and measure its verification too")
    parser.add_argument("--max-rss-bytes", type=int, default=DEFAULT_BUDGETS["peakRssBytes"])
    args = parser.parse_args()
    if not 5 <= args.iterations <= 200 or not 0 <= args.warmup <= 20 or args.max_rss_bytes < 1:
        parser.error("Use 5-200 iterations, 0-20 warmups and a positive memory budget")
    result = measure(args.package, args.iterations, args.warmup, args.kit)
    result["budgets"]["peakRssBytes"] = args.max_rss_bytes
    result["measuredAt"] = args.measured_at
    result = assess(result)
    raw = json.dumps(result, sort_keys=True, indent=2) + "\n"
    if args.output:
        args.output.write_text(raw, encoding="utf-8")
    print(raw, end="")
    return 0 if result["status"] == "PASS" else 2


if __name__ == "__main__":
    raise SystemExit(main())
