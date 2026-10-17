# Подготовка MiniPOS к процедуре НАП для СУПТО

Дата исходной проверки публичных источников: 2026-10-17. Статус: **PREPARATION_OPEN**, подача не выполнялась. Это дорожка подготовки технического и декларационного пакета, а не утверждение юридического соответствия существующей реализации.

## Что подтверждено на официальных страницах

НАП описывает декларирование соответствия производителем/распространителем и включение версии СУПТО в публичный список после рассмотрения сведений. Поэтому рабочее слово «сертификация» в цели преобразуется в проверяемые результаты подготовки к применимой процедуре, без обещания автоматического сертификата. [Официальный публичный список НАП](https://nra.bg/wps/portal/nra/registers-i-spisuci/Spisuci/page.publichen-elektronen-spisuk-na-SUPTO).

Страница услуги указывает декларацию приложения30, сведения приложения31, руководство и описание реализации требований приложения29 со screenshots. Для локальной установки описаны DB objects и executable/source инструмента извлечения; для SaaS — описание услуги, каналов/портов, локальных компонентов и размещения БД. Подача электронная с КЕП. [Услуга НАП №2888](https://nra.bg/wps/wcm/connect/agency/site/uslugi/deklarirane-na-saotvetstvie-na-softuer-za-upravlenie-na-prodazhbite-supto-v-targovski-obekti-podava-se-ot-proizvoditel-ili-razprostranitel-na-softuera).

На странице производителя доступны порядок, ссылки на нормативные материалы и контрольный лист самооценки. Перед submission проверить действующую редакцию и применимость режима, формы и изменения законодательства. Новость о планируемых изменениях2026 не доказывает вступление нормы в силу; полный актуальный legal baseline этим быстрым поиском не установлен. [Декларирование СУПТО производителями](https://nra.bg/wps/portal/nra/fiskalni-ustroystva-supto-i-e-magazini/supto/page.deklarirane-supto).

## Задания, которые начинать одновременно с исправлениями

| ID | Результат подготовки | Доказательство завершения / зависимость |
|---|---|---|
| N0 — applicable legal baseline | Зафиксировать источник, редакцию, дату действия, region/currency, применимый SUPTO regime, формы и требования | Versioned source inventory и review болгарским профильным специалистом; старые local RU translations и PASS trace не объявляются актуальным правом |
| N1 — product boundary | Определить декларируемое имя/версию MiniPOS и состав: mobile/web, MiniPOS backend, Fiscal backend, S3/BlueCash/local modules и их release versions | Architecture/data-flow/dependency manifest; тонкий POS не объявляется единственной системой только потому, что UI небольшой |
| N2 — declaration owner | Подготовить сведения о производителе/распространителе, полномочиях, представителе и КЕП | Checklist реквизитов, без хранения ключей/подписи в source или тестах; фактическая подача отдельным действием |
| N3 — requirement trace | Сопоставить применимые пункты с actual runtime и tests, переоценить существующие machine PASS | Requirement/source version → GAP/task → exact case → actual component → software/HIL/organizational evidence; статусы OPEN/VERIFIED/EXTERNAL_PENDING, не наличие symbol/string |
| N4 — functional proof | Bulgarian UI/manual, sale lifecycle, operator identity/roles, UNP authority, time/readiness, payments/storno, audit/export/retention, migration/backup | По применимой норме — executable cases, screenshots и vendor/HIL evidence. Offline UNP/authority/DB recovery отдельно review до закрытия compliance |
| N5 — data and installation | Актуальный DB dictionary/relations/fields, hybrid local/cloud storage и data flows, network ports, devices/protocols/security | Generated schema docs из принятой версии, locations/config inventory и reproducible installation procedure; personal data handling проверяется отдельно |
| N6 — extraction/auditor tools | Проверить требуемый способ export/read-only audit и комплект executable/source для применимой deployment модели | Fixtures и roundtrip/export completeness/access isolation tests; выдача данных не зависит от скрытого mutable coordinator cache |
| N7 — dossier draft | Подготовить versioned draft приложений30/31, описание29, manual/screenshots, схемы, release/evidence index | Каждое утверждение реализации ссылается на exact artifact/case; drafts не подписаны и не поданы автоматически |
| N8 — rehearsal и freeze | Пройти применимый self-assessment, demo script и независимое техническое/правовое review | Frozen release/version scope, замечания закрыты или явно blocking; product update/change procedure готова |

N0/N1/N2 стартуют сразу; N3–N7 пополняются при принятии каждого coding пакета; N8 выполняется на fixed candidate. Зависящие от физического устройства cases остаются PHYSICAL_HIL_PENDING, организационные сведения — OWNER_INPUT_REQUIRED. Обе категории не маскируются unit tests.

## Scope технических тестов для traceability

Использовать существующие T-01…T-25 и добавить необходимые cases после N0, не подменяя конкретные требования общим тестом «Annex29 PASS». Особое внимание: один lifecycle и UNP при cloud/local переключении; невозможность обхода фискализации через alternate module; запрет нового списания после UNKNOWN; неизменяемость аудита и доступ к архиву; restore/migrations с сохранением истории; операторские права/идентичность; доказанный final fiscal device probe; actual time/identity из устройства; согласованность бухгалтерского/аудиторского export с persistent rows.

Указанные направления — engineering checklist для сопоставления с N0, не новые юридические выводы о конкретном обязательном пункте. SUPTO profile должен охватывать все реально используемые модули продаж в выбранной поставке, а не только пути, удобные для тестирования.

## Граница готовности

`REGULATORY_PACKAGE_DRAFT` — структура и материалы собраны, открытые вопросы видимы. `REGULATORY_PACKAGE_READY_FOR_REVIEW` — технические утверждения подтверждены и документы готовы к независимой проверке. `READY_FOR_SUBMISSION` — применимость/реквизиты/версия подтверждены ответственным лицом и необходимые внешние проверки выполнены. `SUBMITTED` и `LISTED_BY_NAP` требуют реальных подтверждений соответствующего внешнего действия; программные tests их не выставляют.

Не отправлять обращения/декларации, не подписывать документы КЕП и не публиковать regulatory claims в рамках подготовки стратегии. Техническая цель — подготовить систему и проверяемый пакет; внешний исход НАП отдельный статус.
