import { decodeStrict } from "./cbor.ts";

export type BleFiscalResult = {
  operation_id: string;
  state:
    | "READY"
    | "FISCALIZED"
    | "BLOCKED"
    | "FISCAL_RESULT_UNKNOWN"
    | "REJECTED";
  fiscal_reference?: string;
  error_code?: string;
  operation_sequence?: number;
  unp_sequence?: number;
};
type Pending = {
  resolve: (v: BleFiscalResult) => void;
  reject: (e: Error) => void;
  timer: ReturnType<typeof setTimeout>;
};

export class BleResultWaiters {
  private readonly pending = new Map<string, Pending>();
  wait(operationId: string, timeoutMs = 30_000) {
    if (
      !isUUID(operationId) ||
      timeoutMs < 100 ||
      timeoutMs > 120_000 ||
      this.pending.has(operationId)
    )
      throw new Error("BLE_RESULT_WAIT_INVALID");
    return new Promise<BleFiscalResult>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(operationId);
        reject(new Error("BLE_RESULT_TIMEOUT"));
      }, timeoutMs);
      this.pending.set(operationId, { resolve, reject, timer });
    });
  }
  accept(messageId: string, raw: Uint8Array) {
    const pending = this.pending.get(messageId);
    if (!pending) return false;
    try {
      const value = decodeStrict<Partial<BleFiscalResult>>(raw);
      if (value.operation_id !== messageId || !validState(value.state))
        throw new Error("BLE_RESULT_INVALID");
      this.pending.delete(messageId);
      clearTimeout(pending.timer);
      pending.resolve(value as BleFiscalResult);
    } catch (e) {
      this.pending.delete(messageId);
      clearTimeout(pending.timer);
      pending.reject(e instanceof Error ? e : new Error("BLE_RESULT_INVALID"));
    }
    return true;
  }
  reject(operationId: string, error: unknown) {
    const pending = this.pending.get(operationId);
    if (!pending) return;
    this.pending.delete(operationId);
    clearTimeout(pending.timer);
    pending.reject(
      error instanceof Error ? error : new Error("BLE_SEND_FAILED"),
    );
  }
  cancelAll() {
    for (const [id] of this.pending)
      this.reject(id, new Error("BLE_DISCONNECTED"));
  }
}
function validState(v: unknown): v is BleFiscalResult["state"] {
  return (
    v === "READY" ||
    v === "FISCALIZED" ||
    v === "BLOCKED" ||
    v === "FISCAL_RESULT_UNKNOWN" ||
    v === "REJECTED"
  );
}
function isUUID(v: string) {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
    v,
  );
}
