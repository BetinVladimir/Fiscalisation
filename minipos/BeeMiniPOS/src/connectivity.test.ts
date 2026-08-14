import test from "node:test";
import assert from "node:assert/strict";
import { ConnectivityController, pingFiscal } from "./connectivity.ts";
test("one failure does not fail over and recovery uses hysteresis", () => {
  let now = 0;
  const c = new ConnectivityController(() => now);
  assert.equal(c.observe(false, true), "CLOUD_SUSPECT");
  c.observe(false, true);
  assert.equal(c.observe(false, true), "BLE_ACTIVE");
  assert.equal(c.useBle(), true);
  assert.equal(c.observe(true, true), "CLOUD_RECOVERING");
  now = 5000;
  c.observe(true, true);
  assert.equal(c.useBle(), true);
  now = 10000;
  assert.equal(c.observe(true, true), "CLOUD_HEALTHY");
});
test("ping is HEAD against lightweight path", async () => {
  let seen = "";
  const ok = await pingFiscal(
    "https://x/public/v1",
    100,
    async (input, init) => {
      seen = `${init?.method} ${input}`;
      return new Response(null, {
        status: 204,
        headers: { "X-BeeFiscal-Ping": "1" },
      });
    },
  );
  assert.equal(ok, true);
  assert.equal(seen, "HEAD https://x/connectivity/ping");
});
