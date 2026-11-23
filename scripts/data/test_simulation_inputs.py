from datetime import date, timedelta
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import simulation_inputs as readings


AT = readings.instant("2026-11-21T10:00:00+03:00")
AT_TEXT = readings.utc(AT)
EARLY = readings.utc(AT - timedelta(minutes=10))
LATE = readings.utc(AT + timedelta(minutes=10))


def municipal_sample():
    return json.dumps([dict(zip(readings.MUNICIPAL_FIELDS.values(),
                                ("طرق  ملاحظات", 0, "تم التنفيذ", 0, 1774468495190, "مكرر")))]).encode()


class AccessTests(unittest.TestCase):
    def test_population_raster_preserves_native_grid_and_mask(self):
        try:
            import numpy
            from rasterio.io import MemoryFile
            from rasterio.transform import from_origin
        except ImportError:
            self.skipTest("optional environment reader dependencies are unavailable")
        transform = from_origin(46, 25, .001, .001)
        grid = dict(crs="EPSG:4326", width=3, height=1, transform=list(tuple(transform)[:6]))
        allocation = {"gridEdition": "owned-v1", "grid": grid, "unit": "people-per-cell", "cells": [
            {"row": 0, "column": index, "zoneFractions": {zone: 1}}
            for index, zone in enumerate(["zero", "masked", "fractional"])
        ]}
        with MemoryFile() as file:
            with file.open(driver="GTiff", width=3, height=1, count=1, dtype="float32",
                           nodata=-9999, transform=transform, crs="EPSG:4326") as raster:
                raster.write(numpy.array([[0, -9999, 2.5]], dtype="float32"), 1)
            raw = file.read()
        result = readings.population_raster(raw, json.dumps(allocation).encode())
        self.assertEqual(result["zones"]["zero"]["knownPeople"], 0)
        self.assertIsNone(result["zones"]["masked"]["knownPeople"])
        self.assertEqual(result["zones"]["fractional"]["knownPeople"], 2.5)
        grid["width"] = 4
        with self.assertRaisesRegex(ValueError, "grid differs"):
            readings.population_raster(raw, json.dumps(allocation).encode())

    def test_q1_status_has_export_semantics_and_no_invented_location(self):
        raw = municipal_sample()
        result = readings.municipal(raw, True)
        row = result["rows"][0]
        self.assertEqual(result["rowsWithoutPlaceLabel"], 1)
        self.assertEqual(result["categoryCounts"], {"طرق ملاحظات": 1})
        self.assertEqual(row["receiptAt"], "2026-03-25T19:54:55.190000Z")
        self.assertIsNone(result["availabilityAt"])
        self.assertIsNone(row["reportedPlaceLabel"])
        self.assertFalse("rows" in readings.municipal(raw))
        self.assertTrue(row["snapshot"]["repeatFlag"])
        self.assertIsNone(row["snapshot"]["closureUnit"])
        self.assertEqual(row["snapshot"]["closureValue"], 0)
        self.assertEqual(result, readings.municipal(raw, True))

    def test_direction_and_availability_are_independent_of_inspection_time(self):
        document = {
            "graphEdition": "owned-test", "nodes": ["a", "b", "c"], "destinations": ["a", "b", "c"],
            "edges": [{"id": name, "from": a, "to": b, "distanceM": length}
                      for name, a, b, length in [("ab", "a", "b", 10), ("ba", "b", "a", 10),
                                                  ("ac", "a", "c", 12), ("cb", "c", "b", 5)]],
            "inspections": [
                {"edgeId": edge, "effectiveAt": EARLY, "availableAt": EARLY,
                 "revision": 1, "access": verdict, "evidenceId": f"{edge}-old"}
                for edge, verdict in [("ab", "closed"), ("ba", "closed"), ("ac", "open"), ("cb", "open")]
            ] + [
                {"edgeId": "ab", "effectiveAt": AT_TEXT, "availableAt": LATE,
                 "revision": 2, "access": "open", "evidenceId": "ab-reopen"}
            ],
        }
        before = readings.road_access(document, AT, "a")
        after = readings.road_access(document, readings.instant(LATE), "a")
        self.assertEqual(before["destinations"]["b"]["verifiedDistanceM"], 17)
        self.assertEqual(before["edges"]["ab"]["evidenceId"], "ab-old")
        self.assertEqual(after["destinations"]["b"]["verifiedDistanceM"], 10)
        self.assertEqual(after["edges"]["ba"]["access"], "closed")
        self.assertEqual(readings.road_access(document, AT, "b")["destinations"]["a"]["outcome"],
                         "unreachable-in-graph")
        document["inspections"].pop(0)
        document["inspections"].pop(2)
        self.assertEqual(readings.road_access(document, AT, "a")["destinations"]["b"]["outcome"], "unknown")

    def test_population_zero_missing_and_split_cells(self):
        document = {"gridEdition": "owned-grid", "cells": [
            {"id": "split", "people": 12.5, "zoneFractions": {"a": .4, "b": .5}},
            {"id": "zero", "people": 0, "zoneFractions": {"a": 1}},
            {"id": "masked", "people": None, "zoneFractions": {"c": 1}},
        ]}
        result = readings.population_access(document)
        self.assertEqual(result["zones"]["a"], {"knownPeople": 5, "unknownCellCount": 0})
        self.assertEqual(result["zones"]["c"], {"knownPeople": None, "unknownCellCount": 1})
        self.assertAlmostEqual(sum(zone["knownPeople"] or 0 for zone in result["zones"].values())
                               + result["knownUnallocatedPeople"], 12.5)
        document["cells"][0]["zoneFractions"]["b"] = .8
        with self.assertRaisesRegex(ValueError, "overallocated"):
            readings.population_access(document)


