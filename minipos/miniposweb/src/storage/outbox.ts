import type { FiscalIntent, FiscalOperation } from "../types";
export type OutboxRow = {
  intent: FiscalIntent;
  result?: FiscalOperation;
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
  // Persist before transport I/O. The same client operation UUID and payload
  // can then survive reload/failover without creating a duplicate side effect.
  const db = await database();
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite");
    tx.objectStore(STORE).put(row);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error ?? new Error("OUTBOX_TRANSACTION_ABORTED"));
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
