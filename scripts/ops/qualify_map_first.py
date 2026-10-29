"""Freeze, execute and assess isolated MAP-6 evidence without promoting gates.

No shared credentials, services or databases are used. A short or long reduced
sample never becomes the full declared workload. Failed requests/timeouts remain
in each latency distribution; samples are separated into cold and warm groups.
"""
import argparse
from collections import defaultdict
from datetime import datetime, timezone
import hashlib
import json
import math
import os
from pathlib import Path
import platform
import re
import signal
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
PROFILE = Path(__file__).with_name('map_first_qualification_profile.json')
BACKLOG = ROOT / 'docs/implementation/MAP_FIRST_BACKLOG_2026_10_25.json'


def load(path):
    return json.loads(Path(path).read_text(encoding='utf-8-sig'))


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def save(path, value):
    Path(path).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')


def sources():
    changed = subprocess.run(['git', 'diff', '--name-only', '-z', 'HEAD'], cwd=ROOT,
                             capture_output=True, text=True, check=True)
    untracked = subprocess.run(['git', 'ls-files', '--others', '--exclude-standard', '-z'], cwd=ROOT,
                               capture_output=True, text=True, check=True)
    paths = sorted({p for p in (changed.stdout + untracked.stdout).split('\0')
                    if (p.startswith(('api/', 'apps/', 'db/', 'internal/', 'scripts/', 'deployments/'))
                        or p in ('README.md', 'PRODUCT.md', 'Taskfile.yml'))})
    files = [{'path': p, 'sha256': sha(ROOT / p) if (ROOT / p).is_file() else None,
              'state': 'present' if (ROOT / p).is_file() else 'deleted'} for p in paths]
    baseline = subprocess.run(['git', 'rev-parse', 'HEAD'], cwd=ROOT, capture_output=True,
                              text=True, check=True).stdout.strip()
    # Hash the baseline too: tracked unchanged files are identified by HEAD.
    fingerprint = hashlib.sha256(json.dumps([baseline, files], sort_keys=True).encode()).hexdigest()
    return {'baselineCommit': baseline, 'files': files, 'fingerprint': fingerprint}


def percentile(values, fraction):
    return sorted(values)[max(0, math.ceil(len(values) * fraction) - 1)]


