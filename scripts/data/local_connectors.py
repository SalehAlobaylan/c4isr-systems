"""Trusted, fixed provider bindings. Provider access is supplied privately."""
import json
import hashlib
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RESOURCE = {'workspaceId': 'default', 'environmentId': 'local', 'partition': 'PUBLIC_EVIDENCE'}
def source_id(key):
    logical=key if key.startswith('local-') else 'local-'+key
    return 'scoped_'+hashlib.sha256(b'default/local/PUBLIC_EVIDENCE/').hexdigest()[:24]+'__source__'+logical
PROVIDERS = {
 'usgs': ('https://earthquake.usgs.gov/fdsnws/event/1/query', 'usgs-earthquakes-v1', 48),
 'usgs-history': ('https://earthquake.usgs.gov/fdsnws/event/1/query', 'usgs-earthquakes-v1', 20),
 'metar': ('https://aviationweather.gov/api/data/metar', 'noaa-metar-v1', 96),
 'gdacs': ('https://www.gdacs.org/xml/rss.xml', 'gdacs-eq-rss-v1', 48),
 'cdse': ('https://stac.dataspace.copernicus.eu/v1/search', 'cdse-sentinel2-stac-v1', 24),
 'earthsearch': ('https://earth-search.aws.element84.com/v1/search', 'earthsearch-sentinel2-c1-v1', 24),
 'firms': ('https://firms.modaps.eosdis.nasa.gov/api/area/csv/', 'firms-viirs-noaa20-v1', 24),
 'opensky': ('https://opensky-network.org/api/states/all', 'opensky-states-v1', 96),
 'ais': ('wss://stream.aisstream.io/v0/stream', 'aisstream-position-v1', 96),
}


def configure(env):
    path = ROOT / 'private/local/providers.json'
    access = json.loads(path.read_text(encoding='utf-8')) if path.exists() else {}
    entries, states = [], {}
    for key, (url, adapter, quota) in PROVIDERS.items():
        record = access.get(key, {})
        state, action = 'CONFIGURED', ''
        token_env = ''
        oauth = False
        if key == 'opensky' and record.get('permissionConfirmed') is not True:
            state, action = 'NEEDS_PERMISSION', 'Record OpenSky permission in private/local/providers.json before activation.'
        elif key=='opensky' and record.get('oauthCredentialsFile'):
            try:
                credentials=Path(record['oauthCredentialsFile'])
                if not credentials.is_absolute():credentials=ROOT/credentials
                values=json.loads(credentials.read_text(encoding='utf-8'))
                client_id=values.get('clientId',values.get('client_id',''))
                secret=values.get('clientSecret',values.get('client_secret',''))
                if not isinstance(client_id,str) or not isinstance(secret,str) or not 1<=len(client_id)<=512 or not 8<=len(secret)<=4096 or '\n' in client_id+secret or '\r' in client_id+secret:raise ValueError()
                env['OPENSKY_CLIENT_ID'],env['OPENSKY_CLIENT_SECRET']=client_id,secret
                oauth=True
            except (OSError,ValueError,TypeError,AttributeError):
                state,action='NEEDS_CREDENTIALS','Supply a valid private OpenSky OAuth client credentials JSON file.'
        elif key=='opensky' and record.get('authenticatedRequired') is True:
            state,action='NEEDS_CREDENTIALS','Configure oauthCredentialsFile after obtaining OpenSky API access.'
        if key in ['firms', 'ais']:
            token_env = 'FIRMS_MAP_KEY' if key == 'firms' else 'AISSTREAM_API_KEY'
            if key == 'ais' and record.get('permissionConfirmed') is not True:
                state, action = 'NEEDS_PERMISSION', 'Record AIS use/retention permission and a private API key file.'
            elif record.get('keyFile'):
                try:
                    secret_path = Path(record['keyFile'])
                    if not secret_path.is_absolute(): secret_path = ROOT / secret_path
                    token = secret_path.read_text(encoding='utf-8').strip()
                    if not token or '\n' in token or '\r' in token or len(token) > 4096:raise ValueError()
                    if key=='firms' and not re.fullmatch(r'[A-Za-z0-9_-]{16,128}',token):raise ValueError()
                    env[token_env] = token
                except (OSError,ValueError,TypeError):
                    state,action='NEEDS_CREDENTIALS','The configured private '+key+' key file is unavailable or invalid.'
            elif not env.get(token_env):
                state, action = 'NEEDS_CREDENTIALS', 'Supply a private ' + key + ' key file in private/local/providers.json.'
        states[key] = {'state': state, 'nextAction': action}
        entry = {'id': 'local-' + key, 'sourceId': source_id(key), 'resource': RESOURCE, 'url': url,
                 'adapter': adapter, 'maxRequestsPerDay': quota}
        if state!='CONFIGURED':entry.update(gateState=state,gateNextAction=action)
        if token_env: entry['authTokenEnv'] = token_env
        if oauth:entry.update(oauthClientIdEnv='OPENSKY_CLIENT_ID',oauthClientSecretEnv='OPENSKY_CLIENT_SECRET')
        entries.append(entry)
    return entries, states
