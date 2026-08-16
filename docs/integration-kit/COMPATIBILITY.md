# Compatibility policy

The date-based API version is backward compatible for additive fields and new
endpoints. Removing a field, changing authority, tightening an accepted enum,
or changing signature/idempotency semantics requires a new API version.
Consumers must ignore unknown response fields, but Fiscal rejects unknown
request fields. Deprecations are announced for at least 180 days.

Production readiness requires: HTTPS webhook with raw-body verification,
durable event-id deduplication, encrypted tenant credentials, stable source
IDs and monotonic versions, idempotency persistence before send, operation
polling fallback, and passing `conformance/check.mjs`.
