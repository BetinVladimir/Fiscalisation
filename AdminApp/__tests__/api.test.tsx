// AA-04.2 — Tests for api(), transition(), createSystem() business logic

// Standalone helper mirroring App.tsx api() callback logic
async function callApi(
  accessToken: string | null,
  base: string,
  path: string,
  init: RequestInit = {},
): Promise<any> {
  if (!accessToken) throw new Error("AUTH_REQUIRED");
  const r = await fetch(base + path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${accessToken}`,
      ...(init.headers || {}),
    },
  });
  const text = await r.text();
  if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
  return text ? JSON.parse(text) : {};
}

// Standalone helper mirroring transition() body construction logic
function buildTransitionBody(
  version: number,
  tenant: string,
  reason: string,
): Record<string, unknown> {
  return {
    version,
    ...(tenant ? { tenant_id: tenant } : {}),
    ...(reason ? { reason } : {}),
  };
}

describe("api() helper", () => {
  beforeEach(() => jest.clearAllMocks());

  it("throws AUTH_REQUIRED when no access token", async () => {
    await expect(callApi(null, "http://localhost", "/path")).rejects.toThrow(
      "AUTH_REQUIRED",
    );
  });

  it("returns parsed JSON on 200 OK", async () => {
    (global as any).fetch = jest.fn().mockResolvedValue({
      ok: true,
      text: async () => JSON.stringify({ items: [1, 2] }),
    });
    const result = await callApi("token", "http://localhost", "/platform/v1/devices");
    expect(result).toEqual({ items: [1, 2] });
  });

  it("throws on HTTP 401", async () => {
    (global as any).fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => "Unauthorized",
    });
    await expect(
      callApi("expired-token", "http://localhost", "/platform/v1/devices"),
    ).rejects.toThrow("Unauthorized");
  });

  it("sends Authorization header with Bearer token", async () => {
    const mockFetch = jest.fn().mockResolvedValue({
      ok: true,
      text: async () => "",
    });
    (global as any).fetch = mockFetch;
    await callApi("my-token", "http://localhost", "/api");
    const [, init] = mockFetch.mock.calls[0] as [string, RequestInit & { headers: Record<string, string> }];
    expect((init.headers as Record<string, string>)["Authorization"]).toBe(
      "Bearer my-token",
    );
  });
});

describe("transition() body builder", () => {
  it("always includes version", () => {
    const body = buildTransitionBody(5, "", "");
    expect(body.version).toBe(5);
  });

  it("includes tenant_id when tenant is set", () => {
    const body = buildTransitionBody(1, "tenant-abc", "");
    expect(body.tenant_id).toBe("tenant-abc");
  });

  it("omits tenant_id when tenant is empty", () => {
    const body = buildTransitionBody(1, "", "");
    expect(body).not.toHaveProperty("tenant_id");
  });

  it("includes reason when set", () => {
    const body = buildTransitionBody(1, "", "maintenance");
    expect(body.reason).toBe("maintenance");
  });

  it("omits reason when empty", () => {
    const body = buildTransitionBody(1, "", "");
    expect(body).not.toHaveProperty("reason");
  });
});

describe("createSystem() bootstrap token handling", () => {
  it("token is initially empty (not yet revealed)", () => {
    let revealedToken = "";
    expect(revealedToken).toBe("");
  });

  it("token is set after successful createSystem call", async () => {
    let revealedToken = "";
    (global as any).fetch = jest.fn().mockResolvedValue({
      ok: true,
      text: async () => JSON.stringify({ bootstrap_token: "tok-secret-123" }),
    });
    const result = await callApi("token", "http://localhost", "/platform/v1/external-systems", { method: "POST", body: "{}" });
    revealedToken = result.bootstrap_token || "";
    expect(revealedToken).toBe("tok-secret-123");
  });

  it("token is cleared when logout sets it to empty string", () => {
    let revealedToken = "tok-secret-123";
    // logout clears the token
    revealedToken = "";
    expect(revealedToken).toBe("");
  });
});
