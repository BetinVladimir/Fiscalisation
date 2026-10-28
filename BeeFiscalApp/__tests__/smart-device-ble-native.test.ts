/**
 * BF-01.2 / BF-01.3 — BLE provisioning tests (mock react-native-ble-plx)
 *
 * RED phase: authenticate() throws EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE
 * GREEN phase: authenticate resolves on AUTH_OK, rejects correctly on AUTH_FAIL/timeout
 */

// Mocks must be declared before imports (jest hoisting)
let mockStartScan: jest.Mock;
let mockStopScan: jest.Mock;
let mockCancel: jest.Mock;
let mockDevice: any;
let mockRead: jest.Mock;
let mockWrite: jest.Mock;
let mockMonitor: jest.Mock;
let mockMTU: jest.Mock;

// Notify callback registry keyed by char UUID
const notifyCallbacks: Record<string, (error: Error | null, char: any) => void> = {};

jest.mock("react-native", () => ({
  Platform: { OS: "ios", Version: 18 },
  PermissionsAndroid: {},
}));

jest.mock("react-native-ble-plx", () => ({
  BleManager: jest.fn().mockImplementation(() => ({
    startDeviceScan: (...args: any[]) => mockStartScan(...args),
    stopDeviceScan: () => mockStopScan(),
    cancelDeviceConnection: (...args: any[]) => mockCancel(...args),
    readCharacteristicForDevice: (...args: any[]) => mockRead(...args),
    writeCharacteristicWithResponseForDevice: (...args: any[]) => mockWrite(...args),
    monitorCharacteristicForDevice: (...args: any[]) => mockMonitor(...args),
    requestMTUForDevice: (...args: any[]) => mockMTU(...args),
  })),
}));

import { connectEdgeDeviceV2 } from "../src/smartDeviceBle.native";
import {
  EDGE_V2_SERVICE,
  EDGE_V2_SESSION_CONTROL,
  EDGE_V2_SESSION_RX,
  EDGE_V2_SESSION_TX,
} from "../src/smartDeviceBle.ts";

const DEVICE_ID = "test-device-abc";
const encode = (v: unknown) => globalThis.btoa(JSON.stringify(v));

const CAPABILITY = {
  version: 2 as const,
  device_id: DEVICE_ID,
  capability_id: "cap-001",
  permissions: ["provision"],
  expires_at: Date.now() + 3600_000,
  signature: "dGVzdHNpZw==",
};

function makeDeviceInfo(deviceId = DEVICE_ID) {
  return encode({
    protocol_version: 2,
    device_id: deviceId,
    serial: "SN-0001",
    firmware_version: "1.2.3",
    nonce: "aGVsbG9ub25jZQ==",
  });
}

beforeEach(() => {
  mockStopScan = jest.fn();
  mockCancel = jest.fn().mockResolvedValue(undefined);
  mockWrite = jest.fn().mockResolvedValue({});
  mockMTU = jest.fn().mockResolvedValue({ mtu: 185 });

  mockDevice = {
    id: DEVICE_ID,
    mtu: 185,
    connect: jest.fn(),
    discoverAllServicesAndCharacteristics: jest.fn().mockResolvedValue(undefined),
  };
  mockDevice.connect.mockResolvedValue(mockDevice);

  mockRead = jest.fn().mockResolvedValue({ value: makeDeviceInfo() });

  // monitorCharacteristicForDevice — record callback, return subscription object
  mockMonitor = jest.fn().mockImplementation(
    (_deviceId: string, _service: string, charUUID: string, callback: any) => {
      notifyCallbacks[charUUID] = callback;
      return { remove: jest.fn() };
    }
  );

  mockStartScan = jest.fn().mockImplementation(
    (_services: any, _options: any, callback: any) => {
      callback(null, mockDevice);
    }
  );

  // Clear notify callback registry
  for (const k of Object.keys(notifyCallbacks)) {
    delete notifyCallbacks[k];
  }
});

function sendNotify(charUUID: string, payload: unknown) {
  const cb = notifyCallbacks[charUUID];
  if (!cb) throw new Error(`No monitor registered for ${charUUID}`);
  cb(null, { value: encode(payload) });
}

// Helper: connect and receive challenge then AUTH_OK
async function connectAndAuthOk(): Promise<ReturnType<typeof connectEdgeDeviceV2>> {
  const connPromise = connectEdgeDeviceV2();

  // After a tick, firmware sends challenge
  await Promise.resolve();
  await Promise.resolve();
  // Simulate firmware sending CHALLENGE after receiving AUTH_V2
  setTimeout(() => {
    sendNotify(EDGE_V2_SESSION_RX, { type: "CHALLENGE", nonce: "Y2hhbGxlbmdlbm9uY2U=" });
    // After another tick, firmware sends AUTH_OK
    setTimeout(() => {
      sendNotify(EDGE_V2_SESSION_RX, { type: "AUTH_OK" });
    }, 0);
  }, 0);

  return connPromise;
}

// ── Test 1: AUTH_OK resolves ────────────────────────────────────────────────

test("authenticate resolves when device responds AUTH_OK", async () => {
  const connPromise = connectEdgeDeviceV2();

  await Promise.resolve();
  await Promise.resolve();

  // Trigger CHALLENGE then AUTH_OK
  setTimeout(() => {
    sendNotify(EDGE_V2_SESSION_RX, { type: "CHALLENGE", nonce: "Y2hhbGxlbmdl" });
    setTimeout(() => {
      sendNotify(EDGE_V2_SESSION_RX, { type: "AUTH_OK" });
    }, 0);
  }, 0);

  const conn = await connPromise;
  await expect(conn.authenticate(CAPABILITY)).resolves.toBeUndefined();
});

