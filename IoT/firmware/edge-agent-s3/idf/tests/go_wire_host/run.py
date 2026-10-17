#!/usr/bin/env python3
"""Actual Go backend <-> actual firmware SQLite, sync and IntentProcessor."""
import os,pathlib,subprocess,tempfile,shutil
here=pathlib.Path(__file__).resolve().parent
idf=here.parents[1]
repo=pathlib.Path(os.environ.get("FISCAL_REPO_ROOT",str(here.parents[5])))
cjson=pathlib.Path(os.environ.get("CJSON_SOURCE_DIR",str(pathlib.Path.home()/".platformio/packages/framework-espidf/components/json/cJSON")))
ssl=pathlib.Path(os.environ.get("OPENSSL_PREFIX","/opt/homebrew/opt/openssl@3"))
def run(args,**kw): subprocess.run([str(x) for x in args],check=True,**kw)
with tempfile.TemporaryDirectory(prefix="go-wire-host-") as name:
 d=pathlib.Path(name);backend=d/"backend"
 backend.mkdir()
 for f in ["go.mod","go.sum"]:shutil.copy2(repo/"fiscal-backend"/f,backend/f)
 for sub in ["domain","config","mqttclient"]:shutil.copytree(repo/"fiscal-backend/internal"/sub,backend/"internal"/sub)
 (backend/"cmd/oracle").mkdir(parents=True);shutil.copy2(here/"oracle.go",backend/"cmd/oracle/main.go")
 env=dict(os.environ);env.setdefault("GOCACHE",str(d/"go-cache"))
 run(["go","build","-o",d/"oracle","./cmd/oracle"],cwd=backend,env=env)
 run([os.environ.get("CC","cc"),"-I",cjson,"-c",cjson/"cJSON.c","-o",d/"cJSON.o"])
 args=[os.environ.get("CXX","c++"),"-std=c++17","-Wall","-Wextra","-Werror"]
 if os.environ.get("SANITIZE"): args += ["-fsanitize=address,undefined","-fno-omit-frame-pointer"]
 for inc in [idf/"tests/sync_host",idf/"tests/storage_host",idf/"main",cjson,ssl/"include"]:args += ["-I",inc]
 args += [idf/"main"/f for f in ["durable_storage.cpp","go_json.cpp","go_wire.cpp","intent_processor.cpp","canonical_cbor.cpp"]]
 args += [here/"runtime.cpp",d/"cJSON.o","-L",ssl/"lib","-lcrypto","-lsqlite3","-o",d/"runtime"]
 run(args)
 run([d/"oracle","command",d/"command.json"]);run([d/"runtime","command",d/"command.sqlite",d/"command.json"])
 for suffix,seq in [("normal",None),("wide","9223372036854775806")]:
  db=d/(suffix+".sqlite");body=d/(suffix+".json");ack=d/(suffix+"-ack.json")
  run([d/"runtime","batch",db,body]+([seq] if seq else []));run([d/"oracle","ack",body,ack]);run([d/"runtime","ack",db,ack])
 run([d/"runtime","batch",d/"legacy.sqlite",d/"legacy.json"]);run([d/"runtime","migration",d/"legacy.sqlite",d/"legacy.json"])
