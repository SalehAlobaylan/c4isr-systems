"""Execute the declared MAP-6 required-recovery rehearsals in isolation.

Each rehearsal names a concrete executable check. A rehearsal that could not be
executed is recorded NOT_RUN with the missing instrument; a rehearsal that ran
and failed is FAIL. Nothing here promotes a gate, and no rehearsal touches a
shared installation: every check runs against disposable Testcontainers
services or the caller's own isolated workspace.

The result file is consumed by ``qualify_map_first.py --recovery``.
"""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import platform
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
PROFILE = Path(__file__).with_name('map_first_qualification_profile.json')
SCHEMA = 'emad.map-first-recovery-result.v1'

# Every rehearsal the qualification profile declares, with the check that
# actually exercises it. ``test`` is a Go test in ./internal/integration that
# runs against disposable services; ``kind`` says what the check establishes.
REHEARSALS = [
    {
        'id': 'api-restart',
        'kind': 'process-restart',
        'test': None,
        'establishes': 'The API process stops and starts again with retained cases, releases and private bytes intact.',
        'missing': 'No process-level restart harness exists. The workload harness restarts a worker, not the API server, and restarting a shared API is out of scope for an isolated rehearsal.',
    },
    {
        'id': 'worker-loss',
        'kind': 'process-restart',
        'test': None,
        'establishes': 'A worker disappears mid-job and its work is recovered by another worker without double publication.',
        'missing': 'No worker-kill harness exists. Expired-lease recovery is covered by stale-lease; killing a live worker is a different fault and is not exercised.',
    },
    {
        'id': 'stale-lease',
        'kind': 'integration-test',
        'test': 'TestMapFirstDeliveryCancellationDuringIOAndExpiredLeaseRecovery',
        'establishes': 'An expired delivery lease is recovered and a cancelled job cannot publish.',
    },
    {
        'id': 'storage-loss',
        'kind': 'integration-test',
        'test': 'TestMapFirstPairedRecoveryRetainsCasesApprovalJobsAndPrivateBytes',
        'establishes': 'Private object bytes survive a paired database+object restore with matching hashes.',
    },
    {
        'id': 'provider-backoff',
        'kind': 'integration-test',
        'test': 'TestPublicInitialOutageAndDurableQuota',
        'establishes': 'A provider outage produces durable backoff and a resumable checkpoint rather than a lost request.',
    },
    {
        'id': 'credential-exhaustion',
        'kind': 'integration-test',
        'test': 'TestMapFirstSharedAccountQuotaUncertainChargeAndStorageGuards',
        'establishes': 'An exhausted shared quota refuses new billable work and records the uncertain charge conservatively.',
    },
    {
        'id': 'storage-quota',
        'kind': 'integration-test',
        'test': 'TestMapFirstSharedAccountQuotaUncertainChargeAndStorageGuards',
        'establishes': 'A storage reservation refuses work beyond the declared budget instead of failing after publication.',
    },
    {
        'id': 'lost-response',
        'kind': 'integration-test',
        'test': 'TestMapFirstDurableAcquisitionLostResponseCoverageAndCancellation',
        'establishes': 'A lost provider response retains the original request identity instead of creating a duplicate.',
    },
    {
        'id': 'source-revocation',
        'kind': 'integration-test',
        'test': 'TestPublicConnectorLeasesRevocationAndIsolation',
        'establishes': 'A revoked source stops retrieval and its retained evidence stops being served under the old grant.',
    },
    {
        'id': 'deletion-expiry-interruption',
        'kind': 'integration-test',
        'test': 'TestMapFirstCompositeExpiryDeletionRetryAndTombstone',
        'establishes': 'An interrupted expiry deletion retries and leaves an authorized tombstone rather than a resurrection.',
    },
    {
        'id': 'paired-restore',
        'kind': 'integration-test',
        'test': 'TestMapFirstPairedRecoveryRetainsCasesApprovalJobsAndPrivateBytes',
        'establishes': 'A paired database+object restore retains cases, approval, jobs and private bytes.',
    },
    {
        'id': 'actual-rollback-binary-forward-recovery',
        'kind': 'external-command',
        'test': None,
        'establishes': 'A nominated rollback binary is provisioned, migrated forward and recovered without losing cases or dependencies.',
        'missing': 'Run scripts/ops/qualify_rollback.py with a nominated --binary and --manifest. A source archive is not an installed rollback binary.',
    },
]

BY_ID = {item['id']: item for item in REHEARSALS}
_DOCKER_PROBE = None


def load(path):
    return json.loads(Path(path).read_text(encoding='utf-8-sig'))


def save(path, value):
    Path(path).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')


def declared_ids():
    return list(load(PROFILE)['requiredRecovery'])


def docker_available():
    """Report whether an isolated container runtime can be reached.

    The probe is cached: a hanging daemon must not cost every rehearsal its own
    timeout, and the answer cannot change inside one invocation.
    """
    global _DOCKER_PROBE
    if _DOCKER_PROBE is not None:
        return _DOCKER_PROBE
    try:
        probe = subprocess.run(['docker', 'info', '--format', '{{.ServerVersion}}'],
                               capture_output=True, text=True, timeout=10)
    except (OSError, subprocess.SubprocessError):
        _DOCKER_PROBE = (False, 'the docker CLI is unavailable')
        return _DOCKER_PROBE
    if probe.returncode != 0:
        detail = (probe.stderr or probe.stdout).strip().splitlines()
        _DOCKER_PROBE = (False, 'the docker daemon is unreachable' + (': ' + detail[0] if detail else ''))
    else:
        _DOCKER_PROBE = (True, 'docker server ' + probe.stdout.strip())
    return _DOCKER_PROBE


