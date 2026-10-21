import AsyncStorage from "@react-native-async-storage/async-storage";
import { canonical } from "./goCanonical.ts";
import type { CheckoutPlan } from "./checkoutPlan.ts";

export type DurableCheckout = {
  sale_id: string;
  sale_version: number;
  plan: CheckoutPlan;
  created_at: string;
  source_order_id?: string;
  signed_intent?: Record<string, unknown>;
  execution_scope?: string;
  phase?: "AUTHORIZING" | "SIGNED";
  authorization_request_key?: string;
  source_session_hash?: string;
};
export interface CheckoutStore {
  getItem(key: string): Promise<string | null>;
  setItem(key: string, value: string): Promise<void>;
  removeItem(key: string): Promise<void>;
  getAllKeys?(): Promise<readonly string[]>;
}
const prefix = "beeminipos:checkout:v1:";
const writes = new Map<string, Promise<void>>();
export class CheckoutJournal {
  // The journal freezes the exact receipt/payment plan before either REST or
  // BLE is attempted. Corrupt or conflicting recovery data fails closed.
  private readonly store: CheckoutStore;
  constructor(store: CheckoutStore = AsyncStorage) {
    this.store = store;
  }
  async save(v: DurableCheckout) {
    const id=prefix+v.sale_id,previous=writes.get(id)??Promise.resolve();
    const task=previous.catch(()=>{}).then(()=>this.write(v));writes.set(id,task);
    try{await task;}finally{if(writes.get(id)===task)writes.delete(id);}
  }
  private async write(v: DurableCheckout) {
    if (!v.sale_id || !v.plan.client_operation_id)
      throw new Error("CHECKOUT_JOURNAL_INVALID");
    const saved = await this.load(v.sale_id);
    const preparing = saved?.phase === 'AUTHORIZING' && v.phase === 'SIGNED' && !saved.signed_intent && saved.authorization_request_key && saved.authorization_request_key === v.authorization_request_key && saved.source_session_hash === v.source_session_hash && canonical(saved.plan.payments) === canonical(v.plan.payments) && canonical(saved.plan.expected_total) === canonical(v.plan.expected_total);
    if (saved && ((!preparing && canonical(saved.plan) !== canonical(v.plan)) || saved.sale_version !== v.sale_version || saved.source_order_id !== v.source_order_id || (saved.signed_intent && canonical(saved.signed_intent) !== canonical(v.signed_intent)) || saved.authorization_request_key !== v.authorization_request_key || saved.source_session_hash !== v.source_session_hash))
      throw new Error("CHECKOUT_JOURNAL_IMMUTABLE");
    if (v.signed_intent && (v.signed_intent.intent_id !== v.plan.client_operation_id || v.signed_intent.receipt_session_id !== v.plan.receipt_session_id || !v.execution_scope || !v.signed_intent.execution_authorization || !v.signed_intent.client_proof))
      throw new Error("CHECKOUT_SIGNED_INTENT_INVALID");
    await this.store.setItem(prefix + v.sale_id, JSON.stringify(v));
  }
  async load(saleId: string): Promise<DurableCheckout | null> {
    const raw = await this.store.getItem(prefix + saleId);
    if (!raw) return null;
    try {
      const v = JSON.parse(raw) as DurableCheckout;
      if (
        v.sale_id !== saleId ||
        !v.plan?.client_operation_id ||
        !v.plan?.receipt_session_id ||
        !Array.isArray(v.plan?.payments)
      )
        throw new Error();
      return v;
    } catch {
      throw new Error("CHECKOUT_JOURNAL_CORRUPT");
    }
  }
  async pending(): Promise<DurableCheckout[]> {
    if (!this.store.getAllKeys) return [];
    const keys = (await this.store.getAllKeys())
      .filter((key) => key.startsWith(prefix))
      .sort();
    const entries: DurableCheckout[] = [];
    for (const key of keys) {
      const entry = await this.load(key.slice(prefix.length));
      if (entry) entries.push(entry);
    }
    return entries;
  }
  async remove(saleId: string) {
    await this.store.removeItem(prefix + saleId);
  }
}
