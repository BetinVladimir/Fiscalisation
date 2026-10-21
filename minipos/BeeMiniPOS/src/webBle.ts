import { encodeCanonical } from "./cbor.ts";
import { BleResultWaiters } from "./bleResult.ts";
import type { BleFiscalResult } from "./bleResult.ts";
import { base64urlEncode, randomBytes } from "./portableCrypto.ts";
import {
  authorizedAdvertisingIdentity,
  matchesAdvertisingIdentity,
} from "./bleAdvertising.ts";
import { FrameAssembler, frameMessage } from "./bleFrames.ts";
export type BleSessionPackage = {
  ble_session_id: string;
  tenant_id: string;
  location_id: string;
  register_id: string;
  edge_id: string;
  device_id: string;
  service_uuid: string;
  command_characteristic_uuid: string;
  event_characteristic_uuid: string;
  advertising_identity: string;
  signed_session_ticket: string;
  expires_at: string;
  protocol_version: string;
  security_mode: "OPEN_MVP" | "X25519_AES_GCM";
};
import { BrowserGattOwner } from "./browserGatt.ts";
import { uuidString } from "./bleFrames.ts";
type Connection = {
  owner: BrowserGattOwner;
  command?: any;
  ready: boolean;
  session: BleSessionPackage;
  expected?: string;
  frames: FrameAssembler;
  results: BleResultWaiters;
  busy: boolean;
  clearExpected?: () => void;
};
export function webBluetoothSupported() {
  return (
    typeof navigator !== "undefined" &&
    !!(navigator as any).bluetooth &&
    globalThis.isSecureContext === true
  );
}
export class WebBleBootstrap {
  private connection?: Connection;
  get canSendComplianceIntent() {
    return (
      !!this.connection?.ready &&
      !this.connection.owner.closed &&
      Date.parse(this.connection.session.expires_at) > Date.now()
    );
  }
  prepareSessionPublicKey() {
    return base64urlEncode(randomBytes(32));
  }
  resetSession() {
    const connection = this.connection;
    this.connection = undefined;
    if (connection) {
      connection.ready = false;
      connection.expected = undefined;
      connection.frames = new FrameAssembler();
      connection.results.cancelAll();
      connection.owner.close();
    }
  }
  private live(connection: Connection) {
    connection.owner.guard();
    if (this.connection !== connection) throw new Error("BLE_DISCONNECTED");
    const expiry = Date.parse(connection.session.expires_at);
    if (!Number.isFinite(expiry) || expiry <= Date.now())
      throw new Error("BLE_ROUTE_PACKAGE_EXPIRED");
  }
  async connectFromUserGesture(
    session: BleSessionPackage,
    onEvent: (raw: Uint8Array) => void,
    signal?: AbortSignal,
  ) {
    if (!webBluetoothSupported()) throw new Error("WEB_BLUETOOTH_UNAVAILABLE");
    if (this.connection && !this.connection.owner.closed)
      throw new Error("BLE_CONNECTION_IN_PROGRESS");
    if (session.security_mode !== "OPEN_MVP")
      throw new Error("BLE_SECURITY_MODE_UNSUPPORTED");
    const identity = authorizedAdvertisingIdentity(
      session.advertising_identity,
      session.edge_id,
    );
    const owner = new BrowserGattOwner();
    const connection: Connection = {
      owner,
      session: { ...session },
      ready: false,
      results: new BleResultWaiters(),
      frames: new FrameAssembler(),
      busy: false,
    };
    this.connection = connection;
    owner.onClose(() => {
      connection.ready = false;
      connection.expected = undefined;
      connection.frames = new FrameAssembler();
      connection.results.cancelAll();
    });
    owner.bind(signal);
    try {
      this.live(connection);
      const device: any = await owner.wait(
        (navigator as any).bluetooth.requestDevice({
          filters: [{ services: [session.service_uuid] }],
        }),
      );
      this.live(connection);
      if (!matchesAdvertisingIdentity(device, identity))
        throw new Error("BLE_ADVERTISING_IDENTITY_MISMATCH");
      owner.claim(device);
      const setupDeadline = owner.timer(30_000, "BLE_SETUP_TIMEOUT");
      const server: any = await owner.step(() => device.gatt.connect());
      this.live(connection);
      const service: any = await owner.step(() =>
        server.getPrimaryService(session.service_uuid),
      );
      this.live(connection);
      connection.command = await owner.step(() =>
        service.getCharacteristic(session.command_characteristic_uuid),
      );
      this.live(connection);
      const event: any = await owner.step(() =>
        service.getCharacteristic(session.event_characteristic_uuid),
      );
      this.live(connection);
      owner.listen(event, "characteristicvaluechanged", (e) => {
        if (
          owner.closed ||
          this.connection !== connection ||
          !connection.expected
        )
          return;
        try {
          this.live(connection);
        } catch (error) {
          owner.close(error);
          return;
        }
        const value = e.target?.value as DataView | undefined;
        if (!value || value.byteLength < 58 || value.byteLength > 512) return;
        const frame = new Uint8Array(
          value.buffer,
          value.byteOffset,
          value.byteLength,
        ).slice();
        const expected = connection.expected,
          assembler = connection.frames;
        if (uuidString(frame.slice(4, 20)) !== expected) return;
        void assembler
          .accept(frame)
          .then((done) => {
            if (
              !done ||
              owner.closed ||
              this.connection !== connection ||
              connection.frames !== assembler ||
              connection.expected !== expected
            )
              return;
            connection.expected = undefined;
            connection.clearExpected?.();
            connection.clearExpected = undefined;
            connection.frames = new FrameAssembler();
            connection.results.accept(done.id, done.payload);
            onEvent(done.payload);
          })
          .catch(() => {});
      });
      await owner.step(() => event.startNotifications());
      this.live(connection);
      setupDeadline();
      connection.ready = true;
      return { state: "OPEN_READY" as const, ready: Promise.resolve() };
    } catch (error) {
      owner.close(error);
      connection.results.cancelAll();
      throw error;
    }
  }
  private async send(
    intent: unknown,
    messageId: string,
    mtu: number,
    connection: Connection,
  ) {
    this.live(connection);
    // This is a legacy offer, not a browser MTU negotiation. Rejected writes are
    // terminal: never downsize/rechunk/resubmit a possibly accepted fiscal intent.
    if (!Number.isInteger(mtu) || mtu < 61 || mtu > 515)
      throw new Error("BLE_MTU_TOO_SMALL");
    const frames = await frameMessage(encodeCanonical(intent), messageId, mtu);
    this.live(connection);
    connection.clearExpected?.();
    connection.expected = messageId;
    connection.frames = new FrameAssembler();
    for (const frame of frames) {
      this.live(connection);
      if (!connection.command?.writeValueWithResponse)
        throw new Error("BLE_WRITE_UNSUPPORTED");
      await connection.owner.step(
        () => connection.command.writeValueWithResponse(frame),
        5_000,
      );
      this.live(connection);
    }
    return { messageId, chunks: frames.length };
  }
  private begin(signal?: AbortSignal) {
    const connection = this.connection;
    if (!connection || !this.canSendComplianceIntent)
      throw new Error("BLE_NOT_READY");
    this.live(connection);
    if (connection.busy) throw new Error("BLE_OPERATION_IN_PROGRESS");
    connection.busy = true;
    const remove = connection.owner.bind(signal);
    return { connection, remove };
  }
  async sendComplianceIntent(
    intent: unknown,
    messageId: string,
    mtu = 185,
    signal?: AbortSignal,
  ) {
    const { connection, remove } = this.begin(signal);
    const deadline = connection.owner.timer(30_000, "BLE_SEND_TIMEOUT");
    try {
      const result = await this.send(intent, messageId, mtu, connection);
      // Bound unsolicited result retention after the no-wait API returns.
      if (connection.expected === messageId)
        connection.clearExpected = connection.owner.timer(
          30_000,
          "BLE_RESULT_TIMEOUT",
        );
      return result;
    } catch (error) {
      connection.owner.close(error);
      throw error;
    } finally {
      deadline();
      remove();
      connection.busy = false;
    }
  }
  async sendComplianceIntentAndWait(
    intent: unknown,
    messageId: string,
    mtu = 185,
    timeoutMs = 30_000,
    signal?: AbortSignal,
  ) {
    if (!Number.isInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 120_000)
      throw new Error("BLE_RESULT_WAIT_INVALID");
    const { connection, remove } = this.begin(signal);
    try {
      const result = connection.results.wait(messageId, timeoutMs);
      // Observe both immediately: an early result must not complete a hung write.
      const combined = Promise.all([
        this.send(intent, messageId, mtu, connection),
        result,
      ]);
      const [, value] = await connection.owner.wait(
        combined,
        timeoutMs,
        "BLE_RESULT_TIMEOUT",
      );
      this.live(connection);
      return value;
    } catch (error) {
      connection.owner.close(error);
      throw error;
    } finally {
      connection.results.cancelAll();
      connection.expected = undefined;
      connection.frames = new FrameAssembler();
      connection.busy = false;
      remove();
    }
  }
}
