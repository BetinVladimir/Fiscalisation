#!/usr/bin/env python3
"""Independent Go P256 fixtures checked by actual production mbedTLS authorization."""
import os,pathlib,subprocess,tempfile,sys
cjson=pathlib.Path(os.environ.get("CJSON_SOURCE_DIR",str(pathlib.Path.home()/".platformio/packages/framework-espidf/components/json/cJSON")))
here=pathlib.Path(__file__).resolve().parent;idf=here.parents[1]
sys.path.insert(0,str(here.parent))
from mbedtls_host import crypto_dependency
def run(args,**kw):subprocess.run([str(v) for v in args],check=True,**kw)
with tempfile.TemporaryDirectory(prefix="execution-auth-host-") as name:
 d=pathlib.Path(name);run([os.environ.get("CC","cc"),"-I",cjson,"-c",cjson/"cJSON.c","-o",d/"cJSON.o"]);include,library=crypto_dependency(d)
 env=dict(os.environ);env.setdefault("GOCACHE",str(d/"go-cache"))
 run(["go","run",here/"oracle.go",d/"fixtures"],env=env)
 args=[os.environ.get("CXX","c++"),"-std=c++17","-Wall","-Wextra","-Werror"]
 if os.environ.get("SANITIZE"):args += ["-fsanitize=address,undefined","-fno-omit-frame-pointer"]
 for inc in [include,cjson,idf/"tests/storage_host",idf/"main"]:args += ["-I",inc]
 run(args+[idf/"main/execution_authorization.cpp",idf/"main/go_json.cpp",here/"runtime.cpp",d/"cJSON.o",library,"-o",d/"runtime"])
 run([d/"runtime",d/"fixtures"])
