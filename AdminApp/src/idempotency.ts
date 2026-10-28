import * as ExpoCrypto from "expo-crypto";

export function newKey(): string {
  return ExpoCrypto.randomUUID();
}
