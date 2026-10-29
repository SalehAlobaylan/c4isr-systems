"""Source-backed regional navigation preparation; no provider execution grants."""
import argparse
import base64
import json
import re
from pathlib import Path
from shapely.geometry import mapping, shape
from shapely.ops import transform
from recovery_data import DATA, encoded, sha, store_json
from prepare_geography import rights, NE_TERMS
from local_connectors import source_id

DEFAULT_PROFILE = Path(__file__).with_name('region_profiles')/'middle-east-v1.json'
LAYERS = [('50m_cultural/ne_50m_admin_0_countries','navigation-world-countries-50m'),
          ('10m_cultural/ne_10m_admin_1_states_provinces','navigation-admin1'),
          ('50m_cultural/ne_50m_populated_places','navigation-populated-places')]

def navigation_rights():
    return rights('naturalearth-navigation-public-domain-v2',NE_TERMS,'Made with Natural Earth; retained original source navigation geometry')

def load_profile(value=None):
    p=json.loads(Path(value or DEFAULT_PROFILE).read_text(encoding='utf-8')) if not isinstance(value,dict) else value
    if p.get('schema')!='emad.region-install-profile.v1' or not re.fullmatch(r'[a-zA-Z0-9_.:-]{1,100}',p.get('key','')) or not p.get('name') or len(p.get('definition',''))<10:
        raise ValueError('A versioned, named geographic working definition is required')
    codes=p.get('countries',[])
    if not 1<=len(codes)<=100 or len(set(codes))!=len(codes) or any(not re.fullmatch('[A-Z]{3}',c) for c in codes):
        raise ValueError('Select 1-100 unique source country identifiers')
    studies=p.get('studyAreas',[])
    if not isinstance(studies,list) or len(studies)+len(codes)>128:
        raise ValueError('Region profiles support at most 128 exact country/study members')
    keys=set()
    for study in studies:
        if not isinstance(study,dict) or not re.fullmatch(r'[a-zA-Z0-9_.:-]{1,160}',study.get('key','')) or study['key'] in keys or study.get('parentCountry') not in codes or not 1<=len(study.get('name',''))<=200 or len(study.get('meaning',''))<10 or study.get('geometry',{}).get('type') not in ('Polygon','MultiPolygon'):
            raise ValueError('Study areas need a unique stable key, selected parent, name, meaning and explicit WGS84 polygon')
        keys.add(study['key'])
    return p

def aliases(attrs):
    # Preserve supplied aliases/display scripts. Search normalization is separate.
    return ' | '.join(dict.fromkeys(str(v) for k,v in attrs.items() if k.lower() in {'name','name_en','name_ar','name_alt','formal_en','brk_name','gn_name'} and v))

def bounded_chunks(features, max_bytes=4*1024*1024, max_features=1000):
    chunk=[];size=len(encoded({'type':'FeatureCollection','features':[]}))
    for f in sorted(features,key=lambda f:f['properties']['key']):
        n=len(encoded(f))+1
        if n>max_bytes:raise ValueError('One source feature exceeds admission budget; prepare a reviewed bounded edition')
        if chunk and (size+n>max_bytes or len(chunk)>=max_features):
            yield chunk;chunk=[];size=len(encoded({'type':'FeatureCollection','features':[]}))
        chunk.append(f);size+=n
    if chunk:yield chunk

