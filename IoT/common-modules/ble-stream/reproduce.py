#!/usr/bin/env python3
"""Build/test isolated sources only. No installs, GATT access or firmware flashing."""
from pathlib import Path
import tempfile,subprocess,shutil,hashlib,json,argparse,platform
p=argparse.ArgumentParser();p.add_argument('--evidence',type=Path,required=True);p.add_argument('--repo',type=Path);args=p.parse_args()
src=Path(__file__).resolve().parent;out=args.evidence.resolve();out.mkdir(parents=True,exist_ok=True)
files=['stream.h','stream.c','stream.ts','driver.c','boundary.c','test.mjs','counterfactuals.py']
with tempfile.TemporaryDirectory(prefix='mp026-stageb-') as temporary:
 work=Path(temporary)
 for f in files:shutil.copy2(src/f,work/f)
 def run(name,cmd):
  with (out/name).open('w') as log:
   log.write('COMMAND '+json.dumps(cmd)+'\n');log.flush();subprocess.run(cmd,cwd=work,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=120)
 run('compile.log',['cc','-std=c11','-Wall','-Wextra','-Werror','-fsanitize=address,undefined','-g','stream.c','driver.c','-o','c-peer'])
 run('boundary-compile.log',['cc','-std=c11','-Wall','-Wextra','-Werror','-fsanitize=address,undefined','-g','stream.c','boundary.c','-o','boundary'])
 run('boundary.log',['./boundary'])
 # Prove the boundary oracle detects the exact former unsigned-addition bug.
 unsafe=(work/'stream.c').read_text();guard='s->end>BS_MAX||n>BS_MAX-s->end';assert unsafe.count(guard)==1
 (work/'unsafe-size-addition.c').write_text(unsafe.replace(guard,'(uint64_t)s->end+n>BS_MAX'))
 run('boundary-unsafe-compile.log',['cc','-std=c11','-Wall','-Wextra','-Werror','-fsanitize=address,undefined','-g','unsafe-size-addition.c','boundary.c','-o','unsafe-boundary'])
 bad=subprocess.run(['./unsafe-boundary'],cwd=work,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=30)
 (out/'boundary-unsafe-counterfactual-red.log').write_bytes(bad.stdout)
 assert bad.returncode!=0 and b'AddressSanitizer: stack-buffer-overflow' in bad.stdout,'SIZE_MAX counterfactual falsely green'
 (out/'boundary-unsafe-counterfactual-result.json').write_text(json.dumps({'expected':'RED ASan stack-buffer-overflow','exit':bad.returncode})+'\n')
 run('duplex.log',['node','--experimental-strip-types','test.mjs'])
 run('counterfactuals.log',['python3','counterfactuals.py'])
 shutil.copy2(work/'counterfactual-results.json',out/'counterfactual-results.json')
 for f in (work/'counterfactuals').glob('*/red.log'):
  shutil.copy2(f,out/(f.parent.name+'-expected-red.log'))
 root=args.repo.resolve() if args.repo else src.parents[2]
 for app in ['BeeMiniPOS','miniposweb']:
  compiler=root/'minipos'/app/'node_modules/.bin/tsc'
  if compiler.exists():run(app+'-typescript.log',[str(compiler),'--noEmit','--target','ES2022','--module','ESNext','--strict','--skipLibCheck',str(work/'stream.ts')])
  else:(out/(app+'-typescript.log')).write_text('OPEN: local application TypeScript compiler unavailable; no packages installed.\n')
 sources={f:hashlib.sha256((src/f).read_bytes()).hexdigest() for f in files+['reproduce.py']}
 (out/'manifest.json').write_text(json.dumps({'stage':'B only; actual callbacks/TLS/auth/HIL OPEN','source_sha256':sources,'host':platform.platform(),'logs':{f.name:hashlib.sha256(f.read_bytes()).hexdigest() for f in sorted(out.iterdir()) if f.is_file() and f.name!='manifest.json'}},indent=2)+'\n')
print('PASS Stage B isolated ASan/UBSan C↔TS + six expected-RED mutations')
