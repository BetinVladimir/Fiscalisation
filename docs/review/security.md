# Уязвимости безопасности

**Дата:** 2026-08-21

---

## Backend (Go) — fiscal-backend, beeminipos-backend, edge-agent

---

### S-01 — [HIGH] Auth middleware полностью пропускает запросы при отсутствии провайдера

**Файлы:**
- `fiscal-backend/internal/auth/auth.go`
- `minipos/beeminipos-backend/internal/auth/auth.go`

**Описание:**  
`MiddlewareWithOIDCAndRevocation` возвращает `next` без обёртки — без аутентификации, без логирования, без инъекции claims — если оба `secret` (HMAC ключ) и `oidc` (OIDC verifier) не заданы:

```go
if secret == "" && oidc == nil {
    return next  // любой запрос достигает защищённых обработчиков без идентификации
}
```

Аналогичная проблема в beeminipos-backend:
```go
func MiddlewareWithRevocation(secret string, ...) http.Handler {
    if secret == "" {
        return next
    }
```

**Эксплойт:** При деплое без переменной `AUTH_HMAC_KEY` (случайно, в CI без секретов, в staging) все защищённые эндпоинты становятся полностью открытыми.

**Рекомендация:**
```go
if secret == "" && oidc == nil {
    panic("auth: необходимо настроить AUTH_HMAC_KEY или OIDC")
}
```

---

### S-02 — [HIGH] Захардкоженные дефолтные криптографические секреты

**Файл:** `fiscal-backend/internal/config/config.go`

**Описание:**  
Три криптографических секрета имеют известные дефолтные значения:

| Секрет | Дефолт |
|--------|--------|
| `BLE_SIGNING_KEY` | `"dev-only-ble-signing-key-32-bytes"` |
| `INTEGRATION_SECRET_PEPPER` | `"dev-only-integration-pepper-32-bytes"` |
| `INTEGRATION_ENCRYPTION_KEY_BASE64` | `"MDEy..."` (AES ключ = `0123456789abcdef0123456789abcdef`) |

AES ключ виден в исходном коде и шифрует учётные данные интеграции в БД. Prod-валидация отклоняет эти значения, но staging/review окружения используют их молча.

**Рекомендация:**  
Убрать все дефолты. Использовать `mustenv()` для обязательных секретов — паниковать при старте если переменная не задана.

---

### S-03 — [HIGH] `authorizeSale()` разрешает доступ при отсутствии JWT claims

**Файл:** `fiscal-backend/internal/api/handler.go`

**Описание:**
```go
func (h *handler) authorizeSale(r *http.Request, saleID string) bool {
    tid := tenantID(r)
    if tid == "" {
        return true  // нет tenant claim → доступ к любой продаже разрешён
    }
    ...
}
```
При обходе middleware (см. S-01) или при токене без tenant claim любой неаутентифицированный вызов получает доступ к продажам всех тенантов.

**Рекомендация:**
```go
if tid == "" {
    return false  // отклонить при отсутствии tenant claim
}
```

---

### S-04 — [MEDIUM] Path-matching в `Allowed()` использует `strings.Contains()` — хрупкий

**Файл:** `fiscal-backend/internal/auth/auth.go`

**Описание:**
```go
if strings.Contains(path, "/diagnostics") {
    return hasAny(c, "ADMIN", "SERVICE")
}
if strings.Contains(path, "/admin/") {
    return hasAny(c, "ADMIN")
}
```
Путь `/cashier-diagnostics` или `/public/v1/admin/bulk-export` может случайно совпасть с защищёнными паттернами.

**Рекомендация:**  
Использовать точный prefix-матчинг:
```go
case path == "/internal/v1/diagnostics" || strings.HasPrefix(path, "/internal/v1/diagnostics/"):
```

---

### S-05 — [MEDIUM] Header injection в `Content-Disposition` через URL-сегмент

**Файл:** `fiscal-backend/internal/api/handler.go`

**Описание:**
```go
w.Header().Set("Content-Disposition",
    `attachment; filename="compliance-export-`+parts[2]+exportExtension(media)+`"`)
```
`parts[2]` берётся напрямую из URL запроса. Значение вида `x"; filename="evil.exe` может сформировать некорректный заголовок.

**Рекомендация:**
```go
re := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
safePart := re.ReplaceAllString(parts[2], "_")
```

---

### S-06 — [MEDIUM] Авторизация полностью отключена в non-prod окружениях (beeminipos-backend)

**Файл:** `minipos/beeminipos-backend/internal/api/handler.go`

**Описание:**
```go
func (h *handler) authorizeEmployeeActor(...) bool {
    if !strings.EqualFold(h.c.AppEnv, "prod") {
        return true  // staging/dev — авторизация полностью пропускается
    }
    ...
}
```
Любое staging-окружение с `APP_ENV != "prod"` запускает полностью открытый API.

**Рекомендация:**  
Полностью убрать bypass по окружению. Использовать тестовые фикстуры вместо отключения проверок.

---

