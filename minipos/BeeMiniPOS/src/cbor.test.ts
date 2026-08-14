import assert from "node:assert/strict";
import test from "node:test";
import { encodeCanonical } from "./cbor.ts";
const hex = (v: Uint8Array) => Buffer.from(v).toString("hex");
test("canonical CBOR matches RFC 8949 ordering used by Edge", () => {
  assert.equal(
    hex(encodeCanonical({ z: 2, a: "first" })),
    "a26161656669727374617a02",
  );
  assert.equal(
    hex(encodeCanonical({ a: "first", z: 2 })),
    "a26161656669727374617a02",
  );
});
