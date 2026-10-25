"""Generate bounded terrain and attributed OSM meshes outside the Go API."""
import json
import math
import numpy as np
from shapely.geometry import shape, Polygon
from shapely.ops import triangulate
from pyproj import Transformer
from glb_geometry import GLB, Parts, model
from recovery_data import DATA, JEDDAH, sha, store_json


def build(texture=None):
    frame = json.loads((DATA / 'frame.json').read_text())
    origin = frame['origin']
    pipeline = '+proj=pipeline +step +proj=cart +ellps=WGS84 +step +proj=topocentric +ellps=WGS84 +lon_0=' + str(origin['longitude']) + ' +lat_0=' + str(origin['latitude']) + ' +h_0=' + str(origin['height'])
    enu = Transformer.from_pipeline(pipeline)
    data = np.load(DATA / 'terrain.npz')
    heights, lng, lat = data['heights'], data['longitude'], data['latitude']
    east, north, up = enu.transform(lng, lat, heights)
    vertices = np.stack([east, up, -north],axis=-1).reshape((-1,3))
    size = heights.shape[0]
    triangles=[]
    for j in range(size-1):
        for i in range(size-1):
            a=j*size+i;b=a+1;c=a+size;d=c+1
            if np.isfinite(vertices[[a,b,c,d]]).all(): triangles.extend([[a,b,c],[b,d,c]])
    vertices=np.nan_to_num(vertices)
    uv=np.stack([(lng-JEDDAH['west'])/(JEDDAH['east']-JEDDAH['west']),1-(lat-JEDDAH['south'])/(JEDDAH['north']-JEDDAH['south'])],axis=-1).reshape((-1,2))
    glb=GLB();glb.mesh(vertices,triangles,glb.material('Retained satellite drape' if texture else 'Copernicus DSM', (1,1,1,1) if texture else (.52,.49,.39,1),texture),uv if texture else None)
    osm=json.loads((DATA/'osm-clipped.json').read_text(encoding='utf-8'))
    building_parts, road_parts=Parts(),Parts()
    def ground(x,y):
        i=int(np.clip(round((x-JEDDAH['west'])/(JEDDAH['east']-JEDDAH['west'])*(size-1)),0,size-1));j=int(np.clip(round((y-JEDDAH['south'])/(JEDDAH['north']-JEDDAH['south'])*(size-1)),0,size-1))
        return float(np.nanmin(heights[max(0,j-1):j+2,max(0,i-1):i+2]))
    def point(x,y,h):
        e,n,u=enu.transform(x,y,h);return [e,u,-n]
    def surface(part,geom,height=0,raise_m=0):
        polygons=[geom] if geom.geom_type=='Polygon' else list(geom.geoms) if geom.geom_type=='MultiPolygon' else []
        for polygon in polygons:
            base=ground(polygon.centroid.x,polygon.centroid.y)+raise_m
            for triangle in triangulate(polygon):
                if not polygon.covers(triangle.representative_point()): continue
                coords=list(triangle.exterior.coords)[:3];part.add([point(x,y,base+height) for x,y in coords],[[0,2,1]])
            if height:
                ring=list(polygon.exterior.coords)
                for a,b in zip(ring,ring[1:]):
                    part.add([point(*a,base),point(*b,base),point(*b,base+height),point(*a,base+height)],[[0,1,2],[0,2,3]])
    for feature in osm['buildings']:
        surface(building_parts,shape(feature['geometry']),feature['properties']['heightM'])
    for feature in osm['roads']:
        # The width is a display approximation, never routing/engineering data.
        surface(road_parts,shape(feature['geometry']).buffer(.000025,cap_style='flat',join_style='mitre'),raise_m=.8)
    if building_parts.positions: glb.mesh(building_parts.positions,building_parts.triangles,glb.material('OSM buildings; heights partly estimated',(.63,.67,.67,1)))
    if road_parts.positions: glb.mesh(road_parts.positions,road_parts.triangles,glb.material('OSM roads; approximate display width',(.12,.16,.19,1)))
    binary=glb.bytes()
    if len(binary)>16*1024*1024:raise ValueError('Prepared environment exceeds 16 MiB')
    (DATA/'jeddah-environment.glb').write_bytes(binary)
    lineage=json.loads((DATA/'terrain-lineage.json').read_text())
    manifest={'schema':'emad.prepared-environment.v1','aoi':JEDDAH,'frame':frame,'modelSha256':sha(binary),
              'terrainParentSha256':lineage['dem']['sha256'],'geoidSha256':lineage['geoid']['sha256'],
              'osmParentSha256':osm['parentSha256'],'imagerySha256':sha(texture) if texture else '',
              'inputVerticalDatum':'EGM2008','outputVerticalDatum':'WGS84_ELLIPSOID','transformation':lineage['conversion'],
              'terrainVertices':len(vertices),'buildings':len(osm['buildings']),'roadSegments':len(osm['roads']),
              'limitations':osm['limitations']+['Copernicus DSM includes surface objects, not surveyed bare earth.','Building bases and road widths are approximated for display.','No collision, navigation, hydrodynamic or terrain-clearance guarantee.']}
    store_json(DATA/'prepared-environment.json',manifest)
    for kind in ['fixed-wing','uav','helicopter','vehicle','vessel','sensor']:
        (DATA/(kind+'.glb')).write_bytes(model(kind))
    return manifest


if __name__=='__main__': build()
