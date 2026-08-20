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
