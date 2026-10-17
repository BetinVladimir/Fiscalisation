#!/usr/bin/env python3
"""Compile actual standalone serializer; no firmware mocks or Android/tool downloads."""
import os, pathlib, subprocess, tempfile
here=pathlib.Path(__file__).resolve().parent
main=here.parents[1]/"main"
sdk=pathlib.Path(os.environ.get("IDF_PATH",str(pathlib.Path.home()/".platformio/packages/framework-espidf")))
inc=sdk/"components/json/cJSON"
openssl=pathlib.Path(os.environ.get("OPENSSL_PREFIX","/opt/homebrew/opt/openssl@3"))
with tempfile.TemporaryDirectory(prefix="go-json-host-") as d:
 d=pathlib.Path(d)
 def run(args,**kwargs): return subprocess.run([str(a) for a in args],check=True,**kwargs)
 run([os.environ.get("CC","cc"),"-c",inc/"cJSON.c","-I",inc,"-o",d/"cJSON.o"])
 common=["-std=c++17","-Wall","-Wextra","-Werror","-I",main,"-I",inc,"-I",openssl/"include"]
 run([os.environ.get("CXX","c++"),*common,here/"baseline.cpp",d/"cJSON.o","-o",d/"baseline"])
 red=subprocess.run([str(d/"baseline"),str(here/"golden.tsv")])
 if red.returncode!=1: raise SystemExit("baseline failed to demonstrate known canonical mismatch")
 run([os.environ.get("CXX","c++"),*common,main/"go_json.cpp",here/"test.cpp",d/"cJSON.o","-L",openssl/"lib","-lcrypto","-o",d/"test"])
 run([d/"test",here/"golden.tsv"])
