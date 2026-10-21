import { beforeAll, beforeEach, expect, it, vi } from "vitest";
import { IDBFactory } from "fake-indexeddb";
import { webcrypto } from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import {
  dispatchReconciliation,
  inspectReconciliation,
  pendingReconciliations,
  stageReconciliation,
  revokeReconciliationScope,
  type ReconciliationCommand,
  type ReconciliationDecision,
  type ReconciliationScope,
  type ReconciliationRoute,
} from "../src/fiscal/paymentReconciliation";
let fixture: {
  command: ReconciliationCommand;
  decision: ReconciliationDecision;
  scope: ReconciliationScope;
  grant: Record<string, unknown>;
};
beforeAll(() => {
  const output = path.join(
    fs.mkdtempSync(path.join(os.tmpdir(), "reconciliation-ui-")),
    "fixture.json",
  );
  execFileSync(
    "go",
    [
      "test",
      "./internal/domain",
      "-run",
      "^TestDelegatedReconciliationExplicitAdminCapabilityAndScope$",
      "-count=1",
    ],
    {
      cwd: path.resolve("../../fiscal-backend"),
      env: {
        ...process.env,
        GOCACHE:
          process.env.GOCACHE ??
          path.join(os.tmpdir(), "fiscal-authority-oracle-go-cache"),
        RECONCILIATION_UI_FIXTURE: output,
      },
    },
  );
  fixture = JSON.parse(fs.readFileSync(output, "utf8"));
}, 30000);
beforeEach(() => {
  vi.stubGlobal("indexedDB", new IDBFactory());
  vi.stubGlobal("crypto", webcrypto);
});
const done = (id: string) => ({
  operation_id: id,
  state: "COMMITTED",
  certainty: "PROVEN_SUCCESS",
  updated_at: new Date().toISOString(),
});
it("inspects exact real root-issued scope and decision without projecting fiscal success", async () => {
  const grant = await inspectReconciliation(
    fixture.command,
    fixture.scope,
    fixture.decision,
  );
  expect(grant.actor_id).toMatch(/^integration:/);
  expect(grant.expected.state).toBe("CARD_UNKNOWN");
  expect(grant.observed.state).toBe("CARD_APPROVED");
  expect(grant.expected.amount_minor).toBe("250");
  await expect(
    inspectReconciliation(
      fixture.command,
      { ...fixture.scope, tenant_id: "foreign" },
      fixture.decision,
    ),
  ).rejects.toThrow("SCOPE");
  await expect(
    inspectReconciliation(fixture.command, fixture.scope, {
      ...fixture.decision,
      evidence_reference: "other",
    }),
  ).rejects.toThrow("DECISION");
  await expect(
    inspectReconciliation(
      fixture.command,
      fixture.scope,
      fixture.decision,
      Number(fixture.grant.exp),
    ),
  ).rejects.toThrow("AUTHORITY");
});
it("uncertain submission survives restart and only looks up the identical operation and route", async () => {
  const submitted = vi.fn(async () => {
      throw new Error("lost reply");
    }),
    looked = vi.fn(async (id: string) => done(id));
  const route: ReconciliationRoute = {
    kind: "LOCAL_HTTP",
    probe: async () => true,
    submit: submitted,
    lookup: looked,
  };
  await stageReconciliation(fixture.scope, fixture.command);
  await expect(
    dispatchReconciliation(
      fixture.scope,
      fixture.command,
      [route],
      async () => {},
    ),
  ).rejects.toThrow("lost reply");
  expect(await pendingReconciliations(fixture.scope)).toEqual([
    { command: fixture.command, attempted: true },
  ]);
  const result = await dispatchReconciliation(
    fixture.scope,
    fixture.command,
    [route],
    async () => {},
  );
  expect(result.state).toBe("COMMITTED");
  expect(submitted).toHaveBeenCalledTimes(1);
  expect(looked).toHaveBeenCalledWith(
    fixture.command.client_operation_id,
    expect.any(AbortSignal),
  );
  expect(await pendingReconciliations(fixture.scope)).toEqual([]);
  await dispatchReconciliation(
    fixture.scope,
    fixture.command,
    [route],
    async () => {},
  );
  expect(submitted).toHaveBeenCalledTimes(1);
});
it("fences concurrent tabs and prevents a late lookup from downgrading durable COMMITTED", async () => {
  let release!: (value: ReturnType<typeof done>) => void;
  let releaseLookup!: (value: ReturnType<typeof done>) => void;
  const submit = vi.fn(
    () => new Promise<ReturnType<typeof done>>((r) => (release = r)),
  );
  const lookup = vi.fn(
    () => new Promise<ReturnType<typeof done>>((r) => (releaseLookup = r)),
  );
  const route: ReconciliationRoute = {
    kind: "LOCAL_HTTP",
    probe: async () => true,
    submit,
    lookup,
  };
  const a = dispatchReconciliation(
    fixture.scope,
    fixture.command,
    [route],
    async () => {},
  );
  await vi.waitFor(() => expect(submit).toHaveBeenCalledTimes(1));
  const b = dispatchReconciliation(
    fixture.scope,
    fixture.command,
    [route],
    async () => {},
  );
  await vi.waitFor(() => expect(lookup).toHaveBeenCalledTimes(1));
  release(done(fixture.command.client_operation_id));
  await a;
  releaseLookup({
    ...done(fixture.command.client_operation_id),
    state: "UNKNOWN",
    certainty: "UNKNOWN",
  });
  await b;
  expect(await pendingReconciliations(fixture.scope)).toEqual([]);
  expect(submit).toHaveBeenCalledTimes(1);
});
it("rejects immutable envelope replacement, foreign session and revoked caller before IO", async () => {
  await stageReconciliation(fixture.scope, fixture.command);
  const submit = vi.fn(async (c: ReconciliationCommand) =>
    done(c.client_operation_id),
  );
  const route: ReconciliationRoute = {
    kind: "BLE",
    probe: async () => true,
    submit,
    lookup: async (id) => done(id),
  };
  await expect(
    dispatchReconciliation(
      { ...fixture.scope, source_session_id: "foreign" },
      fixture.command,
      [route],
      async () => {},
    ),
  ).rejects.toThrow("IMMUTABLE");
  await expect(
    dispatchReconciliation(
      fixture.scope,
      {
        ...fixture.command,
        reconciliation_authorization: {
          ...fixture.command.reconciliation_authorization,
          signature: "changed",
        },
      },
      [route],
      async () => {},
    ),
  ).rejects.toThrow("IMMUTABLE");
  await expect(
    dispatchReconciliation(
      fixture.scope,
      fixture.command,
      [route],
      async () => {
        throw new Error("revoked");
      },
    ),
  ).rejects.toThrow("revoked");
  expect(submit).not.toHaveBeenCalled();
});

