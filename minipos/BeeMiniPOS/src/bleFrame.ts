import {
  aesGcmDecrypt,
  aesGcmEncrypt,
  hash256,
  hmac256,
} from "./portableCrypto.ts";
const te = new TextEncoder();
const HEADER_SIZE = 34,
  TAG_SIZE = 16;
function uuidBytes(raw: string) {
  const hex = raw.replaceAll("-", "");
  if (!/^[a-f0-9]{32}$/i.test(hex)) throw new Error("BLE_MESSAGE_ID_INVALID");
  const out = new Uint8Array(16);
  for (let i = 0; i < 16; i++)
    out[i] = Number.parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  return out;
}

export class BleFrameSession {
  private counter = 0n;
  private rxCounter = 0n;
  private readonly txKey: Uint8Array;
  private readonly rxKey: Uint8Array;
  private readonly txPrefix: Uint8Array;
  private readonly rxPrefix: Uint8Array;
  private constructor(
    txKey: Uint8Array,
    rxKey: Uint8Array,
    txPrefix: Uint8Array,
    rxPrefix: Uint8Array,
  ) {
    this.txKey = txKey;
    this.rxKey = rxKey;
    this.txPrefix = txPrefix;
    this.rxPrefix = rxPrefix;
  }
  static client(secret: Uint8Array, sessionId: string) {
    return this.endpoint(secret, sessionId, "client");
  }
  static edge(secret: Uint8Array, sessionId: string) {
    return this.endpoint(secret, sessionId, "edge");
  }
  private static async endpoint(
    secret: Uint8Array,
    sessionId: string,
    role: "client" | "edge",
  ) {
    if (secret.length !== 32 || !sessionId)
      throw new Error("BLE_SESSION_KEY_INVALID");
    const derived = (label: string) => hmac256(secret, te.encode(label));
    const c2e = derived(sessionId + ":client-to-edge"),
      e2c = derived(sessionId + ":edge-to-client"),
      c2ePrefix = hash256(te.encode("c2e:" + sessionId)).slice(0, 4),
      e2cPrefix = hash256(te.encode("e2c:" + sessionId)).slice(0, 4);
    return role === "client"
      ? new BleFrameSession(c2e, e2c, c2ePrefix, e2cPrefix)
      : new BleFrameSession(e2c, c2e, e2cPrefix, c2ePrefix);
  }
  async seal(
    messageId: string,
    chunkIndex: number,
    chunkCount: number,
    flags: number,
    plain: Uint8Array,
  ) {
    if (
      chunkCount < 1 ||
      chunkCount > 65535 ||
      chunkIndex < 0 ||
      chunkIndex >= chunkCount ||
      plain.length > 65535 ||
      flags < 0 ||
      flags > 255
    )
      throw new Error("BLE_CHUNK_INVALID");
    this.counter++;
    const header = new Uint8Array(HEADER_SIZE);
    header[0] = 0x42;
    header[1] = 0x46;
    header[2] = 1;
    header[3] = flags;
    header.set(uuidBytes(messageId), 4);
    const view = new DataView(header.buffer);
    view.setBigUint64(20, this.counter);
    view.setUint16(28, chunkIndex);
    view.setUint16(30, chunkCount);
    view.setUint16(32, plain.length);
    const nonce = new Uint8Array(12);
    nonce.set(this.txPrefix);
    new DataView(nonce.buffer).setBigUint64(4, this.counter);
    const sealed = aesGcmEncrypt(this.txKey, nonce, plain, header);
    const out = new Uint8Array(header.length + sealed.length);
    out.set(header);
    out.set(sealed, header.length);
    return out;
  }
  async open(raw: Uint8Array) {
    if (
      raw.length < HEADER_SIZE + TAG_SIZE ||
      raw[0] !== 0x42 ||
      raw[1] !== 0x46 ||
      raw[2] !== 1
    )
      throw new Error("BLE_FRAME_INVALID");
    const header = raw.slice(0, HEADER_SIZE),
      view = new DataView(header.buffer, header.byteOffset, header.byteLength),
      counter = view.getBigUint64(20),
      chunkIndex = view.getUint16(28),
      chunkCount = view.getUint16(30),
      length = view.getUint16(32);
    if (counter <= this.rxCounter) throw new Error("BLE_FRAME_REPLAY");
    if (
      chunkCount < 1 ||
      chunkIndex >= chunkCount ||
      raw.length !== HEADER_SIZE + length + TAG_SIZE
    )
      throw new Error("BLE_FRAME_INVALID");
    const nonce = new Uint8Array(12);
    nonce.set(this.rxPrefix);
    new DataView(nonce.buffer).setBigUint64(4, counter);
    let plain: Uint8Array;
    try {
      plain = aesGcmDecrypt(this.rxKey, nonce, raw.slice(HEADER_SIZE), header);
    } catch {
      throw new Error("BLE_FRAME_BAD_TAG");
    }
    this.rxCounter = counter;
    return {
      counter,
      chunkIndex,
      chunkCount,
      flags: view.getUint8(3),
      messageId: hexUUID(header.slice(4, 20)),
      plaintext: plain,
    };
  }
}

function hexUUID(v: Uint8Array) {
  const h = Array.from(v, (x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export function chunkPlaintext(payload: Uint8Array, attMtu: number) {
  const max = attMtu - 3 - HEADER_SIZE - TAG_SIZE;
  if (max < 1) throw new Error("BLE_MTU_TOO_SMALL");
  if (payload.length === 0) return [new Uint8Array()];
  const out: Uint8Array[] = [];
  for (let p = 0; p < payload.length; p += max)
    out.push(payload.slice(p, Math.min(payload.length, p + max)));
  if (out.length > 65535) throw new Error("BLE_MESSAGE_TOO_LARGE");
  return out;
}
