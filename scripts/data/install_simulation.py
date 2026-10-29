"""Install validated starter models and a frozen real-context exercise scene."""
import json
import os
from datetime import datetime,timezone
from recovery_data import ROOT,DATA,JEDDAH,encoded,sha,store_json
from prepare_environment import build, build_fixture_models
from install_local import API,work_once,RECEIPTS


def upload(api,path,name,licence,kind='MODEL',source_file=''):
    raw=path.read_bytes();digest=sha(raw)
    value={'requestKey':'installed-'+digest,'sourceVersion':digest,'name':name,'kind':kind,'sha256':digest,'bytes':len(raw),
           'mediaType':'model/gltf-binary' if kind=='MODEL' else 'application/octet-stream',
           'sourceReference':'https://github.com/SalehAlobaylan/c4isr-systems','provenance':'Project generator and retained parameter/source receipts. '+path.name,'license':licence}
    if source_file:value['sourceFileId']=source_file
    admitted=api.call('POST','/simulation-assets/uploads',value)
    if admitted['state']=='PENDING':admitted=api.call('PUT','/simulation-assets/uploads/'+admitted['id']+'/content',raw=raw)
    if kind=='SOURCE':return admitted
    for _ in range(20):
        versions=api.call('GET','/simulation-assets/models')['items']
        found=next((m for m in versions if m['upload']['id']==admitted['id']),None)
        if found:return found
        current=api.call('GET','/simulation-assets/uploads/'+admitted['id'])
        if current['state'] in ['REJECTED','FAILED']:raise RuntimeError(name+' validation failed: '+current.get('errorCode',''))
        work_once('worker-assets','emad-assets')
    raise RuntimeError('Model validation did not complete within the installation budget')

