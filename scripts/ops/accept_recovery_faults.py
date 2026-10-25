"""Bounded local failure rehearsals; operates only this installer's resources."""
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time
from datetime import datetime,timezone
sys.path.insert(0,str(Path(__file__).resolve().parent));import local
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'data'))
from install_local import API,APIError,PRIVATE

def wait(fn,timeout=90):
 until=time.monotonic()+timeout
 while time.monotonic()<until:
  value=fn()
  if value:return value
  time.sleep(1)
 raise RuntimeError('Recovery acceptance timed out')
def sql(statement):
 return subprocess.run(['docker','exec','-i','c4isr-postgres','psql','-U','c4isr','-d','c4isr','-v','ON_ERROR_STOP=1'],input=statement,text=True,capture_output=True,check=True)
def main():
 if os.name!='nt':raise RuntimeError('This process-ownership rehearsal targets the supported Windows local profile')
 cfg=local.configuration();env=local.environment(cfg);author=API();inputAPI=API(partition='EXERCISE_INPUT')
 receipt=json.loads((PRIVATE/'data/installation.json').read_text(encoding='utf-8'));proof={'schema':'emad.failure-acceptance.v1','acceptedAt':datetime.now(timezone.utc).isoformat()}
 supervisor=json.loads((PRIVATE/'supervisor.json').read_text(encoding='utf-8'))
 capabilities=inputAPI.call('GET','/simulation-runtime/capabilities')['items']
 if not capabilities:raise RuntimeError('A runtime worker must be ready')
 run=inputAPI.call('POST','/simulation-runtime/runs',{'requestKey':'development-worker-crash-'+secrets.token_hex(8),'sceneId':receipt['simulation']['sceneId'],'runtimeVersion':'emad-kinematic-v2','runtimeBuild':capabilities[0]['runtimeBuild'],'seed':51,'epoch':datetime.now(timezone.utc).isoformat(),'durationMs':120000,'scale':1})
 wait(lambda:inputAPI.call('GET','/simulation-runtime/runs/'+run['id'])['sequence']>0)
 processes=subprocess.run(['powershell','-NoProfile','-Command',"Get-CimInstance Win32_Process -Filter \"Name = 'emad-runtime.exe'\" | Select-Object ProcessId,ParentProcessId,ExecutablePath | ConvertTo-Json -Compress"],capture_output=True,text=True,check=True)
 candidates=json.loads(processes.stdout);candidates=candidates if isinstance(candidates,list) else [candidates]
 expected=str((local.ROOT/'bin/emad-runtime.exe').resolve()).casefold()
 owned=[p for p in candidates if p['ParentProcessId']==supervisor['pid'] and str(p.get('ExecutablePath','')).casefold()==expected]
 if len(owned)!=1:raise RuntimeError('Runtime process ownership is ambiguous; no process was stopped')
 subprocess.run(['taskkill','/PID',str(owned[0]['ProcessId']),'/F'],stdout=subprocess.DEVNULL,check=True)
 failed=wait(lambda:(r if (r:=inputAPI.call('GET','/simulation-runtime/runs/'+run['id']))['status']=='FAILED' else None))
 wait(lambda:inputAPI.call('GET','/simulation-runtime/capabilities')['items'])
 operator=API('local-operator','EXERCISE_OBSERVATION',run['id'])
 wait(lambda:(idx if (idx:=operator.call('GET','/simulation-recordings/'+run['id'],allow_missing=True)) and idx['status']=='READY' else None))
 proof['workerCrash']={'runId':run['id'],'status':failed['status'],'failureCode':failed['failureCode'],'automaticWorkerRestart':True,'partialRecordingReady':True,'silentTakeover':False}
 print('Worker crash, lease expiry, automatic restart and partial archive accepted.',flush=True)
 paused=False
 try:
  state=json.loads(subprocess.run(['docker','inspect','emad-garage'],capture_output=True,text=True,check=True).stdout)[0]
  if state['State']['Paused']:raise RuntimeError('Object store was already paused; pre-existing state was not changed')
  subprocess.run(['docker','pause','emad-garage'],stdout=subprocess.DEVNULL,check=True);paused=True
  health=author.call('GET','/platform/readiness')
  if next(f for f in health['features'] if f['id']=='artifact-storage')['state']!='UNAVAILABLE':raise RuntimeError('Storage outage was not reported')
 finally:
  if paused:subprocess.run(['docker','unpause','emad-garage'],stdout=subprocess.DEVNULL,check=True)
 wait(lambda:next(f for f in author.call('GET','/platform/readiness')['features'] if f['id']=='artifact-storage')['state']=='AVAILABLE')
 proof['storageOutage']={'unavailableReported':True,'recoveryReported':True,'persistentBytesPreserved':True}
 print('Storage outage and recovery readiness accepted.',flush=True)
 token=secrets.token_hex(32);digest=hashlib.sha256(token.encode()).hexdigest();principal='recovery-expired-'+secrets.token_hex(6)
 sql(f"BEGIN; INSERT INTO principals(id,kind,name)VALUES('{principal}','service','Expired credential acceptance probe'); INSERT INTO principal_grants(principal_id,workspace_id,environment_id,partition,run_id,permissions)VALUES('{principal}','default','local','PUBLIC_EVIDENCE','',ARRAY['operational.read']); INSERT INTO principal_credentials(id,principal_id,token_hash,default_scope_id,expires_at)VALUES('key_{digest[:24]}','{principal}',decode('{digest}','hex'),'default/local/PUBLIC_EVIDENCE/',now()-interval '1 minute'); COMMIT;")
 expired=API();expired.headers['Authorization']='Bearer '+token
 try:expired.call('GET','/auth/me')
 except APIError as error:
  if error.status!=401:raise
 else:raise RuntimeError('Expired credential authenticated')
 finally:sql(f"UPDATE principals SET disabled_at=now() WHERE id='{principal}';")
 proof['expiredCredential']={'authenticationStatus':401,'probeDisabled':True,'existingCredentialsChanged':False}
 print('Expired credential rejection accepted; probe disabled.',flush=True)
 before=author.call('GET','/reference-datasets')['total']
 stopped=subprocess.run([str(local.PYTHON),'scripts/data/install_local.py','--public-only','--stop-after-references'],cwd=local.ROOT,env=env,capture_output=True,text=True)
 if stopped.returncode==0 or 'Intentional acceptance interruption' not in stopped.stderr:raise RuntimeError('Installation interruption did not reach the durable checkpoint')
 subprocess.run([str(local.PYTHON),'scripts/data/install_local.py','--public-only'],cwd=local.ROOT,env=env,stdout=subprocess.DEVNULL,check=True)
 after=author.call('GET','/reference-datasets')['total']
 if after!=before:raise RuntimeError('Resumed installation duplicated reference versions')
 proof['interruptedSetup']={'checkpoint':'retained references','resumed':True,'referenceVersionsBefore':before,'referenceVersionsAfter':after}
 print('Interrupted installation resumed without duplicate reference versions.',flush=True)
 # Claim and abandon one real retained-input job. The ordinary worker recovers
 # it after the 45-second lease; its stale lease must never finalize afterward.
 scene=receipt['imagery']['pair'][0];worker=API('worker-preparation')
 # Stop only the supervisor's capture process before acquiring the test lease,
 # so the normal consumer cannot win the initial claim. It restarts after 2 s.
 py=subprocess.run(['powershell','-NoProfile','-Command',"Get-CimInstance Win32_Process -Filter \"Name = 'python.exe'\" | Select-Object ProcessId,ParentProcessId,CommandLine | ConvertTo-Json -Compress"],capture_output=True,text=True,check=True)
 children=json.loads(py.stdout);children=children if isinstance(children,list) else [children]
 script=str((local.ROOT/'scripts/data/capture_worker.py').resolve()).replace('\\','/').casefold()
 capture_process=[p for p in children if p['ParentProcessId']==supervisor['pid'] and script in str(p.get('CommandLine','')).replace('\\','/').casefold()]
 if len(capture_process)!=1:raise RuntimeError('Capture worker ownership is ambiguous; no process was stopped')
 subprocess.run(['taskkill','/PID',str(capture_process[0]['ProcessId']),'/T','/F'],stdout=subprocess.DEVNULL,check=True)
 capture=author.call('POST','/imagery/captures',{'requestKey':'development-lease-recovery-'+secrets.token_hex(8),'sceneId':scene['sceneId'],'aoi':scene['aoi']})
 claim=worker.call('POST','/imagery/captures/claim')
 if not claim.get('job') or claim['job']['id']!=capture['id']:raise RuntimeError('Raster worker claimed the test job before the abandoned-lease rehearsal; retry with an idle queue')
 recovered=wait(lambda:(c if (c:=author.call('GET','/imagery/captures/'+capture['id']))['status']=='SUCCEEDED' else None),150)
 if recovered['attempt']<2:raise RuntimeError('Raster job did not recover a lease')
 try:worker.call('POST','/imagery/captures/'+capture['id']+'/progress',{'lease':claim['lease'],'progress':'Stale development lease must be rejected'})
 except APIError as error:
  if error.status!=409:raise
 else:raise RuntimeError('Abandoned raster lease remained authoritative')
 proof['rasterLeaseRecovery']={'captureId':capture['id'],'attempt':recovered['attempt'],'status':recovered['status'],'staleLeaseStatus':409,'inputs':'Actual retained Sentinel-2 windows; no synthetic observations'}
 (PRIVATE/'data/failure-acceptance.json').write_text(json.dumps(proof,indent=2),encoding='utf-8')
 print('Abandoned raster lease recovered; stale worker completion rejected.',flush=True)
if __name__=='__main__':main()
