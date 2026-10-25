#!/usr/bin/env python3
"""Run the complete MVP1 generation plan in an explicitly isolated workspace.

Read-only: --list, --validate. Execution: --execute --workspace-root /path/to/isolated/Beeloy.
The workspace must contain Fiscalisation and any required sibling repos POS/BeeloyBackend.
No automatic commits, resets, cleaning, external requests or HIL runs. Resume uses verified
unit receipts outside the repositories. Generated code requires independent review.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import time
import shutil
import signal
import tempfile

HERE=Path(__file__).resolve().parent
MODEL='ollama/qwen3.6:35b-a3b-coding-mtp-q4_K_M'

def digest(data):return hashlib.sha256(data.encode()).hexdigest()
def read_plan(path):
    p=json.loads(Path(path).read_text())
    if p.get('schema')!=2:raise ValueError('expected plan schema 2')
    return p

def ordered(plan,only=None):
    tasks={t['id']:t for t in plan['tasks']}
    selected=set(tasks) if not only else set()
    def include(tid):
        if tid not in tasks:raise ValueError('unknown dependency/task '+tid)
        if tid in selected:return
        selected.add(tid)
        for dep in tasks[tid]['depends']:include(dep)
    if only:include(only)
    result=[];active=set();done=set()
    def visit(tid):
        if tid in active:raise ValueError('dependency cycle at '+tid)
        if tid in done:return
        active.add(tid)
        for dep in tasks[tid]['depends']:
            if dep not in tasks:raise ValueError('unknown dependency '+dep)
            visit(dep)
        active.remove(tid);done.add(tid);result.append(tasks[tid])
    for tid in sorted(selected,key=lambda i:(tasks[i]['priority'],i)):visit(tid)
    return result

def pilot_task(plan,only,unit_id):
    if not only:raise ValueError('pilot-unit requires --only TASK')
    task=next((t for t in plan['tasks'] if t['id']==only),None)
    if task is None or task['mode']!='code' or task.get('alias_of'):raise ValueError('pilot requires a concrete code task')
    units=[u for u in task['units'] if u['id']==unit_id]
    if len(units)!=1:raise ValueError('pilot unit does not belong to task '+only)
    return dict(task,units=units)

def scope_path(root,value):
    root=root.resolve()
    if not value or value in ('.','..'):raise ValueError('empty/root-wide scope')
    if Path(value).is_absolute():raise ValueError('absolute scope '+value)
    p=(root/value).resolve()
    if not any(p==base or base in p.parents for base in (root,root.parent/'POS',root.parent/'BeeloyBackend')):
        raise ValueError('scope escapes isolated repositories: '+value)
    if any(x in p.parts for x in ('.git','node_modules','.store-build')):raise ValueError('forbidden scope '+value)
    return p

def prompt(plan,task,unit,root):
    sources=[]
    for source in unit.get('sources',[]):
        f=root/'docs/MVP1'/source
        if source in plan.get('source_documents',{}) or f.exists():
            # Only task section is embedded; decisions use the final owner section.
            txt=plan.get('source_documents',{}).get(source)
            if txt is None:txt=f.read_text()
            if source=='DECISIONS.md':txt=txt[txt.rfind('## 5. Решения владельца'):]
            elif task.get('spec_override'):txt=task.get('spec','')
            elif task['id'] in txt:
                match=re.search(r'^### '+re.escape(task['id'])+r'\b[^\n]*',txt,re.M)
                if match:
                    following=re.search(r'^#{1,3} ',txt[match.end():],re.M)
                    end=match.end()+following.start() if following else len(txt)
                    txt=txt[match.start():end]
                else:txt=task.get('spec','')
            else:txt=task.get('spec','')
            sources.append(txt)
    body='\n\n'.join([plan['policy'],plan['rules'],f"Задача {task['id']}; подзадача {unit['id']}; фаза {unit.get('phase','green')}",json.dumps(unit,ensure_ascii=False,indent=2),task.get('spec',''),task.get('notes','')]+sources)
    if len(body.encode())//2+1>plan['packet_estimate_limit']:
        # Do not truncate requirements: give exact source references instead of redundant copies.
        body='\n\n'.join([plan['policy'],plan['rules'],json.dumps(unit,ensure_ascii=False,indent=2),task.get('notes',''), 'Полная карточка: docs/MVP1/fixes/'+task['id']+'.md; читать только свою подзадачу и спецификацию по rg/sed.'])
    if len(body.encode())//2+1>plan['packet_estimate_limit']:raise ValueError('packet too large '+unit['id'])
    body=body+'\nРабочий корень Fiscalisation: '+str(root)+'\nРазрешённые sibling пути ../POS и ../BeeloyBackend только внутри изолированного workspace. read_files — только чтение контекста. В files путь файла разрешает только его, путь каталога — его потомков; соседние новые файлы запрещены. Отсутствующий нужный путь — BLOCKED_SCOPE. Не коммитить. Не изменять правила, plan.json, fixes, execution/status.json и сам runner. Для red-фазы создать тест и оставить ожидаемый assertion FAIL; green-фаза обязана выполнить содержательные тесты.'
    if len(body.encode())//2+1>plan['packet_estimate_limit']:raise ValueError('packet too large '+unit['id'])
    return body

def attempt_prompt(packet,error,limit):
    if not error:return packet
    prefix='\nПредыдущая ошибка (хвост):\n'
    error='\n'.join(error.splitlines()[-150:])
    room=max(0,2*(limit-1)-len(packet.encode())-len(prefix.encode()))
    if not room:return packet
    return packet+prefix+error.encode()[-room:].decode('utf-8',errors='ignore')

def validate(plan,root):
    errors=[];ids=set();uids=set()
    if not 0<plan.get('packet_estimate_limit',0)<=12000:errors.append('packet estimate limit must be 1..12000')
    if plan.get('context_tokens',65536)!=65536:errors.append('expected 65536 context budget')
    if 'coverage' in plan:
        required={(path,client,transport) for path in ['P-A','P-B','P-C','P-D','P-S'] for client in ['pos','minipos'] for transport in ['mqtt','ble']}
        routes=plan['coverage'].get('required_routes',[])
        actual={(route.get('path'),route.get('client'),route.get('transport')) for route in routes}
        if actual!=required or len(routes)!=len(required):errors.append('coverage requires exactly 20 client/transport/hardware routes')
        all_ids={t['id'] for t in plan['tasks']}
        for card,targets in plan.get('source_cards',{}).items():
            if not targets or not set(targets)<=all_ids:errors.append('uncovered source card '+card)
        declared=set(plan.get('source_cards',{}))
        expected_cards=set()
        for name,text in plan.get('source_documents',{}).items():
            if name.startswith('ARCH_'):
                expected_cards.update(('SCRIPT-'+value[4:] if value.startswith('SCR-') else value) for value in re.findall(r'^### ([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)\b',text,re.M))
            if name in ('ARCH_03_TESTS_E2E.md','ARCH_18_TEST_LOGIC_SPECS.md'):
                for test_id in re.findall(r'^\| (TST-[A-Z0-9]+-\d+) \|',text,re.M):
                    if test_id not in all_ids:errors.append('missing source test card '+test_id)
        for card in expected_cards-declared:errors.append('missing source-card mapping '+card)
        required_s3={f'TST-S3-{n:02d}' for n in range(1,7)}
        if not required_s3<=all_ids:errors.append('missing S3 test cards')
        for key in ['legacy_tasks','script_tasks','gap_scenarios']:
            if not set(plan['coverage'].get(key,[]))<=all_ids:errors.append('coverage contains missing task ids: '+key)
    try:ordered(plan)
    except ValueError as e:errors.append(str(e))
    for task in plan['tasks']:
        if not re.fullmatch(r'[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+',task['id']):errors.append('unsafe task id '+task['id'])
        if task['id'] in ids:errors.append('duplicate '+task['id'])
        ids.add(task['id'])
        if task.get('mode') not in ('code','external'):errors.append('invalid mode '+task['id'])
        if task.get('alias_of') and task['alias_of'] not in task['depends']+task.get('external_requires',[]):errors.append('alias missing canonical dependency '+task['id'])
        for dep in task.get('external_requires',[]):
            if dep not in {t['id'] for t in plan['tasks']}:errors.append('unknown external dependency '+dep)
        if task.get('alias_of') or task['mode']=='external':continue
        if not task['units']:errors.append('empty task '+task['id'])
        for unit in task['units']:
            if not unit['id'].startswith(task['id']+'.') or not re.fullmatch(r'[A-Z0-9-]+\.\d+(?:-through-\d+)?',unit['id']):errors.append('unsafe unit id '+unit['id'])
            if unit['id'] in uids:errors.append('duplicate unit '+unit['id'])
            uids.add(unit['id'])
            if not unit['files']:errors.append('empty scope '+unit['id'])
            if not unit.get('accept','').strip():errors.append('empty acceptance '+unit['id'])
            try:check_selector(unit.get('accept',''))
            except ValueError as e:errors.append(unit['id']+': '+str(e))
            if unit.get('red_required',True) is not True:errors.append('red evidence required '+unit['id'])
            if unit.get('phase')!='green':errors.append('unsupported phase '+unit['id'])
            if type(unit.get('repeat',1)) is not int or not 1<=unit.get('repeat',1)<=3:errors.append('invalid repeat '+unit['id'])
            if unit.get('regression'):errors.append('regression must use a trusted selector '+unit['id'])
            for f in unit['files']+unit.get('read_files',[])+unit.get('mutation_files',[]):
                if f.startswith('mqttclient/'):errors.append('incorrect module root '+unit['id']+': '+f)
                if f in unit['files']+unit.get('mutation_files',[]) and forbidden(f):errors.append('protected file in scope '+unit['id']+': '+f)
                try:scope_path(root,f)
                except ValueError as e:errors.append(str(e))
            try:prompt(plan,task,unit,root)
            except ValueError as e:errors.append(str(e))
    return errors

def run_process(argv,root,timeout,env=None,output_file=None):
    """Bound output memory and terminate descendants on timeout."""
    process_env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1',**(env or {}))
    # OpenCode uses PWD for project identity, even when Popen changed cwd.
    process_env['PWD']=str(Path(root).resolve());process_env.pop('OLDPWD',None)
    with tempfile.TemporaryFile(mode='w+b') as stream:
        process=subprocess.Popen(argv,cwd=root,stdout=stream,stderr=subprocess.STDOUT,start_new_session=True,env=process_env)
        try:rc=process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:

            try:os.killpg(process.pid,signal.SIGTERM)
            except ProcessLookupError:pass
            try:process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                try:os.killpg(process.pid,signal.SIGKILL)
                except ProcessLookupError:pass
                process.wait()
            rc=124
        leaked=False
        try:os.killpg(process.pid,0)
        except ProcessLookupError:pass
        else:
            leaked=True
            try:os.killpg(process.pid,signal.SIGTERM)
            except ProcessLookupError:pass
            # Reap descendants in the same session even when the direct process exited.
            import time as _time
            _time.sleep(0.05)
            try:os.killpg(process.pid,signal.SIGKILL)
            except ProcessLookupError:pass
        if leaked and rc==0:rc=125
        stream.seek(0,os.SEEK_END);length=stream.tell()
        if output_file is not None:
            # Preserve enough tool history for semantic review without unbounded logs.
            stream.seek(max(0,length-1048576));payload=stream.read()
            if length>1048576:payload=b'[TRUNCATED to last 1 MiB]\n'+payload
            file=Path(output_file);file.write_bytes(payload);file.chmod(0o600)
        stream.seek(max(0,length-12000))
        return rc,('LEAKED_CHILD_PROCESSES\n' if leaked else '')+stream.read().decode('utf-8',errors='replace')

def check_selector(command):
    parts=shlex.split(command)
    if len(parts)!=8 or parts[:4]!=['python3','-m','unittest','discover'] or parts[4]!='-s' or parts[6]!='-p':
        raise ValueError('accept must be a canonical unittest discovery selector, not a shell command')
    start,pattern=parts[5],parts[7]
    if not start.startswith('tests/') and start!='tests':raise ValueError('discovery must be inside tests/')
    if any(v in Path(start).parts for v in ('..','.git')) or not re.fullmatch(r'test_[A-Za-z0-9_.-]+\.py',pattern):raise ValueError('unsafe discovery selector')
    return start,pattern

def execute_check(command,root,timeout,phase='green',repeat=1,require_runtime=False,artifact_dir=None):
    if phase not in ('red','green') or type(repeat) is not int or repeat<1 or repeat>3:return False,'unsupported phase/repeat'
    try:start,pattern=check_selector(command)
    except ValueError as e:return False,str(e)
    if (Path(root)/start/pattern).is_symlink():return False,'symlink acceptance test is not allowed'
    if not (Path(root)/start/pattern).is_file():return False,'missing exact test file: '+start+'/'+pattern
    logs=[]
    for n in range(repeat):
        with tempfile.TemporaryDirectory(prefix='mvp-check-') as directory:
            report=Path(directory)/'result.json'
            argv=[sys.executable,'-I','-B',str(Path(__file__).resolve().parent/'trusted_check.py'),'--root',str(Path(root).resolve()),'--start',start,'--pattern',pattern,'--report',str(report)]
            artifacts=Path(artifact_dir)/str(n+1) if artifact_dir else Path(directory)/'artifacts'
            artifacts.mkdir(parents=True,exist_ok=True)
            try:rc,output=run_process(argv,root,timeout,env={'MVP1_ARTIFACT_DIR':str(artifacts.resolve())})
            except OSError as e:return False,str(e)
            if not report.exists():return False,'NO_TRUSTED_REPORT rc='+str(rc)+'\n'+output
            try:data=json.loads(report.read_text())
            except (ValueError,OSError) as e:return False,'invalid trusted report: '+str(e)
            valid=rc==0 and data.get('schema')==1 and data.get('successful') is True and type(data.get('tests')) is int and data['tests']>0 and data.get('assertions',0)>0 and not any(data.get(k,0) for k in ['errors','failures','skipped','expected_failures','unexpected_successes','constant_assertions','loader_errors'])
            if phase=='red':
                valid=rc==1 and data.get('schema')==1 and data.get('tests',0)>0 and data.get('assertions',0)>0 and data.get('failures',0)>0 and not any(data.get(k,0) for k in ['errors','skipped','expected_failures','unexpected_successes','constant_assertions','loader_errors'])
            if require_runtime and not (data.get('product_calls') or data.get('native_failures' if phase=='red' else 'native_runs') or phase=='red' and any(v.get('script') is True for v in data.get('native_runs',[]))):valid=False;output='NO_RUNTIME_EXERCISED\n'+output
            logs.append(f'RUN {n+1}/{repeat} '+json.dumps(data,ensure_ascii=False)+'\n'+output)
            if not valid:return False,'\n'.join(logs)
    return True,'\n'.join(logs)

def forbidden(path):
    return path=='AGENTS.md' or path.startswith('docs/MVP1/fixes/') or path in ('docs/MVP1/execution/status.json','docs/MVP1/execution/reverify.json','docs/MVP1/execution/routing.json')

def verify_unchanged(root,repos,before,files,protected,original_hashes):
    for repo in repos:
        after=inventory(repo);prev=before[str(repo)]
        for path in set(prev)|set(after):
            if prev.get(path)!=after.get(path) and (not in_scope(root,repo,path,files) or forbidden(path)):
                raise ValueError('outside scope; inspect manually, no reset: '+str(repo/path))
    for repo in protected:
        if inventory(repo)!=original_hashes[str(repo)]:raise ValueError('original repository modified: '+str(repo))

def bundle_hash(repos):
    return digest(json.dumps({repo.name:inventory(repo) for repo in repos},sort_keys=True))

def review_result(plan,queue,repos,state,review_path):
    expected={t['id'] for t in queue if t['mode']=='code' and not t.get('alias_of')}
    bundle={'schema':1,'plan_sha':digest(json.dumps(plan,sort_keys=True)),'workspace_sha':bundle_hash(repos),'tasks':sorted(expected)}
    (state/'review-request.json').write_text(json.dumps(bundle,indent=2))
    if not review_path:return False
    review=json.loads(review_path.read_text())
    return bool(expected) and review.get('schema')==1 and isinstance(review.get('reviewer'),str) and bool(review['reviewer'].strip()) and review.get('verdict')=='approve' and review.get('independent') is True and bool(review.get('reviewer')) and 'qwen' not in review['reviewer'].lower() and review.get('plan_sha')==bundle['plan_sha'] and review.get('workspace_sha')==bundle['workspace_sha'] and isinstance(review.get('tasks'),list) and all(isinstance(t,str) for t in review['tasks']) and len(review['tasks'])==len(expected) and set(review['tasks'])==expected

_FILE_DIGESTS={}

def inventory(repo):
    r=subprocess.run(['git','ls-files','-co','--exclude-standard','-z'],cwd=repo,capture_output=True,check=True)
    head=subprocess.run(['git','rev-parse','--verify','HEAD'],cwd=repo,capture_output=True).stdout
    index=subprocess.run(['git','ls-files','-s','-z'],cwd=repo,capture_output=True,check=True).stdout
    result={'@git-state':hashlib.sha256(head+b'\0'+index).hexdigest()}
    for raw in r.stdout.split(b'\0'):
        if not raw:continue
        path=os.fsdecode(raw);f=repo/path
        if f.is_symlink():result[path]=digest(os.readlink(f))
        elif f.is_file():
            st=f.stat();signature=(st.st_dev,st.st_ino,st.st_size,st.st_mtime_ns,st.st_ctime_ns,st.st_mode)
            key=str(f.resolve());cached=_FILE_DIGESTS.get(key)
            if cached is None or cached[0]!=signature:
                value=hashlib.sha256(f.read_bytes()).hexdigest()
                after=f.stat()
                if (after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns,after.st_mode)!=signature:raise ValueError('file changed during inventory: '+str(f))
                _FILE_DIGESTS[key]=(signature,value)
            result[path]='file:'+str(st.st_mode & 0o777)+':'+_FILE_DIGESTS[key][1]
    return result

def in_scope(root,repo,path,files):
    p=(repo/path).resolve()
    return any(p==scope_path(root,s) or scope_path(root,s) in p.parents for s in files)

def verify_red_evidence(state,uid):
    file=state/(uid+'.json')
    if not file.is_file():raise ValueError('missing receipt '+uid)
    record=json.loads(file.read_text());name=record.get('red_log','')
    if record.get('schema')!=3 or record.get('red_verified') is not True or not re.fullmatch(re.escape(uid)+r'\.[1-3]\.red\.evidence\.log',name) or not (state/name).is_file() or (state/name).is_symlink() or digest((state/name).read_text())!=record.get('red_sha'):
        raise ValueError('missing/changed red evidence '+uid)

def is_test_file(path):
    p=Path(path)
    return any(v in ('tests','__tests__','test','testdata','fixtures') for v in p.parts) or p.name.startswith('test_') or 'golden' in p.name.lower() or p.name.endswith(('_test.go','Test.kt','Tests.kt','Test.cpp','.test.ts','.test.tsx','.spec.ts'))

def prepare_unit(root,unit):
    """Create only declared fixed scaffolding; refuse to replace existing work."""
    root=Path(root)
    if unit.get('test_vectors_file'):
        file=scope_path(root,unit['test_vectors_file'])
        if file.is_symlink() or not file.is_file() or json.dumps(json.loads(file.read_text()),sort_keys=True)!=json.dumps(unit['test_vectors'],sort_keys=True):
            raise ValueError('missing/changed immutable vectors '+unit['id'])
    template=unit.get('harness_template')
    if template:
        compile(template,'<fixed harness>','exec')
        start,pattern=check_selector(unit['accept']);relative=str(Path(start)/pattern)
        if not in_scope(root,root,relative,unit['files']):raise ValueError('fixed harness outside scope')
        file=root/relative
        if file.is_symlink() or file.exists() and (not file.is_file() or file.read_text()!=template):raise ValueError('existing fixed harness differs; review/archive it: '+str(file))
        file.parent.mkdir(parents=True,exist_ok=True)
        if not file.exists():file.write_text(template)
    for relative,content in unit.get('scaffolds',{}).items():
        if forbidden(relative) or not in_scope(root,root,relative,unit['files']):raise ValueError('scaffold outside scope')
        file=root/relative
        if file.is_symlink() or file.exists() and not file.is_file():raise ValueError('invalid scaffold path')
        if not file.exists():
            file.parent.mkdir(parents=True,exist_ok=True);file.write_text(content);file.chmod(0o755)

def model_argv(args,message):
    return shlex.split(args.opencode_cmd)+['run']+getattr(args,'opencode_run_arg',[])+['--model',args.model,message]

def generate_unit(args,plan,task,unit,root,repos,protected,original_hashes,state,uid,attempt,prior_error=""):
    prepare_unit(root,unit)
    scopes=unit['files']+unit.get('mutation_files',[])
    before={str(repo):inventory(repo) for repo in repos}
    originals={}
    for repo in repos:
        for path in before[str(repo)]:
            if path!='@git-state' and not is_test_file(path) and in_scope(root,repo,path,scopes):
                file=repo/path
                if file.is_symlink():raise ValueError('red mutation of symlink not supported: '+str(file))
                originals[file]=(file.read_bytes(),file.stat().st_mode & 0o777)
    for stage in ('red','green'):
        header='[MVP_'+stage.upper()+'_STAGE] '+('Create reproducing tests; for existing correct code temporarily mutate only mutation_files/files. Controller restores non-test mutations before green. Leave an assertion failure, not an import/tool/compile error.' if stage=='red' else 'Implement the minimum fix or verify restored production code. Do not change any red-stage tests. Tests must pass.')+'\n'
        stage_packet=prompt(plan,task,dict(unit,phase=stage),root)
        footer=('\n[MVP_RED_STAGE_STOP] This invocation is RED ONLY. The overall AGENTS workflow is split across sessions. Create the correct test and run it to an assertion failure; STOP immediately after red. Do not implement the real fix or run green. Only a minimal executable stub/new API or a temporary defect mutation is allowed when necessary. Never infer phase from source or artifacts.' if stage=='red' else '\n[MVP_GREEN_STAGE_STOP] GREEN ONLY: implement production fix, keep every red test unchanged. If the oracle is wrong, report BLOCKED_BAD_ORACLE; do not change product behavior to satisfy a wrong test.')
        footer+='\nRun THIS acceptance, not only a native command: '+unit['accept']+'. The fixed harness, if declared, already exists; never modify it.'
        msg=header+attempt_prompt(stage_packet,prior_error,plan['packet_estimate_limit']-(len(header.encode())+len(footer.encode()))//2-2)+footer
        if len(msg.encode())//2+1>plan['packet_estimate_limit']:raise ValueError('stage packet too large '+uid)
        prior={str(repo):inventory(repo) for repo in repos}
        print('MODEL_START',uid,'stage='+stage,'attempt='+str(attempt),'cwd='+str(root),flush=True)
        started=time.monotonic()
        model_artifacts=state/'artifacts'/uid/'model'/stage/str(attempt)
        model_artifacts.mkdir(parents=True,exist_ok=True,mode=0o700)
        rc,output=run_process(model_argv(args,msg),root,args.timeout,env={'MVP1_ARTIFACT_DIR':str(model_artifacts.resolve())},output_file=state/(uid+f'.{attempt}.{stage}.log'))
        print('MODEL_EXIT',uid,'stage='+stage,'rc='+str(rc),'seconds='+str(round(time.monotonic()-started,1)),flush=True)
        verify_unchanged(root,repos,prior,scopes if stage=='red' else unit['files'],protected,original_hashes)
        if stage=='red':
            for repo in repos:
                now=inventory(repo)
                for path in set(prior[str(repo)])|set(now):
                    if path!='@git-state' and is_test_file(path) and prior[str(repo)].get(path)!=now.get(path) and not in_scope(root,repo,path,unit['files']):raise ValueError('read-only test context modified: '+path)
        if unit.get('harness_template'):
            start,pattern=check_selector(unit['accept']);file=root/start/pattern
            if not file.is_file() or file.read_text()!=unit['harness_template']:raise ValueError('fixed harness changed '+uid)
        if rc==124:raise ValueError('model timeout '+uid)
        check_before={str(repo):inventory(repo) for repo in repos}
        ok,evidence=execute_check(unit['accept'],root,args.check_timeout,phase=stage,repeat=1 if stage=='red' else unit.get('repeat',1),require_runtime=unit.get('require_runtime',False),artifact_dir=state/'artifacts'/uid/stage)
        verify_unchanged(root,repos,check_before,[],protected,original_hashes)
        (state/(uid+f'.{attempt}.{stage}.evidence.log')).write_text(evidence)
        if stage=='red':
            # Preserve the actual mutant privately for independent sensitivity review.
            mutations=state/'artifacts'/uid/'red'/'mutations'/str(attempt)
            mutations.mkdir(parents=True,exist_ok=True);mutations.chmod(0o700)
            changed=[]
            candidates=set(originals)
            candidates.update(repo/path for repo in repos for path in inventory(repo) if path!='@git-state' and not is_test_file(path) and in_scope(root,repo,path,scopes))
            for file in sorted(candidates):
                old=originals.get(file,(None,None))[0];new=file.read_bytes() if file.is_file() and not file.is_symlink() else None
                if old==new:continue
                item={'path':str(file)}
                for name,value in [('before',old),('mutant',new)]:
                    item[name+'_sha']=hashlib.sha256(value).hexdigest() if value is not None else None
                    if value is not None:
                        saved=mutations/(str(len(changed))+'.'+name);saved.write_bytes(value);saved.chmod(0o600);item[name+'_artifact']=str(saved)
                changed.append(item)
            (mutations/'manifest.json').write_text(json.dumps(changed,indent=2))
            # Restore only this invocation's scoped non-test mutations, never Git state.
            for repo in repos:
                for path in inventory(repo):
                    file=repo/path
                    if path!='@git-state' and not is_test_file(path) and in_scope(root,repo,path,scopes) and file not in originals:
                        file.unlink()
            for file,(content,mode) in originals.items():
                file.parent.mkdir(parents=True,exist_ok=True);file.write_bytes(content);file.chmod(mode)
            if not ok and changed and rc==0:
                # A small model may implement the fix during red despite instructions.
                # Discard its production proposal, then prove the SAME test fails on
                # the byte-exact baseline. Never accept a merely green model run.
                baseline_before={str(repo):inventory(repo) for repo in repos}
                baseline_ok,baseline_evidence=execute_check(unit['accept'],root,args.check_timeout,phase='red',require_runtime=unit.get('require_runtime',False),artifact_dir=state/'artifacts'/uid/'red'/'baseline')
                verify_unchanged(root,repos,baseline_before,[],protected,original_hashes)
                evidence+='\n[BASELINE_RED_REPLAY restored scoped non-test bytes]\n'+baseline_evidence
                ok=baseline_ok
                (state/(uid+f'.{attempt}.red.evidence.log')).write_text(evidence)
            for repo in repos:
                for path in inventory(repo):
                    if path!='@git-state' and is_test_file(path) and (repo/path).is_symlink():raise ValueError('cannot seal symlink test or fixture: '+path)
            sealed={str(repo/path):value for repo in repos for path,value in inventory(repo).items() if path!='@git-state' and is_test_file(path)}
        else:
            current={str(repo/path):value for repo in repos for path,value in inventory(repo).items() if path!='@git-state' and is_test_file(path)}
            if any(current.get(path)!=value for path,value in sealed.items()):raise ValueError('red tests changed during green '+uid)
        if rc!=0 or not ok:return False,stage.upper()+'_EVIDENCE_FAIL\n'+evidence
    return True,evidence

def main(argv=None):
    a=argparse.ArgumentParser(description=__doc__)
    a.add_argument('--plan',type=Path,default=HERE/'plan.json');a.add_argument('--model',default=os.environ.get('OPENCODE_MODEL',MODEL))
    a.add_argument('--list',action='store_true');a.add_argument('--validate',action='store_true');a.add_argument('--dry-run',action='store_true')
    a.add_argument('--create-workspace',action='store_true',help='create detached git worktrees, copy reviewed fixes packets; no commits')
    a.add_argument('--only');a.add_argument('--pilot-unit',help='execute one explicit unit without dependencies; always partial, never ACCEPTED');a.add_argument('--execute',action='store_true');a.add_argument('--workspace-root',type=Path)
    a.add_argument('--opencode-run-arg',action='append',default=[],choices=['--standalone','--auto','--format=json'],help='explicit run flag; use --opencode-run-arg=--standalone (repeatable)')
    a.add_argument('--opencode-cmd',default='opencode');a.add_argument('--attempts',type=int,default=3)
    a.add_argument('--timeout',type=int,default=1800);a.add_argument('--check-timeout',type=int,default=900)
    a.add_argument('--state-dir',type=Path);a.add_argument('--review',type=Path,help='independent review JSON matching final workspace digest');args=a.parse_args(argv)
    if args.attempts<1 or args.attempts>3 or args.timeout<=0 or args.check_timeout<=0:a.error('attempts must be 1..3 and timeouts positive')
    original=HERE.parents[2];plan=read_plan(args.plan);queue=ordered(plan,args.only)
    if args.pilot_unit:
        if args.review:a.error('pilot-unit cannot accept --review')
        queue=[pilot_task(plan,args.only,args.pilot_unit)]
        print('PILOT_PARTIAL',args.pilot_unit,'dependencies not executed:',','.join(queue[0]['depends']),flush=True)
    errors=validate(plan,original)
    if args.validate:
        for e in errors:print('FAIL',e)
        print(f"tasks={len(plan['tasks'])} errors={len(errors)}")
        return int(bool(errors))
    if args.list or args.dry_run:
        for t in queue:print(t['id'],t['priority'],t['mode'],'alias='+str(t.get('alias_of')),'deps='+','.join(t['depends']))
        print('tasks',len(queue),'validation_errors',len(errors));return int(bool(errors))
    if not args.execute or not args.workspace_root:a.error('execution requires --execute --workspace-root ISOLATED_PARENT')
    if errors:raise SystemExit('invalid plan; run --validate')
    root=(args.workspace_root/'Fiscalisation').resolve()
    if root==original.resolve() or original.resolve() in root.parents:raise SystemExit('refusing original project; provide an isolated checkout')
    if args.create_workspace:
        args.workspace_root.mkdir(parents=True,exist_ok=True)
        for name in ['Fiscalisation','POS','BeeloyBackend']:
            source=original if name=='Fiscalisation' else original.parent/name
            target=root if name=='Fiscalisation' else root.parent/name
            if target.exists():raise SystemExit('create-workspace refuses existing path '+str(target))
            if not (source/'.git').exists():raise SystemExit('missing source repository '+str(source))
            subprocess.run(['git','worktree','add','--detach',str(target),'HEAD'],cwd=source,check=True)
        shutil.copytree(HERE,root/'docs/MVP1/fixes',dirs_exist_ok=True,ignore=shutil.ignore_patterns('__pycache__','*.res.md'))
        for name,text in plan.get('source_documents',{}).items():
            if Path(name).name!=name:raise ValueError('unsafe source document name')
            (root/'docs/MVP1'/name).write_text(text)
    if not (root/'.git').exists():raise SystemExit('workspace Fiscalisation must be a git checkout/worktree (or use --create-workspace)')
    # Use the same reviewed packets and runner in the target checkout.
    if (root/'docs/MVP1/fixes/plan.json').read_bytes()!=args.plan.read_bytes():raise SystemExit('isolated plan differs; synchronize reviewed files first')
    repos=[root]
    for name in ['POS','BeeloyBackend']:
        needed=any(f.startswith('../'+name+'/') for t in queue for u in t['units'] for f in u['files']+u.get('read_files',[]))
        repo=root.parent/name
        if not repo.exists() and not needed:continue
        if not (repo/'.git').exists():raise ValueError('missing isolated sibling '+str(repo))
        if repo.resolve()==(original.parent/name).resolve():raise ValueError('sibling points at original '+name)
        repos.append(repo)
    state=(args.state_dir or args.workspace_root/'.mvp-fixes-state').resolve()
    if any(state==repo.resolve() or repo.resolve() in state.parents for repo in repos):raise SystemExit('state-dir must be outside repositories')
    state.mkdir(parents=True,exist_ok=True)
    import fcntl
    lock=(state/'lock').open('w')
    try:fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    except OSError:raise SystemExit('runner already active in this workspace')
    try:
        protected=[original]+[original.parent/n for n in ['POS','BeeloyBackend'] if (original.parent/n/'.git').exists()]
        original_hashes={str(r):inventory(r) for r in protected}
        finished=set();blocked=sorted({dep for task in queue for dep in task.get('external_requires',[])})
        for t in queue:
            if t['mode']=='external':blocked.append(t['id']);print('EXTERNAL',t['id']);continue
            if not args.pilot_unit and any(d not in finished for d in t['depends']):blocked.append(t['id']);print('BLOCKED dependencies',t['id']);continue
            if t.get('alias_of'):
                if t.get('external_requires'):blocked.append(t['id']);print('EXTERNAL_ALIAS',t['id']);continue
                finished.add(t['id']);continue
            for unit in t['units']:
                uid=unit['id'];packet=prompt(plan,t,unit,root);fingerprint=digest(packet)
                receipt=state/(uid+'.json')
                if receipt.exists():
                    old=json.loads(receipt.read_text())
                    if old.get('schema')!=3 or not old.get('red_verified') or old.get('packet_sha')!=fingerprint:raise ValueError('stale/changed receipt '+uid+'; archive receipt and regenerate/review')
                    verify_red_evidence(state,uid)
                    before={str(repo):inventory(repo) for repo in repos}
                    ok,output=execute_check(unit['accept'],root,args.check_timeout,repeat=unit.get('repeat',1),require_runtime=unit.get('require_runtime',False),artifact_dir=state/'artifacts'/uid)
                    verify_unchanged(root,repos,before,[],protected,original_hashes)
                    if ok:continue
                for attempt in range(1,args.attempts+1):
                    ok,output=generate_unit(args,plan,t,unit,root,repos,protected,original_hashes,state,uid,attempt,output if attempt>1 else '')
                    if ok:
                        receipt.write_text(json.dumps({'schema':3,'packet_sha':fingerprint,'red_verified':True,'red_log':uid+f'.{attempt}.red.evidence.log','red_sha':digest((state/(uid+f'.{attempt}.red.evidence.log')).read_text()),'model':args.model,'time':time.time(),'status':'TESTED_REVIEW_REQUIRED','output':output},ensure_ascii=False,indent=2));print('TESTED',uid);break
                    print('FAIL',uid,output[-2000:])
                else:return 1
            # A task is complete only when all previously red tests now pass.
            for u in t['units']:
                before={str(repo):inventory(repo) for repo in repos}
                ok,output=execute_check(u['accept'],root,args.check_timeout,repeat=u.get('repeat',1),require_runtime=u.get('require_runtime',False),artifact_dir=state/'artifacts'/u['id'])
                verify_unchanged(root,repos,before,[],protected,original_hashes)
                if not ok:print('FINAL FAIL',u['id'],output);return 1
            finished.add(t['id'])
        # Later implementation units can invalidate earlier green units. Recheck the
        # complete selected code closure before issuing a review request/acceptance.
        code_tasks=[task for task in queue if task['mode']=='code' and not task.get('alias_of')]
        for task in code_tasks:
            for unit in task['units']:verify_red_evidence(state,unit['id'])
        if len(code_tasks)>1:
            for task in code_tasks:
                for unit in task['units']:
                    before={str(repo):inventory(repo) for repo in repos}
                    ok,output=execute_check(unit['accept'],root,args.check_timeout,repeat=unit.get('repeat',1),require_runtime=unit.get('require_runtime',False),artifact_dir=state/'artifacts'/unit['id'])
                    verify_unchanged(root,repos,before,[],protected,original_hashes)
                    if not ok:print('FINAL_CLOSURE_FAIL',unit['id'],output);return 1
        if args.pilot_unit:
            report={'schema':1,'status':'PILOT_TESTED_REVIEW_REQUIRED','task':args.only,'units':[args.pilot_unit],'omitted_dependencies':queue[0]['depends'],'model':args.model,'plan_sha':digest(json.dumps(plan,sort_keys=True)),'workspace_sha':bundle_hash(repos)}
            (state/'pilot-report.json').write_text(json.dumps(report,indent=2))
            (state/(args.pilot_unit+'.pilot.json')).write_text(json.dumps(report,indent=2))
            print('PILOT_TESTED_REVIEW_REQUIRED',args.pilot_unit,flush=True)
            return 4
        approved=review_result(plan,queue,repos,state,args.review)
        print('ACCEPTED' if approved else 'TESTED_REVIEW_REQUIRED','tasks',len(finished),'external/blocked',','.join(blocked))
        if not approved:return 4
        return 3 if blocked else 0
    finally:
        lock.close()

if __name__=='__main__':
    try:sys.exit(main())
    except (ValueError,OSError) as e:print('ERROR',e,file=sys.stderr);sys.exit(2)
