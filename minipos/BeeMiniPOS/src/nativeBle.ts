import type { BleSessionPackage } from "./webBle.ts";
import type { BleFiscalResult } from "./bleResult.ts";
export interface NativeBleBootstrapContract {
  readonly canSendComplianceIntent: boolean;
  prepareSessionPublicKey(): string;
  resetSession(): void;
  connectFromUserGesture(
    session: BleSessionPackage,
    onEvent: (raw: Uint8Array) => void,
  ): Promise<{ state: "OPEN_READY"; ready: Promise<void> }>;
  sendComplianceIntentAndWait(
    intent: unknown,
    messageId: string,
    attMtu?: number,
    timeoutMs?: number,
  ): Promise<BleFiscalResult>;
}
export function createNativeBleBootstrap(): NativeBleBootstrapContract {
  throw new Error("NATIVE_BLE_UNAVAILABLE");
}
export async function requestNativeBlePermissions() {
  return false;
}
