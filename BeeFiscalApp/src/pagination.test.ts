import assert from "node:assert/strict";
import test from "node:test";
import { collectCursorPages } from "./pagination.ts";

test("collectCursorPages reads subsequent cursor pages", async () => {
  const paths: string[] = [];
  const result = await collectCursorPages("/devices", async (path) => {
    paths.push(path);
    return paths.length === 1 ? { items: [{ id: 1 }], page: { has_more: true, next_cursor: "next" } } : { items: [{ id: 2 }], page: { has_more: false } };
  });
  assert.deepEqual(result, [{ id: 1 }, { id: 2 }]);
  assert.equal(paths[1], "/devices?limit=100&cursor=next");
});

test("collectCursorPages rejects has_more without a cursor", async () => {
  await assert.rejects(collectCursorPages("/devices", async () => ({ items: [], page: { has_more: true } })), /INVALID_PAGINATION_SEQUENCE/);
});
