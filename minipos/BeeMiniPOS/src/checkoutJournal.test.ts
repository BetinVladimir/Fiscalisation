import test from "node:test";
import assert from "node:assert/strict";
import { CheckoutJournal, type CheckoutStore } from "./checkoutJournal.ts";
class Memory implements CheckoutStore {
  m = new Map<string, string>();
  async getItem(k: string) {
    return this.m.get(k) ?? null;
  }
  async setItem(k: string, v: string) {
    this.m.set(k, v);
  }
  async removeItem(k: string) {
    this.m.delete(k);
  }
  async getAllKeys() {
    return [...this.m.keys()];
  }
}
const entry = {
  sale_id: "sale-1",
  sale_version: 3,
  created_at: "2026-08-13T00:00:00Z",
  plan: {
    client_operation_id: "00000000-0000-4000-8000-000000000001",
    receipt_session_id: "00000000-0000-4000-8000-000000000002",
    payments: [{ payment_id: "p" }],
    expected_total: { amount: "1.00", currency: "EUR" as const },
  },
};
test("checkout survives restart with the exact operation receipt and payments", async () => {
  const store = new Memory(),
    a = new CheckoutJournal(store);
  await a.save(entry);
  const b = new CheckoutJournal(store),
    loaded = await b.load(entry.sale_id);
  assert.deepEqual(loaded, entry);
  await b.remove(entry.sale_id);
  assert.equal(await a.load(entry.sale_id), null);
});
test("corrupt checkout fails closed", async () => {
  const store = new Memory();
  await store.setItem("beeminipos:checkout:v1:sale-1", "{}");
  await assert.rejects(new CheckoutJournal(store).load("sale-1"), /CORRUPT/);
});
test("pending checkout is recovered after process restart with operation correlation", async () => {
  const store = new Memory(),
    before = new CheckoutJournal(store);
  await before.save(entry);
  const after = new CheckoutJournal(store);
  const pending = await after.pending();
  assert.equal(pending.length, 1);
  assert.equal(pending[0].sale_id, "sale-1");
  assert.equal(
    pending[0].plan.client_operation_id,
    "00000000-0000-4000-8000-000000000001",
  );
});
test('journal refuses operation/payload replacement and preserves signed bytes across restart',async()=>{
 const store=new Memory(),journal=new CheckoutJournal(store);
 const signed={...entry,source_order_id:'source-1',execution_scope:'scope',signed_intent:{intent_id:entry.plan.client_operation_id,receipt_session_id:entry.plan.receipt_session_id,execution_authorization:{payload:'same'},client_proof:{signature:'DER'},canonical_payload:{server_sale_id:null}}};
 await journal.save(signed);await assert.rejects(()=>journal.save({...signed,signed_intent:{...signed.signed_intent,client_proof:{signature:'changed'}}}),/IMMUTABLE/);
 await assert.rejects(()=>journal.save({...signed,plan:{...signed.plan,client_operation_id:'new-operation'}}),/IMMUTABLE/);
 assert.deepEqual((await new CheckoutJournal(store).load(entry.sale_id))?.signed_intent,signed.signed_intent);
});
test('concurrent writers cannot replace the first operation',async()=>{const store=new Memory(),one=new CheckoutJournal(store),two=new CheckoutJournal(store);const results=await Promise.allSettled([one.save(entry),two.save({...entry,plan:{...entry.plan,client_operation_id:'replacement'}})]);assert.equal(results.filter(r=>r.status==='fulfilled').length,1);assert.equal((await one.load(entry.sale_id))?.plan.client_operation_id,entry.plan.client_operation_id);});
test('ambiguous authorization preparation survives restart and permits only one signed transition',async()=>{const store=new Memory(),journal=new CheckoutJournal(store);const pending={...entry,source_order_id:'source',phase:'AUTHORIZING' as const,authorization_request_key:'prepare-same-request',source_session_hash:'same-session'};await journal.save(pending);assert.equal((await new CheckoutJournal(store).load(entry.sale_id))?.authorization_request_key,'prepare-same-request');await assert.rejects(()=>journal.save({...pending,authorization_request_key:'second-prepare'}),/IMMUTABLE/);const plan={...entry.plan,client_operation_id:'root-op',receipt_session_id:'root-receipt'};await journal.save({...pending,phase:'SIGNED',plan,execution_scope:'scope',signed_intent:{intent_id:'root-op',receipt_session_id:'root-receipt',execution_authorization:{payload:'root'},client_proof:{signature:'DER'}}});assert.equal((await journal.load(entry.sale_id))?.plan.client_operation_id,'root-op');await assert.rejects(()=>journal.save(pending),/IMMUTABLE|JSON_REQUIRED/);});
