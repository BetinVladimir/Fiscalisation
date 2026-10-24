#!/usr/bin/env python3
"""Последовательный запуск пакетов fixes/*.md через OpenCode — по одной свежей сессии на дефект.

Изоляция: работа идёт в отдельном git worktree на ветке `opencode-fixes`; основная папка проекта не меняется.
Этапы: opencode run (чистый контекст) -> проверка (run-accept.py + checks.json) -> commit.
Сбой проверки: следующая попытка получает текст ошибки и правит код в той же копии; после --attempts попыток
копия откатывается к последнему коммиту и очередь останавливается (дефект не пропускается).
Возобновление: выполненные дефекты определяются по trailer `Fix-Task:` в истории ветки; если текст пакета
изменился после выполнения (`Fix-Prompt-Sha`), скрипт останавливается (--accept-changed разрешает).
Каждый дефект выполняется по подзадачам: одна подзадача = одна свежая сессия = один коммит (малый контекст).
Результат каждого дефекта пишется в <проект>/docs/MVP1/fixes/<ID>.res.md рядом с промптом (после каждой попытки).

Пример (macOS):
  caffeinate -i python3 docs/MVP1/fixes/run_fixes.py --model provider/id   (необязательно: по умолчанию ollama/qwen3.6:35b-a3b-coding-mtp-q4_K_M)
  python3 docs/MVP1/fixes/run_fixes.py --list              # очередь без запуска
  python3 docs/MVP1/fixes/run_fixes.py --only SEQ-01       # один дефект
Важно: AGENTS.md и fixes/ должны быть закоммичены в базовой ветке (worktree строится от HEAD).
Слияние ветки в основную — вручную после просмотра. Внешние сервисы (БД, API) git не откатывает."""
import argparse,fcntl,hashlib,json,os,re,shlex,subprocess,sys,time

HERE=os.path.dirname(os.path.abspath(__file__))
# Целевая модель (Ollama-тег): qwen3.6:35b-a3b-coding-mtp-q4_K_M. Префикс провайдера в OpenCode предполагается `ollama/`;
# если в `opencode models` он другой, скрипт найдёт строку по тегу. Переопределение: --model provider/id или OPENCODE_MODEL.
MODEL_TAG='qwen3.6:35b-a3b-coding-mtp-q4_K_M'
DEFAULT_MODEL='ollama/'+MODEL_TAG
FORBIDDEN=('docs/MVP1/execution/','docs/MVP1/fixes/','AGENTS.md')   # агент не должен менять план и правила

def sh(cmd,cwd,check=False,timeout=None,env=None):
    p=subprocess.run(cmd,cwd=cwd,capture_output=True,text=True,timeout=timeout,env=env)
    if check and p.returncode: raise SystemExit('ОШИБКА %s:\n%s'%(' '.join(cmd),p.stderr.strip() or p.stdout.strip()))
    return p
def git(args,cwd,check=True): return sh(['git']+args,cwd,check)
def sha(text): return hashlib.sha256(text.encode('utf-8')).hexdigest()[:12]
def tail(text,n=150): return '\n'.join(text.strip().split('\n')[-n:])

MODEL_USED=['?']
def write_res(proj,tid,status,attempts,h,commit='',stat='',check_tail='',final_tail='',scope=None,outside=None):
    """Результат пишется рядом с оригинальным промптом: <проект>/docs/MVP1/fixes/<ID>.res.md (перезаписывается)."""
    L=['# Результат %s'%tid,'','- **Модель:** %s'%MODEL_USED[0],'- **Статус:** %s'%status,'- **Время:** %s'%time.strftime('%Y-%m-%d %H:%M:%S'),
       '- **Попыток:** %d'%len(attempts),'- **Версия промпта (sha):** %s'%h]
    if commit: L.append('- **Коммит:** `%s` (ветка worktree, слияние вручную)'%commit)
    if scope: L.append('- **Файлы подзадач:** %s'%', '.join(scope))
    if outside: L.append('- **Изменено вне файлов подзадач:** %s'%', '.join(outside))
    L+=['','## Попытки','','| Подзадача | № | OpenCode rc | Изменено файлов | Проверка | Лог |','|---|---|---|---|---|---|']
    for a in attempts: L.append('| %s | %d | %s | %s | %s | `%s` |'%(a.get('unit','—'),a['n'],a['rc'],a['changed'],a['verdict'],a['log']))
    if stat: L+=['','## Изменения','','```',stat.strip(),'```']
    if check_tail: L+=['','## Вывод проверки (хвост)','','```',check_tail.strip(),'```']
    if final_tail: L+=['','## Ответ OpenCode (хвост)','','```',final_tail.strip(),'```']
    open(os.path.join(proj,'docs/MVP1/fixes',tid+'.res.md'),'w',encoding='utf-8').write('\n'.join(L)+'\n')

