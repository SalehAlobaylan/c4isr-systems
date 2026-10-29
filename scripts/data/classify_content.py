"""Classify exact installed IDs from retained receipts; never infer from names."""
import argparse
import hashlib
import json
from pathlib import Path

from install_local import API, DATA, RECEIPTS, ROOT


def candidates(api, receipts):
    result = []
    receipt_ref = str(RECEIPTS.relative_to(ROOT)).replace('\\', '/')
    for item in receipts.get('references', []):
        current = api.call('GET', '/reference-datasets/' + item['id'])
        if any(current.get(k) != item.get(k) for k in ['id', 'sha256', 'version', 'datasetKey']):
            raise RuntimeError('Reference receipt does not match the retained version')
        result.append(dict(kind='reference', entityId=item['id'], purpose='working',
                           reason='Exact source reference and hash verified against the retained installation receipt; analytical accuracy and rights remain separately declared.',
                           receiptReference=receipt_ref + '#references'))
    imagery = receipts.get('imagery', {})
    for pair in imagery.get('pair', []):
        capture = api.call('GET', '/imagery/captures/' + pair['captureId'])
        if capture['input']['scene']['id'] != pair['sceneId'] or capture.get('result', {}).get('preparationId') != pair['preparationId']:
            raise RuntimeError('Imagery receipt does not match exact capture inputs/results')
        for kind, key in [('scene', 'sceneId'), ('capture', 'captureId'), ('job', 'preparationId')]:
            result.append(dict(kind=kind, entityId=pair[key], purpose='acceptance',
                               reason='Exact retained historical imagery study used for installation acceptance; original real source provenance is preserved.',
                               receiptReference=receipt_ref + '#imagery/pair'))
    for kind, identifier in [('job', imagery.get('comparisonId')),
                             ('assessment', receipts.get('realProposal', {}).get('assessmentId')),
                             ('package', receipts.get('simulation', {}).get('contextPackageId'))]:
        if identifier:
            result.append(dict(kind=kind, entityId=identifier, purpose='development',
                               reason='Exact prepared development workflow output identified by the retained installation receipt; no operational publication or independent validation is asserted.',
                               receiptReference=receipt_ref))
    inventory = ROOT / 'docs/implementation/MAP_FIRST_CONTENT_PURPOSE_INVENTORY_2026_10_25.json'
    result.extend(json.loads(inventory.read_text(encoding='utf-8'))['entries'])
    # Duplicate scene/capture references retain one explicit classification.
    return list({(item['kind'], item['entityId']): item for item in result}.values())


def run(api, receipts, apply=False):
    planned = candidates(api, receipts)
    current = {}
    for kind in {item['kind'] for item in planned}:
        offset = 0
        while True:
            page = api.call('GET', f'/content-purpose?view=all&kind={kind}&offset={offset}')
            current.update({(item['kind'], item['entityId']): item for item in page['items']})
            if not page['hasMore']:
                break
            offset += len(page['items'])
            if offset > 100000 or not page['items']:
                raise RuntimeError('Classification inventory exceeded its explicit bound')
    results = []
    for item in planned:
        previous = current.get((item['kind'], item['entityId']))
        if previous is None:
            results.append(dict(kind=item['kind'], entityId=item['entityId'], state='NOT_PRESENT_OR_NOT_AUTHORIZED'))
            continue
        if previous['version'] and previous['purpose'] != item['purpose']:
            # An installer must not overwrite later human classification.
            raise RuntimeError('Existing purpose conflicts with the installation inventory; review the exact record')
        if apply:
            value = api.call('POST', '/content-purpose', dict(item, expectedVersion=previous['version']))
            results.append(dict(kind=item['kind'], entityId=item['entityId'], state='CLASSIFIED', version=value['version']))
        else:
            results.append(dict(item, state='PLANNED', expectedVersion=previous['version']))
    return dict(schema='emad.purpose-backfill.v1', applied=apply,
                inventorySha256=hashlib.sha256(json.dumps(planned, sort_keys=True).encode()).hexdigest(), entries=results)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true', help='Append the reviewed classifications through the authorized API')
    args = parser.parse_args()
    receipts = json.loads(RECEIPTS.read_text(encoding='utf-8')) if RECEIPTS.exists() else {}
    report = run(API(), receipts, args.apply)
    DATA.mkdir(parents=True, exist_ok=True)
    (DATA / 'content-purpose-backfill.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
    print(json.dumps({'applied': args.apply, 'records': len(report['entries']),
                      'unavailable': sum(r['state'] == 'NOT_PRESENT_OR_NOT_AUTHORIZED' for r in report['entries'])}))


if __name__ == '__main__':
    main()
