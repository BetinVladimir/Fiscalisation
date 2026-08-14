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
export async function connectEdgeDeviceV2(): Promise<SecureSmartDeviceConnection> {
  const bluetooth = (navigator as any).bluetooth;
  if (!bluetooth)
    throw new Error("WEB_BLUETOOTH_REQUIRES_CHROME_SECURE_CONTEXT");
  if (!globalThis.isSecureContext)
    throw new Error("WEB_BLUETOOTH_REQUIRES_SECURE_CONTEXT");
  const device = await bluetooth.requestDevice({
    filters: [{ services: [EDGE_V2_SERVICE] }],
  });
  const server = await device.gatt.connect();
  const service = await server.getPrimaryService(EDGE_V2_SERVICE);
  const infoCharacteristic =
    await service.getCharacteristic(EDGE_V2_DEVICE_INFO);
  const control = await service.getCharacteristic(EDGE_V2_SESSION_CONTROL);
  const raw = await infoCharacteristic.readValue();
  const info = JSON.parse(new TextDecoder().decode(raw.buffer)) as DeviceInfo;
  if (info.protocol_version !== 2) throw new Error("EDGE_PROTOCOL_V2_REQUIRED");
  return {
    deviceInfo: info,
    async authenticate(capability: DeploymentCapability) {
      if (capability.device_id !== info.device_id)
        throw new Error("CAPABILITY_DEVICE_MISMATCH");
      await control.writeValueWithResponse(
        new TextEncoder().encode(
          JSON.stringify({ type: "AUTH_V2", capability }),
        ),
      );
      throw new Error("EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE");
    },
    async setWifi() {
      throw new Error("BLE_SESSION_UNAUTHORIZED");
    },
    async bindStore() {
      throw new Error("BLE_SESSION_UNAUTHORIZED");
    },
    async disconnect() {
      device.gatt?.disconnect();
    },
  };
}
export * from "./smartDeviceBle.ts";
