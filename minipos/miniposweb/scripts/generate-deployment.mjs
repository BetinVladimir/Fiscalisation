import {
  createHash,
  createPrivateKey,
  createPublicKey,
  generateKeyPairSync,
  sign,
  verify,
} from "node:crypto";
import { mkdir, readdir, readFile, stat, writeFile } from "node:fs/promises";
import { join, relative } from "node:path";
const root = process.argv[2] ?? "dist";
async function files(dir) {
  const out = [];
  for (const name of (await readdir(dir)).sort()) {
    const path = join(dir, name),
      info = await stat(path);
    if (info.isDirectory()) {
      if (name !== ".well-known") out.push(...(await files(path)));
    } else {
      const data = await readFile(path),
        p = relative(root, path).replaceAll("\\", "/");
      out.push({
        path: p,
        size: data.length,
        sha256: createHash("sha256").update(data).digest("hex"),
        media_type: p.endsWith(".html")
          ? "text/html"
          : p.endsWith(".css")
            ? "text/css"
            : p.endsWith(".js")
              ? "text/javascript"
              : p.endsWith(".json")
                ? "application/json"
                : "application/octet-stream",
      });
    }
  }
  return out;
}
const inventory = await files(root);
if (!inventory.some((x) => x.path === "index.html"))
  throw new Error("ENTRYPOINT_MISSING");
const build = createHash("sha256")
  .update(JSON.stringify(inventory))
  .digest("hex");
const unsigned = {
  schema_version: 1,
  application_id: "com.beeloy.miniposweb",
  version: process.env.npm_package_version ?? "0.1.0",
  build_id: `sha256:${build}`,
  created_at: new Date().toISOString(),
  minimum_adapter_api: "2026-08-14",
  entrypoint: "index.html",
  files: inventory,
};
let privateKey, publicKey;
const kid = process.env.BEELOY_DEPLOYMENT_KID ?? "minipos-dev-ephemeral";
if (process.env.BEELOY_DEPLOYMENT_PRIVATE_KEY_PEM) {
  privateKey = createPrivateKey(process.env.BEELOY_DEPLOYMENT_PRIVATE_KEY_PEM);
} else {
  if (process.env.APP_ENV === "prod")
    throw new Error("BEELOY_DEPLOYMENT_PRIVATE_KEY_PEM_REQUIRED");
  ({ privateKey, publicKey } = generateKeyPairSync("ed25519"));
  process.stdout.write(
    `DEV_SPA_DEPLOYMENT_PUBLIC_KEY_DER_BASE64=${publicKey.export({ type: "spki", format: "der" }).toString("base64")}\n`,
  );
}
const signature = sign(
  null,
  Buffer.from(JSON.stringify(unsigned)),
  privateKey,
).toString("base64url");
const verifierKey = publicKey ?? createPublicKey(privateKey);
if (!verify(null, Buffer.from(JSON.stringify(unsigned)), verifierKey, Buffer.from(signature, "base64url")))
  throw new Error("DEPLOYMENT_SIGNATURE_SELF_CHECK_FAILED");
const wellKnown = join(root, ".well-known");
await mkdir(wellKnown, { recursive: true });
await writeFile(
  join(wellKnown, "beeloy-pos-deployment.json"),
  JSON.stringify(
    { ...unsigned, signature: { kid, alg: "Ed25519", value: signature } },
    null,
    2,
  ) + "\n",
);
