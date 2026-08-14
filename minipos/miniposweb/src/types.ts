export type Money = { amount: string; currency: "EUR" };
export type SaleItem = {
  item_id: string;
  name: string;
  quantity: string;
  unit_price: Money;
  discount?: Money;
  tax_group: "A" | "B" | "C" | "D";
};
export type SalePayment = {
  payment_id: string;
  type: "CASH" | "CARD";
  amount: Money;
};
export type FiscalCommand =
  | "SALE_FINALIZE"
  | "SALE_REVERSE"
  | "REPORT_X"
  | "REPORT_Z"
  | "CASH_IN"
  | "CASH_OUT"
  | "PRINTER_TEST";
export type FiscalIntent = {
  protocol_version: "1.0";
  intent_id: string;
  action: FiscalCommand;
  client_operation_id: string;
  receipt_session_id: string;
  client_sale_surrogate_id: string;
  server_sale_id: string;
  operator_code: string;
  tenant_id: string;
  location_id: string;
  register_id: string;
  edge_device_id: string;
  binding_generation: number;
  command: FiscalCommand;
  items: SaleItem[];
  payments: SalePayment[];
  metadata: Record<string, unknown>;
  canonical_payload: Record<string, unknown>;
  payload_sha256: string;
};
export type FiscalOperation = {
  operation_id: string;
  state: string;
  certainty: string;
  fiscal_reference?: string;
  terminal_reference?: string;
  error_code?: string;
  updated_at: string;
};
export type RouteKind = "CLOUD" | "LOCAL_HTTP" | "BLE";
