import type {BleSessionPackage} from "./webBle.ts";
import type {BleFiscalResult} from "./bleResult.ts";
export interface NativeBleBootstrapContract{readonly canExecuteFiscalCommand:boolean;connectFromUserGesture(session:BleSessionPackage,onEvent:(raw:Uint8Array)=>void):Promise<{state:"HELLO_SENT";ready:Promise<void>}>;sendFiscalCommandAndWait(envelope:unknown,messageId:string,attMtu?:number,timeoutMs?:number):Promise<BleFiscalResult>}
export function createNativeBleBootstrap():NativeBleBootstrapContract{throw new Error("NATIVE_BLE_UNAVAILABLE")}
export async function requestNativeBlePermissions(){return false}
