import { newKey } from "./idempotency";

// Mock expo-crypto: delegate randomUUID to Node.js crypto (not a constant)
jest.mock("expo-crypto", () => ({
  randomUUID: jest.fn(() => require("node:crypto").randomUUID()),
}));

const { randomUUID: sdkFn } = jest.requireMock("expo-crypto");

let savedCrypto: any;

beforeEach(() => {
  jest.clearAllMocks();
  // Simulate Expo runtime where global.crypto.randomUUID is not available
  savedCrypto = (global as any).crypto;
  (global as any).crypto = undefined;
});

afterEach(() => {
  (global as any).crypto = savedCrypto;
});

const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

test("newKey returns a UUID v4 string", () => {
  const key = newKey();
  expect(typeof key).toBe("string");
  expect(key).toMatch(UUID_V4);
});

test("newKey returns distinct values on successive calls", () => {
  const k1 = newKey();
  const k2 = newKey();
  expect(k1).not.toBe(k2);
});

test("newKey delegates to expo-crypto randomUUID", () => {
  newKey();
  expect(sdkFn).toHaveBeenCalledTimes(1);
});

test("newKey delegates to expo-crypto randomUUID on each call", () => {
  newKey();
  newKey();
  newKey();
  expect(sdkFn).toHaveBeenCalledTimes(3);
});
