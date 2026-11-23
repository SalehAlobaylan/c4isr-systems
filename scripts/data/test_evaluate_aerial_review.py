from pathlib import Path
import tempfile
import unittest
import zipfile

from evaluate_aerial_review import airlab_definition, completed_energy, flood_review, score_handoff, sha
from prepare_aerial_review import reviewed_csv


class AerialEvaluationTests(unittest.TestCase):
    def test_archived_recordings_keep_whole_flights_and_exclude_gaps(self):
        with tempfile.TemporaryDirectory() as name:
            directory = Path(name)
            archive = directory / "flights.zip"
            parameters = directory / "parameters.csv"
            parameters.write_text("flight,route\n14,R1\n15,R2\n", encoding="utf-8")
            table = b"time,battery_voltage,battery_current\n0,10,2\n1,10,2\n5,10,-1\n6,10,5\n"
            with zipfile.ZipFile(archive, "w") as recordings:
                for flight in (14, 15):
                    recordings.writestr(f"flights/{flight}.csv", table)
            result = completed_energy(archive, airlab_definition(archive, parameters), 1)["groups"][0]
            self.assertEqual((result["trainingFlights"], result["heldOutFlights"], result["scoredFlights"]), (1, 1, 1))
            self.assertAlmostEqual(result["recordings"][0]["measuredWh"], 40 / 3600)
            self.assertAlmostEqual(result["p90AbsoluteErrorWh"], 0)
            self.assertEqual(result["excludedHeldOutSeconds"], 4)

    def test_handoff_keeps_missing_acceptance_unresolved(self):
        case = {"reviewSha256": "case", "firstCueAt": "2026-11-18T09:00:00Z",
                "cutoffAt": "2026-11-18T10:05:00Z", "requiredOwner": "Municipal desk"}
        record = {"reviewSha256": "case", "events": [
            {"kind": "acknowledged", "at": "2026-11-18T09:00:00Z"},
            {"kind": "accepted-follow-up", "at": "2026-11-18T09:25:00Z", "owner": "Other desk"},
        ]}
        self.assertEqual(score_handoff(case, record), {"acknowledged": True, "paused": False,
                                                       "handoffAccepted": False, "unresolvedFollowUps": 1})
        record["events"][1]["owner"] = "Municipal desk"
        self.assertEqual(score_handoff(case, record)["unresolvedFollowUps"], 0)
        record["events"][1]["at"] = "2026-11-18T10:06:00Z"
        with self.assertRaises(ValueError):
            score_handoff(case, record)

    def test_historical_extract_keeps_zero_distinct_from_missing(self):
        with tempfile.TemporaryDirectory() as name:
            path = Path(name) / "days.csv"
            path.write_text("day,availableAt,civilCount,unclassified,usableHours,oldestAt\n"
                            "2024-08-01,2024-08-03T00:00:00Z,0,1,6,2024-08-01T01:00:00Z\n"
                            "2024-08-02,2024-08-03T00:00:00Z,,,0,\n", encoding="utf-8")
            result = reviewed_csv(path, "owned", "v1", "exercise", "traffic")
            self.assertEqual(result["traffic"][0]["civilCount"], 0)
            self.assertIsNone(result["traffic"][1]["civilCount"])
            self.assertEqual(result["sources"][0]["sha256"], sha(path.read_bytes()))

    def test_held_out_mask_and_review_metrics_share_the_same_cases(self):
        try:
            import numpy as np
            import rasterio
        except ImportError:
            self.skipTest("requires the aerial raster dependencies")
        with tempfile.TemporaryDirectory() as name:
            directory = Path(name)

            def store(filename, values):
                path = directory / filename
                with rasterio.open(path, "w", driver="GTiff", height=2, width=2, count=1, dtype="uint8",
                                   transform=rasterio.transform.from_origin(5, 5, 1, 1)) as raster:
                    raster.write(np.array(values, dtype=np.uint8), 1)
                return {"path": filename, "sha256": sha(path.read_bytes())}

            truth = store("truth.tif", [[3, 3], [4, 255]])
            estimate = store("estimate.tif", [[3, 4], [4, 3]])
            case = {"id": "one", "split": "test", "collectionGroup": "held-site", "imageSha256": "one",
                    "visualGroup": "held-overlap", "mask": truth, "floodedRoad": True}
            manifest = {"floodedRoadClass": 3, "ignoreClass": 255, "classes": [3, 4], "cases": [case]}
            submissions = {"runs": [
                {"id": "unaided", "cases": [{"id": "one", "verdict": "clear", "reviewSeconds": 10}]},
                {"id": "baseline", "cases": [{"id": "one", "verdict": "inconclusive", "reviewSeconds": 8, "prediction": estimate}]},
                {"id": "assisted", "cases": [{"id": "one", "verdict": "flooded", "reviewSeconds": 6, "prediction": truth}]},
            ]}
            runs = flood_review(manifest, submissions, directory)["runs"]
            self.assertEqual(runs["unaided"]["falseClear"], 1)
            self.assertEqual(runs["baseline"]["inconclusive"], 1)
            self.assertEqual(runs["baseline"]["floodedRoadIoU"], .5)
            self.assertEqual(runs["assisted"]["floodedRoadIoU"], 1)
            manifest["cases"].append({**case, "id": "two", "split": "train"})
            with self.assertRaisesRegex(ValueError, "crosses partitions"):
                flood_review(manifest, submissions, directory)


if __name__ == "__main__":
    unittest.main()
