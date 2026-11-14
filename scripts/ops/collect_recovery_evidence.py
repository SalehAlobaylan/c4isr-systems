"""Publish redacted recovery evidence only after the acceptance gates pass.

Run against the supported local installation after the documented test commands.
This does not execute tests or claim live acceptance of gated providers.
"""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[2]
PRIVATE = ROOT / 'private/local'
OUT = ROOT / 'docs/implementation/evidence/recovery'
sys.path.insert(0, str(ROOT / 'scripts/data'))
from install_local import API
from local_connectors import source_id


def load(path):
    return json.loads(Path(path).read_text(encoding='utf-8'))


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def save(path, value):
    Path(path).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')


def main():
    logs = ROOT / 'tmp/recovery'
    checks = {}
    required = {
        'unitTests': ('go-unit-final.log', 'internal/tracks'),
        'productionBuild': ('ui-build-final.log', 'built in'),
        'mockedBrowser': ('regression-final.log', '80 passed'),
        'persistentBrowser': ('persistent-final.log', '3 passed'),
        'backendBrowser': ('real-browser-final.log', 'internal/integration'),
        'validator': ('validator-final.log', 'pass 6'),
        'linuxWorkerPackage': ('package-final.log', 'Independent Linux runtime executable accepted.'),
    }
    for name, (file, marker) in required.items():
        path = logs / file
        text = path.read_text(encoding='utf-8-sig')
        if marker not in text or '\nFAIL' in text or '\n  1 failed' in text:
            raise RuntimeError('Acceptance gate is missing or failed: ' + name)
        checks[name] = {'status': 'PASS', 'logSha256': digest(path)}
    events = [json.loads(line) for line in (logs / 'integration-final.jsonl').read_text(encoding='utf-8-sig').splitlines()]
    package = [e for e in events if 'Test' not in e and e.get('Action') in ('pass', 'fail')]
    if not package or package[-1]['Action'] != 'pass' or any(e.get('Action') == 'fail' for e in events):
        raise RuntimeError('Full integration suite has not passed')
    checks['fullIntegration'] = {'status': 'PASS', 'passed': sum(e.get('Action') == 'pass' and 'Test' in e for e in events),
                                'skipped': sum(e.get('Action') == 'skip' and 'Test' in e for e in events),
                                'elapsedSeconds': package[-1]['Elapsed'], 'logSha256': digest(logs / 'integration-final.jsonl')}
    protected = load(logs / 'protected.json')
    if any(digest(ROOT / file) != expected for file, expected in protected.items()):
        raise RuntimeError('A protected migration/policy file changed')
    backup = Path(load(PRIVATE / 'last-backup.json')['path'])
    restores = sorted((PRIVATE / 'restore-rehearsals').glob('*/acceptance.json'), key=lambda p: p.stat().st_mtime, reverse=True)
    restore = next((load(p) for p in restores if load(p)['backupManifestSha256'] == digest(backup / 'manifest.json')), None)
    if not restore or restore['modelsAndEnvironmentVerified'] != 7 or len(restore['recordings']) < 2:
        raise RuntimeError('Current paired backup lacks complete isolated restore acceptance')
    installation = load(PRIVATE / 'data/installation.json')
    public, author = API(), API(partition='EXERCISE_INPUT')
    real = public.call('GET', '/assessments/' + installation['realProposal']['assessmentId'] + '/revisions')['items'][0]
    if real['status'] != 'PROPOSED':
        raise RuntimeError('The real assessment no longer has the accepted pending-review state')
    result = public.call('GET', '/imagery/jobs/' + installation['imagery']['comparisonId'] + '/result')
    public_ready = public.call('GET', '/platform/readiness')
    simulation_ready = author.call('GET', '/platform/readiness')
    activations = {key: public.call('GET', '/sources/' + source_id(key) + '/activation') for key in ['usgs', 'usgs-history', 'metar', 'gdacs', 'cdse', 'earthsearch', 'firms', 'opensky', 'ais']}
    if any(activations[key]['state'] not in ['NEEDS_CREDENTIALS', 'NEEDS_PERMISSION'] for key in ['ais']):
        raise RuntimeError('Restricted-provider states changed; perform their appropriate acceptance before republishing this ledger')
    firms_live = None
    if activations['firms']['state'] == 'CONFIGURED':
        proof_path = PRIVATE / 'data/firms-activation.json'
        if proof_path.exists(): firms_live = load(proof_path)
        if not firms_live or firms_live.get('liveAdapterVerification') != 'PASS' or firms_live['run']['status'] not in ['SUCCEEDED', 'PARTIAL'] or firms_live['run'].get('retrieval', {}).get('httpStatus') != 200:
            raise RuntimeError('Configured FIRMS source requires its live acceptance and bounded persistent capture receipt')
    elif activations['firms']['state'] not in ['NEEDS_CREDENTIALS', 'NEEDS_PERMISSION']:
        raise RuntimeError('FIRMS is neither explicitly gated nor accepted')
    opensky_live = None
    if activations['opensky']['state'] == 'CONFIGURED':
        proof_path = PRIVATE / 'data/opensky-activation.json'
        if proof_path.exists(): opensky_live = load(proof_path)
        if not opensky_live or opensky_live['run']['status'] != 'SUCCEEDED' or opensky_live['run'].get('retrieval', {}).get('httpStatus') != 200 or not activations['opensky']['health'].get('lastSuccessAt'):
            raise RuntimeError('Configured OpenSky source requires its bounded live acceptance receipt')
    elif activations['opensky']['state'] not in ['NEEDS_CREDENTIALS', 'NEEDS_PERMISSION']:
        raise RuntimeError('OpenSky is neither explicitly gated nor accepted')
    review = load(PRIVATE / 'data/development-review.json')
    if review.get('twoActualHumansVerified') is not False or len(review.get('sameHumanAliasDenied', [])) != 3:
        raise RuntimeError('Development review evidence is incomplete')
    OUT.mkdir(parents=True, exist_ok=True)
    for name in ['development-review', 'failure-acceptance', 'setup-acceptance', 'graphics-acceptance', 'one-hour-accepted-run', 'hardware-acceptance']:
        value = load(PRIVATE / ('data/' + name + '.json'))
        if name == 'one-hour-accepted-run':
            value['measurementConditions'] = 'Virtual-hour test, requested scale 20; COP samples cover the resumed monitor interval. Concurrent browser regressions and image build. Not a one-hour wall-clock soak.'
        save(OUT / (name + '.json'), value)
    save(OUT / 'restore-acceptance.json', restore)
    if opensky_live: save(OUT / 'opensky-activation.json', opensky_live)
    if firms_live: save(OUT / 'firms-activation.json', firms_live)
    videos = list((PRIVATE / 'browser-results').glob('persistent-persistent-Publ-*/video.webm'))
    if len(videos) != 1:
        raise RuntimeError('Passed walkthrough video is unavailable')
    shutil.copyfile(videos[0], OUT / 'emad-walkthrough.webm')
    files = subprocess.run(['git', 'ls-files', '-co', '--exclude-standard'], cwd=ROOT, capture_output=True, text=True, check=True).stdout.splitlines()
    sources = {name: digest(ROOT / name) for name in sorted(set(files)) if (ROOT / name).is_file() and
               (name.startswith(('internal/', 'apps/', 'scripts/', 'api/', 'db/', 'deployments/')) or name in ['Taskfile.yml', 'go.mod', 'go.sum', 'pnpm-lock.yaml'])}
    evidence = {'schema': 'emad.recovery-acceptance.v1', 'acceptedAt': datetime.now(timezone.utc).isoformat(),
                'scope': 'Agreed bounded local recovery; restricted providers explicitly gated; future advanced S4 excluded',
                'checks': checks, 'protectedFilesVerified': len(protected), 'migrationsAppliedThrough': 43,
                'sourceFiles': sources, 'publicReadiness': public_ready, 'simulationReadiness': simulation_ready,
                'providerActivation': activations, 'realProposalStatus': real['status'],
                'imageryResult': {k: result[k] for k in ['validCells', 'maskedCells', 'declineCells', 'meanDelta', 'parameters']},
                'restoredAnalysisSha256': restore['reproducedAnalysisSha256'],
                'artifacts': {p.name: {'sha256': digest(p), 'bytes': p.stat().st_size} for p in OUT.iterdir() if p.is_file() and p.name != 'ACCEPTANCE.json'}}
    save(OUT / 'ACCEPTANCE.json', evidence)
    ledger_path = ROOT / 'docs/implementation/RECOVERY_STATUS.json'
    ledger = load(ledger_path)
    ledger.update(status='ACCEPTED_WITH_DECLARED_EXTERNAL_GATES', acceptedAt=evidence['acceptedAt'], acceptance='HISTORICAL_RECOVERY.md#recovery-acceptance')
    for task in ledger['tasks']:
        task.update(implementation='IMPLEMENTED_BOUNDED_SCOPE', verification='ACCEPTED', installation='INSTALLED_OR_APPLIED', evidence=['evidence/recovery/ACCEPTANCE.json'])
        task['externalPrerequisites'] = []
        if task['id'] == 'P1.5':
            task.update(verification='BOUNDED_ADAPTER_ACCEPTED_LIVE_GATED', installation='READY_EXTERNAL_FEED_GATED', externalPrerequisites=['Valid FIRMS map key; live retrieval not accepted'])
            if firms_live:
                task.update(verification='LIVE_ADAPTER_AND_BOUNDED_PERSISTENT_CAPTURE_ACCEPTED', installation='FIRMS_ENABLED', externalPrerequisites=[])
                task['liveAcceptanceMeaning']='Initial persisted day page accepted. A PARTIAL lookback continues under unchanged pacing; no complete installed query window is claimed until its success is recorded.'
        if task['id'] in ['P3.1', 'P3.2']:
            task.update(verification='BOUNDED_ADAPTER_ACCEPTED_LIVE_GATED', installation='READY_EXTERNAL_FEEDS_GATED', externalPrerequisites=['Approved OpenSky access and OAuth where required; approved AIS use/retention and API key; no persistent live external tracks claimed'])
            if opensky_live:
                task.update(verification='OPENSKY_BOUNDED_LIVE_RETRIEVAL_ACCEPTED_PROJECTION_REGRESSION_ACCEPTED', installation='OPENSKY_ENABLED_AIS_GATED', externalPrerequisites=['Approved AIS use/retention and API key'])
                task['liveAcceptanceMeaning']='OpenSky OAuth and bounded live retrieval accepted; a zero-result snapshot is not evidence that aircraft are absent.'
        if task['id'] == 'P2.3':
            task.update(verification='CANONICAL_ACCOUNT_SEPARATION_ACCEPTED_REAL_REVIEW_PENDING', installation='REAL_PROPOSAL_PENDING_HUMAN_REVIEW', externalPrerequisites=['Actual human interpretation and independent review before real publication; development review tested separately'])
        if task['id'] == 'P2.4':
            task['installation'] = 'GEOGRAPHIC_EXPORT_AND_DEVELOPMENT_REVIEW_EXPORT_ACCEPTED'
        if task['id'] == 'S4.1':
            task['installation'] = 'INSTALLED_SENSOR_PRESETS_ONLY_FURTHER_S4_DEFERRED'
    mappings = [['D01', 'D09'], ['D02', 'D04', 'D11', 'D12'], ['D03', 'D07', 'D10'], ['D03', 'D05'], ['D06', 'D07'], ['D03', 'D08', 'D12'], ['D03', 'D08', 'D11'], ['D12'], ['D01', 'D11']]
    for item, gaps in zip(ledger['recovery'], mappings):
        item.update(implementation='IMPLEMENTED', verification='ACCEPTED', installation='PERSISTENT_LOCAL_ACCEPTED', auditItems=gaps, evidence=['HISTORICAL_RECOVERY.md#recovery-acceptance', 'evidence/recovery/ACCEPTANCE.json'])
        item['externalPrerequisites'] = []
        if item['id']=='R3':
            gated=[name for key,name in [('firms','FIRMS'),('opensky','OpenSky'),('ais','AIS')] if activations[key]['state']!='CONFIGURED']
            if gated: item['externalPrerequisites']=[', '.join(gated)+' access remains explicitly gated']
    save(ledger_path, ledger)
    print('Published redacted acceptance evidence and separate implementation/verification/installation ledger.')


if __name__ == '__main__':
    main()
