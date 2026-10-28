#!/usr/bin/env python3
"""Fresh lock-pinned dependencies + exact copied actual transports, no UI checkout."""
from pathlib import Path
import hashlib,json,os,re,shlex,shutil,subprocess,sys,tempfile
from check_lock import check,sha
HERE=Path(__file__).resolve().parent;REPO=HERE.parents[1]
SOURCES=[f'minipos/BeeMiniPOS/src/{name}' for name in ['webBle.ts','browserGatt.ts','bleAdvertising.ts','bleFrames.ts','portableCrypto.ts','bleResult.ts','cbor.ts','cborDelivery.test.ts']]+[f'minipos/miniposweb/src/fiscal/{name}' for name in ['webBleRoute.ts','browserGatt.ts']]+['minipos/miniposweb/tests/webBleLifecycle.test.ts']
EXPECTED_CASES={(state,amount) for state in ['FISCALIZED','FISCAL_RESULT_UNKNOWN','REJECTED'] for amount in ['2147483648','9007199254740993','9223372036854775807','-9223372036854775808']}
def verify_sources(repo,sandbox,hashes):
 for name,digest in hashes.items():
  source=repo/name;copied=sandbox/name
  if not source.resolve().is_relative_to(repo.resolve()) or not copied.resolve().is_relative_to(sandbox.resolve()) or source.is_symlink() or copied.is_symlink() or not source.is_file() or not copied.is_file() or sha(source)!=digest or sha(copied)!=digest:raise ValueError('source ownership/hash changed: '+name)
def validate_fixtures(path):
 rows=[json.loads(line) for line in path.read_text().splitlines()]
 if len(rows)!=12 or {(r['state'],r['amount']) for r in rows}!=EXPECTED_CASES:raise ValueError('missing/duplicate production fixture cases')
 for row in rows:
  payload=bytes.fromhex(row['hex']);frames=[bytes.fromhex(frame) for frame in row['frames']];offset=0
  if not payload or len(payload)>8192 or not frames:raise ValueError('fixture payload bounds')
  for frame in frames:
   if len(frame)<58 or frame[:4]!=b'BFF1' or frame[4:20].hex()!=row['id'].replace('-','') or int.from_bytes(frame[20:22],'big')!=len(payload) or int.from_bytes(frame[22:24],'big')!=offset or frame[25:57]!=hashlib.sha256(payload).digest():raise ValueError('fixture frame identity/digest/order')
   part=frame[57:];offset+=len(part)
   if frame[24]!=int(offset==len(payload)):raise ValueError('fixture final flag')
  if b''.join(frame[57:] for frame in frames)!=payload:raise ValueError('fixture frame payload mismatch')
 return {'rows':len(rows),'sha256':sha(path)}
def validate_reports(tap,report):
 text=tap.read_text()
 if not re.search(r'^# pass 1$',text,re.M) or not re.search(r'^# skipped 0$',text,re.M) or not re.search(r'^# fail 0$',text,re.M):raise ValueError('actual waiter fixture test skipped/failed')
 result=json.loads(report.read_text())
 if not result.get('success') or result.get('numFailedTests')!=0 or result.get('numPendingTests')!=0 or result.get('numPassedTests',0)<66:raise ValueError('full transport lifecycle test scope not passed')
 names={a['fullName']:a['status'] for test in result['testResults'] for a in test['assertionResults']}
 for kind in ['mini','pwa']:
  wanted=f'{kind}: actual WebBluetooth transport actual S3 delivery int64 bytes survive real result projection'
  if names.get(wanted)!='passed':raise ValueError('actual producer projection case not passed: '+wanted)
 return {'mini_waiter_tests':1,'vitest_passed':result['numPassedTests'],'fixture_projection_tests':2}
