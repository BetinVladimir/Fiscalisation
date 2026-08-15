// Non-destructive contract smoke test. Requires an already enrolled tenant.
const base = process.env.BEEFISCAL_BASE_URL?.replace(/\/$/, "");
const token = process.env.BEEFISCAL_TENANT_TOKEN;
if (!base || !token) throw new Error("BEEFISCAL_BASE_URL and BEEFISCAL_TENANT_TOKEN are required");
const key = crypto.randomUUID(), source = `conformance-${crypto.randomUUID()}`;
const headers = { Authorization:`Bearer ${token}`, "Content-Type":"application/json", "Idempotency-Key":key, "Source-Version":"1", "BeeFiscal-Source-Actor-Type":"SERVICE", "BeeFiscal-Source-Actor-Id":"integration-kit-conformance" };
const send = () => fetch(`${base}/integration/v1/locations/${source}`, { method:"PUT", headers, body:JSON.stringify({ display_name:"Conformance location", active:false }) });
const first = await send(), firstBody = await first.json(); if (first.status !== 202) throw new Error(`first mutation ${first.status}: ${JSON.stringify(firstBody)}`);
const replay = await send(), replayBody = await replay.json(); if (replay.status !== 202 || replayBody.operation_id !== firstBody.operation_id) throw new Error("idempotent replay returned another operation");
const changed = await fetch(`${base}/integration/v1/locations/${source}`, { method:"PUT", headers, body:JSON.stringify({ display_name:"changed" }) });
if (changed.status !== 409) throw new Error(`idempotency mismatch must be 409, got ${changed.status}`);
console.log(`PASS operation=${firstBody.operation_id}`);
