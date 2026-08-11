export type BleDeploymentAuthority={tenant_id:string;register_id:string;device_id:string};
export type ExpectedBleDeploymentAuthority={register_id:string;device_id:string};

export function validateBleDeploymentAuthority(session:BleDeploymentAuthority,expected:ExpectedBleDeploymentAuthority){
 if(!session.tenant_id)throw new Error("BLE_TENANT_BINDING");
 if(session.register_id!==expected.register_id)throw new Error("BLE_REGISTER_BINDING");
 if(session.device_id!==expected.device_id)throw new Error("BLE_DEVICE_BINDING");
 return session;
}
