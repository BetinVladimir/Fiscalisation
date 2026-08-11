import test from "node:test";
import assert from "node:assert/strict";
import {encodeCanonical} from "./cbor.ts";
import {BleResultWaiters} from "./bleResult.ts";

const id="00112233-4455-4677-8899-aabbccddeeff";
test("correlated result resolves exactly one waiter",async()=>{const w=new BleResultWaiters(),result=w.wait(id,1000);assert.equal(w.accept(id,encodeCanonical({operation_id:id,state:"FISCALIZED",fiscal_reference:"FD-1"})),true);assert.equal((await result).fiscal_reference,"FD-1");assert.equal(w.accept(id,encodeCanonical({operation_id:id,state:"FISCALIZED"})),false)});
test("message and operation correlation mismatch rejects",async()=>{const w=new BleResultWaiters(),result=w.wait(id,1000);w.accept(id,encodeCanonical({operation_id:"11112233-4455-4677-8899-aabbccddeeff",state:"FISCALIZED"}));await assert.rejects(result,/BLE_RESULT_INVALID/)});
test("timeout and send rejection do not leak waiter",async()=>{const w=new BleResultWaiters();await assert.rejects(w.wait(id,100),/BLE_RESULT_TIMEOUT/);const again=w.wait(id,1000);w.reject(id,new Error("WRITE_FAILED"));await assert.rejects(again,/WRITE_FAILED/)});
