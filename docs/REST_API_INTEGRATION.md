Да, тогда SD-карта сильно упрощает задачу. В вашем случае ESP32-S3 camera module можно использовать не как «кэш в памяти», а как локальный web-origin с постоянной копией POS-приложения на SD. ESP-IDF поддерживает HTTP-сервер на ESP32-S3 и работу с SD-картами через SDMMC/SDSPI, поэтому такая схема технически штатная. 
Я бы построил это так:
                 Internet
                     │
                     ▼
             Cloud POS Backend
             + SPA releases
                     │
                 HTTPS sync
                     │
                     ▼
            ┌─────────────────┐
            │    ESP32-S3     │
            │                 │
            │  HTTP server    │
            │  Local API      │
            │  Sync engine    │
            └────────┬────────┘
                     │
                 microSD
                     │
        ┌────────────┼────────────┐
        │            │            │
      SPA v42     SPA v43     POS data
                                  │
                                  ├─ sales
                                  ├─ queue
                                  └─ logs

Browser
   │
   ▼
http(s)://pos-box.local/
   │
   └──────────► ESP32-S3
ESP32-S3 умеет использовать SD через SDMMC, причем сам S3 имеет SDMMC host с несколькими режимами шины; конкретная camera-плата, конечно, может ограничивать доступные GPIO из-за камеры и разводки SD. 
Для SPA я бы вообще не делал proxy-cache отдельных HTTP-запросов. Лучше хранить на SD полноценный release:
/sdcard/
    spa/
        current.txt

        releases/
            2026.08.10-1/
                index.html
                assets/
                    app.a821.js
                    app.91c2.css
                    logo.svg

            2026.08.11-1/
                index.html
                assets/
                    app.c312.js
                    app.b411.css

    data/
        transactions/
        sync/
        config/
current.txt содержит, например:
2026.08.11-1
Когда браузер делает:
GET /
ESP32 фактически отдаёт:
/sdcard/spa/releases/2026.08.11-1/index.html
А:
GET /assets/app.c312.js
отдаёт файл непосредственно с SD.
Это гораздо надежнее, чем динамически выяснять при каждом запросе, есть интернет или нет.
Для обновления я бы использовал manifest:
{
  "version": "2026.08.12-3",
  "files": [
    {
      "path": "index.html",
      "sha256": "..."
    },
    {
      "path": "assets/app.9213.js",
      "sha256": "..."
    }
  ],
  "signature": "..."
}
ESP32 делает:
GET cloud/releases/latest.json

          │
          ▼
version > current?
          │
         yes
          │
          ▼
создать releases/new-version/
          │
          ▼
скачать файлы
          │
          ▼
проверить SHA256
          │
          ▼
проверить подпись manifest
          │
          ▼
атомарно изменить current.txt
И особенно важный момент: старую версию сразу не удалять.
current = v43
previous = v42
downloading = v44
Если интернет исчез во время скачивания v44:
v43 остается рабочей
Если новая версия оказалась битой:
rollback → v42
Это дает практически transaction-like обновление frontend.
Для самого browser POS получается очень простая модель. Он всегда открывает:
http://pos-box.local/
или, если решите вопрос с сертификатом:
https://pos-box.local/
А API использует relative URLs:
await fetch("/api/v1/fiscal/status");

