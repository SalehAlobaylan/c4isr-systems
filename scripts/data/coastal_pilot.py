"""Retain and validate paired native GEBCO 2026 cells for a bounded coastal area."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import math
from pathlib import Path
import re
import zipfile
from urllib.request import urlopen

BASE = "https://dap.ceda.ac.uk/thredds/dodsC/bodc/gebco/global/gebco_2026/"
DATA = {"elevation": "ice_surface_elevation/netcdf/GEBCO_2026.nc", "tid": "type_identifier_grid/netcdf/gebco_2026_tid.nc"}
EDITION = "GEBCO_2026"
SPACING = 1 / 240
PILOT = (49.48, 26.72, 50.08, 27.04)
MAX_CELLS = 65536
TYPES = {0,10,11,12,13,14,15,16,17,40,41,42,43,44,45,46,47,48,70,71,72,127}
TID_FILL = 127
DATUM = "GEBCO mixed sources; nominal mean sea level"
COAST_SHA256 = "bfa04cdbcbef07ef90dfca1dabb48062eca29900a113df0f389303e255484017"
COAST_VERSION = "5.0.0-pre9"


def limits(bounds):
    w,s,e,n=bounds
    if not (-180<=w<e<=180 and -90<=s<n<=90 and e-w<=2 and n-s<=2):
        raise ValueError("Use a bounded coastal rectangle")
    x0,y0=[math.ceil((v-origin)*240-.5) for v,origin in ((w,-180),(s,-90))]
    x1,y1=[math.floor((v-origin)*240-.5) for v,origin in ((e,-180),(n,-90))]
    if (x1-x0+1)*(y1-y0+1)>MAX_CELLS or x1-x0<1 or y1-y0<1:
        raise ValueError("Coastal native sample budget exceeded")
    return x0,y0,x1,y1


def parse(raw, name, window):
    x0,y0,x1,y1=window
    width,height=x1-x0+1,y1-y0+1
    content=raw.decode("ascii")
    section=content.split("---------------------------------------------\n",1)
    if len(section)!=2 or DATA[name] not in section[0]:
        raise ValueError("Unexpected CEDA dataset response")
    parts=re.split(rf"(?m)^{name}\.(?:{name}|lat|lon)\[.*?\]\s*\n",section[1])
    if len(parts)!=4:
        raise ValueError("Unexpected CEDA array labels")
    values=[]
    for i,row in enumerate(parts[1].strip().splitlines()):
        index,*cells=row.split(",")
        if index!=f"[{i}]" or len(cells)!=width:
            raise ValueError("Unexpected CEDA row")
        values.extend(map(int,cells))
    if len(values)!=width*height:
        raise ValueError("Unexpected CEDA grid shape")
    lat,lon=([float(v) for v in part.strip().split(",")] for part in parts[2:])
    for axis,start,length,origin in ((lat,y0,height,-90),(lon,x0,width,-180)):
        if len(axis)!=length or any(abs(v-(origin+(start+i+.5)/240))>1e-9 or not math.isfinite(v) for i,v in enumerate(axis)):
            raise ValueError("Grid coordinates differ from native pixel centres")
    return values


def validate(doc):
    if (doc["schema"],doc["edition"],doc["datum"],doc["spacingDegrees"],doc["registration"]) != (
            "emad.gebco-pair.v1",EDITION,DATUM,SPACING,"pixel-centre"):
        raise ValueError("Unsupported bathymetry representation")
    w=limits(doc["bounds"])
    if list(w)!=doc["window"]:
        raise ValueError("Bathymetry grid bounds and native window differ")
    size=(w[2]-w[0]+1)*(w[3]-w[1]+1)
    if any(len(doc[k])!=size for k in DATA) or any(type(v) is not int or not -12000<=v<=9000 for v in doc["elevation"]) or (
            any(type(v) is not int or v not in TYPES for v in doc["tid"])):
        raise ValueError("Unexpected elevation or source type")
    if not any(v<0 and t not in (0,TID_FILL) for v,t in zip(doc["elevation"],doc["tid"])):
        raise ValueError("No retained negative seabed samples")
    if set(doc["parents"])!=set(DATA) or any(not re.fullmatch(r"[0-9a-f]{64}",doc["parents"][k]) for k in DATA):
        raise ValueError("Both native source parents must be pinned")
    return size


def acquire(root,bounds):
    root.mkdir(parents=True,exist_ok=True)
    x0,y0,x1,y1=window=limits(bounds)
    parents={}
    doc=dict(schema="emad.gebco-pair.v1",edition=EDITION,datum=DATUM,spacingDegrees=SPACING,
             registration="pixel-centre",window=list(window),bounds=list(bounds),parents={})
    for name,file in DATA.items():
        query=f"{BASE}{file}.ascii?{name}%5B{y0}:1:{y1}%5D%5B{x0}:1:{x1}%5D"
        with urlopen(query,timeout=35) as response:
            raw=response.read((2<<20)+1)
            if response.status!=200 or len(raw)>2<<20 or response.url!=query:
                raise ValueError("Unusable bounded CEDA response")
        doc[name]=parse(raw,name,window)
        with (root/f"gebco-{name}.ascii").open("xb") as out:
            out.write(raw)
        digest=hashlib.sha256(raw).hexdigest()
        doc["parents"][name]=digest
        parents[name]=dict(url=query,sha256=digest,bytes=len(raw))
    count=validate(doc)
    raw=json.dumps(doc,sort_keys=True,separators=(",",":"),allow_nan=False).encode()
    with (root/"gebco-pair.json").open("xb") as out:
        out.write(raw)
    receipt=dict(edition=EDITION,parents=parents,window=window,bounds=bounds,selectedCells=count,
                 retrievedAt=datetime.now(timezone.utc).isoformat(),sha256=hashlib.sha256(raw).hexdigest(),bytes=len(raw))
    with (root/"gebco-receipt.json").open("x",encoding="utf-8") as out:
        json.dump(receipt,out,indent=2)
    return receipt


def bundle(root, coast, expected, output):
    """Bind exact provider responses and the independently sourced coastline."""
    raw=coast.read_bytes()
    if len(raw)>8<<20 or hashlib.sha256(raw).hexdigest()!=expected:
        raise ValueError("Coastline archive differs from its pin")
    members={"coast.zip":raw}
    for name in DATA:
        members[f"{name}.ascii"]=(root/f"gebco-{name}.ascii").read_bytes()
    members["pair.json"]=(root/"gebco-pair.json").read_bytes()
    with zipfile.ZipFile(output,"x",compression=zipfile.ZIP_DEFLATED) as z:
        for name,value in members.items():
            z.writestr(zipfile.ZipInfo(name),value,compress_type=zipfile.ZIP_DEFLATED)


def inspect(path):
    import fiona
    from shapely.geometry import LineString, MultiLineString, Point, box, shape
    with zipfile.ZipFile(path) as z:
        entries=z.infolist()
        if len(entries)!=4 or {x.filename for x in entries}!={"coast.zip","elevation.ascii","tid.ascii","pair.json"} or any(
                x.is_dir() or x.file_size<1 or x.file_size>8<<20 for x in entries):
            raise ValueError("Unexpected coastal capture members")
        doc=json.loads(z.read("pair.json"))
        size=validate(doc)
        for name in DATA:
            raw=z.read(name+".ascii")
            if hashlib.sha256(raw).hexdigest()!=doc["parents"][name] or parse(raw,name,doc["window"])!=doc[name]:
                raise ValueError("Canonical cells differ from exact provider responses")
        coast=z.read("coast.zip")
        if hashlib.sha256(coast).hexdigest()!=COAST_SHA256:
            raise ValueError("Coastline archive differs from its profile pin")
    import tempfile
    from collections import Counter
    lines=[]
    with tempfile.TemporaryDirectory(prefix="emad-coast-") as temp:
        coast_path=Path(temp)/"coast.zip"
        coast_path.write_bytes(coast)
        with zipfile.ZipFile(coast_path) as z:
            if len(z.infolist())>12 or sum(x.file_size for x in z.infolist())>32<<20 or "ne_10m_coastline.VERSION.txt" not in z.namelist():
                raise ValueError("Unexpected coastline archive")
            version=z.read("ne_10m_coastline.VERSION.txt").decode().strip()
        if version!=COAST_VERSION:
            raise ValueError("Unexpected Natural Earth coastline edition")
        with fiona.open("zip://"+coast_path.as_posix(),enabled_drivers=["ESRI Shapefile"]) as src:
            if src.crs.to_epsg()!=4326:
                raise ValueError("Unexpected coastline CRS")
            area=box(*doc["bounds"])
            for feature in src.filter(bbox=area.bounds):
                g=shape(feature.geometry).intersection(area)
                if g.is_empty:
                    continue
                parts=[g] if isinstance(g,LineString) else [x for x in getattr(g,"geoms",[]) if isinstance(x,LineString)]
                lines.extend([list(map(list,x.coords)) for x in parts])
    if not lines or sum(map(len,lines))>5000:
        raise ValueError("Missing or excessive coastline geometry")
    geometry=MultiLineString(lines)
    counts=Counter()
    near=Counter(cells=0,negative=0,tidLandNegative=0,nonLandNonnegative=0,missingSourceType=0)
    lo=0
    x0,y0,x1,_=doc["window"]
    width=x1-x0+1
    for i,(e,t) in enumerate(zip(doc["elevation"],doc["tid"])):
        counts[str(t)]+=1
        if e<0 and t not in (0,TID_FILL):
            lo=min(lo,e)
        x,y=i%width,i//width
        if geometry.distance(Point(-180+(x0+x+.5)/240,-90+(y0+y+.5)/240))<=SPACING:
            near["cells"]+=1
            near["negative"]+=e<0
            near["tidLandNegative"]+=e<0 and t==0
            near["nonLandNonnegative"]+=e>=0 and t not in (0,TID_FILL)
            near["missingSourceType"]+=t==TID_FILL
    summary={"bounds":doc["bounds"],"window":doc["window"],"gridCells":size,"spacingDegrees":SPACING,"edition":EDITION,
             "datum":DATUM,"minimumElevationM":lo,"sourceTypes":dict(counts),"missingSourceType":counts[str(TID_FILL)],"nearCoast":dict(near),"coastVersion":version,
             "coastSha256":hashlib.sha256(coast).hexdigest(),"coastline":lines,"parents":doc["parents"]}
    return doc,summary


if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__)
    sub=p.add_subparsers(dest="action",required=True)
    a=sub.add_parser("acquire")
    a.add_argument("output",type=Path)
    a.add_argument("--bounds",nargs=4,type=float,default=PILOT)
    b=sub.add_parser("bundle")
    for key in ("source","coast","output"):
        b.add_argument(key,type=Path)
    a=p.parse_args()
    if a.action=="acquire":
        print(json.dumps(acquire(a.output,a.bounds)))
    else:
        bundle(a.source,a.coast,COAST_SHA256,a.output)
        _,summary=inspect(a.output)
        summary.pop("coastline")
        print(json.dumps(summary))