class AerialTests(unittest.TestCase):
    def test_cems_filters_actual_delivery_without_claiming_archive_completeness(self):
        code = "EMSR-OWNED"
        product = {
            "activationCode": code, "aoiNumber": 1, "id": 22, "type": "DEL",
            "extent": "POLYGON EMPTY", "monitoringNumber": 1,
            "version": {"number": 2, "uuid": "v2", "statusCode": "F", "deliveryTime": "2026-11-21T07:00:00"},
            "images": [{"uuid": "source-image", "acquisitionTime": EARLY}],
        }
        activation = {"code": code, "sensitive": False, "category": "Flood",
                      "aois": [{"number": 1, "name": "Owned example", "extent": "POLYGON EMPTY", "products": [product]}]}
        document = {"count": 1, "next": None, "results": [activation]}
        earlier = readings.cems_products(document, code, AT - timedelta(microseconds=1))
        self.assertEqual(earlier["products"], [])
        current = readings.cems_products(document, code, AT)
        self.assertEqual(current["products"][0]["versionNumber"], 2)
        self.assertEqual(current["products"][0]["sourceImages"][0]["sourceImageId"], "source-image")
        self.assertEqual(current["versionCoverage"], "versions-in-supplied-response")

    def test_image_pair_hides_unavailable_scenes_and_checks_assets(self):
        before = dict(id="before", acquiredAt=EARLY, availableAt=EARLY, reviewedAreaRevision="area",
                      usableFraction=.9, verifiedAssets=["visual", "mask"], sensor="sensor")
        after = dict(before, id="after", acquiredAt=AT_TEXT, availableAt=LATE)
        missing_asset = dict(after, id="incomplete", availableAt=AT_TEXT, verifiedAssets=["visual"])
        request = {"studyAreaRevision": "area", "requiredAssets": ["visual", "mask"],
                   "scenes": [before, after, missing_asset]}
        self.assertEqual(readings.paired_images(request, AT, AT, .8)["outcome"], "needs-another-observation")
        pair = readings.paired_images(request, AT, readings.instant(LATE), .8)
        self.assertEqual((pair["before"], pair["after"]), ("before", "after"))

    def test_energy_integrates_adjacent_measured_samples_only(self):
        sample = b"flight,time,battery_voltage,battery_current\n17,0,18,10\n17,1,18,10\n17,9,18,10\n17,10,,\n"
        result = readings.airlab_energy(sample, 2)
        self.assertAlmostEqual(result["measuredWh"], .05)
        self.assertEqual((result["includedSeconds"], result["excludedSeconds"]), (1, 9))
        self.assertIsNone(readings.airlab_energy(sample, .5)["measuredWh"])

    def test_flood_review_scores_uncertainty_and_group_leakage(self):
        row = dict(id="test", collectionGroup="test-site", reviewVerdict="clear", floodedRoad=True,
                   intersection=5, union=10, scoredPixels=20)
        document = dict(developmentGroups=["training"], cases=[row, dict(row, id="test2", reviewVerdict="inconclusive")])
        result = readings.floodnet_scores(document)
        self.assertEqual((result["falseClear"], result["actuallyFlooded"], result["inconclusive"]), (1, 2, 1))
        self.assertEqual(result["floodedRoadIoU"], .5)
        document["developmentGroups"].append("test-site")
        with self.assertRaisesRegex(ValueError, "overlap"):
            readings.floodnet_scores(document)


