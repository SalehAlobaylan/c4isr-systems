"""Inventory retained environment inputs without changing or admitting them.

Reads raw/*.receipt.json and their sibling assets, plus explicit historical
geography manifests. Optional database queries are read-only administrative
inventory, not application authorization or release approval. No downloads.
"""
import argparse
from collections import Counter, defaultdict
from datetime import datetime, timezone
import hashlib
import json
import math
from pathlib import Path
import re
import subprocess
from urllib.parse import urlsplit, urlunsplit


def load(path):
    return json.loads(path.read_text(encoding='utf-8'))


def digest(path):
    value = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            value.update(chunk)
    return value.hexdigest()


def contained(root, path):
    resolved = path.resolve()
    if not resolved.is_relative_to(root):
        raise ValueError('input resolves outside the declared data root')
    return resolved


def public_location(value):
    """Never copy userinfo, signed query parameters or fragments to the report."""
    parsed = urlsplit(value or '')
    if parsed.scheme not in ('http', 'https') or not parsed.hostname:
        return None
    return urlunsplit((parsed.scheme, parsed.hostname, parsed.path, '', ''))


def raster_metadata(path):
    import rasterio
    with rasterio.open(path) as raster:
        nodata = raster.nodata
        if nodata is not None and not math.isfinite(nodata):
            nodata = str(nodata)
        return {'crs': str(raster.crs), 'shape': [raster.height, raster.width],
                'boundsInSourceCRS': list(raster.bounds),
                'pixelSizeInSourceCRS': list(raster.res), 'bands': raster.count,
                'nodata': nodata, 'units': list(raster.units),
                'qualification': 'Header inspection only; no coverage or accuracy certification.'}


def imagery_inventory(data_root):
    root = Path(data_root).resolve(strict=True)
    rows, issues = [], []
    for path in sorted(contained(root, root / 'imagery').glob('*/receipts.json')):
        try:
            receipts = load(contained(root, path))
            scene = load(contained(root, path.parent / 'scene.json'))
            receipt_hash = digest(path)
            if not isinstance(receipts, dict) or not 1 <= len(receipts) <= 32:
                raise ValueError('invalid capture receipt document')
            for band, receipt in sorted(receipts.items()):
                if not re.fullmatch(r'[A-Z0-9]+_[0-9]+m', band):
                    raise ValueError('invalid retained band identity')
                asset = contained(root, path.parent / (band + '.tif'))
                row = {'capture': path.parent.name, 'band': band, 'sourceScene': scene.get('providerId'),
                       'acquiredAt': scene.get('acquiredAt'), 'retrievedAt': receipt.get('retrievedAt'),
                       'parentLocation': public_location(receipt.get('href')), 'parentETag': receipt.get('parentETag'),
                       'receiptSha256': receipt_hash, 'integrity': 'UNVERIFIED'}
                if asset.is_file():
                    row.update(sha256=digest(asset), bytes=asset.stat().st_size)
                    row['integrity'] = ('PASS' if row['sha256'] == receipt.get('captureSha256')
                                        and row['bytes'] == receipt.get('bytes') else 'FAIL')
                if row['integrity'] != 'PASS':
                    issues.append({'code': 'IMAGERY_WINDOW_INTEGRITY', 'capture': row['capture'],
                                   'band': band, 'status': row['integrity']})
                rows.append(row)
        except (OSError, ValueError, TypeError, AttributeError) as error:
            issues.append({'code': 'IMAGERY_RECEIPT_UNAVAILABLE', 'capture': path.parent.name,
                           'errorType': type(error).__name__})
    return {'bands': rows, 'issues': issues, 'verifiedBands': sum(row['integrity'] == 'PASS' for row in rows)}