def resolve_model(cmd,explicit):
    """--model -> OPENCODE_MODEL -> DEFAULT_MODEL, если он есть в `opencode models`; иначе единственная строка с MODEL_TAG."""
    m=explicit or os.environ.get('OPENCODE_MODEL')
    if m: return m
    try: out=subprocess.run(shlex.split(cmd)+['models'],capture_output=True,text=True,timeout=120).stdout
    except (FileNotFoundError,subprocess.TimeoutExpired): raise SystemExit('не удалось выполнить `%s models`; укажите --model provider/id'%cmd)
    lines=[l.strip() for l in out.split('\n') if l.strip()]
    if DEFAULT_MODEL in lines: return DEFAULT_MODEL
    cand=[l for l in lines if l.endswith(MODEL_TAG)]
    if len(cand)==1: return cand[0]
    raise SystemExit('Модель %s не найдена в `opencode models` (подходящих строк: %d).%s\nУкажите точный id: --model provider/id (или OPENCODE_MODEL).'%(DEFAULT_MODEL,len(cand),'\n  '+'\n  '.join(cand[:10]) if cand else ''))

def queue(fixes_dir,routes,order='id'):
    """Порядок = порядок строк INDEX.md (P0 по номеру, затем P1, P2)."""
    rows=[]
    for line in open(os.path.join(fixes_dir,'INDEX.md'),encoding='utf-8'):
        m=re.match(r'\| \[([A-Z]+-\d+)\]\([^)]*\) \| (P\d) \| ([\w-]+) \|',line)
        if m: rows.append(m.groups())
    if order=='id':   # строго по номерам: BUG-001, BUG-002, …, затем SEQ-01
        rows.sort(key=lambda r:(0 if r[0].startswith('BUG-') else 1,int(r[0].split('-')[1])))
    keep=[r for r in rows if routes=='all' or r[2]==routes]
    return keep,[r for r in rows if r not in keep]

def done_map(wt,branch,base):
    """{unit: prompt_sha} по коммитам ветки поверх базы; unit = ID подзадачи (Fix-Unit) или ID дефекта."""
    out=git(['log','%s..%s'%(base,branch),'--format=%B%x00'],wt,check=False).stdout
    res={}
    for msg in out.split('\x00'):
        u=re.search(r'^Fix-Unit: (\S+)',msg,re.M) or re.search(r'^Fix-Task: (\S+)',msg,re.M);s=re.search(r'^Fix-Prompt-Sha: (\S+)',msg,re.M)
        if u: res[u.group(1)]=s.group(1) if s else ''
    return res

def units_of(wt,tid):
    """Подзадачи дефекта: [(id, заголовок, файлы)]. Каждая выполняется в отдельной сессии (малый контекст)."""
    try:
        S=json.load(open(os.path.join(wt,'docs/MVP1/execution/status.json'),encoding='utf-8'))['tasks']
        t=[x for x in S if x['id']==tid][0]
        u=[(st['id'],st.get('title',''),[f.rstrip('/') for f in st.get('files',[]) if f]) for st in t.get('subtasks',[])]
        return u or [(tid,t.get('notes','')[:200],[])]
    except Exception: return [(tid,'',[])]

def task_files(wt,tid):
    try:
        S=json.load(open(os.path.join(wt,'docs/MVP1/execution/status.json'),encoding='utf-8'))['tasks']
        t=[x for x in S if x['id']==tid][0]
        return sorted({f.rstrip('/') for st in t.get('subtasks',[]) for f in st.get('files',[]) if f})
    except Exception: return []

def snapshot(proj):
    """Отпечаток изменений основного проекта: {путь: sha} для всех файлов из git status (кроме *.res.md и логов скрипта)."""
    out=git(['status','--porcelain','-uall'],proj,check=False).stdout;snap={}
    for l in out.split('\n'):
        if not l.strip(): continue
        f=l[3:].split(' -> ')[-1].strip('"')
        if f.endswith('.res.md'): continue
        try: snap[f]=hashlib.sha1(open(os.path.join(proj,f),'rb').read()).hexdigest()
        except (IsADirectoryError,FileNotFoundError,PermissionError): snap[f]='-'
    return snap

