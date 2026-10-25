"""Retain and admit world navigation and Saudi subdivisions; no feed expansion."""
import io
import json
import zipfile
import base64
import shapefile
from shapely.geometry import shape, mapping, box
from shapely.ops import transform
from recovery_data import DATA, JEDDAH, download, encoded, sha, store_json
from prepare_geography import rights, NE_TERMS
from local_connectors import source_id


def records(dataset, name):
    path, receipt = download('https://naturalearth.s3.amazonaws.com/'+dataset+'.zip', name+'.zip', 32*1024*1024)
    with zipfile.ZipFile(path) as archive:
        def part(extension): return io.BytesIO(archive.read(next(n for n in archive.namelist() if n.endswith(extension))))
        reader=shapefile.Reader(shp=part('.shp'),shx=part('.shx'),dbf=part('.dbf'),encoding='utf-8')
        acquisition={k:v for k,v in receipt.items() if k not in ('archived','chunks','manifestKey','manifestSha256')}
        return [(row.record.as_dict(),shape(row.shape.__geo_interface__)) for row in reader.iterShapeRecords()],acquisition


def install(api):
    editions=[]
    countries,receipt=records('110m_cultural/ne_110m_admin_0_countries','navigation-world-countries')
    features=[]
    for attrs,geometry in countries:
        # Normalize upstream Russia boundary floating-point noise only.
        geometry=transform(lambda x,y,z=None:(max(-180.,min(180.,x)),max(-90.,min(90.,y))),geometry)
        code=str(attrs.get('ADM0_A3') or attrs.get('SOV_A3'))
        names=[attrs.get(k) for k in ['NAME','NAME_EN','NAME_AR','FORMAL_EN','BRK_NAME']]
        props={'key':'country:'+code,'parentKey':'','kind':'country','name':attrs.get('NAME_EN') or attrs['NAME'],
               'aliases':' | '.join(str(n) for n in names if n),'iso':code,'transformation':'Topology-preserving simplification 0.015 degrees; normalize floating-point noise at WGS84 limits','meaning':'Generalized navigation reference; not an official jurisdiction determination'}
        features.append({'type':'Feature','properties':props,'geometry':mapping(geometry.simplify(.015,preserve_topology=True))})
    editions.append(('World countries · Natural Earth 1:110m',features,receipt))
    divisions,receipt=records('10m_cultural/ne_10m_admin_1_states_provinces','navigation-admin1')
    features=[];parents=[]
    for attrs,geometry in divisions:
        if attrs.get('adm0_a3')!='SAU':continue
        code=str(attrs.get('iso_3166_2') or attrs.get('adm1_code'))
        props={'key':'region:'+code,'parentKey':'country:SAU','kind':'region','name':attrs.get('name_en') or attrs['name'],
               'aliases':' | '.join(str(attrs[k]) for k in ['name','name_en','name_ar','gn_name'] if attrs.get(k)),
               'meaning':'Retained generalized first-order administrative reference; source edition date in receipt'}
        geometry=geometry.simplify(.003,preserve_topology=True)
        features.append({'type':'Feature','properties':props,'geometry':mapping(geometry)});parents.append((props['key'],geometry))
    if len(features)<10:raise RuntimeError('Saudi administrative reference unexpectedly incomplete')
    jeddah=box(JEDDAH['west'],JEDDAH['south'],JEDDAH['east'],JEDDAH['north'])
    parent=next((key for key,g in parents if g.covers(jeddah.centroid)),'country:SAU')
    features.append({'type':'Feature','properties':{'key':'study:jeddah-coast','parentKey':parent,'kind':'study','name':'Jeddah coastal study area','aliases':'Jeddah جدة coastal environment','meaning':'Bounded prepared environment; not a municipal boundary'},'geometry':mapping(jeddah)})
    editions.append(('Saudi administrative navigation · Natural Earth',features,receipt))
    places,receipt=records('50m_cultural/ne_50m_populated_places','navigation-populated-places')
    saudi=next(g for a,g in countries if a.get('ADM0_A3')=='SAU')
    features=[]
    for n,(attrs,geometry) in enumerate(places):
        if not saudi.covers(geometry):continue
        parent=next((key for key,g in parents if g.covers(geometry)),'country:SAU')
        features.append({'type':'Feature','properties':{'key':'place:SAU:'+str(n),'parentKey':parent,'kind':'place','name':attrs.get('NAME_EN') or attrs.get('NAME') or 'Retained populated place','aliases':' | '.join(str(attrs[k]) for k in ['NAME','NAME_EN','NAME_AR'] if attrs.get(k)),'meaning':'Place location; not a city boundary'},'geometry':mapping(geometry)})
    if features:editions.append(('Saudi populated places · Natural Earth',features,receipt))
    admitted=[]
    for label,features,receipt in editions:
        raw=encoded({'type':'FeatureCollection','features':features})
        digest=sha(raw)
        value={'sourceId':source_id('naturalearth'),'label':label,'version':digest[:24],
               'rights':rights('naturalearth-navigation-public-domain-v1',NE_TERMS,'Made with Natural Earth; retained generalized navigation reference'),
               'receipt':receipt,'sha256':digest,'artifact':base64.b64encode(raw).decode()}
        result=api.call('POST','/geography/editions',value)
        admitted.append({'id':result['id'],'label':label,'features':len(features),'sha256':digest,'receipt':receipt})
    store_json(DATA/'geographic-navigation.json',{'schema':'emad.geography-navigation.v1','editions':admitted})
    print('Geographic navigation installed: '+str(sum(e['features'] for e in admitted))+' admitted features',flush=True)
    return admitted


if __name__=='__main__':
    from install_local import API
    install(API())
