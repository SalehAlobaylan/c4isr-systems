"""Independent Python raster worker using scoped leases and retained byte windows."""
import argparse
import signal
import threading
import time
from install_local import API, artifact, work_once
from capture_imagery import capture


def work(api, claim):
    job, lease = claim['job'], claim['lease']
    stopped, finished = threading.Event(), threading.Event()
    def progress(message, result=None, failed=False):
        return api.call('POST','/imagery/captures/'+job['id']+'/progress',{'lease':lease,'progress':message,'result':result,'failed':failed})
    def keep_lease():
        while not finished.wait(10):
            try: progress('Retaining and preparing source windows')
            except Exception: stopped.set();return
    thread=threading.Thread(target=keep_lease,daemon=True);thread.start()
    try:
        directory,metadata=capture(job['input']['scene'],job['input']['aoi'],cancelled=stopped)
        if stopped.is_set():return
        source=job['sourceId']
        subset=artifact(api,directory/'subset.json',source,'imagery_subset')
        rgb=artifact(api,directory/'rgb.png',source,'imagery_preview','image/png')
        quality=artifact(api,directory/'quality.png',source,'imagery_preview','image/png')
        receipt=artifact(api,directory/'receipts.json',source,'imagery_capture')
        windows=[]
        for name in ['B04_10m.tif','B08_10m.tif','SCL_20m.tif','B03_10m.tif','B02_10m.tif']:
            if stopped.is_set():return
            windows.append(artifact(api,directory/name,source,'imagery_window','image/tiff')['id'])
        prepared=api.call('POST','/imagery/jobs',{'requestKey':'capture-prepare-'+subset['sha256'][:32],'kind':'PREPARE','sceneId':job['input']['scene']['id'],'artifactId':subset['id']})
        if prepared['status']!='SUCCEEDED':work_once('worker-imagery','emad-imagery')
        for _ in range(120):
            if stopped.is_set():return
            prepared=api.call('GET','/imagery/jobs/'+prepared['id'])
            if prepared['status'] in ['SUCCEEDED','FAILED','CANCELLED']:break
            time.sleep(1)
        if prepared['status']!='SUCCEEDED':raise RuntimeError('retained_subset_preparation_failed')
        progress('Real source windows retained and validated',{'preparationId':prepared['id'],'subsetArtifactId':subset['id'],
                 'rgbArtifactId':rgb['id'],'qualityArtifactId':quality['id'],'receiptArtifactId':receipt['id'],'windowArtifactIds':windows})
        print('Capture complete: '+job['id'],flush=True)
    except Exception:
        if not stopped.is_set():
            try:progress('Capture failed; retained inputs are preserved for retry',failed=True)
            except Exception:pass
        raise
    finally:
        finished.set();thread.join(timeout=2)


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--once',action='store_true');args=parser.parse_args()
    api=API('worker-preparation');shutdown=threading.Event()
    signal.signal(signal.SIGINT,lambda *_:shutdown.set())
    signal.signal(signal.SIGTERM,lambda *_:shutdown.set())
    def readiness():
        while not shutdown.wait(3):
            try:api.call('POST','/platform/worker-heartbeats',{'feature':'preparation'})
            except Exception:pass
    threading.Thread(target=readiness,daemon=True).start()
    while not shutdown.is_set():
        try:
            api.call('POST','/platform/worker-heartbeats',{'feature':'preparation'})
            claim=api.call('POST','/imagery/captures/claim')
            if claim.get('job'):work(api,claim)
        except Exception as error:
            print('Raster capture worker operation failed ('+type(error).__name__+')',flush=True)
            if args.once:raise
            shutdown.wait(5)
        if args.once:return
        shutdown.wait(1)


if __name__=='__main__':main()