// ── Test 2: AUTH_FAIL rejects with EDGE_AUTH_REJECTED ──────────────────────

test("authenticate rejects with EDGE_AUTH_REJECTED when device responds AUTH_FAIL", async () => {
  const connPromise = connectEdgeDeviceV2();

  await Promise.resolve();
  await Promise.resolve();

  setTimeout(() => {
    sendNotify(EDGE_V2_SESSION_RX, { type: "CHALLENGE", nonce: "Y2hhbGxlbmdl" });
    setTimeout(() => {
      sendNotify(EDGE_V2_SESSION_RX, { type: "AUTH_FAIL" });
    }, 0);
  }, 0);

  const conn = await connPromise;
  await expect(conn.authenticate(CAPABILITY)).rejects.toThrow("EDGE_AUTH_REJECTED");
});

// ── Test 3: AUTH_TIMEOUT ───────────────────────────────────────────────────

test("authenticate rejects with EDGE_AUTH_TIMEOUT when no response in 10 s", async () => {
  jest.useFakeTimers();
  try {
    // connectEdgeDeviceV2 resolves via Promise mocks (no real timers needed)
    const conn = await connectEdgeDeviceV2();
    const authPromise = conn.authenticate(CAPABILITY);
    // Flush: let authenticate's writeCharacteristicWithResponseForDevice resolve
    // and let Promise.race register its 10s setTimeout before we advance time
    await Promise.resolve();
    await Promise.resolve();
    // No notify sent — advance past 10 s auth timeout
    jest.advanceTimersByTime(11_000);
    await expect(authPromise).rejects.toThrow("EDGE_AUTH_TIMEOUT");
  } finally {
    jest.useRealTimers();
  }
});

// ── Test 4: EDGE_CONNECT_TIMEOUT calls cancelDeviceConnection ──────────────

test("EDGE_CONNECT_TIMEOUT fires after 15 s and cancelDeviceConnection is called", async () => {
  jest.useFakeTimers();
  try {
    mockDevice.connect = jest.fn().mockReturnValue(new Promise<void>(() => {})); // never resolves
    const connectPromise = connectEdgeDeviceV2();
    // Flush microtasks so the connect step (not just scan) is reached
    await Promise.resolve();
    await Promise.resolve();
    jest.advanceTimersByTime(16_000);
    await expect(connectPromise).rejects.toThrow("EDGE_CONNECT_TIMEOUT");
    expect(mockCancel).toHaveBeenCalledWith(DEVICE_ID);
  } finally {
    jest.useRealTimers();
  }
});

// ── Test 5: setWifi fragments into chunks ≤ MTU-3 ──────────────────────────

test("setWifi writes fragments each at most MTU-3 bytes encoded", async () => {
  const connPromise = connectEdgeDeviceV2();

  await Promise.resolve();
  await Promise.resolve();

  setTimeout(() => {
    sendNotify(EDGE_V2_SESSION_RX, { type: "CHALLENGE", nonce: "Y2hhbGxlbmdl" });
    setTimeout(() => {
      sendNotify(EDGE_V2_SESSION_RX, { type: "AUTH_OK" });
    }, 0);
  }, 0);

  const conn = await connPromise;
  await conn.authenticate(CAPABILITY);

  const longSsid = "a".repeat(60);
  const longPass = "b".repeat(60);

  // After setWifi, simulate WIFI_OK
  setTimeout(() => {
    sendNotify(EDGE_V2_SESSION_RX, { type: "WIFI_OK" });
  }, 0);

  await conn.setWifi(longSsid, longPass);

  // Verify writes to SESSION_TX (setWifi data) are ≤ MTU-3 bytes each (decoded)
  const sessionTxWrites = mockWrite.mock.calls.filter(
    (call: any[]) => call[2] === EDGE_V2_SESSION_TX,
  );
  expect(sessionTxWrites.length).toBeGreaterThan(0);
  const mtu = 185;
  const maxPayloadBytes = mtu - 3; // ATT overhead
  for (const call of sessionTxWrites) {
    const encoded: string = call[3]; // 4th arg is value (base64)
    if (typeof encoded === "string") {
      const decoded = globalThis.atob(encoded);
      expect(decoded.length).toBeLessThanOrEqual(maxPayloadBytes);
    }
  }
});

// ── Test 6: setWifi before authenticate throws BLE_SESSION_UNAUTHORIZED ────

test("setWifi before authenticate throws BLE_SESSION_UNAUTHORIZED", async () => {
  const connPromise = connectEdgeDeviceV2();
  await Promise.resolve();
  await Promise.resolve();

  const conn = await connPromise;
  await expect(conn.setWifi("net", "pass")).rejects.toThrow("BLE_SESSION_UNAUTHORIZED");
});

// ── Test 7: CAPABILITY_DEVICE_MISMATCH ─────────────────────────────────────

test("authenticate throws CAPABILITY_DEVICE_MISMATCH when device_id does not match", async () => {
  const connPromise = connectEdgeDeviceV2();
  await Promise.resolve();
  await Promise.resolve();

  const conn = await connPromise;
  const wrongCap = { ...CAPABILITY, device_id: "wrong-device" };
  await expect(conn.authenticate(wrongCap)).rejects.toThrow("CAPABILITY_DEVICE_MISMATCH");
});
