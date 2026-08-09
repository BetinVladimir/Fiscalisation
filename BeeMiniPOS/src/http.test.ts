import assert from "node:assert/strict";
import test from "node:test";
import { fetchWithTimeout, type FetchLike } from "./http.ts";

test("bounded fetch aborts a hung request", async () => {
  let observedSignal: AbortSignal | undefined;
  const hung: FetchLike = async (_input, init) => {
    observedSignal = init?.signal || undefined;
    await new Promise<void>((_resolve, reject) => {
      observedSignal?.addEventListener("abort", () => reject(observedSignal?.reason), { once: true });
    });
    throw new Error("unreachable");
  };
  await assert.rejects(fetchWithTimeout("https://api.example", {}, 100, hung), /HTTP_TIMEOUT/);
  assert.equal(observedSignal?.aborted, true);
});

test("bounded fetch returns a completed response and validates policy", async () => {
  const response = new Response("ok", { status: 200 });
  const immediate: FetchLike = async () => response;
  assert.equal(await fetchWithTimeout("https://api.example", {}, 100, immediate), response);
  await assert.rejects(fetchWithTimeout("https://api.example", {}, 0, immediate), /HTTP_TIMEOUT_INVALID/);
});
