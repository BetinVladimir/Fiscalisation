#!/usr/bin/env python3
"""Actual fiscal serial runtime: vendor-issued document identity proof."""
import os,pathlib,subprocess,tempfile
here=pathlib.Path(__file__).resolve().parent;idf=here.parents[1]
repo=pathlib.Path(os.environ.get("FISCAL_REPO_ROOT",str(here.parents[5])))
core=pathlib.Path(os.environ.get("PROTOCOL_SOURCE_ROOT",str(repo/"IoT/protocol-abstraction")))
cjson=pathlib.Path(os.environ.get("CJSON_SOURCE_DIR",str(pathlib.Path.home()/".platformio/packages/framework-espidf/components/json/cJSON")))
ssl=pathlib.Path(os.environ.get("OPENSSL_PREFIX","/opt/homebrew/opt/openssl@3"))
def run(args,**kw):subprocess.run([str(x) for x in args],check=True,**kw)
with tempfile.TemporaryDirectory(prefix="receipt-proof-") as name:
 d=pathlib.Path(name)
 run([os.environ.get("CC","cc"),"-I",cjson,"-c",cjson/"cJSON.c","-o",d/"json.o"])
 args=[os.environ.get("CXX","c++"),"-std=c++17","-Wall","-Wextra","-Werror"]
 if os.environ.get("SANITIZE"):args += ["-fsanitize=address,undefined","-fno-omit-frame-pointer"]
 for inc in [here,idf/"tests/sync_host",idf/"tests/storage_host",idf/"main",core/"include",cjson,ssl/"include"]:args += ["-I",inc]
 args += [idf/"main"/f for f in ["durable_storage.cpp","profile_executor.cpp","profile_orchestrator.cpp","serial_exchange.cpp"]]
 args += [core/"src"/f for f in ["FrameCodec.cpp","CommandPayload.cpp"]]
 args += [here/"runtime.cpp",d/"json.o","-L",ssl/"lib","-lcrypto","-lsqlite3","-o",d/"runtime"]
 run(args)
 for profile in ["datecs","daisy"]:
  for action in ["sale","reversal","recover-sale"]:
   for scenario in ["valid","bad-close","wrong-number","wrong-unp","substring","wrong-type","bad-date","wrong-serial"]:
    if action=="recover-sale" and scenario=="bad-close":continue
    run([d/"runtime",profile,action,scenario])
  run([d/"runtime",profile,"recover-reversal","valid"])
 run([d/"runtime","datecs","sale","line-limit"])
 for action in ["sale","reversal"]:run([d/"runtime","daisy",action,"not-ejt"])
 for scenario in ["wrong-link","wrong-fm"]:run([d/"runtime","daisy","reversal",scenario])
