#!/usr/bin/env python3
"""PIPE-01: запускает accept-команды подзадач одной задачи и печатает вердикт (код продукта не меняет).
  python3 docs/MVP1/tools/run-accept.py TASK-ID [--subtask TASK-ID.N] [--timeout 600] [--record]
Команды выполняются из корня Fiscalisation. exit 0 только если все выбранные accept вернули 0.
--record пишет execution/runs/<ID>.json {subtask, cmd, rc, seconds, tail(40 строк), at}. Статус в status.json НЕ меняется."""
import json,os,subprocess,sys,time,datetime
H=os.path.dirname(os.path.abspath(__file__));M=os.path.dirname(H);ROOT=os.path.abspath(os.path.join(M,'..','..'))
a=sys.argv[1:]
if not a: print(__doc__);sys.exit(2)
tid=a[0];only=a[a.index('--subtask')+1] if '--subtask' in a else None
to=int(a[a.index('--timeout')+1]) if '--timeout' in a else 600
S=json.load(open(os.path.join(M,'execution','status.json')));T=S['tasks'];T=T if isinstance(T,list) else list(T.values())
t=[x for x in T if x['id']==tid]
if not t: print('нет задачи',tid);sys.exit(2)
res=[];bad=0
for st in t[0].get('subtasks',[]):
    if only and st['id']!=only: continue
    cmd=st.get('accept','');t0=time.time()
    try:
        p=subprocess.run(['bash','-c',cmd],cwd=ROOT,capture_output=True,text=True,timeout=to);rc=p.returncode;out=(p.stdout+p.stderr)
    except subprocess.TimeoutExpired: rc=124;out='TIMEOUT'
    res.append({'subtask':st['id'],'cmd':cmd,'rc':rc,'seconds':round(time.time()-t0,1),'tail':'\n'.join(out.strip().split('\n')[-40:]),'at':datetime.datetime.now().isoformat(timespec='seconds')})
    bad+=rc!=0;print('%s rc=%d %s'%(st['id'],rc,'PASS' if rc==0 else 'FAIL'))
if '--record' in a:
    os.makedirs(os.path.join(M,'execution','runs'),exist_ok=True);json.dump(res,open(os.path.join(M,'execution','runs',tid+'.json'),'w'),ensure_ascii=False,indent=1)
print('run-accept %s: %d/%d PASS'%(tid,len(res)-bad,len(res)));sys.exit(1 if bad else 0)
