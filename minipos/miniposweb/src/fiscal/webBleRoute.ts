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
const uuidBytes = (id: string) =>
  Uint8Array.from(
    id
      .replaceAll("-", "")
      .match(/../g)!
      .map((value) => parseInt(value, 16)),
  );
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
  private values = new Map<
    string,
    { total: number; digest: Uint8Array; offset: number; parts: Uint8Array[] }
  >();
  async accept(frame: Uint8Array) {
    if (
      frame.length <= HEADER ||
      !MAGIC.every((value, index) => frame[index] === value)
    )
      throw new Error("BLE_FRAME_INVALID");
    const id = uuidString(frame.slice(4, 20)),
      view = new DataView(frame.buffer, frame.byteOffset, frame.byteLength),
      total = view.getUint16(20),
      offset = view.getUint16(22),
      final = frame[24] === 1,
      digest = frame.slice(25, 57),
      part = frame.slice(57);
    let state = this.values.get(id);
    if (!state) {
      if (offset !== 0) throw new Error("BLE_FRAME_ORDER");
      state = { total, digest, offset: 0, parts: [] };
      this.values.set(id, state);
    }
    if (
      total !== state.total ||
      offset !== state.offset ||
      !digest.every((v, i) => v === state!.digest[i])
    )
      throw new Error("BLE_FRAME_CONFLICT");
    state.parts.push(part);
    state.offset += part.length;
    if (!final) return;
    const payload = new Uint8Array(total);
    let at = 0;
    for (const value of state.parts) {
      payload.set(value, at);
      at += value.length;
    }
    this.values.delete(id);
    const actual = new Uint8Array(
      await crypto.subtle.digest("SHA-256", payload),
    );
    if (!actual.every((v, i) => v === digest[i]))
      throw new Error("BLE_FRAME_DIGEST");
    return { id, payload };
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
    const bluetooth = (
      navigator as unknown as {
        bluetooth: { requestDevice(value: unknown): Promise<any> };
      }
    ).bluetooth;
    if (!bluetooth) throw new Error("WEB_BLUETOOTH_UNAVAILABLE");
    const device = await bluetooth.requestDevice({
      filters: [{ name: this.configuration.advertisingIdentity }],
      optionalServices: [this.configuration.serviceUUID],
    });
    const server = await device.gatt.connect(),
      service = await server.getPrimaryService(this.configuration.serviceUUID),
      command = await service.getCharacteristic(this.configuration.commandUUID),
      event = await service.getCharacteristic(this.configuration.eventUUID),
      assembler = new Assembler();
    await event.startNotifications();
    const result = new Promise<FiscalOperation>((resolve, reject) => {
      const timeout = setTimeout(
        () => reject(new Error("BLE_RESULT_TIMEOUT")),
        120000,
      );
      signal.addEventListener(
        "abort",
        () => {
          clearTimeout(timeout);
          reject(new DOMException("aborted", "AbortError"));
        },
        { once: true },
      );
      event.addEventListener("characteristicvaluechanged", async (e: any) => {
        try {
          const value = new Uint8Array(
              e.target.value.buffer,
              e.target.value.byteOffset,
              e.target.value.byteLength,
            ),
            message = await assembler.accept(value);
          if (!message || message.id !== intent.client_operation_id) return;
          clearTimeout(timeout);
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
        } catch (error) {
          clearTimeout(timeout);
          reject(error);
        }
      });
    });
    const payload = encoder.encode(intent);
    for (const frame of await frames(payload, intent.client_operation_id))
      await command.writeValueWithResponse(frame);
    return result;
  }
}
