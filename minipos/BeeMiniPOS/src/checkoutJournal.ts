import AsyncStorage from "@react-native-async-storage/async-storage";
import type {CheckoutPlan} from "./checkoutPlan.ts";

export type DurableCheckout={sale_id:string;sale_version:number;plan:CheckoutPlan;created_at:string};
export interface CheckoutStore{getItem(key:string):Promise<string|null>;setItem(key:string,value:string):Promise<void>;removeItem(key:string):Promise<void>;getAllKeys?():Promise<readonly string[]>}
const prefix="beeminipos:checkout:v1:";
export class CheckoutJournal{
 private readonly store:CheckoutStore;
 constructor(store:CheckoutStore=AsyncStorage){this.store=store}
 async save(v:DurableCheckout){if(!v.sale_id||!v.plan.client_operation_id)throw new Error("CHECKOUT_JOURNAL_INVALID");await this.store.setItem(prefix+v.sale_id,JSON.stringify(v))}
 async load(saleId:string):Promise<DurableCheckout|null>{const raw=await this.store.getItem(prefix+saleId);if(!raw)return null;try{const v=JSON.parse(raw) as DurableCheckout;if(v.sale_id!==saleId||!v.plan?.client_operation_id||!v.plan?.receipt_session_id||!Array.isArray(v.plan?.payments))throw new Error();return v}catch{throw new Error("CHECKOUT_JOURNAL_CORRUPT")}}
 async pending():Promise<DurableCheckout[]>{if(!this.store.getAllKeys)return[];const keys=(await this.store.getAllKeys()).filter((key)=>key.startsWith(prefix)).sort();const entries:DurableCheckout[]=[];for(const key of keys){const entry=await this.load(key.slice(prefix.length));if(entry)entries.push(entry)}return entries}
 async remove(saleId:string){await this.store.removeItem(prefix+saleId)}
}
