"""Prepare a signed-elevation and source-type coastal reference release."""
import argparse
from contextlib import contextmanager
import hashlib
import json
from pathlib import Path
import sys
import tempfile
import zipfile

sys.path.insert(0,str(Path(__file__).resolve().parent))
import coastal_pilot as coast
import environment_library as base

PROFILE="coastal-context-v1"
LAYER="bathymetry"
verify_prepared=base.verify


def toolchain():
    return hashlib.sha256(base.encoded({"base":base.toolchain(),"reader":base.file_digest(Path(coast.__file__))[0],
                                        "processor":base.file_digest(Path(__file__))[0]})).hexdigest()


@contextmanager
def registered():
    previous=base.PROFILE_LAYERS
    base.PROFILE_LAYERS={**previous,PROFILE:{LAYER}}
    try:
        yield
    finally:
        base.PROFILE_LAYERS=previous


def prepare(root):
    from shapely.geometry import box,shape
    recipe=json.loads(base.read_small(root/"recipe.json"))
    if recipe["schema"]!="emad.environment-recipe.v1" or recipe["profile"]["id"]!=PROFILE or recipe["toolchainSha256"]!=toolchain():
        raise ValueError("Unsupported coastal profile or processor digest")
    asset,=recipe["inputs"]
    rep=asset["representation"]
    if not base.ASSET_ID.fullmatch(asset["id"]) or (rep["layer"],rep["format"],rep["productEdition"],rep["crs"],rep["verticalDatum"],rep["semantics"])!=(
            LAYER,"gebco-pair-zip",coast.EDITION,"EPSG:4326",coast.DATUM,"signed-elevation-and-source-type"):
        raise ValueError("The profile requires one retained GEBCO pair and coastline")
    capture=root/"inputs"/asset["id"]
    if base.file_digest(capture,8<<20)!=(asset["sha256"],asset["bytes"]):
        raise ValueError("Coastal capture differs from the recipe")
    doc,summary=coast.inspect(capture)
    area=shape(recipe["area"]["geometry"])
    if area.is_empty or not area.is_valid or not box(*doc["bounds"]).equals(area):
        raise ValueError("Coastal capture must bind the exact selected rectangle")
    frame,error=base.frame_check(area)
    output=root/"coastal-grid.json"
    output.write_bytes(base.encoded({"grid":doc,"summary":summary}))
    manifest={"schema":base.SCHEMA,"profileId":PROFILE,"toolchainSha256":recipe["toolchainSha256"],"inputSha256":recipe["inputSha256"],
              "areaRevisionId":recipe["area"]["revisionId"],"area":recipe["area"]["geometry"],"frame":frame,"inputs":[asset],
              "members":[base.member(output,asset,{"crs":"EPSG:4326","nativeSpacingDegrees":coast.SPACING,"verticalDatum":coast.DATUM},None)],
              "quality":{"coordinateErrorM":error,"physicalAccuracy":"UNKNOWN","meaning":"Regional bathymetry context and separate coastline.",
                         "heightMethod":"Retained signed elevation in metres; no vertical datum conversion or land seam stitching.",
                         "verticalCheck":None,"unsupported":["navigation","port-scale survey","water levels","physical seabed models"],
                         "coastal":{"grid":doc,"summary":summary}}}
    (root/"manifest.json").write_bytes(base.encoded(manifest))
    base.prepared_zip(root/"prepared.zip",root,["coastal-grid.json","manifest.json"])
    return manifest


def verify(path,expected=None):
    with registered():
        result=verify_prepared(path,expected)
    if result["profile"]!=PROFILE:
        raise ValueError("Not a coastal context package")
    with zipfile.ZipFile(path) as z:
        _,member=base.archive_json(z,base.archive_entries(z),"coastal-grid.json")
        if member!=result["quality"]["coastal"] or coast.validate(member["grid"])!=member["summary"]["gridCells"]:
            raise ValueError("Coastal grid or summary differs from its manifest")
    return result


def rebuild(source,output):
    old=base.prepare,base.verify
    base.prepare,base.verify=prepare,verify
    try:
        return base.rebuild(source,output)
    finally:
        base.prepare,base.verify=old


if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__)
    sub=p.add_subparsers(dest="action",required=True)
    sub.add_parser("toolchain")
    sub.add_parser("prepare").add_argument("path",type=Path)
    v=sub.add_parser("verify")
    v.add_argument("path",type=Path)
    v.add_argument("--manifest-sha256")
    r=sub.add_parser("rebuild")
    r.add_argument("path",type=Path)
    r.add_argument("output",type=Path)
    a=p.parse_args()
    try:
        if a.action=="toolchain":
            print(toolchain())
        elif a.action=="prepare":
            prepare(a.path)
            print(json.dumps(verify(a.path/"prepared.zip")))
        elif a.action=="verify":
            print(json.dumps(verify(a.path,a.manifest_sha256)))
        else:
            print(json.dumps(rebuild(a.path,a.output)))
    except (OSError,ValueError,KeyError,TypeError,zipfile.BadZipFile) as exc:
        p.exit(2,f"{exc}\n")
