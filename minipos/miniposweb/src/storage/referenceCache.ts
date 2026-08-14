const DB = "beeloy-miniposweb-reference",
  STORE = "snapshots";
function database(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB, 1);
    request.onupgradeneeded = () => request.result.createObjectStore(STORE);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
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
  });
  db.close();
}
export async function cacheGet<T>(
  key: string,
): Promise<{ value: T; updated_at: string } | undefined> {
  const db = await database();
  const value = await new Promise<{ value: T; updated_at: string } | undefined>(
    (resolve, reject) => {
      const request = db.transaction(STORE).objectStore(STORE).get(key);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    },
  );
  db.close();
  return value;
}
