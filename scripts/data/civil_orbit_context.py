"""Prepare retained NOAA-20 ephemeris context offline; never schedules a capture."""
import argparse
from datetime import datetime, timedelta, timezone
import hashlib
from importlib.metadata import version
import json
import math
from pathlib import Path

SOURCE = "https://celestrak.org/NORAD/elements/gp.php?CATNR=43013&FORMAT=JSON"
TERMS = "https://celestrak.org/usage-policy.php"
SCHEMA = "emad.civil-orbit-context.v1"
SAUDI_REGIONAL = (34, 16, 56, 33)
ENGINES = {"sgp4":"2.25", "skyfield":"1.53", "jplephem":"2.24", "numpy":"2.5.4", "certifi":"2026.10.19"}


def encode(value):
    return json.dumps(value,sort_keys=True,separators=(",",":"),allow_nan=False).encode()


def read(path, budget):
    with path.open("rb") as source:
        data=source.read(budget+1)
    if not data or len(data)>budget:
        raise ValueError("Retained input exceeds its size budget or is empty")
    return data


def timestamp(value):
    parsed=datetime.fromisoformat(value)
    if parsed.tzinfo is None:
        raise ValueError("Supply an explicit time zone")
    return parsed.astimezone(timezone.utc)


def orbital_context(raw, expected, retrieved, start, end, step, bounds=SAUDI_REGIONAL):
    from sgp4.api import SGP4_ERRORS, Satrec, jday
    from sgp4 import omm
    from skyfield.api import EarthSatellite, load, wgs84
    installed={k:version(k) for k in ENGINES}
    if installed!=ENGINES or hashlib.sha256(raw).hexdigest()!=expected:
        raise ValueError("Retained element bytes or propagation engine differ from their pins")
    entry,=json.loads(raw)
    if (entry["NORAD_CAT_ID"],entry["OBJECT_ID"],entry["CLASSIFICATION_TYPE"],entry["EPHEMERIS_TYPE"])!=(43013,"2017-073A","U",0):
        raise ValueError("Only the identified civil NOAA-20 GP element product is supported")
    epoch=datetime.fromisoformat(entry["EPOCH"])
    if epoch.tzinfo is not None:
        raise ValueError("Unexpected CelesTrak UTC epoch notation")
    epoch=epoch.replace(tzinfo=timezone.utc)
    if not 30<=step<=600 or end<=start or (end-start).total_seconds()%step or (end-start).total_seconds()//step+1>240:
        raise ValueError("Use 2–240 equally spaced samples at 30–600 seconds per step")
    if start<epoch-timedelta(days=1) or end>epoch+timedelta(days=1) or end>retrieved or epoch>retrieved:
        raise ValueError("Orbit context unavailable: use a retrospective window within 24 hours of the retained element epoch")
    w,s,e,n=bounds
    if not (-180<=w<e<=180 and -90<=s<n<=90 and e-w<=40 and n-s<=30):
        raise ValueError("Use one bounded regional context rectangle")
    satellite=Satrec()
    omm.initialize(satellite,entry)
    ts=load.timescale(builtin=True)
    earth=EarthSatellite.from_satrec(satellite,ts)
    items=[]
    for i in range(int((end-start).total_seconds()//step)+1):
        at=start+timedelta(seconds=i*step)
        jd,fraction=jday(at.year,at.month,at.day,at.hour,at.minute,at.second+at.microsecond/1e6)
        error,r,v=satellite.sgp4(jd,fraction)
        if error:
            raise ValueError("Propagation unavailable: "+SGP4_ERRORS.get(error,str(error)))
        geo=wgs84.geographic_position_of(earth.at(ts.from_datetime(at)))
        lon,lat,height=geo.longitude.degrees,geo.latitude.degrees,geo.elevation.km
        if not all(math.isfinite(number) for number in (*r,*v,lon,lat,height)):
            raise ValueError("Non-finite orbital sample")
        if w<=lon<=e and s<=lat<=n:
            items.append({"at":at.isoformat(),"temeKm":[round(x,8) for x in r],"temeKmPerSecond":[round(x,10) for x in v],
                          "wgs84":{"longitude":round(lon,8),"latitude":round(lat,8),"ellipsoidHeightKm":round(height,8)}})
    return {"schema":SCHEMA,"use":"Retrospective civil weather satellite subpoint context; predicted sample geometry, no instrument footprint or visibility claim.",
            "source":{"name":"CelesTrak GP JSON","url":SOURCE,"termsUrl":TERMS,"sha256":expected,"bytes":len(raw),
                      "retrievedAt":retrieved.isoformat(),"redistribution":"not_approved_for_this_pilot"},
            "object":{"norad":43013,"internationalDesignator":"2017-073A","name":"NOAA-20 (JPSS-1)","epoch":epoch.isoformat(),
                      "elementSet":entry["ELEMENT_SET_NO"],"revolutionAtEpoch":entry["REV_AT_EPOCH"]},
            "algorithm":{"packages":installed,"gravity":"WGS72","nativeFrame":"TEME","time":"UTC",
                         "displayFrame":"WGS84 geodetic subpoint and ellipsoidal height, using Skyfield bundled time tables",
                         "maxAgeSeconds":86400,"physicalAccuracy":"UNKNOWN"},
            "window":{"from":start.isoformat(),"to":end.isoformat(),"endInclusive":True,"stepSeconds":step,"bounds":list(bounds)},
            "state":"samples_in_region" if items else "no_sample_in_region","samples":items}


def geojson(result):
    return {"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[item["wgs84"]["longitude"],item["wgs84"]["latitude"]]},
             "properties":{"at":item["at"],"meaning":"Computed subpoint sample only"}} for item in result["samples"]]}


