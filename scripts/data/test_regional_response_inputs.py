from datetime import datetime, timezone
from hashlib import sha256
import json
from pathlib import Path
import tempfile
import unittest

from evaluate_coastal_search import compile_drifters, evaluate_search
from prepare_regional_response import attach_port_calls, reception_coverage


class RegionalResponseInputTests(unittest.TestCase):
    def test_reception_deduplicates_and_preserves_a_disconnect(self):
        packet = sha256(b"receipt").hexdigest()
        records = {"days": [
            {"day": "2024-01-01", "availableAt": "2024-01-03T00:00:00Z", "window": [0, 60],
             "connected": [[0, 10], [8, 12], [40, 60]],
             "receipts": [{"minute": 8, "digest": packet}, {"minute": 45, "digest": packet}]},
            {"day": "2024-01-02", "availableAt": "2024-01-03T00:00:00Z", "window": [0, 60],
             "connected": [], "receipts": []},
        ]}
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            file = folder / "reception.json"
            file.write_text(json.dumps(records))
            coverage = reception_coverage(file, "ais-fixture", "owned", "exercise")
            days = coverage["coast"]["coverage"]
            self.assertEqual((days[0]["usableMinutes"], days[0]["uniqueReports"], days[0]["duplicates"]), (32, 1, 1))
            self.assertIsNone(days[1]["uniqueReports"])

            normalized, port = folder / "normalized.json", folder / "port.json"
            normalized.write_text(json.dumps(coverage))
            port.write_text(json.dumps({"features": [{"attributes": {"portid": "port24", "year": 2024,
                                                                      "month": 1, "day": 2, "portcalls": 0}}]}))
            comparison = attach_port_calls(normalized, port, "port24", datetime(2024, 1, 4, tzinfo=timezone.utc))["coast"]["coverage"]
            self.assertIsNone(comparison[0]["portWatchCalls"])
            self.assertEqual(comparison[1]["portWatchCalls"], 0)
            self.assertEqual(comparison[1]["portWatchSourceId"], "imf-portwatch-port24")
            self.assertEqual(comparison[1]["availableAt"], "2024-01-03T00:00:00Z")
            self.assertEqual(comparison[1]["portWatchAvailableAt"], "2024-01-04T00:00:00Z")
            records["days"][0]["receipts"][0]["minute"] = 15
            file.write_text(json.dumps(records))
            with self.assertRaisesRegex(ValueError, "out-of-coverage"):
                reception_coverage(file, "ais-fixture", "owned", "exercise")

    def test_noaa_reference_releases_only_prior_conditions_and_scores_union(self):
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            raw = (b"ID,time,latitude,longitude,ve,vn,sst\n,UTC,degrees_north,degrees_east,m/s,m/s,Kelvin\n"
                   b"123,2017-01-15T00:00:00Z,31.1,-51.1,0.1,-0.2,291\n"
                   b"123,2017-01-15T01:00:00Z,31.1,-51.1,0.1,-0.2,291\n"
                   b"123,2017-01-15T02:00:00Z,31.1,-51.3,0.1,-0.2,291\n")
            source, layout = folder / "drifter.csv", folder / "cells.json"
            source.write_bytes(raw)
            start, hidden, end = "2017-01-15T00:00:00Z", "2017-01-15T01:00:00Z", "2017-01-15T03:00:00Z"
            case = {"id": "test", "providerId": "123", "observationGroup": "group-one", "split": "test",
                    "from": start, "cutoff": end, "conditionAt": start, "blindFrom": hidden}
            doc = {"captureSha256": sha256(raw).hexdigest(), "cases": [case], "cells": [
                {"id": "east", "bounds": [-51.25, 31, -51, 31.25]},
                {"id": "west", "bounds": [-51.5, 31, -51.25, 31.25]},
            ]}
            layout.write_text(json.dumps(doc))
            public, reference = compile_drifters(source, layout)
            self.assertEqual(sha256(public).hexdigest(), reference["exerciseSha256"])
            self.assertNotIn(b"providerId", public)
            self.assertNotIn(b"observations", public)
            held = json.loads(public)
            receipts = {"cases": [{"id": "test", "effort": [{"cell": "east", "completedAt": start}] * 2,
                                    "sightings": [{"cell": "east", "at": hidden}, {"cell": "west", "at": hidden}]}]}
            report = evaluate_search(held, reference, receipts)
            self.assertEqual(report["verifiedCases"], 1)
            self.assertEqual(report["cases"][0]["firstVerifiedSightingSeconds"], 3600)
            self.assertEqual(report["cases"][0]["searchedAreaKm2"], held["cells"][0]["areaKm2"])
            self.assertEqual(report["cases"][0]["unverifiedClaims"], 1)
            receipts["cases"][0]["sightings"] = []
            self.assertEqual(evaluate_search(held, reference, receipts)["unresolvedCases"], 1)
            doc["cases"].append({**case, "split": "development", "id": "leak"})
            layout.write_text(json.dumps(doc))
            with self.assertRaisesRegex(ValueError, "crosses observation groups"):
                compile_drifters(source, layout)


if __name__ == "__main__":
    unittest.main()
