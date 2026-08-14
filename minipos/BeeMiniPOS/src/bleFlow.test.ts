import test from "node:test";
import assert from "node:assert/strict";
import { ackControl, BleMessageAssembler, nackControl } from "./bleFlow.ts";

test("assembler reports gaps, accepts identical duplicate and completes in order", () => {
  const a = new BleMessageAssembler(),
    b = (s: string) => new TextEncoder().encode(s);
  let state = a.accept({
    messageId: "m",
    chunkIndex: 1,
    chunkCount: 3,
    plaintext: b("B"),
  });
  assert.equal(state.highestContiguous, -1);
  assert.equal(state.missingBitmap, "101");
  state = a.accept({
    messageId: "m",
    chunkIndex: 1,
    chunkCount: 3,
    plaintext: b("B"),
  });
  assert.equal(state.missingBitmap, "101");
  state = a.accept({
    messageId: "m",
    chunkIndex: 0,
    chunkCount: 3,
    plaintext: b("A"),
  });
  assert.equal(state.highestContiguous, 1);
  state = a.accept({
    messageId: "m",
    chunkIndex: 2,
    chunkCount: 3,
    plaintext: b("C"),
  });
  assert.equal(new TextDecoder().decode(state.complete), "ABC");
});
test("assembler enforces one in-flight message and duplicate consistency", () => {
  const a = new BleMessageAssembler(),
    v = new Uint8Array([1]);
  a.accept({ messageId: "a", chunkIndex: 0, chunkCount: 2, plaintext: v });
  assert.throws(
    () =>
      a.accept({ messageId: "b", chunkIndex: 0, chunkCount: 1, plaintext: v }),
    /BLE_BUSY/,
  );
  assert.throws(
    () =>
      a.accept({
        messageId: "a",
        chunkIndex: 0,
        chunkCount: 2,
        plaintext: new Uint8Array([2]),
      }),
    /BLE_DUPLICATE_MISMATCH/,
  );
  a.cancel("a");
  assert.doesNotThrow(() =>
    a.accept({ messageId: "b", chunkIndex: 0, chunkCount: 1, plaintext: v }),
  );
});
test("ACK carries contiguous progress and missing bitmap", () => {
  const v = ackControl("session", 2n, {
    messageId: "00112233-4455-6677-8899-aabbccddeeff",
    highestContiguous: 1,
    missingBitmap: "010",
  });
  assert.equal(v.type, "ACK");
  assert.equal(v.counter, 2);
  assert.deepEqual(v.payload, { highest_contiguous: 1, missing_bitmap: "010" });
  assert.throws(
    () =>
      ackControl("", 1n, {
        messageId: "m",
        highestContiguous: 0,
        missingBitmap: "",
      }),
    /BLE_ACK_INVALID/,
  );
});
test("NACK contains only protocol reason", () => {
  const v = nackControl(
    "session",
    3n,
    "00112233-4455-6677-8899-aabbccddeeff",
    "BUSY",
  );
  assert.deepEqual(v.payload, { reason: "BUSY" });
  assert.throws(
    () => nackControl("", 1n, "m", "BAD_CHUNK"),
    /BLE_NACK_INVALID/,
  );
});
