#!/usr/bin/env python3
from pathlib import Path
import os,subprocess,tempfile
h=Path(__file__).resolve().parent;i=h.parents[1];src=Path(os.environ.get('SOURCE_ROOT',str(i)))
cjson=Path(os.environ.get('CJSON_SOURCE_DIR',str(Path.home()/'.platformio/packages/framework-espidf/components/json/cJSON')));ssl=Path(os.environ.get('OPENSSL_PREFIX') or subprocess.check_output(['pkg-config','--variable=prefix','openssl'],text=True).strip())
with tempfile.TemporaryDirectory(prefix='terminal-guard-') as d:
 d=Path(d);subprocess.run([os.environ.get('CC','cc'),'-I'+str(cjson),'-c',str(cjson/'cJSON.c'),'-o',str(d/'json.o')],check=True)
 subprocess.run([os.environ.get('CXX','c++'),'-std=c++17','-Wall','-Wextra','-Werror','-pthread','-I'+str(i/'tests/storage_host'),'-I'+str(src/'main'),'-I'+str(cjson),'-I'+str(ssl/'include'),str(src/'main/durable_storage.cpp'),str(src/'main/profile_orchestrator.cpp'),str(h/'runtime.cpp'),str(d/'json.o'),'-L'+str(ssl/'lib'),'-lcrypto','-lsqlite3','-o',str(d/'test')],check=True)
 for mode in 'contradictory-committed resolve-event-collision concurrent claim-fault outcome-fault sending same-terminal other-terminal cash legacy legacy-unknown-route approved-complete approved-open resolve-approved resolve-declined resolve-wrong-terminal resolve-wrong-amount resolve-bad-evidence resolve-outbox-fault'.split():subprocess.run([str(d/'test'),mode,str(d/(mode+'.sqlite'))],check=True)
