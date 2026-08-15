export type Actor = { type: "USER" | "SERVICE"; id: string; sessionId?: string };
export type AcceptedOperation = { operation_id: string; status: string; status_url: string; accepted_at: string };

export class BeeFiscalIntegrationClient {
  constructor(private readonly baseUrl: string, private readonly token: string) {}
  private async request<T>(path: string, init: RequestInit, key?: string): Promise<T> {
    const headers = new Headers(init.headers); headers.set("Authorization", `Bearer ${this.token}`); headers.set("Content-Type", "application/json");
    if (key) headers.set("Idempotency-Key", key);
    const response = await fetch(`${this.baseUrl.replace(/\/$/, "")}${path}`, { ...init, headers });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw Object.assign(new Error(body?.error?.message ?? `HTTP ${response.status}`), { status: response.status, body });
    return body as T;
  }
  startEnrollment(payload: unknown, key: string) { return this.request<any>("/integration/v1/enrollments", { method: "POST", body: JSON.stringify(payload) }, key); }
  verifyEnrollment(temporaryToken: string, code: string, key: string) { return new BeeFiscalIntegrationClient(this.baseUrl, temporaryToken).request<any>("/integration/v1/enrollments:verify", { method: "POST", body: JSON.stringify({ code }) }, key); }
  mutate(path: string, method: "PUT" | "DELETE", version: number, actor: Actor, key: string, payload?: unknown) {
    const headers: Record<string, string> = { "Source-Version": String(version), "BeeFiscal-Source-Actor-Type": actor.type, "BeeFiscal-Source-Actor-Id": actor.id };
    if (actor.sessionId) headers["BeeFiscal-Source-Actor-Session-Id"] = actor.sessionId;
    return this.request<AcceptedOperation>(`/integration/v1/${path}`, { method, headers, body: method === "PUT" ? JSON.stringify(payload ?? {}) : undefined }, key);
  }
  operation(id: string) { return this.request<any>(`/integration/v1/operations/${encodeURIComponent(id)}`, { method: "GET" }); }
}
