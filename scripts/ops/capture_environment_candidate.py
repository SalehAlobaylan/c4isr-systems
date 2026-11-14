"""Capture immutable installed image/configuration and exact release identities."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

import requests

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/"data"))
from check_installed_environment_library import read_range
from intake_environment import LibraryAPI

IMAGE=re.compile(r"sha256:[0-9a-f]{64}\Z")
HASH=re.compile(r"[0-9a-f]{64}\Z")


def docker(*args):
    result=subprocess.run(["docker",*args],check=True,capture_output=True,text=True,timeout=25)
    return result.stdout.strip()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value,sort_keys=True,separators=(",",":"),allow_nan=False).encode()


def container(name):
    data=json.loads(docker("inspect",name))[0]
    if not data["State"]["Running"] or not IMAGE.fullmatch(data["Image"]):
        raise ValueError("Candidate container is stopped or has no immutable image")
    cfg=data["Config"]
    fingerprint={"image":data["Image"],"env":sorted(cfg["Env"]),"command":cfg["Cmd"],"entrypoint":cfg["Entrypoint"],
                 "readonly":data["HostConfig"]["ReadonlyRootfs"],"memoryLimit":data["HostConfig"]["Memory"]}
    return {"name":name,"image":data["Image"],"configurationSha256":sha(canonical(fingerprint)),"startedAt":data["State"]["StartedAt"]}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ("targets","reader","steward","toolchain","output","metrics"):
        parser.add_argument(name,type=Path)
    parser.add_argument("--api",required=True)
    parser.add_argument("--workspace",required=True)
    parser.add_argument("--environment",required=True)
    parser.add_argument("--project",required=True)
    parser.add_argument("--database",default="c4isr")
    parser.add_argument("--http-workers", action="store_true", help="pin the two database-free Compose worker services")
    args=parser.parse_args()
    if not all(re.fullmatch(r"[a-z0-9_-]{1,100}",x) for x in (args.project,args.database)):
        raise ValueError("Invalid candidate project or database name")
    if args.output.exists() or args.metrics.exists() or args.output.resolve()==args.metrics.resolve():
        raise ValueError("Use two new output paths")
    now=datetime.now(timezone.utc).isoformat()
    api=LibraryAPI(args.api,args.reader,args.workspace,args.environment)
    admin=LibraryAPI(args.api,args.steward,args.workspace,args.environment)
    targets=json.loads(args.targets.read_text())
    exact=[]
    for t in targets:
        if not re.fullmatch(r"georelease_[a-z0-9]{26}",t["id"]) or not HASH.fullmatch(t["sha256"]):
            raise ValueError("Malformed release identity")
        release=api.call("GET","/environment-releases/"+t["id"])
        manifest=release["manifest"]
        if release["id"]!=t["id"] or release["manifestSha256"]!=t["sha256"]:
            raise ValueError("Installed release differs from its pin")
        exact.append({**t,"state":release["state"],"areaRevisionId":manifest["areaRevisionId"],"profileId":manifest["profileId"],
                      "toolchain":manifest["toolchainSha256"],"inputSha256":manifest["inputSha256"],
                      "preparedSha256":read_range(api,release["id"]),
                      "admissions":sorted({a["admissionId"] for a in manifest["inputs"]})})
    workers = ("environment-http-1", "environment-http-secondary-1") if args.http_workers else ("environment-library-1", "environment-library-2")
    names = ("api-1", "ui-1", "postgres-1", "garage-1", *workers)
    services={name:container(args.project+"-"+name) for name in names}
    versions=docker("exec",services["postgres-1"]["name"],"psql","-U","c4isr","-d",args.database,"-Atc",
                    "SELECT version_id FROM (SELECT DISTINCT ON(version_id) version_id,is_applied FROM goose_db_version ORDER BY version_id,id DESC) latest WHERE is_applied ORDER BY version_id")
    migrations=[int(n) for n in versions.splitlines() if int(n)>0]
    if not migrations or services[workers[0]]["image"] != services[workers[1]]["image"]:
        raise ValueError("Candidate requires applied migrations and both pinned worker replicas")
    toolchain=args.toolchain.read_text().strip()
    if not HASH.fullmatch(toolchain):
        raise ValueError("Invalid preparation toolchain pin")
    identity={"resource":{"workspace":args.workspace,"environment":args.environment},"toolchain":toolchain,"migrations":migrations,
              "workerTransport":"http" if args.http_workers else "direct",
              "imagesAndConfiguration":{n:{"image":r["image"],"configurationSha256":r["configurationSha256"]} for n,r in services.items()},
              "releasePins":[{k:v for k,v in r.items() if k!="state"} for r in exact]}
    candidate={"schema":"emad.environment-installed-candidate.v1","capturedAt":now,"installationSha256":sha(canonical(identity)),
               "apiImage":services["api-1"]["image"],"uiImage":services["ui-1"]["image"],"workerImage":services[workers[1]]["image"],
               "apiStartedAt":services["api-1"]["startedAt"],"services":services,"identity":identity,"releases":exact,
               "sourceCommit":None,"sourceCommitStatus":"Unrecorded worktree build; observed image identities are authoritative."}
    with requests.get(admin.origin+"/environment-operations/metrics",headers=admin.headers,timeout=12,allow_redirects=False) as res:
        res.raise_for_status()
        series={name:float(value) for name,value in (line.split() for line in res.text.splitlines() if line.startswith("c4isr_environment_"))}
    required = {"due_jobs", "active_leases", "expired_leases", "failed_jobs", "oldest_due_job_seconds",
                "pending_asset_bytes", "retained_asset_bytes", "assets_awaiting_purge"}
    if not all("c4isr_environment_" + key in series for key in required):
        raise ValueError("Incomplete metrics sample")
    metrics={"schema":"emad.environment-storage-metrics.v1","at":datetime.now(timezone.utc).isoformat(),"installationSha256":candidate["installationSha256"],"values":series}
    for path,document in ((args.output,candidate),(args.metrics,metrics)):
        with path.open("x",encoding="utf-8") as out:
            json.dump(document,out,indent=2,allow_nan=False)
    print(json.dumps({"installationSha256":candidate["installationSha256"],"migration":max(migrations),"releases":len(exact),"metrics":series}))


if __name__=="__main__":
    main()
