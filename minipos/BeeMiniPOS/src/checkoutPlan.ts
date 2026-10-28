export type CheckoutPlan = {
  client_operation_id: string;
  receipt_session_id: string;
  payments: unknown[];
  expected_total: { amount: string; currency: "EUR" };
};
export function newCheckoutPlan(
  uuid: () => string,
  payments: unknown[],
  total: number,
): CheckoutPlan {
  if (!payments.length || !Number.isFinite(total) || total <= 0)
    throw new Error("CHECKOUT_PLAN_INVALID");
  return {
    client_operation_id: uuid(),
    receipt_session_id: uuid(),
    payments,
    expected_total: { amount: total.toFixed(2), currency: "EUR" },
  };
}
/** Legacy call sites cannot construct an unsigned monetary command. */
export function bleFinalizeIntent(..._arguments: unknown[]): never {
  throw new Error('EXECUTION_AUTHORIZATION_REQUIRED');
}