def main():
    p=argparse.ArgumentParser(description=__doc__)
    sub=p.add_subparsers(dest="action",required=True)
    for name in ("prepare","verify"):
        q=sub.add_parser(name)
        q.add_argument("elements",type=Path)
        q.add_argument("manifest",type=Path)
        q.add_argument("geojson",type=Path)
        q.add_argument("--sha256",required=True,help="Expected hash of the retained provider response")
        if name=="prepare":
            q.add_argument("--retrieved",required=True,type=timestamp)
            q.add_argument("--from",dest="start",required=True,type=timestamp)
            q.add_argument("--to",dest="end",required=True,type=timestamp)
            q.add_argument("--step",default=60,type=int)
            q.add_argument("--bounds",nargs=4,type=float,default=SAUDI_REGIONAL)
    a=p.parse_args()
    try:
        raw=read(a.elements,65536)
        if a.action=="prepare":
            result=orbital_context(raw,a.sha256,a.retrieved,a.start,a.end,a.step,a.bounds)
            if a.manifest.resolve()==a.geojson.resolve() or a.manifest.exists() or a.geojson.exists():
                raise ValueError("Use two new output paths")
            for path,data in ((a.manifest,encode(result)),(a.geojson,encode(geojson(result)))):
                with path.open("xb") as out: out.write(data)
        else:
            artifact=read(a.manifest,1<<20)
            prior=json.loads(artifact)
            w=prior["window"]
            result=orbital_context(raw,a.sha256,timestamp(prior["source"]["retrievedAt"]),timestamp(w["from"]),timestamp(w["to"]),w["stepSeconds"],w["bounds"])
            if encode(result)!=artifact or encode(geojson(result))!=read(a.geojson,1<<20):
                raise ValueError("Artifact does not reproduce from retained elements and sampling intent")
        print(json.dumps({"state":result["state"],"samples":len(result["samples"]),"sha256":hashlib.sha256(encode(result)).hexdigest()}))
    except (ValueError,KeyError,TypeError,OSError) as exc:
        p.exit(2,f"{exc}\n")


if __name__=="__main__":
    main()
