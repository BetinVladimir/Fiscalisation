# BeeFiscal integration kit

External systems use HTTPS only. RabbitMQ and Fiscal PostgreSQL are private implementation details.

1. Ask a platform integration administrator to register the system and reveal its bootstrap token once.
2. Start and verify company enrollment using a unique caller-generated `Idempotency-Key` for each logical action.
3. Encrypt the returned tenant credential in a tenant-scoped secret store; never log it.
4. Send resource mutations with a stable source ID, monotonic `Source-Version`, and source actor headers.
5. Poll the returned operation URL and also process signed webhooks idempotently by event ID.

The contract is [openapi.yaml](./openapi.yaml). Production onboarding requires passing the conformance suite. Bulk OTP onboarding is intentionally outside the first release.

`typescript/` contains a dependency-free reference client and constant-time webhook verifier. Persist each idempotency key before sending and reuse it after a timeout. Verify the signature against the raw body before parsing JSON.

Run the non-destructive acceptance/idempotency smoke test with `node conformance/check.mjs` after setting `BEEFISCAL_BASE_URL` and `BEEFISCAL_TENANT_TOKEN`.

Additional artifacts include JSON Schemas, a Postman collection, a local raw-body webhook receiver, and the production readiness/compatibility policy. Sandbox credentials are created through AdminApp in the isolated sandbox environment; they are never committed to this kit.

See [SANDBOX.md](./SANDBOX.md) for automated isolated provisioning and the full Rabbit-backed conformance run.

Open `reference.html` through any static HTTP server for the rendered API reference. CI lints the OpenAPI document and rejects breaking contract changes. The TypeScript reference client intentionally has no runtime dependency; regenerate API types from `openapi.yaml` with `npx openapi-typescript openapi.yaml -o typescript/types.generated.ts` when the contract changes.
