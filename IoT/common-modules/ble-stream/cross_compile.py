#!/usr/bin/env python3
"""Compile-only portability check; no native app runtime/device access."""
from pathlib import Path
import argparse,subprocess,tempfile,json,hashlib
p=argparse.ArgumentParser();p.add_argument('--ndk',type=Path,required=True);p.add_argument('--log',type=Path,required=True);a=p.parse_args();src=Path(__file__).resolve().parent
with tempfile.TemporaryDirectory(prefix='mp026-cross-') as t,a.log.open('w') as log:
 out=Path(t);commands=[
 [str(a.ndk/'toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android24-clang'),'-std=c11','-Wall','-Wextra','-Werror','-c',str(src/'stream.c'),'-o',str(out/'android.o')],
 ['xcrun','--sdk','iphoneos','clang','-target','arm64-apple-ios15.1','-std=c11','-Wall','-Wextra','-Werror','-c',str(src/'stream.c'),'-o',str(out/'ios.o')],
 ['xcrun','--sdk','iphonesimulator','clang','-target','arm64-apple-ios15.1-simulator','-std=c11','-Wall','-Wextra','-Werror','-c',str(src/'stream.c'),'-o',str(out/'ios-sim.o')]]
 log.write('SOURCE_SHA256 '+hashlib.sha256((src/'stream.c').read_bytes()).hexdigest()+'\n')
 for command in commands:
  log.write('COMMAND '+json.dumps(command)+'\n');log.flush();subprocess.run(command,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=60);log.write('PASS compile-only\n')
