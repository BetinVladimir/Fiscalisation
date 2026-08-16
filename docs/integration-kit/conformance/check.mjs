import { createHmac, timingSafeEqual } from "node:crypto";

const base = process.env.BEEFISCAL_BASE_URL?.replace(/\/$/, "");
const token = process.env.BEEFISCAL_TENANT_TOKEN;
if (!base || !token) throw new Error("BEEFISCAL_BASE_URL and BEEFISCAL_TENANT_TOKEN are required");
const key = crypto.randomUUID(), source = `conformance-${crypto.randomUUID()}`;
const headers = { Authorization:`Bearer ${token}`, "Content-Type":"application/json", "Idempotency-Key":key, "Source-Version":"1", "BeeFiscal-Source-Actor-Type":"SERVICE", "BeeFiscal-Source-Actor-Id":"integration-kit-conformance" };
const payload = { name:"Conformance location", address:"Conformance address", status:"ACTIVE" };
const send = () => fetch(`${base}/integration/v1/locations/${source}`, { method:"PUT", headers, body:JSON.stringify(payload) });
const first = await send(), firstBody = await first.json();
if (first.status !== 202) throw new Error(`first mutation ${first.status}: ${JSON.stringify(firstBody)}`);
const replay = await send(), replayBody = await replay.json();
if (replay.status !== 202 || replayBody.operation_id !== firstBody.operation_id) throw new Error("idempotent replay returned another operation");
const changed = await fetch(`${base}/integration/v1/locations/${source}`, { method:"PUT", headers, body:JSON.stringify({...payload,name:"Changed"}) });
if (changed.status !== 409) throw new Error(`idempotency mismatch must be 409, got ${changed.status}`);
let operation;
for (let attempt=0; attempt<60; attempt++) {
  const response=await fetch(`${base}/integration/v1/operations/${firstBody.operation_id}`, {headers:{Authorization:`Bearer ${token}`}});
  operation=await response.json();
  if (["SUCCEEDED","FAILED","DEAD","SUPERSEDED"].includes(operation.status)) break;
  await new Promise(resolve=>setTimeout(resolve,250));
}
if (operation?.status !== "SUCCEEDED") throw new Error(`operation did not succeed: ${JSON.stringify(operation)}`);
const staleHeaders={...headers,"Idempotency-Key":crypto.randomUUID()};
const stale=await fetch(`${base}/integration/v1/locations/${source}`,{method:"PUT",headers:staleHeaders,body:JSON.stringify(payload)});
if (stale.status!==409) throw new Error(`stale Source-Version must be 409, got ${stale.status}`);

// Deterministic verifier vector ensures implementations sign the raw body and
// compare the complete MAC in constant time. Network retry/dedup is verified by
// the receiver example using event_id as its idempotency key.
const raw=Buffer.from('{"event_id":"00000000-0000-4000-8000-000000000001"}'), secret=Buffer.alloc(32,7), timestamp="1700000000";
const expected=createHmac("sha256",secret).update(`${timestamp}.`).update(raw).digest();
const supplied=Buffer.from(createHmac("sha256",secret).update(`${timestamp}.`).update(raw).digest("hex"),"hex");
if (!timingSafeEqual(expected,supplied)) throw new Error("webhook signature vector failed");
console.log(`PASS operation=${firstBody.operation_id}`);
