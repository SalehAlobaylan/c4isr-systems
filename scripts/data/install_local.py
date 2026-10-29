"""Idempotently install retained real content into the supported local profile."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import time
from datetime import datetime, timedelta, timezone
import requests
from recovery_data import ROOT, DATA, SAUDI, JEDDAH, encoded, sha, now, store_json, archive_sources
from local_connectors import configure, PROVIDERS, source_id
from prepare_geography import prepare, rights

PRIVATE = Path(os.environ.get('EMAD_PRIVATE_DIR',str(ROOT/'private/local')))
CREDS = Path(os.environ.get('EMAD_CREDENTIAL_DIR',str(PRIVATE/'credentials')))
RECEIPTS = DATA / 'installation.json'
class APIError(RuntimeError):
    def __init__(self,status,message):super().__init__(message);self.status=status


class API:
    def __init__(self, profile=None, partition='PUBLIC_EVIDENCE', run_id=None):
        profile = profile or os.environ.get('EMAD_INSTALLER_PROFILE','local-analyst')
        self.origin=os.environ.get('EMAD_API_URL')
        if not self.origin:
            cfg = json.loads((PRIVATE/'config.json').read_text(encoding='utf-8'))
            self.origin = 'http://127.0.0.1:' + str(cfg['apiPort'])
        self.headers = {'Authorization': 'Bearer ' + (CREDS/(profile+'.token')).read_text().strip(),
                        'X-EMAD-Workspace-ID':'default','X-EMAD-Environment-ID':'local','X-EMAD-Partition':partition}
        if run_id: self.headers['X-EMAD-Run-ID'] = run_id

    def call(self, method, path, body=None, raw=None, allow_missing=False):
        headers = self.headers.copy()
        if raw is None: headers['Content-Type'] = 'application/json'
        response = requests.request(method,self.origin+'/api/v1'+path,headers=headers,data=raw if raw is not None else encoded(body) if body is not None else None,timeout=150,allow_redirects=False)
        if 300<=response.status_code<400:raise RuntimeError('API redirect rejected')
        if allow_missing and response.status_code == 404:return None
        if response.status_code >= 400:
            try: message=response.json().get('error',{}).get('message','')
            except ValueError: message=''
            raise APIError(response.status_code,f'{method} {path}: HTTP {response.status_code} {message}')
        return response.json() if response.content and response.headers.get('Content-Type','').startswith('application/json') else response.content


def source(api, identifier, name, metadata, active=True):
    canonical=source_id(identifier)
    existing = api.call('GET','/sources/'+canonical,allow_missing=True)
    if existing is None:
        return api.call('POST','/sources',{'id':identifier,'name':name,'type':'external','status':'active' if active else 'offline','metadata':metadata})
    if existing['name']!=name or existing['type']!='external': raise RuntimeError('Installer source ID conflicts with an existing resource: '+identifier)
    if active and existing['status']!='active':api.call('POST','/sources/'+canonical+'/status',{'status':'active'})
    return existing


def admission(key):
    product = {'usgs':('usgs-earthquakes','v1','U.S. Geological Survey','USGS'),
               'usgs-history':('usgs-earthquakes','v1','U.S. Geological Survey','USGS'),
               'metar':('noaa-awc-metar','v1','Originating official station networks; station IDs retained','NOAA Aviation Weather Center'),
               'gdacs':('gdacs-earthquake-bulletins','v1','GDACS automatic model bulletin with cited scientific source','GDACS / European Commission JRC'),
               'cdse':('sentinel-2-l2a','stac-1.1-l2a','ESA Copernicus Sentinel-2','Copernicus Data Space Ecosystem'),
               'earthsearch':('sentinel-2-c1-l2a','sentinel2-c1-l2a','ESA Copernicus Sentinel-2','Element 84 Earth Search / Sinergise'),
               'firms':('VIIRS_NOAA20_NRT','2.0NRT','NASA LANCE / NOAA-20 VIIRS','NASA FIRMS'),
               'opensky':('opensky-state-vectors','v1','Contributing ADS-B receivers','OpenSky Network'),
               'ais':('aisstream-position-reports','v1','Contributing AIS receivers','AISStream')}[key]
    dataset, version, producer, provider=product
    terms={'usgs':'https://www.usgs.gov/information-policies-and-instructions/copyrights-and-credits',
           'usgs-history':'https://www.usgs.gov/information-policies-and-instructions/copyrights-and-credits',
           'metar':'https://www.weather.gov/disclaimer','gdacs':'https://www.gdacs.org/About/termofuse.aspx',
           'earthsearch':'https://sentinels.copernicus.eu/documents/247904/690755/Sentinel_Data_Legal_Notice',
           'cdse':'https://sentinels.copernicus.eu/documents/247904/690755/Sentinel_Data_Legal_Notice',
           'firms':'https://www.earthdata.nasa.gov/learn/find-data/near-real-time/firms',
           'opensky':'https://opensky-network.org/about/terms-of-use','ais':'https://aisstream.io/terms'}[key]
    imagery=key in ['earthsearch','cdse']
    licence=rights('local-'+key+'-rights-v1',terms,producer+' via '+provider+'; unendorsed attributed development context')
    licence['allowSimulationExport']=imagery
    if imagery: licence['attribution']='Contains modified Copernicus Sentinel data (2025); ESA, Element 84 / Sinergise or CDSE as recorded per scene'
    policy={'aoi':JEDDAH if imagery or key in ['firms','opensky','ais'] else SAUDI,
            'lookbackSeconds':31*86400 if imagery or key=='usgs-history' else 7*86400,
            'freshForSeconds':86400,'unavailableAfterSeconds':7*86400,'minIntervalSeconds':3600,
            'timeoutSeconds':10,'maxPages':3,'maxRecordsPerPage':25 if imagery else 200,
            'maxResponseBytes':4*1024*1024,'maxRequestsPerDay':PROVIDERS[key][2]}
    if key=='metar':policy.update(lookbackSeconds=21600,freshForSeconds=3600,unavailableAfterSeconds=7200,maxPages=1,maxRecordsPerPage=400,maxResponseBytes=1048576,minIntervalSeconds=900)
    if key=='gdacs':policy.update(lookbackSeconds=604800,maxPages=1,maxRecordsPerPage=100,maxResponseBytes=1048576,minIntervalSeconds=1800)
    if key in ['opensky','ais']:policy.update(lookbackSeconds=300,freshForSeconds=60,unavailableAfterSeconds=300,maxPages=1,maxRecordsPerPage=200,maxResponseBytes=1048576,minIntervalSeconds=900)
    if key=='firms':policy['lookbackSeconds']=3*86400
    return {'analystQuestion':'Which official natural-hazard reports and station observations contextualize this selected Saudi region?',
            'originalProducer':producer,'upstreamProvider':provider,'connectorId':'local-'+key,'connectorPrincipalId':'worker-'+key,
            'dataset':dataset,'productVersion':version,'coverageDescription':'Bounded Saudi-region snapshot; source footprints may include neighbouring areas. '+('Historical retained March 2025 capture, not current conditions.' if key=='usgs-history' else 'No completeness or absence claim.'),
            'revisionPolicy':'content_hash','enabled':True,'rights':licence,'policy':policy}


def ensure_admission(api,key):
    value=admission(key)
    old=api.call('GET','/sources/'+source_id(key)+'/admission',allow_missing=True)
    if old is not None and all(old.get(k)==v for k,v in value.items()):return old
    return api.call('POST','/sources/'+source_id(key)+'/admission',value)


def register_public(api, receipts,stop_after_references=False):
    _, states = configure(os.environ.copy())
    for key in ['naturalearth','osm','dem']:
        source(api,'local-'+key,{'naturalearth':'Saudi geographic reference · Natural Earth','osm':'Jeddah roads and buildings · OpenStreetMap','dem':'Jeddah surface elevation · Copernicus DEM'}[key],
               {'category':'GEOGRAPHIC_REFERENCE','activation':{'state':'RETAINED'},'synthetic':False,'coverage':SAUDI if key=='naturalearth' else JEDDAH})
    for key,state in states.items():
        source(api,'local-'+key,{'usgs':'USGS earthquakes · Saudi region','usgs-history':'USGS retained March 2025 · historical context','metar':'Official station weather · Saudi region','gdacs':'Official natural-hazard bulletins · GDACS','cdse':'Copernicus Data Space · Sentinel-2 catalogue','earthsearch':'Sentinel-2 Collection 1 · Earth Search','firms':'NASA FIRMS · NOAA-20 thermal detections','opensky':'External aircraft observations · OpenSky','ais':'External vessel observations · AISStream'}[key],
               {'category':'IMAGERY' if key in ['earthsearch','cdse'] else 'PUBLIC_OBSERVATION','activation':state,'synthetic':False},state['state']=='CONFIGURED')
        if state['state']=='CONFIGURED':ensure_admission(api,key)
    receipts['providerActivation']=states
    references=[]
    for value in prepare():
        value=dict(value,sourceId=source_id(value['sourceId']))
        installed=api.call('POST','/reference-datasets',value)
        references.append({'id':installed['id'],'datasetKey':installed['datasetKey'],'sha256':installed['sha256'],'version':installed['version']})
    receipts['references']=references;store_json(RECEIPTS,receipts)
    print('Installed '+str(len(references))+' retained reference versions',flush=True)
    if stop_after_references:raise RuntimeError('Intentional acceptance interruption after durable reference checkpoint')
    captures=receipts.setdefault('captures',{})
    for key in ['usgs','usgs-history','metar','gdacs','earthsearch','cdse']:
        if key in captures and captures[key].get('status') in ['SUCCEEDED','PARTIAL']:continue
        connector=API('worker-'+key)
        body={'from':'2025-03-01T00:00:00Z','to':'2025-03-31T23:59:59Z'} if key in ['usgs-history','earthsearch','cdse'] else None
        try:result=connector.call('POST','/sources/'+source_id(key)+'/connector-runs',body)
        except APIError as error:
            if error.status!=409:raise
            captures[key]={'status':'WAITING_FOR_SOURCE_RETRY','meaning':'Existing source pacing/outage backoff remains enforced. Inspect source health for the next attempt.'}
            store_json(RECEIPTS,receipts);print(key+': waiting for the existing source retry window',flush=True);continue
        captures[key]={'id':result['id'],'status':result['status'],'newEvidence':result['newEvidence'],'duplicates':result['duplicates'],'failureCode':result.get('failureCode','')}
        store_json(RECEIPTS,receipts)
        print(key+': '+result['status']+'; '+str(result['newEvidence'])+' retained records'+(' ('+result.get('failureCode','')+')' if result.get('failureCode') else ''),flush=True)


def artifact(api,path,source_id,kind,media='application/json',admission_id=None):
    raw=path.read_bytes();digest=sha(raw)
    intent='installed-'+digest+('-'+sha(admission_id.encode())[:16] if admission_id else '')
    identifier='art_'+sha(('default/local/PUBLIC_EVIDENCE/|'+source_id+'|'+intent).encode())
    existing=api.call('GET','/artifacts/'+identifier,allow_missing=True)
    if existing is not None:
        if existing['sha256']!=digest:raise RuntimeError('Retained artifact identity conflict')
        return existing if existing['state']=='READY' else api.call('PUT','/artifacts/'+identifier+'/content',raw=raw)
    value=api.call('POST','/artifacts',{'requestKey':intent,'sourceId':source_id,**({'requestedAdmissionId':admission_id} if admission_id else {}),'sha256':digest,'bytes':len(raw),'mediaType':media,'kind':kind,
                   'retainUntil':(datetime.now(timezone.utc)+timedelta(days=365)).isoformat().replace('+00:00','Z'),'provenance':'Retained real public-source capture; see immutable installation capture receipts. '+path.name})
    if value['state']!='READY':value=api.call('PUT','/artifacts/'+value['id']+'/content',raw=raw)
    return value


def work_once(name, app):
    exe=ROOT/'bin'/(app+('.exe' if os.name=='nt' else ''))
    subprocess.run([str(exe),'-token-file',str(CREDS/(name+'.token')),'-once'],cwd=ROOT,env=os.environ.copy(),check=True)


def imagery(api,receipts):
    from capture_imagery import capture
    candidates=api.call('GET','/imagery/scenes')['items']
    candidates=sorted([s for s in candidates if s['sourceId']==source_id('earthsearch') and s['acquiredAt'].startswith('2025-03')],key=lambda s:s['acquiredAt'])
    if len(candidates)<2:raise RuntimeError('Real imagery acceptance needs two retained C1 catalogue scenes; inspect Earth Search capture health')
    # Same tile/orbit avoids a cross-track interpretation. March spans a bounded
    # retained demonstration interval; zero change remains an honest result.
    before=candidates[0]
    after=next((s for s in reversed(candidates) if s['tile']==before['tile'] and s['orbit']==before['orbit'] and s['acquiredAt']!=before['acquiredAt']),None)
    if after is None:raise RuntimeError('No compatible real before/after pair in the retained interval')
    prepared=[]
    for scene in [before,after]:
        queued=api.call('POST','/imagery/captures',{'requestKey':'installed-real-'+scene['contentHash'][:32],'sceneId':scene['id'],'aoi':JEDDAH})
        if queued['status']=='FAILED' and queued['attempt']<3:queued=api.call('POST','/imagery/captures/'+queued['id']+'/retry')
        if queued['status']!='SUCCEEDED':
            subprocess.run([sys.executable,str(ROOT/'scripts/data/capture_worker.py'),'--once'],cwd=ROOT,env=os.environ.copy(),check=True)
        queued=api.call('GET','/imagery/captures/'+queued['id'])
        if queued['status']!='SUCCEEDED':raise RuntimeError('Real capture failed: '+queued.get('errorCode',''))
        result=queued['result'];directory,captured=capture(scene,JEDDAH)
        prepared.append({'sceneId':scene['id'],'captureId':queued['id'],'preparationId':result['preparationId'],'rgbArtifactId':result['rgbArtifactId'],'qualityArtifactId':result['qualityArtifactId'],'aoi':JEDDAH,'directory':str(directory.relative_to(ROOT)),'captured':captured})
    comparison=api.call('POST','/imagery/jobs',{'requestKey':'local-real-comparison-'+sha(encoded([v['preparationId'] for v in prepared]))[:32],
                                            'kind':'NDVI_CHANGE','preparationIds':[v['preparationId'] for v in prepared],
                                            'parameters':{'declineThreshold':.2,'minValidFraction':.5}})
    if comparison['status']!='SUCCEEDED':work_once('worker-imagery','emad-imagery')
    comparison=api.call('GET','/imagery/jobs/'+comparison['id'])
    if comparison['status']!='SUCCEEDED':raise RuntimeError('Real comparison failed: '+comparison.get('errorCode',''))
    receipts['imagery']={'pair':prepared,'comparisonId':comparison['id'],'result':comparison['result']}
    if not receipts.get('realProposal'):
        proposal=api.call('POST','/assessments/imagery-proposals',{'jobId':comparison['id'],'conclusion':'Real retained Sentinel-2 environmental indicator prepared for human interpretation. Change pixels and masks require analyst review; this is not a verified claim of vegetation loss.','method':'ALGORITHM'})
        receipts['realProposal']={'assessmentId':proposal['assessmentId'],'revisionId':proposal['id'],'status':proposal['status'],'humanReviewPending':True}
    store_json(RECEIPTS,receipts)
    return ROOT/prepared[-1]['directory']/'rgb.png'


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--public-only',action='store_true');parser.add_argument('--stop-after-references',action='store_true',help='Acceptance-only interruption at a durable installation checkpoint');args=parser.parse_args()
    receipts=json.loads(RECEIPTS.read_text()) if RECEIPTS.exists() else {'schema':'emad.installation.v1','createdAt':now()}
    api=API();register_public(api,receipts,args.stop_after_references)
    from install_geography import install as install_geography
    receipts['geographicNavigation']=install_geography(api)
    if not args.public_only:
        texture=imagery(api,receipts)
        from install_simulation import install
        install(api,receipts,texture)
    archive_sources()
    store_json(RECEIPTS,receipts)
    print('Retained content installation finished; actual human review remains explicit.',flush=True)


if __name__=='__main__':main()
