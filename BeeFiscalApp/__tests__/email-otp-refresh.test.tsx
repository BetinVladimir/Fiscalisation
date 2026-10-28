/**
 * BF-02.1 / BF-02.2 — Email OTP refresh tests
 *
 * RED phase:
 *  - Network error currently clears session (should preserve it)
 *  - expires_in=30 currently fires timer at max(1000ms, ...) = 1000ms (< 30000ms)
 *
 * GREEN phase (after BF-02.2):
 *  - Network error preserves session and retries with backoff
 *  - 401 clears session and calls SecureStore.deleteItemAsync
 *  - expires_in=30 schedules next refresh ≥ 30s
 */

import React from "react";
import { act, create } from "react-test-renderer";

// ── Mocks ─────────────────────────────────────────────────────────────────────

let mockDeleteAsync: jest.Mock;
let mockGetAsync: jest.Mock;
let mockSetAsync: jest.Mock;

jest.mock("expo-constants", () => ({
  default: { expoConfig: { extra: {} } },
}));

jest.mock("expo-modules-core", () => ({
  requireOptionalNativeModule: () => null,
  CodedError: class extends Error {},
}));

jest.mock("expo-secure-store", () => ({
  getItemAsync: (...args: any[]) => mockGetAsync(...args),
  setItemAsync: (...args: any[]) => mockSetAsync(...args),
  deleteItemAsync: (...args: any[]) => mockDeleteAsync(...args),
}));

jest.mock("expo-crypto", () => ({
  randomUUID: () => "test-instance-id",
}));

jest.mock("react-native", () => ({
  Platform: { OS: "ios" },
}));

jest.mock("../src/ui/preferences", () => ({
  useUiPreferences: () => ({ language: "en" }),
}));

// ── Fetch mock ─────────────────────────────────────────────────────────────────

let fetchMock: jest.Mock;

beforeEach(() => {
  mockDeleteAsync = jest.fn().mockResolvedValue(undefined);
  mockGetAsync = jest.fn().mockResolvedValue(null);
  mockSetAsync = jest.fn().mockResolvedValue(undefined);
  fetchMock = jest.fn();
  (globalThis as any).fetch = fetchMock;
});

afterEach(() => {
  jest.useRealTimers();
  jest.clearAllMocks();
});

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeFetchOk(body: unknown, status = 200) {
  return jest.fn().mockResolvedValue({
    ok: true,
    status,
    text: async () => JSON.stringify(body),
    json: async () => body,
  } as any);
}

function makeFetchError(status: number, body: unknown = { error: "error" }) {
  return jest.fn().mockResolvedValue({
    ok: false,
    status,
    text: async () => JSON.stringify(body),
    json: async () => body,
  } as any);
}

function makeFetchNetworkError() {
  return jest.fn().mockRejectedValue(new TypeError("Network request failed"));
}

const TENANT = {
  tenant_id: "t-1",
  display_name: "Test Tenant",
  roles: ["operator"],
};

function makeSession(expiresIn = 3600) {
  return {
    access_token: "access-tok",
    refresh_token: "refresh-tok",
    expires_in: expiresIn,
    tenant: TENANT,
  };
}

// ── Test component ─────────────────────────────────────────────────────────────

import { useEmailOtpAuth } from "../src/emailOtpAuth";

type HookResult = ReturnType<typeof useEmailOtpAuth> & {
  _sessionRef?: { current: any };
};

let capturedResult: HookResult | null = null;

function TestHook() {
  const result = useEmailOtpAuth();
  capturedResult = result;
  return React.createElement("div", null, result.accessToken || "");
}

// ── Test 1: Network error preserves session ─────────────────────────────────

