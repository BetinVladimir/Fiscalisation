import type {
  FiscalCommand,
  FiscalIntent,
  SaleItem,
  SalePayment,
} from "../types";
const canonical = (v: unknown): string =>
  v === null
    ? "null"
    : Array.isArray(v)
      ? `[${v.map(canonical).join(",")}]`
      : typeof v === "object"
        ? `{${Object.entries(v as Record<string, unknown>)
            .sort(([a], [b]) => a.localeCompare(b))
            .map(([k, x]) => `${JSON.stringify(k)}:${canonical(x)}`)
            .join(",")}}`
        : JSON.stringify(v);
const hex = (v: ArrayBuffer) =>
  [...new Uint8Array(v)].map((x) => x.toString(16).padStart(2, "0")).join("");
export async function buildIntent(input: {
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
  const operation = input.operationId ?? crypto.randomUUID(),
    receipt = input.receiptSessionId ?? crypto.randomUUID();
  const sale = input.saleId ?? operation,
    surrogate = input.surrogateId ?? sale;
  const items = input.items ?? [], payments = input.payments ?? [];
  const salePayload = {
    currency: "EUR",
    server_sale_id: surrogate,
    external_id: surrogate,
    location_id: input.locationId,
    operator_id: input.operatorCode,
    items: items.map(({ item_id, ...item }) => ({ line_id: item_id, ...item })),
    payments,
    metadata: {},
  };
  const canonical_payload = ["PRINTER_TEST", "REPORT_X", "REPORT_Z"].includes(input.command)
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
  return { ...body, payload_sha256 };
}
export { canonical };
