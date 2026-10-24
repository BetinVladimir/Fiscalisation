#!/usr/bin/env python3
"""Генерирует docs/MVP1/fixes/<ID>.md (пакет на дефект для одной сессии OpenCode) и fixes/INDEX.md.
  python3 docs/MVP1/tools/gen-fixes.py            # перегенерировать (после apply_reverify.py)
Дефект = BUG-* и SEQ-01, не DONE. Пакет = шапка (состояние кода, проверка) + вывод task-pack.py."""
import json,os,subprocess,sys
H=os.path.dirname(os.path.abspath(__file__));M=os.path.dirname(H);ROOT=os.path.abspath(os.path.join(M,'..','..'))
L=lambda n:json.load(open(os.path.join(M,'execution',n),encoding='utf-8'))
S={t['id']:t for t in L('status.json')['tasks']};R=L('reverify.json');RT=L('routing.json')
def tok(s):
    cy=sum(1 for ch in s if 'Ѐ'<=ch<='ӿ');return int((len(s)-cy)/3.6+cy/1.9)+1
out=os.path.join(M,'fixes');os.makedirs(out,exist_ok=True)
def big_files(t):
    res=[]
    for st in t.get('subtasks',[]):
        for f in st.get('files',[]):
            for b in (ROOT,os.path.join(ROOT,'..','POS'),os.path.join(ROOT,'..','BeeloyBackend')):
                q=os.path.join(b,f)
                if os.path.isfile(q):
                    n=sum(1 for _ in open(q,encoding='utf-8',errors='ignore'))
                    if n>300: res.append('%s (%d строк)'%(f,n))
                    break
    return sorted(set(res))
rows=[]
for i in sorted(x for x in S if (x.startswith('BUG-') or x=='SEQ-01') and S[x]['status']!='DONE'):
    pack=subprocess.run([sys.executable,os.path.join(H,'task-pack.py'),i],capture_output=True,text=True,cwd=ROOT).stdout
    r=R.get(i,{})
    head='<!-- Сгенерировано tools/gen-fixes.py; не править руками -->\n# Дефект %s · %s · исполнитель: локальная модель (qwen), по подзадачам: %d\n\n**Состояние кода на 2026-10-24:** %s — %s\n**Проверка состояния:** `%s`\n**Приёмка подзадач:** `python3 docs/MVP1/tools/run-accept.py %s`\n%s**Правила:** `AGENTS.md` (правь только файлы подзадачи; сначала тест и покажи, что он падает; целые центы; UNKNOWN ≠ FAILED).\n\n---\n\n'%(i,S[i].get('severity',''),len(S[i].get('subtasks',[])) or 1,r.get('verdict','не проверялось'),r.get('note','—'),r.get('cmd','—'),i,(('**Крупные файлы — не читать целиком, только grep -n и sed -n:** '+'; '.join(big_files(S[i]))+'\n') if big_files(S[i]) else ''))
    txt=head+pack;open(os.path.join(out,i+'.md'),'w',encoding='utf-8').write(txt)
    rows.append((i,S[i].get('severity',''),'qwen',r.get('verdict','—'),tok(txt),(r.get('note') or S[i]['notes'].split(' | [reverify')[0])[:90]))
idx=['# fixes · пакеты задач по дефектам\n','Один файл = один дефект = вход для одной сессии OpenCode (qwen 64k). Генерация: `python3 docs/MVP1/tools/gen-fixes.py`. Порядок: P0 по номеру, затем P1, P2.\n','**Автозапуск:** `python3 docs/MVP1/fixes/run_fixes.py --model provider/model` — по одной свежей сессии OpenCode на дефект в отдельном git worktree, проверка, коммит, возобновление после остановки; `--list` показывает очередь, `--only ID` один дефект. Выполняются **все** дефекты локальной моделью, каждая подзадача — отдельной сессией (малый контекст), по умолчанию строго по номерам. Перед запуском закоммитьте `AGENTS.md` и `docs/MVP1/fixes/`.\n','| Дефект | P | Кто | Код сейчас | ≈токенов | Подзадач | Суть |','|---|---|---|---|---|---|---|']
for r in sorted(rows,key=lambda x:(x[1],x[0])): idx.append('| [%s](%s.md) | %s | %s | %s | %d | %d | %s |'%(r[0],r[0],r[1],r[2],r[3],r[4],len(S[r[0]].get('subtasks',[])) or 1,r[5].replace('|','/')))
open(os.path.join(out,'INDEX.md'),'w',encoding='utf-8').write('\n'.join(idx)+'\n');print(len(rows),'packs, max tokens',max(r[4] for r in rows))
