// Web Bluetooth promises cannot be cancelled. Retain admission until every native
// call settles, so a late connect cannot reconnect a replacement owner's GATT.
const owners = new WeakMap<object, BrowserGattOwner>();
// device.id is an origin-scoped lifecycle key, never an authentication identity.
// New JS wrappers may still operate on the same underlying physical connection.
const devices = new Map<string, BrowserGattOwner>();
const MAX_DEVICE_LEASES = 32;
export class BrowserGattOwner {
  closed = false;
  private error = new Error("BLE_DISCONNECTED");
  private reject!: (error: Error) => void;
  private cancelled = new Promise<never>((_, reject) => {
    this.reject = reject;
  });
  private cleanup = new Set<() => void>();
  private pending = 0;
  private gatt?: any;
  private deviceId?: string;
  constructor() {
    void this.cancelled.catch(() => {});
  }
  onClose(cleanup: () => void) {
    if (this.closed) cleanup();
    else this.cleanup.add(cleanup);
  }
  guard() {
    if (this.closed) throw this.error;
  }
  bind(signal?: AbortSignal) {
    if (!signal) return () => {};
    const abort = () => this.close(new DOMException("aborted", "AbortError"));
    signal.addEventListener("abort", abort, { once: true });
    const remove = () => {
      signal.removeEventListener("abort", abort);
      this.cleanup.delete(remove);
    };
    this.cleanup.add(remove);
    if (signal.aborted) abort();
    return remove;
  }
  claim(device: any) {
    this.guard();
    const gatt = device.gatt;
    if (!gatt) throw new Error("BLE_GATT_UNAVAILABLE");
    const id = device.id;
    if (typeof id !== "string" || !id.length || id.length > 512)
      throw new Error("BLE_DEVICE_ID_INVALID");
    if (owners.has(gatt) || devices.has(id))
      throw new Error("BLE_CONNECTION_DRAINING");
    if (devices.size >= MAX_DEVICE_LEASES)
      throw new Error("BLE_CONNECTION_CAPACITY");
    owners.set(gatt, this);
    devices.set(id, this);
    this.gatt = gatt;
    this.deviceId = id;
    this.listen(device, "gattserverdisconnected", () => this.close());
  }
  listen(target: any, type: string, listener: (event: any) => void) {
    this.guard();
    target.addEventListener(type, listener);
    this.cleanup.add(() => target.removeEventListener(type, listener));
  }
  timer(milliseconds: number, code: string) {
    const timer = setTimeout(() => this.close(new Error(code)), milliseconds);
    const remove = () => {
      clearTimeout(timer);
      this.cleanup.delete(remove);
    };
    this.cleanup.add(remove);
    return remove;
  }
  private release() {
    if (this.gatt && owners.get(this.gatt) === this && this.pending === 0) {
      // Synchronous cleanup precedes admission of a replacement using this object.
      try {
        this.gatt.disconnect?.();
      } catch {}
      owners.delete(this.gatt);
      if (this.deviceId && devices.get(this.deviceId) === this)
        devices.delete(this.deviceId);
    }
  }
  close(error: unknown = new Error("BLE_DISCONNECTED")) {
    if (this.closed) return;
    this.closed = true;
    this.error = error instanceof Error ? error : new Error("BLE_IO_FAILED");
    for (const cleanup of this.cleanup) cleanup();
    this.cleanup.clear();
    this.reject(this.error);
    // Retain the admission lease during an unresolved native call.
    if (this.pending && this.gatt && owners.get(this.gatt) === this) {
      try {
        this.gatt.disconnect?.();
      } catch {}
    }
    this.release();
  }
  async wait<T>(
    promise: Promise<T>,
    milliseconds?: number,
    code = "BLE_STEP_TIMEOUT",
  ): Promise<T> {
    void promise.catch(() => {});
    this.guard();
    const remove =
      milliseconds === undefined ? () => {} : this.timer(milliseconds, code);
    try {
      const value = await Promise.race([promise, this.cancelled]);
      this.guard();
      return value;
    } catch (error) {
      this.close(error);
      throw error;
    } finally {
      remove();
    }
  }
  async step<T>(call: () => Promise<T>, milliseconds = 10_000): Promise<T> {
    this.guard();
    this.pending++;
    const raw = Promise.resolve().then(() => {
      this.guard();
      return call();
    });
    const settled = raw.finally(() => {
      this.pending--;
      if (this.closed) this.release();
    });
    // wait attaches rejection handlers immediately, including after cancellation.
    return this.wait(settled, milliseconds);
  }
}
