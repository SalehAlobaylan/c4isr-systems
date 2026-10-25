"""Bounded public-source capture and receipts shared by local preparation jobs."""
import hashlib
import json
import os
from pathlib import Path
import struct
import zlib
from datetime import datetime, timezone
from urllib.parse import urlsplit, urljoin
import requests

ROOT = Path(__file__).resolve().parents[2]
DATA = Path(os.environ.get('EMAD_PRIVATE_DIR',str(ROOT/'private/local'))) / 'data'
SAUDI = {'west': 34., 'south': 16., 'east': 56., 'north': 33.}
JEDDAH = {'west': 39.144, 'south': 21.464, 'east': 39.216, 'north': 21.536}
ALLOWED = {'naturalearth.s3.amazonaws.com', 'download.geofabrik.de', 'copernicus-dem-30m.s3.amazonaws.com',
           'cdn.proj.org', 'earth-search.aws.element84.com', 'e84-earth-search-sentinel-data.s3.us-west-2.amazonaws.com'}


def encoded(value):
    return json.dumps(value, separators=(',', ':'), ensure_ascii=False, allow_nan=False).encode()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def now():
    return datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')


def store_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + '.partial')
    temporary.write_bytes(encoded(value))
    temporary.replace(path)


def download(url, name, limit=512 * 1024 * 1024):
    parts = urlsplit(url)
    if parts.scheme != 'https' or parts.hostname not in ALLOWED or parts.username or parts.password or parts.fragment:
        raise ValueError('Unadmitted preparation endpoint')
    target = DATA / 'raw' / name
    receipt = target.with_suffix(target.suffix + '.receipt.json')
    if target.exists() and receipt.exists():
        record = json.loads(receipt.read_text(encoding='utf-8'))
        if record.get('requestedUrl', record['url']) != url or target.stat().st_size != record['bytes'] or sha(target.read_bytes()) != record['sha256']:
            raise ValueError('Retained source integrity mismatch: ' + name)
        return target, record
    target.parent.mkdir(parents=True, exist_ok=True)
    temporary = target.with_suffix(target.suffix + '.partial')
    digest, total = hashlib.sha256(), 0
    response = requests.get(url, stream=True, timeout=(15, 60), allow_redirects=False)
    for _ in range(3):
        if not response.is_redirect:
            break
        destination = urljoin(response.url, response.headers.get('Location', ''))
        redirected = urlsplit(destination)
        response.close()
        if redirected.scheme != 'https' or redirected.hostname != parts.hostname or redirected.port not in (None, 443) or redirected.username or redirected.password or redirected.fragment:
            raise ValueError('Cross-origin preparation redirect rejected')
        response = requests.get(destination, stream=True, timeout=(15, 60), allow_redirects=False)
    with response:
        response.raise_for_status()
        if response.is_redirect:
            raise ValueError('Preparation redirects are not admitted')
        with temporary.open('wb') as output:
            for block in response.iter_content(1024 * 1024):
                total += len(block)
                if total > limit:
                    raise ValueError('Source exceeds preparation budget')
                digest.update(block)
                output.write(block)
        record = {'requestedUrl': url, 'url': response.url, 'sha256': digest.hexdigest(), 'bytes': total, 'retrievedAt': now(),
                  'etag': response.headers.get('ETag'), 'lastModified': response.headers.get('Last-Modified')}
    temporary.replace(target)
    store_json(receipt, record)
    print('Retained ' + name + ' (' + str(total) + ' bytes)', flush=True)
    return target, record


def png(rgb):
    import numpy as np
    a = np.asarray(rgb, dtype=np.uint8)
    height, width, channels = a.shape
    if channels not in (3, 4) or width > 4096 or height > 4096:
        raise ValueError('PNG budget exceeded')
    chunk = lambda kind, body: struct.pack('>I', len(body)) + kind + body + struct.pack('>I', zlib.crc32(kind + body) & 0xffffffff)
    rows = b''.join(b'\0' + row.tobytes() for row in a)
    return b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 2 if channels == 3 else 6, 0, 0, 0)) + chunk(b'IDAT', zlib.compress(rows, 6)) + chunk(b'IEND', b'')


def archive_sources():
    """Retain large original files in bounded private S3 chunks as well as cache."""
    import boto3
    from botocore.exceptions import ClientError
    client = boto3.client('s3', endpoint_url=os.environ['C4ISR_ARTIFACT_S3_ENDPOINT'],
                          region_name=os.environ['C4ISR_ARTIFACT_S3_REGION'],
                          aws_access_key_id=os.environ['C4ISR_ARTIFACT_S3_ACCESS_KEY'],
                          aws_secret_access_key=os.environ['C4ISR_ARTIFACT_S3_SECRET_KEY'])
    bucket = os.environ['C4ISR_ARTIFACT_S3_BUCKET']
    def retain(key, data, media):
        try:
            response = client.get_object(Bucket=bucket, Key=key)
        except ClientError as error:
            if error.response['Error']['Code'] not in ('NoSuchKey', '404'):
                raise
            client.put_object(Bucket=bucket, Key=key, Body=data, ContentType=media)
            response = client.get_object(Bucket=bucket, Key=key)
        with response['Body'] as body:
            existing = body.read(len(data) + 1)
        if len(existing) != len(data) or sha(existing) != sha(data):
            raise ValueError('Retained source archive integrity mismatch; existing bytes were not overwritten: ' + key)
    for path in sorted((DATA / 'raw').glob('*.receipt.json')):
        record = json.loads(path.read_text(encoding='utf-8'))
        source = Path(str(path).removesuffix('.receipt.json'))
        chunks = []
        digest, total = hashlib.sha256(), 0
        with source.open('rb') as file:
            for i in range(100):
                data = file.read(8 * 1024 * 1024)
                if not data:
                    break
                digest.update(data)
                total += len(data)
                key = 'installation/sources/' + record['sha256'] + '/' + str(i)
                retain(key, data, 'application/octet-stream')
                chunks.append({'key': key, 'sha256': sha(data), 'bytes': len(data)})
        if digest.hexdigest() != record['sha256'] or total != record['bytes']:
            raise ValueError('Retained original source no longer matches its acquisition receipt')
        record.update({'archived': True, 'chunks': chunks})
        # Identical source bytes can have distinct acquisition receipts (URLs,
        # ETags or retrieval times). Retain each receipt by its own hash; the
        # original legacy manifest and content-addressed chunks stay immutable.
        record.pop('manifestKey', None)
        record.pop('manifestSha256', None)
        manifest = encoded(record)
        manifest_hash = sha(manifest)
        manifest_key = 'installation/sources/' + record['sha256'] + '/receipts/' + manifest_hash + '.json'
        retain(manifest_key, manifest, 'application/json')
        record.update({'manifestKey': manifest_key, 'manifestSha256': manifest_hash})
        store_json(path, record)
