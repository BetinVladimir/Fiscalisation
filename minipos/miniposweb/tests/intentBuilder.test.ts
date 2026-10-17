import { describe, expect, it } from "vitest";
import { buildIntent, canonical } from "../src/fiscal/intentBuilder";

describe("canonical fiscal intent", () => {
  it("is stable and carries client UUID", async () => {
    const operation = "00000000-0000-4000-8000-000000000010";
    const receipt = "00000000-0000-4000-8000-000000000011";
    const intent = await buildIntent({
      tenantId: "00000000-0000-4000-8000-000000000001",
      locationId: "00000000-0000-4000-8000-000000000002",
      registerId: "00000000-0000-4000-8000-000000000003",
      edgeDeviceId: "00000000-0000-4000-8000-000000000004",
      operatorCode: "A001",
      bindingGeneration: 1,
      command: "PRINTER_TEST",
      operationId: operation,
      receiptSessionId: receipt,
    });
    expect(intent.client_operation_id).toBe(operation);
    expect(intent.intent_id).toBe(operation);
    expect(intent.payload_sha256).toMatch(/^[0-9a-f]{64}$/);
    expect(canonical({ b: 1, a: 2 })).toBe('{"a":2,"b":1}');
  });

  it("keeps an offline surrogate separate from an absent backend sale", async () => {
    const intent = await buildIntent({
      tenantId: "tenant", locationId: "location", registerId: "register",
      edgeDeviceId: "edge", operatorCode: "A001", bindingGeneration: 1,
      command: "SALE_FINALIZE", surrogateId: "local-surrogate",
      operationId: "operation", receiptSessionId: "receipt",
    });
    expect(intent.server_sale_id).toBeNull();
    expect(intent.canonical_payload.server_sale_id).toBeNull();
    expect(intent.client_sale_surrogate_id).toBe("local-surrogate");
    expect(intent.canonical_payload.external_id).toBe("local-surrogate");
    expect(intent.intent_id).toBe("operation");
  });
  it("preserves the backend sale ID independently of its external ID", async () => {
    const intent = await buildIntent({
      tenantId: "tenant", locationId: "location", registerId: "register",
      edgeDeviceId: "edge", operatorCode: "A001", bindingGeneration: 1,
      command: "SALE_FINALIZE", saleId: "backend-sale", surrogateId: "external-sale",
    });
    expect(intent.server_sale_id).toBe("backend-sale");
    expect(intent.canonical_payload.server_sale_id).toBe("backend-sale");
    expect(intent.client_sale_surrogate_id).toBe("external-sale");
  });
});
