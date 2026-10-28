import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { canonical } from "../src/fiscal/goCanonical";

describe("Go canonical wire bytes", () => {
  it("matches actual encoding/json for fiscal payloads and Unicode keys", () => {
    const vectors: unknown[] = [null, true, false, 0, 9007199254740991,
      "<receipt>&\u2028\u2029\n\t\"\\", { z: 1, A: 2, a: 3, _: 4 },
      { "😀": 1, "\ue000": 2, "é": "кафе", "e": "€" },
      { server_sale_id: null, operator_code: "A001", items: [{ name: "Tea <&>", quantity: "1.000", unit_price: { amount: "2.10", currency: "EUR" } }] },
    ];
    for (let i = 0; i < 256; i++) vectors.push({ [`key${i}`]: String.fromCodePoint(32 + i), a: [i, null, "<>&"] });
    const actual = execFileSync("go", ["run", fileURLToPath(new URL("./fixtures/goCanonical.go", import.meta.url))], { input: JSON.stringify(vectors), encoding: "utf8" }).trimEnd().split("\n");
    expect(vectors.map(canonical)).toEqual(actual);
  }, 60000);
  it("rejects values outside the exact fiscal JSON subset", () => {
    for (const value of [undefined, NaN, Infinity, 0.1, -0, 9007199254740992, new Date(), { a: undefined }, "\ud800", "\udc00", [, 1]])
      expect(() => canonical(value)).toThrow();
  });
});
