"""Validate the staging composition and smoke-test the built worker image."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
sys.path.insert(0,str(Path(__file__).resolve().parent))
import local

def main():
 cfg=local.configuration();cfg['profile']=os.environ.get('C4ISR_PRODUCT_PROFILE','map');env=local.environment(cfg)
 folder=local.PRIVATE/'staging-contract';folder.mkdir(exist_ok=True)
 files={'postgres_password':'packaging-check-only','database_url':'postgres://c4isr:packaging-check-only@postgres:5432/c4isr?sslmode=disable','auth_tokens':'operator-01=packaging-check-only','ui_token':'packaging-check-only','grafana_admin_password':'packaging-check-only','object_access_key':cfg['accessKey'],'object_secret_key':cfg['secretKey']}
 for name,data in files.items():
  path=folder/name;path.write_text(data,encoding='utf-8');os.chmod(path,0o600)
 for name in ['garage.env','garage.toml']:shutil.copyfile(local.PRIVATE/name,folder/name)
 shutil.copyfile(local.PRIVATE/'generated-connectors.json',folder/'connectors.json')
 for path in local.CREDS.glob('*.token'):shutil.copyfile(path,folder/path.name)
 composition=dict(env,STAGING_SECRET_DIR=str(folder),EMAD_CONNECTOR_SOURCE_ID='scoped_cc8a43409a82eaa8647ac38c__source__local-usgs')
 subprocess.run(['docker','compose','-f','deployments/staging/docker-compose.yml','-f','deployments/staging/delivery.yml','config','--quiet'],cwd=local.ROOT,env=composition,check=True)
 print('Staging delivery composition valid.',flush=True)
 worker={key:value for key,value in env.items() if key.startswith('C4ISR_') and '\n' not in value and not key.endswith('_FILE')}
 worker.update({'C4ISR_DATABASE_URL':env['C4ISR_DATABASE_URL'].replace('localhost:5433','host.docker.internal:5433').replace('127.0.0.1:5433','host.docker.internal:5433'),'C4ISR_ARTIFACT_S3_ENDPOINT':'http://host.docker.internal:3900','C4ISR_PUBLIC_CONNECTORS_FILE':'','C4ISR_RUNTIME_BUILDS_FILE':'/app/bin/runtime-builds.txt','C4ISR_ENV':'staging','C4ISR_ARTIFACT_S3_ALLOW_HTTP':'true'})
 worker.pop('C4ISR_RUNTIME_BUILDS',None)
 settings=folder/'worker.env';settings.write_text('\n'.join(k+'='+v for k,v in worker.items())+'\n',encoding='utf-8');os.chmod(settings,0o600)
 specs=[('worker-imagery','emad-imagery'),('local-map-analyst','emad-delivery')]
 if local.product_profile(cfg)=='historical-full':specs += [('worker-assets','emad-assets'),('worker-recordings','emad-recordings')]
 for name,app in specs:
  subprocess.run(['docker','run','--rm','--read-only','--tmpfs','/tmp','--env-file',str(settings),'--mount',f'type=bind,source={local.CREDS},target=/run/secrets,readonly','--entrypoint','/app/bin/'+app,'emad-delivery:recovery','-token-file','/run/secrets/'+name+'.token','-once'],cwd=local.ROOT,check=True)
  print(app+': scoped container startup and queue accepted.',flush=True)
 subprocess.run(['docker','run','--rm','--read-only','--tmpfs','/tmp','--entrypoint','python','emad-delivery:recovery','-c',"import sys;sys.path.insert(0,'scripts/data');import install_local,rasterio,pyproj,osmium;print('Raster worker dependencies ready')"],check=True)
 if local.product_profile(cfg)=='historical-full':subprocess.run(['docker','run','--rm','--entrypoint','/app/bin/emad-runtime','emad-delivery:recovery','-describe'],stdout=subprocess.DEVNULL,check=True)
 print('Linux '+local.product_profile(cfg)+' delivery executables accepted.',flush=True)
if __name__=='__main__':main()
