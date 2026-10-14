import type { AuthTokens } from '../api/client';
type TokenStorage = Pick<Storage,'getItem'|'setItem'|'removeItem'>;
const accessKey='minipos-access-token',refreshKey='minipos-refresh-token';
/** Preserve the previous pair if a browser rejects one of the writes. */
export function persistAuthTokens(storage:TokenStorage,tokens:AuthTokens):void {
 if (!tokens || typeof tokens.access_token !== 'string' || !tokens.access_token.trim() || typeof tokens.refresh_token !== 'string' || !tokens.refresh_token.trim() || !Number.isFinite(tokens.expires_in) || tokens.expires_in <= 0) throw new Error('AUTH_RESPONSE_INVALID');
 const previousAccess=storage.getItem(accessKey),previousRefresh=storage.getItem(refreshKey);
 try {
  storage.setItem(accessKey,tokens.access_token);
  storage.setItem(refreshKey,tokens.refresh_token);
 } catch {
  let rollbackFailed=false;
  for(const [key,value] of [[accessKey,previousAccess],[refreshKey,previousRefresh]] as const){
   try {if(value===null)storage.removeItem(key);else storage.setItem(key,value);} catch {rollbackFailed=true;}
  }
  // Report failure to the caller; it must not enter the authenticated UI.
  throw new Error(rollbackFailed?'AUTH_STORAGE_ROLLBACK_FAILED':'AUTH_STORAGE_WRITE_FAILED');
 }
}
