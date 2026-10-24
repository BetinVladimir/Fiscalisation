#!/usr/bin/env python3
"""DOC-007: применяет execution/reverify.json к status.json (CLOSED -> DONE, PARTIAL -> заметка «Осталось», OPEN -> проверено).
  python3 docs/MVP1/tools/apply_reverify.py          # применить
  python3 docs/MVP1/tools/apply_reverify.py --check  # exit 1, если status.json расходится с reverify.json"""
import json,os,sys
H=os.path.dirname(os.path.abspath(__file__));M=os.path.dirname(H)
R=json.load(open(os.path.join(M,'execution','reverify.json'),encoding='utf-8'))
p=os.path.join(M,'execution','status.json');S=json.load(open(p,encoding='utf-8'));bad=[]
for t in S['tasks']:
    r=R.get(t['id'])
    if not r: continue
    tag='[reverify 2026-10-24 %s] '%r['verdict']
    want={'CLOSED':'DONE','PARTIAL':'IN_PROGRESS','OPEN':'NOT_STARTED'}.get(r['verdict'])
    if want and t['status']!=want and not (want=='NOT_STARTED' and t['status'] in ('IN_PROGRESS','BLOCKED_SPEC')):
        bad.append(t['id']); t['status']=want
    base=t.get('notes','').split(' | [reverify')[0]
    new=(base+' | '+tag+r['note']+' | проверка: '+r['cmd']).strip(' |')
    if new!=t.get('notes',''): bad.append(t['id']); t['notes']=new
    t['reverify']=r['verdict']
if '--check' in sys.argv: print('apply_reverify stale=%d'%len(set(bad)));sys.exit(1 if bad else 0)
json.dump(S,open(p,'w',encoding='utf-8'),ensure_ascii=False,indent=1);print('applied',len(R))
