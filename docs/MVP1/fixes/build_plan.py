#!/usr/bin/env python3
"""Build reproducible qwen work packets without changing execution/status.json."""
import argparse, hashlib, json, re, subprocess
from pathlib import Path

POLICY = '''Цель: POS + MiniPOS, MQTT и BLE, BlueCash, Datecs/S3/COM, BluePay/S3/BLE, Daisy Compact/S3/USB, Smart S/Android/HTTP. D-1…D-20: последние подтверждённые решения DECISIONS §5 приоритетнее старых карточек. Транзакции до HIL. Деньги: целые центы, half-up; golden 1200+167=1367, CASH 500/CARD 867. UNKNOWN после записи: только lookup, никаких повторных денежных команд/автовозврата. Сторно до refund; crash/timeout refund сохраняет отдельное durable состояние. Compact: CARD/split запрещены до I/O. Smart S: только LAB без номера; PROD UNKNOWN, не синтетический номер. E-чек выключен: проверять отказ, не реализовывать отправку. BC-50: сторно cmd43, внешний терминал; часы читать cmd62, WARN >5мин/BLOCK >2ч, business_date Europe/Sofia; валютный режим 2/3. Datecs SEQ 0x20..0x7f, Daisy 0x20..0xff, CP1251. Статусы исторического трекера не доказывают готовность. Никогда не менять status.json и не записывать HIL PASS по эмулятору.'''
RULES = '''Одна свежая сессия на подзадачу. Соблюдать AGENTS.md; если правила уже включены в system context, повторно файл не читать. Читать не более 3 файлов одновременно, >300 строк — rg и диапазоны sed. Не сканировать build/node_modules/.store-build. plan.json читать программно по нужным ID; source_documents целиком в контекст не выводить. Файлы sources — контекст; изменять files. mutation_files разрешены только для временной red-мутации: контроллер восстанавливает исходные байты до green. Остальные read_files — только чтение. Обязательные тесты включать в приёмку; запрет passWithNoTests, SKIP и ослабления assert. Native suite harness: использовать готовый native_harness.check_native_tests из docs/MVP1/fixes (API: case,argv,root,cwd,required_tests,markers,minimum,timeout). Harness всегда требует rc0 и реальные результаты; red определяет контроллер. Не писать _is_red_phase и не выбирать expected по исходнику/фазе. Сначала воспроизводящий тест: сохранить red evidence с реальной assertion failure, не compile/tool/config error; затем минимальная реализация и green. Для самостоятельной генерации тестов: доказать чувствительность временной мутацией продукта в изолированной копии и вернуть мутацию. Ответ до 8000 токенов, diff одной итерации до 400 строк. Не угадывать vendor-параметры, не хардкодить успех. Нет инструмента/SDK/ответа: BLOCKED с причиной, не PASS. После изменения публичного контракта обновить схемы/lock и проверить потребителей. Адресные тесты подзадачи обязательны; полная регрессия модулей в REGRESSION-01; E2E 3 из 3. Исходные FAIL других модулей из baseline не скрывать и не подменять положительным статусом MVP. Никаких deploy, запросов к реальному эквайрингу, отправки писем или изменения реальных данных.'''
ALIASES = {'BUG-027':'VG-02','ARCH-005':'BUG-023','ARCH-001':'TG-001','BUG-063':'VG-01','BUG-064':'VG-02','BUG-065':'VG-03','BUG-066':'VG-04','BUG-067':'VG-05','BUG-068':'VG-06','BUG-069':'VG-07','BUG-070':'VG-09','NAP-16':'VG-09','BP-01':'DOC-V3'}
EXTERNAL = {'GATE-01','MC-01'}

def estimate(s):
    # Conservative planning bound; actual tokenizer is provider-dependent.
    return len(s.encode('utf-8')) // 2 + 1

def cards_in(p):
    cards={}
    for f in sorted(p.glob('ARCH_*.md')):
        text=f.read_text()
        for match in re.finditer(r'^### ([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)\b[^\n]*',text,re.M):
            key=match[1]
            if key.startswith('SCR-'):key='SCRIPT-'+key[4:]
            following=re.search(r'^#{1,3} ',text[match.end():],re.M)
            end=match.end()+following.start() if following else len(text)
            cards.setdefault(key,(text[match.start():end].strip(),f.name))
    return cards

def normalize(s):
    s=s.replace('python3 scripts/check_plan_acceptance.rb','ruby scripts/check_plan_acceptance.rb')
    s=s.replace('IoT/protocol-abstraction/src/intent_processor.cpp','IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp')
    s=re.sub(r'(?<![\w./])POS/', '../POS/',s)
    s=re.sub(r'(?<![\w./])BeeloyBackend/', '../BeeloyBackend/',s)
    s=re.sub(r'(?m)^go (test|vet|build) ',r'cd fiscal-backend && go \1 ',s)
    s=s.replace('npx eslint src --rule','npx eslint src') if 'no-restricted-syntax' not in s else s
    return s

def paths(text):
    result=[]
    for v in re.findall(r'`([^`\n]+)`',text):
        if re.match(r'^(?:\.\./)?(?:[A-Za-z][\w.-]*/)+[\w./*-]+$',v) and '*' not in v:
            result.append(normalize(v))
    return list(dict.fromkeys(result))

def mode(tid):
    if tid.startswith(('NORM-','DOC-V','PRB-')) or tid in {'GATE-01','DSY-06','HILMX-01','POS-BLE-03'}:return 'external'
    return 'code'

def regression(files):
    suites=[]
    for f in files:
        if f.startswith('fiscal-backend/'): suites.append('cd fiscal-backend && go test -json ./... -count=1')
        if f.startswith('SmartDevices/'): suites.append('cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest :daisy-smart-app:testDebugUnitTest --rerun-tasks')
        if f.startswith('IoT/'): suites.append('make iot-test')
        if f.startswith('../POS/pos-app/'):suites.append('cd ../POS/pos-app && npx jest --runInBand')
        if f.startswith('../POS/pos-gateway/'):suites.append('cd ../POS/pos-gateway && go test -json ./... -count=1')
        if f.startswith('../POS/erp-connector/'):suites.append('cd ../POS/erp-connector && go test -json ./... -count=1')
        if f.startswith('minipos/beeminipos-backend/'):suites.append('cd minipos/beeminipos-backend && go test -json ./... -count=1')
    return list(dict.fromkeys(suites))

