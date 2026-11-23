"""Owned geographic records for the Desert Maps normalization workflow."""

from copy import deepcopy
from datetime import datetime, timezone
import json
from base64 import b64decode, b64encode
import unittest
from unittest.mock import Mock

from prepare_desert_maps import (
    compare, digest, encode, normalize, place_batches, prepare_handoff,
    publish_points, read_handoff, search_key, thematic_payloads,
)


RIGHTS = {
    "id": "emad-owned-fixture",
    "legalBasis": "Owned fictional records",
    "termsUrl": "https://example.org/owned-fixture",
    "attribution": "EMAD owned fixture",
    "retainRaw": True,
    "allowAnalysis": True,
    "allowAtlasDisplay": True,
    "allowThematicDisplay": True,
    "allowOfflineDistribution": True,
    "allowCivilianExercise": True,
}


def point(x, y):
    return {"type": "Point", "coordinates": [x, y]}


def layer(layer_id, title, *entries):
    return {"id": layer_id, "title": title, "entryCount": len(entries), "entries": list(entries)}


def delivery():
    return {
        "schema": "emad.desert-maps-delivery.v1", "publisher": "EMAD owned fixture",
        "crs": "OGC:CRS84", "version": "v1", "complete": True,
        "retrievedAt": "2026-11-22T00:00:00Z",
        "maps": [
            {"id": "riyadh-fixture", "region": "Riyadh · fixture", "layers": [
                layer("water-a", "تجمع مياه 1",
                      {"name": "وادي المثال", "id": "wadi", "geometry": point(46.5, 24.5)}),
                layer("water-b", "تجمع مياه 2",
                      {"geometry": {"type": "LineString", "coordinates": [[46, 24], [46.1, 24.1]]}}),
                layer("unknown", "مصدر غير معروف", {"geometry": point(46, 24)}),
            ]},
            {"id": "east-fixture", "region": "Eastern Province · fixture", "layers": [
                layer("local", "أسماء ثانوية",
                      {"id": "first", "name": "المثال", "geometry": point(49, 26)},
                      {"id": "second", "name": "المثال", "geometry": point(50, 26)},
                      {"name": "No geometry"}),
            ]},
            {"id": "jazan-fixture", "region": "Jazan · fixture", "layers": [
                layer("local-a", "أسماء ثانوية", {"name": "أرض المثال", "geometry": point(42, 16)}),
                layer("local-b", "أسماء ثانوية", {"name": "أرض المثال", "geometry": point(43, 16)}),
            ]},
        ],
    }


