import { PermissionsAndroid, Platform } from "react-native";
import { BleManager } from "react-native-ble-plx";
import {
  EDGE_V2_DEVICE_INFO,
  EDGE_V2_SERVICE,
  EDGE_V2_SESSION_CONTROL,
  EDGE_V2_SESSION_RX,
  EDGE_V2_SESSION_TX,
} from "./smartDeviceBle.ts";
import type {
  DeploymentCapability,
  DeviceInfo,
  SecureSmartDeviceConnection,
} from "./smartDeviceBle.ts";

// BleManager is initialized lazily to avoid Bluetooth state on non-BLE platforms
let _manager: BleManager | null = null;
function getManager(): BleManager {
  if (!_manager) _manager = new BleManager();
  return _manager;
}

const decode = (v: string) => JSON.parse(globalThis.atob(v));
const encode = (v: unknown) => globalThis.btoa(JSON.stringify(v));

async function permission(): Promise<void> {
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

/**
 * Set up a single-use notify listener on SESSION_RX.
 * Returns a function that waits for the next notification.
 */
function makeNotifyQueue(manager: BleManager, deviceId: string): {
  waitForNotify: () => Promise<Record<string, unknown>>;
  sub: { remove: () => void };
} {
  let notifyResolve: ((v: Record<string, unknown>) => void) | null = null;
  let notifyReject: ((e: Error) => void) | null = null;

  const sub = manager.monitorCharacteristicForDevice(
    deviceId,
    EDGE_V2_SERVICE,
    EDGE_V2_SESSION_RX,
    (error: Error | null, char: { value: string | null } | null) => {
      if (error) {
        notifyReject?.(error);
      } else if (char?.value) {
        notifyResolve?.(decode(char.value) as Record<string, unknown>);
      }
    },
  );

  const waitForNotify = (): Promise<Record<string, unknown>> =>
    new Promise<Record<string, unknown>>((res, rej) => {
      notifyResolve = res;
      notifyReject = rej;
    });

  return { waitForNotify, sub };
}

/** Fragment a JSON payload into base64-encoded chunks, each decoded ≤ maxBytes. */
function fragmentPayload(payload: string, maxBytes: number): string[] {
  const chunks: string[] = [];
  for (let i = 0; i < payload.length; i += maxBytes) {
    chunks.push(payload.slice(i, i + maxBytes));
  }
  return chunks.map((c) => globalThis.btoa(c));
}

export async function connectEdgeDeviceV2(): Promise<SecureSmartDeviceConnection> {
  await permission();
  const manager = getManager();

  // Scan for device (15s timeout)
  const found = await new Promise<any>((resolve, reject) => {
    let done = false;
    const timer = setTimeout(() => {
      if (done) return;
      done = true;
      manager.stopDeviceScan();
      reject(new Error("EDGE_SCAN_TIMEOUT"));
    }, 15_000);
    manager.startDeviceScan([EDGE_V2_SERVICE], null, (error: Error | null, device: any) => {
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

  // Connect with 15s timeout
  let connectTimer: ReturnType<typeof setTimeout> | null = null;
  let connectDone = false;
  const device: any = await new Promise((resolve, reject) => {
    connectTimer = setTimeout(() => {
      if (connectDone) return;
      connectDone = true;
      manager.cancelDeviceConnection(found.id).catch(() => {});
      reject(new Error("EDGE_CONNECT_TIMEOUT"));
    }, 15_000);

    (found.connect() as Promise<any>).then(
      (d: any) => {
        if (connectDone) return;
        connectDone = true;
        clearTimeout(connectTimer!);
        resolve(d);
      },
      (err: Error) => {
        if (connectDone) return;
        connectDone = true;
        clearTimeout(connectTimer!);
        reject(err);
      },
    );
  });

  await device.discoverAllServicesAndCharacteristics();

  const raw = await manager.readCharacteristicForDevice(
    device.id,
    EDGE_V2_SERVICE,
    EDGE_V2_DEVICE_INFO,
  );
  if (!raw.value) throw new Error("EDGE_DEVICE_INFO_MISSING");
  const info = decode(raw.value) as DeviceInfo;
  if (info.protocol_version !== 2) throw new Error("EDGE_PROTOCOL_V2_REQUIRED");

  // ATT payload capacity: MTU - 3 bytes overhead
  const mtu: number = typeof device.mtu === "number" && device.mtu > 3 ? device.mtu : 23;
  const maxPayload = mtu - 3;

  let authorized = false;

  return {
    deviceInfo: info,

    async authenticate(capability: DeploymentCapability): Promise<void> {
      if (capability.version !== 2 || capability.device_id !== info.device_id)
        throw new Error("CAPABILITY_DEVICE_MISMATCH");

      const { waitForNotify, sub } = makeNotifyQueue(manager, device.id);

      try {
        // Send AUTH_V2 to SESSION_CONTROL
        await manager.writeCharacteristicWithResponseForDevice(
          device.id,
          EDGE_V2_SERVICE,
          EDGE_V2_SESSION_CONTROL,
          encode({ type: "AUTH_V2", capability }),
        );

        // Wait for CHALLENGE (10s timeout)
        const challenge = await Promise.race([
          waitForNotify(),
          new Promise<never>((_, reject) =>
            setTimeout(() => reject(new Error("EDGE_AUTH_TIMEOUT")), 10_000),
          ),
        ]);

        // BLOCKED_SPEC: The exact challenge signing mechanism is not described in
        // the authoritative firmware protocol documents reviewed for this task.
        // Forwarding the nonce as-is as a placeholder; real signing must be
        // implemented once the firmware signing spec is available.
        await manager.writeCharacteristicWithResponseForDevice(
          device.id,
          EDGE_V2_SERVICE,
          EDGE_V2_SESSION_CONTROL,
          encode({ type: "AUTH_RESPONSE", signed_nonce: challenge["nonce"] }),
        );

        // Wait for AUTH_OK or AUTH_FAIL (10s timeout)
        const result = await Promise.race([
          waitForNotify(),
          new Promise<never>((_, reject) =>
            setTimeout(() => reject(new Error("EDGE_AUTH_TIMEOUT")), 10_000),
          ),
        ]);

        if (result["type"] === "AUTH_FAIL") throw new Error("EDGE_AUTH_REJECTED");
        if (result["type"] !== "AUTH_OK") throw new Error("EDGE_AUTH_UNEXPECTED");

        authorized = true;
      } finally {
        sub.remove();
      }
    },

    async setWifi(ssid: string, password: string): Promise<void> {
      if (!authorized) throw new Error("BLE_SESSION_UNAUTHORIZED");

      const { waitForNotify, sub } = makeNotifyQueue(manager, device.id);

      try {
        const payload = JSON.stringify({ type: "SET_WIFI", ssid, password });
        const fragments = fragmentPayload(payload, maxPayload);

        for (const fragment of fragments) {
          await manager.writeCharacteristicWithResponseForDevice(
            device.id,
            EDGE_V2_SERVICE,
            EDGE_V2_SESSION_TX,
            fragment,
          );
        }

        // Wait for WIFI_OK or WIFI_FAIL
        const result = await waitForNotify();
        if (result["type"] === "WIFI_FAIL") throw new Error("EDGE_WIFI_FAILED");
        if (result["type"] !== "WIFI_OK") throw new Error("EDGE_WIFI_UNEXPECTED");
      } finally {
        sub.remove();
      }
    },

    async bindStore(
      locationId: string,
      registerId: string,
      roles: string[],
    ): Promise<void> {
      if (!authorized) throw new Error("BLE_SESSION_UNAUTHORIZED");

      const { waitForNotify, sub } = makeNotifyQueue(manager, device.id);

      try {
        const payload = JSON.stringify({
          type: "BIND_STORE",
          location_id: locationId,
          register_id: registerId,
          roles,
        });
        const fragments = fragmentPayload(payload, maxPayload);

        for (const fragment of fragments) {
          await manager.writeCharacteristicWithResponseForDevice(
            device.id,
            EDGE_V2_SERVICE,
            EDGE_V2_SESSION_TX,
            fragment,
          );
        }

        // Wait for BIND_OK or BIND_FAIL
        const result = await waitForNotify();
        if (result["type"] === "BIND_FAIL") throw new Error("EDGE_BIND_FAILED");
        if (result["type"] !== "BIND_OK") throw new Error("EDGE_BIND_UNEXPECTED");
      } finally {
        sub.remove();
      }
    },

    async disconnect(): Promise<void> {
      authorized = false;
      await manager.cancelDeviceConnection(device.id).catch(() => {});
    },
  };
}

export * from "./smartDeviceBle.ts";
