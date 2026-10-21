/** A committed session instance owns each request; token strings may repeat. */
export class RequestAuthFence {
 private owner?:{token:string;unauthorized?:()=>void};
 activate(token:string){const owner={token};this.owner=owner;return()=>{if(this.owner===owner)this.owner=undefined;};}
 setUnauthorized(token:string,callback:()=>void){if(this.owner?.token===token)this.owner.unauthorized=callback;}
 revoke(){this.owner=undefined;}
 capture(token:string,headers?:HeadersInit){const owner=this.owner,override=new Headers(headers).get('Authorization');const current=()=>Boolean(owner&&token&&(override===null||override===`Bearer ${token}`)&&this.owner===owner&&owner.token===token);return{current,unauthorized:()=>{if(!current())return false;this.owner=undefined;owner?.unauthorized?.();return true;}};}
}
