export const EDGE_V2_SERVICE="7b6f2000-7c6d-4c7a-9e4f-424545464953";
export const EDGE_V2_DEVICE_INFO="7b6f2001-7c6d-4c7a-9e4f-424545464953";
export const EDGE_V2_SESSION_CONTROL="7b6f2002-7c6d-4c7a-9e4f-424545464953";
export const EDGE_V2_SESSION_RX="7b6f2003-7c6d-4c7a-9e4f-424545464953";
export const EDGE_V2_SESSION_TX="7b6f2004-7c6d-4c7a-9e4f-424545464953";
export type DeviceInfo={protocol_version:2;device_id:string;serial:string;firmware_version:string;nonce:string};
export type DeploymentCapability={version:2;device_id:string;capability_id:string;permissions:string[];expires_at:number;signature:string};
export interface SecureSmartDeviceConnection{deviceInfo:DeviceInfo;authenticate(capability:DeploymentCapability):Promise<void>;setWifi(ssid:string,password:string):Promise<void>;bindStore(locationId:string,registerId:string,roles:string[]):Promise<void>;disconnect():Promise<void>}
export async function connectEdgeDeviceV2():Promise<SecureSmartDeviceConnection>{throw new Error("SMART_DEVICE_BLE_V2_UNAVAILABLE")}
