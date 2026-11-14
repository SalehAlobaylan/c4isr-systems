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


def finite(value):
    return type(value) in (int, float) and math.isfinite(value)


# A19 (docs/implementation/MAP_FIRST_PLAN.md) fixes
# the closing sample and the exit measurement. These values are quoted from the
# plan unchanged: the validator never re-derives a threshold from an
# observation and never relaxes one after a run.
MIN_TARGET_ANALYSTS = 3
MIN_INDEPENDENT_REVIEWERS = 1
UNASSISTED_EXIT_THRESHOLD = 0.80
OBSERVED_OUTCOMES = ('PASS', 'FAIL', 'BLOCKED')
OBSERVED_ROLES = ('analyst', 'reviewer', 'source-administrator', 'restricted-reader')


def count(value):
    """A recorded count is a non-negative integer; a boolean is not a count."""
    return type(value) is int and value >= 0


def task_totals(tasks):
    """Aggregate assigned attempts, completions and assistance for task records."""
    totals = {'taskRecords': 0, 'assignedAttempts': 0, 'completedUnassisted': 0,
              'completedWithAssistance': 0, 'assistanceCount': 0}
    for task in tasks or []:
        if not isinstance(task, dict):
            continue
        totals['taskRecords'] += 1
        if count(task.get('attempts')): totals['assignedAttempts'] += task['attempts']
        if count(task.get('completedUnassisted')): totals['completedUnassisted'] += task['completedUnassisted']
        if count(task.get('assistedCompletions')): totals['completedWithAssistance'] += task['assistedCompletions']
        if count(task.get('assistanceCount')): totals['assistanceCount'] += task['assistanceCount']
    return totals


def evidence_path(entry):
    """Resolve a recorded repository-local evidence path, or None if unusable."""
    value = entry.get('path') if isinstance(entry, dict) else None
    if not isinstance(value, str) or not value: return None
    path = (ROOT / value).resolve()
    return path if path.is_relative_to(ROOT) else None


# Reasons a declared budget cannot be observed from a workload sample. They
# name the missing instrument instead of implying an untested pass.
BUDGET_UNMEASURED = {
    'firstUsefulMapMs': 'No browser journey ran in this workload sample; first useful map time is not observable from workload API calls.',
    'controlResponseMs': 'No browser control interaction ran in this workload sample; control responsiveness is not observable from workload API calls.',
    'canceledServerReadMs': 'This run cancelled no in-flight server read; the abandoned-read duration was never observed.',
    'recoverySeconds': 'Recovery duration is measured by the required-recovery executor, not by a workload sample.',
    'searchCoverageP95Ms': 'No coverage search completed in this run.',
    'mapFamilyTileP95Ms': 'No map-family read completed in this run.',
    'detailedGeoJSONBytes': 'No detailed geography read completed in this run.',
    'referenceTileBytes': 'No map-family tile read completed in this run.',
}


