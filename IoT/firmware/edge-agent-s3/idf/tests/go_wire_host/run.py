#!/usr/bin/env python3
"""Actual Go backend <-> actual firmware SQLite, sync and IntentProcessor."""
import os,pathlib,subprocess,tempfile,shutil,sys,json,base64
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parent.parent))
from mbedtls_host import crypto_dependency
here=pathlib.Path(__file__).resolve().parent
idf=here.parents[1]
repo=pathlib.Path(os.environ.get("FISCAL_REPO_ROOT",str(here.parents[5])))
cjson=pathlib.Path(os.environ.get("CJSON_SOURCE_DIR",str(pathlib.Path.home()/".platformio/packages/framework-espidf/components/json/cJSON")))
ssl=pathlib.Path(os.environ.get("OPENSSL_PREFIX","/opt/homebrew/opt/openssl@3"))
def run(args,**kw): subprocess.run([str(x) for x in args],check=True,**kw)
with tempfile.TemporaryDirectory(prefix="go-wire-host-") as name:
 d=pathlib.Path(name);mbed_include,mbed_library=crypto_dependency(d);backend=d/"backend"
 backend.mkdir()
 for f in ["go.mod","go.sum"]:shutil.copy2(repo/"fiscal-backend"/f,backend/f)
 for sub in ["domain","config","mqttclient","provisioningseal"]:shutil.copytree(repo/"fiscal-backend/internal"/sub,backend/"internal"/sub)
 (backend/"cmd/oracle").mkdir(parents=True);shutil.copy2(here/"oracle.go",backend/"cmd/oracle/main.go")
 env=dict(os.environ);env.setdefault("GOCACHE",str(d/"go-cache"))
 run(["go","build","-o",d/"oracle","./cmd/oracle"],cwd=backend,env=env)
 run([os.environ.get("CC","cc"),"-I",cjson,"-c",cjson/"cJSON.c","-o",d/"cJSON.o"])
 args=[os.environ.get("CXX","c++"),"-std=c++17","-Wall","-Wextra","-Werror"]
 if os.environ.get("SANITIZE"): args += ["-fsanitize=address,undefined","-fno-omit-frame-pointer"]
 for inc in [mbed_include,idf/"tests/sync_host",idf/"tests/storage_host",idf/"main",cjson,ssl/"include"]:args += ["-I",inc]
 args += [idf/"main"/f for f in ["durable_storage.cpp","go_json.cpp","go_wire.cpp","intent_processor.cpp","execution_authorization.cpp","canonical_cbor.cpp"]]
 args += [here/"runtime.cpp",d/"cJSON.o",mbed_library,"-L",ssl/"lib","-lcrypto","-lsqlite3","-o",d/"runtime"]
 run(args)
 run([d/"oracle","command",d/"command.json"]);run([d/"runtime","command",d/"command.sqlite",d/"command.json"])
 # The same actual backend-prepared online command also traverses the real
 # IntentProcessor and exports accepted/result evidence for consumer replay.
 output=pathlib.Path(os.environ.get("AUTHORITY_FIXTURE_OUTPUT_DIR",str(d)));output.mkdir(parents=True,exist_ok=True)
 fixture=output/"mqtt-online-fixture.json"
 run([d/"oracle","fixture-command",idf/"tests/authority_host/online_issuer_fixture.json",fixture])
 source=json.loads(fixture.read_text());key=source["trust_key_der_base64"]
 (d/"trust.der").write_bytes(base64.urlsafe_b64decode(key+"="*((-len(key))%4)))
 run(["openssl","pkey","-pubin","-inform","DER","-in",d/"trust.der","-out",d/"trust.pem"])
 actual=[v for v in args if str(v) not in {str(idf/"main/canonical_cbor.cpp"),str(here/"runtime.cpp")}]
 actual[-1]=d/"authority-runtime"
 run(actual+[idf/"tests/authority_host/intent.cpp"])
 run([d/"authority-runtime",fixture,d/"trust.pem",d/"mqtt-online.sqlite","mqtt-valid",output/"mqtt-online-events.json"])
 for suffix,seq in [("normal",None),("wide","9223372036854775806")]:
  db=d/(suffix+".sqlite");body=d/(suffix+".json");ack=d/(suffix+"-ack.json")
  run([d/"runtime","batch",db,body]+([seq] if seq else []));run([d/"oracle","ack",body,ack]);run([d/"runtime","ack",db,ack])
 run([d/"runtime","batch",d/"legacy.sqlite",d/"legacy.json"]);run([d/"runtime","migration",d/"legacy.sqlite",d/"legacy.json"])