class DesertMapsTest(unittest.TestCase):
    def test_entries_conserve_geometry_and_identity(self):
        raw = encode(delivery())
        normalized = normalize(raw, RIGHTS)
        self.assertEqual(normalized, normalize(raw, RIGHTS))
        self.assertEqual(len(normalized["layers"]), 6)
        self.assertEqual(len(normalized["entries"]), 8)
        self.assertEqual(len({entry["key"] for entry in normalized["entries"]}), 8)
        self.assertEqual(sum(entry["searchEligible"] for entry in normalized["entries"]), 5)
        self.assertEqual(sum(entry["displayEligible"] for entry in normalized["entries"]), 6)
        vector = normalized["reference"]["features"]
        self.assertEqual(vector[1]["geometry"]["type"], "LineString")
        self.assertIsNone(vector[1]["properties"]["name"])
        self.assertEqual(vector[0]["geometry"]["coordinates"], [46.5, 24.5])
        places = json.loads(next(place_batches(normalized)))["features"]
        self.assertEqual(sum(feature["properties"]["name"] == "المثال" for feature in places), 2)
        self.assertTrue(all(feature["properties"]["kind"] == "place" for feature in places))
        self.assertEqual(search_key("وَادِي المِثَـال"), "وادي المثال")
        self.assertEqual(search_key("أرض المثال"), "ارض المثال")

    def test_review_and_expiry(self):
        first = normalize(encode(delivery()), RIGHTS)
        second_delivery = deepcopy(delivery())
        second_delivery["version"] = "v2"
        second_delivery["maps"][0]["layers"][0]["entries"][0]["name"] = "وادي المثل"
        second = normalize(encode(second_delivery), RIGHTS)
        key = first["entries"][0]["key"]
        self.assertEqual(compare(first, second)["changed"], [key])
        self.assertTrue(compare(first, second)["needsContinuityReview"])
        now = datetime.now(timezone.utc)
        handoff = encode(prepare_handoff(first, key, "owned-release-v1", "2098-01-01T00:00:00Z"))
        self.assertEqual(read_handoff(handoff, digest(handoff), now)["properties"]["name"], "وادي المثال")
        with self.assertRaisesRegex(ValueError, "expired"):
            read_handoff(handoff, digest(handoff), datetime(2099, 1, 1, tzinfo=timezone.utc))
        with self.assertRaisesRegex(ValueError, "trusted digest"):
            read_handoff(handoff + b" ", digest(handoff), now)
        first["rights"] = {**RIGHTS, "allowOfflineDistribution": False}
        with self.assertRaisesRegex(ValueError, "allowOfflineDistribution"):
            prepare_handoff(first, key, "owned-release-v1", "2098-01-01T00:00:00Z")

    def test_partial_delivery_and_atomic_activation_request(self):
        api = Mock()
        staged = normalize(encode(delivery()), RIGHTS)
        with self.assertRaisesRegex(ValueError, "partial"):
            publish_points(api, "example", {**staged, "complete": False}, "previous")
        api.call.assert_not_called()
        api.call.side_effect = [{"id": "staged-batch"}, {"id": "released"}]
        self.assertEqual(publish_points(api, "example", staged, "previous"), {"id": "released"})
        stage, activate = api.call.call_args_list
        self.assertIs(stage.args[2]["staged"], True)
        self.assertEqual(activate.args[2]["editionIds"], ["staged-batch"])
        self.assertEqual(activate.args[2]["expectedHead"], "previous")

    def test_delivery_inventory(self):
        raw = delivery()
        raw["maps"][0]["layers"][0]["entryCount"] += 1
        with self.assertRaisesRegex(ValueError, "manifest"):
            normalize(encode(raw), RIGHTS)

    def test_control_point_rejects_plausible_swapped_axes(self):
        raw = delivery()
        raw["maps"][0]["controlPoint"] = {
            "layerId": "water-a", "entryId": "wadi", "coordinates": [46.5, 24.5],
        }
        normalized = normalize(encode(raw), RIGHTS)
        self.assertEqual(normalized["layers"][0]["searchEligible"], 1)
        raw["maps"][0]["layers"][0]["entries"][0]["geometry"] = point(24.5, 46.5)
        with self.assertRaisesRegex(ValueError, "control point"):
            normalize(encode(raw), RIGHTS)

    def test_online_member_can_be_read_offline_with_a_separate_fictional_bulletin(self):
        member = encode({"id": "old-wadi", "properties": {"name": "وادي المثال"}})
        package = {
            "schema": "emad.desert-maps-civilian-reference.v2",
            "expiresAt": "2098-01-01T00:00:00Z", "attribution": "Owned fixture",
            "member": b64encode(member).decode("ascii"), "memberSha256": digest(member),
        }
        raw = encode(package)
        bulletin = {"kind": "fictional-bulletin", "placeId": "old-wadi",
                    "author": "Owned test bulletin", "issuedAt": "2026-11-22T01:00:00Z"}
        now = datetime.now(timezone.utc)
        self.assertEqual(read_handoff(raw, digest(raw), now)["id"], bulletin["placeId"])
        changed = encode({**package, "member": b64encode(b"{}").decode("ascii")})
        with self.assertRaisesRegex(ValueError, "feature digest"):
            read_handoff(changed, digest(changed), now)

    def test_aliases_keep_review_provenance_and_original_display_name(self):
        raw = delivery()
        entry = raw["maps"][0]["layers"][0]["entries"][0]
        entry["reviewedAliases"] = [{"name": "وادي الاختبار", "reviewRef": "owned-review-1"}]
        normalized = normalize(encode(raw), RIGHTS)
        place = json.loads(next(place_batches(normalized)))["features"][0]["properties"]
        self.assertEqual(place["name"], "وادي المثال")
        self.assertEqual(place["aliases"], "وادي الاختبار")
        self.assertEqual(place["reviewedAliases"][0]["reviewRef"], "owned-review-1")
        del entry["reviewedAliases"][0]["reviewRef"]
        with self.assertRaisesRegex(ValueError, "review reference"):
            normalize(encode(raw), RIGHTS)

    def test_thematic_import_preserves_unnamed_drawing(self):
        normalized = normalize(encode(delivery()), RIGHTS)
        requests = list(thematic_payloads(normalized, "example"))
        self.assertEqual(len(requests), 3)
        self.assertTrue(all(r["resolution"] == {"value": 0, "unit": "unknown"} for r in requests))
        drawings = [item for r in requests for item in json.loads(b64decode(r["artifact"]))["features"]
                    if item["geometry"]["type"] == "LineString"]
        self.assertEqual(len(drawings), 1)
        self.assertIsNone(drawings[0]["properties"]["name"])
        self.assertEqual(drawings[0]["properties"]["attribution"], RIGHTS["attribution"])
        normalized["rights"] = {**RIGHTS, "allowThematicDisplay": False}
        with self.assertRaisesRegex(ValueError, "allowThematicDisplay"):
            list(thematic_payloads(normalized, "example"))


if __name__ == "__main__":
    unittest.main()
