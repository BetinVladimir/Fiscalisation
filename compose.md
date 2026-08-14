# Docker Compose configuration

В репозитории используются две схемы Docker Compose:

- `docker-compose.yml` — старый объединённый локальный стек;
- `compose.fiscalisation*.yaml` и `compose.minipos*.yaml` — раздельные стеки BeeFiscal и BeeMiniPOS с дополнительными настройками для разных окружений.

## Принцип сборки конфигурации

Файлы `*.dev.yaml`, `*.prod.yaml` и `*.e2e.yaml` являются overlay-файлами. Они не описывают полный стек и должны передаваться Docker Compose после соответствующего базового файла. Docker Compose объединяет файлы слева направо: значения из последующих файлов дополняют или переопределяют базовую конфигурацию.

Таким образом, конфигурация организована по двум измерениям:

| Продукт | Базовый файл | Доступные overlay-файлы |
|---|---|---|
| BeeFiscal | `compose.fiscalisation.yaml` | `dev`, `prod`, `e2e` |
| BeeMiniPOS | `compose.minipos.yaml` | `dev`, `prod`, `e2e` |

## BeeFiscal

### `compose.fiscalisation.yaml`

Базовый стек BeeFiscal:

- PostgreSQL с отдельной базой `fiscal` и persistent volume;
- `fiscal-backend`;
- Caddy как HTTP/HTTPS ingress;
- внутренняя сеть для связи сервисов;
- отдельные параметры подключения к обычной и RLS-роли PostgreSQL;
- подключение каталога device PKI только для чтения.

Файл содержит общую конфигурацию, используемую во всех окружениях.

### `compose.fiscalisation.dev.yaml`

Настройки локальной разработки:

- устанавливает `APP_ENV=dev`;
- разрешает stub-адаптеры;
- настраивает Caddy на обычный HTTP (`:80`).

Пример запуска:

```bash
docker compose \
  -p beefiscal-dev \
  -f compose.fiscalisation.yaml \
  -f compose.fiscalisation.dev.yaml \
  up --build
```

### `compose.fiscalisation.prod.yaml`

Production-настройки:

- устанавливает `APP_ENV=prod`;
- запрещает stub-адаптеры;
- требует явно передать публичный URL, CORS, OIDC, MQTT и ключи подписи;
- задаёт production URI для MQTT и deployment descriptor SPA;
- переводит backend в режим `read_only`;
- удаляет Linux capabilities и включает `no-new-privileges`.

Файл намеренно использует выражения `${VARIABLE:?required}`: запуск должен завершиться ошибкой, если обязательный production-секрет или адрес не задан.

Пример запуска:

```bash
docker compose \
  -f compose.fiscalisation.yaml \
  -f compose.fiscalisation.prod.yaml \
  up -d
```

### `compose.fiscalisation.e2e.yaml`

Дополнительная изоляция для end-to-end тестов:

- данные PostgreSQL размещаются в `tmpfs` и исчезают после остановки стека;
- backend подключается к PostgreSQL через Unix socket;
- обычные сети сервисов заменяются внутренней сетью `dbroute`;
- init-скрипты базы подключаются только для чтения.

E2E-конфигурация накладывается поверх базового и dev-файла:

```bash
docker compose \
  -f compose.fiscalisation.yaml \
  -f compose.fiscalisation.dev.yaml \
  -f compose.fiscalisation.e2e.yaml \
  up --build
```

На практике эту комбинацию формирует `scripts/e2e-two-compose.sh`.

## BeeMiniPOS

### `compose.minipos.yaml`

Базовый стек BeeMiniPOS:

- отдельная PostgreSQL с базой `minipos`;
- `beeminipos-backend`;
- web-приложение `miniposweb`;
- Caddy как ingress;
- отдельные private, ingress и egress сети;
- параметры интеграции с BeeFiscal;
- отдельные обычная и RLS-роли PostgreSQL.

### `compose.minipos.dev.yaml`

Настройки локальной разработки:

- устанавливает `APP_ENV=dev`;
- настраивает Caddy на обычный HTTP (`:80`).

Пример запуска:

```bash
docker compose \
  -p beeminipos-dev \
  -f compose.minipos.yaml \
  -f compose.minipos.dev.yaml \
  up --build
```

### `compose.minipos.prod.yaml`

Production-настройки:

- устанавливает `APP_ENV=prod`;
- требует OIDC/OAuth-параметры, CORS и production hostname;
- требует ключ проверки webhook и ключ локальной фискальной подписи;
- отключает статические dev-токены;
- переводит backend в режим `read_only`;
- удаляет Linux capabilities и включает `no-new-privileges`.

Пример запуска:

```bash
docker compose \
  -f compose.minipos.yaml \
  -f compose.minipos.prod.yaml \
  up -d
```

### `compose.minipos.e2e.yaml`

E2E-overlay выполняет для MiniPOS ту же функцию, что и `compose.fiscalisation.e2e.yaml` для BeeFiscal:

- временная база данных в `tmpfs`;
- подключение через Unix socket;
- изолированная сеть `dbroute`;
- init-скрипты базы только для чтения.

Он также применяется третьим слоем после базового и dev-файла.

## Отличие от `docker-compose.yml`

`docker-compose.yml` поднимает единый локальный стек, включающий PostgreSQL, Redis, EMQX, RabbitMQ и оба backend-сервиса. В нём оба backend используют одну базу `fiscal`, а инфраструктурные порты публикуются на host.

Новая схема решает другую задачу:

- разделяет BeeFiscal и BeeMiniPOS на независимые Compose projects;
- использует разные базы данных и учётные данные;
- ограничивает сетевую связность;
- разделяет dev, production и E2E-политику;
- добавляет production hardening и обязательную проверку конфигурации.

Поэтому overlay-файлы не дублируют `docker-compose.yml`: они являются частями новой раздельной схемы развёртывания.

## Проверка итоговой конфигурации

Перед запуском можно вывести результат объединения файлов без создания контейнеров:

```bash
docker compose \
  -f compose.fiscalisation.yaml \
  -f compose.fiscalisation.dev.yaml \
  config
```

Аналогичные проверки dev- и production-комбинаций входят в цель `compose-check` в `Makefile`.
