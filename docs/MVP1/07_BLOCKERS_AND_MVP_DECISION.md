# Блокеры и решение о готовности MVP1

Дата повторной проверки: 2026-08-13.  
Проверенный commit: `963d4e3`.  
Текущий итог: **software simulator slice — PASS; обязательный multi-device
physical MVP — NO-GO; production BG — NO-GO**.

## 1. Три разных уровня готовности

| Уровень | Что доказывает | Текущий статус |
|---|---|---|
| Software simulation MVP | domain/API/DB/UI/compose semantics на simulator/STUB | PASS по существующим автоматическим gates |
| Working physical MVP | BlueCash-50, DP-150 MX, BluePad-50 Plus и Daisy Compact S 01 по scope `index.md` | NO-GO |
| Bulgarian production | hardware MVP + SUPTO/legal/vendor/release/operations evidence | NO-GO |

Зелёный `make full-regression` не может автоматически повышать второй или третий
уровень. `contracts/mvp-acceptance-v1.json` сейчас корректно содержит
`pilot_status=NO_GO` и `production_status=NO_GO`, но его
`base_profile_status=PASS` относится только к профилю
`BG_MVP_FUNCTIONAL_NONPROD`.

## 2. Software blockers, устранимые в репозитории

Эти блокеры не требуют юридического решения и должны быть реализованы командой.
Они подробно специфицированы в документе 06.

| ID | Фактическое состояние | Условие закрытия |
|---|---|---|
| SW-01 | Android journal выдаёт DER `SHA256withECDSA`, backend принимает P1363 | единый Kotlin/Go signature profile и cross-language/Keystore tests |
| SW-02 | backend readiness для MQTT равен соединению с broker | signed physical probe ФУ/FMIN/pinpad и lease policy |
| SW-03 | нет aggregate `SALE_FINALIZE`; BLE хранит одну payment leg | одна durable receipt session с ordered payments и compensation |
| SW-04 | RRN/auth/original payment находятся в process-memory map | SQLite payment journal, lookup/reconcile и reversal после reboot |
| SW-05 | MQTT BlueCash parser теряет discount/unit/department | lossless canonical mapping и exact-total golden tests |
| SW-06 | release composition содержит `*_NOT_INSTALLED` adapters | реальный selected vendor adapter и static no-stub release gate |
| SW-07 | X/Z/cash/clock/probe не используют единый MQTT dispatcher | uniform durable command outbox/result materialization |
| SW-08 | один global driver вместо route per active register binding | route resolver, fencing и no-silent-fallback tests |
| SW-09 | fiscal operator credentials/FMIN/time не связаны end-to-end | versioned operator mapping, physical FMIN/time verify |
| SW-10 | compose не даёт воспроизводимый TLS broker/ACL контур | broker profile или проверяемый external dependency provisioning |
| SW-11 | sync projection не моделирует physical saga/compensation | receipt/payment/step events и crash-safe materializer |
| SW-12 | новые команды не закреплены всеми machine contracts/DDL | OpenAPI + AsyncAPI/JSON Schema + BLE CBOR + DDL + trace lock |
| SW-13 | contract gate: 8 Platform device OpenAPI operations не зарегистрированы runtime router | отдельный Admin/Platform router с auth/audit либо versioned removal; contract test PASS |
| SW-14 | нет обязательного lightweight ping и автоматической cross-transport continuation state machine | документ 09 реализован целиком; REST↔BLE stub/HIL tests PASS |
| SW-15 | client UUID не закреплён как end-to-end idempotency key каждой mutation | OpenAPI/DDL/REST/MQTT/BLE/journal/WebHook используют MiniPOS-generated UUID; concurrency/failover tests PASS |

Codex/разработчик может закрыть эти пункты программно при наличии vendor SDK и
доступа к hardware для финальной проверки. Их нельзя формально исключить из
working physical MVP.

## 3. Внешние блокеры, которые нельзя честно решить только изменением кода

### EXT-01 — BlueCash vendor runtime/API

