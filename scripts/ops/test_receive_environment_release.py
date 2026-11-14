import hashlib
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

from receive_environment_release import receive


class ReceiverTest(unittest.TestCase):
    def test_checks_pinned_display_before_publishing_archive(self):
        raw = b"verified display fixture"
        digest = hashlib.sha256(raw).hexdigest()
        release = {"id": "riyadh", "state": "RELEASED", "manifestSha256": "manifest", "manifest": {
            "profileId": "reference-context-v1", "areaRevisionId": "area", "toolchainSha256": "processor",
            "quality": {"physicalAccuracy": "UNKNOWN", "unsupported": []}, "inputs": [],
        }}
        display = {"id": "terrain", "releaseId": "riyadh", "manifestSha256": "manifest", "state": "READY",
                   "sha256": digest, "bytes": len(raw), "recipe": "terrain-glb-v2"}
        for changed in (False, True):
            with self.subTest(changed=changed):
                api = SimpleNamespace(origin="https://receiver.example", headers={}, call=Mock(side_effect=[
                    {"items": [release]}, release, display,
                ]))
                response = Mock(status_code=200, headers={"ETag": f'"{digest}"', "Content-Length": str(len(raw))})
                response.iter_content.return_value = iter([b"x" + raw[1:] if changed else raw])
                with (patch("receive_environment_release.read_range", return_value="archive"),
                      patch("receive_environment_release.requests.get") as stream,
                      patch("receive_environment_release.download", return_value={"preparedBytes": 50, "preparedSha256": "archive"}) as download):
                    stream.return_value.__enter__.return_value = response
                    if changed:
                        with self.assertRaisesRegex(ValueError, "SHA-256 check"):
                            receive(api, "riyadh", "manifest", "archive", Path("prepared.zip"), [("terrain", digest)])
                        download.assert_not_called()
                    else:
                        result = receive(api, "riyadh", "manifest", "archive", Path("prepared.zip"), [("terrain", digest)])
                        self.assertEqual(result["displays"], [{"id": "terrain", "recipe": "terrain-glb-v2", "sha256": digest, "bytes": len(raw)}])
                        download.assert_called_once()
