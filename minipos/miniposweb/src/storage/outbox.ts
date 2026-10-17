import type { FiscalIntent, FiscalOperation, RouteKind } from "../types";
export type OutboxRow = {
  intent: FiscalIntent;
  result?: FiscalOperation;
  attempt?: { route: RouteKind; startedAt: string };
  posSync?: {
    shift_id: string;
    external_id: string;
    lines: Array<Record<string, unknown>>;
    payments: Array<Record<string, unknown>>;
    synced_at?: string;
  };
  createdAt: string;
  updatedAt: string;
};
const DB = "beeloy-miniposweb",
  STORE = "fiscal-outbox";

let dbPromise: Promise<IDBDatabase> | null = null;

function database(): Promise<IDBDatabase> {
  if (!dbPromise) {
    const attempt = new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open(DB, 1);
      request.onupgradeneeded = () =>
        request.result.createObjectStore(STORE, {
          keyPath: "intent.client_operation_id",
        });
      request.onsuccess = () => {
        const db = request.result;
        db.addEventListener("close", () => { if (dbPromise === attempt) dbPromise = null; });
        db.addEventListener("versionchange", () => { db.close(); if (dbPromise === attempt) dbPromise = null; });
        resolve(db);
      };
      request.onerror = () => reject(request.error);
    });
    dbPromise = attempt;
    void attempt.catch(() => { if (dbPromise === attempt) dbPromise = null; });
  }
  return dbPromise;
}

export async function putOutbox(row: OutboxRow) {
  const db = await database();
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite"), store = tx.objectStore(STORE);
    const request = store.get(row.intent.client_operation_id);
    let failure: unknown;
    request.onsuccess = () => {
      try {
        const current: OutboxRow | undefined = request.result;
        if (!current) { store.put(row); return; }
        assertIntent(current, row.intent);
        if (current.attempt && row.attempt && current.attempt.route !== row.attempt.route)
          throw new Error("OUTBOX_ATTEMPT_CONFLICT");
        const result = mergeResult(current.result, row.result);
        store.put({ ...current, ...row, intent: current.intent,
          createdAt: current.createdAt, attempt: current.attempt ?? row.attempt,
          result,
          posSync: row.posSync ? { ...row.posSync,
            ...(current.posSync?.synced_at ? { synced_at: current.posSync.synced_at } : {}),
          } : current.posSync,
        });
      } catch (error) { failure = error; tx.abort(); }
    };
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(failure ?? tx.error);
    tx.onabort = () => reject(failure ?? tx.error ?? new Error("OUTBOX_TRANSACTION_ABORTED"));
  });
}
export async function getOutbox(id: string): Promise<OutboxRow | undefined> {
  const db = await database();
  return new Promise<OutboxRow | undefined>((resolve, reject) => {
    const tx = db.transaction(STORE), r = tx.objectStore(STORE).get(id);
    tx.oncomplete = () => resolve(r.result);
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error ?? new Error("OUTBOX_TRANSACTION_ABORTED"));
    r.onerror = () => reject(r.error);
  });
}
export async function listOutbox(): Promise<OutboxRow[]> {
  const db = await database();
  return new Promise<OutboxRow[]>((resolve, reject) => {
    const tx = db.transaction(STORE), r = tx.objectStore(STORE).getAll();
    tx.oncomplete = () => resolve(r.result);
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error ?? new Error("OUTBOX_TRANSACTION_ABORTED"));
    r.onerror = () => reject(r.error);
  });
}

