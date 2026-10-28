import { PermissionsAndroid, Platform } from 'react-native';
import { BleManager } from 'react-native-ble-plx';
import { NativeBleTransport } from './nativeBleTransport.ts';
export async function requestNativeBlePermissions() {
  if (Platform.OS !== "android") return true;
  const api = Number(Platform.Version);
  if (api >= 31) {
    const result = await PermissionsAndroid.requestMultiple([
      PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN,
      PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT,
    ]);
    return (
      result[PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN] ===
        PermissionsAndroid.RESULTS.GRANTED &&
      result[PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT] ===
        PermissionsAndroid.RESULTS.GRANTED
    );
  }
  return (
    (await PermissionsAndroid.request(
      PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION,
    )) === PermissionsAndroid.RESULTS.GRANTED
  );
}

export function createNativeBleBootstrap(){return new NativeBleTransport(new BleManager(),Platform.OS==='android');}