def declared_budget_checks(operations, targets, measurements):
    """Report every declared budget. Unmeasured budgets are never assumed to pass."""
    budgets = targets['budgets']
    coverage = ('catalog_search', 'geography_search', 'weather_query')
    explicit = {}
    for row in measurements:
        explicit[row['budget']] = row
    checks = []

    def measured(budget, observed, basis, operation=None, condition=None):
        target = budgets[budget]
        entry = {'budget': budget, 'target': target, 'observed': observed, 'basis': basis,
                 'status': 'PASS' if observed <= target else 'FAIL'}
        if operation is not None:
            entry['operation'] = operation
        if condition is not None:
            entry['condition'] = condition
        checks.append(entry)

    def unmeasured(budget, reason):
        checks.append({'budget': budget, 'target': budgets[budget], 'observed': None,
                       'basis': 'not measured by this run', 'status': 'NOT_MEASURED', 'reason': reason})

    def worst(names, key):
        values = [o[key] for o in operations if o['operation'] in names]
        return max(values) if values else None

    # Each operation is reported separately. Collapsing catalog, geography and
    # weather into one worst value hides which operation actually missed its
    # target, which is the first question a failing run has to answer.
    coverage_operations = ('catalog_search', 'geography_search', 'weather_query')
    observed_coverage = [o for o in operations if o['operation'] in coverage_operations]
    if not observed_coverage:
        unmeasured('searchCoverageP95Ms', BUDGET_UNMEASURED['searchCoverageP95Ms'])
    else:
        for entry in observed_coverage:
            measured('searchCoverageP95Ms', entry['p95Ms'],
                     'nearest-rank p95 of %s (%s) over all completed requests including failures and timeouts'
                     % (entry['operation'], entry['condition']),
                     operation=entry['operation'], condition=entry['condition'])
    map_operations = [o for o in operations if o['operation'] == 'map_family']
    if not map_operations:
        unmeasured('mapFamilyTileP95Ms', BUDGET_UNMEASURED['mapFamilyTileP95Ms'])
    else:
        for entry in map_operations:
            measured('mapFamilyTileP95Ms', entry['p95Ms'],
                     'nearest-rank p95 of map_family (%s) over all completed requests including failures and timeouts'
                     % entry['condition'],
                     operation=entry['operation'], condition=entry['condition'])
    payload = worst(('geography_search',), 'maxResponseBytes')
    if payload is None:
        unmeasured('detailedGeoJSONBytes', BUDGET_UNMEASURED['detailedGeoJSONBytes'])
    else:
        measured('detailedGeoJSONBytes', payload,
                 'largest detailed geography catalog response (GeoJSON feature collection) in bytes')
    payload = worst(('map_family',), 'maxResponseBytes')
    if payload is None:
        unmeasured('referenceTileBytes', BUDGET_UNMEASURED['referenceTileBytes'])
    else:
        measured('referenceTileBytes', payload, 'largest map-family tile response in bytes')
    for budget in ('firstUsefulMapMs', 'controlResponseMs', 'canceledServerReadMs', 'recoverySeconds'):
        row = explicit.get(budget)
        if row is None:
            unmeasured(budget, BUDGET_UNMEASURED[budget])
        else:
            measured(budget, row['observed'], row['basis'])
    order = list(targets['budgets'])
    checks.sort(key=lambda check: order.index(check['budget']))
    return checks


def declared_gate_checks(profile, end, targets, telemetry_rows, recovery):
    """Report the soak, concurrency and recovery gates the profile declares."""
    gates = []

    def gate(gate_id, target, observed, status, basis, reason=None):
        entry = {'gate': gate_id, 'target': target, 'observed': observed, 'status': status, 'basis': basis}
        if reason:
            entry['reason'] = reason
        gates.append(entry)

    soak = targets['soak']
    elapsed = end.get('elapsedSeconds') if isinstance(end, dict) else None
    if not finite(elapsed):
        gate('soak.wallClockSeconds', soak['wallClockSeconds'], None, 'NOT_MEASURED',
             'no completed wall clock was recorded', 'the run recorded no elapsed wall clock')
    elif elapsed < soak['wallClockSeconds']:
        gate('soak.wallClockSeconds', soak['wallClockSeconds'], elapsed, 'NOT_MEASURED',
             'measured wall clock seconds of this run',
             'the declared soak window was not exercised; this run is shorter than the declared soak')
    else:
        gate('soak.wallClockSeconds', soak['wallClockSeconds'], elapsed, 'PASS',
             'measured wall clock seconds of this run')
    restart = end.get('workerRestart') if isinstance(end, dict) else None
    if isinstance(restart, dict) and type(restart.get('performed')) is int:
        performed = restart['performed']
        status = 'PASS' if performed >= soak['plannedWorkerRestarts'] else 'FAIL'
        gate('soak.plannedWorkerRestarts', soak['plannedWorkerRestarts'], performed, status,
             restart.get('basis') or 'planned worker restarts performed and recorded by this run')
    else:
        gate('soak.plannedWorkerRestarts', soak['plannedWorkerRestarts'], None, 'NOT_MEASURED',
             'no planned worker restart was recorded', 'this run recorded no worker restart outcome')
    observed = None
    for row in telemetry_rows:
        value = row.get('activeRasterCaptures')
        if finite(value):
            observed = value if observed is None else max(observed, value)
    target = targets['concurrency']['maximumActiveRasterCaptures']
    if observed is None:
        gate('concurrency.maximumActiveRasterCaptures', target, None, 'NOT_MEASURED',
             'no active raster capture was observed', 'no native raster capture ran in this run')
    else:
        gate('concurrency.maximumActiveRasterCaptures', target, observed,
             'PASS' if observed <= target else 'FAIL', 'maximum concurrently active raster captures observed')
    results = {}
    for item in (recovery or {}).get('items', []):
        if isinstance(item, dict) and isinstance(item.get('id'), str):
            results[item['id']] = item
    for identifier in targets['requiredRecovery']:
        item = results.get(identifier)
        if item is None:
            gate('requiredRecovery.' + identifier, 'PASS', None, 'NOT_RUN',
                 'no rehearsal result was supplied to this summary',
                 'the required-recovery executor has not recorded this rehearsal')
            continue
        status = item.get('status')
        status = status if status in ('PASS', 'FAIL') else 'NOT_RUN'
        reason = item.get('reason') or 'rehearsal executed in isolation'
        gate('requiredRecovery.' + identifier, 'PASS', item.get('observed'), status, reason)
    return gates


