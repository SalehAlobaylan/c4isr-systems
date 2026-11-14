"""Tests for the required-recovery executor and the rollback rehearsal.

These tests never require Docker. They exercise argument handling, the
never-fabricate-a-PASS rule, the NOT_RUN-when-nothing-is-nominated rule and the
result-file shape that qualify_map_first.py consumes.
"""
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

OPS = Path(__file__).resolve().parent
sys.path.insert(0, str(OPS))

import qualify_recovery  # noqa: E402
import qualify_rollback  # noqa: E402


def load_module(name):
    spec = importlib.util.spec_from_file_location(name, OPS / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class RecoveryDeclarations(unittest.TestCase):
    def test_every_declared_rehearsal_has_a_definition(self):
        declared = qualify_recovery.declared_ids()
        defined = {item['id'] for item in qualify_recovery.REHEARSALS}
        self.assertEqual(set(declared) - defined, set(),
                         'a declared rehearsal has no executor definition')
        self.assertEqual(defined - set(declared), set(),
                         'the executor defines a rehearsal the profile does not declare')

    def test_every_definition_states_what_it_establishes(self):
        for item in qualify_recovery.REHEARSALS:
            self.assertTrue(item['establishes'].strip(), item['id'])
            self.assertTrue(item['test'] or item.get('missing'), item['id'])

    def test_an_undefined_rehearsal_is_never_reported_as_passing(self):
        original = qualify_recovery.REHEARSALS
        original_docker = qualify_recovery.docker_available
        try:
            qualify_recovery.docker_available = lambda: (False, 'no runtime in this test')
            qualify_recovery.REHEARSALS = [item for item in original if item['id'] != 'paired-restore']
            qualify_recovery.BY_ID = {item['id']: item for item in qualify_recovery.REHEARSALS}
            result = qualify_recovery.execute(timeout=1)
        finally:
            qualify_recovery.docker_available = original_docker
            qualify_recovery.REHEARSALS = original
            qualify_recovery.BY_ID = {item['id']: item for item in original}
        missing = [item for item in result['items'] if item['id'] == 'paired-restore']
        self.assertEqual(len(missing), 1)
        self.assertEqual(missing[0]['status'], 'NOT_RUN')
        self.assertIn('no definition', missing[0]['reason'])

    def test_an_unavailable_runtime_never_yields_a_pass(self):
        original = qualify_recovery.docker_available
        try:
            qualify_recovery.docker_available = lambda: (False, 'no runtime in this test')
            result = qualify_recovery.execute(timeout=1)
        finally:
            qualify_recovery.docker_available = original
        self.assertEqual(result['counts']['PASS'], 0)
        self.assertEqual(result['status'], 'PARTIAL')
        for item in result['items']:
            if item['status'] == 'NOT_RUN' and item['kind'] == 'integration-test':
                self.assertIn('no runtime in this test', item['reason'])

    def test_a_failed_rehearsal_fails_the_file(self):
        original_docker = qualify_recovery.docker_available
        original_test = qualify_recovery.run_test
        try:
            qualify_recovery.docker_available = lambda: (True, 'stub')
            qualify_recovery.run_test = lambda name, timeout: ('FAIL', None, 'stub failure', False)
            result = qualify_recovery.execute(timeout=1)
        finally:
            qualify_recovery.docker_available = original_docker
            qualify_recovery.run_test = original_test
        self.assertEqual(result['status'], 'FAIL')
        self.assertGreater(result['counts']['FAIL'], 0)

    def test_result_shape_is_what_the_summarizer_consumes(self):
        original = qualify_recovery.docker_available
        try:
            qualify_recovery.docker_available = lambda: (False, 'no runtime in this test')
            result = qualify_recovery.execute(timeout=1)
        finally:
            qualify_recovery.docker_available = original
        self.assertEqual(result['schema'], qualify_recovery.SCHEMA)
        self.assertEqual(result['qualification'], 'NOT_QUALIFIED')
        self.assertIsInstance(result['items'], list)
        for item in result['items']:
            self.assertIn(item['status'], ('PASS', 'FAIL', 'NOT_RUN'))
            self.assertIsInstance(item['id'], str)
            self.assertIsInstance(item['reason'], str)

    def test_only_runs_a_single_rehearsal(self):
        original = qualify_recovery.docker_available
        try:
            qualify_recovery.docker_available = lambda: (False, 'no runtime in this test')
            result = qualify_recovery.execute(only='stale-lease', timeout=1)
        finally:
            qualify_recovery.docker_available = original
        self.assertEqual([item['id'] for item in result['items']], ['stale-lease'])

    def test_unknown_only_is_rejected(self):
        with self.assertRaises(ValueError):
            qualify_recovery.execute(only='not-a-rehearsal', timeout=1)


class RollbackRefusals(unittest.TestCase):
    def test_no_nominated_binary_is_not_run(self):
        result = qualify_rollback.rehearsal(None, None, timeout=1)
        self.assertEqual(result['status'], 'NOT_RUN')
        self.assertIn('not accepted as a substitute', result['reason'])
        self.assertEqual(result['qualification'], 'NOT_QUALIFIED')

    def test_the_source_archive_is_rejected_as_a_binary(self):
        archive = qualify_rollback.SOURCE_ARCHIVE
        candidate = archive / 'baseline.json'
        if not candidate.is_file():
            self.skipTest('the retained baseline is not present')
        errors = qualify_rollback.validate_binary(candidate, candidate)
        self.assertTrue(any('source archive' in error for error in errors), errors)

    def test_a_binary_without_a_manifest_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / 'emad-api'
            binary.write_bytes(b'\x7fELF' + b'\0' * 64)
            errors = qualify_rollback.validate_binary(binary, None)
        self.assertTrue(any('manifest' in error for error in errors), errors)

    def test_a_manifest_hash_mismatch_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / 'emad-api'
            binary.write_bytes(b'\x7fELF' + b'\0' * 64)
            manifest = Path(directory) / 'manifest.json'
            manifest.write_text(json.dumps({'sha256': '0' * 64, 'version': 'x'}), encoding='utf-8')
            errors = qualify_rollback.validate_binary(binary, manifest)
        self.assertTrue(any('does not match' in error for error in errors), errors)

    def test_a_valid_binary_without_a_harness_is_not_run_not_pass(self):
        original = qualify_rollback.docker_available
        try:
            qualify_rollback.docker_available = lambda: (True, 'stub runtime')
            with tempfile.TemporaryDirectory() as directory:
                binary = Path(directory) / 'emad-api'
                binary.write_bytes(b'\x7fELF' + b'\0' * 64)
                manifest = Path(directory) / 'manifest.json'
                manifest.write_text(json.dumps({'sha256': qualify_rollback.digest(binary),
                                                'version': 'test'}), encoding='utf-8')
                result = qualify_rollback.rehearsal(binary, manifest, timeout=1)
                expected = qualify_rollback.digest(binary)
        finally:
            qualify_rollback.docker_available = original
        self.assertEqual(result['status'], 'NOT_RUN')
        self.assertIn('no isolated rollback harness', result['reason'])
        self.assertEqual(result['binary']['sha256'], expected)

    def test_verified_families_cover_the_declared_dependencies(self):
        names = {name for name, _ in qualify_rollback.VERIFIED_FAMILIES}
        for required in ('investigations', 'geographic_releases', 'retrieval_profile_state',
                         'review_attestations', 'composite_outputs', 'environment_releases'):
            self.assertIn(required, names)


class SummarizerIntegration(unittest.TestCase):
    """The recovery file must move the summarizer's gates without inventing a pass."""

    def test_recovery_items_reach_the_declared_gate_checks(self):
        qualify = load_module('qualify_map_first')
        profile = qualify.load(qualify.PROFILE)
        recovery = {'items': [{'id': 'stale-lease', 'status': 'PASS', 'observed': 'test',
                               'reason': 'stub'}]}
        gates = qualify.declared_gate_checks({'fullDataset': False}, None, profile, [], recovery)
        by_id = {gate['gate']: gate for gate in gates}
        self.assertEqual(by_id['requiredRecovery.stale-lease']['status'], 'PASS')
        self.assertEqual(by_id['requiredRecovery.paired-restore']['status'], 'NOT_RUN')

    def test_an_absent_recovery_file_leaves_every_rehearsal_not_run(self):
        qualify = load_module('qualify_map_first')
        profile = qualify.load(qualify.PROFILE)
        gates = qualify.declared_gate_checks({'fullDataset': False}, None, profile, [], None)
        recovery_gates = [gate for gate in gates if gate['gate'].startswith('requiredRecovery.')]
        self.assertEqual(len(recovery_gates), len(profile['requiredRecovery']))
        self.assertTrue(all(gate['status'] == 'NOT_RUN' for gate in recovery_gates))


if __name__ == '__main__':
    unittest.main()
