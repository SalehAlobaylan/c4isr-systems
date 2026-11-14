"""Extract bounded GHS-OBAT point attributes without assigning them to OSM footprints."""
import argparse
import csv
import hashlib
import io
import json
import math
from pathlib import Path
import re
import zipfile

ID = re.compile(r"[0-9a-f]{32}\Z")
EDITION = "GHS-OBAT R2024A V1.0"


def retain(archive, expected, bounds, output, receipt):
    with archive.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    if digest != expected:
        raise ValueError("Publisher archive does not match the pinned SHA-256")
    west, south, east, north = bounds
    if not (-180 <= west < east <= 180 and -85 <= south < north <= 85 and east - west <= 1 and north - south <= 1):
        raise ValueError("Use a bounded study area")
    with zipfile.ZipFile(archive) as package:
        files = [m for m in package.infolist() if re.fullmatch(r"GHS_OBAT_CSV_[A-Z]{3}_E2020_R2024A_V1_0\.csv", m.filename)]
        if len(files) != 1 or files[0].file_size > 2 << 30:
            raise ValueError("Unexpected GHS-OBAT archive")
        features, seen = [], set()
        scanned = 0
        with io.TextIOWrapper(package.open(files[0]), encoding="utf-8", newline="") as stream:
            data = csv.DictReader(stream)
            if not {"id", "lon", "lat", "height"} <= set(data.fieldnames or []):
                raise ValueError("Missing GHS-OBAT point or height fields")
            for row in data:
                scanned += 1
                lon, lat = float(row["lon"]), float(row["lat"])
                if not (west <= lon <= east and south <= lat <= north):
                    continue
                key = row["id"]
                if key in seen or not ID.fullmatch(key) or len(seen) >= 30000:
                    raise ValueError("Duplicate, malformed or excessive GHS-OBAT identifiers")
                seen.add(key)
                height = float(row["height"])
                if not math.isnan(height) and not (0 < height <= 2000):
                    raise ValueError("Unexpected GHS-OBAT height value")
                features.append({"type": "Feature", "geometry": {"type": "Point", "coordinates": [lon, lat]},
                                 "properties": {"sourceId": key, "height": row["height"]}})
        features.sort(key=lambda feature: feature["properties"]["sourceId"])
        if not features:
            raise ValueError("No GHS-OBAT centroids in the requested study area")
        encoded = json.dumps({"type": "FeatureCollection", "features": features}, sort_keys=True,
                             separators=(",", ":"), allow_nan=False).encode()
        with output.open("xb") as stream:
            stream.write(encoded)
        summary = {"sourceEdition": EDITION, "archiveSHA256": digest, "archiveBytes": archive.stat().st_size,
                   "member": files[0].filename, "scannedRows": scanned, "bounds": bounds, "selectedCentroids": len(features),
                   "modeledHeights": sum(f["properties"]["height"] != "NaN" for f in features),
                   "retainedSHA256": hashlib.sha256(encoded).hexdigest(), "retainedBytes": len(encoded),
                   "method": "Inclusive native centroid selection. Heights are modeled from gridded data, without matching to OSM."}
        with receipt.open("x", encoding="utf-8") as stream:
            json.dump(summary, stream, indent=2)
        return summary


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archive", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("receipt", type=Path)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--bounds", type=float, nargs=4, required=True)
    args = parser.parse_args()
    print(json.dumps(retain(args.archive, args.sha256, args.bounds, args.output, args.receipt)))