def summarize(path, qualification_profile=None):
    expected_duration = None
    if qualification_profile is None:
        frozen_path = Path(path).with_name('frozen-input.json')
        if not frozen_path.is_file(): raise ValueError('Standalone summary requires the original frozen-input.json')
        frozen = load(frozen_path)
        qualification_profile = frozen['profile']
        expected_duration = frozen['durationSeconds']
    groups = defaultdict(list)
    profile = end = None
    telemetry, telemetry_errors = 0, 0
    with Path(path).open(encoding='utf-8-sig') as stream:
        for line in stream:
            row = json.loads(line)
            if end is not None: raise ValueError('Records after completion')
            if profile is None and row['kind'] != 'profile': raise ValueError('Profile must be first')
            if row['kind'] == 'profile':
                if profile: raise ValueError('Multiple workload profiles')
                profile = row
            elif row['kind'] == 'end':
                if end: raise ValueError('Multiple completion records')
                end = row
            elif row['kind'] == 'telemetry':
                if type(row['success']) is not bool: raise ValueError('Invalid telemetry outcome')
                telemetry += 1
                telemetry_errors += not row['success']
            elif row['kind'] == 'request':
                if row['operation'] not in ('catalog_search', 'scope_resolve', 'scheduled_retrieval', 'map_family', 'geography_search', 'weather_query'):
                    raise ValueError('Unknown operation label')
                if not isinstance(row['success'], bool) or not isinstance(row['timeout'], bool):
                    raise ValueError('Missing request outcome')
                if row['timeout'] and row['success'] or row['success'] and not 200 <= row['status'] < 300:
                    raise ValueError('Inconsistent request outcome')
                if not math.isfinite(row['elapsedMs']) or row['elapsedMs'] < 0:
                    raise ValueError('Invalid measured duration')
                groups[(row['operation'], 'cold' if row['cold'] else 'warm')].append(row)
    operations = []
    for (operation, condition), rows in sorted(groups.items()):
        durations = [r['elapsedMs'] for r in rows]  # deliberately includes failures/timeouts
        operations.append({'operation': operation, 'condition': condition, 'requests': len(rows),
            'errors': sum(not r['success'] for r in rows), 'timeouts': sum(r['timeout'] for r in rows),
            'errorRate': sum(not r['success'] for r in rows) / len(rows),
            'timeoutRate': sum(r['timeout'] for r in rows) / len(rows),
            'p50Ms': percentile(durations, .5), 'p95Ms': percentile(durations, .95),
            'p99Ms': percentile(durations, .99), 'maxResponseBytes': max(r['bytes'] for r in rows),
            'quantileMethod': 'nearest-rank; all completed requests including failed/timeouts'})
    full_dataset = bool(profile and profile.get('fullDataset') is True)
    required_operations = ['catalog_search', 'scope_resolve', 'scheduled_retrieval']
    if full_dataset: required_operations += ['map_family', 'geography_search', 'weather_query']
    missing = [name for name in required_operations
               if not any(o['operation'] == name for o in operations)]
    errors = sum(o['errors'] for o in operations)
    targets = qualification_profile
    budget_checks = []
    for o in operations:
        if o['operation'] not in ('catalog_search','geography_search','weather_query','map_family'): continue
        target = targets['budgets']['mapFamilyTileP95Ms' if o['operation']=='map_family' else 'searchCoverageP95Ms']
        budget_checks.append({'operation':o['operation'],'condition':o['condition'],'targetP95Ms':target,
            'observedP95Ms':o['p95Ms'],'status':'PASS' if o['p95Ms']<=target else 'FAIL'})
    dataset_checks = []
    if full_dataset:
        for key,target in targets['dataset'].items():
            observed = profile.get(key)
            dataset_checks.append({'field':key,'target':target,'observed':observed,
                'status':'PASS' if type(observed) is int and observed>=target else 'FAIL'})
        for key in ('analysts','scheduledProfiles'):
            observed = profile.get(key);target = targets['concurrency'][key]
            dataset_checks.append({'field':key,'target':target,'observed':observed,
                'status':'PASS' if type(observed) is int and observed>=target else 'FAIL'})
    def positive_number(value):
        return type(value) in (int, float) and math.isfinite(value) and value > 0
    duration_complete = bool(profile and end and positive_number(profile.get('durationSeconds'))
        and positive_number(end.get('elapsedSeconds')) and end['elapsedSeconds'] >= profile['durationSeconds'])
    if expected_duration is not None:
        duration_complete = duration_complete and profile['durationSeconds'] == expected_duration
    complete = bool(profile and end and end.get('completed') is True and duration_complete and telemetry and not missing)
    consistent = bool(end and end.get('failures') == errors)
    return {'schema': 'emad.map-first-workload-result.v1',
        'sampleStatus': 'PASS' if complete and consistent and not errors and not telemetry_errors and all(c['status'] == 'PASS' for c in [*budget_checks,*dataset_checks]) else 'FAIL',
        'qualification': 'NOT_QUALIFIED', 'profile': profile, 'end': end,
        'telemetrySamples': telemetry, 'telemetryErrors': telemetry_errors, 'operations': operations,
        'sampleBudgetChecks': budget_checks,
        'datasetChecks': dataset_checks,
        'budgetProfileContentSha256': hashlib.sha256(json.dumps(targets, sort_keys=True).encode()).hexdigest(),
        'durationComplete': duration_complete,
        'missingOperations': missing, 'incomplete': not complete, 'outcomeCountsConsistent': consistent,
        'limitations': [('Full declared read volumes with scoped cold computations and warm pinned reads; no raster concurrency or browser timing.' if full_dataset else 'Reduced dataset; no map-family/tile timing, browser first useful map or control response measurement.'),
            'Heap telemetry is not process/container RSS; no measured retained-object/freshness-growth qualification.',
            'No real process restart, paired restore, actual rollback binary or independent target users in this sample.',
            'A longer sample measures actual elapsed time but does not close missing workload/recovery gates.'],
        'samplesSha256': sha(path)}


