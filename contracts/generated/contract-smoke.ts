import type { operations as PublicOperations, paths as PublicPaths } from "./openapi-public-v1";
import type { operations as RuntimeOperations, paths as RuntimePaths } from "./openapi-runtime-v1";
import { createFiscalOpenApiClient, createRuntimeOpenApiClient } from "../../BeeMiniPOS/src/openapiClients";

type Assert<T extends true> = T;
type Has<Key extends PropertyKey, Value> = Key extends keyof Value ? true : false;

type PublicSurface = [
  Assert<Has<"createSale", PublicOperations>>,
  Assert<Has<"createWebhookEndpoint", PublicOperations>>,
  Assert<Has<"uploadEdgeSyncBatch", PublicOperations>>,
  Assert<Has<"/sales", PublicPaths>>,
  Assert<Has<"/minipos/orders/{order_id}/checkout", PublicPaths>>,
];

type RuntimeSurface = [
  Assert<Has<"getMiniPosConfiguration", RuntimeOperations>>,
  Assert<Has<"receiveFiscalWebhookAtMiniPos", RuntimeOperations>>,
  Assert<Has<"executeCanonicalEdgeCommandForHIL", RuntimeOperations>>,
  Assert<Has<"createPeriodizedComplianceExport", RuntimeOperations>>,
  Assert<Has<"getPeriodizedComplianceExport", RuntimeOperations>>,
  Assert<Has<"downloadPeriodizedComplianceExportArtifact", RuntimeOperations>>,
  Assert<Has<"/minipos/configuration", RuntimePaths>>,
  Assert<Has<"/internal/v1/storage", RuntimePaths>>,
  Assert<Has<"/exports/periodized", RuntimePaths>>,
];

export type GeneratedContractSmoke = PublicSurface | RuntimeSurface;

const fiscal = createFiscalOpenApiClient({ baseUrl: "https://fiscal.example.test/public/v1" });
const runtime = createRuntimeOpenApiClient({ baseUrl: "https://pos.example.test/public/v1" });
void fiscal.GET("/version", { params: { header: { "X-Api-Version": "2026-08-07" } } });
void runtime.GET("/minipos/configuration", { params: { header: { "X-Api-Version": "2026-08-07" } } });
void runtime.POST("/exports/periodized", {
  params: {header: {"X-Api-Version": "2026-08-07", "Idempotency-Key": "contract-smoke-export-0001"}},
  body: {type: "SUPTO_18_1", from: "2025-12-31T20:00:00Z", to: "2026-01-01T02:00:00Z", format: "JSON"},
});