### S-07 — [MEDIUM] Внутренние ошибки утекают клиентам через поле `detail`

**Файл:** `minipos/beeminipos-backend/internal/api/handler.go`

**Описание:**  
`problem()` возвращает сырую строку ошибки в поле `detail` JSON-ответа, включая ошибки БД, SQL table names, сетевые ошибки.

**Рекомендация:**  
Логировать полную ошибку на сервере, клиенту возвращать correlation ID и общее сообщение.

---

### S-08 — [MEDIUM] SMTP не принудительно использует TLS

**Файл:** `minipos/beeminipos-backend/internal/emailauth/service.go`

**Описание:**  
`smtp.SendMail` использует STARTTLS оппортунистически — если сервер не объявляет STARTTLS, библиотека прозрачно переходит на plaintext. OTP-коды и email пользователей передаются без шифрования.

**Рекомендация:**  
Использовать явное TLS-соединение (порт 465, implicit TLS / SMTPS), либо `DialAndStartTLS` с проверкой `TLSEnabled()` перед отправкой.

---

### S-09 — [MEDIUM] CORS wildcard разрешён в production конфиге (fiscal-backend)

**Файл:** `fiscal-backend/internal/config/config.go`

**Описание:**  
Дефолт `CORS_ALLOWED_ORIGINS = "*"`. Prod-валидация содержит логическую ошибку: `c.CORSAllowedOrigins != "*"` — это условие НЕ выполняется для `"*"`, поэтому `"*"` проходит валидацию как допустимое prod-значение.

**Рекомендация:**
```go
if c.AppEnv == "prod" && (c.CORSAllowedOrigins == "*" || !secureOrigins(c.CORSAllowedOrigins)) {
    return errors.New("CORS_ALLOWED_ORIGINS должен быть явным HTTPS списком в PROD")
}
```

---

### S-10 — [LOW] Минимальный размер RSA ключа для OIDC — 1024 бита (устарело)

**Файл:** `fiscal-backend/internal/auth/oidc.go`

**Описание:**  
JWKS verifier отклоняет ключи короче 1024 бит. NIST SP 800-131A признал 1024-битный RSA устаревшим в 2013 году. Минимум должен быть 2048 бит.

**Рекомендация:**  
Изменить проверку: `if len(n) < 256` (2048 бит).

---

### S-11 — [LOW] DNS rebinding в SSRF-protection transport

**Файл:** `fiscal-backend/internal/webhook/safe_transport.go`

**Описание:**  
DNS разрешается и IP валидируется только при первом соединении. При HTTP keep-alive переиспользуется существующее соединение, игнорируя изменение DNS (rebinding-атака).

**Рекомендация:**  
`transport.DisableKeepAlives = true` — форсировать новое разрешение DNS на каждый запрос.

---

## Frontend (TypeScript/React) — miniposweb, BeeMiniPOS, BeeFiscalApp, AdminApp

---

### S-12 — [HIGH] Access и refresh токены хранятся в `localStorage`

**Файлы:**
- `minipos/miniposweb/src/App.tsx` (строки 53–55, 87–93)
- `minipos/miniposweb/src/AuthFlow.tsx` (строка 19)

**Описание:**  
`minipos-access-token` и `minipos-refresh-token` хранятся в `localStorage`, доступном синхронно любому JS в том же origin. XSS-атака = кража refresh token = бессрочный доступ.

**Рекомендация:**  
Использовать `HttpOnly` cookies — сервер устанавливает, JS не может читать. В мобильном приложении `expo-secure-store` применяется корректно — надо применить тот же подход в web.

---

### S-13 — [HIGH] JWT декодируется на клиенте без проверки подписи

**Файлы:**
- `minipos/miniposweb/src/App.tsx` (строки 35–43, функция `tenantFrom()`)
- `BeeFiscalApp/src/oidcRoles.ts` (строки 9–33)

**Описание:**  
`tenantFrom()` и `accessTokenRoles()` декодируют JWT через `atob()` без верификации подписи. Извлечённый `tenant_id` встраивается в `FiscalIntent` и формирует `payload_sha256`. При подмене токена (XSS, компрометация session store) злоумышленник может встроить произвольный `tenant_id` в фискальный intent.

**Рекомендация:**  
`tenant_id` для построения intent должен извлекаться из аутентифицированной серверной сессии (`/operator-session`), а не из сырого JWT на клиенте.

---

### S-14 — [HIGH] BLE канал работает в режиме `OPEN_MVP` — нет шифрования фискальных данных

**Файлы:**
- `minipos/BeeMiniPOS/src/webBle.ts` (строки 77–78)
- `minipos/BeeMiniPOS/src/nativeBle.native.ts` (строки 68–69)

**Описание:**  
Код отклоняет любую сессию с `security_mode !== "OPEN_MVP"` бросая `BLE_SECURITY_MODE_UNSUPPORTED`. В режиме `OPEN_MVP` фискальные intent-фреймы (CBOR) передаются незашифрованными с только SHA-256 для целостности.