def inventory(data_root, inspect_rasters=False):
    root = Path(data_root).resolve(strict=True)
    raw_root = contained(root, root / 'raw')
    if not raw_root.is_dir():
        raise ValueError('declared data root has no raw directory')
    result = {'schema': 'emad.environment-input-inventory.v1',
              'recordedAt': datetime.now(timezone.utc).isoformat(),
              'status': 'INVENTORIED_NOT_QUALIFIED',
              'scope': ['raw/*.receipt.json and sibling retained files',
                        'imagery/*/receipts.json and retained bands', 'osm-clipped.json',
                        'prepared-environment.json and its GLB'],
              'assets': [], 'issues': [],
              'limits': ['Does not inventory credentials, backups or remote object storage. Imagery parents are recorded from receipts.',
                         'Matching hashes prove retained-byte integrity, not source authenticity, rights, accuracy or completeness.',
                         'No data is classified, admitted, converted, deleted or released.']}
    groups = defaultdict(list)
    for receipt_path in sorted(raw_root.glob('*.receipt.json')):
        name = receipt_path.name.removesuffix('.receipt.json')
        row = {'path': 'raw/' + name, 'receiptPath': 'raw/' + receipt_path.name}
        try:
            receipt = load(contained(root, receipt_path))
            asset = contained(root, raw_root / name)
            row.update({'receiptSha256': digest(receipt_path),
                        'claimedSha256': receipt.get('sha256'),
                        'claimedBytes': receipt.get('bytes'),
                        'retrievedAt': receipt.get('retrievedAt'),
                        'sourceLocation': public_location(receipt.get('url'))})
            row.update({'sha256': digest(asset), 'bytes': asset.stat().st_size})
            row['integrity'] = 'PASS' if (row['sha256'] == row['claimedSha256'] and
                                                       row['bytes'] == row['claimedBytes']) else 'FAIL'
            groups[row['sha256']].append(row['path'])
            if inspect_rasters and asset.suffix.lower() in ('.tif', '.tiff'):
                row['raster'] = raster_metadata(asset)
        except (OSError, ValueError, KeyError, ImportError) as error:
            row['integrity'] = 'UNVERIFIED'
            row['errorType'] = type(error).__name__
        result['assets'].append(row)
        if row['integrity'] != 'PASS':
            result['issues'].append({'code': 'RAW_INPUT_INTEGRITY', 'path': row['path'],
                                     'status': row['integrity']})
    if not result['assets']:
        result['issues'].append({'code': 'NO_RECEIPTED_RAW_INPUTS'})
    result['imagery'] = imagery_inventory(root)
    result['issues'].extend(result['imagery']['issues'])
    osm_path = contained(root, root / 'osm-clipped.json')
    if osm_path.is_file():
        osm = load(osm_path)
        result['historicalVectors'] = {
            'sha256': digest(osm_path), 'area': osm.get('aoi'),
            'countsBeforeBudget': osm.get('countsBeforeBudget'),
            'retained': {kind: len(osm.get(kind, [])) for kind in ('roads', 'buildings', 'places')},
            'heightBasis': dict(Counter(f.get('properties', {}).get('heightBasis', 'unknown')
                                       for f in osm.get('buildings', []))),
            'limitations': osm.get('limitations', [])}
    manifest_path = contained(root, root / 'prepared-environment.json')
    if manifest_path.is_file():
        manifest = load(manifest_path)
        result['historicalEnvironment'] = {k: manifest[k] for k in
            ('schema', 'aoi', 'frame', 'modelSha256', 'terrainParentSha256', 'geoidSha256',
             'osmParentSha256', 'inputVerticalDatum', 'outputVerticalDatum', 'terrainVertices',
             'buildings', 'roadSegments', 'limitations') if k in manifest}
        model = contained(root, root / 'jeddah-environment.glb')
        actual = digest(model) if model.is_file() else None
        result['historicalEnvironment']['modelIntegrity'] = (
            'PASS' if actual and actual == manifest.get('modelSha256') else 'FAIL')
        if result['historicalEnvironment']['modelIntegrity'] != 'PASS':
            result['issues'].append({'code': 'HISTORICAL_MODEL_INTEGRITY'})
    result['duplicateByteGroups'] = [paths for paths in groups.values() if len(paths) > 1]
    result['summary'] = {'receiptedAssets': len(result['assets']),
                         'verifiedAssets': sum(a['integrity'] == 'PASS' for a in result['assets']),
                         'retainedBytes': sum(a.get('bytes', 0) for a in result['assets']),
                         'uniqueContentHashes': len(groups)}
    return result


def database_inventory(container):
    statement = """
    BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
    SELECT json_build_object(
      'migration', (SELECT max(version_id) FROM goose_db_version WHERE is_applied),
      'referenceDatasets', (SELECT count(*) FROM reference_datasets),
      'referencesWithAccuracy', (SELECT count(*) FROM reference_datasets WHERE document->'accuracy' IS NOT NULL AND document->'accuracy'<>'null'::jsonb),
      'referencesWithEnvironmentPermission', (SELECT count(*) FROM reference_datasets WHERE document#>>'{rights,outputPermissions,geographicEnvironment}'='true'),
      'geographicEditions', (SELECT count(*) FROM geographic_editions),
      'geographicReleases', (SELECT count(*) FROM geographic_releases),
      'imageryScenes', (SELECT count(*) FROM imagery_scenes),
      'environmentDeliveryJobs', (SELECT count(*) FROM delivery_jobs WHERE kind='environment'),
      'deliveryJobKinds', (SELECT coalesce(json_agg(x),'[]') FROM (SELECT kind,state,count(*) AS count FROM delivery_jobs GROUP BY kind,state) x),
      'referenceMetadata', (SELECT coalesce(json_agg(x),'[]') FROM
         (SELECT id,dataset_key AS key,kind,document->>'verticalDatum' AS datum,
          document->>'parentArtifactSha256' AS parent_sha256,
          document->'aoi' AS area,document->'accuracy' AS accuracy,
          document#>'{rights,outputPermissions}' AS output_permissions
          FROM reference_datasets ORDER BY dataset_key) x));
    COMMIT;
    """
    process = subprocess.run(['docker', 'exec', '-i', container, 'psql', '-X', '-qAt',
                              '-U', 'c4isr', '-d', 'c4isr', '-v', 'ON_ERROR_STOP=1'],
                             input=statement, text=True, capture_output=True, check=True, timeout=60)
    result = json.loads(process.stdout)
    result['scope'] = 'Administrative metadata counts across retained scopes; original reference permissions only. No live authorization check.'
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--data-root', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--inspect-rasters', action='store_true')
    parser.add_argument('--additional-imagery-root', type=Path, action='append', default=[])
    parser.add_argument('--database-container', help='Opt-in read-only metadata query against the existing c4isr database')
    args = parser.parse_args()
    if args.output.exists():
        parser.error('output exists; choose a new inventory path')
    report = inventory(args.data_root, args.inspect_rasters)
    report['supplementalImagery'] = [imagery_inventory(root) for root in args.additional_imagery_root]
    for imagery in report['supplementalImagery']:
        report['issues'].extend(imagery['issues'])
    if args.database_container:
        report['database'] = database_inventory(args.database_container)
    serialized = json.dumps(report, indent=2, ensure_ascii=False, allow_nan=False) + '\n'
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open('x', encoding='utf-8') as stream:
        stream.write(serialized)
    print(json.dumps({'status': report['status'], **report['summary'],
                      'integrityIssues': len(report['issues']), 'output': str(args.output)}))
    return int(bool(report['issues']))


if __name__ == '__main__':
    raise SystemExit(main())
