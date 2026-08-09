import test from "node:test";
import assert from "node:assert/strict";
import {authorizedAdvertisingIdentity,matchesAdvertisingIdentity,requireAdvertisingIdentity} from "./bleAdvertising.ts";

test("BLE discovery accepts only the REST-authorized advertising identity",()=>{
 assert.equal(matchesAdvertisingIdentity({name:"edge-authorized"},"edge-authorized"),true);
 assert.equal(matchesAdvertisingIdentity({localName:"edge-authorized"},"edge-authorized"),true);
 assert.equal(matchesAdvertisingIdentity({name:"edge-foreign"},"edge-authorized"),false);
 assert.equal(matchesAdvertisingIdentity({},"edge-authorized"),false);
});

test("BLE discovery rejects missing or oversized advertising authority",()=>{
 assert.throws(()=>requireAdvertisingIdentity("  "),/BLE_ADVERTISING_IDENTITY_INVALID/);
 assert.throws(()=>requireAdvertisingIdentity("x".repeat(129)),/BLE_ADVERTISING_IDENTITY_INVALID/);
});

test("BLE advertising identity is bound to the signed Edge identity",()=>{
 assert.equal(authorizedAdvertisingIdentity("edge-authorized","edge-authorized"),"edge-authorized");
 assert.throws(()=>authorizedAdvertisingIdentity("edge-foreign","edge-authorized"),/BLE_ADVERTISING_AUTHORITY_MISMATCH/);
});
