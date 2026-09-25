# BeeFiscal Platform Admin

React Native / Expo приложение — административная панель платформы фискализации. Одна точка входа, один файл (`App.tsx`), два раздела.

---

## Авторизация

`src/usePlatformOidc.ts` — хук поверх `expo-auth-session`:

- **Протокол:** OIDC + PKCE (`ResponseType.Code` с `code_verifier`)
- **Scopes:** `openid`, `profile`, `beefiscal.platform`
- **Env:** `EXPO_PUBLIC_PLATFORM_OIDC_ISSUER`, `EXPO_PUBLIC_PLATFORM_OIDC_CLIENT_ID`
- **Redirect URI:** `beeloy-fiscaladmin-prod://oauth/callback (demo: beeloy-fiscaladmin-demo://oauth/callback)`
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

<!-- deployment-and-store-listing:start -->
## Deployment and store listing

Canonical deployment and store metadata for **BeeFiscal Admin**. Store links identify the configured product pages; they may remain unavailable publicly while a listing is in draft or internal testing.

| Environment | Web | iOS bundle ID | App Store | Android package | Google Play |
| --- | --- | --- | --- | --- | --- |
| Production | [fiscal-admin.beeloy.org](https://fiscal-admin.beeloy.org) | `org.beeloy.fiscaladmin.prod` | [App Store](https://apps.apple.com/app/id6814689414) | `org.beeloy.fiscaladmin.prod` | [Google Play](https://play.google.com/store/apps/details?id=org.beeloy.fiscaladmin.prod) |
| Demo | [demo-fiscal-admin.beeloy.org](https://demo-fiscal-admin.beeloy.org) | `org.beeloy.fiscaladmin.demo` | [App Store](https://apps.apple.com/app/id6814689882) | `org.beeloy.fiscaladmin.demo` | [Google Play](https://play.google.com/store/apps/details?id=org.beeloy.fiscaladmin.demo) |

### Deployment commands

- Production: `npm run deploy:web`
- Demo: `npm run deploy:web-demo`
- Cloudflare configuration: [wrangler.jsonc](./wrangler.jsonc)

### Icons

The icon represents **fiscal administration**, with the small Beeloy bee mark at the lower-right side.

- Application and iOS icon: [store/icon.png](./store/icon.png)
- Android adaptive foreground: [store/adaptive-icon.png](./store/adaptive-icon.png)
- App Store artwork, 1024 × 1024: [store/graphics/app-store-icon-1024.png](./store/graphics/app-store-icon-1024.png)
- Google Play artwork, 512 × 512: [store/graphics/google-play-icon-512.png](./store/graphics/google-play-icon-512.png)
- Editable vector source: [store/graphics/icon.svg](./store/graphics/icon.svg)

### Store description (en-US)

**Title:** BeeFiscal Admin

**Short description:** Administer BeeFiscal devices and external system integrations.

**Full description:**

> BeeFiscal Admin is an administrative app for authorized BeeFiscal platform operators. View fiscal device inventory and device details, manage tenant assignments and perform supported device lifecycle actions.
>
> Review external system integrations, enrollment conflicts and integration health indicators. Access requires a configured organization sign-in provider and platform administrator permissions.
>
> An account and access to the corresponding service are required. Features depend on your permissions and the services enabled by your organization.

Source: [store/listing.en-US.json](./store/listing.en-US.json). The same copy is used for production and demo unless a store-specific override is documented later.
<!-- deployment-and-store-listing:end -->
