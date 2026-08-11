export const BLUECASH_ACTIVATION_SERVICE = "7b6f1000-7c6d-4c7a-9e4f-424545464953";
export const BLUECASH_TOKEN_CHARACTERISTIC = "7b6f1001-7c6d-4c7a-9e4f-424545464953";
export const BLUECASH_STATUS_CHARACTERISTIC = "7b6f1002-7c6d-4c7a-9e4f-424545464953";
export function activationTokenFrames(jwt:string,transferId:string,fragmentSize=120):string[]{if(!jwt||!/[A-Za-z0-9-]{8,64}/.test(transferId)||fragmentSize<32||fragmentSize>256)throw new Error("ACTIVATION_FRAME_INPUT_INVALID");const parts=Array.from({length:Math.ceil(jwt.length/fragmentSize)},(_,index)=>jwt.slice(index*fragmentSize,(index+1)*fragmentSize));if(parts.length>32)throw new Error("ACTIVATION_TOKEN_TOO_LARGE");return parts.map((part,index)=>`BFA1|${transferId}|${index+1}|${parts.length}|${part}`)}
export interface SmartDeviceBleConnection { writeActivationToken(jwt:string):Promise<string>; disconnect():Promise<void> }
export async function connectBlueCashForActivation():Promise<SmartDeviceBleConnection>{throw new Error("SMART_DEVICE_BLE_UNAVAILABLE")}
