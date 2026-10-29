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

    def test_complete_record_then_source_and_evidence_drift(self):
        # Validator-only in-memory fixture; never persisted as user evidence.
        evidence = {'path': Path(__file__).resolve().relative_to(ROOT).as_posix(), 'sha256': sha(__file__)}
        profile = load(PROFILE)
        row = {'participantId': 'fixture', 'canonicalHumanId': 'author', 'observerId': 'observer',
            'startedAt': '2026-10-27T10:00:00+03:00', 'endedAt': '2026-10-27T10:05:00+03:00',
            'locale': 'en', 'timezone': 'Asia/Riyadh', 'device': 'fixture', 'inputMethod': 'keyboard',
            'assistance': 'none', 'baselineWorkflow': 'fixture', 'baselineSeconds': 90,
            'completionSeconds': 80, 'usefulness': 'fixture', 'explanation': 'fixture',
            'outcome': 'PASS', 'evidence': [evidence]}
        row.update({key: profile[key] for key in ('geography', 'selectionForms', 'evidenceStates', 'workflowChecks')})
        sessions = [dict(row, role=role, participantId=role, canonicalHumanId='author' if role == 'analyst' else role)
                    for role in ('analyst', 'reviewer', 'source-administrator', 'restricted-reader')]
        doc = {'schema': 'emad.observed-map-sessions.v1', 'status': 'OBSERVED', 'synthetic': False,
               'releaseFingerprint': 'a'*64, 'caseContributorHumanIds': ['author'],
               'sessions': sessions, 'assistiveTechnologyEvidence': evidence}
        self.assertEqual(validate_sessions(doc, 'a'*64, profile)['status'], 'PASS')
        self.assertEqual(validate_sessions(doc, 'other-release', profile)['status'], 'FAIL')
        broken = copy.deepcopy(doc); broken['sessions'][0]['evidence'][0]['sha256'] = 'wrong'
        self.assertEqual(validate_sessions(broken, 'a'*64, profile)['status'], 'FAIL')
        broken=copy.deepcopy(doc)
        for session in broken['sessions']:session['workflowChecks']=[]
        self.assertEqual(validate_sessions(broken, 'a'*64, profile)['status'],'FAIL')
        broken=copy.deepcopy(doc);broken['releaseFingerprint']=''
        self.assertEqual(validate_sessions(broken, '', profile)['status'],'FAIL')
        broken=copy.deepcopy(doc);broken['sessions'][1]['participantId']=broken['sessions'][0]['participantId']
        self.assertEqual(validate_sessions(broken, 'a'*64, profile)['status'],'FAIL')

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
