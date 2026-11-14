"""Rehearse an actual nominated rollback binary, isolated from any installation.

The MAP-6 plan requires compatibility with the real installed rollback binary,
not with a source snapshot. This command therefore refuses to substitute the
retained source archive under private/qualification for a binary: when nothing
has been nominated it records NOT_RUN with the reason, and it never reports a
compatibility it did not exercise.

The rehearsal provisions the nominated binary against disposable services,
applies the forward migrations to the current schema, rolls back with the
nominated binary and then recovers forward, verifying that cases, geographic
releases, profile mappings, canonical review identity, briefs and environment
dependencies survive with matching object hashes.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
SCHEMA = 'emad.map-first-rollback-rehearsal.v1'
# A source snapshot is not a rollback binary. Naming it here keeps the refusal
# explicit instead of leaving the distinction to prose.
SOURCE_ARCHIVE = ROOT / 'private' / 'qualification' / 'rollback-baseline-2026-10-29'

# What the rehearsal must find intact after rollback and forward recovery. Each
# entry is a durable family the release depends on.
VERIFIED_FAMILIES = [
    ('investigations', 'Cases and their immutable revisions'),
    ('geographic_releases', 'Activated geographic releases and editions'),
    ('retrieval_profile_state', 'Profile mappings and their execution grants'),
    ('review_attestations', 'Canonical review identity'),
    ('composite_outputs', 'Briefs and their frozen manifests'),
    ('environment_releases', 'Environment definitions and releases'),
]


def load(path):
    return json.loads(Path(path).read_text(encoding='utf-8-sig'))


def save(path, value):
    Path(path).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')


def digest(path):
    value = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for chunk in iter(lambda: stream.read(1 << 20), b''):
            value.update(chunk)
    return value.hexdigest()


def docker_available():
    try:
        probe = subprocess.run(['docker', 'info', '--format', '{{.ServerVersion}}'],
                               capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.SubprocessError):
        return False, 'the docker CLI is unavailable'
    if probe.returncode != 0:
        detail = (probe.stderr or probe.stdout).strip().splitlines()
        return False, 'the docker daemon is unreachable' + (': ' + detail[0] if detail else '')
    return True, 'docker server ' + probe.stdout.strip()


def validate_binary(binary, manifest):
    """Reject anything that is not an identified, hashed executable."""
    errors = []
    if not binary.is_file():
        errors.append('the nominated rollback binary does not exist')
        return errors
    if binary.stat().st_size == 0:
        errors.append('the nominated rollback binary is empty')
    try:
        if SOURCE_ARCHIVE in binary.resolve().parents:
            errors.append('the nominated path is the retained source archive, not an installed rollback binary')
    except OSError:
        pass
    if manifest is None:
        errors.append('no manifest was supplied; the binary cannot be identified')
        return errors
    if not manifest.is_file():
        errors.append('the supplied manifest does not exist')
        return errors
    try:
        recorded = load(manifest)
    except (OSError, ValueError):
        errors.append('the supplied manifest is not valid JSON')
        return errors
    expected = recorded.get('sha256') or recorded.get('binarySha256')
    if not expected:
        errors.append('the manifest records no binary hash')
    elif expected != digest(binary):
        errors.append('the binary hash does not match its manifest')
    if not recorded.get('version') and not recorded.get('buildId'):
        errors.append('the manifest records no version or build identity')
    return errors


def rehearsal(binary, manifest, timeout):
    """Return the rehearsal result. Never reports an unexercised compatibility."""
    base = {
        'schema': SCHEMA,
        'recordedAt': datetime.now(timezone.utc).isoformat(),
        'qualification': 'NOT_QUALIFIED',
        'rehearsal': 'actual-rollback-binary-forward-recovery',
        'scope': 'Isolated disposable services only. No shared installation was migrated, rolled back or restarted.',
        'hardware': {'platform': platform.platform(), 'logicalCPUs': os.cpu_count()},
        'verifiedFamilies': [{'family': name, 'meaning': meaning} for name, meaning in VERIFIED_FAMILIES],
        'meaning': ('A source snapshot is not an installed rollback binary. This rehearsal reports only '
                    'what it exercised against the nominated binary.'),
    }
    if binary is None:
        base.update({
            'status': 'NOT_RUN',
            'observed': None,
            'reason': ('No rollback binary was nominated. The retained source archive at '
                       'private/qualification/rollback-baseline-2026-10-29 is a repository baseline, '
                       'not the installed rollback binary, and is not accepted as a substitute.'),
            'nextStep': 'Nominate the installed rollback binary and its manifest, then rerun with --binary and --manifest.',
        })
        return base
    errors = validate_binary(binary, manifest)
    if errors:
        base.update({'status': 'FAIL', 'observed': None, 'reason': '; '.join(errors)})
        return base
    available, detail = docker_available()
    if not available:
        base.update({'status': 'NOT_RUN', 'observed': None,
                     'reason': 'isolated services are unavailable: ' + detail})
        return base
    # The rehearsal needs the disposable-service harness. It is not implemented
    # yet, and reporting a compatibility without exercising it would be a lie.
    base.update({
        'status': 'NOT_RUN',
        'observed': None,
        'reason': ('The binary and manifest are valid, but no isolated rollback harness exists yet to '
                   'provision the binary, migrate forward, roll back and recover while comparing object hashes.'),
        'nextStep': 'Implement the isolated rollback harness before this rehearsal can report a result.',
        'binary': {'path': str(binary), 'sha256': digest(binary), 'bytes': binary.stat().st_size},
        'manifest': {'path': str(manifest), 'sha256': digest(manifest)},
    })
    return base


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, help='result path; refuses to overwrite')
    parser.add_argument('--binary', type=Path, help='the nominated installed rollback binary')
    parser.add_argument('--manifest', type=Path, help='the manifest identifying that binary')
    parser.add_argument('--timeout', type=int, default=1800)
    parser.add_argument('--list', action='store_true', help='print what the rehearsal verifies')
    args = parser.parse_args()
    if args.list:
        print('This rehearsal verifies that a nominated rollback binary preserves:')
        for name, meaning in VERIFIED_FAMILIES:
            print('  %-26s %s' % (name, meaning))
        print('\nIt refuses to substitute the retained source archive for an installed binary.')
        return 0
    if not args.output:
        parser.error('--output is required unless --list is used')
    if args.output.exists():
        parser.error('output exists; choose a new result path')
    result = rehearsal(args.binary, args.manifest, args.timeout)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    save(args.output, result)
    print(json.dumps({'status': result['status'], 'reason': result['reason'],
                      'output': str(args.output)}))
    return 0 if result['status'] == 'PASS' else 2


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print('Rollback rehearsal failed: %s. No gate was promoted.' % error, file=sys.stderr)
        sys.exit(1)
