import type {NativeKeyProvider} from './nativeExecutionAuthority.ts';
const unavailable=async():Promise<never>=>{throw Error('NATIVE_EXECUTION_KEY_PROVIDER_REQUIRED');};
const provider:NativeKeyProvider={prepare:unavailable,sign:unavailable,revoke:unavailable};
export default provider;
