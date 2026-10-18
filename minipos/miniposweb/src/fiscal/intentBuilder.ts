import type {
  FiscalCommand,
  FiscalIntent,
  SaleItem,
  SalePayment,
} from "../types";
import {signIntentProof,type AuthorizedContext} from "./executionAuthorization";
import { canonical } from "./goCanonical";
const hex = (v: ArrayBuffer) =>
  [...new Uint8Array(v)].map((x) => x.toString(16).padStart(2, "0")).join("");
export async function buildIntent(input: {
  execution?: AuthorizedContext;
  serverPreparedPayload?:Record<string,unknown>;
  tenantId: string;
  locationId: string;
  registerId: string;
  edgeDeviceId: string;
  bindingGeneration: number;
  operatorCode: string;
  saleId?: string;
  surrogateId?: string;
  command: FiscalCommand;
  items?: SaleItem[];
  payments?: SalePayment[];
  metadata?: Record<string, unknown>;
  operationId?: string;
  receiptSessionId?: string;
}): Promise<FiscalIntent> {
  const context=input.execution;
  if(input.command==='SALE_FINALIZE'&&!context)throw new Error('EXECUTION_AUTHORIZATION_REQUIRED');
  if(context&&(context.claims.tenant_id!==input.tenantId||context.claims.location_id!==input.locationId||context.claims.register_id!==input.registerId||context.claims.edge_device_id!==input.edgeDeviceId||context.claims.binding_generation!==input.bindingGeneration||context.claims.operator_code!==input.operatorCode))throw new Error('EXECUTION_AUTHORIZATION_ROUTE_MISMATCH');
  const assigned=context?.claims.unp_authority;
  if(assigned&&((input.saleId??null)!==assigned.server_sale_id||(input.operationId&&input.operationId!==assigned.operation_id)||(input.receiptSessionId&&input.receiptSessionId!==assigned.receipt_session_id)))throw new Error('SERVER_PREPARED_CONTEXT_MISMATCH');
  if(input.saleId&&!assigned&&input.command==='SALE_FINALIZE')throw new Error('SERVER_PREPARED_AUTHORITY_REQUIRED');
  const operation = assigned?.operation_id??input.operationId ?? crypto.randomUUID(),
    receipt = assigned?.receipt_session_id??input.receiptSessionId ?? crypto.randomUUID();
  const sale = input.saleId ?? null,
    surrogate = input.surrogateId ?? input.saleId ?? operation;
  const items = input.items ?? [],
    payments = input.payments ?? [];
  const salePayload = {
    currency: "EUR",
    server_sale_id: sale,
    external_id: surrogate,
    location_id: input.locationId,
    operator_id: context?.claims.operator_id??input.operatorCode,
    operator_code: context?.claims.operator_code??input.operatorCode,
    items: items.map(({ item_id, ...item }) => Object.fromEntries(Object.entries({ line_id: item_id, ...item }).filter(([,v])=>v!==undefined))),
    payments,
    metadata: {},
  };
  const canonical_payload = assigned ? input.serverPreparedPayload??(()=>{throw new Error("SERVER_PREPARED_PAYLOAD_REQUIRED")})() : ["PRINTER_TEST", "REPORT_X", "REPORT_Z"].includes(
    input.command,
  )
    ? { metadata: {} }
    : salePayload;
  const body = {
    protocol_version: "1.0" as const,
    intent_id: operation,
    action: input.command,
    client_operation_id: operation,
    receipt_session_id: receipt,
    client_sale_surrogate_id: surrogate,
    server_sale_id: sale,
    operator_code: input.operatorCode,
    tenant_id: input.tenantId,
    location_id: input.locationId,
    register_id: input.registerId,
    edge_device_id: input.edgeDeviceId,
    binding_generation: input.bindingGeneration,
    command: input.command,
    items,
    payments,
    metadata: input.metadata ?? {},
    canonical_payload,
  };
  const payload_sha256 = hex(
    await crypto.subtle.digest(
      "SHA-256",
      new TextEncoder().encode(canonical(canonical_payload)),
    ),
  );
  if(assigned&&payload_sha256!==assigned.payload_sha256)throw new Error('SERVER_PREPARED_PAYLOAD_MISMATCH');
  if(context){const client_proof=await signIntentProof(context,{intent_id:operation,receipt_session_id:receipt,payload_sha256,server_sale_id:sale,client_sale_surrogate_id:surrogate,command:input.command});return {...body,payload_sha256,execution_authorization:context.envelope,client_proof};}
  return { ...body, payload_sha256 };
}
export { canonical };