await fetch("/api/v1/sale", {
  method: "POST",
  headers: {
    "Content-Type": "application/json"
  },
  body: JSON.stringify(sale)
});
Получается один origin:
pos-box.local
├── /
├── /assets/*
└── /api/*
Поэтому вообще исчезает значительная часть проблем с:
CORS
Private Network Access
public-site → LAN
разными IP кассового модуля
ESP32-S3 выступает шлюзом.
Самое интересное — SD-карту я бы использовал не только для SPA. Для вашей POS-системы намного ценнее возможность сделать durable transaction journal:
/data/transactions/
    01989A...json
    01989B...json
    01989C...json
Например:
{
  "id": "01989b83-...",
  "createdAt": "2026-08-11T00:53:12+03:00",
  "state": "fiscalized",
  "cloudSync": false,

  "sale": {
    "items": []
  },

  "fiscal": {
    "receiptNo": "18342",
    "result": "OK"
  }
}
После появления интернета:
SD queue
   │
   ▼
Cloud API
   │
   ▼
ACK
   │
   ▼
transaction.cloudSync = true
И здесь я бы придерживался жесткого правила:
кассовая операция считается завершенной не после ответа frontend и не после попытки отправки в cloud, а только после того, как ESP32 сохранил её состояние на постоянный носитель.
Например:
Browser
  │
  │ POST /api/sale ID=123
  ▼
ESP32
  │
  ├─► SD: sale 123 = processing
  │
  ├─► fiscal module
  │
  ├─► SD: sale 123 = fiscalized
  │
  └─► Browser: SUCCESS
А не:
Browser
  │
  ▼
Fiscal module
  │
 SUCCESS
  │
  ▼
потом когда-нибудь пишем SD    ← риск
Потому что при выключении питания между этими событиями можно получить фискализированный чек, о котором POS «забыл».
Есть только один момент, который я бы проверил до проектирования платы/firmware: конкретную модель ESP32-S3 camera board. На camera-модулях камера и microSD нередко делят или сильно расходуют GPIO, DMA и внутренние ресурсы. Сам ESP32-S3 поддерживает camera+microSD-платы — у Espressif даже есть S3-платы, где одновременно интегрированы камера и microSD, — но конкретная разводка вашей платы определяет доступный режим SD и свободные GPIO. 
В целом SD на десятки гигабайт для SPA — огромный запас. Размер накопителя здесь почти не будет ограничением. Куда важнее будут надежность записи, корректное восстановление после power loss и производительность при одновременных операциях camera + SD + Wi-Fi + HTTP + fiscal API.
Для POS я бы в итоге разделил задачи ESP32 так:
Core / tasks

HTTP
 ├─ static SPA from SD
 └─ REST/WebSocket API

POS
 ├─ fiscal protocol
 └─ peripheral control

Storage
 ├─ SPA versions
 ├─ transaction journal
 └─ configuration

Network
 ├─ Wi-Fi/LAN
 ├─ cloud sync
 └─ update downloader
И это уже выглядит как вполне жизнеспособный offline-first POS appliance, а не просто ESP32, раздающий несколько HTML-файлов.

Да. В таком варианте я бы разделил облачную авторизацию, локальную авторизацию ESP32 и подпись каждой локальной команды. При этом мобильное приложение и web-приложение могут использовать практически одинаковый протокол.
Предлагаемая схема:
                        INTERNET
                            │
                            │ HTTPS
                            ▼
                   ┌─────────────────┐
                   │  Cloud REST API │
                   │                 │
                   │ auth            │
                   │ device registry │
                   │ secret issuing  │
                   └───────┬─────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
          Mobile App                Web POS
              │                         │
              │ accessToken             │ accessToken
              │                         │
              │ GET /pos/device         │ GET /pos/device
              ▼                         ▼
       получает:                  получает:
       ESP32 address              ESP32 address
       local token                local token
       HMAC secret                HMAC secret
              │                         │
              └────────────┬────────────┘
                           │
                           │ LOCAL NETWORK
                           │ HTTP
                           ▼
                ┌────────────────────┐
                │     ESP32-S3       │
                │                    │
                │ HTTP Web Server    │
                │ REST API           │
                │ HMAC verifier      │
                │ SPA cache on SD    │
                │ transaction queue  │
                └────────────────────┘
1. Обычная авторизация пользователя
И mobile app, и web POS сначала работают с обычным облачным API:
POST https://api.example.com/auth/login
Получают, например:
{
  "accessToken": "eyJ...",
  "expiresIn": 3600
}
Это обычный cloud access token.
После этого клиент запрашивает кассовое устройство:
GET https://api.example.com/api/v1/pos/device
Authorization: Bearer eyJ...
Cloud знает, какая ESP32 привязана к данной торговой точке/кассе, и возвращает:
{
  "deviceId": "POS-F82A71",
  "localAddress": "http://192.168.1.52",
  "localToken": "lt_7Fa...",
  "hmacKey": "base64:...",
  "expiresAt": "2026-08-11T04:00:00Z"
}
Но я бы немного изменил эту структуру: localToken и hmacKey должны быть отдельными сущностями.
Например:
Cloud access token
       │
       │ используется только Cloud API
       ▼
Cloud
       │
       ├── localSessionToken
       │
       └── sessionHmacKey
Это позволяет никогда не передавать основной cloud access token на ESP32.

2. Локальная сессия ESP32
Cloud создает короткоживущую локальную сессию:
sessionId = 8b72...
deviceId  = POS-F82A71
expires   = +8h
key       = 256 random bits
ESP32 должна каким-то образом знать этот же ключ.
Здесь есть два хороших варианта.
Если ESP32 имеет интернет:
Cloud
  │
  │ HTTPS
  ▼
ESP32
Cloud передает/активирует session key на ESP32.
И только после этого отдает его клиенту.
Получается:
Mobile/Web             Cloud                 ESP32
    │                    │                     │
    │ Bearer token       │                     │
    ├───────────────────►│                     │
    │                    │ create session      │
    │                    │────────────────────►│
    │                    │ session/key         │
    │                    │◄────────────────────│
    │ local token + key  │                     │
    │◄───────────────────│                     │
После этого интернет может исчезнуть.
       INTERNET
           X

Phone/Web ──HTTP──► ESP32
             HMAC
Локальная POS-система продолжает работать.

3. Каждый запрос к ESP32 подписывается HMAC
Например клиент хочет создать продажу:
POST http://192.168.1.52/api/v1/sales
Authorization: Local lt_abc123
X-Timestamp: 1786401125
X-Nonce: 01989ca5-...
X-Content-SHA256: 54f8...
X-Signature: ...
Тело:
{
  "transactionId": "01989ca5-...",
  "items": [
    {
      "sku": "1234",
      "qty": 1,
      "price": 12.50
    }
  ]
}
Canonical string для подписи я бы определил жестко:
POST
/api/v1/sales
1786401125
01989ca5-...
54f8c12d...SHA256(body)
lt_abc123
И затем:
signature =
HMAC-SHA256(
    sessionHmacKey,
    canonicalRequest
)
ESP32 делает те же вычисления и сравнивает подпись.
Таким образом нельзя:
изменить JSON;
заменить URL;
заменить HTTP method;
повторно использовать старый запрос;
создать новую кассовую команду без ключа.

4. Обязательно nonce + anti-replay
Для кассы это особенно важно.
Без защиты от replay злоумышленник, записав:
POST /api/v1/sales
может повторить запрос.
Поэтому ESP32 хранит:
session
 ├── sessionId
 ├── expiresAt
 └── recentlySeenNonces
Если:
nonce уже использован
ESP32 отвечает:
409 Conflict
или:
{
  "error": "REPLAY_DETECTED"
}
А сама продажа дополнительно защищается:
transactionId
То есть даже при повторной доставке:
transactionId = abc
ESP32 отвечает существующим результатом, но не делает второй чек.
Это очень важное свойство для фискальных операций.

5. Web-приложение находится на ESP32
ESP32 хранит SPA на SD:
/sd/
  web/
    current/
      index.html
      assets/
        app.js
        app.css
Браузер открывает:
http://192.168.1.52/
ESP32 возвращает POS UI.
Получаем same-origin:
http://192.168.1.52/
http://192.168.1.52/api/v1/status
http://192.168.1.52/api/v1/sales
Поэтому web frontend делает просто:
fetch("/api/v1/sales", ...)
CORS для ESP32 API вообще не нужен.

6. Но web-приложение должно получить secret из Cloud
Это тоже нормально.
Хотя сама страница загружена через:
http://192.168.1.52/
она может обращаться к:
https://api.example.com
Cloud должен разрешить нужный origin через CORS.
Схема:
Browser

1. GET http://192.168.1.52/
          │
          ▼
       ESP32 SD
          │
          ▼
       POS SPA


2. SPA
    │
    │ HTTPS
    ▼
https://api.example.com
    │
    │ Authorization: Bearer ...
    ▼
Cloud
    │
    ▼
local session credentials


3. SPA
    │
    │ HTTP + Local token + HMAC
    ▼
192.168.1.52
ESP32
То есть frontend имеет два API:
const cloudApi = "https://api.example.com";

const localApi = window.location.origin;

7. Идеальный bootstrap
Я бы сделал endpoint:
POST https://api.example.com/api/v1/local-sessions
Authorization: Bearer CLOUD_ACCESS_TOKEN
Content-Type: application/json

{
  "deviceId": "POS-F82A71"
}
Ответ:
{
  "sessionId": "ls_01989...",
  "localToken": "lcl_7fas...",
  "hmacKey": "2a0db....",
  "device": {
    "id": "POS-F82A71",
    "url": "http://192.168.1.52"
  },
  "expiresAt": "2026-08-11T09:00:00Z"
}
После этого клиент может работать offline.

8. Что должно храниться в web POS
Я бы не сохранял HMAC secret в localStorage.
Лучше держать его:
JS memory
а если нужно переживать перезагрузку/потерю интернета:
IndexedDB
с дополнительной защитой настолько, насколько это возможно в browser environment.
Но здесь есть принципиальное ограничение вашей схемы.
HTTP + Web App + secret
Если страница работает через обычный:
http://192.168.1.52
то HMAC защищает целостность и аутентичность, но не конфиденциальность.
То есть:
Phone
  │
  │ HTTP
  ▼
ESP32
человек, способный перехватывать Wi-Fi-трафик, потенциально может увидеть:
localToken
request body
response body
И если сам hmacKey когда-либо передается ESP32 через HTTP — это уже критическая ошибка.
Но в предлагаемой схеме:
Cloud ─HTTPS─► Browser
ключ приходит шифрованно.
А в HTTP-запросах к ESP32 ключ никогда не передается:
hmacKey      ← не передаем

HMAC(message) ← передаем
Поэтому пассивный наблюдатель ключ из запроса непосредственно не получает.
Однако XSS или активная MITM-атака против HTTP web-приложения — более серьезная проблема: злоумышленник, способный подменить загружаемый JS, сможет украсть секрет после его получения из cloud.
Это главный security trade-off такой архитектуры.

9. Поэтому я бы добавил проверку целостности SPA
Cloud знает hash текущего web bundle:
{
  "webVersion": "4.32.1",
  "bundleHash": "sha256:..."
}
ESP32 скачивает только подписанный release:
manifest.json
manifest.sig
и проверяет цифровую подпись производителя до активации.
Например:
Cloud private signing key
        │
        │ Ed25519 signature
        ▼
manifest
        │
        ▼
ESP32 embedded public key
Это защищает SD от простой подмены файлов.
Но это не заменяет HTTPS между браузером и ESP32 при наличии активного злоумышленника в LAN.

10. Mobile App значительно безопаснее
Для native app схема выглядит особенно хорошо:
                    CLOUD

iOS/Android ─HTTPS────► API
     │                  │
     │                  ├─ ESP address
     │                  ├─ local token
     │                  └─ HMAC session key
     │
     │ HTTP + HMAC
     ▼
ESP32
Secret хранится:
iOS     → Keychain
Android → Keystore
Нет загружаемого через HTTP JavaScript, который можно подменить.
Поэтому для native клиента HTTP + HMAC намного разумнее, чем для браузерного клиента.

11. Я бы еще немного изменил выдачу адреса ESP32
Не доверял бы IP полностью только cloud:
{
  "localAddress": "192.168.1.52"
}
потому что DHCP может поменять адрес.
Cloud скорее должен возвращать:
{
  "deviceId": "POS-F82A71",
  "hostname": "pos-f82a71.local",
  "lastKnownAddress": "192.168.1.52"
}
Клиент делает:
1. попробовать lastKnownAddress
2. попробовать mDNS hostname
3. discovery _pos-agent._tcp.local
И после соединения обязательно проверяет:
GET /api/v1/device
ESP отвечает:
{
  "deviceId": "POS-F82A71"
}
И этот ответ также должен участвовать в HMAC challenge.
Таким образом нельзя случайно подключиться к соседней кассе.

12. Полная схема
Я бы в итоге сделал именно так:
                      ┌──────────────────┐
                       │   CLOUD BACKEND  │
                       │                  │
                       │ Auth             │
                       │ POS registry     │
                       │ session service  │
                       │ SPA releases     │
                       └───────┬──────────┘
                               │ HTTPS
                    ┌──────────┴───────────┐
                    │                      │
                    ▼                      ▼
             iOS / Android              Browser
                    │                      │
                    │                      │
              cloud token            cloud token
                    │                      │
                    ├──────────┬───────────┤
                               │
                               ▼
                         Cloud REST
                               │
                               ▼
                      local credentials
                      ┌────────────────┐
                      │ deviceId       │
                      │ address        │
                      │ sessionId      │
                      │ localToken     │
                      │ HMAC key       │
                      │ expiresAt      │
                      └───────┬────────┘
                              │
                    LOCAL NETWORK
                              │
                              │ HTTP
                              │ Local token
                              │ HMAC
                              ▼
                     ┌─────────────────┐
                     │    ESP32-S3     │
                     │                 │
                     │ REST API        │
                     │ HMAC verifier   │
                     │ anti-replay     │
                     │ transaction DB  │
                     │ sync queue      │
                     │                 │
                     │ microSD         │
                     │ └── cached SPA  │
                     └────────┬────────┘
                              │
                    fiscal / peripherals
И lifecycle:
ONLINE BOOTSTRAP

Client
  │
  ├──► Cloud login
  │
  ├──► obtain device/session
  │
  └──► ESP32 authenticate


NORMAL

Client ──HTTP + HMAC──► ESP32
ESP32  ──HTTPS────────► Cloud


WAN FAILURE

               X Cloud

Client ──HTTP + HMAC──► ESP32
                         │
                         ├── fiscal operations
                         └── durable offline queue


WAN RESTORED

ESP32 ──HTTPS──► Cloud
       sync queue
Один важный выбор
Я бы выдавал локальную HMAC-сессию не на 1 час, а, например, на рабочую смену, с возможностью offline grace period.
Иначе возникает неприятный сценарий:
09:00 login
10:00 интернет пропал
10:01 local session expired
и касса внезапно перестает работать именно тогда, когда offline-режим нужен больше всего.
Лучше:
online validity:       например 8–12 часов
offline grace:         например до конца текущей смены
renew whenever online
revocation:            при следующем online sync
А для критических операций обязательно оставить transactionId + HMAC + nonce + durable journal. Именно эта комбинация даст устойчивость к повторным запросам, перезагрузке телефона, ESP32 и обрыву связи.


