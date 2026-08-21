# BeeFiscal Platform — Комплексный технический аудит

**Дата:** 2026-08-21  
**Охват:** Backend (Go), Frontend (TypeScript/React), IoT (C++), Инфраструктура (Docker Compose, БД)  
**Методология:** Статический анализ исходного кода всех компонентов платформы

---

## Структура отчёта

| Документ | Содержание |
|----------|------------|
| [security.md](security.md) | Уязвимости безопасности (auth, crypto, injection, transport) |
| [scalability.md](scalability.md) | Проблемы масштабируемости (БД, пагинация, память, соединения) |
| [bugs.md](bugs.md) | Логические ошибки и баги (бизнес-логика, гонки, IoT) |
| [infrastructure.md](infrastructure.md) | DevOps: Docker Compose, healthcheck, БД, CI/CD |

---

## Сводная таблица критических находок (HIGH severity)

| # | Компонент | Категория | Описание |
|---|-----------|-----------|----------|
| S-01 | fiscal-backend | Security | Auth middleware полностью пропускает запросы если нет провайдера аутентификации |
| S-02 | fiscal-backend | Security | Захардкоженные дефолтные секреты (AES ключ, HMAC, pepper) |
| S-03 | fiscal-backend | Security | `authorizeSale()` разрешает доступ когда JWT claims отсутствуют |
| S-04 | miniposweb | Security | Access/refresh токены в `localStorage` — доступны через XSS |
| S-05 | miniposweb | Security | JWT декодируется без проверки подписи; `tenant_id` встраивается в фискальный intent |
| S-06 | BeeMiniPOS | Security | BLE канал не зашифрован (`OPEN_MVP`) — фискальные данные в открытом виде |
| S-07 | BeeMiniPOS | Security | BLE `authenticate()` всегда бросает исключение — криптографический провайдер не реализован |
| I-01 | Инфраструктура | Security | CORS `*` wildcard в production compose |
| I-02 | Инфраструктура | Security | Device CA приватный ключ смонтирован в API-контейнер |
| I-03 | Инфраструктура | Security | `sslmode=disable` на всех соединениях с БД |
| I-04 | Инфраструктура | Security | Dev-секреты с известными значениями активны если prod overlay не подключён |
| SC-01 | fiscal-backend | Scalability | Все listing-эндпоинты загружают весь тенант в память без пагинации |
| SC-02 | fiscal-backend | Scalability | N+1 запрос в `operations()` при фильтрации по `register_id` |
| SC-03 | miniposweb | Scalability | IndexedDB открывается и закрывается на каждой операции |
| B-01 | fiscal-backend | Bug | `newID()` на основе nanosecond timestamp — коллизии при нескольких инстансах |
| B-02 | miniposweb | Bug | Плавающая точка в денежных суммах — ошибки округления в фискальных чеках |
| B-03 | miniposweb | Bug | Матчинг позиций корзины по имени товара, а не ID — неверный `product_id` |
| B-04 | miniposweb | Bug | `shifts.items[0]` без проверки — `undefined` смена без уведомления |
| I-05 | Инфраструктура | Operational | PostgreSQL контейнер без `restart: unless-stopped` |

---

## Приоритеты устранения

### Блокируют деплой в production
1. **S-01** — убрать bypass аутентификации при отсутствии провайдера
2. **S-03** — `authorizeSale()` должен отклонять при отсутствии tenant claim
3. **I-01** — запретить CORS `*` в prod
4. **I-03** — включить `sslmode=require` в DATABASE_URL
5. **B-01** — заменить `newID()` на UUID v4

### До следующего релиза
6. **S-02** + **I-04** — убрать все дефолтные значения секретов
7. **S-06** — BLE шифрование (`X25519_AES_GCM`) должно быть активировано
8. **SC-01**, **SC-02** — пагинация и N+1 в listing-эндпоинтах
9. **B-02** — целочисленная арифметика для денежных сумм
10. **I-05** — добавить `restart: unless-stopped` для PostgreSQL

### Среднесрочно
11. **S-04** — токены в HttpOnly cookies вместо localStorage
12. **S-07** — реализовать BLE crypto provider
13. **I-02** — вынести подпись CA сертификатов в отдельный sidecar
14. Healthcheck для всех контейнеров
15. Resource limits для контейнеров
