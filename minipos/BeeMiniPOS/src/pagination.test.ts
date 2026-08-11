import assert from "node:assert/strict";
import test from "node:test";
import { collectCursorPages } from "./pagination.ts";

test("collectCursorPages returns every page and preserves filters", async () => {
  const paths: string[] = [];
  const result = await collectCursorPages<number>("/orders?state=COMPLETED", async (path) => {
    paths.push(path);
    return paths.length === 1
      ? { items: [1, 2], page: { has_more: true, next_cursor: "two words" } }
      : { items: [3], page: { has_more: false, next_cursor: null } };
  });
  assert.deepEqual(result, [1, 2, 3]);
  assert.deepEqual(paths, ["/orders?state=COMPLETED&limit=100", "/orders?state=COMPLETED&limit=100&cursor=two%20words"]);
});

test("collectCursorPages rejects repeated cursors", async () => {
  await assert.rejects(collectCursorPages("/products", async () => ({ items: [], page: { has_more: true, next_cursor: "repeat" } })), /INVALID_PAGINATION_SEQUENCE/);
});