def summarize(path, qualification_profile=None, recovery=None):
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
    telemetry_rows, measurements = [], []
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
                telemetry_rows.append(row)
            elif row['kind'] == 'measurement':
                if row['budget'] not in qualification_profile['budgets']: raise ValueError('Unknown budget measurement')
                if not finite(row['observed']) or row['observed'] < 0: raise ValueError('Invalid measured budget value')
                if not isinstance(row.get('basis'), str) or not row['basis']: raise ValueError('Missing measured basis')
                measurements.append(row)
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
    declared_budgets = declared_budget_checks(operations, targets, measurements)
    declared_gates = declared_gate_checks(profile, end, targets, telemetry_rows, recovery)
    budget_failed = [check for check in declared_budgets if check['status'] == 'FAIL']
    passed = bool(complete and consistent and not errors and not telemetry_errors and not budget_failed
                  and all(check['status'] == 'PASS' for check in [*budget_checks, *dataset_checks]))
    return {'schema': 'emad.map-first-workload-result.v1',
        'sampleStatus': 'PASS' if passed else 'FAIL',
        'qualification': 'NOT_QUALIFIED', 'profile': profile, 'end': end,
        'telemetrySamples': telemetry, 'telemetryErrors': telemetry_errors, 'operations': operations,
        'sampleBudgetChecks': budget_checks,
        'declaredBudgetChecks': declared_budgets,
        'declaredGateChecks': declared_gates,
        'budgetGate': {'declared': len(declared_budgets),
            'measured': sum(check['status'] in ('PASS', 'FAIL') for check in declared_budgets),
            'failed': len(budget_failed),
            'notMeasured': sum(check['status'] == 'NOT_MEASURED' for check in declared_budgets),
            'status': 'FAIL' if budget_failed else 'PASS',
            'basis': 'a measured budget outside its target fails the run; an unmeasured budget is reported and never assumed'},
        'datasetChecks': dataset_checks,
        'budgetProfileContentSha256': hashlib.sha256(json.dumps(targets, sort_keys=True).encode()).hexdigest(),
        'durationComplete': duration_complete,
        'missingOperations': missing, 'incomplete': not complete, 'outcomeCountsConsistent': consistent,
        'limitations': [('Full declared read volumes with scoped cold computations and warm pinned reads; no raster concurrency or browser timing.' if full_dataset else 'Reduced dataset; no map-family/tile timing, browser first useful map or control response measurement.'),
            'Process RSS is the isolated API process resident set; container or cgroup limits and browser memory are not measured.',
            'Retained-object growth counts database rows in the disposable isolated database; it is not an installed retention audit.',
            'Queue age and retrieval freshness are sampled inside the isolated database; they are not source observation freshness.',
            'Unmeasured declared budgets are reported as NOT_MEASURED with the missing instrument named.',
            'No paired restore, actual rollback binary or independent target users in this sample.',
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


def run_sample(output, seconds, full_dataset=False, recovery=None):
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
        report = summarize(samples, frozen['profile'], recovery) if samples.exists() else {'sampleStatus': 'FAIL', 'qualification': 'NOT_QUALIFIED', 'reason': 'No samples; inspect integration.jsonl'}
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
    """Assess an observed-session record for completeness and consistency.

    A recorded FAIL or BLOCKED journey is evidence, not a defect: it is
    counted, reported and measured against A19's exit conditions. The command
    never asserts that a human session happened and never awards
    qualification; it judges the record only.
    """
    errors = []
    if not isinstance(fingerprint,str) or re.fullmatch(r'[0-9a-f]{64}',fingerprint) is None:
        errors.append('A complete candidate source fingerprint is required')
    if document.get('schema') != 'emad.observed-map-sessions.v1': errors.append('Unknown session schema')
    if document.get('releaseFingerprint') != fingerprint: errors.append('Sessions are not pinned to this release')
    if document.get('status') != 'OBSERVED' or document.get('synthetic') is not False:
        errors.append('Observed non-synthetic sessions have not been recorded')
    observer = document.get('observer')
    if not isinstance(observer, dict) or not observer.get('observerId'):
        errors.append('Appointed observer identity missing')
    sessions = document.get('sessions', [])
    if not sessions: errors.append('No observed sessions were recorded')
    roles, matrix = set(), defaultdict(set)
    participants = {}
    contributors = set(document.get('caseContributorHumanIds', []))
    if not contributors: errors.append('Case contributor identities missing')

    # The reviewer independence assertion is A19's, not the tool's: the tool
    # checks that it is made, that it names every observed reviewer and that
    # no observed reviewer is a case contributor or an observed analyst.
    independence = document.get('reviewerIndependence')
    declared_reviewers = set()
    if not isinstance(independence, dict):
        errors.append('Reviewer independence assertion missing')
    else:
        if independence.get('asserted') is not True:
            errors.append('Reviewer independence is not asserted')
        declared = independence.get('reviewerCanonicalHumanIds')
        if not isinstance(declared, list) or not declared:
            errors.append('Reviewer independence assertion names no reviewer')
        else:
            declared_reviewers = {human for human in declared if isinstance(human, str)}
        for key in ('attestedBy', 'attestedAt', 'basis'):
            if not independence.get(key):
                errors.append('Reviewer independence assertion is missing: '+key)

    assistive = document.get('assistiveTechnologyEvidence')
    if not isinstance(assistive, dict): errors.append('Assistive technology observation missing')
    else:
        path = evidence_path(assistive)
        if path is None or not path.is_file() or sha(path) != assistive.get('sha256'):
            errors.append('Assistive technology evidence missing or changed')
        for key in ('participantId', 'technology', 'observedAt'):
            if not assistive.get(key):
                errors.append('Assistive technology observation is missing: '+key)

    role_humans = defaultdict(set)
    outcome_counts = {'PASS': 0, 'FAIL': 0, 'BLOCKED': 0}
    totals = {'taskRecords': 0, 'assignedAttempts': 0, 'completedUnassisted': 0,
              'completedWithAssistance': 0, 'assistanceCount': 0}
    totals_by_role = defaultdict(lambda: {'taskRecords': 0, 'assignedAttempts': 0, 'completedUnassisted': 0,
                                          'completedWithAssistance': 0, 'assistanceCount': 0})
    covered_tasks, unresolved, wrong_area, critical_open = set(), [], [], []
    coverage_explained = True
    state_explained = True

    for row in sessions:
        role = row.get('role'); roles.add(role)
        if role not in OBSERVED_ROLES: errors.append('Unknown observed role')
        human = row.get('canonicalHumanId')
        if isinstance(human,str) and human: role_humans[role].add(human)
        if role == 'analyst' and human not in contributors:
            errors.append('Analyst absent from canonical contributor inventory')
        for key in ('participantId', 'canonicalHumanId', 'observerId', 'startedAt', 'endedAt', 'locale',
                    'timezone', 'device', 'inputMethod', 'assistance', 'baselineWorkflow',
                    'baselineSeconds', 'completionSeconds', 'usefulness', 'explanation', 'evidence'):
            if key not in row or row[key] is None or row[key] == '': errors.append('Missing observed field: '+key)
        consent = row.get('consent')
        if not isinstance(consent, dict):
            errors.append('Observed consent is missing')
        else:
            if consent.get('obtained') is not True:
                errors.append('Observed consent was not obtained')
            for key in ('obtainedAt', 'scope'):
                if not consent.get(key):
                    errors.append('Observed consent is missing: '+key)
        if role == 'reviewer' and human in contributors:
            errors.append('Reviewer is a case contributor or alias')
        participant = row.get('participantId')
        if isinstance(participant,str) and isinstance(human,str):
            if participant in participants and participants[participant] != human:
                errors.append('A participant maps to conflicting canonical humans')
            participants[participant] = human
        try:
            start, end = datetime.fromisoformat(row['startedAt']), datetime.fromisoformat(row['endedAt'])
            if start.tzinfo is None or end.tzinfo is None or end <= start:
                errors.append('Invalid observed session time')
        except (KeyError, ValueError, TypeError): errors.append('Invalid observed session time')
        outcome = row.get('outcome')
        if outcome not in OBSERVED_OUTCOMES:
            errors.append('Unknown observed task outcome')
        else:
            outcome_counts[outcome] += 1
        for key in ('baselineSeconds', 'completionSeconds'):
            if type(row.get(key)) not in (float, int) or not math.isfinite(row[key]) or row[key] < 0:
                errors.append('Invalid observed duration: '+key)
        corrections = row.get('corrections') or []
        if corrections and not row.get('retestEvidence'): errors.append('Corrections lack retest evidence')
        for entry in corrections:
            if not isinstance(entry,dict) or not entry.get('task') or not entry.get('defect') or not entry.get('repair'):
                errors.append('Invalid recorded correction')
            elif entry.get('retestOutcome') not in ('PASS','FAIL'):
                errors.append('Recorded correction lacks a retest outcome')
        for key in ('geography', 'selectionForms', 'evidenceStates', 'workflowChecks'):
            entries = row.get(key)
            if not isinstance(entries, list):
                errors.append('Missing observed '+key+' entries')
                continue
            matrix[key].update(entries)
        for evidence in [*row.get('evidence', []), *row.get('retestEvidence', [])]:
            path = evidence_path(evidence)
            if path is None or not path.is_file() or sha(path) != (evidence.get('sha256') if isinstance(evidence, dict) else None):
                errors.append('Missing or changed observed evidence')
        if not row.get('evidence'): errors.append('No observed evidence')
        tasks = row.get('tasks')
        if not isinstance(tasks, list) or not tasks:
            errors.append('Missing observed task attempts')
            tasks = []
        for key, value in task_totals(tasks).items():
            totals[key] += value
            totals_by_role[role][key] += value
        for task in tasks:
            if not isinstance(task,dict) or not isinstance(task.get('task'), str) or not task['task']:
                errors.append('Missing observed task record')
                continue
            label = task['task']
            covered_tasks.add(label)
            task_outcome = task.get('outcome')
            if task_outcome not in OBSERVED_OUTCOMES:
                errors.append('Unknown observed task outcome: '+label)
            for key in ('attempts', 'completedUnassisted', 'assistedCompletions', 'assistanceCount'):
                if not count(task.get(key)):
                    errors.append('Invalid observed task count: '+label+':'+key)
            if all(count(task.get(key)) for key in ('attempts','completedUnassisted','assistedCompletions')):
                if task['completedUnassisted'] + task['assistedCompletions'] > task['attempts']:
                    errors.append('Observed completions exceed assigned attempts: '+label)
            for key in ('wrongAreaOrWrongPeriod', 'coverageVersusAbsenceExplained',
                        'resultAndReviewStateExplained', 'critical', 'retested'):
                if not isinstance(task.get(key), bool):
                    errors.append('Missing observed task judgement: '+label+':'+key)
            if task.get('wrongAreaOrWrongPeriod') is True:
                wrong_area.append({'participantId': participant, 'task': label})
            if task.get('coverageVersusAbsenceExplained') is not True:
                coverage_explained = False
            if task.get('resultAndReviewStateExplained') is not True:
                state_explained = False
            if task_outcome in ('FAIL','BLOCKED'):
                unresolved.append({'participantId': participant, 'task': label, 'outcome': task_outcome})
                if task.get('critical') is True:
                    correction = next((entry for entry in corrections
                                       if isinstance(entry,dict) and entry.get('task') == label), None)
                    passed = correction is not None and correction.get('retestOutcome') == 'PASS'
                    if bool(task.get('retested')) != passed:
                        errors.append('Recorded retest does not match the recorded correction: '+label)
                    if not (bool(task.get('retested')) and passed):
                        critical_open.append({'participantId': participant, 'task': label, 'outcome': task_outcome})
    if not set(OBSERVED_ROLES) <= roles:
        errors.append('Missing observed roles')
    for key in ('geography', 'selectionForms', 'evidenceStates', 'workflowChecks'):
        if not set(profile[key]) <= matrix[key]: errors.append('Incomplete observed '+key+' matrix')
    if not set(profile['workflowChecks']) <= covered_tasks:
        errors.append('Workflow check matrix is not covered by recorded task attempts')

    analysts = role_humans.get('analyst', set())
    reviewers = role_humans.get('reviewer', set())
    if len(analysts) < MIN_TARGET_ANALYSTS:
        errors.append('Observed target analyst sample is below the A19 minimum')
    if len(reviewers) < MIN_INDEPENDENT_REVIEWERS:
        errors.append('No independent reviewer was observed')
    if reviewers & analysts:
        errors.append('Reviewer is also an observed analyst')
    if reviewers and declared_reviewers and not declared_reviewers >= reviewers:
        errors.append('Reviewer independence assertion does not name every observed reviewer')
    sample = {'required': {'targetAnalysts': MIN_TARGET_ANALYSTS, 'independentReviewers': MIN_INDEPENDENT_REVIEWERS},
              'observed': {'targetAnalysts': len(analysts), 'independentReviewers': len(reviewers),
                           'sourceAdministrators': len(role_humans.get('source-administrator', set())),
                           'restrictedReaders': len(role_humans.get('restricted-reader', set()))},
              'reviewersDistinctFromCaseContributors': bool(reviewers) and not reviewers & contributors,
              'status': 'MET' if len(analysts) >= MIN_TARGET_ANALYSTS
                        and len(reviewers) >= MIN_INDEPENDENT_REVIEWERS
                        and not reviewers & contributors and not reviewers & analysts else 'NOT_MET',
              'basis': 'A19 proposes at least three target analysts and one independent reviewer, separate from the case contributors.'}

    assigned = totals['assignedAttempts']
    rate = (totals['completedUnassisted'] / assigned) if assigned else None
    # An empty record has not satisfied an exit condition; it has not been
    # measured against one. Only an observed attempt can make a condition MET.
    measured = assigned > 0
    conditions = [
        {'condition': 'participant-sample', 'status': sample['status'], 'observed': sample['observed'],
         'basis': 'A19 proposes at least three target analysts and one independent reviewer, separate from the case contributors.'},
        {'condition': 'no-wrong-area-or-wrong-period',
         'status': 'NOT_EVALUABLE' if not measured else ('MET' if not wrong_area else 'NOT_MET'),
         'observed': len(wrong_area), 'basis': 'A19 exit: no wrong-area or wrong-period outcomes.'},
        {'condition': 'coverage-versus-absence-explained',
         'status': 'NOT_EVALUABLE' if not measured else ('MET' if coverage_explained else 'NOT_MET'),
         'observed': coverage_explained, 'basis': 'A19 exit: all participants correctly explain coverage versus absence.'},
        {'condition': 'result-and-review-state-explained',
         'status': 'NOT_EVALUABLE' if not measured else ('MET' if state_explained else 'NOT_MET'),
         'observed': state_explained, 'basis': 'A19 exit: all participants correctly explain result and review status.'},
        {'condition': 'unassisted-rate-at-least-80-percent',
         'status': 'NOT_EVALUABLE' if rate is None else ('MET' if rate >= UNASSISTED_EXIT_THRESHOLD else 'NOT_MET'),
         'observed': rate, 'threshold': UNASSISTED_EXIT_THRESHOLD,
         'basis': 'A19 exit: at least 80% of assigned core task attempts complete without developer assistance.'},
        {'condition': 'critical-failures-repaired-and-retested',
         'status': 'NOT_EVALUABLE' if not measured else ('MET' if not critical_open else 'NOT_MET'),
         'observed': len(critical_open), 'basis': 'A19 exit: every critical failed journey is repaired and retested.'},
    ]
    statuses = {condition['status'] for condition in conditions}
    exit_status = 'MET' if statuses == {'MET'} else ('NOT_EVALUABLE' if 'NOT_EVALUABLE' in statuses else 'NOT_MET')
    exit_metric = {'source': 'A19 observed independent usefulness',
                   'assignedAttempts': assigned, 'completedUnassisted': totals['completedUnassisted'],
                   'completedWithAssistance': totals['completedWithAssistance'],
                   'assistanceCount': totals['assistanceCount'], 'unassistedRate': rate,
                   'threshold': UNASSISTED_EXIT_THRESHOLD, 'status': exit_status, 'conditions': conditions,
                   'basis': 'Measured from the recorded task attempts of this record only; the thresholds are the governing plan values and are never re-derived from an observation.'}
    errors = list(dict.fromkeys(errors))
    return {'schema': 'emad.observed-map-sessions-assessment.v1',
            'status': 'FAIL' if errors else 'PASS', 'recordStatus': 'FAIL' if errors else 'PASS',
            'qualification': 'NOT_QUALIFIED', 'a19Status': 'NOT_RUN',
            'errors': errors, 'observedSessions': len(sessions), 'outcomes': outcome_counts,
            'unresolvedJourneys': unresolved, 'wrongAreaOrWrongPeriod': wrong_area,
            'participantSample': sample, 'taskTotals': totals, 'taskTotalsByRole': dict(totals_by_role),
            'exitMetric': exit_metric,
            'scope': ('Record completeness, identity and consistency checks plus the A19 exit measurement of the '
                      'recorded attempts; observer must attest the underlying observations truthfully. A recorded '
                      'FAIL or BLOCKED journey is reported evidence, not a rejection. This command never asserts that '
                      'a human session occurred and never awards qualification; A19 and T23 stay NOT_RUN.')}


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
    run = commands.add_parser('sample'); run.add_argument('--output', type=Path, required=True); run.add_argument('--seconds', type=int, default=5); run.add_argument('--full-dataset', action='store_true'); run.add_argument('--recovery', type=Path)
    summary = commands.add_parser('summarize'); summary.add_argument('samples', type=Path); summary.add_argument('--output', type=Path, required=True); summary.add_argument('--recovery', type=Path)
    sessions = commands.add_parser('sessions'); sessions.add_argument('record', type=Path); sessions.add_argument('--release-fingerprint', required=True); sessions.add_argument('--output', type=Path, required=True)
    ledger = commands.add_parser('ledger'); ledger.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    recovery = load(args.recovery) if getattr(args, 'recovery', None) else None
    if args.action == 'sample': return run_sample(args.output, args.seconds, args.full_dataset, recovery)
    if args.action == 'ledger': release_ledger(args.output); return 0
    if args.action == 'summarize': result = summarize(args.samples, None, recovery); passed = result['sampleStatus'] == 'PASS'
    else: result = validate_sessions(load(args.record), args.release_fingerprint, load(PROFILE)); passed = result['status'] == 'PASS'
    save(args.output, result); print(json.dumps(result)); return 0 if passed else 2


if __name__ == '__main__':
    try: sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError):
        print('Qualification command failed; inspect retained local evidence. No gate was promoted.', file=sys.stderr)
        sys.exit(1)
