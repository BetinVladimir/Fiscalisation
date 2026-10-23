#!/usr/bin/env python3
"""Маршрутизация задач по исполнителям (ARCH_21 §3).
  python3 docs/MVP1/tools/route-tasks.py            # печатает сводку
  python3 docs/MVP1/tools/route-tasks.py --write    # пишет execution/routing.json
  python3 docs/MVP1/tools/route-tasks.py --check    # exit 1, если routing.json устарел или есть задача без маршрута
Маршруты: human (решение/письмо/юрист), qwen (кодит qwen), codex-first (кодит сильная модель, qwen ревьюит тесты),
hil (физическое устройство, делает разработчик)."""
import json,os,re,sys
HERE=os.path.dirname(os.path.abspath(__file__));MVP=os.path.dirname(HERE);ROOT=os.path.abspath(os.path.join(MVP,'..','..'))
HUMAN=('DOC','NORM','SCOPE','GATE','PRB','STYLE','W0','MC','CL','SCR')
HIL=('HILMX',)
CODEX_KW=re.compile(r'HMAC|CBOR|ECDSA|NimBLE|mutex|saga|транзакц|атомарн|ротац|keystore|CAS|WAL|power-cut|питани',re.I)
def load():
    s=json.load(open(os.path.join(MVP,'execution','status.json')));t=s['tasks']
    return list(t.values() if isinstance(t,dict) else t)
def route(t):
    pre=t['id'].split('-')[0]
    if pre in HIL or t['id'].startswith('TST-HIL'): return 'hil','HIL'
    if pre in HUMAN or 'owner action' in t.get('notes',''): return 'human','нет кода'
    files=set();big=[]
    for st in t.get('subtasks',[]):
        for f in st.get('files',[]):
            files.add(f);p=os.path.join(ROOT,f)
            if not f.startswith('docs/') and os.path.isfile(p) and os.path.getsize(p)>40000: big.append(os.path.basename(f))
    code=[f for f in files if not f.startswith('docs/')]
    if big: return 'codex-first','файл >10k токенов: '+','.join(sorted(set(big)))
    if len(code)>4: return 'codex-first','>4 файлов кода'
    txt=t.get('notes','')+' '.join(st.get('title','') for st in t.get('subtasks',[]))
    if CODEX_KW.search(txt) and t.get('severity')=='P0': return 'codex-first','P0 + критичная область: '+CODEX_KW.search(txt).group(0)
    return 'qwen','по умолчанию'
def main():
    T=load();R={t['id']:dict(zip(('route','why'),route(t))) for t in T}
    cnt={}
    for r in R.values(): cnt[r['route']]=cnt.get(r['route'],0)+1
    out=os.path.join(MVP,'execution','routing.json')
    if '--write' in sys.argv: json.dump(R,open(out,'w'),ensure_ascii=False,indent=1,sort_keys=True)
    if '--check' in sys.argv:
        old=json.load(open(out)) if os.path.exists(out) else {}
        bad=[i for i in R if old.get(i)!=R[i]]
        print('routing tasks=%d stale=%d %s'%(len(R),len(bad),cnt));sys.exit(1 if bad else 0)
    print(cnt)
main()
