# BeeFiscal Admin: store release

App: `fiscaladmin`. Production: `org.beeloy.fiscaladmin.prod`; demo: `org.beeloy.fiscaladmin.demo`.

Общая инструкция находится в `BeeloyBackend/store-release/STORE-RELEASE.md`, отчёт проверок — в `READINESS.md` рядом с ней.

`npm run store:check` проверяет локальную конфигурацию и показывает недостающие внешние настройки. `deploy:android` / `deploy:ios` собирают и отправляют; demo — с суффиксом `:demo`.