class ContextTests(unittest.TestCase):
    def test_outage_source_preserves_unknown_end_and_availability(self):
        row = {"id": "owned", "eventType": "OUTAGE", "locations": ["SA"], "startDate": AT_TEXT,
               "endDate": None, "scope": "Regional example", "outage": {"outageType": "REGIONAL"}}
        page = {"success": True, "result": {"annotations": [row]}}
        report = readings.radar_outages(page, "SA", readings.instant(EARLY), readings.instant(LATE))
        self.assertIsNone(report["annotations"][0]["endAt"])
        self.assertIsNone(report["availabilityAt"])
        self.assertEqual(readings.radar_outages(page, "US", readings.instant(EARLY),
                                               readings.instant(LATE))["annotations"], [])
        with self.assertRaisesRegex(ValueError, "successful"):
            readings.radar_outages(dict(page, success=False), "SA", AT, readings.instant(LATE))

    def test_late_correction_reaches_desks_at_different_times(self):
        document = {"messages": [
            {"id": "correction-a", "recipient": "a", "sentAt": EARLY, "receivedAt": AT_TEXT,
             "acknowledgedAt": LATE},
            {"id": "correction-b", "recipient": "b", "sentAt": EARLY, "receivedAt": LATE},
        ]}
        early = readings.delivery_replay(document, AT)
        self.assertEqual(early["recipients"]["a"]["delivered"][0]["lagSeconds"], 600)
        self.assertEqual(early["recipients"]["a"]["awaitingAcknowledgment"], 1)
        self.assertEqual(early["recipients"]["b"]["delivered"], [])
        self.assertEqual(early["outstandingDeliveries"], 1)
        late = readings.delivery_replay(document, readings.instant(LATE))
        self.assertEqual(late["outstandingDeliveries"], 0)
        self.assertEqual(late["recipients"]["a"]["awaitingAcknowledgment"], 0)

    def test_staffing_comparison_tracks_in_service_and_unstarted_arrivals(self):
        document = {"from": EARLY, "arrivals": [
            {"id": "first", "arrivedAt": EARLY, "serviceSeconds": 601},
            {"id": "second", "arrivedAt": EARLY, "serviceSeconds": 600},
        ], "profiles": [{"id": label, "staffedSlots": slots}
                        for label, slots in [("closed", 0), ("one", 1), ("two", 2)]]}
        result = readings.regional_queue(document, AT)["profiles"]
        self.assertEqual(result["closed"]["awaitingService"], 2)
        self.assertEqual(result["one"]["inServiceAtCutoff"], 1)
        self.assertEqual(result["one"]["awaitingService"], 1)
        self.assertEqual(result["two"]["completed"], 1)
        self.assertEqual(result["two"]["medianStartedWaitSeconds"], 0)

    def test_frozen_port_baseline_and_recovery_with_a_missing_day(self):
        start = date(2024, 1, 15)

        def feature(offset, calls):
            d = start + timedelta(days=offset)
            return {"attributes": dict(portid="port24", year=d.year, month=d.month, day=d.day, portcalls=calls)}

        page = {"features": [feature(i, 4) for i in range(-14, 0)] +
                [feature(0, 0), feature(1, 4), feature(3, 4), feature(4, 4)]}
        result = readings.port_activity(page, "port24", start, start + timedelta(days=5), 2, 2)
        self.assertEqual(result["daily"][0]["portCalls"], 0)
        self.assertIsNone(result["daily"][2]["portCalls"])
        self.assertEqual(result["daily"][4]["weekdayBaselineMedian"], 4)
        self.assertEqual(result["firstSustainedReturnConfirmedOn"], "2024-01-19")
        with self.assertRaisesRegex(ValueError, "partial"):
            readings.port_activity(dict(page, exceededTransferLimit=True), "port24", start,
                                   start + timedelta(days=5), 2, 2)

    def test_regional_outcomes_preserve_the_published_residual(self):
        basis = dict(edition="SRCA-2025", region="Eastern Province", unit="cases", populationScope="field cases",
                     periodFrom="2025-01-01", periodToExclusive="2026-01-01")
        document = {"indicators": [dict(basis, id=key, measure=key, value=value)
                                   for key, value in [("total", 110718), ("transfer", 71841), ("onsite", 38865)]],
                    "reconciliations": [{"totalId": "total", "componentIds": ["transfer", "onsite"]}]}
        self.assertEqual(readings.regional_indicators(document)["reconciliations"][0]["unallocatedResidual"], 12)
        document["indicators"][1]["populationScope"] = "other"
        with self.assertRaisesRegex(ValueError, "reporting scopes"):
            readings.regional_indicators(document)

    def test_notice_delivery_and_supersession(self):
        active = dict(noticeId="road-maintenance", evidenceId="publisher", issuedAt=EARLY, availableAt=EARLY,
                      effectiveAt=EARLY, revision=1, summary="Planned street works", status="active")
        copy_arrived = dict(active, evidenceId="distributor", availableAt=AT_TEXT)
        cancellation = dict(active, evidenceId="cancel", revision=2, issuedAt=AT_TEXT, effectiveAt=AT_TEXT,
                            availableAt=LATE, status="cancelled")
        document = {"notices": [cancellation, copy_arrived, active]}
        before = readings.notices_at(document, AT)["notices"]["road-maintenance"]
        self.assertEqual(before["evidenceIds"], ["distributor", "publisher"])
        self.assertEqual(before["status"], "active")
        self.assertEqual(readings.notices_at(document, readings.instant(LATE))["notices"]["road-maintenance"]["status"],
                         "cancelled")


class CLITests(unittest.TestCase):
    def test_municipal_cli_publishes_a_receipt_and_refuses_changed_input(self):
        script = Path(__file__).with_name("read_simulation_inputs.py")
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "q1.json"
            path.write_bytes(municipal_sample())
            command = [sys.executable, str(script), "municipal", str(path)]
            completed = subprocess.run(command, capture_output=True, text=True, check=True)
            receipt = json.loads(completed.stdout)
            self.assertEqual(receipt["schema"], "emad.simulation-input-analysis.v1")
            self.assertEqual(receipt["result"]["rowCount"], 1)
            path.write_bytes(b"[]")
            changed = subprocess.run([*command, "--sha256", receipt["inputSha256"]],
                                     capture_output=True, text=True)
            self.assertNotEqual(changed.returncode, 0)
            self.assertIn("SHA-256 mismatch", changed.stderr)
