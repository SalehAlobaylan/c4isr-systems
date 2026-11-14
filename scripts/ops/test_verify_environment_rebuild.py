import hashlib
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

from verify_environment_rebuild import check_bundle


class PortableRebuildInputTest(unittest.TestCase):
    def test_preflight_rejects_invalid_members_before_reconstruction(self):
        payload = b"retained payload"
        digest = hashlib.sha256(payload).hexdigest()
        asset = {"id": "geoasset_" + "b" * 64, "sha256": digest, "bytes": len(payload)}
        area = {"type": "Polygon", "coordinates": [[[1, 1], [2, 1], [2, 2], [1, 1]]]}
        shared = {"inputSha256": "a" * 64, "toolchainSha256": "c" * 64, "inputs": [asset]}
        manifest = {**shared, "schema": "emad.environment-release.v1", "profileId": "reference-surface-v1",
                    "areaRevisionId": "area-test", "area": area}
        recipe = {**shared, "schema": "emad.environment-recipe.v1", "profile": {"id": manifest["profileId"]},
                  "area": {"revisionId": "area-test", "geometry": area}}
        raw = json.dumps(manifest).encode()
        expected = hashlib.sha256(raw).hexdigest()
        path = "inputs/" + digest
        for name, members, pin, reason in (
            ("valid", {path: payload}, expected, None),
            ("incorrect digest", {path: payload}, "0" * 64, "selected manifest"),
            ("missing", {}, expected, "Missing retained input"),
            ("corrupt", {path: b"corrupt payload!"[:len(payload)]}, expected, "hash differs"),
            ("extra", {path: payload, "unexpected": b"x"}, expected, "Unexpected member"),
            ("unsafe path", {path: payload, "../escape": b"x"}, expected, "Unexpected member"),
        ):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                bundle = Path(tmp) / "bundle.zip"
                with zipfile.ZipFile(bundle, "w") as archive:
                    archive.writestr("recipe.json", json.dumps(recipe))
                    archive.writestr("manifest.json", raw)
                    for member, content in members.items():
                        archive.writestr(member, content)
                with zipfile.ZipFile(bundle) as archive:
                    if reason:
                        with self.assertRaisesRegex(ValueError, reason):
                            check_bundle(archive, pin)
                    else:
                        self.assertEqual(check_bundle(archive, pin), manifest)


if __name__ == "__main__":
    unittest.main()
