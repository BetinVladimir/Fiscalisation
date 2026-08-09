import test from "node:test";
import assert from "node:assert/strict";
import {validateBleDeploymentAuthority} from "./bleDeployment.ts";

const expected={tenant_id:"tenant-a",register_id:"register-a",device_id:"device-a"};

test("BLE authority matches the active tenant register and final device",()=>{
 assert.equal(validateBleDeploymentAuthority({...expected},expected).device_id,"device-a");
 for(const changed of [
  {...expected,tenant_id:"tenant-b"},
  {...expected,register_id:"register-b"},
  {...expected,device_id:"device-b"},
 ])assert.throws(()=>validateBleDeploymentAuthority(changed,expected),/BLE_(TENANT|REGISTER|DEVICE)_BINDING/);
});