// Read/write transactions serialize initialization and send claims across tabs.
export async function prepareOutbox(row: OutboxRow): Promise<OutboxRow> {
  const db = await database();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite"), store = tx.objectStore(STORE);
    const request = store.get(row.intent.client_operation_id);
    let result: OutboxRow;
    let failure: unknown;
    request.onsuccess = () => {
      result = request.result ?? row;
      try { assertIntent(result, row.intent); if (!request.result) store.put(row); }
      catch (error) { failure = error; tx.abort(); }
    };
    tx.oncomplete = () => resolve(result);
    tx.onerror = () => reject(failure ?? tx.error);
    tx.onabort = () => reject(failure ?? tx.error ?? new Error("OUTBOX_TRANSACTION_ABORTED"));
  });
}
export async function claimOutboxAttempt(row: OutboxRow, route: RouteKind): Promise<boolean> {
  const db = await database();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite"), store = tx.objectStore(STORE);
    const request = store.get(row.intent.client_operation_id);
    let claimed = false;
    let failure: unknown;
    request.onsuccess = () => {
      const current: OutboxRow = request.result ?? row;
      try { assertIntent(current, row.intent); }
      catch (error) { failure = error; tx.abort(); return; }
      if (current.attempt || current.result) return;
      claimed = true;
      store.put({ ...current, attempt: { route, startedAt: new Date().toISOString() }, updatedAt: new Date().toISOString() });
    };
    tx.oncomplete = () => resolve(claimed);
    tx.onerror = () => reject(failure ?? tx.error);
    tx.onabort = () => reject(failure ?? tx.error ?? new Error("OUTBOX_TRANSACTION_ABORTED"));
  });
}

function assertIntent(current: OutboxRow, intent: FiscalIntent) {
  if (current.intent.payload_sha256 !== intent.payload_sha256) throw new Error("IDEMPOTENCY_PAYLOAD_CONFLICT");
  for (const key of ["tenant_id", "location_id", "register_id", "operator_code", "edge_device_id", "binding_generation"] as const)
    if (current.intent[key] !== intent[key]) throw new Error("OUTBOX_SCOPE_CONFLICT");
  for (const key of ["protocol_version", "intent_id", "action", "command", "receipt_session_id", "server_sale_id", "client_sale_surrogate_id"] as const)
    if (current.intent[key] !== intent[key]) throw new Error("IDEMPOTENCY_ENVELOPE_CONFLICT");
}
// Merge inside one transaction: a slow lookup must not restore an old scope or
// posSync snapshot after another asynchronous operation updates the record.
export async function updateOutboxResult(intent: FiscalIntent, result: FiscalOperation): Promise<FiscalOperation> {
  const db = await database();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite"), store = tx.objectStore(STORE);
    const request = store.get(intent.client_operation_id);
    let failure: unknown;
    let saved: FiscalOperation;
    request.onsuccess = () => {
      try {
        const current: OutboxRow | undefined = request.result;
        if (!current) throw new Error("OUTBOX_OPERATION_MISSING");
        assertIntent(current, intent);
        saved = mergeResult(current.result, result)!;
        store.put({ ...current, result: saved, updatedAt: new Date().toISOString() });
      } catch (error) { failure = error; tx.abort(); }
    };
    tx.oncomplete = () => resolve(saved);
    tx.onerror = () => reject(failure ?? tx.error);
    tx.onabort = () => reject(failure ?? tx.error ?? new Error("OUTBOX_TRANSACTION_ABORTED"));
  });
}

export function isProvenTerminal(result: FiscalOperation): boolean {
  const success = ["COMMITTED", "FISCALIZED", "REVERSED", "PRINTER_TESTED", "CANCELLED"].includes(result.state);
  const failure = ["REJECTED", "FAILED", "COMPENSATED"].includes(result.state);
  return (success && result.certainty === "PROVEN_SUCCESS") ||
    (failure && ["PROVEN_FAILURE", "NOT_SENT"].includes(result.certainty));
}
function mergeResult(current?: FiscalOperation, incoming?: FiscalOperation): FiscalOperation | undefined {
  if (!current || !isProvenTerminal(current)) return incoming ?? current;
  if (!incoming || !isProvenTerminal(incoming)) return current;
  // Timestamps may differ between lookups. Proven outcomes/references may not.
  for (const key of ["operation_id", "state", "certainty", "fiscal_reference", "terminal_reference", "error_code"] as const)
    if (current[key] !== incoming[key]) throw new Error("OUTBOX_TERMINAL_RESULT_CONFLICT");
  return current;
}
