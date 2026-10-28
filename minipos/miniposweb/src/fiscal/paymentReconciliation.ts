import { canonical } from "./goCanonical";
import {
  sha256,
  unbase64url,
  type AuthorizationEnvelope,
} from "./executionAuthorization";
import type { FiscalOperation, RouteKind } from "../types";
export type ReconciliationDecision = {
  operation_id: string;
  payment_id: string;
  observed_state: string;
  terminal_reference?: string;
  rrn?: string;
  auth_code?: string;
  reversal_reference?: string;
  reversal_rrn?: string;
  reversal_auth_code?: string;
  evidence_reference: string;
  evidence_sha256: string;
};
export type ReconciliationCommand = {
  command: "PAYMENT_RECONCILE";
  client_operation_id: string;
  reconciliation_authorization: AuthorizationEnvelope;
};
export type ReconciliationPayment = {
  payment_id: string;
  operation_id: string;
  receipt_id: string;
  state: string;
  amount_minor: string;
  terminal_reference: string;
  rrn: string;
  auth_code: string;
  reversal_reference: string;
  reversal_rrn: string;
  reversal_auth_code: string;
};
export type ReconciliationGrant = {
  kind: "PAYMENT_RECONCILIATION";
  version: 1;
  scope: "payment.reconcile";
  reconciliation_id: string;
  tenant_id: string;
  location_id: string;
  register_id: string;
  edge_device_id: string;
  terminal_id: string;
  binding_sha256: string;
  binding_generation: number;
  actor_id: string;
  evidence_reference: string;
  evidence_sha256: string;
  nbf: number;
  exp: number;
  expected: ReconciliationPayment;
  observed: ReconciliationPayment;
};
export type ReconciliationScope = {
  tenant_id: string;
  actor_id: string;
  register_id: string;
  edge_device_id: string;
  binding_sha256: string;
  binding_generation: number;
  source_session_id: string;
  app_instance_id: string;
};
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
// TLS authenticates this response; firmware independently verifies the root signature.
export async function inspectReconciliation(
  command: ReconciliationCommand,
  scope: ReconciliationScope,
  decision: ReconciliationDecision,
  now = Math.floor(Date.now() / 1000),
  allowExpiredLookup = false,
): Promise<ReconciliationGrant> {
  const e = command.reconciliation_authorization;
  if (
    command.command !== "PAYMENT_RECONCILE" ||
    !uuid.test(command.client_operation_id) ||
    e.algorithm !== "ES256_DER" ||
    !e.key_id ||
    !e.signature ||
    !/^[0-9a-f]{64}$/.test(e.sha256)
  )
    throw new Error("RECONCILIATION_ENVELOPE_INVALID");
  const raw = new TextDecoder("utf-8", { fatal: true }).decode(
    unbase64url(e.payload),
  );
  const g = JSON.parse(raw) as ReconciliationGrant;
  if (
    canonical(g) !== raw ||
    (await sha256(raw)) !== e.sha256 ||
    g.kind !== "PAYMENT_RECONCILIATION" ||
    g.version !== 1 ||
    g.scope !== "payment.reconcile" ||
    g.reconciliation_id !== command.client_operation_id ||
    !Number.isSafeInteger(g.nbf) ||
    !Number.isSafeInteger(g.exp) ||
    g.nbf > now ||
    (!allowExpiredLookup && now >= g.exp) ||
    g.exp <= g.nbf
  )
    throw new Error("RECONCILIATION_AUTHORITY_INVALID");
  for (const field of [
    "tenant_id",
    "actor_id",
    "register_id",
    "edge_device_id",
    "binding_sha256",
    "binding_generation",
  ] as const)
    if (g[field] !== scope[field])
      throw new Error("RECONCILIATION_SCOPE_MISMATCH");
  if (
    !g.terminal_id ||
    !g.location_id ||
    g.evidence_reference !== decision.evidence_reference ||
    g.evidence_sha256 !== decision.evidence_sha256 ||
    !/^[0-9a-f]{64}$/.test(g.evidence_sha256) ||
    g.expected.operation_id !== decision.operation_id ||
    g.expected.payment_id !== decision.payment_id ||
    g.observed.state !== decision.observed_state
  )
    throw new Error("RECONCILIATION_DECISION_MISMATCH");
  for (const row of [g.expected, g.observed]) {
    if (
      !uuid.test(row.payment_id) ||
      !uuid.test(row.operation_id) ||
      !uuid.test(row.receipt_id) ||
      !/^([1-9][0-9]*)$/.test(row.amount_minor) ||
      BigInt(row.amount_minor) > 9223372036854775807n
    )
      throw new Error("RECONCILIATION_PAYMENT_INVALID");
    for (const field of [
      "terminal_reference",
      "rrn",
      "auth_code",
      "reversal_reference",
      "reversal_rrn",
      "reversal_auth_code",
    ] as const)
      if (typeof row[field] !== "string")
        throw new Error("RECONCILIATION_PAYMENT_INVALID");
  }
  for (const field of [
    "payment_id",
    "operation_id",
    "receipt_id",
    "amount_minor",
  ] as const)
    if (g.expected[field] !== g.observed[field])
      throw new Error("RECONCILIATION_PAYMENT_CHANGED");
  for (const field of [
    "terminal_reference",
    "rrn",
    "auth_code",
    "reversal_reference",
    "reversal_rrn",
    "reversal_auth_code",
  ] as const)
    if (decision[field] && g.observed[field] !== decision[field])
      throw new Error("RECONCILIATION_REFERENCE_CHANGED");
  const purchase =
    ["CARD_SENDING", "CARD_UNKNOWN"].includes(g.expected.state) &&
    ["CARD_APPROVED", "CARD_DECLINED", "CARD_NOT_SENT"].includes(
      g.observed.state,
    );
  const reversal =
    ["REVERSAL_SENDING", "REVERSAL_UNKNOWN"].includes(g.expected.state) &&
    ["REVERSED", "REVERSAL_FAILED"].includes(g.observed.state);
  if (!purchase && !reversal)
    throw new Error("RECONCILIATION_TRANSITION_INVALID");
  return g;
}
export interface ReconciliationRoute {
  kind: Exclude<RouteKind, "CLOUD">;
  probe(signal: AbortSignal): Promise<boolean>;
  submit(
    command: ReconciliationCommand,
    signal: AbortSignal,
  ): Promise<FiscalOperation>;
  lookup(id: string, signal: AbortSignal): Promise<FiscalOperation>;
}
type Row = {
  scope: string;
  command: ReconciliationCommand;
  attempt?: ReconciliationRoute["kind"];
  result?: FiscalOperation;
};
function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const r = indexedDB.open("beeloy-minipos-payment-reconciliation", 1);
    r.onupgradeneeded = () => r.result.createObjectStore("commands");
    r.onsuccess = () => resolve(r.result);
    r.onerror = () => reject(r.error);
  });
}
const scopeKey = (scope: ReconciliationScope) => canonical(scope);
async function mutate(
  scope: ReconciliationScope,
  command: ReconciliationCommand,
  change: (old: Row) => Row,
): Promise<Row> {
  const db = await open();
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction("commands", "readwrite"),
        store = tx.objectStore("commands"),
        revocation = store.get("scope:" + scopeKey(scope)),
        r = store.get(command.client_operation_id);
      let value: Row;
      r.onsuccess = () => {
        try {
          if (revocation.result?.revoked)
            throw new Error("RECONCILIATION_SESSION_REVOKED");
          const old = r.result as Row | undefined;
          if (
            old &&
            (old.scope !== scopeKey(scope) ||
              canonical(old.command) !== canonical(command))
          )
            throw new Error("RECONCILIATION_IMMUTABLE_CONFLICT");
          value = change(old ?? { scope: scopeKey(scope), command });
          store.put(value, command.client_operation_id);
        } catch (e) {
          tx.abort();
          reject(e);
        }
      };
      tx.oncomplete = () => resolve(value);
      tx.onerror = () => reject(tx.error);
      tx.onabort = () =>
        reject(tx.error ?? new Error("RECONCILIATION_STORAGE_ABORTED"));
    });
  } finally {
    db.close();
  }
}
export async function pendingReconciliations(
  scope: ReconciliationScope,
): Promise<{ command: ReconciliationCommand; attempted: boolean }[]> {
  const db = await open();
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction("commands"),
        store = tx.objectStore("commands"),
        revocation = store.get("scope:" + scopeKey(scope)),
        r = store.getAll();
      tx.oncomplete = () =>
        resolve(
          revocation.result?.revoked
            ? []
            : (r.result as Row[])
                .filter(
                  (row) =>
                    row.command &&
                    row.scope === scopeKey(scope) &&
                    !confirmedReconciliation(row.result),
                )
                .map((row) => ({
                  command: row.command,
                  attempted: !!row.attempt,
                })),
        );
      tx.onerror = () => reject(tx.error);
    });
  } finally {
    db.close();
  }
}
export const RECONCILIATION_DEADLINES = Object.freeze({
  probe: 3000,
  submit: 15000,
  lookup: 10000,
});
class ReconciliationTimeout extends Error {
  constructor(stage: keyof typeof RECONCILIATION_DEADLINES) {
    super("RECONCILIATION_" + stage.toUpperCase() + "_TIMEOUT");
  }
}
// Race independently of transport cancellation: BLE/native promises may ignore AbortSignal.
// A late settlement cannot change the durable state or trigger another submission.
async function boundedRoute<T>(
  stage: keyof typeof RECONCILIATION_DEADLINES,
  call: (signal: AbortSignal) => Promise<T>,
): Promise<T> {
  const controller = new AbortController();
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await new Promise<T>((resolve, reject) => {
      timer = setTimeout(() => {
        reject(new ReconciliationTimeout(stage));
        controller.abort();
      }, RECONCILIATION_DEADLINES[stage]);
      Promise.resolve()
        .then(() => call(controller.signal))
        .then(resolve, reject);
    });
  } finally {
    if (timer !== undefined) clearTimeout(timer);
  }
}
export async function dispatchReconciliation(
  scope: ReconciliationScope,
  command: ReconciliationCommand,
  routes: ReconciliationRoute[],
  assertActive: () => Promise<void>,
): Promise<FiscalOperation> {
  await assertActive();
  let row = await mutate(scope, command, (old) => old);
  if (confirmedReconciliation(row.result)) return row.result!;
  if (row.attempt) {
    const route = routes.find((r) => r.kind === row.attempt);
    if (!route) throw new Error("RECONCILIATION_LOOKUP_ROUTE_REQUIRED");
    const result = await boundedRoute("lookup", (signal) =>
      route.lookup(command.client_operation_id, signal),
    );
    if (result.operation_id !== command.client_operation_id)
      throw new Error("RECONCILIATION_RESULT_ID_MISMATCH");
    const saved = await mutate(scope, command, (old) =>
      confirmedReconciliation(old.result) ? old : { ...old, result },
    );
    return saved.result!;
  }
  for (const route of routes) {
    try {
      if (!(await boundedRoute("probe", (signal) => route.probe(signal))))
        continue;
    } catch (error) {
      if (error instanceof ReconciliationTimeout) continue;
      throw error;
    }
    await assertActive();
    const raw = JSON.parse(
      new TextDecoder().decode(
        unbase64url(command.reconciliation_authorization.payload),
      ),
    ) as ReconciliationGrant;
    if (Math.floor(Date.now() / 1000) >= raw.exp)
      throw new Error("RECONCILIATION_AUTHORITY_EXPIRED");
    let claimed = false;
    row = await mutate(scope, command, (old) => {
      if (old.attempt) return old;
      claimed = true;
      return { ...old, attempt: route.kind };
    });
    if (!claimed)
      return dispatchReconciliation(scope, command, routes, assertActive);
    try {
      const result = await boundedRoute("submit", (signal) =>
        route.submit(command, signal),
      );
      if (result.operation_id !== command.client_operation_id)
        throw new Error("RECONCILIATION_RESULT_ID_MISMATCH");
      const saved = await mutate(scope, command, (old) =>
        confirmedReconciliation(old.result) ? old : { ...old, result },
      );
      return saved.result!;
    } catch (error) {
      const unknown: FiscalOperation = {
        operation_id: command.client_operation_id,
        state: "UNKNOWN",
        certainty: "UNKNOWN",
        error_code: "RECONCILIATION_SUBMISSION_UNCERTAIN",
        updated_at: new Date().toISOString(),
      };
      const saved = await mutate(scope, command, (old) =>
        confirmedReconciliation(old.result) ? old : { ...old, result: unknown },
      );
      if (confirmedReconciliation(saved.result)) return saved.result!;
      throw error;
    }
  }
  throw new Error("NO_RECONCILIATION_ROUTE");
}

export async function stageReconciliation(
  scope: ReconciliationScope,
  command: ReconciliationCommand,
): Promise<void> {
  await mutate(scope, command, (old) => old);
}

export function confirmedReconciliation(result?: FiscalOperation): boolean {
  return result?.state === "COMMITTED" && result.certainty === "PROVEN_SUCCESS";
}

export async function revokeReconciliationScope(
  scope: ReconciliationScope,
): Promise<void> {
  const db = await open();
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction("commands", "readwrite");
      tx.objectStore("commands").put(
        { revoked: true },
        "scope:" + scopeKey(scope),
      );
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
      tx.onabort = () => reject(tx.error);
    });
  } finally {
    db.close();
  }
}
