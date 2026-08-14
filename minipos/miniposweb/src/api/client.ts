import type { FiscalIntent, FiscalOperation, SalePayment } from "../types";
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}
async function json<T>(
  url: string,
  init: RequestInit = {},
  token?: string,
  appInstance?: string,
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (appInstance) headers.set("X-App-Instance-Id", appInstance);
  const response = await fetch(url, { ...init, headers, cache: "no-store" });
  if (!response.ok) throw new ApiError(response.status, await response.text());
  return response.status === 204
    ? (undefined as T)
    : (response.json() as Promise<T>);
}
export type Product = {
  id: string;
  name: string;
  price: { amount: string; currency: "EUR" };
  tax_group: "A" | "B" | "C" | "D";
  status: string;
};
export type Employee = {
  id: string;
  first_name: string;
  last_name: string;
  operator_code: string;
  roles: string[];
};
export type OperatorSession = {
  authenticated: boolean;
  employee: Employee;
  expires_at: string;
};
export type Shift = {
  id: string;
  register_id: string;
  employee_id: string;
  state: string;
  version: number;
};
export type PosConfiguration = {
  location_id: string;
  fiscal_register_id: string;
  fiscal_adapter_id: string;
  binding_generation: number;
  adapter_base_url: string;
  ble_advertising_identity?: string;
  ble_service_uuid?: string;
  ble_command_uuid?: string;
  ble_event_uuid?: string;
};
export class LocalFiscalClient {
  constructor(
    readonly baseUrl: string,
    readonly token: () => string | undefined,
  ) {}
  health(signal?: AbortSignal) {
    return json<{ status: string }>(`${this.baseUrl}/healthz`, { signal });
  }
  ready(signal?: AbortSignal) {
    return json<Record<string, unknown>>(
      `${this.baseUrl}/readyz`,
      { signal },
      this.token(),
    );
  }
  submit(intent: FiscalIntent, signal?: AbortSignal) {
    return json<FiscalOperation>(
      `${this.baseUrl}/intents`,
      {
        method: "POST",
        body: JSON.stringify(intent),
        signal,
        headers: {
          "Idempotency-Key": intent.client_operation_id,
          "X-Beeloy-API-Version": "2026-08-14",
        },
      },
      this.token(),
    );
  }
  operation(id: string, signal?: AbortSignal) {
    return json<FiscalOperation>(
      `${this.baseUrl}/operations/${encodeURIComponent(id)}`,
      { signal },
      this.token(),
    );
  }
}
export class MiniPosClient {
  constructor(
    readonly baseUrl: string,
    readonly token: () => string | undefined,
    readonly appInstance: string,
  ) {}
  session() {
    return json<OperatorSession>(
      `${this.baseUrl}/operator-session`,
      {},
      this.token(),
      this.appInstance,
    );
  }
  configuration() {
    return json<PosConfiguration>(
      `${this.baseUrl}/configuration`,
      {},
      this.token(),
      this.appInstance,
    );
  }
  fiscalRouteHealth(signal?: AbortSignal) {
    return json<{ status: string }>(
      `${this.baseUrl}/fiscal-route-health`,
      { signal },
      this.token(),
      this.appInstance,
    );
  }
  products() {
    return json<{ items: Product[] }>(
      `${this.baseUrl}/products`,
      {},
      this.token(),
      this.appInstance,
    );
  }
  shifts(employee: string, state = "OPEN") {
    return json<{ items: Shift[] }>(
      `${this.baseUrl}/shifts?employee_id=${encodeURIComponent(employee)}&state=${state}`,
      {},
      this.token(),
      this.appInstance,
    );
  }
  openShift(register: string, employee: string) {
    return json<Shift>(
      `${this.baseUrl}/shifts`,
      {
        method: "POST",
        body: JSON.stringify({ register_id: register, employee_id: employee }),
      },
      this.token(),
      this.appInstance,
    );
  }
  createOrder(shiftId: string) {
    return json<{ id: string; external_id: string; version: number }>(
      `${this.baseUrl}/orders`,
      { method: "POST", body: JSON.stringify({ shift_id: shiftId }) },
      this.token(),
      this.appInstance,
    );
  }
  addLine(
    orderId: string,
    version: number,
    line: Record<string, unknown>,
    key: string,
  ) {
    return json<{ id: string; version: number }>(
      `${this.baseUrl}/orders/${orderId}/lines`,
      {
        method: "POST",
        headers: { "Idempotency-Key": key, "If-Match": `\"${version}\"` },
        body: JSON.stringify(line),
      },
      this.token(),
      this.appInstance,
    );
  }
  checkout(
    orderId: string,
    payments: SalePayment[],
    intent: FiscalIntent,
    signal?: AbortSignal,
  ) {
    return json<Record<string, unknown>>(
      `${this.baseUrl}/orders/${orderId}/checkout-batch`,
      {
        method: "POST",
        signal,
        headers: { "Idempotency-Key": intent.client_operation_id },
        body: JSON.stringify({
          payments,
          metadata: { receipt_session_id: intent.receipt_session_id },
        }),
      },
      this.token(),
      this.appInstance,
    ).then(
      (value) =>
        ({
          operation_id: String(
            value.operation_id ?? intent.client_operation_id,
          ),
          state: String(value.state ?? "UNKNOWN"),
          certainty:
            value.state === "FISCALIZED" ? "PROVEN_SUCCESS" : "UNKNOWN",
          fiscal_reference: value.fiscal_reference
            ? String(value.fiscal_reference)
            : undefined,
          updated_at: String(value.updated_at ?? new Date().toISOString()),
        }) satisfies FiscalOperation,
    );
  }
  importOfflineOrder(body: Record<string, unknown>, externalId: string) {
    return json<Record<string, unknown>>(
      `${this.baseUrl}/orders:import-offline`,
      {
        method: "POST",
        headers: { "Idempotency-Key": externalId },
        body: JSON.stringify(body),
      },
      this.token(),
      this.appInstance,
    );
  }
  reverseOrder(orderId: string, reasonCode: string, operationId: string) {
    return json<Record<string, unknown>>(
      `${this.baseUrl}/orders/${encodeURIComponent(orderId)}/reversals`,
      {
        method: "POST",
        headers: { "Idempotency-Key": operationId },
        body: JSON.stringify({ reason_code: reasonCode }),
      },
      this.token(),
      this.appInstance,
    );
  }
  localToken(body: Record<string, unknown>, idempotencyKey: string) {
    return json<{
      access_token: string;
      expires_at: string;
      adapter_base_url: string;
    }>(
      `${this.baseUrl}/fiscal-local-tokens`,
      {
        method: "POST",
        headers: {
          "Idempotency-Key": idempotencyKey,
          "X-Api-Version": "2026-08-07",
        },
        body: JSON.stringify(body),
      },
      this.token(),
      this.appInstance,
    );
  }
}
