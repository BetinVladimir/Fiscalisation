#!/usr/bin/env python3
"""Генерирует OPEN_WORK.md — только актуальные работы после сверки с кодом (execution/reverify.json).
  python3 docs/MVP1/tools/gen-open-work.py [--check]"""
import json,os,sys,collections
H=os.path.dirname(os.path.abspath(__file__));M=os.path.dirname(H)
S=json.load(open(os.path.join(M,'execution/status.json'),encoding='utf-8'))['tasks']
R=json.load(open(os.path.join(M,'execution/reverify.json'),encoding='utf-8'))
RT=json.load(open(os.path.join(M,'execution/routing.json'),encoding='utf-8')) if os.path.exists(os.path.join(M,'execution/routing.json')) else {}
def line(t):
    r=R.get(t['id'],{});who=RT.get(t['id'],{}).get('route','')
    return '| %s | %s | %s | %s | `%s` |'%(t['id'],t.get('severity',''),who,r.get('note','').replace('|','/')[:170],r.get('cmd','').replace('|','/')[:90])
hdr='| Задача | P | Кто | Что осталось / доказательство | Проверка |\n|---|---|---|---|---|'
closed=[t for t in S if t['status']=='DONE']
part=[t for t in S if t.get('reverify')=='PARTIAL']
opn=[t for t in S if t.get('reverify')=='OPEN' and t['status']!='DONE']
unv=[t for t in S if t['status']!='DONE' and 'reverify' not in t]
c=collections.Counter(t['id'].split('-')[0] for t in unv)
o=['# OPEN_WORK · актуальные работы после сверки с кодом (2026-10-24)\n',
'Сгенерировано `tools/gen-open-work.py` из `execution/reverify.json`. Проверка статическая (grep/чтение кода): на машине нет Go и Gradle, поэтому Go/Kotlin-тесты **не запускались**; наличие теста ≠ его зелёный прогон.\n',
'Итого: закрыто %d, частично %d, подтверждённо открыто %d, не проверялось кодом %d (всего %d).\n'%(len(closed),len(part),len(opn),len(unv),len(S)),
'## 1. Закрыто (из очереди убрано)\nОтмечены `DONE`. Для пяти из них закрытие подтверждено кодом при этой сверке: BUG-009, S3D-02, DSY-01, TG-002, BUG-060 (ротация пароля keystore — действие владельца). Остальные — задачи-документы/решения.\n',
'| Задача | Заметка |\n|---|---|']
for t in closed: o.append('| %s | %s |'%(t['id'],t['notes'][:150].replace('|','/')))
o+=['\n## 2. Закрыты ошибочно или не полностью → переоткрыты\nBUG-001, BUG-002, BUG-003, BUG-010, BUG-011 стояли как `DONE` с пометкой «Fixed», но код этого не подтверждает (например `validMoney` по-прежнему принимает префикс +/−).\n','\n## 3. Частично сделано (осталось сузить)\n',hdr]+[line(t) for t in part]
o+=['\n## 4. Подтверждённо открыто (в коде следов реализации нет или дефект на месте)\n',hdr]+[line(t) for t in opn]
o+=['\n## 5. Не проверялось кодом (%d задач)\nЮридические/нормативные, документные, решения и задачи, чьи признаки закрытия не извлекаются поиском. Считаются актуальными, пока не доказано обратное. Состав по префиксам: %s.\n'%(len(unv),', '.join('%s %d'%(k,v) for k,v in sorted(c.items()))),
'Список: '+', '.join(t['id'] for t in unv)+'\n']
text='\n'.join(o)
p=os.path.join(M,'OPEN_WORK.md')
if '--check' in sys.argv:
    cur=open(p,encoding='utf-8').read() if os.path.exists(p) else ''
    print('open-work stale=%d'%(cur!=text));sys.exit(0 if cur==text else 1)
open(p,'w',encoding='utf-8').write(text);print(len(text),'chars',len(closed),len(part),len(opn),len(unv))
