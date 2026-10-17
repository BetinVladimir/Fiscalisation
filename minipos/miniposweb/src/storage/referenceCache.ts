const DB = "beeloy-miniposweb-reference",
  STORE = "snapshots";

let dbPromise: Promise<IDBDatabase> | null = null;

function database(): Promise<IDBDatabase> {
  if (!dbPromise) {
    const attempt = new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open(DB, 1);
      request.onupgradeneeded = () => request.result.createObjectStore(STORE);
      request.onsuccess = () => {
        const db = request.result;
        db.addEventListener("close", () => { if(dbPromise===attempt)dbPromise=null; });
        db.addEventListener("versionchange",()=>{db.close();if(dbPromise===attempt)dbPromise=null;});
        resolve(db);
      };
      request.onerror = () => reject(request.error);
    });
    dbPromise=attempt;void attempt.catch(()=>{if(dbPromise===attempt)dbPromise=null;});
  }
  return dbPromise;
}

export async function cachePut<T>(key: string, value: T) {
  const db = await database();
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite");
    tx.objectStore(STORE).put(
      { value, updated_at: new Date().toISOString() },
      key,
    );
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error ?? new Error("REFERENCE_TRANSACTION_ABORTED"));
  });
}
export async function cacheGet<T>(
  key: string,
): Promise<{ value: T; updated_at: string } | undefined> {
  const db = await database();
  return new Promise<{ value: T; updated_at: string } | undefined>(
    (resolve, reject) => {
      const tx=db.transaction(STORE),request=tx.objectStore(STORE).get(key);
      tx.oncomplete = () => resolve(request.result);
      tx.onerror = () => reject(tx.error);
      tx.onabort = () => reject(tx.error ?? new Error("REFERENCE_TRANSACTION_ABORTED"));
      request.onerror = () => reject(request.error);
    },
  );
}