def prepare(profile, loader):
    profile=load_profile(profile)
    countries,country_receipt=loader(*LAYERS[0]);divisions,division_receipt=loader(*LAYERS[1]);places,place_receipt=loader(*LAYERS[2])
    wanted=set(profile['countries']);country_shapes={};world=[];regional=[];points=[];parents={};counts={c:{'regions':0,'places':0} for c in wanted}
    def feature(attrs,geometry,key,parent,kind,name,scale,meaning):
        if not geometry.is_valid or geometry.is_empty:raise ValueError('Invalid source geometry: '+key)
        return {'type':'Feature','properties':{'key':key,'parentKey':parent,'kind':kind,'name':name,'aliases':aliases(attrs),'scale':scale,'meaning':meaning,'geometryTransformation':'Original source geometry; clamp floating-point WGS84 limit noise only'},'geometry':mapping(geometry)}
    for attrs,g in countries:
        g=transform(lambda x,y,z=None:(max(-180.,min(180.,x)),max(-90.,min(90.,y))),g)
        code=str(attrs.get('ADM0_A3') or attrs.get('SOV_A3'))
        world.append(feature(attrs,g,'country:'+code,'','country',attrs.get('NAME_EN') or attrs['NAME'],'1:50m','Generalized Natural Earth boundary interpretation; not an official jurisdiction determination'))
        if code in wanted:country_shapes[code]=g
    missing=wanted-set(country_shapes)
    if missing:raise ValueError('Requested country identities are unavailable in this source: '+','.join(sorted(missing)))
    for attrs,g in divisions:
        country=attrs.get('adm0_a3')
        if country not in wanted:continue
        if attrs.get('ne_id') in (None,'',-99):raise ValueError('Subdivision requires a stable source NE_ID')
        key='region:ne:'+str(attrs['ne_id'])
        regional.append(feature(attrs,g,key,'country:'+country,'region',attrs.get('name_en') or attrs['name'],'1:10m','Source first-order administrative navigation; scale is not survey accuracy'))
        parents.setdefault(country,[]).append((key,g));counts[country]['regions']+=1
    seen=set()
    for attrs,g in places:
        country=attrs.get('ADM0_A3')
        if country not in wanted:continue
        upstream=attrs.get('NE_ID')
        if upstream in (None,'',-99):raise ValueError('Place requires a stable source NE_ID; row offsets are forbidden')
        key='place:ne:'+str(upstream)
        if key in seen:raise ValueError('Duplicate source place identity: '+key)
        seen.add(key)
        parent=next((k for k,area in sorted(parents.get(country,[])) if area.covers(g)),'country:'+country)
        points.append(feature(attrs,g,key,parent,'place',attrs.get('NAME_EN') or attrs.get('NAME') or str(upstream),'1:50m','Source place point; no municipal boundary or local detail implied'))
        counts[country]['places']+=1
    for study in profile.get('studyAreas',[]):
        country=study['parentCountry']
        if country not in wanted:raise ValueError('Study area parent must be a selected country')
        g=shape(study['geometry']);parent=next((k for k,area in sorted(parents.get(country,[])) if area.covers(g)),'country:'+country)
        regional.append(feature({},g,study['key'],parent,'study',study['name'],'explicit study',study['meaning']))
    chunks=[]
    for label,features,receipt in [('World countries',world,country_receipt),('Regional subdivisions',regional,division_receipt),('Regional place points',points,place_receipt)]:
        for i,part in enumerate(bounded_chunks(features)):
            raw=encoded({'type':'FeatureCollection','features':part});digest=sha(raw)
            chunks.append({'label':label+' · '+profile['key']+' · '+str(i+1),'version':digest[:24],'sha256':digest,'artifact':base64.b64encode(raw).decode(),'receipt':receipt,'featureCount':len(part)})
    version=sha(encoded({'schema':'regional-navigation-v2','profile':profile,'chunks':[{k:v for k,v in c.items() if k!='artifact'} for c in chunks],'rights':navigation_rights(),'staged':True}))[:24]
    return {'schema':'emad.regional-geography-prepared.v1','profile':profile,'version':version,'chunks':chunks,'depth':counts,'meaning':'Reference navigation only; no retrieval profile or execution permission is created.'}

def install(api, profile=None, loader=None, output=None):
    if loader is None:
        from install_geography import records
        loader=records
    plan=prepare(profile,loader);source=source_id('naturalearth')
    # Read the expected head before staging; a concurrent activation cannot be
    # silently overwritten. Chunk admissions are content-addressed/idempotent.
    heads=api.call('GET','/geography/releases')
    expected=next((h['releaseId'] for h in heads['items'] if h['sourceId']==source),'')
    admitted=[]
    for chunk in plan['chunks']:
        value={k:v for k,v in chunk.items() if k!='featureCount'}
        value.update({'sourceId':source,'staged':True,'rights':navigation_rights()})
        actual=api.call('POST','/geography/editions',value)
        admitted.append({'id':actual['id'],'sha256':chunk['sha256'],'features':chunk['featureCount'],'label':chunk['label']})
        # Progress is resumable, never a claim that a partial release is active.
        store_json(Path(output or DATA/'geographic-navigation.json'),{'schema':'emad.geography-navigation.v2','state':'STAGING','version':plan['version'],'editions':admitted})
    release=api.call('POST','/geography/releases',{'sourceId':source,'key':plan['profile']['key'],'version':plan['version'],'editionIds':[v['id'] for v in admitted],'expectedHead':expected,'activate':True})
    members=[{'kind':'administrative','sourceId':source,'logicalKey':'country:'+code,'releaseId':release['id']} for code in sorted(plan['profile']['countries'])]
    members.extend({'kind':'named','sourceId':source,'logicalKey':study['key'],'releaseId':release['id']} for study in plan['profile'].get('studyAreas',[]))
    manifest=api.call('POST','/geography/profiles',{'key':plan['profile']['key'],'version':plan['version'],'name':plan['profile']['name'],'definition':plan['profile']['definition'],'members':members})
    result={'schema':'emad.geography-navigation.v2','state':'ACTIVE','releaseId':release['id'],'profileId':manifest['id'],'version':plan['version'],'editions':admitted,'depth':plan['depth']}
    store_json(Path(output or DATA/'geographic-navigation.json'),result)
    return admitted

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--profile',type=Path,default=DEFAULT_PROFILE)
    parser.add_argument('--prepare-only',type=Path,help='Write a reviewable source-backed manifest; do not access the application')
    args=parser.parse_args()
    if args.prepare_only:
        from install_geography import records
        plan=prepare(args.profile,records);store_json(args.prepare_only,plan)
        print(json.dumps({'state':'PREPARED_ONLY','path':str(args.prepare_only),'version':plan['version'],'depth':plan['depth']},ensure_ascii=True))
    else:
        from install_local import API
        install(API(profile='local-source-admin'),args.profile)

if __name__=='__main__':main()
