#!/usr/bin/env node
import {generateKeyPairSync, createPrivateKey, createPublicKey, sign, verify} from "node:crypto";
import {readFileSync, writeFileSync} from "node:fs";

const [command, ...args] = process.argv.slice(2);

try {
  if (command === "generate" && args.length === 2) {
    const {privateKey, publicKey} = generateKeyPairSync("ed25519");
    writeFileSync(args[0], privateKey.export({type: "pkcs8", format: "pem"}), {mode: 0o600});
    writeFileSync(args[1], publicKey.export({type: "spki", format: "pem"}), {mode: 0o644});
  } else if (command === "public" && args.length === 1) {
    const privateKey = createPrivateKey(readFileSync(args[0]));
    if (privateKey.asymmetricKeyType !== "ed25519") throw new Error("release key must be Ed25519");
    const publicKey = createPublicKey(privateKey);
    process.stdout.write(publicKey.export({type: "spki", format: "pem"}));
  } else if (command === "sign" && args.length === 3) {
    const privateKey = createPrivateKey(readFileSync(args[0]));
    if (privateKey.asymmetricKeyType !== "ed25519") throw new Error("release key must be Ed25519");
    writeFileSync(args[2], sign(null, readFileSync(args[1]), privateKey), {mode: 0o644});
  } else if (command === "verify" && args.length === 3) {
    const publicKey = createPublicKey(readFileSync(args[0]));
    if (publicKey.asymmetricKeyType !== "ed25519") throw new Error("trusted release key must be Ed25519");
    if (!verify(null, readFileSync(args[1]), publicKey, readFileSync(args[2]))) process.exitCode = 1;
  } else {
    throw new Error("usage: release-signature.mjs generate <private> <public> | public <private> | sign <private> <data> <signature> | verify <public> <data> <signature>");
  }
} catch (error) {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}
