import { PermissionsAndroid, Platform } from "react-native";
import { BleManager } from "react-native-ble-plx";
import {
  EDGE_V2_DEVICE_INFO,
  EDGE_V2_SERVICE,
  EDGE_V2_SESSION_CONTROL,
} from "./smartDeviceBle.ts";
import type {
  DeploymentCapability,
  DeviceInfo,
  SecureSmartDeviceConnection,
} from "./smartDeviceBle.ts";
const manager = new BleManager();
const decode = (v: string) => JSON.parse(globalThis.atob(v));
const encode = (v: unknown) => globalThis.btoa(JSON.stringify(v));
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
export async function connectEdgeDeviceV2(): Promise<SecureSmartDeviceConnection> {
  await permission();
  const found = await new Promise<any>((resolve, reject) => {
    let done = false;
    const timer = setTimeout(() => {
      if (done) return;
      done = true;
      manager.stopDeviceScan();
      reject(new Error("EDGE_SCAN_TIMEOUT"));
    }, 15_000);
    manager.startDeviceScan([EDGE_V2_SERVICE], null, (error, device) => {
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
  await device.discoverAllServicesAndCharacteristics();
  const raw = await manager.readCharacteristicForDevice(
    device.id,
    EDGE_V2_SERVICE,
    EDGE_V2_DEVICE_INFO,
  );
  if (!raw.value) throw new Error("EDGE_DEVICE_INFO_MISSING");
  const info = decode(raw.value) as DeviceInfo;
  if (info.protocol_version !== 2) throw new Error("EDGE_PROTOCOL_V2_REQUIRED");
  let authorized = false;
  return {
    deviceInfo: info,
    async authenticate(capability: DeploymentCapability) {
      if (capability.version !== 2 || capability.device_id !== info.device_id)
        throw new Error("CAPABILITY_DEVICE_MISMATCH");
      await manager.writeCharacteristicWithResponseForDevice(
        device.id,
        EDGE_V2_SERVICE,
        EDGE_V2_SESSION_CONTROL,
        encode({ type: "AUTH_V2", capability }),
      );
      throw new Error("EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE");
    },
    async setWifi() {
      if (!authorized) throw new Error("BLE_SESSION_UNAUTHORIZED");
    },
    async bindStore() {
      if (!authorized) throw new Error("BLE_SESSION_UNAUTHORIZED");
    },
    async disconnect() {
      authorized = false;
      await manager.cancelDeviceConnection(device.id).catch(() => {});
    },
  };
}
export * from "./smartDeviceBle.ts";
