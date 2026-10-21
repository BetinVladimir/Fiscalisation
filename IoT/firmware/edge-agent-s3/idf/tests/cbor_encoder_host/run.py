#!/usr/bin/env python3
"""Independent hexadecimal oracles for the actual typed CBOR encoder."""
from pathlib import Path
import os, shlex, subprocess, tempfile
HERE=Path(__file__).resolve().parent; IDF=HERE.parents[1]
def run(args):subprocess.run([str(x) for x in args],check=True,timeout=180)
cjson=Path(os.environ.get('CJSON_SOURCE_DIR',os.environ.get('CJSON_SOURCE',str(Path.home()/'.platformio/packages/framework-espidf/components/json/cJSON'))))
mbed=Path(os.environ.get('MBEDTLS_SOURCE_DIR',str(Path.home()/'.platformio/packages/framework-espidf/components/mbedtls/mbedtls')))
for p in [cjson/'cJSON.c',cjson/'cJSON.h',mbed/'include/mbedtls/pk.h']:
 if not p.is_file():raise RuntimeError(f'actual host dependency missing: {p}')
cc=shlex.split(os.environ.get('CC','cc'));cxx=shlex.split(os.environ.get('CXX','c++'))
flags=['-fsanitize=address,undefined','-fno-omit-frame-pointer'] if os.environ.get('SANITIZE') else []
with tempfile.TemporaryDirectory(prefix='beefiscal-cbor-encoder-') as temporary:
 d=Path(temporary);library=os.environ.get('MBEDTLS_CRYPTO_LIBRARY')
 if not library:
  run(['cmake','-S',mbed,'-B',d/'mbedtls','-DENABLE_PROGRAMS=OFF','-DENABLE_TESTING=OFF','-DUSE_SHARED_MBEDTLS_LIBRARY=OFF'])
  run(['cmake','--build',d/'mbedtls','--target','mbedcrypto','-j',str(min(os.cpu_count() or 2,8))]);library=d/'mbedtls/library/libmbedcrypto.a'
 if not Path(library).is_file():raise RuntimeError('actual mbedTLS crypto library missing')
 run(cc+flags+['-I',cjson,'-c',cjson/'cJSON.c','-o',d/'json.o'])
 command=cxx+['-std=c++17','-Wall','-Wextra','-Werror']+flags
 for p in [mbed/'include',IDF/'tests/storage_host',IDF/'main',cjson]:command+=['-I',p]
 run(command+[IDF/'main'/n for n in ['canonical_cbor.cpp','go_json.cpp','go_wire.cpp']]+[HERE/'vectors.cpp',d/'json.o',library,'-o',d/'vectors'])
 run([d/'vectors'])
