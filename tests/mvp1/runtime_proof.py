"""Observe executed Python product functions and captured native test results.

This rejects incidental processes/imports, not malicious test implementations.
Independent semantic review remains mandatory.
"""
from functools import lru_cache
import json
import hashlib
import shutil
import re
import subprocess
import socket
import sys
import os
import time
import xml.etree.ElementTree as ET
from pathlib import Path

def test_count(output):
    if isinstance(output,bytes):output=output.decode('utf-8',errors='replace')
    if not isinstance(output,str):return 0
    go=set();structured=False
    for line in output.splitlines():
        try:event=json.loads(line)
        except ValueError:continue
        if not isinstance(event,dict) or not event.get('Package') or event.get('Action') not in ('run','start','output','pass','fail','skip','pause','cont'):continue
        structured=True
        if event.get('Action') in ('fail','skip'):return 0
        if event.get('Action')=='pass' and event.get('Test'):go.add((event.get('Package'),event['Test']))
    if structured:return len(go)
    if re.search(r'(?:[1-9]\d* (?:skipped|todo|pending|disabled)|skipped=[1-9]|#\s*(?:SKIP|TODO)|\bxfail|\bxpass|FAILED \(|\[\s*FAILED\s*\]\s*[1-9]|\b[1-9]\d* (?:tests? )?failed\b)',output,re.I):return 0
    for pattern in [r'Ran (\d+) tests?\b',r'\[\s*PASSED\s*\]\s+(\d+) tests?',r'Tests:\s+(?:\d+ skipped,\s*)?(\d+) passed',r'\b(\d+) passed(?:,| in |$)',r'100% tests passed, 0 tests failed out of (\d+)']:
        found=re.search(pattern,output)
        if found:return int(found[1])
    tap=re.search(r'^1\.\.(\d+)\s*$',output,re.M)
    if tap and not re.search(r'^not ok\b|#\s*(?:SKIP|TODO)\b',output,re.M|re.I) and [int(v) for v in re.findall(r'^ok\s+(\d+)(?:\s|$)',output,re.M)] == list(range(1,int(tap[1])+1)):return int(tap[1])
    return 0

def failed_test_count(output):
    if isinstance(output,bytes):output=output.decode('utf-8',errors='replace')
    if not isinstance(output,str) or re.search(r'ImportError|ModuleNotFoundError|SyntaxError|FAILED \(errors=|\bERROR:',output):return 0
    events=[]
    for line in output.splitlines():
        try:event=json.loads(line)
        except ValueError:continue
        if isinstance(event,dict) and event.get('Action')=='fail' and event.get('Test'):events.append(event['Test'])
    if events:return len(set(events))
    for pattern in [r'FAILED \(failures=(\d+)\)',r'\b(\d+) failed(?:,| in |$)',r'\[\s*FAILED\s*\]\s+(\d+) tests?']:
        found=re.search(pattern,output)
        if found:return int(found[1])
    return 0

def junit_count(cwd,started,failed=False):
    total=0
    for base in [Path(cwd),Path(cwd)/'SmartDevices']:
        for pattern in ('build/test-results/**/TEST-*.xml','*/build/test-results/**/TEST-*.xml'):
            for file in base.glob(pattern):
                if file.stat().st_mtime_ns<started:continue
                try:node=ET.parse(file).getroot()
                except (ET.ParseError,OSError):continue
                if any(e.tag in ('error','skipped') or e.tag in ('testsuite','testsuites') and any(int(e.get(k,'0'))>0 for k in ('errors','skipped')) for e in node.iter()):continue
                cases=[e for e in node.iter() if e.tag=='testcase']
                bad=[e for e in cases if e.find('failure') is not None]
                if failed:total+=len(bad)
                elif not bad and not any(int(e.get('failures','0')) for e in node.iter() if e.tag in ('testsuite','testsuites')):total+=len(cases)
                destination=os.environ.get('MVP1_ARTIFACT_DIR')
                if destination:
                    archive=Path(destination)/'native-junit';archive.mkdir(parents=True,exist_ok=True)
                    shutil.copy2(file,archive/(hashlib.sha256(str(file).encode()).hexdigest()[:12]+'-'+file.name))
    return total

