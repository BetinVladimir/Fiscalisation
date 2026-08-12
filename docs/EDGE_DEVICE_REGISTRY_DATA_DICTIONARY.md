# Edge Device Registry data dictionary

Authoritative DDL: `fiscal-backend/internal/persistence/migrations/20260812_device_registry.sql`.

| Table | Ownership | Purpose | Retention |
|---|---|---|---|
| `fiscal_device_registry` | Platform | Immutable manufacturing identity and device lifecycle | Device lifetime + regulatory archive |
| `fiscal_manufacturing_stations` | Platform security | Workload identities allowed to register hardware | Indefinite audit record |
| `fiscal_device_bindings_v2` | Tenant/RLS | Tenant, location, register and role bindings with fencing version | Indefinite history; never hard-delete active history |
| `fiscal_actor_installations` | Tenant/RLS | Admin/POS installation public keys | Until revoke + audit retention |
| `fiscal_device_capabilities` | Tenant/RLS | Capability metadata and signed-object digest, never private keys | Expiry + security audit retention |
| `fiscal_device_auth_challenges` | Platform/device auth | One-time proof-of-possession challenges | TTL cleanup after consumption/expiry |
| `fiscal_device_revocations` | Platform security | Monotonic device/capability/installation revocation feed | Indefinite |

Security invariants:

- device, Admin and POS private keys are never columns;
- `MANUFACTURED` devices have no tenant;
- only one pending/active binding exists per device;
- only one active fiscal role exists per register;
- tenant-owned binding/installation/capability tables enforce PostgreSQL RLS;
- registry/manufacturing access requires platform or manufacturing workload identity;
- every mutation increments optimistic `version` or `binding_version` and emits audit evidence.
