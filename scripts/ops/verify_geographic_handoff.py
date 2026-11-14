"""Verify a frozen geographic handoff and its locally retained display dependencies."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re

DIGEST = re.compile('[0-9a-f]{64}\\Z')


def digest(path, limit):
    result = hashlib.sha256()
    total = 0
    with path.open('rb') as stream:
        while block := stream.read(256 << 10):
            total += len(block)
            if total > limit:
                raise ValueError(f'Dependency exceeds its byte limit: {path.name}')
            result.update(block)
    return result.hexdigest()


def date(value):
    # Python datetime stores microseconds. Preserve the remaining nanoseconds
    # when comparing windows and deterministic revision order.
    match = re.fullmatch(r'(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?(Z|[+-]\d\d:\d\d)', value)
    if not match:
        raise ValueError('Expected an explicit RFC 3339 timestamp')
    seconds = datetime.fromisoformat(match[1] + match[3]).astimezone(timezone.utc)
    return seconds, int((match[2] or '').ljust(9, '0'))


def verify(file, expected, release, displays):
    if not DIGEST.fullmatch(expected) or digest(file, 4 << 20) != expected:
        raise ValueError('Handoff bytes differ from the trusted SHA-256')
    package = json.loads(file.read_bytes())
    if package['schema'] != 'emad.geographic-context.v1' or package['missingData'] != 'explicit_unavailable':
        raise ValueError('Unsupported geographic handoff schema or missing-data policy')
    if date(package['expiresAt']) <= (datetime.now(timezone.utc).replace(microsecond=0), 0):
        raise ValueError('The handoff dependency lifetime has expired')
    geography = package['geography']
    if digest(release, 32 << 20) != geography['manifestSha256']:
        raise ValueError('Exact geographic release manifest is unavailable')
    if json.loads(release.read_bytes())['areaRevisionId'] != package['scope']['area']['revisionId']:
        raise ValueError('The release and observations use different area revisions')
    for ref in geography['displays']:
        identifier = ref['id']
        if not re.fullmatch('geodisplay_[0-9a-f]{64}', identifier):
            raise ValueError('Invalid display identifier')
        path = displays / identifier
        if not path.is_file() or digest(path, 32 << 20) != ref['sha256']:
            raise ValueError(f'Exact display dependency unavailable: {identifier}')
    window = package['observedWindow']
    start, end = date(window['from']), date(window['to'])
    cutoff = date(package['availabilityCutoff']) if package.get('availabilityCutoff') else None
    temporal = package['scope']['temporal']
    if (temporal['interval'] != window or start >= end or (temporal['revisionView'] == 'known_at') != bool(cutoff)
            or temporal.get('asOf') != package.get('availabilityCutoff') or temporal['revisionView'] not in ('latest', 'known_at')):
        raise ValueError('The occurrence or availability predicate differs from the scoped read')
    terms = {(term['sourceId'], term['admissionId']) for term in package['terms']
             if term['attribution'] and term['termsUrl'] and term['legalBasis']}
    if not terms:
        raise ValueError('Handoff usage terms are missing')
    previous = None
    seen = set()
    for observation in package['observations']:
        timestamp = date(observation['reported']['observedAt'])
        available = date(observation['availableAt'])
        date(observation['retrievedAt'])
        if observation.get('providerUpdatedAt'):
            date(observation['providerUpdatedAt'])
        report = observation.get('providerReport')
        if report:
            history = report['report']['eventHistory']
            if (report['kind'] != 'REPORT' or report['extractionMethod'] != 'nasa-eonet-floods-v1'
                    or history['providerId'] != observation['providerRecordId'] or history['category'] != 'floods'
                    or not 1 <= len(history['geometries']) <= 64 or date(report['from']) != timestamp):
                raise ValueError('Frozen provider report does not bind its original evidence')
            for shape in history['geometries']:
                date(shape['date'])
                if shape['type'] not in ('Point','Polygon','MultiPolygon') or not isinstance(shape['coordinates'], list):
                    raise ValueError('Invalid retained provider geometry')
        key = timestamp, observation['evidenceId']
        if (not start <= timestamp < end or cutoff and available > cutoff or previous and key <= previous
                or key[1] in seen or not DIGEST.fullmatch(observation['contentSha256']) or observation['reported']['withdrawn']
                or (observation['sourceId'], observation['admissionId']) not in terms):
            raise ValueError(f'Invalid frozen observation dependency: {observation["evidenceId"]}')
        previous = key
        seen.add(key[1])
    moving=package.get('moving')
    count=0
    if moving:
        from math import isfinite
        if moving['areaRevisionId']!=package['scope']['area']['revisionId'] or not 1<=len(moving['items'])<=200-len(seen):
            raise ValueError('Frozen moving selection uses an invalid area or size')
        previous=None
        for item in moving['items']:
            observed,available=date(item['observedAt']),date(item['availableAt'])
            date(item['retrievedAt']); date(item['knownAt'])
            key=observed,item['evidenceId']
            position=item['reported']['position']
            if (item['trackId']!=moving['trackId'] or date(item['reported']['observedAt'])!=observed or not start<=observed<end
                    or cutoff and available>cutoff or previous and key<=previous or key[1] in seen
                    or not DIGEST.fullmatch(item['contentSha256']) or (item['sourceId'],item['admissionId']) not in terms
                    or item['decision'] not in ('APPLIED','LATE','QUARANTINED') or item['reported']['kind'] not in ('aircraft','vessel')
                    or not all(isfinite(position[k]) and -lim<=position[k]<=lim for k,lim in (('lng',180),('lat',90)))):
                raise ValueError('Invalid frozen moving revision')
            previous=key
            seen.add(key[1]); count+=1
        prior=start
        for gap in moving['gaps']:
            begin,finish=date(gap['from']),date(gap['to'])
            if not prior<=begin<finish<=end or gap['reason'] not in ('report_gap','quarantined_report','window_edge_unknown'):
                raise ValueError('Invalid frozen moving gap')
            prior=finish
    return {'status': 'PASS', 'handoffId': package['id'], 'releaseId': geography['releaseId'], 'observations': len(seen)-count,
            'movingObservations':count,
            'displays': len(geography['displays']), 'expiresAt': package['expiresAt']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('manifest', type=Path)
    parser.add_argument('--sha256', required=True)
    parser.add_argument('--release-manifest', type=Path, required=True)
    parser.add_argument('--displays', type=Path, required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(verify(args.manifest, args.sha256, args.release_manifest, args.displays)))
    except (KeyError, OSError, ValueError, TypeError) as exc:
        parser.exit(2, f'{exc}\n')


if __name__ == '__main__':
    main()
