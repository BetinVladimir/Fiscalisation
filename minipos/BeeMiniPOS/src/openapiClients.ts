import createClient from "openapi-fetch";
import type { paths as FiscalPaths } from "../../../contracts/generated/openapi-public-v1";
import type { paths as RuntimePaths } from "../../../contracts/generated/openapi-runtime-v1";

export type OpenApiClientOptions = {
  baseUrl: string;
  authorization?: string;
};

const headers = (authorization?: string) => ({
  "X-Api-Version": "2026-08-07",
  ...(authorization ? { Authorization: authorization } : {}),
});

export const createFiscalOpenApiClient = ({
  baseUrl,
  authorization,
}: OpenApiClientOptions) =>
  createClient<FiscalPaths>({
    baseUrl: baseUrl.replace(/\/$/, ""),
    headers: headers(authorization),
  });

export const createRuntimeOpenApiClient = ({
  baseUrl,
  authorization,
}: OpenApiClientOptions) =>
  createClient<RuntimePaths>({
    baseUrl: baseUrl.replace(/\/$/, ""),
    headers: headers(authorization),
  });
