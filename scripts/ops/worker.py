"""Container process wrapper with scoped readiness and graceful child shutdown."""
import argparse
import signal
import subprocess
import sys
import threading
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'data'))
from install_local import API

def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--name',required=True)
 parser.add_argument('--feature',required=True,choices=['imagery','assets','recordings','runtime','connectors','preparation'])
 parser.add_argument('command',nargs=argparse.REMAINDER)
 args=parser.parse_args();command=args.command[1:] if args.command[:1]==['--'] else args.command
 if not command:raise RuntimeError('worker executable required')
 api=API(args.name,'PUBLIC_EVIDENCE' if args.feature in ['imagery','connectors','preparation'] else 'EXERCISE_INPUT')
 stopped=threading.Event();process=subprocess.Popen(command)
 def stop(*_):stopped.set();process.terminate()
 signal.signal(signal.SIGTERM,stop);signal.signal(signal.SIGINT,stop)
 def heartbeat():
  while not stopped.wait(3):
   if process.poll() is not None:return
   try:
    body={'feature':args.feature}
    if args.feature=='runtime':body['runtimeBuild']=Path('/app/bin/runtime-builds.txt').read_text(encoding='utf-8').strip()
    api.call('POST','/platform/worker-heartbeats',body)
   except Exception:print('Worker readiness unavailable; inspecting the API and credential may be required.',flush=True)
 threading.Thread(target=heartbeat,daemon=True).start()
 try:return process.wait()
 finally:stopped.set()

if __name__=='__main__':sys.exit(main())
