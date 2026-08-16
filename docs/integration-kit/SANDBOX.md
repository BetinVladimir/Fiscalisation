# Integration sandbox

The repository sandbox uses the same HTTP, PostgreSQL and RabbitMQ paths as production. It does not expose RabbitMQ or PostgreSQL to an external-system client.

Start it with a dedicated non-production signing key:

```sh
AUTH_HMAC_KEY=server2server-e2e-signing-key-32-bytes \
docker compose -f compose.fiscalisation.yaml -f compose.fiscalisation.dev.yaml up -d --build fiscal-backend
sh scripts/e2e-server2server.sh
```

The provisioning test creates a unique external system, performs OTP enrollment, verifies that the Fiscal organization exists, and exercises location, register and operator commands through RabbitMQ. Its webhook URL uses the reserved `.invalid` domain, so test tenant data cannot be delivered outside the sandbox. Tokens remain process-local and are never printed or committed.

For a shared sandbox, an integration administrator creates the external system through AdminApp and transfers the reveal-once bootstrap token through the approved secret manager. Test tenant enrollment still uses a real mailbox OTP; there is no onboarding bypass.
