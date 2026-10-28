/** Production fiscal requests must never be sent after local auth authority is cleared. */
export function requireFiscalAuthorization(production:boolean,token:string){
 if(production&&!token.trim())throw new Error('AUTH_REQUIRED');
 return token;
}