В репозитории есть protocol codec, но production composition содержит
`DATECS_FISCAL_VENDOR_BRIDGE_NOT_INSTALLED` и
`DATECS_PINPAD_VENDOR_BRIDGE_NOT_INSTALLED`. Для закрытия нужны:

- поддерживаемый Datecs Android fiscal API/SDK и версия для конкретной прошивки;
- поддерживаемый Datecspay SDK, license/runtime dependencies;
- письменная compatibility matrix BlueCash-50 model/OS/fiscal firmware;
- разрешённый способ доступа к встроенным fiscal и payment interfaces.

**Кто закрывает:** Datecs/vendor + владелец продукта.  
**Evidence:** полученный пакет с checksum/license, зафиксированные версии,
release adapter без stub, vendor-backed bench run.

### EXT-02 — acquirer/processor test environment

Реальные approve/decline/timeout/reversal нельзя доказать без активированного
тестового терминала, merchant/acquirer configuration, тестовых карт и разрешённых
ключей/параметров. Секреты нельзя генерировать или обходить самостоятельно.

**Кто закрывает:** банк/эквайер/Datecs и компания-владелец merchant account.  
**Evidence:** test terminal onboarding, неэкспортируемые terminal keys,
approve/decline/timeout/reversal receipts и reconciliation record.

### EXT-03 — неопределённость storno command 43 на BC-50

Примеры BlueCash используют `open_StornoReceiptAsync`, но protocol PDF отмечает,
что команда 43 не используется на BC-50. Нельзя угадывать физический путь или
объявлять его поддержанным по simulator test.

**Кто закрывает:** Datecs.  
**Нужное решение:** письменное указание, какой Android SDK/wire command применять
для CASH/CARD storno на выбранных firmware.  
**Evidence:** vendor answer + golden exchange + physical storno HIL, включая
crash после card reversal и после fiscal close.

### EXT-04 — физическое устройство и HIL-доступ

Нужны реальный BlueCash-50, доступ к его Android build/install/debug workflow,
печать, fiscal memory/KLEN test mode разрешённого типа и возможность управляемо
отключать сеть/питание. Без этого невозможно доказать socket permissions,
реальные status bits, paper/FM failures, BLE radio и power-loss recovery.

**Кто закрывает:** владелец hardware/test lab.  
**Evidence:** device inventory (serial/FMIN/firmware), signed HIL log, фото/сканы
чеков и terminal evidence, fault matrix.

### EXT-04A — DP-150 MX COM/electrical/firmware profile

Нельзя подтвердить рабочий COM route только protocol parser. Требуются точные
RS-232 voltage levels, pinout/cable gender, baud/parity profile, питание и EUR
fiscal firmware выбранного DP-150 MX. Ошибка electrical profile способна
повредить ESP32 или кассу.

**Кто закрывает:** Datecs/authorized service + hardware owner.  
**Evidence:** vendor pinout/firmware statement, reviewed schematic/BOM,
осциллограф/bench record и полный DP-150 HIL.

### EXT-04B — BluePad-50 Plus BLE/acquirer profile

Нужны разрешённые BLE services/pairing semantics, поддерживаемый Datecspay SDK,
совместимая terminal firmware и acquirer activation. Документация протокола не
даёт права создавать/подменять terminal keys.

**Кто закрывает:** Datecs + acquirer.  
**Evidence:** compatibility/permission statement, activated test terminal,
pair/reconnect/purchase/lookup/reversal HIL и combined DP-150 saga.

### EXT-04C — Daisy Compact S 01 USB host/electrical/EUR profile

Нужны подтверждение USB host compatibility с ESP32-S3, VID/PID/interfaces,
питание/inrush, кабель и применимость native protocol к EUR firmware конкретной
Daisy Compact S 01.

**Кто закрывает:** Daisy/vendor + hardware owner.  
**Evidence:** vendor statement, USB descriptors, electrical review и HIL всех
обязательных fiscal commands/disconnect/recovery.

### EXT-05 — IdP/passwordless deployment

