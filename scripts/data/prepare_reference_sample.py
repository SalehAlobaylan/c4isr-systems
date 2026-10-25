"""Rebuild the bounded Jeddah/Makkah reference sample from pinned public data.

Preparation tool only; never runs in the API. Requires numpy, tifffile and
imagecodecs. Network reads are HTTPS, byte-bounded and use fixed source URLs.
The global elevation file is accessed by range; it is never downloaded whole.
"""
import base64
import hashlib
import io
import json
import math
from pathlib import Path
import sys
import urllib.request
from datetime import datetime, timezone

VECTOR = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_50m_populated_places_simple.geojson"
TERRAIN = "https://www.ngdc.noaa.gov/mgg/global/relief/ETOPO2022/data/60s/60s_bed_elev_gtif/ETOPO_2022_v1_60s_N90W180_bed.tif"
AOI = {"west": 39, "south": 21, "east": 40, "north": 22}

def fetch(url, start=None, size=None):
    headers = {"User-Agent": "EMAD-reference-preparation/1"}
    if start is not None:
        headers["Range"] = f"bytes={start}-{start+size-1}"
    with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=20) as r:
        if start is not None and r.status != 206:
            raise ValueError("provider did not honor bounded byte-range access")
        cap = size if size is not None else 2*1024*1024
        raw = r.read(cap+1)
        if len(raw) > cap:
            raise ValueError("provider response exceeded preparation budget")
        return raw

class RangeFile(io.RawIOBase):
    def __init__(self):
        self.pos = 0
        self.total = 0
        self.length = 478386633  # pinned v1 artifact; never use this to fetch the entire file
    def readable(self): return True
    def seekable(self): return True
    def tell(self): return self.pos
    def seek(self, offset, whence=0):
        self.pos = offset if whence == 0 else self.pos+offset if whence == 1 else self.length+offset
        return self.pos
    def read(self, size=-1):
        if size < 0 or size > 1024*1024 or self.total+size > 4*1024*1024:
            raise ValueError("TIFF range budget exceeded")
        if size == 0: return b""
        raw = fetch(TERRAIN, self.pos, size)
        self.pos += len(raw)
        self.total += len(raw)
        return raw

def artifact(raw):
    return {"artifact": base64.b64encode(raw).decode(), "sha256": hashlib.sha256(raw).hexdigest()}

def metadata(key, version, label, kind, producer, reference, terms, attribution):
    return {
        "sourceId": "RETURNED_CANONICAL_SOURCE_ID", "datasetKey": key,
        "version": version, "label": label, "kind": kind,
        "originalProducer": producer, "crs": "EPSG:4326", "aoi": AOI,
        "retrievedAt": datetime.now(timezone.utc).isoformat(),
        "retrievalReference": reference,
        "rights": {"id": key+"-rights-v1", "legalBasis": "Public reference data; reviewed source terms linked below",
                   "termsUrl": terms, "attribution": attribution, "retainRaw": True,
                   "allowAnalysis": True, "allowSimulationExport": True},
    }

