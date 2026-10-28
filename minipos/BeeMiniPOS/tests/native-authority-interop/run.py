#!/usr/bin/env python3
"""Actual OS P256 -> actual Go issuer -> production PoP JS -> actual S3/SQLite.
Requires a booted simulator/emulator with the test app installed. No OS mocks.
The separate test apps expose prepare/sign only in their test executables.
"""
import argparse,base64,json,os,pathlib,subprocess,time
from instrumentation import validate_android_output
app=pathlib.Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('platform',choices=['ios','android']);p.add_argument('--device',required=True);p.add_argument('--output',type=pathlib.Path,required=True);p.add_argument('--repo',type=pathlib.Path,default=app.parents[1]);p.add_argument('--online',action='store_true');p.add_argument('--adb',default=str(pathlib.Path.home()/'Library/Android/sdk/platform-tools/adb'));a=p.parse_args();a.output.mkdir(parents=True,exist_ok=True)
def run(args,**kw):
 kw.setdefault("timeout",600)
 return subprocess.run([str(x) for x in args],check=True,**kw)
def read(args):return subprocess.check_output([str(x) for x in args],text=True,timeout=120)
scope={'backend':'https://native-authority-test.beeloy.local','tenant':'tenant-1','actorSubject':'subject-1','appInstance':'app-1','sessionID':'native-test-session-'+a.platform}
# This is the exact executionScopeID encoding for this ASCII-only fixture.
scope_id=json.dumps(list(scope.values()),separators=(',',':'))
(a.output/'scope.json').write_text(json.dumps(scope)); evidence_path=a.output/'native-evidence.json'
def native(message=''):
 if a.platform=='ios':
  bundle='com.beeloy.native-provider-test';subprocess.run(['xcrun','simctl','terminate',a.device,bundle],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30)
  container=pathlib.Path(read(['xcrun','simctl','get_app_container',a.device,bundle,'data']).strip());output=container/'Documents/native-authority-evidence.json'
  output.unlink(missing_ok=True)
  args=['xcrun','simctl','launch',a.device,bundle,'--authority-scope',scope_id]
  if message:args+=['--authority-message',message]
  run(args)
  for _ in range(100):
   if output.exists():break
   time.sleep(.1)
  evidence=json.loads(output.read_text())
 else:
  package='expo.modules.beeloyexecutioncrypto.test'
  # Never let a missing/zero-test run reuse an old same-scope prepare result.
  run([a.adb,'-s',a.device,'exec-out','run-as',package,'rm','-f','files/native-authority-evidence.json'],timeout=30)
  args=[a.adb,'-s',a.device,'shell','am','instrument','-w','-e','class','expo.modules.beeloyexecutioncrypto.ExecutionKeyStoreTest#authorityInterop','-e','authority_scope',scope_id]
  if message:args+=['-e','authority_message',message]
  args+=[package+'/androidx.test.runner.AndroidJUnitRunner'];result=read(args)
  validate_android_output(result)
  evidence=json.loads(read([a.adb,'-s',a.device,'exec-out','run-as',package,'cat','files/native-authority-evidence.json']))
 if evidence['message']!=message:raise RuntimeError('STALE_OS_EVIDENCE')
 evidence_path.write_text(json.dumps(evidence));return evidence
prepared=native();fixture=a.output/'issuer.json';env=dict(os.environ,BROWSER_CLIENT_SPKI=prepared['public_key'],BROWSER_AUTHORITY_FIXTURE=str(fixture),BROWSER_APP_INSTANCE=scope['appInstance'])
if a.online:env['BROWSER_AUTHORITY_ONLINE']='1'
run(['go','test','./internal/domain','-run','^TestBrowserExecutionAuthorizationFixture$','-count=1'],cwd=a.repo/'fiscal-backend',env=env)
message_path=a.output/'message.txt';run(['node','--experimental-strip-types',app/'tests/native-authority-interop/proof.ts',fixture,evidence_path,a.output/'scope.json',message_path])
signed=native(message_path.read_text())
if signed['public_key']!=prepared['public_key'] or not signed['signature']:raise RuntimeError('OS_KEY_NOT_PERSISTED')
f=json.loads(fixture.read_text());f['command']['client_proof']['signature']=signed['signature'];fixture.write_text(json.dumps(f))
env=dict(os.environ,AUTHORITY_EXTERNAL_FIXTURE=str(fixture),AUTHORITY_FIXTURE_OUTPUT_DIR=str(a.output))
run(['python3',a.repo/'IoT/firmware/edge-agent-s3/idf/tests/authority_host/run.py'],cwd=a.repo,env=env)
print(a.platform,'actual OS key -> Go issuer -> production PoP -> S3 PASS')
