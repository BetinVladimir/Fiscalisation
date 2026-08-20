# BeeFiscal Platform Admin

React Native / Expo приложение — административная панель платформы фискализации. Одна точка входа, один файл (`App.tsx`), два раздела.

---

## Авторизация

`src/usePlatformOidc.ts` — хук поверх `expo-auth-session`:

- **Протокол:** OIDC + PKCE (`ResponseType.Code` с `code_verifier`)
- **Scopes:** `openid`, `profile`, `beefiscal.platform`
- **Env:** `EXPO_PUBLIC_PLATFORM_OIDC_ISSUER`, `EXPO_PUBLIC_PLATFORM_OIDC_CLIENT_ID`
- **Redirect URI:** `beefiscalplatformadmin://oauth/callback`
- **Токен:** хранится в памяти (`useState`). Logout — обнуляет токен, refresh token не используется.

До входа — только кнопка "Sign in with OIDC + PKCE". После — весь интерфейс.

---

## Раздел: Devices

Управление инвентарём фискальных устройств.

**Список:** `GET /platform/v1/devices?serial=&state=`

**Карточка устройства:** `serial`, `state`, `tenant_id`, `firmware_version`, `manufacturing_batch`, `device_key_thumbprint`, `hardware_revision`, `binding_version`.

**Переходы состояний** — `POST /platform/v1/devices/{id}:{action}` с `Idempotency-Key` и `version` (optimistic locking):

| Кнопка | Action | Требует |
|---|---|---|
| Assign tenant | `assign-tenant` | `tenant_id` |
| Unassign | `unassign-tenant` | — |
| Suspend | `suspend` | — |
| Resume | `resume` | — |
| Retire | `retire` | `reason` (обязателен) |

---

## Раздел: External Systems

Управление внешними системами-интеграторами.

**При открытии загружает параллельно:**
- `GET /platform/v1/external-systems`
- `GET /platform/v1/enrollment-conflicts`
- `GET /platform/v1/integration-metrics`

**Integration health** (read-only): `command_backlog`, `command_dead`, `webhook_backlog`, `webhook_dead`, `enrollment_conflicts`, `otp_locked`.

**Регистрация:** `POST /platform/v1/external-systems` → возвращает `bootstrap_token`, показывается один раз.

**Действия над системой:**
- **Rotate key** → `:rotate-key` (возвращает новый токен)
- **Suspend / Resume enrollment** → `:suspend` / `:resume`
- **Edit** — PATCH с `If-Match: version`

**Журнал системы** (при выборе):
- `GET .../audit-events` — лог действий
- `GET .../tenant-bindings` — привязанные тенанты
- `GET .../webhook-deliveries` — история доставки; для статуса `DEAD` — кнопка **Requeue** (`POST .../webhook-deliveries/{id}:requeue`)

**Enrollment conflicts** — конфликт фискального идентификатора между тенантами. Разрешение требует `reason`:
- **Keep existing** — оставить старого тенанта
- **Block existing and continue** — заменить

---

## Ключевые детали

- Все мутации используют `Idempotency-Key: crypto.randomUUID()`
- Переходы устройств передают `version` — защита от race condition
- PATCH использует `If-Match` заголовок
- API base URL: `EXPO_PUBLIC_PLATFORM_API_URL` (по умолчанию `http://localhost:8080`)
- Состояние полностью in-memory, персистентности нет