def main():
    import numpy as np
    import tifffile
    out = Path(sys.argv[1] if len(sys.argv) > 1 else "testdata/reference/jeddah-makkah")
    out.mkdir(parents=True, exist_ok=True)
    original = fetch(VECTOR)
    features = []
    for f in json.loads(original)["features"]:
        lng, lat = f["geometry"]["coordinates"][:2]
        if 39 <= lng <= 40 and 21 <= lat <= 22:
            f["id"] = str(f["properties"]["ne_id"])
            features.append(f)
    if len(features) < 2: raise ValueError("expected Jeddah and Makkah reference places")
    raw = json.dumps({"type": "FeatureCollection", "features": features}, separators=(",", ":"), ensure_ascii=False).encode()
    vector = metadata("natural-earth-places-jeddah-makkah", "5.1.2-subset-v1", "Jeddah and Makkah reference places", "vector", "Natural Earth", VECTOR,
                      "https://www.naturalearthdata.com/about/terms-of-use/", "Made with Natural Earth, v5.1.2, 1:50 million")
    vector.update(artifact(raw))
    vector.update({"parentArtifactSha256": hashlib.sha256(original).hexdigest(), "verticalDatum": "not_applicable",
                   "resolution": {"value": 50000000, "unit": "map_scale"},
                   "validity": "Static Natural Earth v5.1.2 release; not a current infrastructure survey",
                   "quality": "Generalized place locations at 1:50 million scale; not building extents or routing data",
                   "transformation": "Select original point features inside [39,21,40,22]; preserve source properties and coordinates"})
    terrain = metadata("etopo2022-jeddah-makkah", "2022-v1-60s-subset-v1", "Jeddah/Makkah coarse elevation reference", "elevation", "NOAA NCEI", TERRAIN,
                       "https://coastwatch.pfeg.noaa.gov/erddap/griddap/ETOPO_2022_v1_15s.html", "NOAA NCEI, ETOPO 2022 v1, DOI 10.25921/fd45-gt74")
    remote = RangeFile()
    with tifffile.TiffFile(remote) as tif:
        page = tif.pages[0]
        sx, sy, _ = page.tags[33550].value
        _, _, _, west, north, _ = page.tags[33922].value
        # Keep cell centres inside the AOI; retain them as point samples.
        c0 = math.ceil((39-west)/sx-0.5); c1 = math.floor((40-west)/sx-0.5)
        r0 = math.ceil((north-22)/sy-0.5); r1 = math.floor((north-21)/sy-0.5)
        values = np.full((r1-r0+1, c1-c0+1), np.nan)
        across = math.ceil(page.imagewidth/page.tilewidth)
        for tr in range(r0//page.tilelength, r1//page.tilelength+1):
            for tc in range(c0//page.tilewidth, c1//page.tilewidth+1):
                index = tr*across+tc
                compressed = fetch(TERRAIN, page.dataoffsets[index], page.databytecounts[index])
                remote.total += len(compressed)
                if remote.total > 4*1024*1024: raise ValueError("tile budget exceeded")
                tile, _, _ = page.decode(compressed, index)
                tile = tile.reshape(page.tilelength, page.tilewidth)
                rr0=max(r0,tr*page.tilelength);rr1=min(r1+1,(tr+1)*page.tilelength)
                cc0=max(c0,tc*page.tilewidth);cc1=min(c1+1,(tc+1)*page.tilewidth)
                values[rr0-r0:rr1-r0,cc0-c0:cc1-c0] = tile[rr0-tr*page.tilelength:rr1-tr*page.tilelength,cc0-tc*page.tilewidth:cc1-tc*page.tilewidth]
        grid = {"width": values.shape[1], "height": values.shape[0], "west": west+(c0+0.5)*sx,
                "south": north-(r1+0.5)*sy, "stepX": sx, "stepY": sy, "registration": "point_samples",
                "values": [float(v) if math.isfinite(v) and v != -999999 else None for v in values[::-1].flat]}
    raw = json.dumps(grid, separators=(",", ":"), allow_nan=False).encode()
    terrain.update(artifact(raw))
    terrain.update({"verticalDatum": "EGM2008 orthometric height, metres, positive up",
                    "resolution": {"value": 60, "unit": "arc_seconds"},
                    "validity": "Static ETOPO 2022 v1 compilation; source survey dates vary",
                    "quality": "Coarse global relief, about 1.85 km north-south spacing; includes bathymetry; not local engineering terrain",
                    "transformation": f"Read source TIFF cells rows {r0}:{r1+1}, columns {c0}:{c1+1}; preserve float metre heights and cell-centre coordinates; reverse rows to south-to-north; no vertical conversion"})
    for name, item in (("places",vector),("elevation",terrain)):
        (out/(name+".json")).write_text(json.dumps(item,ensure_ascii=False,indent=2)+"\n",encoding="utf-8",newline="\n")
        print(name, item["sha256"], len(base64.b64decode(item["artifact"])))
    print("terrain source bytes fetched:",remote.total)

if __name__ == "__main__": main()