def run_test(name, timeout):
    """Run one isolated integration test and classify its outcome honestly."""
    env = os.environ.copy()
    env.update({'C4ISR_TEST_INTEGRATION': '1', 'C4ISR_QUALIFICATION': '1'})
    command = ['go', 'test', '-p', '1', '-json', '-count=1', '-timeout', '%ds' % timeout,
               '-run', '^%s$' % name, './internal/integration']
    try:
        process = subprocess.run(command, cwd=ROOT, env=env, capture_output=True,
                                 encoding='utf-8', errors='replace',
                                 timeout=timeout + 120)
    except subprocess.TimeoutExpired:
        return 'FAIL', None, 'the rehearsal exceeded its %d s timeout' % timeout, True
    except (OSError, subprocess.SubprocessError) as error:
        return 'NOT_RUN', None, 'the rehearsal could not start: %s' % type(error).__name__, False
    passed = skipped = False
    detail = ''
    for line in process.stdout.splitlines():
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            continue
        if row.get('Action') == 'pass' and row.get('Test') == name:
            passed = True
        if row.get('Action') == 'skip' and row.get('Test') == name:
            skipped = True
        if row.get('Action') == 'output' and row.get('Test') == name:
            text = (row.get('Output') or '').strip()
            if text and not text.startswith(('=== RUN', '=== PAUSE', '=== CONT', '--- ')):
                detail = text[:400]
    if skipped:
        return 'NOT_RUN', None, 'the rehearsal skipped: ' + (detail or 'its prerequisites were not met'), False
    if passed and process.returncode == 0:
        return 'PASS', 'test ' + name, 'isolated integration test passed', False
    return 'FAIL', 'test ' + name, (detail or 'the rehearsal test failed'), False


def rehearse(item, timeout):
    """Return one rehearsal result. Never infers a pass from absence."""
    base = {'id': item['id'], 'kind': item['kind'], 'establishes': item['establishes']}
    if item['kind'] == 'external-command':
        base.update({'status': 'NOT_RUN', 'observed': None, 'reason': item['missing']})
        return base
    if item['test'] is None:
        base.update({'status': 'NOT_RUN', 'observed': None, 'reason': item['missing']})
        return base
    available, detail = docker_available()
    if not available:
        base.update({'status': 'NOT_RUN', 'observed': None,
                     'reason': 'isolated services are unavailable: ' + detail})
        return base
    status, observed, reason, timed_out = run_test(item['test'], timeout)
    base.update({'status': status, 'observed': observed, 'reason': reason})
    if timed_out:
        base['timedOut'] = True
    return base


def execute(only=None, timeout=900):
    declared = declared_ids()
    selected = [BY_ID[identifier] for identifier in declared if identifier in BY_ID]
    unknown = [identifier for identifier in declared if identifier not in BY_ID]
    if only:
        if only not in BY_ID:
            raise ValueError('unknown rehearsal: %s' % only)
        selected = [BY_ID[only]]
    items = [rehearse(item, timeout) for item in selected]
    if unknown:
        for identifier in unknown:
            items.append({'id': identifier, 'status': 'NOT_RUN', 'observed': None,
                          'reason': 'the executor has no definition for this declared rehearsal'})
    counts = {state: sum(item['status'] == state for item in items)
              for state in ('PASS', 'FAIL', 'NOT_RUN')}
    return {
        'schema': SCHEMA,
        'recordedAt': datetime.now(timezone.utc).isoformat(),
        'qualification': 'NOT_QUALIFIED',
        'scope': 'Isolated rehearsals only. No shared installation, credential or database was touched.',
        'hardware': {'platform': platform.platform(), 'processor': platform.processor(),
                     'logicalCPUs': os.cpu_count()},
        'declared': declared,
        'executed': len(items),
        'counts': counts,
        'status': 'FAIL' if counts['FAIL'] else ('PASS' if counts['NOT_RUN'] == 0 else 'PARTIAL'),
        'meaning': ('A rehearsal that did not run is not evidence that the fault is handled. This file '
                    'records what was exercised, not what was hoped.'),
        'items': items,
    }


def listing():
    declared = declared_ids()
    print('%-42s %-20s %s' % ('REHEARSAL', 'KIND', 'CHECK'))
    print('-' * 110)
    for identifier in declared:
        item = BY_ID.get(identifier)
        if item is None:
            print('%-42s %-20s %s' % (identifier, 'undefined', 'no executor definition'))
            continue
        check = item['test'] or item['missing']
        print('%-42s %-20s %s' % (identifier, item['kind'], check))
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, help='result path; refuses to overwrite')
    parser.add_argument('--only', help='run a single declared rehearsal')
    parser.add_argument('--timeout', type=int, default=900, help='per-rehearsal timeout in seconds')
    parser.add_argument('--list', action='store_true', help='print the rehearsals without running them')
    parser.add_argument('--dry-run', action='store_true', help='report what would run and exit')
    args = parser.parse_args()
    if args.list:
        return listing()
    if args.dry_run:
        available, detail = docker_available()
        print(json.dumps({'isolatedServices': available, 'detail': detail,
                          'wouldRun': [i['id'] for i in REHEARSALS if i['test']],
                          'wouldNotRun': [i['id'] for i in REHEARSALS if not i['test']]}))
        return 0
    if not args.output:
        parser.error('--output is required unless --list or --dry-run is used')
    if args.output.exists():
        parser.error('output exists; choose a new result path')
    result = execute(args.only, args.timeout)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    save(args.output, result)
    print(json.dumps({'status': result['status'], 'counts': result['counts'],
                      'output': str(args.output)}))
    return 0 if result['status'] == 'PASS' else 2


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print('Recovery rehearsal failed: %s. No gate was promoted.' % error, file=sys.stderr)
        sys.exit(1)