it("durably revokes cached administrator grants on authoritative denial", async () => {
  await stageReconciliation(fixture.scope, fixture.command);
  await revokeReconciliationScope(fixture.scope);
  expect(await pendingReconciliations(fixture.scope)).toEqual([]);
  const submit = vi.fn();
  await expect(
    dispatchReconciliation(
      fixture.scope,
      fixture.command,
      [
        {
          kind: "LOCAL_HTTP",
          probe: async () => true,
          submit,
          lookup: async (id) => done(id),
        },
      ],
      async () => {},
    ),
  ).rejects.toThrow("REVOKED");
  expect(submit).not.toHaveBeenCalled();
});

it("bounds a non-cancellable probe and selects another route before any write", async () => {
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  try {
    let began!: () => void;
    const started = new Promise<void>((r) => (began = r));
    let signal: AbortSignal | undefined;
    const firstSubmit = vi.fn(),
      secondSubmit = vi.fn(async (c: ReconciliationCommand) =>
        done(c.client_operation_id),
      );
    const first: ReconciliationRoute = {
      kind: "LOCAL_HTTP",
      probe: (s) => {
        signal = s;
        began();
        return new Promise(() => {});
      },
      submit: firstSubmit,
      lookup: async (id) => done(id),
    };
    const second: ReconciliationRoute = {
      kind: "BLE",
      probe: async () => true,
      submit: secondSubmit,
      lookup: async (id) => done(id),
    };
    const result = dispatchReconciliation(
      fixture.scope,
      fixture.command,
      [first, second],
      async () => {},
    );
    await started;
    await vi.advanceTimersByTimeAsync(3000);
    expect((await result).state).toBe("COMMITTED");
    expect(signal?.aborted).toBe(true);
    expect(firstSubmit).not.toHaveBeenCalled();
    expect(secondSubmit).toHaveBeenCalledTimes(1);
  } finally {
    vi.useRealTimers();
  }
});

it("times out uncertain submission and lookup, ignores late success and never resends", async () => {
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  try {
    let began!: () => void;
    const started = new Promise<void>((r) => (began = r));
    let complete!: (v: ReturnType<typeof done>) => void;
    let submitSignal: AbortSignal | undefined;
    const submit = vi.fn((c: ReconciliationCommand, s: AbortSignal) => {
      submitSignal = s;
      began();
      return new Promise<ReturnType<typeof done>>((r) => (complete = r));
    });
    const lookup = vi.fn(async (id: string) => done(id));
    const route: ReconciliationRoute = {
      kind: "LOCAL_HTTP",
      probe: async () => true,
      submit,
      lookup,
    };
    const first = dispatchReconciliation(
      fixture.scope,
      fixture.command,
      [route],
      async () => {},
    );
    const failed = expect(first).rejects.toThrow("SUBMIT_TIMEOUT");
    await started;
    await vi.advanceTimersByTimeAsync(15000);
    await failed;
    expect(submitSignal?.aborted).toBe(true);
    complete(done(fixture.command.client_operation_id));
    await Promise.resolve();
    expect(await pendingReconciliations(fixture.scope)).toHaveLength(1);
    let searching!: () => void;
    const searchStarted = new Promise<void>((r) => (searching = r));
    let lookupSignal: AbortSignal | undefined;
    route.lookup = vi.fn((id, s) => {
      lookupSignal = s;
      searching();
      return new Promise<ReturnType<typeof done>>(() => {});
    });
    const second = dispatchReconciliation(
      fixture.scope,
      fixture.command,
      [route],
      async () => {},
    );
    const timeout = expect(second).rejects.toThrow("LOOKUP_TIMEOUT");
    await searchStarted;
    await vi.advanceTimersByTimeAsync(10000);
    await timeout;
    expect(lookupSignal?.aborted).toBe(true);
    expect(await pendingReconciliations(fixture.scope)).toHaveLength(1);
    route.lookup = lookup;
    expect(
      (
        await dispatchReconciliation(
          fixture.scope,
          fixture.command,
          [route],
          async () => {},
        )
      ).state,
    ).toBe("COMMITTED");
    expect(lookup).toHaveBeenCalledWith(
      fixture.command.client_operation_id,
      expect.any(AbortSignal),
    );
    expect(submit).toHaveBeenCalledTimes(1);
    expect(await pendingReconciliations(fixture.scope)).toEqual([]);
  } finally {
    vi.useRealTimers();
  }
});