def install(root,report):
    root=Path(root).resolve();bases=[root,root.parent/'POS',root.parent/'BeeloyBackend']
    @lru_cache(maxsize=4096)
    def product(file):
        try:
            if str(file).startswith('<'):return False
            p=Path(file).resolve()
            if not p.is_file():return False
        except (TypeError,ValueError):return False
        for base in bases:
            if base in p.parents:
                rel=p.relative_to(base)
                return not any(v in ('tests','__tests__','test','testdata','fixtures','__pycache__') for v in rel.parts) and not p.name.startswith('test_') and p.name not in ('conftest.py',) and 'golden' not in p.name.lower() and not p.name.endswith(('_test.go','Test.kt','Tests.kt','Test.cpp','.test.ts','.test.tsx','.spec.ts')) and not str(rel).startswith('docs/MVP1/fixes/')
        return False
    report['product_calls']=[];report['native_runs']=[];report['native_failures']=[];report['runtime_io']=[]
    processes=[];compiled=set()
    previous=sys.getprofile()
    def profile(frame,event,arg):
        if event=='call' and frame.f_code.co_name!='<module>' and product(frame.f_code.co_filename):
            value=frame.f_code.co_filename+':'+frame.f_code.co_name
            if value not in report['product_calls']:report['product_calls'].append(value)
        if previous:previous(frame,event,arg)
    sys.setprofile(profile)
    communicate=subprocess.Popen.communicate
    original_init=subprocess.Popen.__init__
    def initialized(process,*a,**kw):
        process._mvp_started=time.time_ns();process._mvp_cwd=kw.get('cwd') or os.getcwd()
        original_init(process,*a,**kw)
        command=process.args
        if isinstance(command,(list,tuple)) and command:
            command=[str(v) for v in command];exe=Path(command[0]).name
            candidate=False
            for value in command[1:]:
                if value.startswith('-'):continue
                path=Path(value);path=path if path.is_absolute() else Path(process._mvp_cwd)/path
                if path.is_file() and product(path):candidate=True
            candidate=candidate or exe=='go' and len(command)>2 and command[1]=='run' and (Path(process._mvp_cwd)/'go.mod').is_file()
            if candidate and (exe.startswith(('python','node','java')) or exe=='go' or product(command[0])):processes.append(process)
    subprocess.Popen.__init__=initialized
    connect=socket.socket.connect
    def observed_connect(sock,address):
        value=connect(sock,address)
        if isinstance(address,tuple) and address[0] in ('127.0.0.1','::1','localhost'):
            for process in processes:
                if process.poll() is None:report['runtime_io'].append({'argv':list(process.args),'address':list(address)})
        return value
    socket.socket.connect=observed_connect
    def observed(process,*a,**kw):
        out,err=communicate(process,*a,**kw)
        destination=os.environ.get('MVP1_ARTIFACT_DIR')
        if destination:
            archive=Path(destination)/'native-output';archive.mkdir(parents=True,exist_ok=True)
            number=len(list(archive.glob('*.stdout.log')))+1
            for label,value in [('stdout',out),('stderr',err)]:
                if value is not None:(archive/(str(number)+'.'+label+'.log')).write_bytes(value if isinstance(value,bytes) else value.encode())
        argv=process.args
        if isinstance(argv,(list,tuple)) and argv:
            command=[str(v) for v in argv];exe=Path(command[0]).name
            count=test_count(out)+test_count(err)
            if exe in ('g++','c++','clang++','gcc','clang') and process.returncode==0 and '-o' in command:
                sources=[Path(v) if Path(v).is_absolute() else Path(process._mvp_cwd)/v for v in command[1:] if v.endswith(('.cpp','.cc','.c'))]
                if any(product(v) for v in sources):
                    output=Path(command[command.index('-o')+1]);compiled.add((output if output.is_absolute() else Path(process._mvp_cwd)/output).resolve())
            failures=failed_test_count(out)+failed_test_count(err)
            native=(exe=='go' and len(command)>1 and command[1]=='test') or ('gradlew' in exe and any('test' in v.lower() for v in command[1:])) or (exe in ('node','npx','npm','ctest') and any(v in ' '.join(command) for v in ('jest','vitest','test','ctest')))
            if 'gradlew' in exe:
                count=junit_count(process._mvp_cwd,process._mvp_started)
                failures=junit_count(process._mvp_cwd,process._mvp_started,failed=True) if process.returncode else 0
            script=False;known_wrapper=False
            if exe.startswith(('python','bash','sh','ruby')) and len(command)>1 and not command[1].startswith('-'):
                path=Path(command[1]);path=path if path.is_absolute() else Path(process._mvp_cwd)/path
                script=path.is_file() and product(path)
                known_wrapper=False
                try:
                    relative=path.resolve().relative_to(root)
                    original=Path(__file__).resolve().parents[3]/relative
                    known_wrapper=exe in ('bash','sh') and path.name.startswith('run_') and str(relative).startswith('IoT/') and original.is_file() and hashlib.sha256(path.read_bytes()).digest()==hashlib.sha256(original.read_bytes()).digest()
                except (ValueError,OSError):pass
                native=(script or known_wrapper) and (count>0 or failures>0)
            combined=''.join(v.decode('utf-8',errors='replace') if isinstance(v,bytes) else (v or '') for v in (out,err))
            assertion=bool(re.search(r"Assertion failed:|Assertion ['\"].+failed",combined,re.I)) and not re.search(r'static_assert|static assertion|error:',combined,re.I)
            if process.returncode!=0 and assertion and (known_wrapper or Path(command[0]).resolve() in compiled):
                failures=max(failures,1);native=True
            if count>0 and (native or Path(command[0]).resolve() in compiled or exe not in ('echo','printf','cat','true','false') and Path(command[0]).is_absolute() and Path(command[0]).is_file() and not exe.startswith(('python','bash','sh','ruby','node')) and any(base in Path(command[0]).resolve().parents for base in bases)):
                if process.returncode==0:report['native_runs'].append({'argv':command,'tests':count})
            if process.returncode!=0 and failures>0 and native:report['native_failures'].append({'argv':command,'tests':failures})
            elif script and process.returncode in (0,1) and not (count>0 and native and process.returncode==0):
                combined=(out or b'') if isinstance(out,bytes) else (out or '')
                if isinstance(combined,bytes):combined=combined.decode('utf-8',errors='replace')
                extra=(err or b'') if isinstance(err,bytes) else (err or '')
                if isinstance(extra,bytes):extra=extra.decode('utf-8',errors='replace')
                suite=bool(re.search(r'Ran \d+ tests?|\"Action\"|^1\.\.\d+|\[\s*(?:PASSED|FAILED)\s*\]|Tests:|tests passed,',combined+extra,re.M))
                if (process.returncode==0 or re.search(r'AssertionError|Assertion failed:',combined+extra)) and not suite and not re.search(r'SyntaxError|ImportError|ModuleNotFoundError|command not found|syntax error|build failed',combined+extra,re.I):
                    report['native_runs' if process.returncode==0 else 'native_failures'].append({'argv':command,'script':True})
        return out,err
    subprocess.Popen.communicate=observed
    def cleanup():
        sys.setprofile(previous);subprocess.Popen.communicate=communicate;subprocess.Popen.__init__=original_init;socket.socket.connect=connect
    return cleanup
