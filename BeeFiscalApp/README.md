# BeeFiscalApp

Expo React Native приложение для мониторинга и администрирования своего тенанта.

## Запуск
```bash
npm install
npm run start
```

## Назначение
- мониторинг состояния кассовых устройств
- обзор ключевых метрик по тенанту
- администрирование tenant-настроек (дальнейшее развитие)

## Тесты

```bash
npm test
EXPO_PUBLIC_APP_ENV=dev \
EXPO_PUBLIC_REGISTER_ID=00000000-0000-4000-8000-000000000001 \
EXPO_PUBLIC_FISCAL_API_URL=http://fiscal-admin.test/public/v1 \
npm run build:e2e
npm run test:e2e
```

Playwright-набор проверяет HTTP-контракты диагностики, printer test,
provisioning, BLE session, SmartDevice activation, UNKNOWN reconciliation,
отчётов и audit-фильтров. Реальные BLE, secure element, фискальное и банковское
оборудование остаются в HIL/instrumentation suites.

<!-- deployment-and-store-listing:start -->
## Deployment and store listing

Canonical deployment and store metadata for **BeeFiscal**. Store links identify the configured product pages; they may remain unavailable publicly while a listing is in draft or internal testing.

| Environment | Web | iOS bundle ID | App Store | Android package | Google Play |
| --- | --- | --- | --- | --- | --- |
| Production | [fiscal.beeloy.org](https://fiscal.beeloy.org) | `org.beeloy.beefiscal.prod` | [App Store](https://apps.apple.com/app/id6814689623) | `org.beeloy.beefiscal.prod` | [Google Play](https://play.google.com/store/apps/details?id=org.beeloy.beefiscal.prod) |
| Demo | [demo-fiscal.beeloy.org](https://demo-fiscal.beeloy.org) | `org.beeloy.beefiscal.demo` | [App Store](https://apps.apple.com/app/id6814689855) | `org.beeloy.beefiscal.demo` | [Google Play](https://play.google.com/store/apps/details?id=org.beeloy.beefiscal.demo) |

### Deployment commands

- Production: `npm run deploy:web`
- Demo: `npm run deploy:web-demo`
- Cloudflare configuration: [wrangler.jsonc](./wrangler.jsonc)

### Icons

The icon represents **fiscal receipts and compliance**, with the small Beeloy bee mark at the lower-right side.

- Application and iOS icon: [store/icon.png](./store/icon.png)
- Android adaptive foreground: [store/adaptive-icon.png](./store/adaptive-icon.png)
- App Store artwork, 1024 × 1024: [store/graphics/app-store-icon-1024.png](./store/graphics/app-store-icon-1024.png)
- Google Play artwork, 512 × 512: [store/graphics/google-play-icon-512.png](./store/graphics/google-play-icon-512.png)
- Editable vector source: [store/graphics/icon.svg](./store/graphics/icon.svg)

### Store description (en-US)

**Title:** BeeFiscal

**Short description:** Monitor your BeeFiscal devices, diagnostics and tenant activity.

**Full description:**

> BeeFiscal gives authorized users a mobile view of their fiscal device environment. Monitor device status and tenant metrics, review reports and audit information, and access supported diagnostic workflows.
>
> Device setup and connection features depend on compatible hardware and the services enabled for your tenant. This app requires access to a configured BeeFiscal environment.
>
> An account and access to the corresponding service are required. Features depend on your permissions and the services enabled by your organization.

Source: [store/listing.en-US.json](./store/listing.en-US.json). The same copy is used for production and demo unless a store-specific override is documented later.
<!-- deployment-and-store-listing:end -->
