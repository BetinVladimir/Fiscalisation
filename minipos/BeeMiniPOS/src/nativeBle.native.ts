import { PermissionsAndroid, Platform } from "react-native";
import { BleManager } from "react-native-ble-plx";
import type { BleError, Characteristic } from "react-native-ble-plx";
import { decodeStrict, encodeCanonical } from "./cbor.ts";
import type { BleFiscalResult } from "./bleResult.ts";
import { BleResultWaiters } from "./bleResult.ts";
import type { BleSessionPackage } from "./webBle.ts";
import type { NativeBleBootstrapContract } from "./nativeBle.ts";
import {
  base64urlDecode,
  base64urlEncode,
  randomBytes,
} from "./portableCrypto.ts";
import {
  authorizedAdvertisingIdentity,
  matchesAdvertisingIdentity,
} from "./bleAdvertising.ts";
import { FrameAssembler, frameMessage } from "./bleFrames.ts";

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

class NativeBleBootstrap implements NativeBleBootstrapContract {
  private manager = new BleManager();
  private deviceId?: string;
  private session?: BleSessionPackage;
  private ready = false;
  private results = new BleResultWaiters();
  private frames = new FrameAssembler();
  get canSendComplianceIntent() {
    return this.ready;
  }
  prepareSessionPublicKey() {
    return base64urlEncode(randomBytes(32));
  }
  resetSession() {
    this.results.cancelAll();
    if (this.deviceId)
      void this.manager.cancelDeviceConnection(this.deviceId).catch(() => {});
    this.deviceId = undefined;
    this.session = undefined;
    this.ready = false;
    this.results = new BleResultWaiters();
  }
  async connectFromUserGesture(
    session: BleSessionPackage,
    onEvent: (raw: Uint8Array) => void,
  ) {
    if (session.security_mode !== "OPEN_MVP")
      throw new Error("BLE_SECURITY_MODE_UNSUPPORTED");
    if (new Date(session.expires_at).getTime() <= Date.now())
      throw new Error("BLE_ROUTE_PACKAGE_EXPIRED");
    const advertisingIdentity = authorizedAdvertisingIdentity(
      session.advertising_identity,
      session.edge_id,
    );
    const device = await new Promise<any>((resolve, reject) => {
      let done = false;
      const timer = setTimeout(() => {
        if (done) return;
        done = true;
        this.manager.stopDeviceScan();
        reject(new Error("BLE_SCAN_TIMEOUT"));
      }, 10_000);
      this.manager.startDeviceScan(
        [session.service_uuid],
        null,
        (error, found) => {
          if (done) return;
          if (error) {
            done = true;
            clearTimeout(timer);
            this.manager.stopDeviceScan();
            reject(error);
            return;
          }
          if (found && matchesAdvertisingIdentity(found, advertisingIdentity)) {
            done = true;
            clearTimeout(timer);
            this.manager.stopDeviceScan();
            resolve(found);
          }
        },
      );
    });
    const connected = await device.connect();
    await connected.discoverAllServicesAndCharacteristics();
    if (Platform.OS === "android") await connected.requestMTU(185);
    this.deviceId = connected.id;
    this.session = session;
    connected.monitorCharacteristicForService(
      session.service_uuid,
      session.event_characteristic_uuid,
      (error: BleError | null, characteristic: Characteristic | null) => {
        if (error) {
          this.ready = false;
          this.results.cancelAll();
          return;
        }
        if (!characteristic?.value) return;
        void this.frames
          .accept(fromStandardBase64(characteristic.value))
          .then((done) => {
            if (!done) return;
            this.results.accept(done.id, done.payload);
            onEvent(done.payload);
          })
          .catch(() => {});
      },
    );
    this.ready = true;
    return { state: "OPEN_READY" as const, ready: Promise.resolve() };
  }
  async sendComplianceIntentAndWait(
    intent: unknown,
    messageId: string,
    _attMtu = 185,
    timeoutMs = 30_000,
  ) {
    if (!this.ready || !this.session) throw new Error("BLE_NOT_READY");
    const result = this.results.wait(messageId, timeoutMs);
    try {
      for (const frame of await frameMessage(
        encodeCanonical(intent),
        messageId,
        _attMtu,
      ))
        await this.write(this.session.command_characteristic_uuid, frame);
    } catch (e) {
      this.results.reject(messageId, e);
    }
    return result;
  }
  private async write(characteristic: string, raw: Uint8Array) {
    if (!this.deviceId || !this.session) throw new Error("BLE_NOT_CONNECTED");
    await this.manager.writeCharacteristicWithResponseForDevice(
      this.deviceId,
      this.session.service_uuid,
      characteristic,
      toStandardBase64(raw),
    );
  }
}
export function createNativeBleBootstrap() {
  return new NativeBleBootstrap();
}
function toStandardBase64(v: Uint8Array) {
  const raw = base64urlEncode(v).replaceAll("-", "+").replaceAll("_", "/");
  return raw + "=".repeat((4 - (raw.length % 4)) % 4);
}
function fromStandardBase64(v: string) {
  return base64urlDecode(
    v.replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", ""),
  );
}
function operationId(raw: Uint8Array) {
  try {
    const value = decodeStrict<Partial<BleFiscalResult>>(raw);
    return typeof value.operation_id === "string" ? value.operation_id : "";
  } catch {
    return "";
  }
}
