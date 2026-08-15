import { createHmac, timingSafeEqual } from "node:crypto";
export function deriveWebhookKey(systemToken: string): Buffer { return createHmac("sha256", systemToken).update("beefiscal-webhook-signing-v1").digest(); }
export function verifyWebhook(body: Buffer, signature: string, key: Buffer, now = Math.floor(Date.now()/1000), tolerance = 300): boolean {
  const fields = Object.fromEntries(signature.split(",").map(v => v.split("=", 2) as [string,string])); const ts = Number(fields.t);
  if (!Number.isFinite(ts) || Math.abs(now-ts)>tolerance || !/^[a-f0-9]{64}$/.test(fields.v1 ?? "")) return false;
  const expected=createHmac("sha256",key).update(`${fields.t}.`).update(body).digest(), supplied=Buffer.from(fields.v1,"hex");
  return supplied.length===expected.length && timingSafeEqual(supplied,expected);
}
