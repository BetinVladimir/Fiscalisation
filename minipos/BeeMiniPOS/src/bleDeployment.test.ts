import test from "node:test";
import assert from "node:assert/strict";
import {validateBleDeploymentAuthority} from "./bleDeployment.ts";

const session={tenant_id:"tenant-from-server",register_id:"register-a",device_id:"device-a"};
const expected={register_id:"register-a",device_id:"device-a"};

test("BLE authority accepts the server-owned tenant and matches register and final device",()=>{
 assert.equal(validateBleDeploymentAuthority(session,expected).tenant_id,"tenant-from-server");
 for(const changed of [
  {...session,tenant_id:""},
  {...session,register_id:"register-b"},
  {...session,device_id:"device-b"},
 ])assert.throws(()=>validateBleDeploymentAuthority(changed,expected),/BLE_(TENANT|REGISTER|DEVICE)_BINDING/);
});
