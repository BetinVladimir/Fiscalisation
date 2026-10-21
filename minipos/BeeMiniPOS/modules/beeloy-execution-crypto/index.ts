import { requireNativeModule } from 'expo-modules-core';
export interface NativeExecutionCrypto {
  prepare(scope: string): Promise<string>;
  sign(scope: string, expectedPublicKey: string, messageBase64: string): Promise<string>;
  revoke(scope: string): Promise<void>;
}
// No software-key or WebCrypto fallback: a binary without this native module fails closed.
export default requireNativeModule<NativeExecutionCrypto>('BeeloyExecutionCrypto');