def test_path(value):
    path=Path(value)
    return any(v in ('tests','__tests__','test','testdata','fixtures') for v in path.parts) or path.name.startswith('test_') or 'golden' in path.name.lower() or path.name.endswith(('_test.go','Test.kt','Tests.kt','Test.cpp','.test.ts','.test.tsx','.spec.ts'))

def build(root,out):
    p=root/'docs/MVP1';out.mkdir(parents=True,exist_ok=True)
    src=json.loads((p/'execution/status.json').read_text())['tasks'];cards=cards_in(p)
    tasks={}
    inventory=[Path(v) for v in subprocess.run(['rg','--files','--hidden','-g','!node_modules','-g','!.git','-g','!build','-g','!.store-build'],cwd=root,capture_output=True,text=True,check=True).stdout.splitlines()]
    for t in src:
        tid=t['id'];card,doc=cards.get(tid,('',t['doc']))
        task={'id':tid,'priority':t.get('severity') or 'P1','source':doc,'source_status':t['status'],'mode':mode(tid),'depends':t.get('depends',[])[:], 'alias_of':ALIASES.get(tid),'units':[], 'spec':normalize(card), 'notes':normalize(t.get('notes','').split(' | [reverify')[0])}
        for st in t.get('subtasks',[]):
            title=normalize(st['title']);files=[normalize(f).rstrip('/') for f in st.get('files',[]) if f and f not in ('—','то же')]
            files=[f for f in files if not ('`' in f or '...' in f)]
            # Bare names resolve only when unique across source scopes, never invent a path.
            fixed=[]
            for f in files:
                if '/' not in f:
                    candidates=[root/x for x in inventory if x.name==f] if '.' in f else []
                    if len(candidates)==1:f=str(candidates[0].relative_to(root))
                fixed.append(f)
            task['units'].append({'id':st['id'],'title':title,'files':list(dict.fromkeys(fixed)), 'test':st.get('test',''), 'accept':normalize(st.get('accept','')), 'sources':[doc], 'phase':'red' if re.search(r'красн|FAIL до|до правки FAIL',st.get('test',''),re.I) and not re.search(r'тест.*правк|реализ|перепис|замен',title,re.I) else 'green'})
        if not task['units'] and task['mode']=='code' and not task['alias_of']:
            files=paths(card)
            task['units']=[{'id':tid+'.1','title':normalize(card or t.get('notes','')),'files':files,'test':'Выполнить тесты и DoD исходной карточки; подтвердить red/green.', 'accept':'','sources':[doc],'phase':'green'}]
        tasks[tid]=task
    # Script IDs use a separate namespace: SCR-36 already has a different meaning in status.json.
    for tid,(card,doc) in cards.items():
        if not tid.startswith('SCRIPT-'):continue
        m=re.search(r'\*\*Приёмка:\*\*\s*`([^`]+)`',card)
        files=paths(card)
        tasks[tid]={'id':tid,'priority':'P0' if '· P0' in card else 'P1','source':doc,'source_status':'UNVERIFIED','mode':'code','depends':[], 'spec':normalize(card),'notes':'Самотест good/bad, затем реализация, затем Makefile/CI. Не менять legacy status.json.', 'units':[{'id':tid+'.1','title':'Реализовать скрипт по полной спецификации и good/bad selftests; подключить в Makefile и scripts/README.md. Не принимать пустой набор проверок.','files':list(dict.fromkeys(files+['Makefile','scripts/README.md','scripts/tests'])),'sources':[doc],'test':'good rc0; bad rc1 с ожидаемым кодом; отсутствующая среда rc3; fixtures не подгонять под реализацию.','accept':normalize(m[1]) if m else '', 'phase':'green'}]}
    # Test-only rows are first-class packets even when absent in status.json.
    for doc in ['ARCH_03_TESTS_E2E.md','ARCH_18_TEST_LOGIC_SPECS.md']:
        for line in (p/doc).read_text().splitlines():
            m=re.match(r'^\| (TST-[A-Z0-9]+-\d+) \|',line)
            if not m or m[1] in tasks:continue
            tid=m[1];detail=cards.get(tid,('',doc))[0] or line
            owners=re.findall(r'\b(?:BUG|TG|VG)-\d+\b',line)
            files=paths(line+detail)
            tasks[tid]={'id':tid,'priority':'P1','source':doc,'source_status':'UNTRACKED','mode':'code','depends':owners,'spec':normalize(detail),'notes':'Самостоятельное покрытие ARCH_03/18; владельцы реализации перечислены в depends.', 'units':[{'id':tid+'.1','title':'Создать/проверить тест по Given/When/Then исходной спецификации; подтвердить чувствительность мутацией в изолированной копии.','files':files,'sources':['ARCH_03_TESTS_E2E.md','ARCH_18_TEST_LOGIC_SPECS.md'],'test':'Реальная проверка поведения; удалить/исказить проверяемый эффект временно → FAIL; вернуть → PASS.','accept':'','phase':'green'}]}
    # Apply reviewed corrections in this execution overlay only.
    tasks['BUG-031']['units']=[{'id':'BUG-031.1','title':'Тест printerTestClosesNonFiscalReceiptOnFailure: сбой cmd42 закрывает нефискальный документ cmd39; корректно журналировать ошибку закрытия. Реализовать минимальную правку.','files':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt','SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt'],'sources':['ARCH_01_BUGS.md'],'test':'До правки assertion FAIL; после PASS, проверить точный порядок вызовов.','accept':'cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*printerTestClosesNonFiscalReceiptOnFailure*" --rerun-tasks','phase':'green'}]
    for tid in ['BUG-062','BUG-042','BUG-043','BUG-061']:
        t=tasks[tid]
        extras={'BUG-062':['contracts/bg-unp-v1-golden.json','SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt','SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/UnpGoldenTest.kt','IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp','IoT/firmware/edge-agent-s3/idf/tests/intent_processor_native.cpp'], 'BUG-042':['fiscal-backend/internal/domain/payment_capability_test.go','fiscal-backend/internal/domain/service.go','IoT/firmware/edge-agent-s3/idf/tests/intent_processor_native.cpp'], 'BUG-043':['IoT/firmware/edge-agent-s3/idf/tests/intent_processor_native.cpp'], 'BUG-061':['../POS/pos-app/src/infrastructure/supto/LocalReceiptOperations.ts','../POS/pos-app/src/shared/utils/decimal.golden.test.ts','../POS/pos-app/eslint.config.js']}[tid]
        for u in t['units']:u['files']=list(dict.fromkeys(u['files']+extras))
    tasks['BUG-061']['units'][0]['accept']='cd ../POS/pos-app && npx jest --runInBand -t roundingGolden'
    for u in tasks['BUG-061']['units'][1:]:u['accept']='cd ../POS/pos-app && npx jest --runInBand -t roundingGolden && npx eslint src'
    # Preserve work implementation even when an external dependency is unresolved.
    for t in tasks.values():
        t['external_requires']=[d for d in t['depends'] if d in tasks and tasks[d]['mode']=='external']
        t['depends']=[d for d in t['depends'] if d not in t['external_requires']]
        if t.get('alias_of'):
            t['depends']=[t['alias_of']];t['units']=[];t['spec']='Алиас '+t['alias_of']+'. Собственной реализации/приёмки нет; завершение только по канонической задаче.'
        for u in t['units']:
            # Include explicit test paths from the specification; runner preflight refuses unresolved scopes.
            u['files']=list(dict.fromkeys(u['files']+[f for f in paths(t['spec']) if 'test' in f.lower()]))
            u['regression']=regression(u['files'])
    add_extensions(tasks)
    add_e2e_gaps(tasks, p)
    complete_units(tasks, root)
    execution_contracts(tasks, root)
    import runpy
    runpy.run_path(str(Path(__file__).with_name('review_overrides.py')))['fix_plan'](tasks)
    tasks['REGRESSION-01']={'id':'REGRESSION-01','priority':'P0','mode':'code','source':'ARCH_21_DELIVERY_PIPELINE.md','source_status':'NEW','depends':[t['id'] for t in tasks.values() if t['mode']=='code' and not t.get('alias_of')],'notes':'Финальные ворота кодогенерации; не заменяют HIL/вендоров.','spec':'После всей очереди выполнить regression всех затронутых модулей, contract locks, Postgres integration, EMU E2E 3/3. Никакого HIL PASS. Каждый сбой — отдельный дефект, не ослаблять тест.','units':[{'id':'REGRESSION-01.1','title':'Создать единый offline/test-environment regression harness: subprocess module runners, проверка ненулевого числа тестов, результаты per module, зависимости среды как BLOCKED. Запустить backend/Android/S3/POS/MiniPOS/ERP/public API contract+integration suites и tests/e2e трижды.','files':['tests/e2e/test_mvp1_regression.py','tests/e2e/COVERAGE.md','Makefile'],'sources':['ARCH_21_DELIVERY_PIPELINE.md'],'test':'Все применимые модули и EMU E2E успешно, no tests/SKIP/missing tool не PASS.','accept':'python3 -m unittest discover -s tests/e2e -p test_mvp1_regression.py','regression':[],'repeat':3,'phase':'green','require_runtime':True}]}
    for task in tasks.values():
        for unit in task['units']:
            unit['red_required']=True
            unit['mutation_files']=list(dict.fromkeys(unit.get('mutation_files',[])+[f for f in unit.get('read_files',[]) if not f.startswith('docs/') and not f.endswith('.md') and not test_path(f)]))
    # Canonicalize dependencies and remove obsolete alias back-edges.
    for tid,t in tasks.items():
        deps=[]
        for d in t['depends']:
            d=ALIASES.get(d,d)
            if d!=tid and d not in deps:deps.append(d)
        t['external_requires']=list(dict.fromkeys(t.get('external_requires',[])+[d for d in deps if d in tasks and tasks[d]['mode']=='external']))
        t['depends']=[d for d in deps if d not in t['external_requires']]
    manifest={'schema':2,'coverage':{'legacy_tasks':[t['id'] for t in src],'script_tasks':[t for t in tasks if t.startswith('SCRIPT-')],'gap_scenarios':[f'E2E-GAP-{n:03d}' for n in range(1,37)],'required_routes':[{'path':path,'client':client,'transport':transport,'scenario_policy':'capability_driven'} for path in ['P-A','P-B','P-C','P-D','P-S'] for client in ['pos','minipos'] for transport in ['mqtt','ble']]},'source_documents':{f.name:f.read_text() for f in sorted(p.glob('*.md'))},'source_cards':{key:[key] if key in tasks else [] for key in cards},'context_tokens':65536,'packet_estimate_limit':12000,'policy':POLICY,'rules':RULES,'tasks':list(tasks.values())}
    (out/'plan.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
    (out/'golden_money_validation.json').write_text(json.dumps(tasks['BUG-002']['units'][0]['test_vectors'],ensure_ascii=False,indent=2)+'\n')
    for t in tasks.values():
        body=f"<!-- Generated by build_plan.py; edit source/overrides, then rebuild. -->\n# {t['id']} · {t['priority']} · {t['mode']}\n\nИсточник: ../{t['source']}; legacy status: {t['source_status']} (не evidence).\nЗависимости: {', '.join(t['depends']) or '—'}. Внешние входы: {', '.join(t.get('external_requires',[])) or '—'}.\n\n{POLICY}\n\n{RULES}\n\n{t.get('notes','')}\n\n"
        for u in t['units']:
            body+=f"## {u['id']}\n{u['title']}\n\nФайлы: {', '.join(u['files']) or 'уточнить --prepare; выполнение заблокировано до явного списка'}\nВременная mutation scope (только red, затем восстановление): {', '.join(u.get('mutation_files',[])) or 'только files'}\nКонтекст (только чтение): {', '.join(u.get('read_files', [])) or 'источники задачи выше'}\nТест: {u['test']}\nПредложенная команда: `{u['accept'] or 'уточнить --prepare; пустая приёмка запрещена'}`\n\n"
        body+='## Исходная спецификация (историческая; при конфликте действует политика выше)\n'+t.get('spec','')+'\n'
        (out/(t['id']+'.md')).write_text(body)
    rows=['# fixes — полная очередь кодогенерации MVP1','','Источник исполнения: plan.json; legacy execution/status.json не изменяется.','', 'Запуск и ограничения: [README](README.md). Внешние задачи не исполняются автоматически.','', '| Задача | P | Режим | Подзадач | Источник |','|---|---|---|---|---|']
    for t in tasks.values():rows.append(f"| [{t['id']}]({t['id']}.md) | {t['priority']} | {t['mode']} | {len(t['units'])} | {t['source']} |")
    (out/'INDEX.md').write_text('\n'.join(rows)+'\n')
    coverage=['# План E2E: GAP-001…036', '', 'Назначение: tests/e2e. Это спецификация генерации, не отчёт PASS.', '', '| GAP | Логика Given / When / Then | Пакет |', '|---|---|---|']
    for n,logic in enumerate(GAP_LOGIC,1):
        gap=f'GAP-{n:03d}';coverage.append(f'| {gap} | {logic} | [E2E-{gap}](E2E-{gap}.md) |')
    coverage+=['', 'Общие требования: реальный production path; fake только внешней границы; lost response после записи; counters на fiscal/purchase/refund; exact operation ID/digest/UNP; crash/restart; deterministic oracle; 3 из 3; нет SKIP/синтетического HIL PASS.', '', 'Дополнительные разрывы: E2EMATRIX-01 (20 маршрутов и capabilities), SUPTO-01 (профили и архив/экспорты), REFUND-01 (crash refund), EVIDENCE-01 (внешний trust anchor), TST-E2E-01…26 (продажа, скидки, НДС, сторно, offline, Smart S, BluePay, POS BLE).']
    (out/'E2E_COVERAGE.md').write_text('\n'.join(coverage)+'\n')
    print(f"generated {len(tasks)} packets, {sum(len(t['units']) for t in tasks.values())} units")

# Extensions below are concrete missing work, not synthetic DONE statuses.
def add_extensions(tasks):
    rows=[
    ('S3FIX-01','Reservation и ошибки storage','durable_storage.cpp intent_processor.cpp','durable_storage_native.cpp','Каждый reserve/write/flush/checkpoint: ошибка до I/O → purchase/fiscalCalls==0. УНП сохраняет 7 цифр. Restart не переиспользует УНП.'),
    ('S3FIX-02','Атомарные terminal state и outbox','durable_storage.cpp intent_processor.cpp','durable_storage_native.cpp','SQLite transaction фиксирует terminal+outbox вместе. FAIL_AT на каждой записи/commit: после restart оба есть либо обоих нет.'),
    ('S3FIX-03','Readiness и ownership до recovery','app_main.cpp profile_executor.cpp','profile_executor_native.cpp','Recovery после запуска UART/USB/BLE, matching binding generation/device serial/UNP. Чужой открытый чек не отменяется.'),
    ('S3FIX-04','Индивидуальные карточные платежи','intent_processor.cpp payment_reconciliation.cpp','intent_processor_native.cpp','Два CARD с разными payment_id: отдельные RRN/AuthID/STAN, частичный сбой и restart; успешную покупку/refund не повторять.'),
    ('S3FIX-05','Immutable sync batch и canonical hash','sync_runtime.cpp durable_storage.cpp','sync_runtime_native.cpp','Одинаковые C++/Go bytes/hash; in-flight batch ID/hash/content неизменны при новых событиях, reboot и повторной отправке; неверный ACK ничего не удаляет.'),
    ('S3FIX-06','Expiry и bounded storage','command_queue.cpp durable_storage.cpp','command_queue_native.cpp','Просроченная команда → отказ до I/O; заполнение SD не удаляет unacked evidence и безопасно блокирует новую денежную операцию; проверить TTL/лимиты и restart.'),
    ('S3FIX-07','Bootstrap trust и BLE изоляция','ble_runtime.cpp mqtt_runtime.cpp','ble_runtime_native.cpp','Onboarding доставляет trust/CA, Wi-Fi, binding и secrets; factory reset→restart→trusted MQTT. Adv budget ≤31 bytes, MTU23/247, чужая session/origin отклоняется.'),
    ]
    for tid,title,code,test,criterion in rows:
        base='IoT/firmware/edge-agent-s3/idf/'
        files=[base+'main/'+v for v in code.split()]+[base+'tests/'+test]
        tasks[tid]={'id':tid,'priority':'P0','mode':'code','source':'TRACE_EXISTING_REGISTERS.md','source_status':'NEW','depends':[], 'spec':criterion,'notes':'Новая явная реализация ранее транзитивного S3 GAP. До HIL; host proof отдельно от hardware evidence.', 'units':[{'id':tid+'.1','title':title+'. '+criterion,'files':files,'sources':['TRACE_EXISTING_REGISTERS.md','ARCH_19_HIL_SCENARIOS.md'],'test':criterion,'accept':'make iot-test','regression':['make iot-test'],'phase':'green'}]}
    extra=[
    ('PERSIST-01','Транзакционная реализация D-3: нормализованные operation/event/UNP/idempotency/outbox, WithTx; миграции up/down/up; не ограничиваться проверкой старого snapshot.', ['fiscal-backend/internal/persistence','fiscal-backend/internal/domain/repository.go','database/fiscal','fiscal-backend/internal/domain/repository_contract_test.go'], ['TG-001'], 'cd fiscal-backend && bash ../scripts/postgres-integration.sh'),
    ('MATRIX-01','Каноническая JSON-матрица: 5 аппаратных путей × POS/MiniPOS × MQTT/BLE =20 маршрутов; S1..S8 по capabilities. Compact CARD/split — негатив, не денежный успех. СУПТО поперёк профилей. Генератор HIL/E2E читает JSON. BLOCKED/SKIP/нет evidence → fail. Selftest: удалить POS BLE строку → fail. Физический PASS не генерировать.', ['docs/MVP1/fixtures/mvp_matrix.json','scripts/check-hil-matrix.py','scripts/tests/test_hil_matrix.py'], [], 'python3 -m unittest discover -s scripts/tests -p test_hil_matrix.py'),
    ('SUPTO-01','Профильная трассировка СУПТО включая Smart S. Создать тесты операторов/прав, журнал UI фильтров, полный event catalog, экспортов 18.1..18.9, архив backup/restore. E-чек выключен: тест отказа. Юридические ответы не выдумывать.', ['fiscal-backend/internal/domain/supto_profile_test.go','fiscal-backend/internal/api/supto_profile_test.go','BeeFiscalApp/src/__tests__/auditFilters.test.ts','docs/MVP1/fixtures/supto_profiles.json'], [], 'cd fiscal-backend && go test -json ./internal/domain ./internal/api -run SuptoProfile -count=1'),
    ('REFUND-01','Durable refund recovery для BC-50/S3/Smart S: kill после storno, после refund send, до refund ACK; lookup по payment_id, запрет повторного сторно/успешного refund. Smart S без команды возврата → REFUND_MANUAL_REQUIRED; тесты multi-CARD.', ['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt','SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/RefundRecoveryTest.kt','IoT/firmware/edge-agent-s3/idf/main/payment_reconciliation.cpp','IoT/firmware/edge-agent-s3/idf/tests/payment_reconciliation_native.cpp'], ['BUG-071'], 'cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*RefundRecoveryTest*" --rerun-tasks'),
    ('EVIDENCE-01','Factory manifest различает app/merged image; валидатор HIL использует внешний trust store, не ключ из bundle. Tests: подменённый ключ, битый hash, неверный тип образа → fail; валидный доверенный fixture → pass.', ['IoT/device-manufacturing','IoT/hil','scripts/tests/test_evidence_trust.py'], [], 'python3 -m unittest discover -s scripts/tests -p test_evidence_trust.py'),
    ('E2EMATRIX-01','Параметризованные EMU E2E по каждой применимой ячейке mvp_matrix.json. Ключ route.path/client/transport/scenario; контракты, checksum, CP1251, money, duplicate, UNKNOWN lookup, restart. Создать Make target; каждый E2E 3/3, counters assert. Smart S LAB/PROD и Compact negative explicit; не эмулировать физический HIL PASS.', ['tests/e2e','tests/hil_emu','Makefile'], ['MATRIX-01','EMU-01','EMU-02','EMU-03','EMU-04','EMU-05','EMU-06'], 'make e2e-local'),
    ]
    for tid,title,files,deps,accept in extra:
        tasks[tid]={'id':tid,'priority':'P0','mode':'code','source':'MVP_GOAL.md','source_status':'NEW','depends':deps,'spec':title,'notes':'Явное закрытие разрыва ревью; реальные внешние gates остаются открыты.', 'units':[{'id':tid+'.1','title':title,'files':files,'sources':['MVP_GOAL.md','DECISIONS.md'],'test':title,'accept':accept,'regression':regression(files),'phase':'green'}]}
    tasks['TG-001']['depends'].append('PERSIST-01')
    tasks['PERSIST-01']['depends']=[]
    tasks['SCR-40']['depends']=['MATRIX-01']
    for tid in ['TST-E2E-25','TST-E2E-26','TST-E2E-16','NAP-P-01','TG-003','VG-11','VG-21','HILMX-01','SCR-40']:
        tasks[tid]['notes']+=' Нормативный override: '+POLICY
    tasks['E2EMATRIX-01']['depends'] += ['S3FIX-01','S3FIX-02','S3FIX-03','S3FIX-04','S3FIX-05','S3FIX-06','S3FIX-07']


def add_e2e_gaps(tasks, mvp):
    # One executable scenario module for every known GAP, including explicit exclusions.
    for line in (mvp/'TRACE_EXISTING_REGISTERS.md').read_text().splitlines():
        if not line.startswith('| GAP-'):continue
        cols=[v.strip() for v in line.strip('|').split('|')]
        gap,summary=cols[:2];tid='E2E-'+gap
        owners=[v for v in re.findall(r'\b(?:BUG|TG|TST)-[A-Z0-9-]+\b',cols[3]) if v in tasks]
        evidence='tests/e2e/evidence/'+gap.lower()
        test='tests/e2e/gaps/test_'+gap.lower().replace('-','_')+'.py'
        logic=GAP_LOGIC[int(gap.split('-')[1])-1]
        title=f'{gap}: {summary}. Given/When/Then: {logic}'
        tasks[tid]={'id':tid,'priority':'P0','mode':'code','source':'TRACE_EXISTING_REGISTERS.md','source_status':'NEW','depends':list(dict.fromkeys(owners)), 'spec':title,'notes':'Реальный production adapter в тестовом окружении + управляемый fake boundary. Не моделировать продукт внутри теста. Если runtime нельзя запустить — integration harness и явный BLOCKED, не mocked PASS. Счётчики side effects, evidence и 3 последовательных запуска обязательны.', 'units':[{'id':tid+'.1','title':title+' Написать исполняемый unittest/harness, описание логики в tests/e2e/COVERAGE.md, fixtures и запуск в CI; не оставлять pass/TODO/skip.','files':[test,'tests/e2e/gaps/support','tests/e2e/COVERAGE.md','tests/e2e/fixtures/'+gap.lower(),'Makefile'],'sources':['TRACE_EXISTING_REGISTERS.md','ARCH_19_HIL_SCENARIOS.md'],'test':'assert исход/суммы/ключи/ровно-один side effect; fault case проверяет запрет второго эффекта; три одинаковых прогона; HIL отдельно.','accept':f'python3 -m unittest discover -s tests/e2e/gaps -p test_{gap.lower().replace("-","_")}.py','regression':[],'repeat':3,'phase':'green','require_runtime':True}]}

GAP_LOGIC=[
'MQTT backend publish -> реальный ingress S3/Android; exact tenant/device topic, msg_id; один результат и документ; неправильный namespace не принимается.',
'S3 batch отправлен, business ACK доставлен в реальный ACK handler; checkpoint двигается один раз; дубликат ACK не удаляет новые события; неверный hash отвергается.',
'BeeFiscalApp onboarding -> fake BLE peripheral с production BFPE parser; authenticate, Wi-Fi, binding, restart; крипто-заглушка запрещена.',
'Factory reset -> CA/trust/network/secrets -> TLS broker -> READY; неверный CA или потеря trust записи блокирует деньги; проверить restart.',
'Provisioning payload больше MTU23: фрагментация и сборка без oversized write; потеря/перестановка фрагмента не применяет partial config.',
'Production advertising builder сериализует реальные поля; payload <=31 bytes, scan response разрешён; scanner находит UUID/name; overflow -> отказ.',
'MTU23/247, payload1KB через production BLE codec; exact bytes; lost fragment -> timeout/UNKNOWN после записи, без повторного денежного send.',
'Два BLE клиента/сессии: чужой origin/proof, stale nonce, replay и переключение владельца; ни одного unauthorized execute, законная сессия работает.',
'Daisy production codec + 17 golden frames, Cyrillic и STAT; bad BCC/length отвергаются; full cash receipt через эмулятор USB.',
'Datecs Kotlin/C++ production codec: один oracle LEN=data+0x2A, TX/RX/BCC/status и битые кадры; несоответствие хотя бы байта ломает тест.',
'Serial SYN, NAK, late frame, silent deadline: byte-identical resend только по протоколу; timeout after write UNKNOWN; no double document.',
'100+ Datecs и 225+ Daisy команд, wrap/restart/cache; корректные SEQ/CP1251/slots, новая команда не получает cached чужой ответ.',
'BluePad wake/flow/chunks production adapter: completion каждого chunk; disconnect между chunks -> UNKNOWN/NotSent по реальной границе записи.',
'BluePad два operation IDs, reordered/late responses и STAN/RRN/AuthID; чужой result не завершает операцию; missing STAN -> unresolved/manual.',
'S3 negative price/qty/tax/discount/payment sum: rejected до reservation/card/fiscal I/O; golden1367 проходит.',
'Reserve -> kill -> reboot с той же SD DB; тот же UNP с 7-digit sequence, тот же operation ID; нет повторной покупки/печати.',
'Каждый storage write/flush/checkpoint возвращает ошибку по очереди; денежные calls=0 после сбоя; prior committed evidence сохранено.',
'Kill между terminal write/outbox write/commit; после рестарта оба либо ни одного; delivery повторяется, device execution нет.',
'2 CARD + CASH, отдельные payment IDs; один approved, другой timeout, reboot; evidence и компенсация индивидуальны, approved payment не повторяется.',
'Recovery запущен при USB/UART not ready и затем чужом open receipt; ждёт readiness/ownership; чужой чек не отменяется.',
'ФУ success с реальным номером и success без номера; первый сохраняет evidence, второй UNKNOWN; synthetic reference никогда не FISCALIZED.',
'Один sync event C++ -> Go: exact canonical bytes/hash, Cyrillic/quotes/newline/amount, порядок map не меняет digest.',
'Batch отправлен, ACK потерян, новые события добавлены и reboot; resend exact batch ID/hash/body; ACK удаляет только исходный batch.',
'Все команды и results по schema backend/S3; sale ID vs operation ID, UNKNOWN и device association сохраняются; invalid envelope -> structured refusal.',
'Expired command, full queue/SD и retention cutoff; expired не исполняется, unacked не вытесняется; bounded storage и безопасный отказ.',
'MiniPOS Web после submit теряет ответ, переключает MQTT/BLE; только lookup тем же key/digest, submitCalls==1.',
'Native checkout: kill/background после durable outbox и после BLE write; restart/resume/sync не повторяют документ/покупку и сохраняют pending.',
'MiniPOS production backend PostgreSQL full regression с webhook-before-link, dispatch timeout, restart; no downgrade SUCCEEDED/no lost result.',
'Чистая сборка Android production composition и startup runtime service; реалистичный lifecycle; README commands исполняются в CI, demo/stub не попадают в prod.',
'EMU evidence имеет явный kind=EMU; HIL verifier отказывает при предъявлении EMU вместо physical, отсутствии фото/raw logs/device firmware; HIL PASS не синтезировать.',
'Manufacturing flash manifest app vs merged: адреса/partition/hash; неверный тип образа отклоняется до flash; физическую прошивку не выполнять.',
'Tremol вне MVP: capability/route запрещены явным structured error до I/O; не реализовывать неподтверждённый Tremol adapter.',
'Smart S production HTTP port + FakeSmartFiscalServer/Bridge: деньги из cents, drop after body UNKNOWN без второй печати; LAB без номера, PROD UNKNOWN; refund manual.',
'Подписанный evidence bundle с ключом из самого bundle; незнакомый ключ отвергается внешним trust store; доверенный fixture принимается, tamper отвергается.',
'Подпись результата валидна, но tenant/device/binding generation чужие или устарели; result не применяется; верный immutable association применяется один раз.',
'Device reference содержит quotes, backslash, Cyrillic и длинную строку; production JSON serializer не обрезает/не портит; restart сохраняет exact reference.'
]

def complete_units(tasks, root):
    # Legacy bare paths and empty accept fields become explicit generation contracts.
    for t in tasks.values():
        if t.get('alias_of') or t['mode']=='external':continue
        for u in t['units']:
            files=[]
            for f in u['files']:
                if f.startswith('internal/'):f='fiscal-backend/'+f
                if f in ['service.go','repository.go','handler.go']:
                    f='fiscal-backend/internal/'+('api/' if f=='handler.go' else 'domain/')+f
                files.append(f)
            if not files:
                if t['id'].startswith(('TST-KT','TST-VENDOR')):files=['SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/'+t['id'].replace('-','')+'Test.kt']
                elif t['id'].startswith('TST-POS'):files=['../POS/pos-app/src/__tests__/'+t['id'].lower()+'.test.ts']
                elif t['id'].startswith('TST-S3'):files=['IoT/firmware/edge-agent-s3/idf/tests/'+t['id'].lower().replace('-','_')+'.cpp']
                elif t['id'].startswith('TST-INT'):files=['tests/e2e/integration/test_'+t['id'].lower().replace('-','_')+'.py']
                elif t['id'].startswith('TST-CT'):files=['tests/e2e/contracts/test_'+t['id'].lower().replace('-','_')+'.py']
                elif t['id'].startswith('TST-GO'):files=['fiscal-backend/internal/domain/'+t['id'].lower().replace('-','_')+'_test.go']
                else:files=['docs/MVP1/specs/'+t['id']+'.md']
            u['files']=list(dict.fromkeys(files))
            # File generation alone is never a green acceptance criterion for code.
            cmd=u.get('accept','')
            weak=not cmd or bool(re.search(r'(^|&&)\s*(ls |grep |rg |test |jq |! |.*wc -l)',cmd)) or cmd.endswith('--rule')
            suites=regression(u['files'])
            if weak:
                if suites:cmd=' && '.join('('+c+')' for c in suites)
                elif any(f.endswith('.py') for f in files):
                    f=next(f for f in files if f.endswith('.py'));cmd='python3 -m unittest discover -s '+str(Path(f).parent)+' -p '+Path(f).name
                else:
                    # Documentation/test metadata have a distinct content validator, never product PASS.
                    u['artifact_only']=True
                    cmd='python3 docs/MVP1/fixes/check_artifacts.py '+t['id']+' --unit '+u['id']
            u['accept']=cmd
            u['regression']=suites
            # A generation task with only tests still has to run that test and demonstrate mutation sensitivity.
            if u.get('phase')=='red' and not re.search(r'--tests| -run ',cmd):u['phase']='green'
            if t['id'].startswith('TST-E2E') or t['id'].startswith('E2E-'):u['repeat']=3
            if t['id']=='TG-001':u['regression']=['cd fiscal-backend && go test -json ./... -count=1']
            # Ensure source test destinations are allowed, including test-first subtasks.
            if re.search(r'сначала тест|тест.*красн|тест-характер',u['title'],re.I):
                if any(f.startswith('fiscal-backend/') for f in files):u['files'].append('fiscal-backend/internal/domain/'+t['id'].lower().replace('-','_')+'_test.go')
                if any(f.startswith('SmartDevices/') for f in files):u['files'].append('SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash')
            # Red-only work is executed with the following implementation in one bounded session.
        merged=[];pending=[]
        for u in t['units']:
            if u.get('phase')=='red':pending.append(u);continue
            if pending:
                u=dict(u);u['id']=pending[0]['id']+'-through-'+u['id'].split('.')[-1]
                u['title']='\n'.join(v['title'] for v in pending+[u])
                u['files']=list(dict.fromkeys(f for v in pending+[u] for f in v['files']))
                u['test']='Сначала обязательный воспроизводящий red, затем green: '+'; '.join(v['test'] for v in pending+[u])
                u['phase']='green';pending=[]
            merged.append(u)
        for u in pending:u['phase']='green';u['test']+='; генерация теста: подтвердить чувствительность мутацией.';merged.append(u)
        t['units']=merged


def execution_contracts(tasks, root):
    # Concrete execution scope; no synthetic documentation substitute for code work.
    module_map={
      'ARCH-006':['fiscal-backend/internal/domain/service.go','fiscal-backend/internal/domain/driver_selection_test.go'],
      'ARCH-008':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt','SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashComplianceIntent.kt'],
      'ARCH-009':['contracts','../POS/docs/contracts','../POS/pos-gateway/internal/handler','../POS/pos-app/src/infrastructure/runtime'],
      'BUG-044':['IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp','IoT/firmware/edge-agent-s3/idf/tests/intent_processor_native.cpp'],
      'CODE-001':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt'],
      'CODE-003':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt'],
      'CODE-005':['fiscal-backend/internal/domain','fiscal-backend/internal/api'],
      'CODE-006':['fiscal-backend/internal/domain/receipt_saga.go','fiscal-backend/internal/domain/receipt_plan.go'],
      'CODE-007':['fiscal-backend/internal/domain'],
      'POS-09':['../POS/.gitignore','scripts/check-no-secrets.sh'],
      'POS-10':['../POS/pos-app/src/shared/utils/decimal.ts','../POS/pos-app/src/domain/calculation'],
      'POS-02':['../POS/pos-app/src/infrastructure/runtime/Adapters.ts','../POS/pos-app/src/infrastructure/di/container.impl.ts'],
      'POS-05':['../POS/pos-app/src/infrastructure/runtime/Adapters.ts','../POS/pos-gateway/internal/handler'],
      'POS-06':['../POS/pos-app/src/infrastructure/runtime/Adapters.ts'],
      'POS-07':['../POS/pos-app/src/infrastructure','../POS/pos-gateway/internal/handler'],
      'POS-12':['../POS/pos-app/src/infrastructure'],
      'POS-13':['../POS/pos-app/src/infrastructure/runtime','../POS/pos-gateway/internal'],
      'NAP-20':['../POS/pos-gateway/internal/handler','../POS/pos-app/src/infrastructure'],
      'NAP-21':['fiscal-backend/internal/domain/exports.go'],
      'VG-15':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt'],
      'STYLE-003':['SmartDevices','IoT/protocol-abstraction','Makefile'],
      'W0-1':['docs/MVP1/baseline','scripts/mvp1-baseline.sh'],
      'W0-2':['docs/MVP1/fixtures/plan_coverage.json','scripts/check_plan_coverage.rb'],
      'W0-3':['docs/MVP1/test-matrix.tsv','scripts/check-test-matrix.sh'],
    }
    module_map.update({
      'TG-005':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt'],
      'TG-006':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt','SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsResponsesTest.kt'],
      'POS-03':['../POS/pos-gateway/internal/drain','../POS/pos-gateway/internal/handler','../POS/pos-app/src/infrastructure/runtime'],
      'POS-04':['../POS/pos-app/src/infrastructure/supto','fiscal-backend/internal/domain'],
      'NAP-03':['fiscal-backend/internal/domain','SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt'],
      'NAP-08':['fiscal-backend/internal/domain','fiscal-backend/internal/api','database/fiscal'],
      'NAP-10':['fiscal-backend/internal/api','fiscal-backend/internal/auth'],
      'NAP-11':['fiscal-backend/internal/domain/exports.go'],
      'NAP-12':['fiscal-backend/internal/domain/unp_allocator.go'],
      'NAP-13':['fiscal-backend/internal/domain','contracts/supto-feature-scope.json'],
      'NAP-18':['fiscal-backend/internal/domain/reversal_policy.go'],
      'NAP-23':['contracts/supto-feature-scope.json','fiscal-backend/internal/domain'],
      'TG-011':['fiscal-backend/internal/api','docs/runbooks/ALERTS.md'],
      'VG-18':['contracts/supto-feature-scope.json'],
      'VG-21':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsClock.kt'],
      'VG-22':['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt'],
    })
    for tid,t in tasks.items():
        if t.get('alias_of') or t['mode']=='external':continue
        all_files=list(dict.fromkeys(f for u in t['units'] for f in u['files']))
        for u in t['units']:
            files=[]
            inherited=[f for f in all_files if not f.startswith('docs/MVP1/specs/') and f not in ('то же','—')]
            for f in u['files']+inherited+module_map.get(tid,[]):
                if f.startswith('Fiscalisation/'):f=f[len('Fiscalisation/'):]
                if f.startswith(('pos-app/','pos-gateway/','erp-connector/')):f='../POS/'+f
                if f.startswith('drain/'):f='../POS/pos-gateway/internal/'+f
                if f=='LocalReceiptOperations.ts':f='../POS/pos-app/src/application/workflows/LocalReceiptOperations.ts'
                if f=='NNN_sale_import_evidence.sql':f='database/fiscal'
                if f=='reconciler.go':f='fiscal-backend/internal/domain/reconciler.go'
                if f=='electronic_receipt.go':f='fiscal-backend/internal/domain/electronic_receipt.go'
                if f=='docs/MVP1/ARCH_06_SUPTO_NAP_REQUIREMENTS.md':f='docs/MVP1/ARCH_06_SUPTO_NAP_READINESS.md'
                if '<date>' in f:f=f.replace('<date>','release')
                if '*' in f or f.startswith(('TST-E2E-','AndroidTransactionJournal.')):continue
                if f.startswith('docs/MVP1/specs/') and (tid in module_map or inherited):continue
                if f.startswith('public-api-core/'):f='../BeeloyBackend/'+f
                explicit_moves={
                    'tests/host/DatecsCodecVendorHost.kt':'SmartDevices/bluecash-app/tests/host/DatecsCodecVendorHost.kt',
                    'docs/remediation/2026-10-17/MVP_GATES.md':'MVP_GATES.md',
                    'tools/extract_vendor_frames.py':'docs/MVP1/tools/extract_vendor_frames.py',
                    'IoT/firmware/edge-agent-s3/idf/tests/sync_runtime_native.cpp':'IoT/firmware/edge-agent-s3/idf/tests/sync_host/sync_runtime_native.cpp',
                }
                f=explicit_moves.get(f,f)
                if f.startswith('idf/tests/'):f='IoT/firmware/edge-agent-s3/'+f
                if f.startswith('fixtures/'):f='docs/MVP1/'+f
                if f=='../POS/pos-app/src/infrastructure/supto/LocalReceiptOperations.ts':f='../POS/pos-app/src/application/workflows/LocalReceiptOperations.ts'
                files.append(f.rstrip('/'))
            # Fix malformed test-only Go paths from the table's abbreviated names.
            if tid.startswith('TST-GO-'):
                files=[f for f in files if not f.startswith('internal/')]
            verify='tests/mvp1/test_'+u['id'].lower().replace('-','_').replace('.','_')+'.py'
            u['legacy_accept']='' if 'check_artifacts.py' in u['accept'] else u['accept']
            u['accept']='python3 -m unittest discover -s tests/mvp1 -p '+Path(verify).name
            u['files']=list(dict.fromkeys(files+[verify]))
            u['phase']='green';u.pop('artifact_only',None)
            u['title']+='\nПриёмочный тест '+verify+': unittest вызывает реальный код/нативный тестовый runner/скрипт (subprocess без shell), проверяет точные ожидаемые результаты из спецификации и ненулевой счётчик тестов. Для документной работы валидирует структуру и требуемые поля, не только наличие файла. Не использовать always-true, mocked production function, skip, чтение ожидаемого из реализации. Сначала test-first и чувствительность к дефекту, затем минимальная правка. Старую команду legacy_accept сверить с реальной структурой и запускать из harness только если она содержательно проверяет эту подзадачу. Scope точный: путь файла разрешает только этот файл; путь каталога — его потомков. Новый файл допустим только внутри явно указанного каталога, не рядом с разрешённым файлом. Если нужного пути нет в files — BLOCKED_SCOPE, не создавать его.'
            # Remove coarse duplicate regression of unrelated apps; native behavior checked by harness.
            u['regression']=[]
            if tid.startswith('E2E-GAP-'):
                test='tests/e2e/gaps/test_'+tid[4:].lower().replace('-','_')+'.py'
                u['accept']='python3 -m unittest discover -s tests/e2e/gaps -p '+Path(test).name
                u['files']=[f for f in u['files'] if f!=verify]
                u['title']=u['title'].split('\nПриёмочный тест')[0]
                u['repeat']=3
        # Verification-only steps need access to the actual implementation/test files from this task.
        for u in t['units']:
            if not any(f.endswith(('.go','.kt','.cpp','.ts','.tsx','.sh','.rb')) for f in u['files']) and tid in module_map:
                u['files']=list(dict.fromkeys(u['files']+module_map[tid]))
    # Existing script packets provide the executable prerequisites; never wait on an absent legacy SCR ID.
    prerequisites={'W0-2':['SCRIPT-01'],'W0-3':['SCRIPT-03'],'TG-004':['BUG-028','BUG-029'],'NAP-15':['SCRIPT-29'],'DOC-002':['SCRIPT-15'],'DOC-003':['SCRIPT-36'],'TG-015':['SCRIPT-06'],'TG-014':['SCRIPT-30'],'TG-016':['SCRIPT-31']}
    for tid,deps in prerequisites.items():tasks[tid]['depends']=list(dict.fromkeys(tasks[tid]['depends']+deps))
    # Do not execute legal research/network checks or destructive key rotation as code generation.
    for tid in ['NAP-N0','NAP-22','VG-14','SCOPE-01']:
        tasks[tid]['mode']='external';tasks[tid]['units']=[]
    # Fine-grained E2E implementation and executable gap coverage share one reviewed logic index.
    tasks['E2EMATRIX-01']['depends']+=['E2E-GAP-'+str(i).zfill(3) for i in range(1,37)]

if __name__=='__main__':
    a=argparse.ArgumentParser();a.add_argument('--root',type=Path,default=Path(__file__).resolve().parents[3]);a.add_argument('--out',type=Path);args=a.parse_args();build(args.root,args.out or args.root/'docs/MVP1/fixes')