def run_recorded_test(command, env, log, timeout):
    options = {'creationflags': subprocess.CREATE_NEW_PROCESS_GROUP | subprocess.CREATE_NO_WINDOW} if os.name == 'nt' else {'start_new_session': True}
    try:
        process = subprocess.Popen(command, cwd=ROOT, env=env, stdout=log,
            stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, **options)
    except OSError:
        return 127, 'START_FAILED'
    try:
        return process.wait(timeout=timeout), None
    except (subprocess.TimeoutExpired, KeyboardInterrupt) as error:
        reason = 'INTERRUPTED' if isinstance(error, KeyboardInterrupt) else 'TIMEOUT'
        # Only this invocation's owned process tree is stopped. No shared API,
        # workers or containers are selected by name.
        if os.name == 'nt':
            try:
                subprocess.run(['taskkill', '/PID', str(process.pid), '/T', '/F'],
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
            except (OSError, subprocess.TimeoutExpired): pass
        else:
            try: os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError: pass
        try: process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            if os.name != 'nt':
                try: os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError: pass
            process.kill(); process.wait(timeout=5)
        return process.returncode, reason


def run_sample(output, seconds, full_dataset=False):
    if not 1 <= seconds <= 90000: raise ValueError('Duration must be 1–90000 actual seconds')
    output = output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    frozen = {'schema': 'emad.map-first-run-input.v1', 'frozenAt': datetime.now(timezone.utc).isoformat(),
        'profile': load(PROFILE), 'profileSha256': sha(PROFILE), 'source': sources(),
        'durationSeconds': seconds, 'fullDataset': full_dataset, 'hardware': {'platform': platform.platform(),
        'processor': platform.processor(), 'logicalCPUs': os.cpu_count()},
        'isolation': 'Go Testcontainers only; no shared installation', 'sharedMigrationPerformed': False}
    save(output / 'frozen-input.json', frozen)
    env = os.environ.copy()
    env.update({'GOMAXPROCS': '2', 'C4ISR_TEST_INTEGRATION': '1', 'C4ISR_QUALIFICATION': '1',
        'C4ISR_QUALIFICATION_DURATION': f'{seconds}s', 'C4ISR_TEST_EVIDENCE_DIR': str(output),
        'C4ISR_QUALIFICATION_FULL':'1' if full_dataset else '0'})
    test_name = 'TestMapFirstFullVolumeConcurrentQualification' if full_dataset else 'TestMapFirstQualificationConcurrentSample'
    command = ['go', 'test', '-p', '1', '-json', '-count=1', '-timeout', f'{seconds+300}s',
               '-run', '^'+test_name+'$', './internal/integration']
    start = time.monotonic()
    with (output / 'integration.jsonl').open('w', encoding='utf-8') as log:
        exit_code, failure = run_recorded_test(command, env, log, seconds+360)
    samples = output / 'workload.jsonl'
    try:
        report = summarize(samples, frozen['profile']) if samples.exists() else {'sampleStatus': 'FAIL', 'qualification': 'NOT_QUALIFIED', 'reason': 'No samples; inspect integration.jsonl'}
    except (ValueError, KeyError, TypeError):
        report = {'sampleStatus': 'FAIL', 'qualification': 'NOT_QUALIFIED', 'reason': 'Incomplete or malformed samples; inspect integration.jsonl'}
    duration_matched = bool(report.get('profile') and report['profile'].get('durationSeconds') == seconds)
    dataset_matched = bool(report.get('profile') and bool(report['profile'].get('fullDataset')) == full_dataset)
    report.update({'commandExitCode': exit_code, 'runnerFailure': failure, 'runnerElapsedSeconds': time.monotonic()-start,
                   'frozenDurationMatched': duration_matched,
                   'frozenDatasetMatched': dataset_matched,
                   'sourceUnchanged': sources()['fingerprint'] == frozen['source']['fingerprint']})
    if exit_code or failure or not duration_matched or not dataset_matched or not report['sourceUnchanged']: report['sampleStatus'] = 'FAIL'
    save(output / 'result.json', report)
    print(json.dumps({'sampleStatus': report['sampleStatus'], 'qualification': 'NOT_QUALIFIED', 'output': str(output)}))
    return int(report['sampleStatus'] != 'PASS')


def validate_sessions(document, fingerprint, profile):
    errors = []
    if not isinstance(fingerprint,str) or re.fullmatch(r'[0-9a-f]{64}',fingerprint) is None:
        errors.append('A complete candidate source fingerprint is required')
    if document.get('schema') != 'emad.observed-map-sessions.v1': errors.append('Unknown session schema')
    if document.get('releaseFingerprint') != fingerprint: errors.append('Sessions are not pinned to this release')
    if document.get('status') != 'OBSERVED' or document.get('synthetic') is not False:
        errors.append('Observed non-synthetic sessions have not been recorded')
    sessions = document.get('sessions', [])
    roles, matrix = set(), defaultdict(set)
    participants = {}
    contributors = set(document.get('caseContributorHumanIds', []))
    if not contributors: errors.append('Case contributor identities missing')
    for row in sessions:
        role = row.get('role'); roles.add(role)
        if role not in ('analyst', 'reviewer', 'source-administrator', 'restricted-reader'):
            errors.append('Unknown observed role')
        if role == 'analyst' and row.get('canonicalHumanId') not in contributors:
            errors.append('Analyst absent from canonical contributor inventory')
        for key in ('participantId', 'canonicalHumanId', 'observerId', 'startedAt', 'endedAt', 'locale',
                    'timezone', 'device', 'inputMethod', 'assistance', 'baselineWorkflow',
                    'baselineSeconds', 'completionSeconds', 'usefulness', 'explanation', 'evidence'):
            if key not in row or row[key] is None or row[key] == '': errors.append('Missing observed field: '+key)
        if role == 'reviewer' and row.get('canonicalHumanId') in contributors:
            errors.append('Reviewer is a case contributor or alias')
        participant, human = row.get('participantId'), row.get('canonicalHumanId')
        if isinstance(participant,str) and isinstance(human,str):
            if participant in participants and participants[participant] != human:
                errors.append('A participant maps to conflicting canonical humans')
            participants[participant] = human
        try:
            start, end = datetime.fromisoformat(row['startedAt']), datetime.fromisoformat(row['endedAt'])
            if start.tzinfo is None or end.tzinfo is None or end <= start:
                errors.append('Invalid observed session time')
        except (KeyError, ValueError, TypeError): errors.append('Invalid observed session time')
        if row.get('outcome') != 'PASS': errors.append('Unresolved observed task outcome')
        for key in ('baselineSeconds', 'completionSeconds'):
            if type(row.get(key)) not in (float, int) or not math.isfinite(row[key]) or row[key] < 0:
                errors.append('Invalid observed duration: '+key)
        if row.get('corrections') and not row.get('retestEvidence'): errors.append('Corrections lack retest evidence')
        for key in ('geography', 'selectionForms', 'evidenceStates', 'workflowChecks'):
            matrix[key].update(row.get(key, []))
        for evidence in [*row.get('evidence', []), *row.get('retestEvidence', [])]:
            path = (ROOT / evidence.get('path', '')).resolve()
            if not path.is_relative_to(ROOT) or not path.is_file() or sha(path) != evidence.get('sha256'):
                errors.append('Missing or changed observed evidence')
        if not row.get('evidence'): errors.append('No observed evidence')
    if not {'analyst', 'reviewer', 'source-administrator', 'restricted-reader'} <= roles:
        errors.append('Missing observed roles')
    for key in ('geography', 'selectionForms', 'evidenceStates', 'workflowChecks'):
        if not set(profile[key]) <= matrix[key]: errors.append('Incomplete observed '+key+' matrix')
    at = document.get('assistiveTechnologyEvidence')
    if not isinstance(at, dict): errors.append('Assistive technology observation missing')
    else:
        path = (ROOT / at.get('path', '')).resolve()
        if not path.is_relative_to(ROOT) or not path.is_file() or sha(path) != at.get('sha256'):
            errors.append('Assistive technology evidence missing or changed')
    return {'status': 'FAIL' if errors else 'PASS', 'errors': errors, 'observedSessions': len(sessions),
            'scope': 'Record completeness and identity checks; observer must attest the underlying observations truthfully.'}


def release_ledger(output):
    backlog = load(BACKLOG)
    tasks = []
    for task in backlog['tasks']:
        tasks.append({key: task[key] for key in ('id', 'title', 'status', 'gapIds', 'acceptanceIds', 'evidence', 'remainingVerification')})
    blockers = [f'{t["id"]} closure is {t["status"]}' for t in tasks if t['status'] != 'COMPLETE']
    blockers += [f'{s["id"]} is {s["status"]}' for s in backlog['acceptanceScenarios'] if s['mandatory'] and s['status'] != 'PASS']
    references = sorted({entry['reference'].split('#', 1)[0]
        for task in tasks for entries in task['evidence'].values() for entry in entries
        if isinstance(entry, dict) and entry.get('reference')})
    evidence_files = []
    for reference in references:
        path = (ROOT / reference).resolve()
        present = path.is_relative_to(ROOT) and path.is_file()
        evidence_files.append({'path': reference, 'status': 'PRESENT' if present else 'MISSING',
                               'sha256': sha(path) if present else None})
        if not present: blockers.append('Evidence unavailable: '+reference)
    # This command assembles traceability, never adjudicates independent approval.
    report = {'schema': 'emad.map-first-release-ledger.v1', 'recordedAt': datetime.now(timezone.utc).isoformat(),
        'status': 'NOT_QUALIFIED', 'source': sources(), 'profileSha256': sha(PROFILE),
        'installation': {'migration': 'User reports applied through 00053; not independently inspected',
                         'sharedDeployment': 'NOT_VERIFIED', 'geographicCoverage': 'Only scope-specific evidence; no global detail claim'},
        'tasks': tasks, 'gaps': backlog['gapCoverage'], 'acceptance': backlog['acceptanceScenarios'],
        'hardeningChecks': backlog['hardeningChecks'], 'requirements': backlog['requirements'],
        'evidenceFiles': evidence_files, 'blockers': blockers,
        'qualificationAuthority': 'Named owner/reviewer and all mandatory evidence required; this command does not promote the release.'}
    save(output, report)
    print(json.dumps({'status': report['status'], 'tasks': len(tasks), 'blockers': len(blockers), 'output': str(output)}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='action', required=True)
    run = commands.add_parser('sample'); run.add_argument('--output', type=Path, required=True); run.add_argument('--seconds', type=int, default=5); run.add_argument('--full-dataset', action='store_true')
    summary = commands.add_parser('summarize'); summary.add_argument('samples', type=Path); summary.add_argument('--output', type=Path, required=True)
    sessions = commands.add_parser('sessions'); sessions.add_argument('record', type=Path); sessions.add_argument('--release-fingerprint', required=True); sessions.add_argument('--output', type=Path, required=True)
    ledger = commands.add_parser('ledger'); ledger.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.action == 'sample': return run_sample(args.output, args.seconds, args.full_dataset)
    if args.action == 'ledger': release_ledger(args.output); return 0
    if args.action == 'summarize': result = summarize(args.samples); passed = result['sampleStatus'] == 'PASS'
    else: result = validate_sessions(load(args.record), args.release_fingerprint, load(PROFILE)); passed = result['status'] == 'PASS'
    save(args.output, result); print(json.dumps(result)); return 0 if passed else 2


if __name__ == '__main__':
    try: sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError):
        print('Qualification command failed; inspect retained local evidence. No gate was promoted.', file=sys.stderr)
        sys.exit(1)
