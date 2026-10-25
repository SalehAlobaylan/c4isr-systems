"""Retain real Saudi reference layers and transform a bounded Jeddah environment."""
import base64
import io
import json
import math
import zipfile
from pathlib import Path
import numpy as np
import shapefile
from shapely.geometry import box, shape, mapping, Polygon, LineString
from shapely.ops import transform
import rasterio
from rasterio.warp import reproject, Resampling
from rasterio.transform import from_bounds
from pyproj import Transformer
import osmium
from recovery_data import DATA, SAUDI, JEDDAH, download, encoded, sha, store_json

NE_TERMS = 'https://www.naturalearthdata.com/about/terms-of-use/'
OSM_TERMS = 'https://www.openstreetmap.org/copyright'
DEM_TERMS = 'https://dataspace.copernicus.eu/explore-data/data-collections/copernicus-contributing-missions/collections-description/COP-DEM'


def rights(name, terms, attribution):
    return {'id': name, 'legalBasis': 'Published public dataset licence; retained attributed development snapshot', 'termsUrl': terms,
            'attribution': attribution, 'retainRaw': True, 'allowAnalysis': True, 'allowSimulationExport': True}


def feature(identifier, geometry, properties):
    return {'type': 'Feature', 'id': str(identifier), 'geometry': mapping(geometry), 'properties': properties}


def reference(source, key, features, receipt, aoi, producer, licence, quality, kind='vector', datum='not_applicable', resolution=None):
    raw = encoded({'type': 'FeatureCollection', 'features': features} if kind == 'vector' else features)
    if len(raw) > 524288:
        raise ValueError('Reference artifact exceeds 512 KiB: ' + key)
    return {'sourceId': source, 'datasetKey': key, 'version': sha(raw)[:20], 'label': key.replace('-', ' ').title(),
            'kind': kind, 'originalProducer': producer, 'crs': 'EPSG:4326', 'verticalDatum': datum,
            'resolution': resolution or {'value': 50000000, 'unit': 'map_scale'}, 'aoi': aoi,
            'validity': 'Retained dated reference snapshot; no claim of current completeness', 'quality': quality,
            'transformation': 'emad.geography.v1; explicit AOI intersection and geometry simplification; source bytes retained',
            'rights': licence, 'retrievalReference': receipt['url'], 'parentArtifactSha256': receipt['sha256'],
            'retrievedAt': receipt['retrievedAt'], 'sha256': sha(raw), 'artifact': base64.b64encode(raw).decode()}