Злоумышленник в радиусе Bluetooth может:
1. Пассивно перехватить фискальные данные (наименования товаров, цены, operator codes, tenant IDs)
2. Проводить relay или replay атаки

Реализация `X25519_AES_GCM` (ECDH + HKDF + AES-GCM) существует в `bleHandshake.ts`, но недостижима через bootstrap-код.

**Рекомендация:**  
Активировать `X25519_AES_GCM` путь. Отключить `OPEN_MVP` в production сборках через feature flag.

---

### S-15 — [HIGH] BLE `authenticate()` всегда бросает исключение — crypto provider не реализован

**Файлы:**
- `BeeFiscalApp/src/smartDeviceBle.native.ts` (строка 80)
- `BeeFiscalApp/src/smartDeviceBle.web.ts` (строка 38)

**Описание:**  
В обеих реализациях метод `authenticate()` безусловно бросает `EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE`. `AUTH_V2` payload отправляется на устройство до броска исключения — устройство получает неаутентифицированные данные. Провизионирование устройств через этот путь полностью сломано.

**Рекомендация:**  
Реализовать crypto provider (используя `@noble/curves`, уже есть в `portableCrypto.ts`). До реализации этот путь не должен быть доступен в production.

---

### S-16 — [MEDIUM] `prepareSessionPublicKey()` возвращает случайные байты вместо X25519 ключа

**Файл:** `minipos/BeeMiniPOS/src/nativeBle.native.ts` (строка 53)

**Описание:**  
`prepareSessionPublicKey()` возвращает `base64urlEncode(randomBytes(32))` — 32 случайных байта, не X25519 публичный ключ. Согласование ключей на стороне устройства завершится с ошибкой.

**Рекомендация:**  
Заменить на `x25519Pair().publicKey` из модуля `portableCrypto`.

---

### S-17 — [MEDIUM] Детерминированный AES-GCM nonce в BLE AUTH_PROOF — риск повторного использования

**Файл:** `minipos/BeeMiniPOS/src/bleHandshake.ts` (строки 183–192)

**Описание:**  
12-байтный nonce для AES-GCM вычисляется как `hash256("BeeFiscal BLE auth nonce|" + session_id).slice(0, 12)`. Детерминированный nonce при коллизии session_id ведёт к катастрофическому нарушению AES-GCM: XOR двух шифртекстов раскрывает оба открытых текста и позволяет подделывать ciphertext.

**Рекомендация:**  
Использовать `randomBytes(12)` для AUTH_PROOF nonce, включать его в сообщение.

---

### S-18 — [MEDIUM] `adapter_base_url` из конфигурации сервера используется как HTTP endpoint без валидации

**Файлы:**
- `minipos/miniposweb/src/App.tsx` (строки 363–366, 442–445)
- `minipos/miniposweb/src/api/client.ts` (`LocalFiscalClient`, строка 66)

**Описание:**  
`adapter_base_url` из `/configuration` API используется напрямую для `LocalFiscalClient`. При компрометации конфигурационного канала атакующий может направить клиент на внешний сервер, куда будут отправляться аутентифицированные фискальные intents.

**Рекомендация:**  
Проверять, что URL резолвится в private IP (127.0.0.1, 192.168.x.x, 10.x.x.x). При не-local URL — явный вопрос пользователю.

---

### S-19 — [MEDIUM] OTP: нет client-side обработки rate-limit ответов

**Файлы:**
- `minipos/miniposweb/src/AuthFlow.tsx`
- `minipos/BeeMiniPOS/src/emailAuth.ts`
- `BeeFiscalApp/src/emailOtpAuth.ts`

**Описание:**  
6-значный числовой OTP имеет 1,000,000 комбинаций. Клиент не различает HTTP 429 от других ошибок — все ловятся как `Error(text)`. Автоматический перебор практически осуществим без серверного ограничения.

**Рекомендация:**  
Обрабатывать HTTP 429 отдельно с чётким сообщением. Использовать 8-значный или буквенно-цифровой OTP.

---

### S-20 — [MEDIUM] Onboarding token хранится в состоянии React без срока жизни

**Файлы:**
- `minipos/miniposweb/src/AuthFlow.tsx`
- `minipos/BeeMiniPOS/src/emailAuth.ts`

**Описание:**  
`onboarding_token` хранится в React state без ограничения времени жизни. При незавершённом onboarding-флоу токен остаётся активным до размонтирования компонента.

**Рекомендация:**  
Устанавливать клиентский TTL (10 минут). Очищать токен сразу после успешного или неуспешного onboarding.

---

### S-21 — [LOW] OIDC logout не отзывает токен на Authorization Server

**Файл:** `AdminApp/src/usePlatformOidc.ts` (строки 32, 59)

**Описание:**  
`logout()` устанавливает `setToken("")` без вызова OIDC `end_session_endpoint`. Токен остаётся валидным на AS до истечения TTL.

**Рекомендация:**  
Вызывать `end_session_endpoint` при logout.