def supports_dir(cmd):
    try: return '--dir' in subprocess.run(shlex.split(cmd)+['run','--help'],capture_output=True,text=True,timeout=60).stdout
    except Exception: return False

def changed_files(wt):
    out=git(['status','--porcelain','-uall'],wt).stdout
    return [l[3:].split(' -> ')[-1] for l in out.split('\n') if l.strip()]

def run_checks(wt,tid,checks,timeout,unit=None):
    """Возвращает (ok, текст ошибки). Сначала приёмка задачи, затем глобальные проверки."""
    cmds=[[sys.executable,'docs/MVP1/tools/run-accept.py',tid]+(['--subtask',unit] if unit and unit!=tid else [])]+[c for c in checks]
    for c in cmds:
        try: p=sh(c,wt,timeout=timeout)
        except subprocess.TimeoutExpired: return False,'TIMEOUT: '+' '.join(c)
        except FileNotFoundError: return False,'команда не найдена: '+c[0]
        if p.returncode: return False,'$ %s\nrc=%d\n%s'%(' '.join(c),p.returncode,tail(p.stdout+'\n'+p.stderr))
    return True,''

def main():
    ap=argparse.ArgumentParser(description=__doc__,formatter_class=argparse.RawDescriptionHelpFormatter)
    root_default=os.path.abspath(os.path.join(HERE,'..','..','..'))
    ap.add_argument('--project',default=root_default,help='корень репозитория Fiscalisation')
    ap.add_argument('--model',default=None,help='provider/id из `opencode models`; по умолчанию '+DEFAULT_MODEL)
    ap.add_argument('--opencode-cmd',default='opencode',help='команда OpenCode (для тестов можно подставить заглушку)')
    ap.add_argument('--checks',default=None,help='JSON со списком команд, например [["npm","test"]]')
    ap.add_argument('--routes',default='all',choices=['qwen','codex-first','all'],help='фильтр по колонке «Кто» INDEX.md; по умолчанию all — все дефекты выполняются локальной моделью')
    ap.add_argument('--order',default='id',choices=['id','priority'],help='id — строго по номерам (по умолчанию), priority — P0, затем P1, P2')
    ap.add_argument('--attempts',type=int,default=3)
    ap.add_argument('--timeout',type=int,default=1800,help='секунд на один запуск OpenCode')
    ap.add_argument('--check-timeout',type=int,default=900)
    ap.add_argument('--branch',default='opencode-fixes')
    ap.add_argument('--worktree',default=None,help='путь worktree (по умолчанию ~/.opencode-batch/<проект>/wt)')
    ap.add_argument('--only',default=None);ap.add_argument('--from',dest='start',default=None)
    ap.add_argument('--strict-scope',action='store_true',help='менять только файлы подзадач (иначе предупреждение)')
    ap.add_argument('--accept-changed',action='store_true')
    ap.add_argument('--list',action='store_true');ap.add_argument('--dry-run',action='store_true')
    a=ap.parse_args()

    proj=os.path.abspath(a.project);name=os.path.basename(proj)
    home=os.path.expanduser('~/.opencode-batch/%s'%name);logs=os.path.join(home,'logs');os.makedirs(logs,exist_ok=True)
    wt=os.path.abspath(a.worktree or os.path.join(home,'wt'))
    q,skipped=queue(os.path.join(proj,'docs/MVP1/fixes'),a.routes,a.order)
    if a.only: q=[r for r in q if r[0]==a.only]
    if a.start:
        ids=[r[0] for r in q]
        if a.start not in ids: raise SystemExit('нет %s в очереди'%a.start)
        q=q[ids.index(a.start):]
    if a.list or a.dry_run:
        for i,(tid,p,route) in enumerate(q,1): print('%3d. %-9s %s %s'%(i,tid,p,route))
        print('в очереди %d, пропущено по маршруту %d (%s)'%(len(q),len(skipped),', '.join(r[0] for r in skipped[:12])+('…' if len(skipped)>12 else '')))
        return 0
    lock=open(os.path.join(home,'lock'),'w')
    try: fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    except OSError: raise SystemExit('уже запущен другой экземпляр скрипта')

    a.model=resolve_model(a.opencode_cmd,a.model);MODEL_USED[0]=a.model;print('модель:',a.model)
    base=git(['rev-parse','--abbrev-ref','HEAD'],proj).stdout.strip()
    if not os.path.isdir(wt):
        exists=sh(['git','rev-parse','--verify','--quiet',a.branch],proj).returncode==0
        git(['worktree','add']+([wt,a.branch] if exists else ['-b',a.branch,wt,'HEAD']),proj)
        print('worktree создан:',wt,'ветка',a.branch)
    if git(['status','--porcelain'],wt).stdout.strip():
        raise SystemExit('в worktree есть незакоммиченные изменения: %s\nПроверьте их и выполните `git -C %s reset --hard && git -C %s clean -fd`.'%(wt,wt,wt))
    for need in ('AGENTS.md','docs/MVP1/fixes/INDEX.md'):
        if not os.path.exists(os.path.join(wt,need)):
            raise SystemExit('в ветке нет %s: закоммитьте AGENTS.md и docs/MVP1/fixes в базовой ветке, затем удалите worktree и запустите снова'%need)
    checks=json.load(open(a.checks,encoding='utf-8')) if a.checks else []
    base_ref=git(['merge-base',a.branch,base],proj).stdout.strip() if base!=a.branch else git(['rev-list','--max-parents=0','HEAD'],proj).stdout.split()[0]
    t0=time.time();ok_ids=[]
    use_dir=supports_dir(a.opencode_cmd);main_snap=snapshot(proj)
    print('рабочая копия агента:',wt,'| --dir:', 'да' if use_dir else 'нет (PWD задаётся окружением)')
    READ_RULES=('Рабочая директория — '+wt+' (cwd). ВСЕ пути относительные от неё. Запрещено читать и менять файлы вне неё, в том числе основной проект '+proj+'. Правила чтения: файлы длиннее 300 строк НЕ читай целиком — найди нужное через grep -n и читай диапазон (sed -n \'A,Bp\' файл); '
                'правь точечно. Не открывай больше 3 файлов за сессию. Лишние инструменты и поиск по всему репозиторию не нужны.')
    for tid,prio,route in q:
        pfile=os.path.join(wt,'docs/MVP1/fixes',tid+'.md')
        if not os.path.exists(pfile): raise SystemExit('нет пакета %s в worktree'%pfile)
        text=open(pfile,encoding='utf-8').read();h=sha(text)
        units=units_of(wt,tid);atts=[];last_out='';outside=[]
        for ui,(uid,utitle,ufiles) in enumerate(units,1):
            done=done_map(wt,a.branch,base_ref)
            if uid in done:
                if done[uid] and done[uid]!=h and not a.accept_changed:
                    raise SystemExit('%s: пакет изменён после выполнения (было %s, стало %s). Проверьте и добавьте --accept-changed.'%(uid,done[uid],h))
                print('[пропуск] %s уже выполнен'%uid);continue
            scope=ufiles or task_files(wt,tid);last_err=''
            for n in range(1,a.attempts+1):
                if n==1:
                    msg=('Выполни ТОЛЬКО подзадачу %s (%d из %d) дефекта %s: %s\nФайлы подзадачи: %s\nОстальные подзадачи делаются отдельными сессиями — не трогай их. '
                         'Следуй AGENTS.md. %s Перед завершением запусти приёмку подзадачи и покажи результат.\n\n--- ПАКЕТ ДЕФЕКТА ---\n%s')%(uid,ui,len(units),tid,utitle,', '.join(scope) or 'см. пакет',READ_RULES,text)
                else:
                    msg=('Предыдущая попытка по подзадаче %s не прошла проверку. Исправь причину в коде, не ослабляя тесты и не расширяя набор файлов (%s). %s '
                         'Пакет дефекта: docs/MVP1/fixes/%s.md (прочитай нужный раздел). Ошибка проверки:\n\n%s')%(uid,', '.join(scope) or 'см. пакет',READ_RULES,tid,last_err)
                cmd=shlex.split(a.opencode_cmd)+['run']+(['--dir',wt] if use_dir else [])+(['--model',a.model] if a.model else [])+[msg]
                log=os.path.join(logs,'%s.attempt%d.log'%(uid,n));print('[%s] %s попытка %d/%d …'%(time.strftime('%H:%M:%S'),uid,n,a.attempts),flush=True)
                try:
                    p=sh(cmd,wt,timeout=a.timeout,env=dict(os.environ,PWD=wt));out=p.stdout+'\n'+p.stderr;rc=p.returncode
                except subprocess.TimeoutExpired: out='TIMEOUT OpenCode';rc=124
                except FileNotFoundError: raise SystemExit('не найдена команда %s (установите OpenCode или укажите --opencode-cmd)'%a.opencode_cmd)
                open(log,'w',encoding='utf-8').write(out);last_out=tail(out,60)
                att={'unit':uid,'n':n,'rc':rc,'changed':0,'verdict':'—','log':log};atts.append(att)
                now_snap=snapshot(proj)
                if now_snap!=main_snap:
                    diff=sorted(k for k in set(now_snap)|set(main_snap) if now_snap.get(k)!=main_snap.get(k))
                    write_res(proj,tid,'ОСТАНОВЛЕНО: агент изменил основной проект',atts,h,check_tail='Изменены вне worktree: '+', '.join(diff),final_tail=last_out)
                    raise SystemExit('\n✖ Агент изменил файлы ОСНОВНОГО проекта (вне worktree): %s\nПроверьте `git -C %s diff`, откатите вручную (git checkout -- <файл>) и проверьте, что OpenCode работает в %s (см. лог %s).'%(', '.join(diff[:10]),proj,wt,log))
                ch=changed_files(wt);att['changed']=len(ch)
                bad=[f for f in ch if f.startswith(FORBIDDEN)]
                if bad:
                    git(['checkout','--']+bad,wt,check=False);git(['clean','-fd','--']+bad,wt,check=False)
                    last_err='Запрещённые файлы изменены и откатены: %s'%', '.join(bad);print('  ',last_err);ch=changed_files(wt)
                def progress(extra=''): write_res(proj,tid,'В РАБОТЕ',atts,h,check_tail=extra,final_tail=last_out,scope=scope,outside=outside)
                if not ch:
                    last_err='OpenCode не изменил ни одного файла (rc=%d). Лог: %s'%(rc,log);att['verdict']='нет изменений';print('  ',last_err);progress(last_err);continue
                outside=[f for f in ch if scope and not any(f==s or f.startswith(s+'/') for s in scope)]
                if outside:
                    msg_o='вне файлов подзадачи: %s'%', '.join(outside)
                    if a.strict_scope: last_err='Изменены файлы '+msg_o+'. Верни их и правь только: '+', '.join(scope);att['verdict']='вне области';print('  ',last_err);progress(last_err);continue
                    print('   предупреждение:',msg_o)
                ok,err=run_checks(wt,tid,checks,a.check_timeout,uid)
                if not ok: last_err=err;att['verdict']='не прошла';print('   проверка не прошла:',err.split('\n')[0]);progress(err);continue
                att['verdict']='прошла'
                git(['add','-A'],wt)
                git(['commit','-q','-m','fix(%s): %s (OpenCode)\n\nFix-Task: %s\nFix-Unit: %s\nFix-Prompt-Sha: %s\nFix-Attempt: %d'%(tid,uid,tid,uid,h,n)],wt)
                cm=git(['rev-parse','--short','HEAD'],wt).stdout.strip();print('  ✔ %s выполнен, коммит %s'%(uid,cm));ok_ids.append(uid)
                last_commit=(cm,git(['show','--stat','--format=','HEAD'],wt).stdout)
                if ui==len(units): write_res(proj,tid,'ВЫПОЛНЕНО',atts,h,commit=cm,stat=last_commit[1],check_tail='приёмка подзадач и checks.json: OK',final_tail=last_out,scope=scope,outside=outside)
                else: progress('подзадача %s выполнена, коммит %s'%(uid,cm))
                break
            else:
                write_res(proj,tid,'НЕ ВЫПОЛНЕНО (копия откатена, подзадача %s)'%uid,atts,h,stat=git(['diff','--stat','HEAD'],wt,check=False).stdout,check_tail=last_err,final_tail=last_out,scope=scope,outside=outside)
                git(['reset','--hard','HEAD'],wt);git(['clean','-fd'],wt)
                print('\n✖ %s не выполнен за %d попыток. Копия откатена к последнему коммиту. Последняя ошибка:\n%s\nЛоги: %s'%(uid,a.attempts,tail(last_err,40),logs))
                return 1
    print('\nГотово: выполнено сейчас подзадач %d, за %.0f мин. Ветка %s, worktree %s. Слияние — вручную после просмотра.'%(len(ok_ids),(time.time()-t0)/60,a.branch,wt))
    return 0
if __name__=='__main__': sys.exit(main())
