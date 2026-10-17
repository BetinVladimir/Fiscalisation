import type { FiscalIntent, FiscalOperation, RouteKind } from "../types";
import { getOutbox, prepareOutbox, claimOutboxAttempt, updateOutboxResult, isProvenTerminal } from "../storage/outbox";
export interface FiscalRoute {
  kind: RouteKind;
  probe(signal: AbortSignal): Promise<boolean>;
  submit(intent: FiscalIntent, signal: AbortSignal): Promise<FiscalOperation>;
  lookup(id: string, signal: AbortSignal): Promise<FiscalOperation>;
}
export class RouteController {
  // Route changes are allowed only before a transport attempt. Once a command
  // may have reached a device, recovery is lookup-only to prevent a duplicate
  // receipt or a second card debit after an ambiguous timeout.
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
    const old = await prepareOutbox({
      intent, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
    });
    if (old.intent.payload_sha256 !== intent.payload_sha256)
      throw new Error("IDEMPOTENCY_PAYLOAD_CONFLICT");
    const uncertain = (result?: FiscalOperation) => !!result && !this.terminal(result);
    if (old.result && !uncertain(old.result)) return old.result;
    // Legacy uncertain rows did not record the original route. They must also
    // recover without submitting, even when lookup fails or returns HTTP 404.
    if (old.attempt || uncertain(old.result)) return this.recover(intent);
    // Probe cloud while serving new operations locally; return only after the
    // configured number of healthy observations. Recovery remains lookup-only.
    if (this.preferred !== "CLOUD") {
      const cloud = this.routes.find(route => route.kind === "CLOUD");
      if (cloud) {
        try { this.observe("CLOUD", await this.timed(this.probeTimeoutMs, signal => cloud.probe(signal))); }
        catch { this.observe("CLOUD", false); }
      }
    }
    for (const route of this.ordered()) {
      let available = false;
      try {
        available = await this.timed(this.probeTimeoutMs, signal => route.probe(signal));
      } catch { this.observe(route.kind, false); }
      if (!available) { this.observe(route.kind, false); continue; }
      // IndexedDB serializes this claim across controllers/tabs. Commit it
      // before invoking submit: a reload at any later point is lookup-only.
      if (!(await claimOutboxAttempt(old, route.kind))) return this.recover(intent);
      let result: FiscalOperation;
      try {
        result = await this.timed(this.operationTimeoutMs, signal => route.submit(intent, signal));
      } catch {
        // The durable attempt remains sufficient if this second write fails.
        const saved = await updateOutboxResult(intent, {
          operation_id: intent.client_operation_id, state: "UNKNOWN", certainty: "UNKNOWN",
          error_code: "FISCAL_SUBMISSION_UNCERTAIN", updated_at: new Date().toISOString(),
        });
        if (this.terminal(saved)) return saved;
        throw new Error("FISCAL_SUBMISSION_UNCERTAIN");
      }
      const saved = await updateOutboxResult(intent, result);
      this.observe(route.kind, true);
      return saved;
    }
    throw new Error("NO_FISCAL_ROUTE");
  }
  private terminal(result: FiscalOperation): boolean {
    return isProvenTerminal(result);
  }
  private async recover(intent: FiscalIntent): Promise<FiscalOperation> {
    const current = await getOutbox(intent.client_operation_id);
    if (current?.result && this.terminal(current.result)) return current.result;
    const routes = current?.attempt
      ? this.routes.filter(route => route.kind === current.attempt!.route)
      : this.ordered();
    for (const route of routes) {
      let result: FiscalOperation;
      try {
        result = await this.timed(this.operationTimeoutMs, signal => route.lookup(intent.client_operation_id, signal));
      } catch { continue; }
      return updateOutboxResult(intent, result);
    }
    throw new Error("FISCAL_SUBMISSION_UNCERTAIN");
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
