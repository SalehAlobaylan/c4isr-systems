"""Supported local EMAD lifecycle. Never resets a database or prints secrets."""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import secrets
import signal
import socket
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
PRIVATE = ROOT / 'private' / 'local'
CREDS = PRIVATE / 'credentials'
WINDOWS = os.name == 'nt'
EXE = '.exe' if WINDOWS else ''
PYTHON = ROOT / '.venv-recovery' / ('Scripts/python.exe' if WINDOWS else 'bin/python')
COMPOSE = ['docker', 'compose', '-f', 'deployments/docker-compose.yml', '-f', 'deployments/local/docker-compose.yml']
APPS = ['c4isr-server', 'emad-local', 'emad-imagery', 'emad-assets', 'emad-recordings', 'emad-runtime', 'emad-connectors']


def write_new(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open('x', encoding='utf-8', newline='\n') as file:
        os.chmod(path, 0o600)
        file.write(data)


def run(args, env=None, **kwargs):
    return subprocess.run([str(v) for v in args], cwd=ROOT, env=env, check=True, **kwargs)


def configuration():
    PRIVATE.mkdir(parents=True, exist_ok=True)
    if WINDOWS:
        # chmod does not establish a private Windows DACL. Limit this installer's
        # own directory to its user, SYSTEM and local administrators; children
        # inherit this ACL. No sibling workspace files are changed.
        identity=subprocess.run(['whoami','/user','/fo','csv','/nh'],capture_output=True,text=True,check=True)
        sid=next(csv.reader(identity.stdout.strip().splitlines()))[1]
        subprocess.run(['icacls',str(PRIVATE),'/inheritance:r','/grant:r','*'+sid+':(OI)(CI)F','*S-1-5-18:(OI)(CI)F','*S-1-5-32-544:(OI)(CI)F'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True)
    else:
        os.chmod(PRIVATE,0o700)
    cfgfile = PRIVATE / 'config.json'
    if not cfgfile.exists():
        write_new(cfgfile, json.dumps({'schema': 'emad.local.v1', 'apiPort': 8081, 'webPort': 5173,
                                      'bucket': 'emad-local', 'region': 'garage',
                                      'accessKey': 'GK' + secrets.token_hex(16),
                                      'secretKey': secrets.token_hex(32), 'rpcSecret': secrets.token_hex(32)}, indent=2))
    cfg = json.loads(cfgfile.read_text(encoding='utf-8'))
    if cfg.get('schema') != 'emad.local.v1':
        raise RuntimeError('Unsupported private local configuration')
    garage = PRIVATE / 'garage.env'
    if not garage.exists():
        write_new(garage, '\n'.join(['GARAGE_DEFAULT_ACCESS_KEY=' + cfg['accessKey'],
                                    'GARAGE_DEFAULT_SECRET_KEY=' + cfg['secretKey'],
                                    'GARAGE_DEFAULT_BUCKET=' + cfg['bucket'], '']))
    toml = PRIVATE / 'garage.toml'
    if not toml.exists():
        write_new(toml, 'metadata_dir = "/var/lib/garage/meta"\ndata_dir = "/var/lib/garage/data"\n'
                  'db_engine = "sqlite"\nreplication_factor = 1\nrpc_bind_addr = "[::]:3901"\n'
                  'rpc_public_addr = "127.0.0.1:3901"\nrpc_secret = "' + cfg['rpcSecret'] + '"\n'
                  '[s3_api]\ns3_region = "garage"\napi_bind_addr = "[::]:3900"\n')
    return cfg


def environment(cfg):
    env = os.environ.copy()
    env.update({'C4ISR_ENV': 'development', 'C4ISR_DATABASE_AUTH': 'true', 'C4ISR_AUTH_REQUIRED': 'true',
                'C4ISR_HTTP_ADDR': '127.0.0.1:' + str(cfg['apiPort']),
                'C4ISR_ARTIFACT_S3_ENDPOINT': 'http://127.0.0.1:3900',
                'C4ISR_ARTIFACT_S3_BUCKET': cfg['bucket'], 'C4ISR_ARTIFACT_S3_REGION': cfg['region'],
                'C4ISR_ARTIFACT_S3_ACCESS_KEY': cfg['accessKey'], 'C4ISR_ARTIFACT_S3_SECRET_KEY': cfg['secretKey'],
                'C4ISR_PUBLIC_CONNECTORS_FILE': str(PRIVATE / 'generated-connectors.json'),
                'VITE_API_PROXY_TARGET': 'http://127.0.0.1:' + str(cfg['apiPort']),
                'C4ISR_LOG_LEVEL': 'warn'})
    env.setdefault('C4ISR_DATABASE_URL', 'postgres://c4isr:c4isr@localhost:5433/c4isr?sslmode=disable')
    # Preserve the existing explicit development bootstrap credential. New
    # capabilities use separately provisioned database identities.
    env.setdefault('C4ISR_AUTH_TOKENS', 'operator-01=dev-operator-token')
    descriptor = ROOT / 'bin/runtime.json'
    if descriptor.exists():
        env['C4ISR_RUNTIME_BUILDS'] = json.loads(descriptor.read_text())['runtimeBuild']
    sys.path.insert(0, str(ROOT / 'scripts/data'))
    from local_connectors import configure
    entries, states = configure(env)
    (PRIVATE / 'generated-connectors.json').write_text(json.dumps(entries, indent=2), encoding='utf-8')
    (PRIVATE / 'provider-status.json').write_text(json.dumps(states, indent=2), encoding='utf-8')
    return env


def request(cfg, path, token=None, method='GET', body=None, resource=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token.strip()
    if resource:
        headers.update({'X-EMAD-Workspace-ID': 'default', 'X-EMAD-Environment-ID': 'local', 'X-EMAD-Partition': resource})
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(f"http://127.0.0.1:{cfg['apiPort']}" + path, headers=headers, data=data, method=method)
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.load(response)


def port_free(port):
    with socket.socket() as sock:
        try:
            sock.bind(('127.0.0.1', port))
            return True
        except OSError:
            return False


def spawn(args, env, name):
    logs = PRIVATE / 'logs'
    logs.mkdir(exist_ok=True)
    output = (logs / (name + '.log')).open('ab')
    try:
        process = subprocess.Popen([str(v) for v in args], cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT,
                                   creationflags=subprocess.CREATE_NO_WINDOW if WINDOWS else 0)
    finally:
        output.close()
    return process


def stop(process):
    if process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)


def wait_api(cfg, process):
    for _ in range(60):
        if process.poll() is not None:
            raise RuntimeError('API exited; inspect private/local/logs/api.log')
        try:
            request(cfg, '/health')
            return
        except (OSError, urllib.error.URLError):
            time.sleep(.5)
    raise RuntimeError('API did not become ready; inspect the local API log')


def ensure_python():
    if not PYTHON.exists():
        run([sys.executable, '-m', 'venv', ROOT / '.venv-recovery'])
    requirements = ROOT / 'scripts/data/requirements-recovery.txt'
    fingerprint = hashlib.sha256(requirements.read_bytes()).hexdigest()
    marker = PRIVATE / 'python-requirements.sha256'
    if not marker.exists() or marker.read_text() != fingerprint:
        run([PYTHON, '-m', 'pip', 'install', '-r', requirements])
        marker.write_text(fingerprint)


def setup(cfg, infrastructure_only=False):
    if not port_free(cfg['apiPort']) or not port_free(cfg['webPort']):
        raise RuntimeError('Stop task dev (or run task local:stop) before setup. Binaries and migrations were not changed while the application was running.')
    env = environment(cfg)
    run([*COMPOSE, 'up', '-d'])
    run(['task', 'db:migrate'], env=env)
    (ROOT / 'bin').mkdir(exist_ok=True)
    for app in APPS:
        run(['go', 'build', '-trimpath', '-o', ROOT / 'bin' / (app + EXE), './apps/' + app])
    result = run([ROOT / 'bin' / ('emad-runtime' + EXE), '-describe'], capture_output=True, text=True)
    (ROOT / 'bin/runtime.json').write_text(result.stdout, encoding='utf-8')
    env = environment(cfg)
    run([ROOT / 'bin' / ('emad-local' + EXE), '-credentials-dir', CREDS], env=env)
    ensure_python()
    # A successful bucket write/read, rather than a listening TCP port, proves
    # Garage's asynchronous key/bucket initialization finished.
    # Credentials travel in the child's environment, never process arguments.
    probe = "import boto3,os;c=boto3.client('s3',endpoint_url=os.environ['C4ISR_ARTIFACT_S3_ENDPOINT'],region_name='garage',aws_access_key_id=os.environ['C4ISR_ARTIFACT_S3_ACCESS_KEY'],aws_secret_access_key=os.environ['C4ISR_ARTIFACT_S3_SECRET_KEY']);b=os.environ['C4ISR_ARTIFACT_S3_BUCKET'];c.put_object(Bucket=b,Key='platform/readiness',Body=b'emad-ready');assert c.get_object(Bucket=b,Key='platform/readiness')['Body'].read()==b'emad-ready'"
    for attempt in range(20):
        result = subprocess.run([str(PYTHON), '-c', probe], env=env, cwd=ROOT, capture_output=True)
        if result.returncode == 0:
            break
        if attempt == 19:
            raise RuntimeError('Private object store write/read failed; check Garage configuration')
        time.sleep(1)
    if not infrastructure_only:
        if not port_free(cfg['apiPort']):
            raise RuntimeError('API port occupied. Stop the existing local development supervisor before setup; no process was killed.')
        api = spawn([ROOT / 'bin' / ('c4isr-server' + EXE)], env, 'api')
        try:
            wait_api(cfg, api)
            run([PYTHON, ROOT / 'scripts/data/install_local.py'], env=env)
        finally:
            stop(api)
    print(('Infrastructure and identities ready; content installation was skipped.' if infrastructure_only else 'Local setup complete.')+' Start the application with: task dev')


def heartbeat_loop(cfg, name, feature, process, done):
    token = (CREDS / (name + '.token')).read_text().strip()
    while not done.wait(3):
        current = process['process'] if isinstance(process, dict) else process
        if current.poll() is not None:
            continue
        try:
            request(cfg, '/api/v1/platform/worker-heartbeats', token, 'POST', {'feature': feature})
        except (OSError, urllib.error.URLError):
            pass


def develop(cfg):
    for port in [cfg['apiPort'], cfg['webPort']]:
        if not port_free(port):
            raise RuntimeError(f'Port {port} is occupied. Stop its existing development process or set a different port in private/local/config.json.')
    env = environment(cfg)
    if not (CREDS / 'local-analyst.token').exists():
        raise RuntimeError('Run task local:setup first')
    # The existing Vite injection is restricted to this local development
    # process. Production builds receive no credential or private config.
    env['VITE_API_TOKEN'] = (CREDS / 'local-analyst.token').read_text().strip()
    supervisor={'pid':os.getpid(),'instance':secrets.token_hex(8)}
    (PRIVATE/'supervisor.json').write_text(json.dumps(supervisor),encoding='utf-8')
    (PRIVATE/'stop-request.json').unlink(missing_ok=True)
    processes, done = [], threading.Event()
    workers = []
    def worker(args, name, feature=None):
        record = dict(args=args, name=name, feature=feature, process=spawn(args,env,name), started=time.monotonic(), failures=0, retryAt=0)
        workers.append(record)
        if feature:
            threading.Thread(target=heartbeat_loop,args=(cfg,name,feature,record,done),daemon=True).start()
    try:
        api = spawn([ROOT / 'bin' / ('c4isr-server' + EXE)], env, 'api'); processes.append(api)
        wait_api(cfg, api)
        specs = [('worker-imagery', 'emad-imagery', 'imagery'), ('worker-assets', 'emad-assets', 'assets'),
                 ('worker-recordings', 'emad-recordings', 'recordings')]
        for name, app, feature in specs:
            worker([ROOT/'bin'/(app+EXE),'-token-file',CREDS/(name+'.token')],name,feature)
        worker([ROOT/'bin'/('emad-runtime'+EXE),'-worker','-url',f"http://127.0.0.1:{cfg['apiPort']}",'-token-file',CREDS/'worker-runtime.token'],'worker-runtime')
        worker([PYTHON,ROOT/'scripts/data/capture_worker.py'],'worker-preparation')
        states = json.loads((PRIVATE / 'provider-status.json').read_text())
        from local_connectors import source_id
        for connector, state in states.items():
            if state['state'] != 'CONFIGURED' or connector == 'usgs-history': continue
            name = 'worker-' + connector
            worker([ROOT/'bin'/('emad-connectors'+EXE),'-token-file',CREDS/(name+'.token'),'-source',source_id(connector)],name,'connectors')
        # Launch Vite directly with Node so terminating this supervisor does not
        # leave a package-manager child holding the frontend port.
        processes.append(spawn(['node', ROOT / 'apps/operator-ui/node_modules/vite/bin/vite.js', ROOT / 'apps/operator-ui', '--host', '127.0.0.1', '--port', str(cfg['webPort']), '--strictPort'], dict(env, INIT_CWD=str(ROOT)), 'web'))
        print(f"EMAD: http://127.0.0.1:{cfg['webPort']} · API: http://127.0.0.1:{cfg['apiPort']}", flush=True)
        print('Development analyst / author session. Other credentials: private/local/credentials. Logs: private/local/logs.', flush=True)
        while not done.wait(1):
            marker=PRIVATE/'stop-request.json'
            if marker.exists():
                try:
                    if json.loads(marker.read_text(encoding='utf-8'))==supervisor:return
                except (OSError,ValueError):pass
            failed = [p for p in processes if p.poll() is not None]
            now=time.monotonic()
            for record in workers:
                if record['process'].poll() is None: continue
                if not record['retryAt']:
                    if now-record['started']>60: record['failures']=0
                    record['failures']+=1
                    delay=min(60,2**min(record['failures'],6))
                    record['retryAt']=now+delay
                    print(f"{record['name']} unavailable; retry in {delay}s. Inspect private/local/logs/{record['name']}.log",flush=True)
                elif now>=record['retryAt']:
                    record['process']=spawn(record['args'],env,record['name'])
                    record['started']=now;record['retryAt']=0
            if failed:
                raise RuntimeError('A development component exited; inspect private/local/logs. All components are stopping.')
    finally:
        done.set()
        for record in workers: stop(record['process'])
        for p in reversed(processes):
            stop(p)
        (PRIVATE/'supervisor.json').unlink(missing_ok=True)
        (PRIVATE/'stop-request.json').unlink(missing_ok=True)


def stop_development(cfg):
    path=PRIVATE/'supervisor.json'
    if not path.exists():
        if port_free(cfg['apiPort']) and port_free(cfg['webPort']):print('Supported development processes are stopped.');return
        raise RuntimeError('No supported supervisor is registered. The occupied port belongs to another process; it was not killed.')
    state=json.loads(path.read_text(encoding='utf-8'))
    (PRIVATE/'stop-request.json').write_text(json.dumps(state),encoding='utf-8')
    for _ in range(40):
        if not path.exists() and port_free(cfg['apiPort']) and port_free(cfg['webPort']):print('EMAD development processes stopped; persistent infrastructure and content remain.');return
        time.sleep(.5)
    raise RuntimeError('Supervisor did not finish shutdown. Inspect its terminal and private/local/logs; no unrelated process was killed.')


def doctor(cfg):
    print('Configuration: private/local/config.json (secrets redacted)')
    run(['docker', 'info', '--format', '{{.ServerVersion}}'])
    run([*COMPOSE, 'ps'])
    try:
        token = (CREDS / 'local-analyst.token').read_text().strip()
        report = request(cfg, '/api/v1/platform/readiness', token, resource='PUBLIC_EVIDENCE')
        print(json.dumps(report, indent=2))
        print('Simulation readiness:')
        print(json.dumps(request(cfg,'/api/v1/platform/readiness',token,resource='EXERCISE_INPUT'),indent=2))
        print('Shared API address: 127.0.0.1:'+str(cfg['apiPort'])+' (explicit IPv4, independent of localhost resolution)')
        invalid=[]
        for file in sorted(CREDS.glob('*.token')):
            try:
                identity=request(cfg,'/api/v1/auth/me',file.read_text(encoding='utf-8').strip())
                print(file.stem+': VALID · '+identity['principal']['kind']+' · '+identity['resource']['partition'])
            except (OSError,urllib.error.URLError):
                invalid.append(file.stem);print(file.stem+': UNAVAILABLE_OR_EXPIRED · credential path '+str(file))
        if invalid:raise RuntimeError('One or more scoped identities failed authentication. Existing credentials were not overwritten.')
    except (OSError, urllib.error.URLError):
        raise RuntimeError('API or development credential unavailable. Run task local:setup and task dev; check port and credential expiry.')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['setup', 'dev', 'doctor','stop'])
    parser.add_argument('--infrastructure-only', action='store_true')
    args = parser.parse_args()
    cfg = configuration()
    if args.action == 'setup': setup(cfg, args.infrastructure_only)
    elif args.action == 'dev': develop(cfg)
    elif args.action=='stop':stop_development(cfg)
    else: doctor(cfg)


if __name__ == '__main__':
    try:
        main()
    except KeyboardInterrupt:
        pass
    except Exception as error:
        print('EMAD local:', error, file=sys.stderr)
        sys.exit(1)
