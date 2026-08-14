import type { FiscalIntent, FiscalOperation, RouteKind } from "../types";
import { getOutbox, putOutbox } from "../storage/outbox";
export interface FiscalRoute {
  kind: RouteKind;
  probe(signal: AbortSignal): Promise<boolean>;
  submit(intent: FiscalIntent, signal: AbortSignal): Promise<FiscalOperation>;
  lookup(id: string, signal: AbortSignal): Promise<FiscalOperation>;
}
export class RouteController {
  private preferred: RouteKind = "CLOUD";
  private cloudSuccesses = 0;
  constructor(
    private routes: FiscalRoute[],
    private probeTimeoutMs = 1500,
    private recoveryThreshold = 3,
    private operationTimeoutMs = 120000,
  ) {}
  private ordered() {
    const weight: Record<RouteKind, number> = {
      CLOUD: 0,
      LOCAL_HTTP: 1,
      BLE: 2,
    };
    return [...this.routes].sort((a, b) =>
      a.kind === this.preferred
        ? -1
        : b.kind === this.preferred
          ? 1
          : weight[a.kind] - weight[b.kind],
    );
  }
  private async timed<T>(
    timeout: number,
    run: (signal: AbortSignal) => Promise<T>,
  ) {
    const controller = new AbortController(),
      timer = setTimeout(() => controller.abort(), timeout);
    try {
      return await run(controller.signal);
    } finally {
      clearTimeout(timer);
    }
  }
  async execute(intent: FiscalIntent): Promise<FiscalOperation> {
    const old = await getOutbox(intent.client_operation_id);
    if (old && old.intent.payload_sha256 !== intent.payload_sha256)
      throw new Error("IDEMPOTENCY_PAYLOAD_CONFLICT");
    if (
      old?.result &&
      !["UNKNOWN", "RECOVERY_REQUIRED", "EXECUTING", "RESERVED"].includes(
        old.result.state,
      )
    )
      return old.result;
    await putOutbox(
      old ?? {
        intent,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
    );
    if (this.preferred !== "CLOUD") {
      const cloud = this.routes.find((route) => route.kind === "CLOUD");
      if (cloud)
        try {
          this.observe(
            "CLOUD",
            await this.timed(this.probeTimeoutMs, (signal) =>
              cloud.probe(signal),
            ),
          );
        } catch {
          this.observe("CLOUD", false);
        }
    }
    for (const route of this.ordered()) {
      try {
        if (
          !(await this.timed(this.probeTimeoutMs, (signal) =>
            route.probe(signal),
          ))
        )
          continue;
        const known = await this.timed(this.probeTimeoutMs, (signal) =>
          route.lookup(intent.client_operation_id, signal),
        ).catch(() => undefined);
        const result =
          known ??
          (await this.timed(this.operationTimeoutMs, (signal) =>
            route.submit(intent, signal),
          ));
        await putOutbox({
          ...old,
          intent,
          result,
          createdAt: old?.createdAt ?? new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        });
        this.observe(route.kind, true);
        return result;
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError") {
          const unknown = {
            operation_id: intent.client_operation_id,
            state: "UNKNOWN",
            certainty: "UNKNOWN",
            error_code: "ROUTE_TIMEOUT",
            updated_at: new Date().toISOString(),
          };
          await putOutbox({
            ...old,
            intent,
            result: unknown,
            createdAt: old?.createdAt ?? new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          });
          continue;
        }
        if (
          typeof error === "object" &&
          error !== null &&
          "status" in error &&
          Number((error as { status: number }).status) >= 400 &&
          Number((error as { status: number }).status) < 500
        )
          throw error;
        this.observe(route.kind, false);
      }
    }
    throw new Error("NO_FISCAL_ROUTE");
  }
  private observe(kind: RouteKind, success: boolean) {
    if (kind === "CLOUD" && success) {
      if (++this.cloudSuccesses >= this.recoveryThreshold)
        this.preferred = "CLOUD";
    } else if (kind === "CLOUD") {
      this.cloudSuccesses = 0;
      this.preferred = "LOCAL_HTTP";
    } else if (success) this.preferred = kind;
  }
}
