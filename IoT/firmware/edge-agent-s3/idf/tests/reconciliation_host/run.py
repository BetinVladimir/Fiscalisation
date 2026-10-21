#!/usr/bin/env python3
import pathlib,os,sys,subprocess,tempfile,json,base64
h=pathlib.Path(__file__).resolve().parent;i=h.parents[1];sys.path.insert(0,str(h.parent))
from mbedtls_host import crypto_dependency
fixture=pathlib.Path(os.environ.get('RECONCILIATION_FIXTURE',str(h/'issuer_fixture.json')))
cjson=pathlib.Path(os.environ.get('CJSON_SOURCE_DIR',os.environ.get('CJSON_SOURCE',str(pathlib.Path.home()/'.platformio/packages/framework-espidf/components/json/cJSON'))))
if not all((cjson/name).is_file() for name in ('cJSON.c','cJSON.h')):
 raise RuntimeError('actual cJSON sources unavailable; set CJSON_SOURCE_DIR')
with tempfile.TemporaryDirectory(prefix='reconciliation-host-') as td:
 d=pathlib.Path(td);inc,lib=crypto_dependency(d)
 def run(a):subprocess.run(list(map(str,a)),check=True)
 run([os.environ.get('CC','cc'),'-I',cjson,'-c',cjson/'cJSON.c','-o',d/'json.o'])
 run([os.environ.get('CXX','c++'),'-std=c++17','-Wall','-Wextra','-Werror','-pthread','-I',inc,'-I',i/'tests/storage_host','-I',i/'main','-I',cjson]+[(pathlib.Path(os.environ['RECONCILIATION_BASE_MAIN'])/f if os.environ.get('RECONCILIATION_BASE_MAIN') and f in ['intent_processor.cpp','durable_storage.cpp'] else i/'main'/f) for f in ['durable_storage.cpp','intent_processor.cpp','go_json.cpp','go_wire.cpp','execution_authorization.cpp','payment_reconciliation.cpp']]+[h/'runtime.cpp',d/'json.o',lib,'-lsqlite3','-o',d/'test'])
 f=json.loads(fixture.read_text());key=f['trust_key_der_base64'];(d/'key.der').write_bytes(base64.urlsafe_b64decode(key+'='*((-len(key))%4)));run(['openssl','pkey','-pubin','-inform','DER','-in',d/'key.der','-out',d/'key.pem'])
 for mode in ['valid','signature','tenant','terminal','binding','expired','future','alias','row','admin-fault','outbox-fault']:
  run([d/'test',fixture,d/'key.pem',d/(mode+'.sqlite'),mode,os.environ.get('RECONCILIATION_EVENT_OUTPUT',str(d/'event.json'))])
 backend=pathlib.Path(os.environ.get('FISCAL_REPO_ROOT',str(i.parents[3])))/'fiscal-backend'
 env=dict(os.environ,PAYMENT_RECONCILIATION_HOST_BINARY=str(d/'test'),GOCACHE=os.environ.get('GOCACHE','/private/tmp/fiscal-reconciliation-go-cache'))
 subprocess.run(['go','test','./internal/domain','-run','TestPaymentReconciliationActualFirmwareInterop','-count=1','-v'],cwd=backend,env=env,check=True)
