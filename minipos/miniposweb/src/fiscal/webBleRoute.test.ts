import { it, expect, vi } from "vitest";
import { WebBleRoute } from "./webBleRoute";
import type { WebBleConfiguration } from "./webBleRoute";
import { Encoder } from "cbor-x";

// CL-02.1: after UNKNOWN result, next submit() skips send() (sendCalls==1)

const cfg: WebBleConfiguration = {
  advertisingIdentity: "TestDevice",
  serviceUUID: "00001234-0000-1000-8000-00805f9b34fb",
  commandUUID: "00001235-0000-1000-8000-00805f9b34fb",
  eventUUID: "00001236-0000-1000-8000-00805f9b34fb",
};

const enc = new Encoder({
  useRecords: false,
  mapsAsObjects: true,
  structuredClone: false,
  variableMapSize: true,
});

const MAGIC = [0x42, 0x46, 0x46, 0x31];
const HEADER = 57;
const OPERATION_ID = "11111111-1111-4111-8111-111111111111";

async function buildResponseFrame(
  operationId: string,
  state: string,
): Promise<DataView> {
  const payload = enc.encode({ state });
  const digest = new Uint8Array(
    await globalThis.crypto.subtle.digest("SHA-256", payload),
  );
  const frame = new Uint8Array(HEADER + payload.length);
  const view = new DataView(frame.buffer);
  MAGIC.forEach((v, i) => (frame[i] = v));
  const idHex = operationId.replaceAll("-", "");
  const idBytes = Uint8Array.from(
    idHex.match(/../g)!.map((h: string) => parseInt(h, 16)),
  );
  frame.set(idBytes, 4);
  view.setUint16(20, payload.length);
  view.setUint16(22, 0);
  frame[24] = 1;
  frame.set(digest, 25);
  frame.set(payload, HEADER);
  return new DataView(frame.buffer, 0, frame.length);
}

vi.mock("./browserGatt", () => ({
  BrowserGattOwner: class {
    closed = false;
    private closeCbs: (() => void)[] = [];
    bind() {}
    close() {
      this.closed = true;
      this.closeCbs.forEach((f) => f());
    }
    onClose(f: () => void) {
      this.closeCbs.push(f);
    }
    guard() {
      if (this.closed) throw new Error("owner closed");
    }
    async wait<T>(p: Promise<T>) {
      return p;
    }
    async step<T>(fn: () => Promise<T>) {
      return fn();
    }
    listen(t: any, e: string, fn: (ev: any) => void) {
      t._listeners ??= {};
      (t._listeners[e] ??= []).push(fn);
    }
    claim(_d: any) {}
    timer(_ms: number, _code: string) {
      return () => {};
    }
  },
}));

function fireEvent(target: any, event: string, payload: any) {
  for (const fn of target._listeners?.[event] ?? []) fn(payload);
}

/**
 * Build a BLE mock.
 * fireOnSubscribe=true: fires the response frame when startNotifications is called
 *   (simulates device pushing status on reconnect, used for recovery/poll flow)
 * fireOnWrite=true: fires the response frame when writeValueWithResponse is called
 *   (normal send flow)
 */
function makeBleMock(responseState: string, fireOnSubscribe: boolean) {
  let writeCalls = 0;
  const eventChar: any = {
    removeEventListener: vi.fn(),
    _listeners: {},
  };
  eventChar.startNotifications = vi.fn().mockImplementation(async () => {
    if (fireOnSubscribe) {
      const resp = await buildResponseFrame(OPERATION_ID, responseState);
      fireEvent(eventChar, "characteristicvaluechanged", { target: { value: resp } });
    }
  });
  eventChar.addEventListener = vi.fn().mockImplementation(
    (type: string, fn: (ev: any) => void) => {
      eventChar._listeners[type] ??= [];
      eventChar._listeners[type].push(fn);
    },
  );
  const commandChar: any = {
    writeValueWithResponse: vi.fn().mockImplementation(async (frame: Uint8Array) => {
      writeCalls++;
      if (!fireOnSubscribe && frame[24] === 1) {
        const resp = await buildResponseFrame(OPERATION_ID, responseState);
        fireEvent(eventChar, "characteristicvaluechanged", { target: { value: resp } });
      }
    }),
  };
  const device: any = {
    name: cfg.advertisingIdentity,
    gatt: {
      connect: vi.fn().mockResolvedValue({
        getPrimaryService: vi.fn().mockResolvedValue({
          getCharacteristic: vi.fn().mockImplementation(async (uuid: string) => {
            if (uuid === cfg.commandUUID) return commandChar;
            if (uuid === cfg.eventUUID) return eventChar;
          }),
        }),
      }),
    },
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  };
  return {
    bluetooth: { requestDevice: vi.fn().mockResolvedValue(device) },
    commandChar,
    getWriteCalls: () => writeCalls,
  };
}

it("UNKNOWN result: send called once; second submit skips send (sendCalls==1)", async () => {
  // First submit: sends the command, device returns UNKNOWN result
  const mock1 = makeBleMock("FAILED", false);
  vi.stubGlobal("navigator", { bluetooth: mock1.bluetooth });
  const route = new WebBleRoute(cfg);
  const intent = { client_operation_id: OPERATION_ID };
  const signal = new AbortController().signal;

  const r1 = await route.submit(intent, signal);
  expect(r1.certainty).toBe("UNKNOWN");
  expect(mock1.getWriteCalls()).toBe(1);

  // Second submit: recovery poll — device pushes status on subscribe, no write
  const mock2 = makeBleMock("FAILED", true);
  vi.stubGlobal("navigator", { bluetooth: mock2.bluetooth });

  const r2 = await route.submit(intent, signal);
  expect(r2.certainty).toBe("UNKNOWN");
  expect(mock2.getWriteCalls()).toBe(0);

  vi.unstubAllGlobals();
});

it("PROVEN_SUCCESS result: next submit sends again", async () => {
  const mock1 = makeBleMock("FISCALIZED", false);
  vi.stubGlobal("navigator", { bluetooth: mock1.bluetooth });
  const route = new WebBleRoute(cfg);
  const intent = { client_operation_id: OPERATION_ID };
  const signal = new AbortController().signal;

  const r1 = await route.submit(intent, signal);
  expect(r1.certainty).toBe("PROVEN_SUCCESS");
  expect(mock1.getWriteCalls()).toBe(1);

  // Not UNKNOWN — next submit also sends
  const mock2 = makeBleMock("FISCALIZED", false);
  vi.stubGlobal("navigator", { bluetooth: mock2.bluetooth });

  const r2 = await route.submit(intent, signal);
  expect(r2.certainty).toBe("PROVEN_SUCCESS");
  expect(mock2.getWriteCalls()).toBe(1);

  vi.unstubAllGlobals();
});