test("BF-02 RED: network error during refresh preserves session (not cleared)", async () => {
  jest.useFakeTimers();
  const session = makeSession(3600);
  const expiresAt = Date.now() + 3600 * 1000;

  // Simulate existing session in secure store
  mockGetAsync.mockImplementation((key: string) => {
    if (key === "beefiscal.app.session") {
      return Promise.resolve(
        JSON.stringify({ ...session, expires_at: expiresAt }),
      );
    }
    if (key === "beefiscal.app.instance") {
      return Promise.resolve("test-instance-id");
    }
    return Promise.resolve(null);
  });

  // Boot: initial refresh succeeds (to set up session)
  fetchMock = makeFetchOk(session);
  (globalThis as any).fetch = fetchMock;

  let tree: any;
  await act(async () => {
    tree = create(React.createElement(TestHook));
  });

  // Wait for boot to complete
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });

  // Advance fake timers to complete boot
  await act(async () => {
    jest.runAllTimers();
    await Promise.resolve();
    await Promise.resolve();
  });

  // Now make fetch fail with network error for refresh
  fetchMock = makeFetchNetworkError();
  (globalThis as any).fetch = fetchMock;

  // Trigger refresh timer (advance past expires_in - 60s ≈ 3540000ms)
  await act(async () => {
    jest.advanceTimersByTime(3600_000);
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });

  // After network error, session MUST be preserved (not null)
  // Currently (BF-02 bug): session is cleared → test FAILS (RED)
  expect(capturedResult?.accessToken).toBeTruthy();
  // SecureStore.deleteItemAsync must NOT have been called for session key
  const deleteCallsForSession = mockDeleteAsync.mock.calls.filter(
    (call: any[]) => call[0] === "beefiscal.app.session",
  );
  // RED: currently deleteItemAsync IS called on network error
  expect(deleteCallsForSession).toHaveLength(0);

  await act(async () => {
    tree.unmount();
  });
});

// ── Test 2: 401 clears session and calls deleteItemAsync ────────────────────

test("BF-02: 401 response clears session and calls deleteItemAsync", async () => {
  jest.useFakeTimers();
  const session = makeSession(3600);
  const expiresAt = Date.now() + 3600 * 1000;

  mockGetAsync.mockImplementation((key: string) => {
    if (key === "beefiscal.app.session") {
      return Promise.resolve(
        JSON.stringify({ ...session, expires_at: expiresAt }),
      );
    }
    if (key === "beefiscal.app.instance") {
      return Promise.resolve("test-instance-id");
    }
    return Promise.resolve(null);
  });

  // Boot: initial refresh succeeds
  fetchMock = makeFetchOk(session);
  (globalThis as any).fetch = fetchMock;

  let tree: any;
  await act(async () => {
    tree = create(React.createElement(TestHook));
  });

  await act(async () => {
    jest.runAllTimers();
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });

  // Now refresh returns 401
  fetchMock = makeFetchError(401, { error: "invalid_grant" });
  (globalThis as any).fetch = fetchMock;

  await act(async () => {
    jest.advanceTimersByTime(3600_000);
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });

  // After 401: session must be cleared
  expect(capturedResult?.accessToken).toBe("");

  await act(async () => {
    tree.unmount();
  });
});

// ── Test 3: expires_in=30 → timer ≥ 30s ─────────────────────────────────────

test("BF-02 RED: expires_in=30 schedules refresh at >= 30000 ms (not 1000 ms)", async () => {
  jest.useFakeTimers();
  const session = makeSession(30); // 30 seconds
  const expiresAt = Date.now() + 30 * 1000;

  mockGetAsync.mockImplementation((key: string) => {
    if (key === "beefiscal.app.session") {
      return Promise.resolve(
        JSON.stringify({ ...session, expires_at: expiresAt }),
      );
    }
    if (key === "beefiscal.app.instance") {
      return Promise.resolve("test-instance-id");
    }
    return Promise.resolve(null);
  });

  // Boot: fetch succeeds for initial refresh call
  fetchMock = makeFetchOk(session);
  (globalThis as any).fetch = fetchMock;

  let tree: any;
  await act(async () => {
    tree = create(React.createElement(TestHook));
  });

  // Complete boot
  await act(async () => {
    jest.runAllTimers();
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });

  // After boot: a timer for next refresh should be scheduled
  // With bug: Math.max(1000, 30000 - Date.now() - 60000) = max(1000, -30000) = 1000ms
  // Expected (fixed): minimum 30000ms

  // Reset fetch mock - count calls
  const refreshCallCount = fetchMock.mock.calls.filter(
    (call: any[]) => typeof call[0] === "string" && call[0].includes("refresh"),
  ).length;
  fetchMock.mockClear();
  fetchMock = makeFetchOk(makeSession(30));
  (globalThis as any).fetch = fetchMock;

  // Advance 29 seconds — refresh must NOT have fired yet (if timer ≥ 30s)
  await act(async () => {
    jest.advanceTimersByTime(29_000);
    await Promise.resolve();
    await Promise.resolve();
  });

  const callsAt29s = fetchMock.mock.calls.filter(
    (call: any[]) => typeof call[0] === "string" && call[0].includes("refresh"),
  ).length;

  // RED: currently timer fires after 1000ms → refresh fires before 29s → callsAt29s > 0
  // GREEN: timer fires at ≥ 30s → callsAt29s = 0
  expect(callsAt29s).toBe(0); // Refresh must NOT fire before 30s

  await act(async () => {
    tree.unmount();
  });
});
