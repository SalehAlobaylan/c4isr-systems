"""Assemble a fail-closed acceptance ledger for the installed environment."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import math
from pathlib import Path
import re

HASH=re.compile(r"[a-f0-9]{64}\Z")
IMAGE=re.compile(r"sha256:[a-f0-9]{64}\Z")
ROLES={"riyadh-detailed","al-ain-second-frame","avignon-workload"}


def load(path):
    with path.open("rb") as f:
        raw=f.read((16<<20)+1)
    if len(raw)>16<<20:
        raise ValueError("Readiness evidence exceeds its byte budget")
    return json.loads(raw),{"name":path.name,"sha256":hashlib.sha256(raw).hexdigest(),"bytes":len(raw)}


def stamp(value):
    at=datetime.fromisoformat(value)
    if at.tzinfo is None:
        raise ValueError("Evidence timestamps require a timezone")
    return at


def ledger(candidate, soak, restart, checks, now):
    if (candidate["schema"]!="emad.environment-installed-candidate.v1" or
        soak["schema"]!="emad.environment-soak.v1" or restart["schema"]!="emad.environment-worker-restart.v1"):
        raise ValueError("Unsupported installed, soak or restart evidence")
    installed=candidate["releases"]
    targets=[{"id":r["id"],"sha256":r["sha256"]} for r in installed]
    if len(installed)!=3 or len({r["id"] for r in installed})!=3 or {r["role"] for r in installed}!=ROLES:
        raise ValueError("Pin three distinct release roles including Riyadh")
    if not HASH.fullmatch(candidate["installationSha256"]) or not all(HASH.fullmatch(r["sha256"]) for r in installed) or not all(IMAGE.fullmatch(candidate[k]) for k in ("apiImage","uiImage","workerImage")):
        raise ValueError("Missing immutable installed candidate digests")
    identity=candidate["identity"]
    canonical=json.dumps(identity,sort_keys=True,separators=(",",":"),allow_nan=False).encode()
    services=identity["imagesAndConfiguration"]
    transport = identity.get("workerTransport", "direct")
    worker_name = {"direct": "environment-library-2", "http": "environment-http-secondary-1"}.get(transport)
    if worker_name is None:
        raise ValueError("Unsupported environment worker transport")
    if (hashlib.sha256(canonical).hexdigest()!=candidate["installationSha256"] or
        identity["releasePins"]!=[{k:v for k,v in r.items() if k!="state"} for r in installed] or
        any(services[name]["image"]!=candidate[key] for name,key in (("api-1","apiImage"),("ui-1","uiImage"),(worker_name,"workerImage")))):
        raise ValueError("Candidate image and release pins do not match its installation digest")
    start, end=stamp(soak["startedAt"]),stamp(soak["checkedAt"])
    restarted=stamp(restart["workerStartedAfter"])
    before=stamp(restart["workerStartedBefore"])
    api_start=stamp(candidate["apiStartedAt"])
    elapsed=soak["elapsedSeconds"]
    if not math.isfinite(elapsed) or not start<end<=now or not 0<=elapsed<=(end-start).total_seconds()+1:
        raise ValueError("Inconsistent observation clocks")
    gaps_ok=0<=soak["maxSampleGapSeconds"]<=soak["sampleGapLimitSeconds"]<=130 and soak["sampleGaps"]==0
    observed_targets=[{"id":r["id"],"sha256":r["sha256"]} for r in soak["candidate"]]
    generation = soak.get("apiGeneration")
    bound = (isinstance(generation, dict) and soak.get("installationSha256") == candidate["installationSha256"] and
             generation == candidate["services"].get("api-1") and generation.get("startedAt") == candidate["apiStartedAt"] and
             generation.get("image") == candidate["apiImage"] and
             generation.get("configurationSha256") == services["api-1"].get("configurationSha256") and
             HASH.fullmatch(generation.get("configurationSha256") or "") is not None)
    has_binding = "installationSha256" in soak or "apiGeneration" in soak
    soak_ok=(bound or not has_binding) and api_start<=start and observed_targets==targets and soak["readerCount"]==5 and soak["requests"]>=5 and soak["failures"]==0 and gaps_ok and 0<=soak["metadataP95Seconds"]<=1
    complete=elapsed>=86400 and (end-start).total_seconds()>=86400
    current=(now-end).total_seconds()<=soak["sampleGapLimitSeconds"]
    operational="PASS" if soak_ok and complete and soak["state"] in {"SOAK_PASS","PRE_RELEASE_OBSERVATION_PASS"} else (
        "RUNNING" if soak_ok and current and soak["state"]=="RUNNING" else "OPEN")
    installed_worker=candidate["services"][worker_name]
    restart_pass=(start<=restarted<=end and before<restarted and restart["apiImage"]==candidate["apiImage"]
                  and restart["workerImage"]==candidate["workerImage"] and restart["apiStartedAt"]==candidate["apiStartedAt"]
                  and installed_worker["startedAt"]==restart["workerStartedAfter"] and installed_worker["name"]==restart["worker"]
                  and restart["metadataHealthStatus"]==200 and restart["queueBefore"]==0)
    released=all(r["state"]=="RELEASED" and soak["lastObservedStates"].get(r["id"])=="RELEASED" for r in installed)
    gates={"metadataObservation":operational,"workerRestart":"PASS" if restart_pass else "OPEN",
           "releaseDecisions":"PASS" if bound and released and operational=="PASS" and soak["state"]=="SOAK_PASS" else "OPEN"}
    # External evidence only qualifies this candidate if it carries the same
    # installation digest and the full set of matching release inputs.
    accepted={}
    for key in ("twoBuildRecovery","metadataVolume","vectorVolume","storageTrend","pairedRestore","portableRebuild","browser","regionalCoverage","independentReceiver"):
        evidence=checks.get(key,{})
        qualifies=(evidence.get("status")=="PASS" and evidence.get("installationSha256")==candidate["installationSha256"]
                   and evidence.get("targets")==targets and HASH.fullmatch(evidence.get("evidenceSha256","")))
        gates[key]="PASS" if qualifies else "OPEN"
        if qualifies:
            accepted[key]={"evidenceSha256":evidence["evidenceSha256"],"sources":evidence.get("sources",[])}
    return {"schema":"emad.environment-readiness-ledger.v1","asOf":now.isoformat(),"state":"READY" if all(v=="PASS" for v in gates.values()) else "NOT_ACCEPTED",
            "installation":candidate,"gates":gates,"observation":{"startedAt":soak["startedAt"],"checkedAt":soak["checkedAt"],
            "elapsedSeconds":elapsed,"requests":soak["requests"],"failures":soak["failures"],"metadataP95Seconds":soak["metadataP95Seconds"],
            "maxSampleGapSeconds":soak["maxSampleGapSeconds"],"sampleGaps":soak["sampleGaps"],"sourceState":soak["state"],"installationBound":bound},
            "workerRestart":restart if restart_pass else None,"acceptedEvidence":accepted,"open":[k for k,v in gates.items() if v!="PASS"]}


if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__)
    for name in ("candidate","soak","restart","checks","output"):
        p.add_argument(name,type=Path)
    p.add_argument("--storage-trend", type=Path, help="verify the full storage sample stream for this observation")
    args=p.parse_args()
    try:
        rows=[load(getattr(args,k)) for k in ("candidate","soak","restart","checks")]
        candidate, soak, restart, checks = (data for data, _ in rows)
        if args.storage_trend:
            from verify_environment_storage_trend import verify
            with args.storage_trend.open("rb") as stream:
                raw = stream.read((16 << 20) + 1)
            if len(raw) > 16 << 20:
                raise ValueError("Storage evidence exceeds its byte budget")
            checks = {**checks, "storageTrend": {**verify(candidate, soak, [json.loads(line) for line in raw.splitlines()]),
                      "evidenceSha256": hashlib.sha256(raw).hexdigest(), "sources": [args.storage_trend.name]}}
        result=ledger(candidate,soak,restart,checks,datetime.now(timezone.utc))
        result["evidence"]=[identity for _,identity in rows]
        with args.output.open("x",encoding="utf-8") as out:
            json.dump(result,out,indent=2,allow_nan=False)
        print(json.dumps({"state":result["state"],"gates":result["gates"],"observation":result["observation"]}))
    except (OSError,KeyError,ValueError,TypeError) as exc:
        p.exit(2,f"{exc}\n")
