import test from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {BleResultWaiters} from "./bleResult.ts";
const path=process.env.CBOR_DELIVERY_FIXTURES;
test("actual production result bytes preserve bigint evidence through waiters", {skip:!path}, async()=>{
 const rows=readFileSync(path!,"utf8").trim().split("\n").map(x=>JSON.parse(x));assert.equal(rows.length,12);
 for(const row of rows){const w=new BleResultWaiters();const pending=w.wait(row.id,1000);assert.equal(w.accept(row.id,Buffer.from(row.hex,"hex")),true);const result=await pending;assert.equal(result.state,row.state);assert.equal(result.fiscal_reference,"FD-1");assert.equal((result as any).payment_evidence[0].amount_minor,row.amount==="2147483648"?2147483648:BigInt(row.amount));assert.deepEqual((result as any).empty,[]);assert.equal((result as any).nullable,null);}
});
