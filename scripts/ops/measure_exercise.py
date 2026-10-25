"""Measure a bounded one-hour virtual exercise in the persistent installation."""
import json
import os
import platform
from pathlib import Path
import statistics
import sys
import time
from datetime import datetime,timezone
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'data'))
from install_local import API,PRIVATE

def main():
 receipt=json.loads((PRIVATE/'data/installation.json').read_text(encoding='utf-8'))
 author=API(partition='EXERCISE_INPUT');scene=author.call('GET','/simulation-studio/scenes/'+receipt['simulation']['sceneId'])
 body=scene['frozen']['input'];body['name']='Acceptance · one-hour bounded abstract exercise'
 for actor in body['actors']:actor['despawnMs']=3600000
 start=time.monotonic()
 if '--resume' in sys.argv:
  run=next(r for r in author.call('GET','/simulation-runtime/runs')['items'] if r['configuration']['durationMs']==3600000 and r['status'] in ['RUNNING','COMPLETED'])
  frozen=author.call('GET','/simulation-studio/scenes/'+run['configuration']['sceneId']);body=frozen['frozen']['input']
 else:
  frozen=author.call('POST','/simulation-studio/scenes',body)
  capabilities=author.call('GET','/simulation-runtime/capabilities')['items']
  if not capabilities:raise RuntimeError('An admitted runtime worker must be running')
  run=author.call('POST','/simulation-runtime/runs',{'requestKey':'one-hour-'+str(time.time_ns()),'sceneId':frozen['id'],'runtimeVersion':frozen['frozen']['runtime'],'runtimeBuild':capabilities[0]['runtimeBuild'],'seed':51,'epoch':datetime.now(timezone.utc).isoformat(),'durationMs':3600000,'scale':20})
 operator=API('local-operator','EXERCISE_OBSERVATION',run['id']);evaluator=API('local-evaluator','EXERCISE_TRUTH',run['id'])
 samplingStartedAt=datetime.now(timezone.utc).isoformat()
 latencies=[];last=-1;errors=[];deadline=start+3600
 while time.monotonic()<deadline:
  run=author.call('GET','/simulation-runtime/runs/'+run['id'])
  if run['elapsedMs']//300000!=last:
   last=run['elapsedMs']//300000;print(f"One-hour exercise: {run['status']} · {run['elapsedMs']/1000:.0f}/3600 virtual seconds",flush=True)
  if run['status'] in ['COMPLETED','FAILED','STOPPED']:break
  at=time.monotonic()
  try:operator.call('GET','/tracks?limit=100');latencies.append((time.monotonic()-at)*1000)
  except Exception:errors.append('COP read failed')
  time.sleep(2)
 if run['status']!='COMPLETED':raise RuntimeError('One-hour exercise did not complete: '+run['status'])
 wall=(datetime.now(timezone.utc)-datetime.fromisoformat(run['configuration']['epoch'].replace('Z','+00:00'))).total_seconds()
 for _ in range(180):
  index=operator.call('GET','/simulation-recordings/'+run['id'],allow_missing=True)
  if index and index['status']=='READY':break
  time.sleep(1)
 else:raise RuntimeError('One-hour archive did not become ready')
 sealed=evaluator.call('GET','/simulation-recordings/'+run['id'])
 metrics=evaluator.call('GET','/simulation-recordings/'+run['id']+'/evaluation')
 evidence={'schema':'emad.exercise-measurement.v1','acceptedAt':datetime.now(timezone.utc).isoformat(),'id':run['id'],'elapsedMs':run['elapsedMs'],'actors':len(body['actors']),'models':len(frozen['frozen']['models']),'sensors':len(body.get('sensors',[])),'runtimeBuild':run['configuration']['runtimeBuild'],'wallSeconds':wall,'virtualDurationSeconds':3600,'scale':20,'timeMeaning':'One hour of virtual time at 20× acceleration, not a one-hour wall-clock soak.','copLatencyMs':{'samples':len(latencies),'median':statistics.median(latencies),'p95':sorted(latencies)[min(len(latencies)-1,int(len(latencies)*.95))],'max':max(latencies)},'operatorEvents':index['manifest']['events'],'truthEvents':sealed['manifest']['events'],'archiveBytes':sum(b['bytes'] for idx in [index,sealed] for b in [*idx['manifest']['chunks'],idx['manifest']['checkpoint']]),'projectionSettled':index['manifest']['projectionSettled'],'failures':errors,'platform':platform.platform(),'logicalCPUCount':os.cpu_count(),'limitations':['Abstract kinematics; no validated flight or physical sensor model.','See separate populated-scene browser measurements for graphics performance.'],'metrics':metrics['metrics']}
 evidence['latencySamplingStartedAt']=samplingStartedAt
 evidence['measurementConditions']='Supported local services active; concurrent browser regressions and an image build occurred during the run. Requested scale excludes API acceptance time; wall duration is measured separately.'
 (PRIVATE/'data/one-hour-accepted-run.json').write_text(json.dumps(evidence,indent=2),encoding='utf-8')
 print('One-hour virtual exercise accepted: '+str(round(wall,2))+' wall seconds; '+str(evidence['operatorEvents'])+' operator events; '+str(evidence['archiveBytes'])+' archive bytes.',flush=True)
if __name__=='__main__':main()
