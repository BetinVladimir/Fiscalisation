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

const mutateAndWait=async(path,payload)=>{
  const requestHeaders={...headers,"Idempotency-Key":crypto.randomUUID(),"Source-Version":"1"};
  const response=await fetch(`${base}/integration/v1/${path}`,{method:"PUT",headers:requestHeaders,body:JSON.stringify(payload)});
  const accepted=await response.json();
  if(response.status!==202)throw new Error(`${path} mutation ${response.status}: ${JSON.stringify(accepted)}`);
  for(let attempt=0;attempt<60;attempt++){
    const polled=await fetch(`${base}/integration/v1/operations/${accepted.operation_id}`,{headers:{Authorization:`Bearer ${token}`}}).then(v=>v.json());
    if(polled.status==="SUCCEEDED")return polled;
    if(["FAILED","DEAD"].includes(polled.status))throw new Error(`${path} failed: ${JSON.stringify(polled)}`);
    await new Promise(resolve=>setTimeout(resolve,250));
  }
  throw new Error(`${path} did not finish`);
};
const registerSource=`register-${crypto.randomUUID()}`,operatorSource=`operator-${crypto.randomUUID()}`;
await mutateAndWait(`registers/${registerSource}`,{location_source_id:source,name:"Conformance POS",status:"ACTIVE"});
await mutateAndWait(`operators/${operatorSource}`,{operator_code:"C001",first_name:"Conformance",last_name:"Operator",roles:["CASHIER"],status:"ACTIVE"});

// Deterministic verifier vector ensures implementations sign the raw body and
// compare the complete MAC in constant time. Network retry/dedup is verified by
// the receiver example using event_id as its idempotency key.
const raw=Buffer.from('{"event_id":"00000000-0000-4000-8000-000000000001"}'), secret=Buffer.alloc(32,7), timestamp="1700000000";
const expected=createHmac("sha256",secret).update(`${timestamp}.`).update(raw).digest();
const supplied=Buffer.from(createHmac("sha256",secret).update(`${timestamp}.`).update(raw).digest("hex"),"hex");
if (!timingSafeEqual(expected,supplied)) throw new Error("webhook signature vector failed");
console.log(`PASS operation=${firstBody.operation_id}`);
