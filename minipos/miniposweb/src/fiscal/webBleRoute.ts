import { BrowserGattOwner } from "./browserGatt";
import { Encoder, decode } from "cbor-x";
import type { FiscalIntent, FiscalOperation } from "../types";
import type { FiscalRoute } from "./routeController";
const encoder = new Encoder({
  useRecords: false,
  mapsAsObjects: true,
  structuredClone: false,
  variableMapSize: true,
});
const MAGIC = [0x42, 0x46, 0x46, 0x31],
  HEADER = 57,
  MAX = 8192;
const uuidBytes = (id: string) => {
  if (
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)
  )
    throw new Error("BLE_MESSAGE_ID_INVALID");
  return Uint8Array.from(
    id
      .replaceAll("-", "")
      .match(/../g)!
      .map((value) => parseInt(value, 16)),
  );
};
const uuidString = (value: Uint8Array) => {
  const hex = [...value].map((x) => x.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
};
async function frames(payload: Uint8Array, id: string, mtu = 185) {
  if (!payload.length || payload.length > MAX)
    throw new Error("BLE_INTENT_TOO_LARGE");
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", payload)),
    capacity = mtu - 3 - HEADER,
    out: Uint8Array[] = [];
  for (let offset = 0; offset < payload.length; offset += capacity) {
    const part = payload.slice(offset, offset + capacity),
      frame = new Uint8Array(HEADER + part.length),
      view = new DataView(frame.buffer);
    frame.set(MAGIC);
    frame.set(uuidBytes(id), 4);
    view.setUint16(20, payload.length);
    view.setUint16(22, offset);
    frame[24] = offset + part.length === payload.length ? 1 : 0;
    frame.set(digest, 25);
    frame.set(part, HEADER);
    out.push(frame);
  }
  return out;
}
class Assembler {
  private verifying = false;
  private state?: {
    total: number;
    digest: Uint8Array;
    offset: number;
    payload: Uint8Array;
  };
  clear() {
    this.state = undefined;
  }
  async accept(frame: Uint8Array) {
    if (this.verifying) throw new Error("BLE_FRAME_VERIFICATION_PENDING");
    if (
      frame.length <= HEADER ||
      frame.length > 512 ||
      !MAGIC.every((v, i) => frame[i] === v)
    )
      throw new Error("BLE_FRAME_INVALID");
    const id = uuidString(frame.slice(4, 20)),
      view = new DataView(frame.buffer, frame.byteOffset, frame.byteLength),
      total = view.getUint16(20),
      offset = view.getUint16(22),
      final = frame[24],
      digest = frame.slice(25, 57),
      part = frame.subarray(57);
    if (
      !total ||
      total > MAX ||
      final > 1 ||
      offset + part.length > total ||
      (final === 1) !== (offset + part.length === total)
    )
      throw new Error("BLE_FRAME_BOUNDS");
    if (!this.state) {
      if (offset !== 0) throw new Error("BLE_FRAME_ORDER");
      this.state = { total, digest, offset: 0, payload: new Uint8Array(total) };
    }
    const state = this.state;
    if (
      total !== state.total ||
      offset !== state.offset ||
      !digest.every((v, i) => v === state.digest[i])
    )
      throw new Error("BLE_FRAME_CONFLICT");
    state.payload.set(part, offset);
    state.offset += part.length;
    if (!final) return;
    this.state = undefined;
    this.verifying = true;
    const actual = new Uint8Array(
      await crypto.subtle.digest("SHA-256", state.payload),
    );
    if (!actual.every((v, i) => v === digest[i]))
      throw new Error("BLE_FRAME_DIGEST");
    return { id, payload: state.payload };
  }
}
export type WebBleConfiguration = {
  advertisingIdentity: string;
  serviceUUID: string;
  commandUUID: string;
  eventUUID: string;
};
export class WebBleRoute implements FiscalRoute {
  // Browser BLE transports the same signed/canonical intent as Local HTTP.
  // Fragment ACKs acknowledge bytes, not fiscal completion; only the correlated
  // terminal result can complete the outbox operation.
  kind = "BLE" as const;
  constructor(private readonly configuration: WebBleConfiguration) {}
  async probe() {
    return (
      typeof (navigator as unknown as { bluetooth?: unknown }).bluetooth !==
      "undefined"
    );
  }
  async lookup(): Promise<FiscalOperation> {
    throw new Error("BLE_LOOKUP_REQUIRES_RECONNECT");
  }
  async submit(
    intent: FiscalIntent,
    signal: AbortSignal,
  ): Promise<FiscalOperation> {
    return this.submitCommand(intent, signal);
  }
  private owner?: BrowserGattOwner;
  async submitCommand(
    intent: { client_operation_id: string },
    signal: AbortSignal,
  ): Promise<FiscalOperation> {
    if (this.owner && !this.owner.closed)
      throw new Error("BLE_OPERATION_IN_PROGRESS");
    const bluetooth = (navigator as any).bluetooth;
    if (!bluetooth) throw new Error("WEB_BLUETOOTH_UNAVAILABLE");
    const owner = new BrowserGattOwner();
    this.owner = owner;
    owner.bind(signal);
    const live = () => {
      owner.guard();
      if (this.owner !== owner) throw new Error("BLE_DISCONNECTED");
    };
    try {
      live();
      uuidBytes(intent.client_operation_id);
      const device: any = await owner.wait(
        bluetooth.requestDevice({
          filters: [{ name: this.configuration.advertisingIdentity }],
          optionalServices: [this.configuration.serviceUUID],
        }),
      );
      live();
      if (device.name !== this.configuration.advertisingIdentity)
        throw new Error("BLE_ADVERTISING_IDENTITY_MISMATCH");
      owner.claim(device);
      const deadline = owner.timer(120_000, "BLE_RESULT_TIMEOUT");
      const setupDeadline = owner.timer(30_000, "BLE_SETUP_TIMEOUT");
      const server: any = await owner.step(() => device.gatt.connect());
      live();
      const service: any = await owner.step(() =>
        server.getPrimaryService(this.configuration.serviceUUID),
      );
      live();
      const command: any = await owner.step(() =>
        service.getCharacteristic(this.configuration.commandUUID),
      );
      live();
      const event: any = await owner.step(() =>
        service.getCharacteristic(this.configuration.eventUUID),
      );
      live();
      const assembler = new Assembler();
      owner.onClose(() => assembler.clear());
      let resolve!: (result: FiscalOperation) => void;
      const result = new Promise<FiscalOperation>((done) => {
        resolve = done;
      });
      owner.listen(event, "characteristicvaluechanged", (e) => {
        if (owner.closed || this.owner !== owner) return;
        const value = e.target?.value as DataView | undefined;
        if (!value || value.byteLength < 58 || value.byteLength > 512) {
          owner.close(new Error("BLE_FRAME_INVALID"));
          return;
        }
        const frame = new Uint8Array(
          value.buffer,
          value.byteOffset,
          value.byteLength,
        ).slice();
        if (uuidString(frame.slice(4, 20)) !== intent.client_operation_id)
          return;
        void assembler
          .accept(frame)
          .then((message) => {
            if (!message || owner.closed || this.owner !== owner) return;
            const decoded = decode(message.payload) as Record<string, unknown>;
            resolve({
              operation_id: message.id,
              state: String(decoded.state ?? "UNKNOWN"),
              certainty:
                decoded.state === "FISCALIZED" ? "PROVEN_SUCCESS" : "UNKNOWN",
              fiscal_reference: decoded.fiscal_reference
                ? String(decoded.fiscal_reference)
                : undefined,
              error_code: decoded.error_code
                ? String(decoded.error_code)
                : undefined,
              updated_at: new Date().toISOString(),
            });
          })
          .catch((error) => owner.close(error));
      });
      await owner.step(() => event.startNotifications());
      live();
      setupDeadline();
      const send = async () => {
        // 185 is a legacy offer, not evidence of negotiated browser ATT capacity.
        for (const frame of await frames(
          encoder.encode(intent),
          intent.client_operation_id,
        )) {
          live();
          if (!command.writeValueWithResponse)
            throw new Error("BLE_WRITE_UNSUPPORTED");
          await owner.step(() => command.writeValueWithResponse(frame), 5_000);
          live();
        }
      };
      const [, value] = await owner.wait(Promise.all([send(), result]));
      live();
      deadline();
      return value;
    } catch (error) {
      owner.close(error);
      throw error;
    } finally {
      owner.close();
      if (this.owner === owner) this.owner = undefined;
    }
  }
}