def install(public,receipts,texture):
    studio=API(partition='EXERCISE_INPUT')
    prepared=build(texture.read_bytes())
    build_fixture_models()
    licence={'id':'emad-development-assets-v1','licenseId':'LicenseRef-EMAD-DevelopmentAssets',
             'termsUrl':'https://github.com/SalehAlobaylan/c4isr-systems','attribution':'EMAD project-authored static development models',
             'legalBasis':'Internal development and synthetic simulation; perpetual source retention; no external redistribution licence asserted. See DEVELOPMENT_ASSETS.md.',
             'allowStorage':True,'allowSimulation':True,'allowRedistribution':False,'archivePermission':'perpetual'}
    generator=upload(studio,ROOT/'scripts/data/glb_geometry.py','EMAD starter geometry generator',licence,'SOURCE')
    names={'fixed-wing':('Fixed-wing aircraft',160),'uav':('Quadrotor UAV',40),'helicopter':('Helicopter',90),
           'vehicle':('Ground vehicle',30),'vessel':('Surface vessel',18),'sensor':('Stationary sensor',0)}
    models,templates={},{}
    for key,(name,speed) in names.items():
        m=upload(studio,DATA/(key+'.glb'),'EMAD '+name,licence,source_file=generator['id']);models[key]=m['id']
        templates[key]=studio.call('POST','/simulation-assets/templates',{'name':name+' · abstract behavior','modelVersionId':m['id'],
                       'behavior':'STATIC' if speed==0 else 'WAYPOINT_KINEMATIC','maxSpeedMps':speed,'fidelity':'abstract_behavior',
                       'limitations':'Visual representation and abstract ENU waypoint movement; no validated physical dynamics or collision model.'})['id']
    geography_licence=dict(licence,id='jeddah-derived-geography-v1',licenseId='LicenseRef-EMAD-AttributedGeographicWork',
                         attribution='Copernicus DEM / ESA / Airbus; NGA / PROJ-data; Contains modified Copernicus Sentinel data (2025); © OpenStreetMap contributors / Geofabrik',
                         legalBasis='Internal attributed produced work; source terms and hash-pinned raw sources retained. OSM ODbL obligations preserved; no unreviewed redistribution asserted.')
    env_source=upload(studio,DATA/'prepared-environment.json','Jeddah prepared environment manifest',geography_licence,'SOURCE')
    environment_model=upload(studio,DATA/'jeddah-environment.glb','Jeddah coastal environment · real geography',geography_licence,source_file=env_source['id'])
    frame=json.loads((DATA/'frame.json').read_text())
    env=studio.call('POST','/simulation-assets/environments',{'name':'Jeddah coast · retained real terrain, imagery and OSM','modelVersionId':environment_model['id'],
                    'frame':frame,'modelAxesToEnu':'x_east_y_up_z_south','fidelity':'visual_only','prepared':prepared,'releaseToOperators':True,
                    'provenance':'Retained Copernicus DSM transformed from EGM2008 with hash-pinned PROJ grid; attributed 2025 Sentinel imagery; retained OSM snapshot. Estimated building heights and display-only roads; explicit operator geography release.'})
    context_id=receipts.get('simulation',{}).get('contextPackageId')
    if not context_id or public.call('GET','/context-packages/'+context_id,allow_missing=True) is None:
        dataset_ids=[v['id'] for v in receipts['references'] if v['datasetKey'].startswith('jeddah-')]
        body={'datasetIds':dataset_ids,'aoi':JEDDAH,'timeWindow':{'from':'2025-03-01T00:00:00Z','to':'2025-04-01T00:00:00Z'},
              'preparationIds':[v['preparationId'] for v in receipts['imagery']['pair']],
              'acceptedQuality':'Mixed-date retained development context: 2025 imagery and later OSM/elevation snapshots. Surface heights, building estimates and omissions remain explicit.'}
        preview=public.call('POST','/context-packages/preview',body)
        package=public.call('POST','/context-packages',{'previewId':preview['id'],'previewSha256':preview['sha256'],
                           'approval':'Local development snapshot accepted for synthetic scene construction under retained attributed source terms; not a statement of current ground conditions or reviewed intelligence.'})
        context_id=package['id']
    presets=[]
    for name,period,drop,delay,noise,confidence in [('Clear synthetic observer',2000,0,0,8,.85),('Intermittent delayed observer',4000,4,1500,30,.6)]:
        presets.append(studio.call('POST','/simulation-studio/sensor-presets',{'name':name,'periodMs':period,'dropEvery':drop,'delayMs':delay,'noiseM':noise,'confidence':confidence,
                        'limitations':'Deterministic abstract scheduled samples, uniform horizontal error and configured omissions. No physical radar or camera model.'}))
    h=frame['origin']['height']
    def position(lng,lat,height):return {'longitude':lng,'latitude':lat,'height':height,'verticalDatum':'WGS84_ELLIPSOID'}
    def actor(key,identifier,lng,lat,height,speed,points):return {'id':identifier,'templateId':templates[key],'position':position(lng,lat,height),'heading':0,'speedMps':speed,
                                                              'waypoints':[position(x,y,height) for x,y in points],'spawnMs':0,'despawnMs':3600000}
    actors=[actor('uav','survey-uav',39.18,21.5,h+160,22,[(39.19,21.51),(39.2,21.495),(39.18,21.5)]),
            actor('fixed-wing','survey-aircraft',39.17,21.49,h+500,80,[(39.205,21.52),(39.16,21.52),(39.17,21.49)]),
            actor('vehicle','support-vehicle',39.19,21.49,h+1.5,8,[(39.195,21.5),(39.19,21.49)]),
            actor('vessel','coastal-vessel',39.15,21.49,h,7,[(39.15,21.515),(39.155,21.49)])]
    sensors=[{k:v for k,v in preset.items() if k in ['periodMs','dropEvery','delayMs','noiseM','confidence']}|{'id':'observer-'+str(i+1),'presetId':preset['id']} for i,preset in enumerate(presets)]
    scene=studio.call('POST','/simulation-studio/scenes',{'name':'Jeddah coastal context exercise','contextPackageId':context_id,'environmentId':env['id'],'actors':actors,'sensors':sensors,
                      'controlled':[{'actorId':'survey-uav','callsign':'Survey UAV (simulated)','telemetryMs':2000}]})
    receipts['simulation']={'sceneId':scene['id'],'environmentId':env['id'],'contextPackageId':context_id,'models':models,'templates':templates,
                            'preparedEnvironmentSha256':sha(encoded(prepared)),'installed':True,'runAcceptance':'PENDING'}
    store_json(RECEIPTS,receipts)
    print('Installed six recognizable models and real georeferenced Jeddah scene: '+scene['id'],flush=True)
