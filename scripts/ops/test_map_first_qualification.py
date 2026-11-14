import copy
from datetime import datetime, timezone, timedelta
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import subprocess
import sys
import os
import qualify_map_first
import recovery_inventory

from qualify_map_first import summarize, validate_sessions, load, sha, sources, run_recorded_test, PROFILE, ROOT
from recovery_inventory import restore_policy, verify_required, unavailable_inputs, load_restore_inventory
from paired_backup import check, restored_object_counts


class QualificationTests(unittest.TestCase):
    def sample(self, rows):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'samples.jsonl'
            path.write_text('\n'.join(json.dumps(r) for r in rows))
            return summarize(path, load(PROFILE))

    def rows(self):
        rows = [{'kind': 'profile', 'synthetic': True, 'durationSeconds': 1}, {'kind': 'telemetry', 'success': True}]
        for operation in ('catalog_search', 'scope_resolve', 'scheduled_retrieval'):
            for i in range(20):
                rows.append({'kind': 'request', 'operation': operation, 'cold': False,
                    'elapsedMs': 5000 if i == 19 else 10, 'success': i != 19,
                    'timeout': i == 19, 'status': 504 if i == 19 else 200, 'bytes': 20})
        rows.append({'kind': 'end', 'completed': True, 'failures': 3, 'elapsedSeconds': 1})
        return rows

    def test_failures_and_timeouts_stay_in_percentiles(self):
        result = self.sample(self.rows())
        self.assertEqual(result['sampleStatus'], 'FAIL')
        for group in result['operations']:
            self.assertEqual(group['p99Ms'], 5000)
            self.assertEqual(group['errorRate'], .05)
            self.assertEqual(group['timeouts'], 1)
        self.assertEqual(result['qualification'], 'NOT_QUALIFIED')

    def test_interrupted_or_empty_run_cannot_pass(self):
        self.assertEqual(self.sample(self.rows()[:-1])['sampleStatus'], 'FAIL')
        self.assertEqual(self.sample([])['sampleStatus'], 'FAIL')

    def test_outcome_mismatch_and_invalid_samples_fail(self):
        rows = self.rows(); rows[-1]['failures'] = 0
        self.assertFalse(self.sample(rows)['outcomeCountsConsistent'])
        rows[2]['elapsedMs'] = float('nan')
        with self.assertRaises(ValueError): self.sample(rows)

    def test_incomplete_duration_and_inconsistent_timeout_cannot_pass(self):
        rows=self.rows();rows[0]['durationSeconds']=86400
        self.assertFalse(self.sample(rows)['durationComplete'])
        rows=self.rows();rows[2]['timeout']=True
        with self.assertRaises(ValueError):self.sample(rows)
        with self.assertRaises(ValueError):self.sample(self.rows()+[{'kind':'telemetry','success':True}])

    def test_standalone_summary_uses_original_budgets(self):
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'workload.jsonl';rows=self.rows()
            for row in rows:
                if row['kind']=='request':row.update(elapsedMs=1500,success=True,timeout=False,status=200)
            rows[-1]['failures']=0
            path.write_text('\n'.join(json.dumps(r) for r in rows))
            with self.assertRaises(ValueError):summarize(path)
            (path.parent/'frozen-input.json').write_text(json.dumps({'profile':load(PROFILE),'durationSeconds':1}))
            changed=load(PROFILE);changed['budgets']['searchCoverageP95Ms']=2000
            alternate=path.parent/'changed.json';alternate.write_text(json.dumps(changed))
            with patch.object(qualify_map_first,'PROFILE',alternate):result=summarize(path)
            self.assertEqual(result['sampleStatus'],'FAIL')
            self.assertEqual(result['sampleBudgetChecks'][0]['targetP95Ms'],1000)

    def test_runner_timeout_stops_owned_process_and_returns_failure(self):
        with tempfile.TemporaryFile() as log:
            code,reason=run_recorded_test([sys.executable,'-c','import time;time.sleep(30)'],os.environ.copy(),log,.1)
        self.assertEqual(reason,'TIMEOUT')
        self.assertNotEqual(code,0)

    def test_full_volume_requires_each_dataset_and_operation(self):
        profile=load(PROFILE);rows=self.rows()
        rows[0].update(fullDataset=True,**profile['dataset'],analysts=5,scheduledProfiles=10)
        for row in rows:
            if row['kind']=='request':row.update(success=True,timeout=False,status=200,elapsedMs=10)
        rows[-1]['failures']=0
        for operation in ('map_family','geography_search','weather_query'):
            rows.insert(-1,{'kind':'request','operation':operation,'cold':True,'elapsedMs':10,'success':True,'timeout':False,'status':200,'bytes':100})
        self.assertEqual(self.sample(rows)['sampleStatus'],'PASS')
        insufficient=copy.deepcopy(rows);insufficient[0]['reportRevisions']=99999
        self.assertEqual(self.sample(insufficient)['sampleStatus'],'FAIL')
        missing=[r for r in rows if r.get('operation')!='map_family']
        self.assertIn('map_family',self.sample(missing)['missingOperations'])
        slow=copy.deepcopy(rows)
        for row in slow:
            if row.get('operation')=='map_family':row['elapsedMs']=2001
        self.assertEqual(self.sample(slow)['sampleStatus'],'FAIL')

    def test_staged_and_deleted_sources_remain_in_fingerprint(self):
        with tempfile.TemporaryDirectory() as directory:
            area=Path(directory);(area/'scripts').mkdir();file=area/'scripts/staged.py';file.write_text('one')
            def git(args,**kwargs):
                if args[1:]==['diff','--name-only','-z','HEAD']:value='scripts/staged.py\0scripts/deleted.py\0'
                elif args[1:]==['ls-files','--others','--exclude-standard','-z']:value=''
                elif args[1:]==['rev-parse','HEAD']:value='baseline'
                else:raise AssertionError('Index-relative inventory cannot identify staged content')
                return subprocess.CompletedProcess(args,0,value,'')
            with patch.object(qualify_map_first,'ROOT',area),patch.object(qualify_map_first.subprocess,'run',side_effect=git):
                first=sources();file.write_text('two');second=sources()
            self.assertNotEqual(first['fingerprint'],second['fingerprint'])
            self.assertEqual(first['files'][0],{'path':'scripts/deleted.py','sha256':None,'state':'deleted'})

    def test_restore_counts_ignore_excluded_keys_absent_from_backup(self):
        objects=[{'key':'kept','bytes':7},{'key':'omitted','bytes':11}]
        counts=restored_object_counts(objects,{'omitted','already-deleted-1','already-deleted-2'})
        self.assertEqual(counts,{'objectsVerified':1,'objectBytesVerified':7})

    def test_legacy_restore_derives_policy_instead_of_skipping_it(self):
        now=datetime.now(timezone.utc)
        row={'key':'expired','sha256':'hash','bytes':2,'state':'READY','required':True,
             'retain_until':(now-timedelta(seconds=1)).isoformat()}
        actual={'tables':{},'securityDefinitions':{},'objects':[row]}
        with patch.object(recovery_inventory,'inventory',return_value=actual):
            snapshot,persisted=load_restore_inventory('isolated','restored','unused',{'schema':'emad.paired-backup.v1'})
        self.assertFalse(persisted)
        self.assertEqual(verify_required(snapshot,[],now),{'expired'})

    def test_persisted_restore_inventory_rejects_dependency_or_acl_drift(self):
        actual={'tables':{'case':{'rows':1,'sha256':'original'}},'securityDefinitions':{'sha256':'acl'},'objects':[]}
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'snapshot.json';path.write_text(json.dumps(actual))
            manifest={'inventory':{'file':'snapshot.json'}}
            with patch.object(recovery_inventory,'inventory',return_value=actual):
                self.assertTrue(load_restore_inventory('isolated','restored',directory,manifest)[1])
                changed=copy.deepcopy(actual);changed['tables']['case']['sha256']='changed';path.write_text(json.dumps(changed))
                with self.assertRaises(RuntimeError):load_restore_inventory('isolated','restored',directory,manifest)
                changed=copy.deepcopy(actual);changed['securityDefinitions']['sha256']='changed';path.write_text(json.dumps(changed))
                with self.assertRaises(RuntimeError):load_restore_inventory('isolated','restored',directory,manifest)

    def test_automation_and_alias_cannot_qualify_observed_sessions(self):
        document = {'schema': 'emad.observed-map-sessions.v1', 'status': 'OBSERVED',
            'synthetic': True, 'releaseFingerprint': 'candidate', 'caseContributorHumanIds': ['same'],
            'sessions': [{'role': 'reviewer', 'canonicalHumanId': 'same', 'outcome': 'PASS'}]}
        result = validate_sessions(document, 'candidate', load(PROFILE))
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('Reviewer is a case contributor or alias', result['errors'])
        self.assertIn('Observed non-synthetic sessions have not been recorded', result['errors'])

    def observed_record(self, profile=None):
        """Validator-only in-memory fixture; never persisted as user evidence."""
        profile = profile or load(PROFILE)
        evidence = {'path': Path(__file__).resolve().relative_to(ROOT).as_posix(), 'sha256': sha(__file__)}

        def task(check, **overrides):
            row = {'task': check, 'outcome': 'PASS', 'attempts': 1, 'completedUnassisted': 1,
                   'assistedCompletions': 0, 'assistanceCount': 0, 'wrongAreaOrWrongPeriod': False,
                   'coverageVersusAbsenceExplained': True, 'resultAndReviewStateExplained': True,
                   'critical': False, 'retested': False}
            row.update(overrides)
            return row

        def session(role, human, participant, checks, matrix=False):
            row = {'participantId': participant, 'canonicalHumanId': human, 'observerId': 'observer',
                   'role': role, 'consent': {'obtained': True, 'obtainedAt': '2026-10-27T09:55:00+03:00',
                                             'scope': 'Observed navigation, brief reading and explanation; no case prose retained.'},
                   'startedAt': '2026-10-27T10:00:00+03:00', 'endedAt': '2026-10-27T10:05:00+03:00',
                   'locale': 'en', 'timezone': 'Asia/Riyadh', 'device': 'fixture', 'inputMethod': 'keyboard',
                   'assistance': 'none', 'baselineWorkflow': 'fixture', 'baselineSeconds': 90,
                   'completionSeconds': 80, 'usefulness': 'fixture', 'explanation': 'fixture',
                   'outcome': 'PASS', 'evidence': [evidence], 'corrections': [], 'retestEvidence': [],
                   'tasks': [task(check) for check in checks]}
            for key in ('geography', 'selectionForms', 'evidenceStates', 'workflowChecks'):
                row[key] = list(profile[key]) if matrix else []
            return row

        sessions = [session('analyst', 'analyst-1', 'participant-1', profile['workflowChecks'], matrix=True),
                    session('analyst', 'analyst-2', 'participant-2', ['correct-area-period', 'coverage-uncertainty']),
                    session('analyst', 'analyst-3', 'participant-3',
                            ['keyboard-only', 'portrait-320px', 'portrait-390px', 'short-landscape', 'text-200-percent']),
                    session('reviewer', 'reviewer-1', 'participant-4', ['case-handoff']),
                    session('source-administrator', 'admin-1', 'participant-5', ['source-administrator-recovery-path']),
                    session('restricted-reader', 'reader-1', 'participant-6', ['offline-brief-reading'])]
        assistive = dict(evidence, participantId='reader-1', technology='screen reader',
                         observedAt='2026-10-27T10:40:00+03:00', observerId='observer')
        document = {'schema': 'emad.observed-map-sessions.v1', 'status': 'OBSERVED', 'synthetic': False,
                    'releaseFingerprint': 'a'*64,
                    'caseContributorHumanIds': ['analyst-1', 'analyst-2', 'analyst-3'],
                    'observer': {'observerId': 'observer', 'appointedBy': 'product validation',
                                 'attestedAt': '2026-10-27T11:00:00+03:00'},
                    'reviewerIndependence': {'asserted': True, 'reviewerCanonicalHumanIds': ['reviewer-1'],
                                             'attestedBy': 'observer', 'attestedAt': '2026-10-27T11:00:00+03:00',
                                             'basis': 'The reviewer is a different canonical human from every case contributor.'},
                    'assistiveTechnologyEvidence': assistive, 'sessions': sessions}
        return document

    def test_complete_record_then_source_and_evidence_drift(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        result = validate_sessions(document, 'a'*64, profile)
        self.assertEqual(result['status'], 'PASS')
        self.assertEqual(result['qualification'], 'NOT_QUALIFIED')
        self.assertEqual(result['a19Status'], 'NOT_RUN')
        self.assertEqual(validate_sessions(document, 'other-release', profile)['status'], 'FAIL')
        broken = copy.deepcopy(document); broken['sessions'][0]['evidence'][0]['sha256'] = 'wrong'
        self.assertEqual(validate_sessions(broken, 'a'*64, profile)['status'], 'FAIL')
        broken = copy.deepcopy(document)
        for session in broken['sessions']: session['workflowChecks'] = []
        self.assertEqual(validate_sessions(broken, 'a'*64, profile)['status'], 'FAIL')
        broken = copy.deepcopy(document); broken['releaseFingerprint'] = ''
        self.assertEqual(validate_sessions(broken, '', profile)['status'], 'FAIL')
        broken = copy.deepcopy(document); broken['sessions'][1]['participantId'] = broken['sessions'][0]['participantId']
        self.assertEqual(validate_sessions(broken, 'a'*64, profile)['status'], 'FAIL')

    def test_failed_journey_is_recorded_and_reported_not_rejected(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        failed = document['sessions'][1]
        failed['outcome'] = 'FAIL'
        failed['tasks'][0].update({'outcome': 'FAIL', 'completedUnassisted': 0})
        result = validate_sessions(document, 'a'*64, profile)
        self.assertEqual(result['status'], 'PASS')
        self.assertEqual(result['outcomes']['FAIL'], 1)
        self.assertEqual(result['unresolvedJourneys'][0]['task'], 'correct-area-period')
        self.assertEqual(result['qualification'], 'NOT_QUALIFIED')
        self.assertEqual(result['a19Status'], 'NOT_RUN')

    def test_critical_failure_must_be_repaired_and_retested(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        critical = document['sessions'][1]
        critical['outcome'] = 'FAIL'
        critical['tasks'][0].update({'outcome': 'FAIL', 'completedUnassisted': 0, 'critical': True})
        result = validate_sessions(document, 'a'*64, profile)
        self.assertEqual(result['status'], 'PASS')
        condition = next(c for c in result['exitMetric']['conditions']
                         if c['condition'] == 'critical-failures-repaired-and-retested')
        self.assertEqual(condition['status'], 'NOT_MET')
        repaired = copy.deepcopy(document)
        session = repaired['sessions'][1]
        session['tasks'][0]['retested'] = True
        session['corrections'] = [{'task': 'correct-area-period', 'defect': 'wrong period selected',
                                   'repair': 'explicit period selector', 'retestOutcome': 'PASS'}]
        session['retestEvidence'] = [session['evidence'][0]]
        result = validate_sessions(repaired, 'a'*64, profile)
        self.assertEqual(result['status'], 'PASS')
        condition = next(c for c in result['exitMetric']['conditions']
                         if c['condition'] == 'critical-failures-repaired-and-retested')
        self.assertEqual(condition['status'], 'MET')
        inconsistent = copy.deepcopy(repaired)
        inconsistent['sessions'][1]['tasks'][0]['retested'] = False
        self.assertEqual(validate_sessions(inconsistent, 'a'*64, profile)['status'], 'FAIL')

    def test_synthetic_and_missing_sessions_still_fail(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        synthetic = copy.deepcopy(document); synthetic['synthetic'] = True
        result = validate_sessions(synthetic, 'a'*64, profile)
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('Observed non-synthetic sessions have not been recorded', result['errors'])
        missing = copy.deepcopy(document); missing['sessions'] = []
        result = validate_sessions(missing, 'a'*64, profile)
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('No observed sessions were recorded', result['errors'])
        empty = copy.deepcopy(document); empty['status'] = 'NOT_RUN'
        result = validate_sessions(empty, 'a'*64, profile)
        self.assertEqual(result['status'], 'FAIL')
        self.assertEqual(result['observedSessions'], 6)

    def test_empty_template_validates_without_crashing(self):
        template = json.loads((ROOT / 'docs/implementation/MAP_FIRST_T23_OBSERVED_SESSIONS.template.json')
                              .read_text(encoding='utf-8'))
        result = validate_sessions(template, 'a'*64, load(PROFILE))
        self.assertEqual(result['status'], 'FAIL')
        self.assertEqual(result['observedSessions'], 0)
        self.assertEqual(result['qualification'], 'NOT_QUALIFIED')
        self.assertEqual(result['a19Status'], 'NOT_RUN')
        self.assertEqual(result['exitMetric']['status'], 'NOT_EVALUABLE')
        for condition in result['exitMetric']['conditions']:
            if condition['condition'] != 'participant-sample':
                self.assertEqual(condition['status'], 'NOT_EVALUABLE')

    def test_missing_reviewer_still_fails(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        document['sessions'] = [s for s in document['sessions'] if s['role'] != 'reviewer']
        result = validate_sessions(document, 'a'*64, profile)
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('No independent reviewer was observed', result['errors'])
        self.assertIn('Missing observed roles', result['errors'])
        self.assertEqual(result['participantSample']['status'], 'NOT_MET')
        below = self.observed_record(profile)
        below['sessions'] = [s for s in below['sessions'] if not (s['role'] == 'analyst' and s['canonicalHumanId'] == 'analyst-3')]
        result = validate_sessions(below, 'a'*64, profile)
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('Observed target analyst sample is below the A19 minimum', result['errors'])

    def test_reviewer_must_be_independent_of_contributors(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        reviewer = next(s for s in document['sessions'] if s['role'] == 'reviewer')
        reviewer['canonicalHumanId'] = 'analyst-1'
        result = validate_sessions(document, 'a'*64, profile)
        self.assertEqual(result['status'], 'FAIL')
        self.assertIn('Reviewer is a case contributor or alias', result['errors'])
        self.assertIn('Reviewer is also an observed analyst', result['errors'])
        unasserted = self.observed_record(profile)
        unasserted['reviewerIndependence']['asserted'] = False
        self.assertEqual(validate_sessions(unasserted, 'a'*64, profile)['status'], 'FAIL')

    def test_unassisted_rate_is_measured_and_thresholded(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        result = validate_sessions(document, 'a'*64, profile)
        self.assertEqual(result['status'], 'PASS')
        self.assertEqual(result['exitMetric']['threshold'], 0.80)
        self.assertEqual(result['exitMetric']['unassistedRate'], 1.0)
        condition = next(c for c in result['exitMetric']['conditions']
                         if c['condition'] == 'unassisted-rate-at-least-80-percent')
        self.assertEqual(condition['status'], 'MET')
        assisted = self.observed_record(profile)
        for task in assisted['sessions'][0]['tasks'][:5]:
            task.update({'completedUnassisted': 0, 'assistedCompletions': 1, 'assistanceCount': 1})
        result = validate_sessions(assisted, 'a'*64, profile)
        self.assertEqual(result['status'], 'PASS')
        self.assertAlmostEqual(result['exitMetric']['unassistedRate'], 17/22)
        condition = next(c for c in result['exitMetric']['conditions']
                         if c['condition'] == 'unassisted-rate-at-least-80-percent')
        self.assertEqual(condition['status'], 'NOT_MET')
        self.assertEqual(result['exitMetric']['status'], 'NOT_MET')
        self.assertEqual(result['exitMetric']['assistanceCount'], 5)
        over = self.observed_record(profile)
        over['sessions'][0]['tasks'][0].update({'completedUnassisted': 2, 'attempts': 1})
        self.assertEqual(validate_sessions(over, 'a'*64, profile)['status'], 'FAIL')

    def test_wrong_area_and_workflow_coverage_are_reported(self):
        profile = load(PROFILE)
        document = self.observed_record(profile)
        document['sessions'][0]['tasks'][0]['wrongAreaOrWrongPeriod'] = True
        result = validate_sessions(document, 'a'*64, profile)
        self.assertEqual(result['status'], 'PASS')
        condition = next(c for c in result['exitMetric']['conditions']
                         if c['condition'] == 'no-wrong-area-or-wrong-period')
        self.assertEqual(condition['status'], 'NOT_MET')
        uncovered = self.observed_record(profile)
        uncovered['sessions'][0]['tasks'] = [t for t in uncovered['sessions'][0]['tasks']
                                             if t['task'] != 'case-handoff']
        uncovered['sessions'][3]['tasks'] = []
        self.assertEqual(validate_sessions(uncovered, 'a'*64, profile)['status'], 'FAIL')

    def test_expired_and_tombstoned_objects_are_not_resurrected(self):
        now = datetime.now(timezone.utc)
        row = {'key': 'current', 'sha256': 'abc', 'bytes': 2, 'state': 'READY',
               'retain_until': (now + timedelta(hours=1)).isoformat(), 'required': True}
        expired = dict(row, key='expired', retain_until=(now-timedelta(seconds=1)).isoformat())
        deleted = dict(row, key='deleted', state='TOMBSTONED', required=False)
        snapshot = {'objects': [row, expired, deleted]}
        self.assertEqual(restore_policy(snapshot, now)[0], {'expired', 'deleted'})
        self.assertEqual(verify_required(snapshot, [row], now), {'expired', 'deleted'})
        with self.assertRaises(RuntimeError): verify_required(snapshot, [], now)
        changed = dict(row, sha256='wrong')
        with self.assertRaises(RuntimeError): verify_required(snapshot, [changed], now)

    def test_conflicting_required_object_hashes_fail(self):
        now = datetime.now(timezone.utc)
        row = {'key': 'same', 'sha256': 'a', 'bytes': 2, 'state': 'READY',
               'retain_until': (now+timedelta(hours=1)).isoformat(), 'required': True}
        with self.assertRaises(RuntimeError): restore_policy({'objects': [row, dict(row, sha256='b')]}, now)

    def test_policy_expired_preparation_is_explicitly_unavailable(self):
        now = datetime.now(timezone.utc)
        row = {'key': 'expired', 'entity_id': 'artifact-1', 'family': 'artifact', 'sha256': 'hash',
               'bytes': 2, 'state': 'READY', 'required': True, 'retain_until': (now-timedelta(seconds=1)).isoformat()}
        snapshot = {'objects': [row], 'preparationArtifacts': [{'job_id': 'preparation-1', 'artifact_id': 'artifact-1'}]}
        self.assertEqual(unavailable_inputs(snapshot, now), ({'artifact-1'}, {'preparation-1'}))

    def test_backup_requires_v2_inventory_and_rejects_byte_tampering(self):
        with tempfile.TemporaryDirectory() as directory:
            area = Path(directory); (area/'fixture').write_bytes(b'fixture')
            entry = {'file': 'fixture', 'bytes': 7, 'sha256': sha(area/'fixture')}
            manifest = {'schema': 'emad.paired-backup.v1', 'database': entry, 'privateFiles': entry, 'objects': []}
            (area/'manifest.json').write_text(json.dumps(manifest))
            self.assertEqual(check(area)['schema'], 'emad.paired-backup.v1')
            manifest['schema'] = 'emad.paired-backup.v2'
            (area/'manifest.json').write_text(json.dumps(manifest))
            with self.assertRaises(RuntimeError): check(area)
            manifest['inventory'] = entry
            (area/'manifest.json').write_text(json.dumps(manifest))
            self.assertEqual(check(area)['schema'], 'emad.paired-backup.v2')
            (area/'fixture').write_bytes(b'changed')
            with self.assertRaises(RuntimeError): check(area)


if __name__ == '__main__': unittest.main()