def main():
 configured=os.environ.get('BLE_TRANSPORT_EVIDENCE_DIR')
 evidence=Path(configured if configured else tempfile.mkdtemp(prefix='ble-transport-evidence-')).resolve();evidence.mkdir(parents=True,exist_ok=True)
 status={'status':'FAILED','stage':'dependency origin check'};env=dict(os.environ)
 # Preserve HOME; ignore ambient npm/Node loader settings instead.
 for key in list(env):
  if key.lower().startswith('npm_config_') or key in ['NODE_PATH','NODE_OPTIONS','NPM_TOKEN','NODE_AUTH_TOKEN']:env.pop(key)
 node=shlex.split(env.get('NODE','node'));npm=shlex.split(env.get('NPM','npm'));go=shlex.split(env.get('GO','go'))
 def run(command,name,*,cwd=REPO,timeout=180):
  with (evidence/name).open('w') as log:subprocess.run([str(x) for x in command],cwd=cwd,env=env,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=timeout)
  return (evidence/name).read_text()
 try:
  status['dependencies']=check(REPO,HERE)
  (evidence/'dependency-summary.json').write_text(json.dumps(status['dependencies'],indent=2)+'\n')
  revision=subprocess.run(['git','rev-parse','HEAD'],cwd=REPO,capture_output=True,text=True,timeout=10)
  status['git_commit']=revision.stdout.strip() if revision.returncode==0 else None
  status['source_snapshot']='git checkout' if revision.returncode==0 else 'unversioned isolated copy; file hashes authoritative'
  status['runner_sha256']=sha(HERE/'run.py');status['checker_sha256']=sha(HERE/'check_lock.py');status['config_sha256']=sha(HERE/'vitest.transport.config.ts')
  with tempfile.TemporaryDirectory(prefix='ble-transport-clean-') as name:
   sandbox=Path(name);cache=sandbox/'npm-cache';user=sandbox/'empty-user.npmrc';global_config=sandbox/'empty-global.npmrc';user.touch();global_config.touch()
   env['NPM_CONFIG_USERCONFIG']=str(user);env['NPM_CONFIG_GLOBALCONFIG']=str(global_config)
   status['home_preserved']=env.get('HOME')==os.environ.get('HOME');status['stage']='toolchain pin'
   node_version=run(node+['--version'],'node-version.log').strip();npm_version=run(npm+['--version'],'npm-version.log').strip()
   if node_version!='v22.14.0' or npm_version!='10.9.2':raise ValueError('requires exact Node22.14.0/npm10.9.2')
   status['toolchain']={'node':node_version,'npm':npm_version}
   for label,command in [('go',go),('cc',shlex.split(env.get('CC','cc'))),('cxx',shlex.split(env.get('CXX','c++')))]:
    status['toolchain'][label]=run(command+(['version'] if label=='go' else ['--version']),label+'-version.log').splitlines()[0]
   status['stage']='copy actual sources'
   hashes={}
   for relative in SOURCES:
    source=REPO/relative
    if not source.resolve().is_relative_to(REPO.resolve()) or source.is_symlink() or not source.is_file():raise ValueError('actual source missing/not regular: '+relative)
    target=sandbox/relative;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target);hashes[relative]=sha(source)
   for relative in ['package.json','package-lock.json','vitest.transport.config.ts']:shutil.copyfile(HERE/relative,sandbox/relative)
   verify_sources(REPO,sandbox,hashes);status['source_sha256']=hashes
   if len(hashes)!=11:raise ValueError('source inventory changed without review')
   status['clean_install']={'node_modules_absent':not (sandbox/'node_modules').exists(),'cache_absent':not cache.exists(),'empty_userconfig_sha256':sha(user),'empty_globalconfig_sha256':sha(global_config)}
   status['stage']='fresh npm ci'
   run(npm+['ci','--ignore-scripts','--no-audit','--no-fund','--userconfig',user,'--globalconfig',global_config,'--cache',cache,'--registry=https://registry.npmjs.org/'],'npm-ci.log',cwd=sandbox,timeout=300)
   if sha(sandbox/'package-lock.json')!=sha(HERE/'package-lock.json') or sha(sandbox/'package.json')!=sha(HERE/'package.json'):raise ValueError('npm changed reviewed dependency manifests')
   if any(p.is_symlink() for p in [sandbox/'node_modules',sandbox/'package.json',sandbox/'package-lock.json']):raise ValueError('dependency root must be owned regular install')
   installed=json.loads((sandbox/'node_modules/.package-lock.json').read_text())['packages'];reviewed=json.loads((HERE/'package-lock.json').read_text())['packages']
   for path,entry in installed.items():
    if path not in reviewed or any(entry.get(key)!=reviewed[path].get(key) for key in ['version','resolved','integrity']):raise ValueError('installed package not in reviewed registry lock: '+path)
   if any('node_modules/'+name not in installed for name in ['cbor-x','@noble/ciphers','@noble/curves','@noble/hashes','vite','vitest']):raise ValueError('required actual runtime dependency not installed')
   status['installed_packages']={'count':len(installed),'sha256':sha(sandbox/'node_modules/.package-lock.json')}
   (evidence/'dependency-summary.json').write_text(json.dumps({**status['dependencies'],'installed':status['installed_packages']},indent=2)+'\n')

   status['stage']='actual native producer'
   fixture=sandbox/'production-delivery.jsonl';env['CBOR_DELIVERY_FIXTURES']=str(fixture)
   env.setdefault('GOCACHE',str(sandbox/'go-cache'));env.setdefault('GOMODCACHE',str(sandbox/'go-mod-cache'))
   run([sys.executable,REPO/'IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py','--case','live'],'native-producer.log',timeout=300)
   status['fixture']=validate_fixtures(fixture);shutil.copyfile(fixture,evidence/'production-delivery.jsonl')
   status['stage']='independent Go decoder'
   run(go+['run',REPO/'IoT/firmware/edge-agent-s3/idf/tests/cbor_encoder_host/oracle.go',fixture],'go-decoder.log')
   status['stage']='actual MiniPOS waiter'
   run(node+['--experimental-strip-types','--test-reporter=tap','--test','src/cborDelivery.test.ts'],'mini-waiter.tap',cwd=sandbox/'minipos/BeeMiniPOS')
   status['stage']='actual full browser lifecycle/projection'
   run(node+[sandbox/'node_modules/vitest/vitest.mjs','run','--config',sandbox/'vitest.transport.config.ts','--reporter=json','--outputFile',evidence/'vitest.json'],'browser.log',cwd=sandbox)
   status['reports']=validate_reports(evidence/'mini-waiter.tap',evidence/'vitest.json')
   status['stage']='final provenance'
   verify_sources(REPO,sandbox,hashes)
   if sha(fixture)!=status['fixture']['sha256']:raise ValueError('fixture changed during test')
   check(REPO,HERE)
   status['status']='PASS';status['stage']='complete'
 except Exception as error:
  status['error']=(f'{type(error).__name__}: subprocess failed or timed out' if isinstance(error,(subprocess.CalledProcessError,subprocess.TimeoutExpired)) else f'{type(error).__name__}: {error}');raise
 finally:
  (evidence/'manifest.json').write_text(json.dumps(status,indent=2)+'\n');print('BLE transport evidence: '+str(evidence),flush=True)
 return 0
if __name__=='__main__':raise SystemExit(main())
