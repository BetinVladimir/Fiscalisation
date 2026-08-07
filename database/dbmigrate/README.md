# dbmigrate

Миграции PostgreSQL (формат `golang-migrate`).

## Структура
- `migrations/*.up.sql` — миграции вверх
- `migrations/*.down.sql` — откат

## Пример запуска через Docker
```bash
docker run --rm -v $(pwd)/database/dbmigrate/migrations:/migrations \
  --network host migrate/migrate \
  -path=/migrations \
  -database "postgres://postgres:postgres@localhost:5432/fiscal?sslmode=disable" up
```
