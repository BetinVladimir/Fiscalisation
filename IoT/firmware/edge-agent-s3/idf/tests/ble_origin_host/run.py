#!/usr/bin/env python3
"""Actual queue -> authenticated processor/SQLite -> production delivery -> BLE.

Only RTOS/NimBLE SDKs are fake; every subprocess and fake worker is bounded.
"""
from pathlib import Path
import argparse, base64, json, os, shlex, subprocess, tempfile

HERE = Path(__file__).resolve().parent
IDF = HERE.parents[1]
REPO = IDF.parents[3]
CASES = ['live','queued-reset','executing-reset','missing-owner','notify-error','duplicate','unavailable','queue-wait','queue-wait-timeout','http','recovery','reconcile-live','reconcile-reset']
p = argparse.ArgumentParser(); p.add_argument('--case', action='append', choices=CASES); args = p.parse_args()
def run(command, *, timeout=180, **kw):
    subprocess.run([str(x) for x in command], check=True, timeout=timeout, **kw)

env = dict(os.environ)
cache = Path(tempfile.gettempdir()) / 'beefiscal-ble-origin-go'
env.setdefault('GOCACHE', str(cache/'build')); env.setdefault('GOMODCACHE', str(cache/'mod'))
cjson = Path(env.get('CJSON_SOURCE_DIR', env.get('CJSON_SOURCE', str(Path.home()/'.platformio/packages/framework-espidf/components/json/cJSON'))))
mbed = Path(env.get('MBEDTLS_SOURCE_DIR', str(Path.home()/'.platformio/packages/framework-espidf/components/mbedtls/mbedtls')))
for dependency in [cjson/'cJSON.c', cjson/'cJSON.h', mbed/'include/mbedtls/pk.h']:
    if not dependency.is_file(): raise RuntimeError(f'actual dependency unavailable: {dependency}')
cc = shlex.split(env.get('CC','cc')); cxx = shlex.split(env.get('CXX','c++'))
with tempfile.TemporaryDirectory(prefix='beefiscal-ble-origin-') as name:
    d = Path(name); fixtures = d/'fixtures'; fixtures.mkdir()
    env['EXECUTION_AUTH_FIXTURE_DIR'] = str(fixtures)
    env['PAYMENT_RECONCILIATION_FIXTURE_DIR'] = str(fixtures/'reconcile')
    run([env.get('GO','go'),'test','./internal/domain','-run','^(TestExecutionAuthorizationActualRootSignatureAndFirmwareFixture|TestPaymentReconciliationIssuance)$','-count=1'], cwd=REPO/'fiscal-backend', env=env)
    fixture = fixtures/'execution-authorization-fixture.json'
    reconciliation = fixtures/'reconcile/fixture.json'
    def trust_pem(source, output):
        key = json.loads(source.read_text())['trust_key_der_base64']
        der = base64.urlsafe_b64decode(key+'='*((-len(key))%4)); encoded = base64.b64encode(der).decode()
        output.write_text('-----BEGIN PUBLIC KEY-----\n'+'\n'.join(encoded[at:at+64] for at in range(0,len(encoded),64))+'\n-----END PUBLIC KEY-----\n')
    trust_pem(fixture,d/'trust.pem'); trust_pem(reconciliation,d/'reconcile.pem')
    library = env.get('MBEDTLS_CRYPTO_LIBRARY')
    if library:
        if not Path(library).is_file(): raise RuntimeError('MBEDTLS_CRYPTO_LIBRARY missing')
    else:
        run(['cmake','-S',mbed,'-B',d/'mbedtls','-DENABLE_PROGRAMS=OFF','-DENABLE_TESTING=OFF','-DUSE_SHARED_MBEDTLS_LIBRARY=OFF'],env=env)
        run(['cmake','--build',d/'mbedtls','--target','mbedcrypto','-j',str(min(os.cpu_count() or 2,8))],env=env)
        library = d/'mbedtls/library/libmbedcrypto.a'
    flags = ['-fsanitize=address,undefined','-fno-omit-frame-pointer'] if env.get('SANITIZE') else []
    run(cc+flags+['-I',cjson,'-c',cjson/'cJSON.c','-o',d/'json.o'])
    include = [HERE/'include',mbed/'include',IDF/'tests/ble_native_include',IDF/'tests/storage_host',IDF/'main',cjson,REPO/'IoT/protocol-abstraction/include']
    command = cxx+['-std=c++17','-Wall','-Wextra','-Werror','-pthread']+flags
    for path in include: command += ['-I',path]
    sources = ['durable_storage.cpp','intent_processor.cpp','go_json.cpp','go_wire.cpp','execution_authorization.cpp','payment_reconciliation.cpp','canonical_cbor.cpp','ble_result_delivery.cpp']
    run(command+[IDF/'main'/s for s in sources]+[HERE/'runtime.cpp',d/'json.o',library,'-lsqlite3','-o',d/'runtime'])
    for case in args.case or CASES:
        source,trust=(reconciliation,d/'reconcile.pem') if case.startswith('reconcile-') else (fixture,d/'trust.pem')
        run([d/'runtime',source,trust,d/(case+'.sqlite'),case],timeout=20)
print(f'Actual BLE origin vertical: {len(args.case or CASES)} scenarios PASS')
