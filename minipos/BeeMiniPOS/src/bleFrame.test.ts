import test from "node:test";
import assert from "node:assert/strict";
import { BleFrameSession, chunkPlaintext } from "./bleFrame.ts";

test("client frame matches BLE v1 header, MTU and monotonic counter", async () => {
  const session = await BleFrameSession.client(
      new Uint8Array(32).fill(7),
      "session-1",
    ),
    id = "00112233-4455-6677-8899-aabbccddeeff";
  const frames = [];
  for (const [i, part] of chunkPlaintext(
    new Uint8Array(300).fill(9),
    185,
  ).entries())
    frames.push(await session.seal(id, i, 3, 0, part));
  assert.equal(frames.length, 3);
  const a = new DataView(
      frames[0].buffer,
      frames[0].byteOffset,
      frames[0].byteLength,
    ),
    b = new DataView(
      frames[1].buffer,
      frames[1].byteOffset,
      frames[1].byteLength,
    );
  assert.equal(a.getUint8(0), 0x42);
  assert.equal(a.getUint8(1), 0x46);
  assert.equal(a.getUint8(2), 1);
  assert.equal(a.getBigUint64(20), 1n);
  assert.equal(b.getBigUint64(20), 2n);
  assert.equal(a.getUint16(28), 0);
  assert.equal(a.getUint16(30), 3);
  assert.equal(frames[0].length, 34 + a.getUint16(32) + 16);
});

test("frame validation rejects unsafe metadata", async () => {
  assert.throws(
    () => chunkPlaintext(new Uint8Array([1]), 52),
    /BLE_MTU_TOO_SMALL/,
  );
  const s = await BleFrameSession.client(new Uint8Array(32).fill(1), "s");
  await assert.rejects(
    s.seal("bad-id", 0, 1, 0, new Uint8Array()),
    /BLE_MESSAGE_ID_INVALID/,
  );
  await assert.rejects(
    s.seal("00112233-4455-6677-8899-aabbccddeeff", 1, 1, 0, new Uint8Array()),
    /BLE_CHUNK_INVALID/,
  );
});

test("directional peers decrypt and reject replay/bad tag", async () => {
  const secret = new Uint8Array(32).fill(3),
    client = await BleFrameSession.client(secret, "pair"),
    edge = await BleFrameSession.edge(secret, "pair"),
    id = "00112233-4455-6677-8899-aabbccddeeff";
  const outbound = await client.seal(
      id,
      0,
      1,
      0,
      new TextEncoder().encode("sale"),
    ),
    opened = await edge.open(outbound);
  assert.equal(new TextDecoder().decode(opened.plaintext), "sale");
  await assert.rejects(edge.open(outbound), /BLE_FRAME_REPLAY/);
  const response = await edge.seal(
    id,
    0,
    1,
    1,
    new TextEncoder().encode("receipt"),
  );
  assert.equal(
    new TextDecoder().decode((await client.open(response)).plaintext),
    "receipt",
  );
  const broken = response.slice();
  broken[broken.length - 1] ^= 1;
  const fresh = await BleFrameSession.client(secret, "pair");
  await assert.rejects(fresh.open(broken), /BLE_FRAME_BAD_TAG/);
});
