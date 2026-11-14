from datetime import datetime, timedelta, timezone
import hashlib
import json
import unittest

from environment_readiness_ledger import ledger


class InstalledLedgerTests(unittest.TestCase):
    def setUp(self):
        self.start=datetime(2026,11,13,7,tzinfo=timezone.utc)
        self.image="sha256:"+"a"*64
        self.end=self.start+timedelta(days=1)
        pins=[{"id":f"release-{i}","role":role,"sha256":str(i)*64} for i,role in enumerate(("riyadh-detailed","al-ain-second-frame","avignon-workload"),1)]
        images={n:{"image":self.image, "configurationSha256": "f" * 64} for n in ("api-1","ui-1","environment-library-2")}
        identity={"imagesAndConfiguration":images,"releasePins":pins}
        self.digest=hashlib.sha256(json.dumps(identity,sort_keys=True,separators=(",",":")).encode()).hexdigest()
        self.targets=[{"id":r["id"],"sha256":r["sha256"]} for r in pins]
        restarted=(self.start+timedelta(hours=1)).isoformat()
        self.candidate={"schema":"emad.environment-installed-candidate.v1","releases":[{**r,"state":"STAGED"} for r in pins],
                        "installationSha256":self.digest,"identity":identity,"apiImage":self.image,"uiImage":self.image,"workerImage":self.image,
                        "apiStartedAt":self.start.isoformat(),"services":{"environment-library-2":{"name":"worker","startedAt":restarted}}}
        self.restart={"schema":"emad.environment-worker-restart.v1","worker":"worker","workerStartedBefore":self.start.isoformat(),"workerStartedAfter":restarted,
                      "apiImage":self.image,"workerImage":self.image,"apiStartedAt":self.start.isoformat(),"metadataHealthStatus":200,"queueBefore":0}
        self.soak={"schema":"emad.environment-soak.v1","startedAt":self.start.isoformat(),"checkedAt":self.end.isoformat(),"elapsedSeconds":86400,
                   "readerCount":5,"requests":150,"failures":0,"maxSampleGapSeconds":30,"sampleGaps":0,"sampleGapLimitSeconds":70,
                   "metadataP95Seconds":.05,"candidate":self.targets,"lastObservedStates":{r["id"]:"STAGED" for r in pins},"state":"PRE_RELEASE_OBSERVATION_PASS"}
        generation = {"name": "api", "startedAt": self.candidate["apiStartedAt"], **images["api-1"]}
        self.candidate["services"]["api-1"] = generation
        self.soak.update(installationSha256=self.digest, apiGeneration=generation)

    def test_prerelease_cannot_be_mistaken_for_readiness(self):
        result=ledger(self.candidate,self.soak,self.restart,{},self.end)
        self.assertEqual(result["state"],"NOT_ACCEPTED")
        self.assertEqual(result["gates"]["metadataObservation"],"PASS")
        self.assertEqual(result["gates"]["releaseDecisions"],"OPEN")
        self.assertEqual(result["gates"]["workerRestart"],"PASS")

    def test_stale_or_different_candidate_evidence_stays_open(self):
        self.restart["workerImage"]="sha256:"+"b"*64
        self.soak.update(state="RUNNING",checkedAt=(self.end-timedelta(hours=2)).isoformat(),elapsedSeconds=79200)
        checks={"browser":{"status":"PASS","installationSha256":"f"*64,"targets":self.targets,"evidenceSha256":"b"*64}}
        result=ledger(self.candidate,self.soak,self.restart,checks,self.end)
        self.assertEqual(result["gates"]["metadataObservation"],"OPEN")
        self.assertEqual(result["gates"]["workerRestart"],"OPEN")
        self.assertEqual(result["gates"]["browser"],"OPEN")

    def test_http_worker_restart_uses_the_installed_secondary_identity(self):
        identity = self.candidate["identity"]
        identity["workerTransport"] = "http"
        for services in (identity["imagesAndConfiguration"], self.candidate["services"]):
            services["environment-http-secondary-1"] = services.pop("environment-library-2")
        canonical = json.dumps(identity, sort_keys=True, separators=(",", ":")).encode()
        self.candidate["installationSha256"] = hashlib.sha256(canonical).hexdigest()
        self.soak["installationSha256"] = self.candidate["installationSha256"]
        result = ledger(self.candidate, self.soak, self.restart, {}, self.end)
        self.assertEqual(result["gates"]["workerRestart"], "PASS")

    def test_every_exact_gate_is_required(self):
        for r in self.candidate["releases"]:r["state"]="RELEASED"
        self.soak.update(state="SOAK_PASS",lastObservedStates={r["id"]:"RELEASED" for r in self.targets})
        seed=ledger(self.candidate,self.soak,self.restart,{},self.end)
        checks={key:{"status":"PASS","installationSha256":self.digest,"targets":self.targets,"evidenceSha256":"b"*64} for key in seed["open"]}
        self.assertEqual(ledger(self.candidate,self.soak,self.restart,checks,self.end)["state"],"READY")
        self.soak["apiGeneration"] = {**self.soak["apiGeneration"], "name": "different-api"}
        result = ledger(self.candidate, self.soak, self.restart, checks, self.end)
        self.assertEqual(result["gates"]["metadataObservation"], "OPEN")
        self.assertEqual(result["gates"]["releaseDecisions"], "OPEN")
        del self.soak["apiGeneration"], self.soak["installationSha256"]
        result = ledger(self.candidate, self.soak, self.restart, checks, self.end)
        self.assertFalse(result["observation"]["installationBound"])
        self.assertEqual(result["gates"]["metadataObservation"], "PASS")
        self.assertEqual(result["gates"]["releaseDecisions"], "OPEN")
        self.candidate["releases"][0]["sha256"]="c"*64
        with self.assertRaisesRegex(ValueError,"installation digest"):
            ledger(self.candidate,self.soak,self.restart,checks,self.end)


if __name__=="__main__":
    unittest.main()