Software поддерживает OIDC Authorization Code + PKCE, но организация должна
выбрать IdP, зарегистрировать Android/iOS/Web clients/redirect URIs и задать
passwordless/passkey policy. Приложение не должно самостоятельно подменять IdP
локальным PIN/общим кассиром.

**Кто закрывает:** tenant IT/security.  
**Evidence:** issuer/audience/JWKS/client registrations и real login, expiry,
refresh, revoke tests.

### EXT-06 — regulatory/legal approval

Рабочий controlled MVP можно тестировать до формального разрешения, но нельзя
использовать как production СУПТО без подтверждения актуальной Н-18 matrix,
vendor/service dossiers и применимых НАП/БИМ/authorized-service процедур.

**Кто закрывает:** болгарский compliance counsel, изготовитель/сервис и при
необходимости НАП/БИМ.  
**Evidence:** подписанная legal matrix, декларационный пакет и hardware evidence.

### EXT-07 — production release trust

Нужны защищённое хранение release-signing keys, реальный vulnerability scanner,
одобренный SBOM/provenance и human sign-off Product/Engineering/QA/Security/
Compliance. Автоматические локальные подписи не заменяют независимых approvers.

**Кто закрывает:** release/security owners.  
**Evidence:** signed release manifest, accepted scan, protected-key provenance и
подписи acceptance record.

## 4. Что не блокирует controlled MVP

Следующее можно оставить после MVP1, если это явно выключено capability/profile:

- Daisy SMART real driver и edge profiles вне DP-150/BluePad/Daisy Compact,
  перечисленных в `index.md`;
- Secure Boot/Flash Encryption/eFuse/JTAG closure — обязательны до production;
- Web Bluetooth, если MVP BLE client только native Android MiniPOS;
- multiple fiscal devices одного role на register;
- invoice, tip, cashback, preauthorization, unsupported optional reports;
- массовый provisioning, OTA rollout rings и manufacturing line.

Это не разрешает ослаблять TLS, tenant isolation, physical readiness, journal,
idempotency, one-receipt semantics или card recovery.

## 5. Запрашиваемые решения и материалы

Чтобы реализация могла дойти до рабочего physical MVP без предположений, владелец
проекта должен предоставить:

1. BlueCash-50, DP-150 MX, BluePad-50 Plus и Daisy Compact S 01 с точными
   hardware/OS/fiscal/payment firmware versions.
2. Поддерживаемые Datecs fiscal/Datecspay и Daisy SDK/API/protocol packages с
   лицензиями и applicability statements.
3. Ответ Datecs по storno для BC-50 и terminal lookup/reversal semantics.
4. DP-150 COM и Daisy USB electrical/interface/vendor profiles.
5. Acquirer test merchant/terminal activation и тестовые сценарии для BlueCash
   и BluePad.
6. Доступ к lab для install/log/power/network/BLE/COM/USB/fiscal tests.
7. Выбранный OIDC IdP и client registrations — нужен до pilot, не блокирует
   ранний device-only HIL.

Без пунктов 1–6 можно завершить contracts, DDL, orchestration и симуляционные
тесты, но нельзя подтвердить **рабочий** physical MVP.

## 6. GO rule

```text
PHYSICAL_MVP_GO =
  every MVP1-P0-* software row PASS
  AND EXT-01..04C evidence accepted
  AND BlueCash-50 fiscal+card HIL PASS
  AND DP-150 MX COM fiscal HIL PASS
  AND BluePad-50 Plus BLE card HIL PASS
  AND DP-150+BluePad combined HIL PASS
  AND Daisy Compact S 01 USB fiscal HIL PASS
  AND zero unresolved UNKNOWN/RECOVERY_REQUIRED test transaction

PILOT_GO = PHYSICAL_MVP_GO AND EXT-05 AND approved operational runbook

PRODUCTION_GO = PILOT_GO AND EXT-06 AND EXT-07
                AND production ESP32/Android security gates applicable
```

До выполнения этой формулы единственный корректный итог — `NO-GO`, даже если
unit, simulator E2E и compose regression зелёные.
