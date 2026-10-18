#!/usr/bin/env python3
"""Actual SQLite authority and Go-issued P256 authorization through IntentProcessor."""
import os,pathlib,subprocess,tempfile,sys,json,base64
here=pathlib.Path(__file__).resolve().parent
idf=here.parents[1]
sys.path.insert(0,str(here.parent))
from mbedtls_host import crypto_dependency
cjson=pathlib.Path(os.environ.get("CJSON_SOURCE_DIR",str(pathlib.Path.home()/".platformio/packages/framework-espidf/components/json/cJSON")))
ssl=pathlib.Path(os.environ.get("OPENSSL_PREFIX","/opt/homebrew/opt/openssl@3"))
def run(args,**kw):subprocess.run([str(x) for x in args],check=True,**kw)
with tempfile.TemporaryDirectory(prefix="authority-host-") as name:
 d=pathlib.Path(name);include,library=crypto_dependency(d)
 run([os.environ.get("CC","cc"),"-I",cjson,"-c",cjson/"cJSON.c","-o",d/"cJSON.o"])
 common=[os.environ.get("CXX","c++"),"-std=c++17","-Wall","-Wextra","-Werror","-pthread"]
 if os.environ.get("SANITIZE"):common += ["-fsanitize=address,undefined","-fno-omit-frame-pointer"]
 for inc in [include,idf/"tests/storage_host",idf/"main",cjson,ssl/"include"]:common += ["-I",inc]
 link=[d/"cJSON.o",library,"-L",ssl/"lib","-lcrypto","-lsqlite3"]
 run(common+[idf/"main/durable_storage.cpp",here/"runtime.cpp"]+link+["-o",d/"storage"])
 for mode in ["format","configuration","boundary","atomic","policy","operators","assigned","concurrent-assigned"]:run([d/"storage",mode,d/(mode+".sqlite")])
 run(common+[idf/"main"/f for f in ["durable_storage.cpp","intent_processor.cpp","go_json.cpp","go_wire.cpp","execution_authorization.cpp","payment_reconciliation.cpp"]]+[here/"intent.cpp"]+link+["-o",d/"intent"])
 for fixture_name in ([os.environ["AUTHORITY_EXTERNAL_FIXTURE"]] if os.environ.get("AUTHORITY_EXTERNAL_FIXTURE") else ["issuer_fixture.json","online_issuer_fixture.json"]):
  fixture=json.loads((here/fixture_name).read_text());key=fixture["trust_key_der_base64"]
  (d/"trust.der").write_bytes(base64.urlsafe_b64decode(key+"="*((-len(key))%4)))
  run(["openssl","pkey","-pubin","-inform","DER","-in",d/"trust.der","-out",d/"trust.pem"])
  output=pathlib.Path(os.environ.get("AUTHORITY_FIXTURE_OUTPUT_DIR",str(d)));output.mkdir(parents=True,exist_ok=True)
  for mode in ["valid","missing","signature","binding","expired","payload"]:run([d/"intent",here/fixture_name,d/"trust.pem",d/(pathlib.Path(fixture_name).name+mode+".sqlite"),mode,output/(pathlib.Path(fixture_name).name+"-events.json")])
