import {PermissionsAndroid,Platform} from 'react-native';
import {BleManager} from 'react-native-ble-plx';
import {PROVISIONING_SERVICE,PROVISIONING_CONTROL,PROVISIONING_INFO,PROVISIONING_STATUS} from './provisioningProtocol';
import type {ProvisioningLink} from './provisioningProtocol';
const manager=new BleManager();
async function permission() {
  if (Platform.OS !== "android") return;
  const api = Number(Platform.Version);
  if (api >= 31) {
    const r = await PermissionsAndroid.requestMultiple([
      PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN,
      PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT,
    ]);
    if (Object.values(r).some((x) => x !== PermissionsAndroid.RESULTS.GRANTED))
      throw new Error("BLE_PERMISSION_DENIED");
  } else if (
    (await PermissionsAndroid.request(
      PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION,
    )) !== PermissionsAndroid.RESULTS.GRANTED
  )
    throw new Error("BLE_PERMISSION_DENIED");
}
export async function connectProvisioningDevice():Promise<ProvisioningLink>{
  await permission();
  const found = await new Promise<any>((resolve, reject) => {
    let done = false;
    const timer = setTimeout(() => {
      if (done) return;
      done = true;
      manager.stopDeviceScan();
      reject(new Error("EDGE_SCAN_TIMEOUT"));
    }, 15_000);
    manager.startDeviceScan([PROVISIONING_SERVICE], null, (error, device) => {
      if (done) return;
      if (error) {
        done = true;
        clearTimeout(timer);
        manager.stopDeviceScan();
        reject(error);
      } else if (device) {
        done = true;
        clearTimeout(timer);
        manager.stopDeviceScan();
        resolve(device);
      }
    });
  });
  const device = await found.connect();
  try{
  await device.discoverAllServicesAndCharacteristics();
  // The default ATT MTU accepts at least 182 bytes after requesting MTU on Android.
  if(Platform.OS==='android')await device.requestMTU(185);
  if(device.mtu<185)throw Error('PROVISIONING_ATT_MTU_TOO_SMALL');
  const read=async(uuid:string)=>{const c=await manager.readCharacteristicForDevice(device.id,PROVISIONING_SERVICE,uuid);if(!c.value)throw Error('EMPTY_PROVISIONING_READ');return JSON.parse(globalThis.atob(c.value))};
  return {readInfo:()=>read(PROVISIONING_INFO),readStatus:()=>read(PROVISIONING_STATUS),write:async(bytes)=>{let binary='';for(const byte of bytes)binary+=String.fromCharCode(byte);await manager.writeCharacteristicWithResponseForDevice(device.id,PROVISIONING_SERVICE,PROVISIONING_CONTROL,globalThis.btoa(binary))},disconnect:async()=>{await manager.cancelDeviceConnection(device.id).catch(()=>{})}};
  }catch(error){await manager.cancelDeviceConnection(device.id).catch(()=>{});throw error}
}
