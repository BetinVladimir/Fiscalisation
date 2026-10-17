#!/usr/bin/env python3
"""Actual Go Prepare REVERSAL -> IntentProcessor -> vendor serial -> Go materializer."""
import os,pathlib,subprocess,tempfile,shutil,sys
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parent.parent))
from mbedtls_host import crypto_dependency
here=pathlib.Path(__file__).resolve().parent
idf=here.parents[1]
repo=pathlib.Path(os.environ.get("FISCAL_REPO_ROOT",str(here.parents[5])))
cjson=pathlib.Path(os.environ.get("CJSON_SOURCE_DIR",str(pathlib.Path.home()/".platformio/packages/framework-espidf/components/json/cJSON")))
ssl=pathlib.Path(os.environ.get("OPENSSL_PREFIX","/opt/homebrew/opt/openssl@3"))
def run(args,**kw): subprocess.run([str(x) for x in args],check=True,**kw)
with tempfile.TemporaryDirectory(prefix="reversal-host-") as name:
 d=pathlib.Path(name);mbed_include,mbed_library=crypto_dependency(d);backend=d/"backend"
 backend.mkdir()
 for f in ["go.mod","go.sum"]:shutil.copy2(repo/"fiscal-backend"/f,backend/f)
 for sub in ["domain","config","mqttclient"]:shutil.copytree(repo/"fiscal-backend/internal"/sub,backend/"internal"/sub)
 (backend/"cmd/oracle").mkdir(parents=True);shutil.copy2(here/"oracle.go",backend/"cmd/oracle/main.go")
 env=dict(os.environ);env.setdefault("GOCACHE",str(d/"go-cache"))
 run(["go","build","-o",d/"oracle","./cmd/oracle"],cwd=backend,env=env)
 run([os.environ.get("CC","cc"),"-I",cjson,"-c",cjson/"cJSON.c","-o",d/"cJSON.o"])
 args=[os.environ.get("CXX","c++"),"-std=c++17","-Wall","-Wextra","-Werror"]
 if os.environ.get("SANITIZE"): args += ["-fsanitize=address,undefined","-fno-omit-frame-pointer"]
 for inc in [mbed_include,here,idf/"tests/sync_host",idf/"tests/storage_host",idf/"main",here.parents[5]/"IoT/protocol-abstraction/include",cjson,ssl/"include"]:args += ["-I",inc]
 args += [idf/"main"/f for f in ["durable_storage.cpp","go_json.cpp","go_wire.cpp","intent_processor.cpp","execution_authorization.cpp","canonical_cbor.cpp","profile_executor.cpp","profile_orchestrator.cpp","serial_exchange.cpp"]]
 args += [here.parents[5]/"IoT/protocol-abstraction/src"/f for f in ["FrameCodec.cpp","CommandPayload.cpp"]]
 args += [here/"runtime.cpp",d/"cJSON.o",mbed_library,"-L",ssl/"lib","-lcrypto","-lsqlite3","-o",d/"runtime"]
 run(args)
 for profile in ["datecs","daisy"]:
  for reason in ["OPERATOR_ERROR","CUSTOMER_RETURN","CUSTOMER_COMPLAINT","TAX_BASE_REDUCTION"]:
   label=profile+"-"+reason
   command=d/(label+"-command.json");batch=d/(label+"-batch.json")
   run([d/"oracle","generate",command,reason])
   run([d/"runtime",profile,d/(label+".sqlite"),command,batch,reason])
   run([d/"oracle","verify",batch])
 for profile in ["datecs","daisy"]:
  for mutation in ["reason-conflict","reason-code","unp-conflict","unp-null","doc-bounds"]:
   label=profile+"-"+mutation;command=d/(label+".json")
   run([d/"oracle","generate",command,"CUSTOMER_RETURN",mutation])
   run([d/"runtime",profile,d/(label+".sqlite"),command,d/"unused.json","negative"])
