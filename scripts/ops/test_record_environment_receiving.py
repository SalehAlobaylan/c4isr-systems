from copy import deepcopy
import hashlib
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import Mock, patch

from record_environment_receiving import confirmed_receipt, digest, record_receiving, verify_receiving


class IndependentReceivingTest(unittest.TestCase):
    def setUp(self):
        self.releases = [
            {"id": f"georelease_{str(i) * 26}", "role": role, "sha256": str(i) * 64,
             "state": "STAGED", "preparedSha256": "c" * 64}
            for i, role in enumerate(("riyadh-detailed", "al-ain-second-frame", "avignon-workload"), 1)
        ]
        self.release = self.releases[0]
        self.display = f"geodisplay_{'d' * 64}"
        self.identity = {"resource": {"workspace": "fixture", "environment": "staging"},
                         "releasePins": [{k: v for k, v in r.items() if k != "state"} for r in self.releases]}
        self.candidate = {"schema": "emad.environment-installed-candidate.v1", "identity": self.identity,
                          "installationSha256": digest(self.identity), "releases": self.releases}
        self.preflight = {"schema": "emad.staging-release-review-preflight.v1", "status": "PASS",
                          "installationSha256": self.candidate["installationSha256"],
                          "releases": [{"id": r["id"], "role": r["role"], "manifestSha256": r["sha256"]} for r in self.releases]}
        self.preflight["releases"][0].update(manifestLayers=["surface", "roads"], objects=[
            {"path": f"/environment-releases/{self.release['id']}/prepared", "sha256": "c" * 64, "bytes": 100},
            {"path": f"/environment-displays/{self.display}/content", "sha256": "e" * 64, "bytes": 50},
        ])
        self.walkthrough = {"releaseId": self.release["id"], "manifestSha256": self.release["sha256"],
                            "receiverOperatorId": "receiver", "terrainOpened": True, "usableLayers": ["surface"],
                            "limitations": ["Physical accuracy remains unknown."]}
        self.download = {"schema": "emad.environment-receiver-check.v1", "status": "PASS", "checkedAt": "2025-01-01T03:00:00Z",
                         "releaseId": self.release["id"], "manifestSha256": self.release["sha256"],
                         "preparedSha256": "c" * 64, "preparedBytes": 100,
                         "displays": [{"id": self.display, "sha256": "e" * 64, "bytes": 50}]}
        self.session = {"resource": {"workspaceId": "fixture", "environmentId": "staging", "partition": "PUBLIC_EVIDENCE"},
                        "operator": {"id": "receiver"}, "principal": {"kind": "human"},
                        "permissions": ["environments.read", "environments.download"]}
        self.origin = "http://127.0.0.1:8286/api/v1"

    def test_authenticates_receiver_and_pins_every_download(self):
        api = Mock(origin=self.origin)
        api.call.return_value = self.session
        with patch("record_environment_receiving.receive", return_value=self.download) as receive:
            result = record_receiving(api, self.candidate, self.preflight, self.walkthrough, Path("riyadh.zip"))
            api.call.assert_called_once_with("GET", "/auth/me")
            receive.assert_called_once_with(api, self.release["id"], self.release["sha256"], "c" * 64,
                                            Path("riyadh.zip"), [(self.display, "e" * 64)])
            self.assertEqual(result["receiverOperatorId"], "receiver")
            self.session["permissions"].append("environments.release")
            with self.assertRaisesRegex(ValueError, "no review authority"):
                record_receiving(api, self.candidate, self.preflight, self.walkthrough, Path("riyadh.zip"))
            self.assertEqual(receive.call_count, 1)

    def verify_evidence(self, receipt, reviewer="reviewer", last_state="RELEASED"):
        responses = []
        for release in self.releases:
            responses.extend(({"state": last_state, "manifestSha256": release["sha256"]}, [
                {"state": state, "manifestSha256": release["sha256"], "humanId": reviewer,
                 "note": "Reviewed the exact retained package.", "occurredAt": f"2025-01-01T0{i}:00:00Z"}
                for i, state in enumerate(("VERIFIED", "RELEASED"), 1)
            ]))
        return verify_receiving(Mock(origin=self.origin, call=Mock(side_effect=responses)), self.candidate, self.preflight,
                                receipt, Path("receiver.json"))

    def test_accepts_only_exact_and_independent_receiving_evidence(self):
        with patch("record_environment_receiving.receive", return_value=self.download):
            api = Mock(origin=self.origin, call=Mock(return_value=self.session))
            receipt = record_receiving(api, self.candidate, self.preflight, self.walkthrough, Path("riyadh.zip"))
        evidence = self.verify_evidence(receipt)
        self.assertEqual(evidence["independentReceiver"]["status"], "PASS")
        self.assertEqual(evidence["independentReceiver"]["evidenceSha256"], digest(evidence["f03ReviewEvidence"]))
        with self.assertRaisesRegex(ValueError, "different human receiver"):
            self.verify_evidence(receipt, reviewer="receiver")
        with self.assertRaisesRegex(ValueError, "recorded verification and release"):
            self.verify_evidence(receipt, last_state="WITHDRAWN")
        incomplete = deepcopy(receipt)
        incomplete["download"]["displays"] = []
        with self.assertRaisesRegex(ValueError, "every verified display"):
            self.verify_evidence(incomplete)
        different_api = {**receipt, "apiOrigin": "http://127.0.0.1:8498/api/v1"}
        with self.assertRaisesRegex(ValueError, "different installation or API origin"):
            self.verify_evidence(different_api)

    def test_rejects_an_edit_to_receiver_confirmed_bytes(self):
        with TemporaryDirectory() as directory:
            path = Path(directory) / "receiver.json"
            original = b'{"receiverOperatorId":"receiver"}'
            path.write_bytes(original)
            sha256 = hashlib.sha256(original).hexdigest()
            self.assertEqual(confirmed_receipt(path, sha256)["receiverOperatorId"], "receiver")
            path.write_bytes(b'{"receiverOperatorId":"another-operator"}')
            with self.assertRaisesRegex(ValueError, "digest confirmed by the receiver"):
                confirmed_receipt(path, sha256)
