import hashlib
import json
import shutil
import tempfile
import unittest
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from unittest.mock import patch

import environment_kit as kit
from receive_environment_release import receive_kit

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "testdata" / "geospatial" / "owned-fixture-v1"
DIGESTS = json.loads((FIXTURE / "digests.json").read_text(encoding="utf-8"))
MANIFEST = json.loads((FIXTURE / "manifest.json").read_text(encoding="utf-8"))
RELEASE = {"id": "georelease_" + "a" * 26, "manifestSha256": DIGESTS["manifestSha256"],
           "manifest": MANIFEST, "state": "RELEASED"}
TERMS = {"issuedAt": "2026-01-01T00:00:00Z", "expiresAt": "2027-01-01T00:00:00Z",
         "grants": [{"id": "grant-1", "scope": "read-native", "revoked": False}]}
NOW = "2026-06-01T00:00:00Z"
PREPARED = {"sha256": DIGESTS["preparedSha256"],
            "bytes": (FIXTURE / "prepared.zip").stat().st_size}


def document(displays=(), terms=TERMS):
    return kit.inventory_document(RELEASE, displays, PREPARED,
                                  purpose="native-inspection", terms=terms)


class KitTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)

    def tearDown(self):
        self.temporary.cleanup()

    def publish(self, displays=(), sources=None, terms=TERMS):
        document_value = document(displays, terms)
        output = self.root / "kit"
        kit.publish(output, document_value, FIXTURE / "prepared.zip", sources or {})
        return output, kit.digest(document_value)

    def test_publish_and_verify_without_network_or_viewer(self):
        output, trusted = self.publish()
        result = kit.verify(output, trusted, NOW)
        self.assertEqual(result["status"], "VERIFIED_KIT")
        self.assertEqual(result["inventorySha256"], trusted)
        self.assertEqual(result["archive"]["status"], "VERIFIED_PACKAGE")
        self.assertEqual(result["release"]["manifestSha256"], DIGESTS["manifestSha256"])
        self.assertEqual(result["inventory"]["profileId"], "reference-surface-v1")
        self.assertFalse((output / "manifest.json").exists())

    def test_native_sample_and_point_use_the_committed_fixture_answers(self):
        output, trusted = self.publish()
        sample = kit.native_read(output, "raster-00-00.tif", 10, 10, now=NOW,
                                 inventory_sha=trusted)
        self.assertEqual(sample["outcome"], "value")
        self.assertEqual(sample["value"], 0.0)
        point = kit.native_point(output, "surface", 46.73, 24.7055, component=0,
                                 now=NOW, inventory_sha=trusted)
        self.assertEqual(point["value"], -10.0)
        self.assertEqual(point["member"], "raster-01-00.tif")

    def test_selected_display_bytes_are_saved_and_bound(self):
        payload = b"owned display bytes"
        source = self.root / "display.glb"
        source.write_bytes(payload)
        display = {"id": "geodisplay_" + "b" * 64, "recipe": "terrain-glb-v2",
                   "sha256": hashlib.sha256(payload).hexdigest(), "bytes": len(payload),
                   "path": "displays/terrain.glb"}
        output, trusted = self.publish([display], {display["id"]: source})
        self.assertEqual((output / "displays" / "terrain.glb").read_bytes(), payload)
        self.assertEqual(kit.verify(output, trusted, NOW)["displays"],
                         [{"id": display["id"], "recipe": display["recipe"],
                           "sha256": display["sha256"], "bytes": len(payload)}])

    def test_display_purpose_and_limitations_are_carried_in_the_inventory(self):
        payload = b"owned display bytes"
        source = self.root / "display.glb"
        source.write_bytes(payload)
        display = {"id": "geodisplay_" + "b" * 64, "recipe": "terrain-glb-v2",
                   "sha256": hashlib.sha256(payload).hexdigest(), "bytes": len(payload),
                   "purpose": "visual-reference",
                   "purposeMeaning": "Rendered visual reference.",
                   "limitations": ["Visual reference only."]}
        output, trusted = self.publish([display], {display["id"]: source})
        checked = kit.verify(output, trusted, NOW)["displays"]
        self.assertEqual(checked[0]["purpose"], "visual-reference")
        self.assertEqual(checked[0]["purposeMeaning"], "Rendered visual reference.")
        self.assertEqual(checked[0]["limitations"], ["Visual reference only."])

    def test_concurrent_verifications_keep_their_own_inventory(self):
        documents = []
        for purpose in ("native-inspection", "native-analysis"):
            value = kit.inventory_document(RELEASE, (), PREPARED, purpose=purpose, terms=TERMS)
            documents.append(value)
        outputs = [self.root / purpose for purpose in ("first", "second")]
        for output, value in zip(outputs, documents):
            kit.publish(output, value, FIXTURE / "prepared.zip")
        with ThreadPoolExecutor(max_workers=2) as pool:
            results = list(pool.map(
                lambda pair: kit.verify(pair[0], kit.digest(pair[1]), NOW),
                zip(outputs, documents)))
        self.assertEqual([item["purpose"] for item in results],
                         ["native-inspection", "native-analysis"])
        self.assertEqual([item["inventorySha256"] for item in results],
                         [kit.digest(value) for value in documents])
        self.assertTrue(all(item["archive"]["status"] == "VERIFIED_PACKAGE" for item in results))

    def test_missing_selected_display_leaves_no_published_directory(self):
        display = {"id": "geodisplay_" + "b" * 64, "recipe": "terrain-glb-v2",
                   "sha256": "c" * 64, "bytes": 3, "path": "displays/terrain.glb"}
        output = self.root / "kit"
        with self.assertRaisesRegex(kit.KitError, "not available"):
            kit.publish(output, document([display]), FIXTURE / "prepared.zip")
        self.assertFalse(output.exists())
        self.assertEqual(list(self.root.glob(".kit.partial-*")), [])

    def test_corrupt_prepared_archive_is_refused(self):
        output, trusted = self.publish()
        with (output / "prepared.zip").open("r+b") as stream:
            stream.seek(100)
            stream.write(b"changed")
        with self.assertRaisesRegex(kit.KitError, "prepared archive digest changed"):
            kit.verify(output, trusted, NOW)

    def test_changed_identity_is_refused(self):
        output, trusted = self.publish()
        with self.assertRaisesRegex(kit.KitError, "independently supplied SHA-256"):
            kit.verify(output, "f" * 64, NOW)

    def test_expired_and_unknown_expiry_are_specific(self):
        output, trusted = self.publish()
        with self.assertRaisesRegex(kit.KitError, "expired"):
            kit.verify(output, trusted, "2028-01-01T00:00:00Z")
        with self.assertRaisesRegex(kit.KitError, "supply the evaluation instant"):
            kit.verify(output, trusted)

    def test_invalid_expiry_instant_is_a_specific_failure(self):
        output, trusted = self.publish(terms={**TERMS, "expiresAt": "soon"})
        with self.assertRaisesRegex(kit.KitError, "recognized instant"):
            kit.verify(output, trusted, NOW)

    def test_unknown_schema_and_reader_contract_are_refused(self):
        output, _ = self.publish()
        inventory = json.loads((output / "inventory.json").read_text(encoding="utf-8"))
        for change, message in (({"schema": "other"}, "schema"),
                                ({"reader": {"contract": "other"}}, "reader contract")):
            edited = {**inventory, **change}
            (output / "inventory.json").write_text(json.dumps(edited), encoding="utf-8")
            trusted = hashlib.sha256((output / "inventory.json").read_bytes()).hexdigest()
            with self.assertRaisesRegex(kit.KitError, message):
                kit.verify(output, trusted, NOW)

    def test_missing_inventory_is_an_interrupted_publish(self):
        output = self.root / "kit"
        output.mkdir()
        with self.assertRaisesRegex(kit.KitError, "no published inventory"):
            kit.verify(output, "a" * 64, NOW)

    def test_existing_output_is_not_overwritten(self):
        output, _ = self.publish()
        with self.assertRaisesRegex(kit.KitError, "already exists"):
            kit.publish(output, document(), FIXTURE / "prepared.zip")

    def test_terms_without_the_operation_grant_are_refused(self):
        terms = {"issuedAt": "2026-01-01T00:00:00Z", "expiresAt": "2027-01-01T00:00:00Z",
                 "grants": [{"id": "grant-2", "scope": "publish", "revoked": False}]}
        output, trusted = self.publish(terms=terms)
        with self.assertRaisesRegex(kit.KitError, "do not grant"):
            kit.verify(output, trusted, NOW)

    def test_receiver_publishes_and_proves_a_kit(self):
        displays = []
        expected_document = document(displays)
        trusted = kit.digest(expected_document)
        output = self.root / "received-kit"

        def fake_download(api, release_id, destination, manifest_digest, digest):
            shutil.copyfile(FIXTURE / "prepared.zip", destination)
            return {"preparedSha256": DIGESTS["preparedSha256"],
                    "preparedBytes": (FIXTURE / "prepared.zip").stat().st_size}

        with (patch("receive_environment_release.discover", return_value=RELEASE),
              patch("receive_environment_release.read_range",
                    return_value=DIGESTS["preparedSha256"]),
              patch("receive_environment_release.download", side_effect=fake_download)):
            result = receive_kit(None, RELEASE["id"], DIGESTS["manifestSha256"],
                                 DIGESTS["preparedSha256"], output, [], trusted,
                                 "native-inspection", TERMS, NOW,
                                 ("raster-00-00.tif", 10, 10),
                                 ("roads", "osm-way-1001"))
        self.assertEqual(result["status"], "PASS")
        self.assertFalse(result["resumed"])
        self.assertEqual(result["kit"]["status"], "VERIFIED_KIT")
        self.assertEqual(result["sample"]["value"], 0.0)
        self.assertEqual(result["feature"]["sourceId"], "osm-way-1001")
        self.assertTrue((output / "inventory.json").is_file())

    def test_receiver_resumes_an_existing_verified_kit(self):
        output, trusted = self.publish()
        with (patch("receive_environment_release.discover", return_value=RELEASE),
              patch("receive_environment_release.read_range") as range_check):
            result = receive_kit(None, RELEASE["id"], DIGESTS["manifestSha256"],
                                 DIGESTS["preparedSha256"], output, [], trusted,
                                 "native-inspection", TERMS, NOW,
                                 ("raster-00-00.tif", 10, 10))
        self.assertTrue(result["resumed"])
        range_check.assert_not_called()

    def test_receiver_refuses_expired_terms_before_downloading(self):
        output = self.root / "kit"
        with (patch("receive_environment_release.discover", return_value=RELEASE),
              patch("receive_environment_release.read_range") as range_check):
            with self.assertRaisesRegex(ValueError, "expired"):
                receive_kit(None, RELEASE["id"], DIGESTS["manifestSha256"],
                            DIGESTS["preparedSha256"], output, [], "a" * 64,
                            "native-inspection", TERMS, "2028-01-01T00:00:00Z",
                            ("raster-00-00.tif", 10, 10))
        range_check.assert_not_called()
        self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