def chunks(source, key, features, receipt, aoi, producer, licence, quality, count=180):
    out = []
    for start in range(0, len(features), count):
        out.append(reference(source, key + '-' + str(start // count + 1), features[start:start + count], receipt, aoi, producer, licence, quality))
    return out


def natural_earth():
    bounds = box(SAUDI['west'], SAUDI['south'], SAUDI['east'], SAUDI['north'])
    result = []
    definitions = [('countries', '50m_cultural/ne_50m_admin_0_countries', .005), ('places', '50m_cultural/ne_50m_populated_places', 0),
                   ('major-roads', '10m_cultural/ne_10m_roads', .002), ('coastline', '50m_physical/ne_50m_coastline', .002)]
    for label, dataset, tolerance in definitions:
        name = dataset.split('/')[-1]
        path, receipt = download('https://naturalearth.s3.amazonaws.com/' + dataset + '.zip', name + '.zip', 64 * 1024 * 1024)
        with zipfile.ZipFile(path) as archive:
            def part(ext): return io.BytesIO(archive.read(next(n for n in archive.namelist() if n.endswith(ext))))
            reader = shapefile.Reader(shp=part('.shp'), shx=part('.shx'), dbf=part('.dbf'), encoding='utf-8')
            features = []
            for i, record in enumerate(reader.iterShapeRecords()):
                attrs = record.record.as_dict()
                geom = shape(record.shape.__geo_interface__)
                if label == 'countries' and attrs.get('ADM0_A3') != 'SAU':
                    continue
                if not geom.intersects(bounds):
                    continue
                clipped = geom.intersection(bounds)
                if tolerance:
                    clipped = clipped.simplify(tolerance, preserve_topology=True)
                if clipped.is_empty or clipped.geom_type == 'GeometryCollection':
                    continue
                props = {k.lower(): v for k, v in attrs.items() if k.lower() in ['name', 'name_en', 'adm0_a3', 'type', 'pop_max', 'scalerank', 'sov_a3']}
                props.update({'layer': label, 'producer': 'Natural Earth', 'measured': False, 'meaning': 'Generalized geographic reference'})
                features.append(feature(name + '-' + str(i), clipped, props))
        result.extend(chunks('local-naturalearth', 'saudi-' + label, features, receipt, SAUDI, 'Natural Earth',
                             rights('naturalearth-public-domain', NE_TERMS, 'Made with Natural Earth'),
                             'Generalized cartography, not a legal boundary or navigation database'))
    return result


def osm_layers():
    path, receipt = download('https://download.geofabrik.de/asia/gcc-states-latest.osm.pbf', 'gcc-states.osm.pbf', 400 * 1024 * 1024)
    area = box(JEDDAH['west'], JEDDAH['south'], JEDDAH['east'], JEDDAH['north'])
    center = area.centroid
    roads, buildings, places = [], [], []

    class Capture(osmium.SimpleHandler):
        def node(self, node):
            if not node.location.valid() or len(places) >= 300:
                return
            if node.tags.get('name') and (node.tags.get('place') or node.tags.get('amenity')):
                from shapely.geometry import Point
                point = Point(node.location.lon, node.location.lat)
                if area.covers(point):
                    places.append(feature('osm-node-' + str(node.id), point, {'name': node.tags['name'], 'layer': 'places', 'sourceType': 'osm'}))

        def way(self, way):
            building, road = way.tags.get('building'), way.tags.get('highway')
            if not building and not road:
                return
            try:
                coordinates = [(n.lon, n.lat) for n in way.nodes]
            except osmium.InvalidLocationError:
                return
            if len(coordinates) < 2:
                return
            xs, ys = zip(*coordinates)
            if max(xs) < JEDDAH['west'] or min(xs) > JEDDAH['east'] or max(ys) < JEDDAH['south'] or min(ys) > JEDDAH['north']:
                return
            props = {'name': way.tags.get('name', ''), 'sourceType': 'osm'}
            if building and len(coordinates) >= 4 and coordinates[0] == coordinates[-1]:
                geom = Polygon(coordinates)
                if not geom.is_valid:
                    return
                geom = geom.intersection(area).simplify(.000003, preserve_topology=True)
                if geom.geom_type != 'Polygon' or geom.is_empty:
                    return
                try:
                    height = float(way.tags.get('height', ''))
                    basis = 'OSM supplied height; unverified'
                except ValueError:
                    try: height = float(way.tags.get('building:levels', '2')) * 3
                    except ValueError: height = 6
                    basis = 'Estimated 3 m per level; two levels if unspecified'
                props.update({'layer': 'buildings', 'heightM': max(2, min(120, height)), 'heightBasis': basis})
                buildings.append(feature('osm-way-' + str(way.id), geom, props))
            elif road:
                geom = LineString(coordinates).intersection(area).simplify(.00001, preserve_topology=True)
                if geom.geom_type not in ['LineString', 'MultiLineString'] or geom.is_empty:
                    return
                props.update({'layer': 'roads', 'highway': road})
                roads.append(feature('osm-way-' + str(way.id), geom, props))

    cache = DATA / 'osm-clipped.json'
    if cache.exists():
        document = json.loads(cache.read_text(encoding='utf-8'))
        if document['parentSha256'] != receipt['sha256']:
            raise ValueError('OSM cache parent changed')
        return document, receipt
    handler = Capture()
    handler.apply_file(str(path), locations=True, idx='flex_mem')
    nearest = lambda f: shape(f['geometry']).centroid.distance(center)
    counts = {'roads': len(roads), 'buildings': len(buildings), 'places': len(places)}
    document = {'parentSha256': receipt['sha256'], 'aoi': JEDDAH, 'countsBeforeBudget': counts,
                'roads': sorted(roads, key=nearest)[:750], 'buildings': sorted(buildings, key=nearest)[:1800], 'places': places[:100],
                'limitations': ['Closed-way footprints only; multipolygon relations omitted.', 'Nearest 1800 buildings and 750 road segments retained within the scene budget.', 'OSM completeness and supplied dimensions are not independently verified.']}
    store_json(cache, document)
    return document, receipt


def terrain():
    dem, receipt = download('https://copernicus-dem-30m.s3.amazonaws.com/Copernicus_DSM_COG_10_N21_00_E039_00_DEM/Copernicus_DSM_COG_10_N21_00_E039_00_DEM.tif', 'jeddah-copdem30.tif', 64 * 1024 * 1024)
    geoid, geoid_receipt = download('https://cdn.proj.org/us_nga_egm08_25.tif', 'us_nga_egm08_25.tif', 128 * 1024 * 1024)
    if geoid_receipt['sha256']!='4191d471eefebf24091b56dbc604353cb3b8cf8cc70e448bb9ae56a272bef17a':
        raise ValueError('EGM2008 grid differs from the admitted hash-pinned transformation input')
    pipeline = '+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad +step +proj=vgridshift +grids=' + geoid.as_posix() + ' +multiplier=1 +step +proj=unitconvert +xy_in=rad +xy_out=deg'
    conversion = Transformer.from_pipeline(pipeline)
    def sample(size):
        # Point samples are south-to-north for the context grid; rendered meshes
        # use this same convention. Ocean missing cells stay explicitly missing.
        step = (JEDDAH['east'] - JEDDAH['west']) / (size - 1)
        xs = np.linspace(JEDDAH['west'], JEDDAH['east'], size)
        ys = np.linspace(JEDDAH['south'], JEDDAH['north'], size)
        lng, lat = np.meshgrid(xs, ys)
        with rasterio.open(dem) as src:
            data = np.array([v[0] for v in src.sample(zip(lng.ravel(), lat.ravel()), masked=True)], dtype=np.float64).reshape((size, size))
        valid = np.isfinite(data) & (data > -12000) & (data < 10000)
        _, _, heights = conversion.transform(lng, lat, data, errcheck=True)
        back = conversion.transform(lng, lat, heights, direction='INVERSE', errcheck=True)[2]
        if np.nanmax(np.abs(back - data)) > .001:
            raise ValueError('Vertical conversion round trip exceeded 1 mm')
        heights[~valid] = np.nan
        return heights, lng, lat, step
    coarse, _, _, step = sample(64)
    fine, lng, lat, _ = sample(256)
    grid = {'width': 64, 'height': 64, 'west': JEDDAH['west'], 'south': JEDDAH['south'], 'stepX': step, 'stepY': step,
            'registration': 'point_samples', 'values': [round(float(v), 4) if math.isfinite(v) else None for v in coarse.ravel()]}
    licence = rights('copdem-public-snapshot', DEM_TERMS, 'Copernicus DEM GLO-30; ESA / Airbus; EGM2008 grid NGA / PROJ-data (public domain)')
    ref = reference('local-dem', 'jeddah-ellipsoid-terrain', grid, receipt, JEDDAH, 'Copernicus DEM / NGA EGM2008', licence,
                    'DSM, not bare-earth surveyed terrain. Context grid decimated from nominal 30 m source; rendered terrain retains finer sampling.', 'elevation', 'WGS84_ELLIPSOID', {'value': step * 3600, 'unit': 'arc_seconds'})
    ref['transformation'] = 'emad.geography.v1; H(EGM2008)+N=WGS84 ellipsoidal h; PROJ vgridshift multiplier +1; geoid SHA256=' + geoid_receipt['sha256'] + '; round-trip error <=0.001 m'
    origin = {'longitude': 39.18, 'latitude': 21.5, 'height': float(fine[128, 128]), 'verticalDatum': 'WGS84_ELLIPSOID'}
    frame = {'crs': 'EPSG:4326', 'order': 'longitude_latitude', 'axes': 'east_north_up', 'units': 'metres',
             'orientation': 'heading_clockwise_from_north_pitch_nose_up_roll_right_degrees', 'transform': 'wgs84-ecef-enu-v1', 'origin': origin}
    np.savez_compressed(DATA / 'terrain.npz', heights=fine, longitude=lng, latitude=lat)
    store_json(DATA / 'frame.json', frame)
    store_json(DATA / 'terrain-lineage.json', {'dem': receipt, 'geoid': geoid_receipt, 'conversion': ref['transformation'], 'frame': frame})
    return ref


def prepare():
    output = DATA / 'reference-inputs.json'
    if output.exists():
        return json.loads(output.read_text(encoding='utf-8'))
    refs = natural_earth()
    clipped, receipt = osm_layers()
    licence = rights('osm-odbl-geofabrik', OSM_TERMS, '© OpenStreetMap contributors; Geofabrik snapshot; ODbL 1.0')
    for label in ['roads', 'buildings', 'places']:
        refs.extend(chunks('local-osm', 'jeddah-' + label, clipped[label], receipt, JEDDAH, 'OpenStreetMap contributors / Geofabrik', licence,
                           '; '.join(clipped['limitations']), count=250))
    refs.append(terrain())
    store_json(output, refs)
    print('Prepared ' + str(len(refs)) + ' real reference versions', flush=True)
    return refs


if __name__ == '__main__':
    prepare()
