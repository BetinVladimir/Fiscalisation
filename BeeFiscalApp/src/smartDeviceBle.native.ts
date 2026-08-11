import {PermissionsAndroid,Platform} from "react-native";
import {BleManager} from "react-native-ble-plx";
import {activationTokenFrames,BLUECASH_ACTIVATION_SERVICE,BLUECASH_STATUS_CHARACTERISTIC,BLUECASH_TOKEN_CHARACTERISTIC} from "./smartDeviceBle.ts";
import type {SmartDeviceBleConnection} from "./smartDeviceBle.ts";

const manager=new BleManager();
const b64=(value:string)=>globalThis.btoa(value);
const text=(value:string)=>globalThis.atob(value);

async function permission(){if(Platform.OS!=="android")return;const api=Number(Platform.Version);if(api>=31){const result=await PermissionsAndroid.requestMultiple([PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN,PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT]);if(result[PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN]!==PermissionsAndroid.RESULTS.GRANTED||result[PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT]!==PermissionsAndroid.RESULTS.GRANTED)throw new Error("BLE_PERMISSION_DENIED")}else if(await PermissionsAndroid.request(PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION)!==PermissionsAndroid.RESULTS.GRANTED)throw new Error("BLE_PERMISSION_DENIED")}

export async function connectBlueCashForActivation():Promise<SmartDeviceBleConnection>{await permission();const found=await new Promise<any>((resolve,reject)=>{let done=false;const timer=setTimeout(()=>{if(done)return;done=true;manager.stopDeviceScan();reject(new Error("BLUECASH_SCAN_TIMEOUT"))},15_000);manager.startDeviceScan([BLUECASH_ACTIVATION_SERVICE],null,(error,device)=>{if(done)return;if(error){done=true;clearTimeout(timer);manager.stopDeviceScan();reject(error)}else if(device){done=true;clearTimeout(timer);manager.stopDeviceScan();resolve(device)}})});const connected=await found.connect();await connected.discoverAllServicesAndCharacteristics();return{async writeActivationToken(jwt){const transferId=`${Date.now()}-${Math.random().toString(36).slice(2,10)}`;for(const frame of activationTokenFrames(jwt,transferId))await manager.writeCharacteristicWithResponseForDevice(connected.id,BLUECASH_ACTIVATION_SERVICE,BLUECASH_TOKEN_CHARACTERISTIC,b64(frame));const status=await manager.readCharacteristicForDevice(connected.id,BLUECASH_ACTIVATION_SERVICE,BLUECASH_STATUS_CHARACTERISTIC);const value=status.value?text(status.value):"UNKNOWN";if(value!=="ACTIVATED")throw new Error(`BLUECASH_ACTIVATION_${value}`);return value},async disconnect(){await manager.cancelDeviceConnection(connected.id).catch(()=>{})}}}
export {BLUECASH_ACTIVATION_SERVICE,BLUECASH_STATUS_CHARACTERISTIC,BLUECASH_TOKEN_CHARACTERISTIC};
export type {SmartDeviceBleConnection};
