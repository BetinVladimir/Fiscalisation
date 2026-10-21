import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setImmediate as immediate } from "node:timers";
import { readFileSync } from "node:fs";
import { createHash, webcrypto } from "node:crypto";
import { Encoder } from "cbor-x";
import { WebBleRoute } from "../src/fiscal/webBleRoute";
// The sibling Expo module uses explicit .ts imports; keep it out of the PWA
// application type graph while exercising the actual class through Vitest.
const miniPath = "../../BeeMiniPOS/src/webBle.ts";
const { WebBleBootstrap } = await import(/* @vite-ignore */ miniPath);
const id = "12345678-1234-4234-8234-123456789012";
const other = "22345678-1234-4234-8234-123456789012";
const cfg = {
  advertisingIdentity: "edge-1",
  serviceUUID: "service",
  commandUUID: "command",
  eventUUID: "event",
};
const session = {
  ble_session_id: id,
  tenant_id: "t",
  location_id: "l",
  register_id: "r",
  edge_id: "edge-1",
  device_id: "d",
  service_uuid: "service",
  command_characteristic_uuid: "command",
  event_characteristic_uuid: "event",
  advertising_identity: "edge-1",
  signed_session_ticket: "unused",
  expires_at: "2099-01-01T00:00:00Z",
  protocol_version: "1",
  security_mode: "OPEN_MVP" as const,
};
const encoder = new Encoder({
  useRecords: false,
  mapsAsObjects: true,
  variableMapSize: true,
});
function defer<T = any>() {
  let resolve!: (v: T) => void, reject!: (e: unknown) => void;
  const promise = new Promise<T>((a, b) => {
    resolve = a;
    reject = b;
  });
  return { promise, resolve, reject };
}
const flush = async () => {
  for (let round = 0; round < 4; round++) {
    for (let i = 0; i < 30; i++) await Promise.resolve();
    await new Promise<void>((done) => immediate(done));
  }
};
class Target {
  listeners = new Map<string, Set<(e: any) => void>>();
  retired: ((e: any) => void)[] = [];
  addEventListener(type: string, fn: (e: any) => void) {
    const s = this.listeners.get(type) ?? new Set();
    s.add(fn);
    this.listeners.set(type, s);
  }
  removeEventListener(type: string, fn: (e: any) => void) {
    this.listeners.get(type)?.delete(fn);
    this.retired.push(fn);
  }
  emit(type: string, event: any = {}) {
    for (const fn of [...(this.listeners.get(type) ?? [])]) fn(event);
  }
  count() {
    return [...this.listeners.values()].reduce((n, s) => n + s.size, 0);
  }
}
let nextDeviceId = 0;
function device() {
  const d: any = new Target();
  d.name = "edge-1";
  d.id = `fake-${++nextDeviceId}`;
  const event: any = new Target();
  event.startNotifications = vi.fn(async () => event);
  const command: any = new Target();
  command.writes = [];
  command.writeValueWithResponse = vi.fn(async (frame: Uint8Array) => {
    command.writes.push(frame.slice());
  });
  const service = {
    getCharacteristic: vi.fn(async (name: string) =>
      name === "command" ? command : event,
    ),
  };
  const server = { getPrimaryService: vi.fn(async () => service) };
  d.gatt = {
    connected: false,
    connect: vi.fn(async () => {
      d.gatt.connected = true;
      return server;
    }),
    disconnect: vi.fn(() => {
      d.gatt.connected = false;
      d.emit("gattserverdisconnected");
    }),
  };
  return { d, event, command, service, server };
}
function reply(operation = id, state = "FISCALIZED", chunk = 125) {
  const body = encoder.encode({ operation_id: operation, state });
  const digest = createHash("sha256").update(body).digest();
  const out: Uint8Array[] = [];
  for (let offset = 0; offset < body.length; offset += chunk) {
    const part = body.slice(offset, offset + chunk),
      f = new Uint8Array(57 + part.length),
      v = new DataView(f.buffer);
    f.set([66, 70, 70, 49]);
    f.set(Buffer.from(operation.replaceAll("-", ""), "hex"), 4);
    v.setUint16(20, body.length);
    v.setUint16(22, offset);
    f[24] = offset + part.length === body.length ? 1 : 0;
    f.set(digest, 25);
    f.set(part, 57);
    out.push(f);
  }
  return out;
}
function notify(event: any, f: Uint8Array) {
  const backing = new Uint8Array(f.length + 9);
  backing.set(f, 4);
  event.emit("characteristicvaluechanged", {
    target: { value: new DataView(backing.buffer, 4, f.length) },
  });
  backing.fill(0); // Adapter must copy synchronously before async digest/continuation.
}
for (const kind of ["mini", "pwa"] as const)
  describe(`${kind}: actual WebBluetooth transport`, () => {
    let sdk: ReturnType<typeof device>,
      request: any,
      controller: AbortController,
      transport: any;
    let events: Uint8Array[];
    beforeEach(() => {
      vi.useFakeTimers();
      sdk = device();
      request = vi.fn(async () => sdk.d);
      controller = new AbortController();
      events = [];
      vi.stubGlobal("navigator", { bluetooth: { requestDevice: request } });
      vi.stubGlobal("isSecureContext", true);
      vi.stubGlobal("crypto", webcrypto);
      transport =
        kind === "mini" ? new WebBleBootstrap() : new WebBleRoute(cfg);
    });
    afterEach(async () => {
      transport.resetSession?.();
      controller.abort();
      await flush();
      vi.useRealTimers();
      vi.unstubAllGlobals();
    });
    async function start(
      intent: any = { client_operation_id: id },
      signal = controller.signal,
    ) {
      if (kind === "mini") {
        await transport.connectFromUserGesture(
          session,
          (raw: Uint8Array) => events.push(raw),
          signal,
        );
        return;
      }
    }
    function send(
      intent: any = { client_operation_id: id },
      signal = controller.signal,
    ) {
      return kind === "mini"
        ? transport.sendComplianceIntentAndWait(intent, id, 185, 30000, signal)
        : transport.submitCommand(intent, signal);
    }
    it.skipIf(!process.env.CBOR_DELIVERY_FIXTURES)("actual S3 delivery int64 bytes survive real result projection", async () => {
      const rows=readFileSync(process.env.CBOR_DELIVERY_FIXTURES!,"utf8").trim().split("\n").map(x=>JSON.parse(x));
      expect(rows).toHaveLength(12);
      for(const row of rows){
        await start({client_operation_id:row.id});
        const result=kind==="mini"?transport.sendComplianceIntentAndWait({client_operation_id:row.id},row.id,185,30000,controller.signal):send({client_operation_id:row.id});await flush();
        for(const f of row.frames)notify(sdk.event,Buffer.from(f,"hex"));
        await flush();if(kind==="mini")expect(events.length).toBeGreaterThan(0);
        const value=await result;expect(value.state).toBe(row.state);expect(value.fiscal_reference).toBe("FD-1");
        if(kind==="mini")expect(value.payment_evidence[0].amount_minor).toBe(row.amount==="2147483648"?2147483648:BigInt(row.amount));
        else expect(value.certainty).toBe(row.state==="FISCALIZED"?"PROVEN_SUCCESS":"UNKNOWN");
        transport.resetSession?.();await flush();
      }
    });
    it("installs listeners before subscription and copies offset DataView; wire unchanged", async () => {
      sdk.event.startNotifications.mockImplementation(async () => {
        expect(sdk.event.count()).toBe(1);
        expect(sdk.d.count()).toBe(1);
        return sdk.event;
      });
      await start();
      const result = send();
      await flush();
      expect(sdk.command.writes.length).toBeGreaterThan(0);
      for (const f of sdk.command.writes) {
        expect([...f.slice(0, 4)]).toEqual([66, 70, 70, 49]);
        expect(f.length).toBeLessThanOrEqual(182);
      }
      for (const f of reply()) notify(sdk.event, f);
      expect((await result).state).toBe("FISCALIZED");
      if (kind === "mini") {
        expect(events.length).toBe(1);
        transport.resetSession();
      }
      expect(sdk.event.count()).toBe(0);
      expect(sdk.d.count()).toBe(0);
      expect(vi.getTimerCount()).toBe(0);
    });
    it("aborted chooser cannot start a late connect", async () => {
      const choose = defer();
      request.mockReturnValue(choose.promise);
      const result =
        kind === "mini"
          ? transport.connectFromUserGesture(
              session,
              () => {},
              controller.signal,
            )
          : send();
      const rejected = expect(result).rejects.toMatchObject({
        name: "AbortError",
      });
      controller.abort();
      await rejected;
      choose.resolve(sdk.d);
      await flush();
      expect(sdk.d.gatt.connect).not.toHaveBeenCalled();
      expect(sdk.d.count()).toBe(0);
    });
    it("already aborted signal never opens chooser", async () => {
      controller.abort();
      const result =
        kind === "mini"
          ? transport.connectFromUserGesture(
              session,
              () => {},
              controller.signal,
            )
          : send();
      await expect(result).rejects.toMatchObject({ name: "AbortError" });
      expect(request).not.toHaveBeenCalled();
    });
    it("native pending connect quarantines same-GATT replacement until settlement", async () => {
      const connect = defer();
      sdk.d.gatt.connect.mockImplementation(() =>
        connect.promise.then(() => {
          sdk.d.gatt.connected = true;
          return sdk.server;
        }),
      );
      const first =
        kind === "mini"
          ? transport.connectFromUserGesture(
              session,
              () => {},
              controller.signal,
            )
          : send();
      const rejected = expect(first).rejects.toMatchObject({
        name: "AbortError",
      });
      await flush();
      controller.abort();
      await rejected;
      const replacement: any =
        kind === "mini" ? new WebBleBootstrap() : new WebBleRoute(cfg);
      const fresh = () =>
        kind === "mini"
          ? replacement.connectFromUserGesture(session, () => {})
          : replacement.submitCommand(
              { client_operation_id: id },
              new AbortController().signal,
            );
      await expect(fresh()).rejects.toThrow("BLE_CONNECTION_DRAINING");
      expect(sdk.d.gatt.connect).toHaveBeenCalledTimes(1);
      connect.resolve(sdk.server);
      await flush();
      expect(sdk.d.gatt.connected).toBe(false);
      sdk.d.gatt.connect.mockImplementation(async () => {
        sdk.d.gatt.connected = true;
        return sdk.server;
      });
      const next = fresh();
      await flush();
      expect(sdk.d.gatt.connected).toBe(true);
      if (kind === "mini") {
        await next;
        replacement.resetSession();
      } else {
        for (const f of reply()) notify(sdk.event, f);
        await next;
      }
    });
    it("same physical id with different JS wrappers remains quarantined until old connect settles", async () => {
      const old = sdk,
        connect = defer();
      old.d.gatt.connect.mockImplementation(() =>
        connect.promise.then(() => {
          old.d.gatt.connected = true;
          return old.server;
        }),
      );
      const first =
        kind === "mini"
          ? transport.connectFromUserGesture(
              session,
              () => {},
              controller.signal,
            )
          : send();
      const rejected = expect(first).rejects.toMatchObject({
        name: "AbortError",
      });
      await flush();
      controller.abort();
      await rejected;
      sdk = device();
      sdk.d.id = old.d.id;
      request.mockResolvedValue(sdk.d);
      controller = new AbortController();
      const replacement: any =
        kind === "mini" ? new WebBleBootstrap() : new WebBleRoute(cfg);
      const fresh = () =>
        kind === "mini"
          ? replacement.connectFromUserGesture(
              session,
              () => {},
              controller.signal,
            )
          : replacement.submitCommand(
              { client_operation_id: id },
              controller.signal,
            );
      const denied = fresh();
      void denied.catch(() => {});
      await flush();
      expect(sdk.d.gatt.connect).not.toHaveBeenCalled();
      await expect(denied).rejects.toThrow("BLE_CONNECTION_DRAINING");
      connect.resolve(old.server);
      await flush();
      expect(old.d.gatt.connected).toBe(false);
      const next = fresh();
      await flush();
      expect(sdk.d.gatt.connected).toBe(true);
      expect(sdk.d.gatt.disconnect).not.toHaveBeenCalled();
      if (kind === "mini") {
        await next;
        replacement.resetSession();
      } else {
        for (const f of reply()) notify(sdk.event, f);
        await next;
      }
    });
    it("pending native device admission is bounded and released after settlement", async () => {
      const pending: {
        deferred: ReturnType<typeof defer>;
        sdk: ReturnType<typeof device>;
      }[] = [];
      try {
        for (let i = 0; i < 32; i++) {
          const one = device(),
            native = defer();
          one.d.gatt.connect.mockReturnValue(native.promise);
          request.mockResolvedValue(one.d);
          const t: any =
              kind === "mini" ? new WebBleBootstrap() : new WebBleRoute(cfg),
            abort = new AbortController();
          const task =
            kind === "mini"
              ? t.connectFromUserGesture(session, () => {}, abort.signal)
              : t.submitCommand({ client_operation_id: id }, abort.signal);
          const rejection = expect(task).rejects.toMatchObject({
            name: "AbortError",
          });
          await flush();
          expect(one.d.gatt.connect).toHaveBeenCalledTimes(1);
          abort.abort();
          await rejection;
          pending.push({ deferred: native, sdk: one });
        }
        const extra = device();
        request.mockResolvedValue(extra.d);
        const t: any =
          kind === "mini" ? new WebBleBootstrap() : new WebBleRoute(cfg);
        await expect(
          kind === "mini"
            ? t.connectFromUserGesture(session, () => {})
            : t.submitCommand({ client_operation_id: id }, controller.signal),
        ).rejects.toThrow("BLE_CONNECTION_CAPACITY");
        expect(extra.d.gatt.connect).not.toHaveBeenCalled();
      } finally {
        for (const p of pending) p.deferred.resolve(p.sdk.server);
        await flush();
      }
    });
    for (const stage of [
      "connect",
      "service",
      "command",
      "event",
      "subscribe",
    ] as const) {
      for (const mode of ["abort", "timeout", "reject"] as const) {
        it(`${mode} during ${stage} leaves no stale setup continuation`, async () => {
          const pending = defer();
          const fn =
            stage === "connect"
              ? sdk.d.gatt.connect
              : stage === "service"
                ? sdk.server.getPrimaryService
                : stage === "subscribe"
                  ? sdk.event.startNotifications
                  : sdk.service.getCharacteristic;
          if (stage === "command" || stage === "event")
            sdk.service.getCharacteristic.mockImplementation((name: string) =>
              name === stage ? pending.promise : Promise.resolve(sdk.command),
            );
          else fn.mockReturnValue(pending.promise);
          const task =
            kind === "mini"
              ? transport.connectFromUserGesture(
                  session,
                  () => {},
                  controller.signal,
                )
              : send();
          const failure = expect(task).rejects.toThrow(
            mode === "abort"
              ? "aborted"
              : mode === "timeout"
                ? "BLE_STEP_TIMEOUT"
                : "SDK_REJECTED",
          );
          await flush();
          const counts = () => [
            sdk.server.getPrimaryService.mock.calls.length,
            sdk.service.getCharacteristic.mock.calls.length,
            sdk.event.startNotifications.mock.calls.length,
          ];
          const before = counts();
          if (mode === "abort") controller.abort();
          else if (mode === "timeout") await vi.advanceTimersByTimeAsync(10000);
          else pending.reject(new Error("SDK_REJECTED"));
          await failure;
          if (mode !== "reject")
            pending.resolve(
              stage === "connect"
                ? sdk.server
                : stage === "service"
                  ? sdk.service
                  : stage === "command"
                    ? sdk.command
                    : sdk.event,
            );
          await flush();
          expect(counts()).toEqual(before);
          expect(sdk.command.writeValueWithResponse).not.toHaveBeenCalled();
          expect(sdk.event.count()).toBe(0);
          expect(sdk.d.count()).toBe(0);
          expect(vi.getTimerCount()).toBe(0);
          if (kind === "mini")
            expect(transport.canSendComplianceIntent).toBe(false);
        });
      }
    }
    if (kind === "mini") {
      it("passes an explicit legacy MTU offer to framing and rejects ATT20 without I/O", async () => {
        await start();
        const task = transport.sendComplianceIntentAndWait(
          { client_operation_id: id, padding: "x".repeat(200) },
          id,
          61,
          30000,
        );
        await flush();
        expect(sdk.command.writes.length).toBeGreaterThan(2);
        expect(
          sdk.command.writes.every((f: Uint8Array) => f.length <= 58),
        ).toBe(true);
        for (const f of reply()) notify(sdk.event, f);
        await task;
        const before = sdk.command.writes.length;
        await expect(
          transport.sendComplianceIntentAndWait(
            { client_operation_id: id },
            id,
            23,
            30000,
          ),
        ).rejects.toThrow("BLE_MTU_TOO_SMALL");
        expect(sdk.command.writes.length).toBe(before);
      });
      it("invalid/expired sessions never start a chooser", async () => {
        for (const expires_at of ["invalid", "2000-01-01T00:00:00Z"])
          await expect(
            transport.connectFromUserGesture(
              { ...session, expires_at },
              () => {},
            ),
          ).rejects.toThrow("BLE_ROUTE_PACKAGE_EXPIRED");
        expect(request).not.toHaveBeenCalled();
      });
    }
    it("disconnect during subscribe never publishes ready or writes", async () => {
      sdk.event.startNotifications.mockImplementation(async () => {
        sdk.d.emit("gattserverdisconnected");
        return sdk.event;
      });
      const result =
        kind === "mini"
          ? transport.connectFromUserGesture(
              session,
              () => {},
              controller.signal,
            )
          : send();
      await expect(result).rejects.toThrow("BLE_DISCONNECTED");
      expect(sdk.command.writeValueWithResponse).not.toHaveBeenCalled();
      if (kind === "mini")
        expect(transport.canSendComplianceIntent).toBe(false);
      expect(sdk.event.count()).toBe(0);
      expect(sdk.d.count()).toBe(0);
    });
    it("setup step deadline retires listeners and prevents late continuation", async () => {
      const service = defer();
      sdk.server.getPrimaryService.mockReturnValue(service.promise);
      const result =
        kind === "mini"
          ? transport.connectFromUserGesture(
              session,
              () => {},
              controller.signal,
            )
          : send();
      const rejected = expect(result).rejects.toThrow("BLE_STEP_TIMEOUT");
      await flush();
      await vi.advanceTimersByTimeAsync(10000);
      await rejected;
      service.resolve(sdk.service);
      await flush();
      expect(sdk.service.getCharacteristic).not.toHaveBeenCalled();
      expect(sdk.d.count()).toBe(0);
    });
    it("abort during write stops subsequent frames; stale notifications cannot finish", async () => {
      const write = defer();
      const writeEntered = defer<void>();
      sdk.command.writeValueWithResponse.mockImplementation(
        () => { writeEntered.resolve(); return write.promise; },
      );
      await start();
      const result = send({
        client_operation_id: id,
        padding: "x".repeat(1000),
      });
      const rejected = expect(result).rejects.toMatchObject({
        name: "AbortError",
      });
      // Await actual write admission: WebCrypto can settle after an arbitrary
      // number of event-loop turns on a loaded host.
      await writeEntered.promise;
      expect(sdk.command.writeValueWithResponse).toHaveBeenCalledTimes(1);
      controller.abort();
      await rejected;
      for (const f of reply())
        for (const listener of sdk.event.retired)
          listener({ target: { value: new DataView(f.buffer) } });
      write.resolve(undefined);
      await flush();
      expect(sdk.command.writeValueWithResponse).toHaveBeenCalledTimes(1);
      expect(events).toHaveLength(0);
    });
    it("early fiscal result does not release a hung write and never resubmits", async () => {
      const stuck = defer();
      sdk.command.writeValueWithResponse.mockImplementation(
        () => stuck.promise,
      );
      await start();
      const result = send();
      const rejected = expect(result).rejects.toThrow("BLE_STEP_TIMEOUT");
      await flush();
      for (const f of reply()) notify(sdk.event, f);
      await flush();
      await vi.advanceTimersByTimeAsync(5000);
      await rejected;
      expect(sdk.command.writeValueWithResponse).toHaveBeenCalledTimes(1);
      expect(sdk.event.count()).toBe(0);
      stuck.resolve(undefined);
      await flush();
    });
    it("write length rejection fails closed without resizing or retry", async () => {
      sdk.command.writeValueWithResponse.mockRejectedValue(
        new DOMException(
          "ATT value exceeds capacity",
          "InvalidModificationError",
        ),
      );
      await start();
      await expect(
        send({ client_operation_id: id, padding: "x".repeat(500) }),
      ).rejects.toMatchObject({ name: "InvalidModificationError" });
      expect(sdk.command.writeValueWithResponse).toHaveBeenCalledTimes(1);
      expect(sdk.d.gatt.connected).toBe(false);
    });
    it("foreign message floods do not allocate assembler state or complete operation", async () => {
      await start();
      const result = send();
      let settled = false;
      void result.then(
        () => (settled = true),
        () => (settled = true),
      );
      await flush();
      for (let i = 0; i < 100; i++)
        for (const f of reply(other, "FISCALIZED", 1)) notify(sdk.event, f);
      await flush();
      expect(settled).toBe(false);
      for (const f of reply()) notify(sdk.event, f);
      expect((await result).operation_id).toBe(id);
    });
    it("concurrent operation is rejected before additional I/O", async () => {
      await start();
      const result = send();
      await flush();
      await expect(send()).rejects.toThrow("BLE_OPERATION_IN_PROGRESS");
      for (const f of reply()) notify(sdk.event, f);
      await result;
      expect(request).toHaveBeenCalledTimes(1);
    });
    it("malformed final/bounds/overflow frames never produce fiscal success", async () => {
      for (const mutate of [
        (f: Uint8Array) => {
          f[24] = 2;
        },
        (f: Uint8Array) => {
          new DataView(f.buffer).setUint16(20, 8193);
        },
        (f: Uint8Array) => {
          new DataView(f.buffer).setUint16(22, 8191);
        },
        (f: Uint8Array) => {
          f[24] = 0;
        },
        (f: Uint8Array) => {
          f[25] ^= 1;
        },
      ]) {
        if (kind === "mini" && !transport.canSendComplianceIntent)
          await start();
        const result = send();
        let success = false;
        void result.then(
          () => (success = true),
          () => {},
        );
        await flush();
        const frame = reply()[0];
        mutate(frame);
        notify(sdk.event, frame);
        await flush();
        expect(success).toBe(false);
        if (kind === "pwa") await expect(result).rejects.toThrow(/BLE_FRAME_/);
        else {
          for (const f of reply()) notify(sdk.event, f);
          await expect(result).resolves.toMatchObject({ state: "FISCALIZED" });
        }
      }
    });
    it("late async digest after abort cannot complete or corrupt replacement", async () => {
      await start();
      const result = send();
      const rejected = expect(result).rejects.toMatchObject({
        name: "AbortError",
      });
      await flush();
      const digest = defer<ArrayBuffer>();
      const original = webcrypto.subtle.digest.bind(webcrypto.subtle);
      // MiniPOS digest is synchronous but dispatches completion in a microtask;
      // PWA uses asynchronous SubtleCrypto. Delay precisely that result verification.
      const spy =
        kind === "pwa"
          ? vi
              .spyOn(webcrypto.subtle, "digest")
              .mockImplementationOnce(() => digest.promise)
          : undefined;
      for (const f of reply()) notify(sdk.event, f);
      controller.abort();
      await rejected;
      if (kind === "mini") expect(events).toHaveLength(0);
      spy?.mockRestore();
      const nextController = new AbortController();
      controller = nextController;
      await start();
      const next = send();
      await flush();
      if (kind === "pwa")
        digest.resolve(
          await original(
            "SHA-256",
            encoder.encode({ operation_id: id, state: "FISCALIZED" }),
          ),
        );
      await flush();
      let done = false;
      void next.then(
        () => (done = true),
        () => {},
      );
      await flush();
      expect(done).toBe(false);
      for (const f of reply()) notify(sdk.event, f);
      await next;
    });
    it("result deadline closes idle result wait and cleans listeners", async () => {
      await start();
      const result = send();
      const rejected = expect(result).rejects.toThrow("BLE_RESULT_TIMEOUT");
      await flush();
      await vi.advanceTimersByTimeAsync(kind === "mini" ? 30000 : 120000);
      await rejected;
      expect(sdk.event.count()).toBe(0);
      expect(sdk.d.count()).toBe(0);
      expect(vi.getTimerCount()).toBe(0);
    });
  });
