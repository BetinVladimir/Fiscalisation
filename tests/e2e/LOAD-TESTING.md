# E2E concurrency gate

Каждый поднятый Fiscalisation E2E stack должен дополнительно пройти
`concurrent-gate.mjs` по безопасным health/query endpoints. Текущий обязательный
MVP baseline: Docker 4 CPU/7 GiB, 4 worker threads, 25 проходов на worker и
endpoint, error rate <0.5%, p95 <1000 ms.

```bash
E2E_CONCURRENCY_URLS='http://127.0.0.1:8080/health' node tests/e2e/concurrent-gate.mjs
```

Stateful fiscal sale/device tests сохраняют последовательное выполнение: общий
fiscal device или register нельзя безопасно использовать конкурентно. Gate
должен получать только идемпотентные или read-only endpoints.

Будущий профиль запускается с `E2E_TARGET_EVIDENCE=1`: Docker 8 CPU/16 GiB,
8 workers, 250 проходов, p95 <800 ms. Он остаётся future scale-out gate.
