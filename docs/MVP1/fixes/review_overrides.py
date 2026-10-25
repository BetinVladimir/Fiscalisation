"""Reviewed execution/coverage corrections. Kept separate from legacy card extraction."""
from copy import deepcopy


def fix_plan(tasks):
    for tid in ['BP-06','CL-01']:
        tasks[tid]['mode']='external';tasks[tid]['units']=[]
    # Summary tasks may aggregate work, but may never write protected legacy status.
    tasks['TG-017']['alias_of']='VG-20'
    tasks['TG-017']['depends']=[t for t in tasks if t.startswith('VG-') and t!='VG-18']
    tasks['TG-017']['units']=[]
    tasks['TG-017']['spec']='Сводный gate вендорных работ; не редактировать status.json. Успех требует все depends, не только VG-20.'
    tasks['BUG-060']['external_requires']=['SECURITY-ROTATION-01']
    tasks['SECURITY-ROTATION-01']={'id':'SECURITY-ROTATION-01','mode':'external','priority':'P0','source':'DECISIONS.md','source_status':'NEW','depends':[],'external_requires':[],'units':[],'notes':'Владелец: ротация реально раскрытых ключей/паролей и решение о git history. Кодогенерация создаёт только scan/конфигурацию. Не выдумывать подтверждение ротации.','spec':'Письменное evidence ротации без секретов; независимая проверка. Не выполнять на реальных credentials автоматически.'}
    tasks['AA-01']['spec_override']=True
    tasks['AA-01']['spec']='Hermes-safe AdminApp idempotency: expo-crypto ~15.0.9; newKey() delegates to Expo randomUUID. Execute ONLY the selected unit. AA-01.1 creates and tests the helper; AA-01.2 migrates App.tsx call sites in its own later session. Never create another unit acceptance test.'
    first=tasks['AA-01']['units'][0]
    first['files']=[p for p in first['files'] if p!='AdminApp/App.tsx']
    first['title']+=' STOP after helper acceptance: do not edit App.tsx, do not begin AA-01.2, do not create test_aa_01_2.py. The controller schedules the next unit.'
    # AdminApp has no Jest configuration; adding dependencies also requires its lock.
    for unit in tasks['AA-01']['units']:
        unit['files']=list(dict.fromkeys(unit['files']+['AdminApp/package-lock.json','AdminApp/jest.config.js']))
        unit['title']+=' AdminApp Jest setup and package-lock.json are explicitly scoped: keep manifest/lock consistent. Use local npx --no-install jest --runInBand. In the native test, mock only the Expo SDK boundary, never newKey: jest.mock expo-crypto randomUUID delegates to jest.requireActual(\"node:crypto\").randomUUID, not a constant. Set global.crypto undefined before calling the real newKey, verify UUID v4, distinct keys, and SDK invocation. A constant SDK mock makes the uniqueness test impossible to pass and is forbidden. AA-01.1 tests the helper; App.tsx call-site migration belongs to AA-01.2.'
    add_files={
      'CL-02':['minipos/miniposweb/src/fiscal','minipos/BeeMiniPOS/src'],
      'BUG-051':['../POS/.env.example'],
      'NAP-04':['fiscal-backend/internal/domain','fiscal-backend/internal/api','../POS/pos-gateway/internal/handler'],
      'POS-11':['fiscal-backend/internal/domain/unp_allocator.go','../POS/pos-app/src/application/workflows','../POS/pos-gateway/internal/handler'],
      'SCOPE-02':['minipos/beeminipos-backend/internal/domain/service.go'],
      'SCOPE-03':['minipos/BeeMiniPOS/App.tsx'],
      'SCOPE-04':['../POS/erp-connector/internal'],
      'SCOPE-05':['../BeeloyBackend/public-api-core/internal/adminerp'],
      'SCOPE-06':['BeeFiscalApp/App.tsx'],
      'SMS-02':['SmartDevices/shared-runtime/src/main/kotlin','SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash','SmartDevices/daisy-smart-app/src/main/kotlin'],
      'SUPTO-01':['fiscal-backend/internal/domain','fiscal-backend/internal/api','BeeFiscalApp/src'],
    }
    for tid,files in add_files.items():
        for u in tasks[tid]['units']:u['files']=list(dict.fromkeys(u['files']+files))
    for t in tasks.values():
        for u in t['units']:
            if t['id'].startswith('TST-E2E-'):
                # Legacy TS/Go paths are optional adapters; canonical E2E lives in requested root.
                base='tests/e2e/scenarios/test_'+t['id'].lower().replace('-','_')+'.py'
                harness=next((f for f in u['files'] if f.startswith('tests/mvp1/')),None)
                u['files']=[f for f in u['files'] if f!=harness]+[base,'tests/e2e/scenarios/support']
                u['accept']='python3 -m unittest discover -s tests/e2e/scenarios -p '+base.rsplit('/',1)[1]
                u['title']+=' Канонический Python E2E в '+base+' вызывает реальный runtime через harness; специфичный driver может быть TS/Go. Не подменять E2E Python-моделью продукта.'
                u['repeat']=3
            if t['id'] in ('TST-E2E-16','TG-008','NAP-13','NAP-23'):
                u['title']='E-чек вне MVP по D-6/NORM-14: реализовать технический запрет и negative тест. Отправку email/электронного бона не реализовывать. '+u['title']
            if t['id'] in ('TST-E2E-25','TST-E2E-26'):
                u['title']='Compact CARD/split: negative без I/O; сторно/refund только для capabilities профиля. СУПТО не аппаратный путь. '+u['title']
    # Split large implementation packets by transaction/runtime boundary, not by arbitrary token clipping.
    def split(tid,rows):
        seed=tasks[tid]['units'][0];units=[]
        for i,(title,files,criterion) in enumerate(rows,1):
            u=deepcopy(seed);u['id']=tid+'.'+str(i)
            verify='tests/mvp1/test_'+tid.lower().replace('-','_')+'_'+str(i)+'.py'
            u['files']=files+[verify];u['title']=title+'; test-first: '+criterion+'. Реализовать конкретную часть продукта; oracle не брать из реализации. Читать по 3 файла диапазонами, менять до 400 строк за итерацию.'
            u['test']=criterion;u['accept']='python3 -m unittest discover -s tests/mvp1 -p '+verify.rsplit('/',1)[1];u['regression']=[]
            units.append(u)
        tasks[tid]['units']=units
    split('PERSIST-01',[
      ('Миграции нормализованных operation/event/UNP/idempotency/outbox и индексов',['database/fiscal','fiscal-backend/internal/persistence/repository_schema_test.go'],'чистая БД и предыдущая версия: up/down/up, UNIQUE на tenant+UNP/key, данные предыдущей версии сохранены'),
      ('Postgres WithTx и атомарная фиксация',['fiscal-backend/internal/persistence','fiscal-backend/internal/domain/repository.go'],'operation,event,UNP,replay,outbox фиксируются вместе; injected failure каждого шага rollback без partial state'),
      ('Транзакционное резервирование операции и УНП',['fiscal-backend/internal/domain','fiscal-backend/internal/persistence'],'ReserveUNP/reserve сохраняют operation+UNP одной транзакцией; rollback не оставляет частичное состояние, counter не переиспользуется после restart'),
      ('Атомарное завершение операции, event и outbox',['fiscal-backend/internal/domain','fiscal-backend/internal/persistence'],'CommitOperationEvent сохраняет terminal+event+outbox вместе; kill на каждой границе и повтор delivery не повторяют ФУ'),
      ('Replay/idempotency и конкурентная финализация',['fiscal-backend/internal/domain','fiscal-backend/internal/api'],'ClaimReplay/PutReplay/ambiguous outcome в одной durable модели; 20 same-key concurrent finalize: один документ/operation/UNP, другой payload конфликт'),
      ('Wiring и исключение snapshot из денежных путей',['fiscal-backend/cmd/fiscal-backend','fiscal-backend/internal/domain','fiscal-backend/internal/persistence'],'prod без БД fail-closed; миграция старых данных сохраняет evidence; реальные PG restart/CAS и все денежные flows используют новый transaction API вместо snapshot как источника истины'),
    ])
    split('S3FIX-07',[
      ('Bootstrap trust/CA/network/binding',['IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp','IoT/firmware/edge-agent-s3/idf/main/mqtt_runtime.cpp','IoT/firmware/edge-agent-s3/idf/tests/ble_runtime_native.cpp'],'factory reset->onboarding->restart->trusted MQTT; неверный CA и failed persistence блокируют READY'),
      ('Advertising budget и fragmentation',['IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp','IoT/firmware/edge-agent-s3/idf/tests/ble_runtime_native.cpp'],'advertisement <=31 bytes, service discoverable; MTU23/247, потеря фрагмента не применяет partial config'),
      ('Session/owner/origin isolation',['IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp','IoT/firmware/edge-agent-s3/idf/tests/ble_runtime_native.cpp'],'два concurrent central, чужой proof/origin, stale nonce/replay: unauthorized execute=0, валидный поток работает'),
    ])
    split('SUPTO-01',[
      ('Операторы/права и event catalog по всем профилям',['fiscal-backend/internal/domain','fiscal-backend/internal/api','docs/MVP1/fixtures/supto_profiles.json'],'BC50,Datecs-S3,Daisy-USB,Smart-S: disabled operator/read-only auditor; login/logout/catalog/storno/cancel audit; unsupported пункт явно обоснован'),
      ('Журнал UI и фильтры',['BeeFiscalApp/src','BeeFiscalApp/App.tsx'],'оператор/период/ФУ/UNP фильтруют реальные записи API, пагинация не теряет данные; auditor не меняет состояние'),
      ('Экспорты и архив restore',['fiscal-backend/internal/domain','fiscal-backend/internal/persistence','scripts/tests','docs/MVP1/fixtures/supto_profiles.json'],'экспорты18.1..18.9 со всеми фильтрами, валюта периода, архив backup->restore сохраняет audit/UNP/evidence; e-чек отказ'),
    ])
    # Offline E2E infrastructure is itself a generation task before dependent scenarios.
    tasks['E2E-HARNESS-01']={'id':'E2E-HARNESS-01','priority':'P0','mode':'code','source':'ARCH_20_EMULATOR_SPECS.md','source_status':'NEW','depends':[],'units':[{'id':'E2E-HARNESS-01.1','title':'Создать subprocess/in-process integration support: запуск реальных Go/Kotlin/C++/TS runtime adapters с управляемыми fake внешними границами, временные БД/ports, counters, fault injection, cleanup, explicit env failure. Проверить запуск/остановку реального тестового runtime и протокол readiness; missing tool не skip. Не создавать альтернативную Python-реализацию Fiscal.','files':['tests/e2e/gaps/support','tests/e2e/scenarios/support','tests/e2e/test_harness_lifecycle.py'],'test':'Runtime реально стартует и принимает команду; teardown не оставляет процессы/ports; missing executable -> failure, не mocked success.','accept':'python3 -m unittest discover -s tests/e2e -p test_harness_lifecycle.py','sources':['ARCH_20_EMULATOR_SPECS.md'],'phase':'green','repeat':1,'regression':[]}],'notes':'Инфраструктура генерации до сценариев.','spec':'Production adapter boundary must be exercised. Native child test counts проверяются harness; независимое ревью обязательно.'}
    mapping={**{i:['EMU-06'] for i in range(3,9)},9:['EMU-02'],**{i:['EMU-01'] for i in range(10,13)},13:['EMU-04'],14:['EMU-04'],33:['EMU-03']}
    for tid,t in tasks.items():
        if tid.startswith(('E2E-GAP-','TST-E2E-')):
            number=int(tid.rsplit('-',1)[1]);extras=mapping.get(number,['EMU-05']) if tid.startswith('E2E-GAP-') else []
            t['depends']=list(dict.fromkeys(t['depends']+['E2E-HARNESS-01']+extras))
    # Previously resolved decisions are immutable inputs; no model-generated legal/vendor answers.
    for tid in ['NAP-05','DOC-006']:
        for u in tasks[tid]['units']:
            u['files']=[f for f in u['files'] if f not in ('docs/MVP1/DECISIONS.md',)]
    # Code-facing aggregate waits on expanded tasks rather than historical numbered summaries.
    tasks['TG-017']['depends']+=['S3FIX-01','S3FIX-02','S3FIX-03','S3FIX-04','S3FIX-05','S3FIX-06','S3FIX-07']

    for t in tasks.values():
        for u in t['units']:
            u['files']=[f for f in u['files'] if f!='docs/MVP1/execution/status.json']

    for t in tasks.values():
        for u in t['units']:
            u['require_runtime']=t['id'].startswith(('E2E-','TST-E2E-')) or any(f.startswith(('fiscal-backend/','SmartDevices/','IoT/','../POS/','minipos/','../BeeloyBackend/','BeeFiscalApp/')) and not f.endswith('.md') for f in u['files'])
            u['title']+=' Обязательны реально выполненные unittest assertions; skip/expectedFailure/no tests и assertEqual(константа,константа) не принимаются. Для кода запустить реальный native runtime/tests subprocess либо импортировать production Python module. Автостатус только TESTED_REVIEW_REQUIRED; независимое ревью проверяет смысл oracle и фактическое покрытие.'
    baseline=tasks['W0-1']['units'][0]
    baseline['title']='Написать scripts/mvp1-baseline.sh и запустить исходную базовую линию модулей из ARCH_21 §6. Сохранить команды, commit, версии инструментов, rc и вывод в docs/MVP1/baseline. Исходные FAIL/отсутствующие инструменты записать явно; baseline report не означает продуктовый PASS. Приёмочный unittest реально запускает сборщик baseline и проверяет полноту записей, включая отрицательный fixture; не чинить чужие дефекты на этой стадии.'
    baseline['require_runtime']=True
    for t in tasks.values():
        if t['mode']=='code' and t['id']!='W0-1' and not t.get('alias_of'):
            t['depends']=list(dict.fromkeys(['W0-1']+t['depends']))

    for number,title in [(4,'Вынести journal/locking/processor strategy в vendor-neutral shared-runtime; BC50 специфичные payload/port остаются адаптерами. Характеристические тесты фиксируют порядок I/O и durable states до выноса; после выноса те же результаты.'),(5,'Подключить shared-runtime MQTT/BLE/HTTP ingress и foreground lifecycle к обоим приложениям. Smart S vendor HTTP реализуется SMS-03/08, не выдумывать SDK. Два транспорта одновременно дают один execute; restart сохраняет pending, BC50 regression не меняется.')]:
        verify='tests/mvp1/test_sms_02_'+str(number)+'.py'
        tasks['SMS-02']['units'].append({'id':'SMS-02.'+str(number),'title':title,'files':['SmartDevices/shared-runtime/src','SmartDevices/bluecash-app/src','SmartDevices/daisy-smart-app/src',verify],'test':'Общий runtime реально вызывается обоими приложениями, нет дублированного lock/journal; native tests до/после выноса.','accept':'python3 -m unittest discover -s tests/mvp1 -p '+verify.rsplit('/',1)[1],'sources':['ARCH_17_MVP_GOAL_PATHS_GAPS.md'],'phase':'green','repeat':1,'regression':[],'require_runtime':True})

    for t in tasks.values():
        for u in t['units']:
            u['title']+=' Тесты не меняют source tree: временные БД/fixtures в tempfile, отчёты/raw evidence в os.environ["MVP1_ARTIFACT_DIR"] (отдельный каталог каждого из трёх прогонов); не писать generated evidence в git checkout.'

    for t in tasks.values():
        for u in t['units']:
            if any(f.startswith(('scripts/','docs/MVP1/tools/')) and f.endswith(('.py','.sh','.rb')) for f in u['files']):u['require_runtime']=True
    implementations={
      1:['MQ-01','MQ-03'],2:['S3FIX-05'],3:['BF-01','BF-02'],4:['S3FIX-07','BF-01','TG-012'],
      5:['S3FIX-07','MQ-02'],6:['S3FIX-07'],7:['MQ-02','S3FIX-07'],8:['MQ-02','S3FIX-07'],
      9:['DSY-02','DSY-03','VG-17'],10:['S3D-01','S3D-02'],11:['S3D-01','BUG-029','DSY-02'],
      12:['S3D-01','SEQ-01','VG-03','VG-23'],13:['BP-02','BP-04'],14:['BP-03','BP-05'],
      15:['BUG-022','BUG-042','BUG-043','S3FIX-01'],16:['S3FIX-01'],17:['S3FIX-01'],18:['S3FIX-02'],
      19:['S3FIX-04','REFUND-01','BP-05'],20:['S3FIX-03'],21:['VG-07','S3D-01','DSY-04','SMS-04'],
      22:['S3FIX-05'],23:['S3FIX-05'],24:['MQ-03','POS-01','TG-007'],25:['S3FIX-06'],
      26:['CL-02','MP-01','MP-02','MP-03'],27:['CL-02','SCOPE-03','MC-01'],
      28:['MP-01','MP-02','MP-03','MP-04','MP-05','MP-06','MP-07'],29:['BUG-032','SMS-02'],
      30:['MATRIX-01','EVIDENCE-01'],31:['EVIDENCE-01'],32:['VG-18'],
      33:['SMS-03','SMS-04','SMS-05','SMS-06','SMS-07','SMS-08','SMS-09'],
      34:['EVIDENCE-01'],35:['MQ-01','MQ-03','PA-02'],36:['MQ-03','S3D-01'],
    }
    for n,deps in implementations.items():
        t=tasks['E2E-GAP-'+str(n).zfill(3)];t['depends']=list(dict.fromkeys(t['depends']+deps))
    for u in tasks['VG-18']['units']:
        u['title']='Tremol вне MVP_GOAL: только явный capability/route отказ до I/O и negative regression. Не реализовывать Tremol adapter, golden/conformance контур и неподтверждённые vendor команды.'
        u['files']+=['fiscal-backend/internal/domain','IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp'];u['require_runtime']=True
    for tid,files in {
      'MQ-01':['IoT/firmware/edge-agent-s3/idf/main/mqtt_runtime.cpp','SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashMqtt.kt'],
      'MQ-03':['contracts','IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp','SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash'],
    }.items():
        for u in tasks[tid]['units']:u['files']=list(dict.fromkeys(u['files']+files));u['require_runtime']=True
    base=['POS-01','POS-03','BUG-050','BUG-051','BUG-053','BUG-055','BUG-056','BUG-020','BUG-021','BUG-022','BUG-023','BUG-024','BUG-025','BUG-028','BUG-029','BUG-030','BUG-032','BUG-072','TG-001','TG-002','MQ-01','VG-01','VG-02','VG-03','VG-04','VG-07','EMU-01','EMU-05']
    extra={7:['POS-12','CL-02','EC-01'],8:['TG-010'],9:['TG-009','VG-08','BUG-071','NAP-01','NAP-02','NAP-03'],10:['TG-009','VG-08','BUG-071'],11:['TG-009'],12:['BUG-005','VG-21'],13:['VG-12'],14:['VG-12','VG-15','EC-05'],15:['BUG-005','VG-21'],16:['TG-008','NAP-13'],17:['BUG-012'],18:['BUG-012'],19:['CL-02','POS-12'],20:['BUG-053'],21:['POS-BLE-02','MQ-02'],22:['SMS-03','SMS-04','SMS-05','SMS-06','SMS-07','SMS-08','SMS-09'],23:['BP-03','BP-04','BP-05'],24:['PERSIST-01'],25:['VG-23','DSY-07','SMS-07'],26:['REFUND-01','DSY-05','SMS-05','BP-05']}
    for n in range(1,27):
        t=tasks['TST-E2E-'+str(n).zfill(2)]
        t['depends']=list(dict.fromkeys(t['depends']+(base if n<=20 else [])+extra.get(n,[])))
    canonical={
      'VG-03':'Datecs Kotlin: SEQ 0x20..0x7F, после0x7F->0x20. После restart нельзя принять кэш ответа за исполнение новой команды; сохранить/восстановить sequence/session по подтверждённому протоколу. NAK retransmission — byte-identical same SEQ; новые команды не теряются. Daisy0xFF проверяется отдельно SEQ-01. Тесты valid0x7F/wrap/restart/device-cache и invalid0x80..0xFF для Datecs.',
      'VG-17':'Только Daisy Compact/S3/USB: CP1251, кириллические налоговые группы, SYN/NAK и золотые кадры 2.0-4. Smart S — отдельный JSON/HTTP протокол, SDK не требуется; его реализация в SMS-*; вопросы DOC-V4B не выдумывать.',
      'TST-E2E-16':'E-чек выключен в MVP: API/UI/feature-scope запрещают запрос электронного чека; нет email delivery и ложного success. Проверить negative независимо от successful paper fiscal receipt. Ничего не отправлять реальным получателям.',
      'TG-008':'Технически выключить электронные чеки по решению владельца; e-receipt запрос получает typed unsupported, ничего не отправляется; feature-scope/UI/API согласованы. Реализацию доставки не создавать.',
      'NAP-13':'E-чек вне MVP: явное исключение и технический negative gate; отсутствие функции не скрывать как IMPLEMENTED. Реализацию доставки/экспорта24 не создавать до изменения решения.',
      'NAP-23':'Нормативная привязка e-чека остаётся external; в коде проверить выключенный feature и отрицательные API/UI сценарии. Не включать отправку email.',
      'TST-E2E-25':'Матрица слотов по capabilities. BC50 и Smart S LAB: CASH500/CARD867=1367, реальные profile slots из утверждённых fixtures/GetPayments. Compact: CASH1367 положительно, CARD/split отрицательно до I/O. Smart S неопределённый слот в PROD блокируется, LAB явно маркируется. Ожидаемое не берётся из тестируемого маппера.',
      'TST-E2E-26':'Пять аппаратных путей, capability-driven, СУПТО не отдельное ФУ. Для поддерживающих карту: подтверждённое фискальное сторно раньше refund; UNKNOWN сторно -> refundCalls0; refund timeout -> durable pending без второй покупки/сторно. Smart S -> REFUND_MANUAL_REQUIRED. Compact без карты: проверить наличное сторно и CARD отказ, не генерировать карточный чек.',
      'NAP-P-01':'Профильная матрица требований СУПТО по BC50, Datecs/S3, Daisy Compact/USB, Smart S; BluePay учитывается как payment часть фискального пути. УНП/операторы/права/журнал/часы/storno/XZ/архив/экспорты, e-чек disabled. Каждое OK связано с реально выполненным тестом/evidence, не текстовым наличием файла.',
      'SCR-40':'Канонический checker читает docs/MVP1/fixtures/mvp_matrix.json, не считает 48 markdown-заголовков. Матрица содержит20 аппаратный-путь/client/transport маршрутов; сценарии разворачиваются по capabilities. HIL записи: result PASS/FAIL/BLOCKED, kind HIL, device/firmware, evidence. Отсутствующий маршрут/сценарий, BLOCKED, FAIL, SKIP, EMU evidence, нет реального evidence -> nonzero. При legacy status нормализация только при отсутствии противоречащего result; конфликт -> fail. Нельзя создавать фактический HIL PASS моделью.',
      'HILMX-01':'Внешняя аппаратная приёмка читает canonical mvp_matrix.json:20 маршрутов, применимые/negative сценарии по capabilities. docs/vendor/hil_results.json использует result, kind, device/firmware, evidence. Только реальные прогоны, без SKIP/EMU/HIL подмены; LAB Smart S не выдаёт выдуманный номер. Код checker реализуют MATRIX-01/SCR-40; этот gate закрывает оператор по реальному evidence.',
    }
    for tid in ['BUG-005','TG-003','VG-11','VG-21','TST-GO-05','TST-VENDOR-08','TST-E2E-12','TST-E2E-15']:
        canonical[tid]='Реальное чтение часов ФУ; BC50 cmd62, cmd61/автоматическая установка часов недоступны. WARN при дрейфе>5мин, block при>2ч; отсутствие trusted measurement блокирует деньги. Рабочая дата Europe/Sofia, полночь/DST. Тесты: host-clock fallback отвергнут, +6мин WARN, +3ч block до card/fiscal I/O, timezone conversion. Никаких TestSyncDriftOver30sTriggersSet и фиктивных SetDeviceTime.'
    for tid,spec in canonical.items():
        t=tasks[tid];t['spec']=spec;t['spec_override']=True;t['notes']='Актуальная спецификация повторного ревью; конфликтующая историческая карточка не является исполняемым требованием. '+t.get('notes','')
        for u in t['units']:
            if tid in ['VG-03','VG-17']:
                u['title']=u['title'].replace('Разрешить SEQ 0x20..0xFF и круговой счётчик','Datecs SEQ 0x20..0x7F и круговой счётчик').replace('без SDK (DOC-V4) Smart S не реализуется','Smart S реализуется отдельно через JSON/HTTP, SDK не требуется')
                u['test']=u['test'].replace('0x20..0xFF','0x20..0x7F') if tid=='VG-03' else u['test']
            if tid in ['TST-E2E-16','TST-E2E-25','TST-E2E-26','TG-008','NAP-13','NAP-23','NAP-P-01','SCR-40','HILMX-01']:
                u['title']='Подзадача '+u['id']+': выполнить свою часть актуальной спецификации: '+spec+' Адресный unittest-harness: '+u['accept']+'. Native runtime/scripts реально вызываются; skips/no tests не допускаются; временные данные/evidence в MVP1_ARTIFACT_DIR.'
                u['test']=spec
    tasks['NAP-P-01']['depends'].append('SUPTO-01')

    for tid in ['BUG-005','TG-003','VG-11','VG-21','TST-GO-05','TST-VENDOR-08','TST-E2E-12','TST-E2E-15']:
        for u in tasks[tid]['units']:
            u['title']=u['title'].replace('TestSyncDriftOver30sTriggersSet','TestClockReadWarnAtFiveMinutesAndBlockAtTwoHours')
            u['title']+=' Актуальные ограничения: '+tasks[tid]['spec']+' DEVICE_TIME/read response: device_time RFC3339 + drift_seconds int, correlation operation_id; драйвер конкретного регистра; daily sync ключ tenant/workstation/business_date, O(1). Не заменять чтение ФУ host clock.'
            u['test']=tasks[tid]['spec']
    checker=tasks['SCR-40']['units'][0]
    checker['id']='SCR-40.1-through-2'
    checker['title']='Одна завершённая подзадача: сначала selftests good/bad (assertion FAIL до исправления), затем реализовать canonical checker и получить green. '+tasks['SCR-40']['spec']
    tasks['SCR-40']['units']=[checker]
    for tid in canonical:
        tasks[tid]['notes']='Актуальная спецификация повторного ревью в поле spec. Противоречащие исторические notes/карточки отменены; использовать текущие unit requirements и source decisions.'
    tasks['TST-VENDOR-08']['spec']+=' Для BC50/Datecs проверить и валюту: READY только при подтверждённом режиме2/3; 0/1/неизвестно -> отказ до card/fiscal I/O. Валюта не выводится из host config вместо чтения устройства.'
    for u in tasks['TST-VENDOR-08']['units']:
        u['title']+=' '+tasks['TST-VENDOR-08']['spec'];u['test']=tasks['TST-VENDOR-08']['spec']
    for tid in ['DSY-08','VG-17']:
        tasks[tid]['external_requires']=[d for d in tasks[tid].get('external_requires',[]) if d!='DOC-V4']+['DOC-V5']
        tasks[tid]['notes']+=' Compact S01: использовать его PDF/firmware evidence, не Smart S JSON документы DOC-V4. Неизвестная команда/режим -> BLOCKED_SPEC, не guessed READY; type/firmware approval DOC-V5 остаётся внешним.'
    tasks['VG-18']['spec']='Tremol вне MVP: только structured unsupported/capability gate и negative tests до I/O. Новый протокол/адаптер, golden conformance и vendor-команды не реализовывать.'
    tasks['VG-18']['spec_override']=True
    # Malformed legacy paths such as "то же", "..." or "Robolectric `Name.kt"
    # must never turn native test generation into a documentation-only packet.
    deep={
      1:(['fiscal-backend/internal/persistence/concurrent_test.go'],['fiscal-backend/internal/persistence'],['TG-001']),
      2:(['fiscal-backend/internal/domain/unp_allocator_concurrent_test.go'],['fiscal-backend/internal/domain/unp_allocator.go'],['TG-002']),
      3:(['fiscal-backend/internal/api/crash_replay_test.go'],['fiscal-backend/internal/api','fiscal-backend/internal/domain'],['TG-001','BUG-012']),
      4:(['fiscal-backend/internal/domain/ecdsa_vectors_test.go','docs/MVP1/fixtures/ecdsa_vectors.json'],['minipos/BeeMiniPOS/modules/beeloy-execution-crypto'],['MC-01']),
      5:(['SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsGoldenFramesTest.kt'],['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt'],['VG-20','VG-01','VG-02']),
      6:(['SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/JournalCrashRecoveryTest.kt','SmartDevices/bluecash-app/build.gradle.kts'],['SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash'],['BUG-030','BUG-032']),
      7:(['minipos/beeminipos-backend/internal/domain/webhook_race_test.go'],['minipos/beeminipos-backend/internal/domain'],['MP-03']),
      8:(['../POS/erp-connector/internal/orchestration/crash_test.go'],['../POS/erp-connector/internal/orchestration'],['EC-01','EC-02']),
      9:(['../BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/fiscal_results_dlq_test.go'],['../BeeloyBackend/public-api-core/internal/adminerp/networkmessaging'],['PA-01','PA-02']),
      10:(['tests/e2e/test_exactly_once.py'],['tests/e2e/gaps/support','tests/e2e/scenarios/support'],['E2E-HARNESS-01','MATRIX-01','BUG-072','TG-001','CL-02','POS-BLE-02','SMS-04','S3D-01','DSY-04','BP-05','MQ-03','EMU-01','EMU-02','EMU-03','EMU-04','EMU-05','EMU-06']),
      11:(['fiscal-backend/internal/domain/business_date_test.go'],['fiscal-backend/internal/domain'],['BUG-005','VG-21']),
      12:(['SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/FrameFuzzTest.kt','IoT/protocol-abstraction/tests/FrameFuzzTest.cpp'],['IoT/protocol-abstraction/src/FrameCodec.cpp','SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt'],['VG-02','DSY-02']),
      13:(['IoT/protocol-abstraction/tests/DaisyGoldenFramesTest.cpp'],['IoT/protocol-abstraction/src/FrameCodec.cpp'],['DSY-02']),
      14:(['IoT/common-modules/datecspay/tests/DatecsPayGoldenFramesTest.cpp','SmartDevices/daisy-smart-app/src/test/kotlin/com/beeloy/fiscal/daisy/SmartProtocolExamplesTest.kt'],['IoT/common-modules/datecspay','SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy'],['BP-02','SMS-08']),
    }
    for n,(files,reads,deps) in deep.items():
        t=tasks['TST-DEEP-'+str(n).zfill(2)];t['depends']=list(dict.fromkeys(t['depends']+deps))
        for u in t['units']:
            harness=[f for f in u['files'] if f.startswith('tests/mvp1/')]
            u['files']=files+harness;u['read_files']=reads;u['require_runtime']=True
            u['title']+=' Обязательные native test deliverables: '+', '.join(files)+'. Исходники read_files разрешены только для чтения. Не заменять native тест документом docs/MVP1/specs; Python harness обязан запустить эти конкретные тесты и проверить ненулевые native test counts.'
        if n==10:
            for u in t['units']:u['accept']='python3 -m unittest discover -s tests/e2e -p test_exactly_once.py';u['repeat']=3
    for u in tasks['NAP-09']['units']:
        u['files']=[f for f in u['files'] if not f.startswith('docs/MVP1/specs/')]+['docs/SUPTO/DECLARATION_DOSSIER.md','scripts/build-supto-dossier.py']
        u['title']+=' Создать реальный экстрактор и структурированное досье с источниками, версиями, пробелами/evidence и явным NORM_PENDING, без выдуманной юридической подписи. Проверить extractor на good/bad fixtures.'
        u['require_runtime']=True
    tasks['NAP-09']['external_requires']=list(dict.fromkeys(tasks['NAP-09'].get('external_requires',[])+['NORM-07']))
    for tid in ['SCOPE-02','SCOPE-03','SCOPE-04','SCOPE-05','SCOPE-06']:
        for u in tasks[tid]['units']:
            reads=[f for f in u['files'] if f.startswith(('fiscal-backend/','minipos/','../POS/','../BeeloyBackend/','BeeFiscalApp/'))]
            u['read_files']=list(dict.fromkeys(u.get('read_files',[])+reads))
            u['files']=[f for f in u['files'] if f not in reads];u['require_runtime']=False
            u['title']+=' Это статический анализ: production read_files не менять. Результат — конкретные findings/карточки с file:symbol и критериями; не объявлять найденные дефекты исправленными.'

    # Every S3 test card has a native deliverable and explicit implementation prerequisite.
    for n,dep in enumerate(['BUG-040','BUG-041','BUG-042','BUG-043','BUG-044','S3FIX-01'],1):
        t=tasks[f'TST-S3-{n:02d}'];t['depends']=list(dict.fromkeys(['W0-1',dep]+t['depends']))
        for u in t['units']:
            native='IoT/firmware/edge-agent-s3/idf/tests/'
            u['files']=[native+'intent_processor_native.cpp',u['files'][-1]]
            if n==1:
                u['files'] += [native+'intent_host',native+'run_native_saga.sh','tests/mvp1/native_intent.py']
            else:
                t['depends']=list(dict.fromkeys(t['depends']+['TST-S3-01']))
            u['read_files']=['IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp','IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp']
            u['require_runtime']=True
            u['title']=t['spec']+' Native regression в указанном C++ тесте; Python harness запускает реальный native runner с захватом вывода и проверкой ненулевого числа тестов. Для power-loss перебрать все границы reserve/execute/outcome/outbox. TST-S3-01 сначала создаёт общий host build в tests/mvp1/native_intent.py и intent_host: компилировать реальные main/intent_processor.cpp и durable_storage.cpp, подменять только hardware/clock/transport boundaries; не копировать алгоритм продукта в тест. Python harness напрямую вызывает compiler с production .cpp аргументами и полученный binary через subprocess, проверяет TAP ok-строки и plan, rc и имена сценариев. Общий helper поддерживает добавление C++ кейсов без изменения Python helper в следующих TST-S3. Включить intent binary в run_native_saga.sh для make iot-test; сохранить существующие receipt/profile тесты. Отсутствие cJSON/OpenSSL/compiler -> BLOCKED, не заменять статическим grep.'
    clock=tasks['BUG-005']
    for u in clock['units']:
        u['files']=[f for f in u['files'] if f!='mqttclient/client.go']
    clock['units'][0]['title']='Сначала воспроизводящий тест TestClockSyncRejectsHostClockAsDeviceClock: отсутствие реального ответа ФУ -> DEVICE_CLOCK_NOT_MEASURED. TestHasDailyClockSyncUsesSofiaBusinessDate: 2026-10-21T22:30Z -> 2026-10-22T01:30+03:00, business_date=2026-10-22. Реализовать DEVICE_TIME read через fiscal-backend/internal/mqttclient/client.go: correlation operation_id, дождаться ProjectedResult с device_time RFC3339 и drift_seconds int, ошибка/timeout без host-clock fallback. Для BC50 используется cmd62; cmd61 не отправлять. Проверить успешный ответ, отсутствующий/чужой ответ, timeout и business_date Europe/Sofia. '+clock['units'][0]['test']
    for t in tasks.values():
        for u in t['units']:
            u['red_required']=True
            u['title']+=' Контроллер сначала проверяет реальный assertion FAIL, затем green без изменения red-тестов. Для test-only работы разрешена временная мутация scoped production файла: контроллер восстановит его исходные байты перед green. Native subprocess обязан захватывать stdout/stderr и выдавать реальные ненулевые test counts (Go JSON, JUnit XML, unittest, pytest, TAP, GTest/Jest/CTest); python --version/echo/import без вызова продукта не являются runtime evidence.'

    # ARCH_07 proposals retain explicit dispositions; no silent/unrelated aliases.
    for tid,target in {'R-01':'SCRIPT-01','R-03':'SCRIPT-02','R-05':'SCOPE-01','R-06':'TST-CRIT-01'}.items():
        tasks[tid]={'id':tid,'priority':'P1','mode':'code','source':'ARCH_07_PLAN70_CRITICAL_REVIEW.md','source_status':'CANONICAL_ALIAS','depends':[target],'alias_of':target,'units':[],'notes':'Историческая карточка ARCH_07; canonical owner '+target+'. Pxx DEFAULT не отменяет текущие решения владельца и MVP_GOAL.','spec':'Явная трассировка требования к '+target+'; собственной фиктивной приёмки нет.'}
    tasks['SCRIPT-01']['spec']+=' Каноническая coverage-map.v1.json связывает все source_cards ARCH с fixes/plan.json и legacy status IDs; новые TST-S3 обязательны, отсутствующая карточка/alias/ссылка -> FAIL unmapped. status.json не изменять.'
    tasks['SCRIPT-02']['spec']+=' Канонические acceptance берутся из fixes/plan.json: каждая code unit имеет точный executable selector, red_required и ненулевые проверки. Legacy Pxx шаблоны не подменяют текущие карточки.'
    for tid,title,files,criterion in [
      ('R-02','Сопоставить исторические P40-P44 с текущими задачами MVP1',['docs/MVP1/fixtures/legacy_plan_mapping.json','scripts/check_legacy_plan_mapping.py'],'Для POS contract, backend config/readiness, BlueCash engine, rounding golden и эксплуатации/пилота указать реальные task IDs и evidence gates. Не создавать дубликаты работ; owner/manual gates остаются внешними. Missing group/unknown ID -> FAIL.'),
      ('R-04','Генератор компактных execution cards из fixes/plan.json',['docs/MVP1/tools/split_execution_cards.py','docs/MVP1/generated_cards'],'По одному файлу на unit, индекс задач и явные source/policy refs; каждая карточка <=150 строк и <=20KB без потери требований. Если требования не помещаются — fail с unit ID, не обрезать. Good/bad fixtures, ссылки и детерминированная пересборка. fixes и status.json только читать.'),
    ]:
        verify='tests/mvp1/test_'+tid.lower().replace('-','_')+'.py'
        tasks[tid]={'id':tid,'priority':'P1','mode':'code','source':'ARCH_07_PLAN70_CRITICAL_REVIEW.md','source_status':'NEW','depends':['W0-1','SCRIPT-01'],'spec':title+'. '+criterion+' Старый Pxx layout заменён текущим canonical execution plan; цель требования сохранена.','notes':'Исполняемая современная редакция исторического предложения ARCH_07.','units':[{'id':tid+'.1','title':title+'. '+criterion,'files':files+[verify],'test':criterion,'accept':'python3 -m unittest discover -s tests/mvp1 -p '+verify.rsplit('/',1)[1],'sources':['ARCH_07_PLAN70_CRITICAL_REVIEW.md'],'phase':'green','repeat':1,'regression':[],'red_required':True,'require_runtime':True}]}

    for u in tasks['SCRIPT-01']['units']:
        u['files']=list(dict.fromkeys(u['files']+['docs/MVP1/execution/coverage-map.v1.json']))
        u['read_files']=list(dict.fromkeys(u.get('read_files',[])+['docs/MVP1/fixes/plan.json']))
    for tid in ('R-02','R-04','SCRIPT-02'):
        for u in tasks[tid]['units']:
            u['read_files']=list(dict.fromkeys(u.get('read_files',[])+['docs/MVP1/fixes/plan.json','docs/MVP1/execution/status.json']))

    for tid in ('R-02','R-04','SCRIPT-01','SCRIPT-02'):tasks[tid]['spec_override']=True

    # Pilot: bounded, executable baseline contract instead of an open-ended report.
    baseline=tasks['W0-1'];baseline['spec_override']=True
    baseline['spec']='Сборщик baseline, не исправление модулей. Реализовать один scripts/mvp1-baseline.sh (допустим Python stdlib внутри shell wrapper), без сторонних зависимостей. Интерфейс --output-dir DIR, --fixture JSON (только тестовый режим), --timeout SECONDS. Полный запуск использует реальные команды ARCH_21 §6: fiscal-backend go test ./...; S3 bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh; protocols bash IoT/protocol-abstraction/run-tests.sh; SmartDevices ./gradlew :bluecash-app:testProdDebugUnitTest :bluecash-app:testDemoDebugUnitTest :daisy-smart-app:testProdDebugUnitTest :daisy-smart-app:testDemoDebugUnitTest --no-daemon --rerun-tasks из SmartDevices; POS npx --no-install jest --runInBand из ../POS/pos-app; MiniPOS npx --no-install vitest run из minipos/miniposweb; e2e make compose-e2e из корня. Не использовать отсутствующий e2e-local, gradlew в bluecash-app или Jest для MiniPOS. Canonical module IDs: fiscal-backend,s3,protocols,smartdevices,pos,minipos,e2e. Не выполнять deploy/HIL/ротацию. Не читать OPEN_WORK/MASTER_PLAN целиком: все команды и контракт здесь.'
    unit=baseline['units'][0]
    unit['title']='Создать сборщик и tests/mvp1/test_w0_1_1.py. Вывод report.json schema=1: commit (или null с причиной), tools (версии либо unavailable), modules ровно 7 записей с id,cwd,argv,rc,status(PASS/FAIL/BLOCKED),reason,duration_seconds,stdout/stderr bounded, overall_status. RC 0 только все PASS, RC 1 при FAIL, RC 3 при missing tool/cwd/timeout; отчёт сохраняется при любом исходе. --output-dir обязателен для harness и берётся из MVP1_ARTIFACT_DIR; тесты не пишут в checkout. --fixture JSON содержит список из 7 объектов id,cwd,argv, используется только для настоящего запуска маленьких Python команд из tempfile, не для чтения готовых PASS. Good fixture 7 успешных команд; bad одна rc1; blocked отсутствующий executable/cwd и timeout. Проверить каждую запись и overall/rc, stdout и rc из реально выполненной команды. Не требовать зелёного продукта для приёмки сборщика. Red: допустим минимальный исполняемый stub сборщика с неверным report; содержательный assertion должен упасть на неверном поле, не на FileNotFound/import/compile. Green реализует контракт без изменения тестов. Размер реализации до 200 строк, не создавать framework. Полный baseline с реальными модулями может быть FAIL/BLOCKED: записать честно, не подменять fixtures. Для пилота достаточно проверки сборщика на good/bad/blocked fixtures; это не достижение MVP.'
    unit['test']='Семь выполненных fixture команд; точный schema/records/overall/rc; good, failure, missing executable/cwd, timeout; red assertion на неверном report, затем те же тесты green.'

    # Pilot rejected an invented 112-command cycle and an extra out-of-scope C++ file.
    seq=tasks['SEQ-01'];seq['spec_override']=True
    seq['spec']='Исправить profile-specific SEQ в SerialSession::exchange, не менять wire codec/ответы Fake. Datecs: 0x20..0x7F включительно = 96 значений. Golden индекс→SEQ: 0→0x20,95→0x7F,96→0x20,97→0x21,112→0x30,192→0x20. Daisy: 0x20..0xFF = 224 значения; 0→0x20,223→0xFF,224→0x20,225→0x21. Индекс 112 НЕ является Datecs wrap. Повтор передачи одной команды использует прежний immutable packet/SEQ.'
    for unit in seq['units']:
        unit['read_files']=['IoT/firmware/edge-agent-s3/idf/main/serial_exchange.h']
        unit['title']=seq['spec']+' Использовать существующий Fake из serial_exchange_native.cpp, не придумывать новый ответ ФУ/парсер и не создавать отдельный .cpp. Старую безусловную проверку f.sent[224]==f.sent[0] заменить на profile-specific golden assertions: для Datecs период 96, для Daisy 224. Datecs SEQ находится в sent[i][5], Daisy в sent[i][2]. Все старые тесты сохранить, исправить только устаревший Datecs oracle. В конце main вывести реальный TAP plan и ok после выполненных assertions. Python harness до 40 строк: subprocess bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh с capture_output/text/timeout, проверить rc0 и TAP; не читать/парсить implementation как oracle. Red оставляет native assertion FAIL на исходном Datecs коде; green минимально исправляет serial_exchange.cpp. Не менять тесты в green. Запрещён новый serial_exchange_datecs_wrap.cpp и прочие дополнительные файлы. Не писать длинный план: прочитать cpp/test/header (3 файла), добавить golden тест и harness, запустить. Полная приёмка — обе serial profiles, 226+ команд, immutable retry SEQ.'
        unit['test']='Golden Datecs cycle 96 (индексы 95,96,112,192), Daisy 224 (223,224,225); не 112. Нативные assertions, TAP, rc; red на реальном runtime, затем неизменный тест green.'
    money=tasks['BUG-002'];money['spec_override']=True
    money['spec']='validMoney(Money): только EUR, ASCII unsigned decimal, ровно две дробные цифры, 1..12 цифр до точки. Уже существующий moneyPattern объявлен в receipt_saga.go; использовать его вместе с ограничением длины integer part. Не переносить/дублировать regex: receipt_saga.go только читать. Не менять отдельный ParseMoney и его существующий предел 7 цифр: эта задача про validMoney. Native tests писать в разрешённый bug_002_test.go, а не service_money_test.go.'
    for unit in money['units']:
        unit['read_files']=list(dict.fromkeys(unit.get('read_files',[])+['fiscal-backend/internal/domain/receipt_saga.go']))
        unit['title']=money['spec']+' В service.go читать только validMoney через rg/sed, файл >300 строк. TestValidMoney таблицей: EUR 0.00,10.99,123456789012.00 принимаются; USD,-1.00,+1.00,1.0,1,00,1.005,１.００,1234567890123.00,1.+1,whitespace отклоняются. В expected не вызывать production parser. Harness до 40 строк запускает go test -json ./internal/domain -run ^TestValidMoney$ -count=1 из fiscal-backend, проверяет rc и наличие Test pass events, no fail/skip/no-tests. Red на существующем приёме знака, green минимальная правка validMoney; никакого grep вместо выполнения Go. Не создавать других test файлов и не менять receipt_saga.go в green.'
        unit['test']='Реальный TestValidMoney: EUR/ASCII/знак/12 vs 13 цифр/дробная часть/currency; ненулевые Go JSON Test events. Red -> те же тесты green.'

    native_cases=['valid_zero','valid_price','valid_max12','reject_currency','reject_minus','reject_plus','reject_fraction1','reject_comma','reject_fraction3','reject_unicode','reject_max13','reject_fraction_sign','reject_space']
    for task_id in ('SEQ-01','BUG-002'):
        for unit in tasks[task_id]['units']:
            if task_id=='BUG-002':
                argv=['go','test','-json','./internal/domain','-run','^TestValidMoney$','-count=1'];cwd='fiscal-backend'
                extra='required_tests='+repr(tuple(['TestValidMoney']+['TestValidMoney/'+v for v in native_cases]))
                unit['title']+=' Go table t.Run labels в порядке перечисленных выше входов: '+','.join(native_cases)+'. Требуются все 13 кейсов и parent TestValidMoney. Использовать Action/Test из реального Go JSON, поля Type там нет.'
            else:
                argv=['bash','IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh'];cwd='.'
                extra="markers=('ok 1 - Datecs cycle96','ok 2 - Daisy cycle224')"
                unit['title']+=' После завершения каждого профиля вывести соответствующую TAP строку ok 1 - Datecs cycle96 / ok 2 - Daisy cycle224; в конце 1..2. Печатать после assertions, не вместо них.'
            template='from pathlib import Path\nimport sys,unittest\nROOT=Path(__file__).resolve().parents[2]\nsys.path.insert(0,str(ROOT/"docs/MVP1/fixes"))\nfrom native_harness import check_native_tests\nclass T(unittest.TestCase):\n    def test_native(self):\n        check_native_tests(self,'+repr(argv)+',root=ROOT,cwd='+repr(cwd)+','+extra+',timeout=120)\n'
            unit['harness_template']=template
            unit['title']+=' Контроллер заранее создаёт этот фиксированный harness; не менять его. Использовать этот harness без phase/source inspection; не писать собственный parser/вызов Python вместо Go. Менять только native тест/продукт по этапу.\n```python\n'+template+'```'

    import runpy
    from pathlib import Path
    templates=runpy.run_path(str(Path(__file__).with_name('harness_templates.py')))
    for unit in tasks['BUG-002']['units']:
        unit['test_vectors']=templates['MONEY_VECTORS']
        unit['test_vectors_file']='docs/MVP1/fixes/golden_money_validation.json'
        unit['read_files']=list(dict.fromkeys(unit.get('read_files',[])+[unit['test_vectors_file']]))
        unit['title']+=' Денежные кейсы НЕ перепечатывать в Go array: загрузить ../../../docs/MVP1/fixes/golden_money_validation.json из package domain через os.ReadFile + encoding/json.Unmarshal. Тип записи: Name string json:name, Currency string json:currency, Amount string json:amount, Want bool json:want. Затем t.Run(c.Name) и сравнить validMoney(Money{Currency:c.Currency,Amount:c.Amount}) с c.Want. Fixture ровно 13 строк, обязательны все имена. reject_fraction_sign содержит 1.+1 и want=false, а не 1.00/true. Fixture и Python harness не менять. Контроллер уже создал harness; red native failure не отменяет выполнение unit accept.'
    unit=tasks['W0-1']['units'][0]
    unit['harness_template']=templates['BASELINE_TEMPLATE']
    unit['scaffolds']={'scripts/mvp1-baseline.sh':'#!/bin/sh\n# RED scaffold for a new collector; replace only in green.\nexit 0\n'}
    unit['title']+=' Контроллер создаёт фиксированный tests/mvp1/test_w0_1_1.py и только при отсутствии скрипта минимальный red scaffold. Harness не менять. Red: запустить unit accept, оставить assertion failure на stub, STOP. Green: реализовать только scripts/mvp1-baseline.sh, не переписывать тест. Fixture — JSON array из 7 объектов id,cwd,argv; не объект с ключом commands. stdout/stderr строки <=8192 символов. При mixed FAIL/BLOCKED overall BLOCKED и rc3. Для каждого subprocess установить env.PWD=resolved cwd (OLDPWD убрать), иначе команды могут выбрать чужой проект. По fixture командные marker files доказывают настоящее выполнение. Инструменты tools — непустой dict с версиями/недоступностью. Отчёт report.json должен появиться для good/fail/missing executable/missing cwd/timeout.'

    tasks['W0-1']['units'][0]['title']+=' Canonical tools keys: python,git,go,c++,node,npm,java,docker; значения — реальные версии или unavailable с причиной. commit равен git rev-parse HEAD из корня; если HEAD отсутствует — null и commit_reason. В baseline fixture есть все поля, включая rc даже для BLOCKED (null/timeout code).'

    tasks['W0-1']['units'][0]['title']=tasks['W0-1']['units'][0]['title'].replace('Создать сборщик и tests/mvp1/test_w0_1_1.py.','Реализовать сборщик; фиксированный Python harness уже подготовлен контроллером.')

    # Fixed native fixture prevents infrastructure detours and broken SDK counters.
    unit=tasks['AA-01']['units'][0]
    unit['harness_template']='from pathlib import Path\nimport sys,json,unittest\nROOT=Path(__file__).resolve().parents[2]\nsys.path.insert(0,str(ROOT/"docs/MVP1/fixes"))\nfrom native_harness import check_native_tests\nclass T(unittest.TestCase):\n def test_native(self):\n  self.assertEqual((ROOT/"AdminApp/src/idempotency.test.ts").read_text(),"jest.mock(\'expo-crypto\', () => ({randomUUID: jest.fn(() => jest.requireActual(\'node:crypto\').randomUUID())}), {virtual: true});\\nconst {newKey} = require(\'./idempotency\');\\nconst sdk = jest.requireMock(\'expo-crypto\');\\nconst savedCrypto = global.crypto;\\nbeforeEach(() => { global.crypto = undefined; jest.clearAllMocks(); });\\nafterAll(() => { global.crypto = savedCrypto; });\\ntest(\'UUID v4\', () => { expect(newKey()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i); });\\ntest(\'distinct keys\', () => { expect(newKey()).not.toEqual(newKey()); });\\ntest(\'SDK delegation\', () => { newKey(); expect(sdk.randomUUID).toHaveBeenCalledTimes(1); });\\ntest(\'no global crypto required\', () => { expect(global.crypto).toBeUndefined(); expect(typeof newKey()).toBe(\'string\'); expect(sdk.randomUUID).toHaveBeenCalledTimes(1); });\\n")\n  check_native_tests(self,["npx","--no-install","jest","--config","jest.config.js","--runInBand","--runTestsByPath","src/idempotency.test.ts","--no-cache"],root=ROOT,cwd="AdminApp",minimum=4,timeout=120)\n def test_dependency(self):\n  p=json.loads((ROOT/"AdminApp/package.json").read_text())\n  self.assertEqual(p.get("dependencies",{}).get("expo-crypto"),"~15.0.9")\n'
    unit['scaffolds']={'AdminApp/src/idempotency.test.ts': "jest.mock('expo-crypto', () => ({randomUUID: jest.fn(() => jest.requireActual('node:crypto').randomUUID())}), {virtual: true});\nconst {newKey} = require('./idempotency');\nconst sdk = jest.requireMock('expo-crypto');\nconst savedCrypto = global.crypto;\nbeforeEach(() => { global.crypto = undefined; jest.clearAllMocks(); });\nafterAll(() => { global.crypto = savedCrypto; });\ntest('UUID v4', () => { expect(newKey()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i); });\ntest('distinct keys', () => { expect(newKey()).not.toEqual(newKey()); });\ntest('SDK delegation', () => { newKey(); expect(sdk.randomUUID).toHaveBeenCalledTimes(1); });\ntest('no global crypto required', () => { expect(global.crypto).toBeUndefined(); expect(typeof newKey()).toBe('string'); expect(sdk.randomUUID).toHaveBeenCalledTimes(1); });\n", 'AdminApp/jest.config.js': "module.exports = {testEnvironment:'node', transform:{'^.+\\\\.tsx?$':['ts-jest',{tsconfig:{module:'CommonJS',target:'ES2020'},diagnostics:false}]}};\n", 'AdminApp/src/idempotency.ts': "export function newKey(): string { return 'red-stub'; }\n"}
    unit['title']+=' Controller prepares a correct fixed Jest test, jest.config.js with ts-jest, and a constant production stub. Do not edit the native test or Python harness. Do not add Babel, another config extension, or another harness. Red: run unit acceptance on stub and STOP at UUID/delegation assertion failures. Green: implement only helper plus manifest/lock dev tooling if needed; keep Jest configuration compatible with the supplied ts-jest setup. Native fixture has four independent tests with jest.fn counters reset beforeEach; a failing counter test must call newKey before checking its calls. Do not manufacture red by breaking a test mock.'

    unit['title']+=' Before red acceptance, install missing local Jest tooling into AdminApp with npm install --ignore-scripts --no-audit --no-fund --save-dev jest@~29.7.0 ts-jest@~29.4.0 typescript@~5.9.2; package.json/package-lock.json are scoped. A missing ts-jest is a setup error, never valid red evidence. Keep dependency installation changes consistent again in green after controller restores manifests.'

    # Application binaries validate an externally managed schema; local SQL is a test fixture.
    tasks['PERSIST-01']['spec_override']=True
    tasks['PERSIST-01']['spec']='Persistence compatibility fixture and atomic durable runtime. database/README.md is authoritative: BeeloyDB owns platform DDL; service RunMigrations is a readiness gate and MUST NOT apply local SQL. Local database/fiscal SQL is isolated compatibility fixture only; its passing tests do not certify canonical BeeloyDB migrations. Execute only selected unit.'
    unit=tasks['PERSIST-01']['units'][0]
    unit['harness_template']='from pathlib import Path\nimport sys,unittest\nROOT=Path(__file__).resolve().parents[2]\nsys.path.insert(0,str(ROOT/"docs/MVP1/fixes"))\nfrom native_harness import check_native_tests\nclass T(unittest.TestCase):\n def test_schema(self):\n  check_native_tests(self,["go","test","-json","./internal/persistence","-run","^TestMVP1Schema","-count=1","-timeout=240s"],root=ROOT,cwd="fiscal-backend",required_tests=("TestMVP1SchemaFresh","TestMVP1SchemaUpgradePreservesData","TestMVP1SchemaRollbackReapply"),minimum=3,timeout=300)\n'
    unit['read_files']=['database/README.md','scripts/postgres-integration.sh','scripts/prepare-fiscal-pg-fixture.py','fiscal-backend/internal/persistence/postgres_fixture_test.go','fiscal-backend/internal/persistence/migrations.go']
    unit['title']='Implement real PostgreSQL compatibility migration tests in repository_schema_test.go. Fixed Python harness invokes three required Go tests TestMVP1SchemaFresh, TestMVP1SchemaUpgradePreservesData, TestMVP1SchemaRollbackReapply. No source-string assertions, SQLite substitution, version-only subprocesses, or tests requiring RunMigrations to read database/fiscal. Use installed initdb/pg_ctl/psql via exec.LookPath and exec.Command; start an OWN temporary PostgreSQL cluster under MVP1_ARTIFACT_DIR, unique Unix socket directory, listen_addresses empty; stop only this owned cluster with cleanup, never connect to shared/production DB or read real credentials. Reuse prepare-fiscal-pg-fixture.py namespace handling; do not edit that script. Fresh: apply actual ordered fixture SQL and assert operation/event/replay/UNP/outbox tables and UNIQUE violations on same tenant+UNP and tenant+idempotency key, valid distinct values accepted. Upgrade: apply predecessor fixture prefix, seed valid records, apply remaining SQL, query exact preserved values and new constraints. Rollback/reapply: exercise actual isolated SQL transaction rollback preserving seeded data, then apply again and query schema/data; if no down files, report that explicit down-migration gate is unsupported, never pretend rollback is full canonical up/down/up. No t.Skip; missing PostgreSQL/tool is BLOCKED, not PASS. Red: first implement correct native tests, temporarily mutate only scoped fixture SQL constraint if already correct, run exact unit accept to real assertion failure, STOP; controller restores fixture before green. Green: repair only fixture product SQL or verify restored fixture, never change sealed tests. Leave application migrations.go unchanged. Keep native assertions exact and runtime-based.'
    unit['test']='Actual disposable PostgreSQL: fresh fixture schema and uniqueness, predecessor-data upgrade preservation, transaction rollback/reapply. Canonical BeeloyDB and missing explicit down migrations remain external unverified gates.'
