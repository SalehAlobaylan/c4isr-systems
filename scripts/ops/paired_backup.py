"""Paired local database/object backup and isolated restore rehearsal.

Backups contain private credentials and source material. They remain under the
ignored private directory. A completed manifest is written only after every
object and the consistent database dump have been hashed.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import sys
import tarfile
import time
from datetime import datetime,timezone
import boto3
import requests

ROOT=Path(__file__).resolve().parents[2]
PRIVATE=ROOT/'private/local'
GARAGE='dxflrs/garage:v2.4.1@sha256:9c96caa2612d3411acc5b0e6701fb238dbfba33e533a6d7d3d811a4b12d0d020'
def digest(path):
 h=hashlib.sha256()
 with Path(path).open('rb') as f:
  for chunk in iter(lambda:f.read(1024*1024),b''):h.update(chunk)
 return h.hexdigest()
def save(path,value):
 path=Path(path);path.write_text(json.dumps(value,indent=2),encoding='utf-8');os.chmod(path,0o600)
def command(args,**kwargs):
 return subprocess.run([str(x) for x in args],cwd=ROOT,check=True,**kwargs)
def client(cfg,endpoint='http://127.0.0.1:3900'):
 return boto3.client('s3',endpoint_url=endpoint,region_name=cfg['region'],aws_access_key_id=cfg['accessKey'],aws_secret_access_key=cfg['secretKey'])
def free(port):
 with socket.socket() as s:
  try:s.bind(('127.0.0.1',port));return True
  except OSError:return False
def check(directory):
 directory=Path(directory).resolve();manifest=json.loads((directory/'manifest.json').read_text(encoding='utf-8'))
 if manifest.get('schema')!='emad.paired-backup.v1':raise RuntimeError('Unsupported paired backup')
 for item in [manifest['database'],manifest['privateFiles'],*manifest['objects']]:
  file=(directory/item['file']).resolve()
  if not file.is_relative_to(directory) or not file.is_file() or file.stat().st_size!=item['bytes'] or digest(file)!=item['sha256']:raise RuntimeError('Backup integrity failed before restore')
 return manifest
def backup():
 cfg=json.loads((PRIVATE/'config.json').read_text(encoding='utf-8'))
 if not free(cfg['apiPort']) or not free(cfg['webPort']):raise RuntimeError('Stop task dev before the coordinated backup. No running process was stopped.')
 stamp=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
 directory=PRIVATE/'backups'/stamp;directory.mkdir(parents=True,exist_ok=False);os.chmod(directory,0o700)
 s3=client(cfg);objects=[]
 # The supported API and workers must be stopped. The dump is independently
 # consistent; immutable objects are then copied without any deletion job.
 with (directory/'database.dump').open('wb') as file:
  command(['docker','exec','c4isr-postgres','pg_dump','-U','c4isr','-d','c4isr','--format=custom','--no-owner'],stdout=file)
 (directory/'objects').mkdir()
 for page in s3.get_paginator('list_objects_v2').paginate(Bucket=cfg['bucket']):
  for item in page.get('Contents',[]):
   key=item['Key'];filename='objects/'+hashlib.sha256(key.encode()).hexdigest()
   target=directory/filename;response=s3.get_object(Bucket=cfg['bucket'],Key=key)
   with target.open('xb') as file:
    for block in iter(lambda:response['Body'].read(1024*1024),b''):file.write(block)
   if target.stat().st_size!=item['Size']:raise RuntimeError('Object changed or was truncated during coordinated backup')
   objects.append({'key':key,'file':filename,'bytes':target.stat().st_size,'sha256':digest(target),'mediaType':response.get('ContentType','application/octet-stream')})
   os.chmod(target,0o600)
 private=directory/'private.tar.gz'
 with tarfile.open(private,'w:gz') as tar:
  for name in ['config.json','garage.toml','garage.env','credentials','data','providers.json','opensky-client.json','firms.key']:
   source=PRIVATE/name
   if source.exists():tar.add(source,arcname=name)
 os.chmod(private,0o600)
 entry=lambda name:{'file':name,'bytes':(directory/name).stat().st_size,'sha256':digest(directory/name)}
 manifest={'schema':'emad.paired-backup.v1','createdAt':datetime.now(timezone.utc).isoformat(),'consistency':'Supported API/workers quiesced; consistent pg_dump plus complete immutable object copy; no retention/deletion process active.','database':entry('database.dump'),'privateFiles':entry('private.tar.gz'),'objects':objects,'sourceBucket':cfg['bucket'],'objectCount':len(objects),'objectBytes':sum(x['bytes'] for x in objects)}
 save(directory/'manifest.json',manifest);check(directory)
 save(PRIVATE/'last-backup.json',{'path':str(directory),'objectCount':len(objects)})
 print(f'Paired backup verified: {directory} · {len(objects)} objects · {manifest["objectBytes"]} object bytes',flush=True)
 return directory
def port(container,internal):
 raw=command(['docker','inspect',container],capture_output=True,text=True)
 return int(json.loads(raw.stdout)[0]['NetworkSettings']['Ports'][str(internal)+'/tcp'][0]['HostPort'])
def wait(fn,seconds=60):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  try:
   if fn():return
  except Exception:pass
  time.sleep(.5)
 raise RuntimeError('Isolated restore component did not become ready')
def restore(directory):
 directory=Path(directory).resolve();m=check(directory)
 token=secrets.token_hex(5);area=PRIVATE/'restore-rehearsals'/token;area.mkdir(parents=True);os.chmod(area,0o700)
 extracted=area/'private';extracted.mkdir()
 with tarfile.open(directory/m['privateFiles']['file'],'r:gz') as tar:
  # Python's data filter rejects traversals, symlinks escaping the destination,
  # special devices and absolute paths. The destination is a fresh directory.
  tar.extractall(extracted,filter='data')
 cfg=json.loads((extracted/'config.json').read_text(encoding='utf-8'))
 pg='emad-restore-pg-'+token;garage='emad-restore-objects-'+token
 volumes=['emad-restore-'+token+'-'+v for v in ['pg','meta','objects']]
 containers=[];api=None;report={}
 try:
  password=secrets.token_hex(24)
  (area/'postgres.env').write_text('POSTGRES_USER=c4isr\nPOSTGRES_DB=c4isr\nPOSTGRES_PASSWORD='+password+'\n',encoding='utf-8');os.chmod(area/'postgres.env',0o600)
  command(['docker','run','-d','--name',pg,'--label','emad.restore='+token,'--env-file',area/'postgres.env','-p','127.0.0.1::5432','-v',volumes[0]+':/var/lib/postgresql/data','imresamu/postgis:17-3.5'],stdout=subprocess.DEVNULL);containers.append(pg)
  command(['docker','run','-d','--name',garage,'--label','emad.restore='+token,'--env-file',extracted/'garage.env','-p','127.0.0.1::3900','--mount',f'type=bind,source={extracted/"garage.toml"},target=/etc/garage.toml,readonly','-v',volumes[1]+':/var/lib/garage/meta','-v',volumes[2]+':/var/lib/garage/data',GARAGE,'/garage','server','--single-node','--default-bucket'],stdout=subprocess.DEVNULL);containers.append(garage)
  # The entrypoint briefly starts a Unix-socket-only initialization server.
  # Require TCP so restore cannot race its intentional shutdown.
  wait(lambda:subprocess.run(['docker','exec',pg,'pg_isready','-h','127.0.0.1','-U','c4isr','-d','c4isr'],capture_output=True).returncode==0)
  command(['docker','exec',pg,'psql','-U','c4isr','-d','c4isr','-v','ON_ERROR_STOP=1','-c','CREATE ROLE emad_data NOLOGIN NOSUPERUSER NOBYPASSRLS;'],stdout=subprocess.DEVNULL)
  # PostGIS images preload their default database with tiger/topology schemas.
  # Restore into a new template0 database, preserving the full dump's DDL/ACLs.
  command(['docker','exec',pg,'createdb','-U','c4isr','--template=template0','emad_restore'],stdout=subprocess.DEVNULL)
  command(['docker','cp',directory/m['database']['file'],pg+':/tmp/recovery.dump'],stdout=subprocess.DEVNULL)
  command(['docker','exec',pg,'pg_restore','-U','c4isr','-d','emad_restore','--no-owner','--exit-on-error','/tmp/recovery.dump'],stdout=subprocess.DEVNULL)
  endpoint='http://127.0.0.1:'+str(port(garage,3900));s3=client(cfg,endpoint)
  wait(lambda:s3.head_bucket(Bucket=cfg['bucket']) is not None)
  for item in m['objects']:
   with (directory/item['file']).open('rb') as file:s3.put_object(Bucket=cfg['bucket'],Key=item['key'],Body=file,ContentType=item['mediaType'])
   raw=s3.get_object(Bucket=cfg['bucket'],Key=item['key'])['Body'];h=hashlib.sha256();size=0
   for block in iter(lambda:raw.read(1024*1024),b''):size+=len(block);h.update(block)
   if h.hexdigest()!=item['sha256'] or size!=item['bytes']:raise RuntimeError('Restored object integrity failure')
  with socket.socket() as sock:sock.bind(('127.0.0.1',0));apiport=sock.getsockname()[1]
  env=os.environ.copy();env.update({'C4ISR_ENV':'development','C4ISR_DATABASE_AUTH':'true','C4ISR_AUTH_REQUIRED':'true','C4ISR_AUTH_TOKENS':'operator-01=dev-operator-token','C4ISR_DATABASE_URL':f'postgres://c4isr:{password}@127.0.0.1:{port(pg,5432)}/emad_restore?sslmode=disable','C4ISR_HTTP_ADDR':'127.0.0.1:'+str(apiport),'C4ISR_ARTIFACT_S3_ENDPOINT':endpoint,'C4ISR_ARTIFACT_S3_REGION':cfg['region'],'C4ISR_ARTIFACT_S3_BUCKET':cfg['bucket'],'C4ISR_ARTIFACT_S3_ACCESS_KEY':cfg['accessKey'],'C4ISR_ARTIFACT_S3_SECRET_KEY':cfg['secretKey'],'C4ISR_LOG_LEVEL':'warn','C4ISR_PUBLIC_CONNECTORS_FILE':''})
  for key in ['C4ISR_DATABASE_URL_FILE','C4ISR_AUTH_TOKENS_FILE','C4ISR_ARTIFACT_S3_ACCESS_KEY_FILE','C4ISR_ARTIFACT_S3_SECRET_KEY_FILE']:env.pop(key,None)
  # Older immutable backups are restored first, then advanced through the
  # existing forward-only migrations in this isolated database.
  command(['task','db:migrate'],env=env,stdout=subprocess.DEVNULL)
  log=(area/'api.log').open('wb');api=subprocess.Popen([str(ROOT/'bin'/('c4isr-server.exe' if os.name=='nt' else 'c4isr-server'))],cwd=ROOT,env=env,stdout=log,stderr=log,creationflags=subprocess.CREATE_NO_WINDOW if os.name=='nt' else 0)
  origin='http://127.0.0.1:'+str(apiport)
  wait(lambda:requests.get(origin+'/health',timeout=2).status_code==200)
  def call(path,partition='PUBLIC_EVIDENCE',name='local-analyst',run=None,blob=False,method='GET',body=None):
   headers={'Authorization':'Bearer '+(extracted/'credentials'/(name+'.token')).read_text(encoding='utf-8').strip(),'X-EMAD-Workspace-ID':'default','X-EMAD-Environment-ID':'local','X-EMAD-Partition':partition}
   if run:headers['X-EMAD-Run-ID']=run
   response=requests.request(method,origin+'/api/v1'+path,headers=headers,json=body,timeout=60,allow_redirects=False)
   if response.status_code not in [200,201,202]:raise RuntimeError('Restored API acceptance failed: '+path+' HTTP '+str(response.status_code))
   return response.content if blob else response.json()
  refs=call('/reference-datasets');receipt=json.loads((extracted/'data/installation.json').read_text(encoding='utf-8'))
  navigation_file=extracted/'data/geographic-navigation.json'
  navigation_proof=None
  if navigation_file.exists():
   expected=json.loads(navigation_file.read_text(encoding='utf-8'))['editions']
   navigation=call('/geography/regions')
   for edition in expected:
    features=[f for f in navigation['features'] if f['properties'].get('editionId')==edition['id']]
    if len(features)!=edition['features'] or any(f['properties'].get('sha256')!=edition['sha256'] for f in features):raise RuntimeError('Restored geographic edition/count/hash mismatch')
    if not any(o['key'].startswith('installation/sources/'+edition['receipt']['sha256']+'/') for o in m['objects']):raise RuntimeError('Retained navigation source archive is missing')
   navigation_proof={'features':len(navigation['features']),'editions':len(expected),'retainedSourcesVerified':True}
  captures=call('/imagery/captures')['items']
  for capture in captures:
   if capture['status']=='SUCCEEDED':
    for key in ['rgbArtifactId','qualityArtifactId','subsetArtifactId','receiptArtifactId']:
     artifact=call('/artifacts/'+capture['result'][key]);raw=call('/artifacts/'+artifact['id']+'/content',blob=True)
     if hashlib.sha256(raw).hexdigest()!=artifact['sha256']:raise RuntimeError('Restored imagery mismatch')
  original=call('/imagery/jobs/'+receipt['imagery']['comparisonId'])
  reproduced=call('/imagery/jobs',method='POST',body={'requestKey':'restore-reproduction-'+token,'kind':'NDVI_CHANGE','preparationIds':[p['preparationId'] for p in receipt['imagery']['pair']],'parameters':{'declineThreshold':.2,'minValidFraction':.5}})
  for _ in range(5):
   command([ROOT/'bin'/('emad-imagery.exe' if os.name=='nt' else 'emad-imagery'),'-token-file',extracted/'credentials/worker-imagery.token','-once'],env=env,stdout=subprocess.DEVNULL)
   reproduced=call('/imagery/jobs/'+reproduced['id'])
   if reproduced['status']=='SUCCEEDED':break
  if reproduced['status']!='SUCCEEDED' or reproduced['result']['sha256']!=original['result']['sha256']:raise RuntimeError('Restored real imagery analysis did not reproduce its retained result')
  scene=call('/simulation-studio/scenes/'+receipt['simulation']['sceneId'],'EXERCISE_INPUT')
  models=call('/simulation-assets/models','EXERCISE_INPUT')['items']
  for model in models:
   raw=call('/simulation-assets/uploads/'+model['upload']['id']+'/content','EXERCISE_INPUT',blob=True)
   if hashlib.sha256(raw).hexdigest()!=model['upload']['sha256']:raise RuntimeError('Restored model/environment mismatch')
  recordingProof=[]
  for item in (extracted/'data').glob('*accepted-run.json'):
   run=json.loads(item.read_text(encoding='utf-8'))['id'];idx=call('/simulation-recordings/'+run,'EXERCISE_OBSERVATION','local-operator',run)
   if idx['status']!='READY':raise RuntimeError('Restored recording is not ready')
   frame=call('/simulation-recordings/'+run+'/frame?elapsedMs='+str(min(5000,idx['manifest']['elapsedMs'])),'EXERCISE_OBSERVATION','local-operator',run)
   recordingProof.append({'runId':run,'events':idx['manifest']['events'],'frameSequence':frame['sequence']})
  report={'schema':'emad.restore-acceptance.v1','acceptedAt':datetime.now(timezone.utc).isoformat(),'backupManifestSha256':digest(directory/'manifest.json'),'objectsVerified':len(m['objects']),'objectBytesVerified':m['objectBytes'],'referenceVersions':refs['total'],'imageryCaptures':len(captures),'modelsAndEnvironmentVerified':len(models),'recordings':recordingProof,'isolation':'New randomly named PostgreSQL and Garage containers/volumes; loopback ephemeral ports. Original installation was not modified.'}
  if navigation_proof:report['geographicNavigation']=navigation_proof
  report['reproducedAnalysisSha256']=reproduced['result']['sha256']
  save(area/'acceptance.json',report)
  print(f'Isolated restore accepted: {len(m["objects"])} objects, {refs["total"]} reference versions, {len(recordingProof)} recording(s). Evidence: {area/"acceptance.json"}',flush=True)
  return report
 finally:
  if api is not None:
   api.terminate()
   try:api.wait(timeout=10)
   except subprocess.TimeoutExpired:api.kill();api.wait()
  for name in reversed(containers):command(['docker','rm','-f',name],stdout=subprocess.DEVNULL)
  # These names are generated in this invocation, never taken from a manifest.
  for name in volumes:subprocess.run(['docker','volume','rm',name],capture_output=True)

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('action',choices=['backup','verify','restore-test']);parser.add_argument('--path',type=Path)
 args=parser.parse_args()
 if args.action=='backup':backup();return
 directory=args.path or Path(json.loads((PRIVATE/'last-backup.json').read_text(encoding='utf-8'))['path'])
 if args.action=='verify':m=check(directory);print('Paired backup integrity verified: '+str(m['objectCount'])+' objects')
 else:restore(directory)
if __name__=='__main__':
 try:main()
 except Exception as error:
  # Do not print provider SDK or subprocess details containing credentials.
  print('Paired backup/restore failed: '+str(error) if isinstance(error,RuntimeError) else 'Paired backup/restore failed ('+type(error).__name__+'); private evidence was retained.',file=sys.stderr);sys.exit(1)
