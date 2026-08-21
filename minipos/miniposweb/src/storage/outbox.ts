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
    dbPromise = new Promise((resolve, reject) => {
      const request = indexedDB.open(DB, 1);
      request.onupgradeneeded = () =>
        request.result.createObjectStore(STORE, {
          keyPath: "intent.client_operation_id",
        });
      request.onsuccess = () => {
        const db = request.result;
        db.addEventListener("close", () => { dbPromise = null; });
        resolve(db);
      };
      request.onerror = () => reject(request.error);
    });
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
  });
}
export async function getOutbox(id: string): Promise<OutboxRow | undefined> {
  const db = await database();
  return new Promise<OutboxRow | undefined>((resolve, reject) => {
    const r = db.transaction(STORE).objectStore(STORE).get(id);
    r.onsuccess = () => resolve(r.result);
    r.onerror = () => reject(r.error);
  });
}
export async function listOutbox(): Promise<OutboxRow[]> {
  const db = await database();
  return new Promise<OutboxRow[]>((resolve, reject) => {
    const r = db.transaction(STORE).objectStore(STORE).getAll();
    r.onsuccess = () => resolve(r.result);
    r.onerror = () => reject(r.error);
  });
}
