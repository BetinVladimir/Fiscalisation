export type CloudState="CLOUD_HEALTHY"|"CLOUD_SUSPECT"|"CLOUD_UNAVAILABLE"|"BLE_ACTIVE"|"CLOUD_RECOVERING";

export class ConnectivityController{
 state:CloudState="CLOUD_HEALTHY";private failures=0;private successes=0;private recoveringSince=0;private readonly now:()=>number;
 constructor(now:()=>number=Date.now){this.now=now}
 observe(ok:boolean,bleReady:boolean){
  if(ok){this.failures=0;if(this.state==="CLOUD_HEALTHY"){return this.state}if(this.state==="CLOUD_SUSPECT"){this.state="CLOUD_HEALTHY";return this.state}if(this.state==="CLOUD_UNAVAILABLE"||this.state==="BLE_ACTIVE"){this.state="CLOUD_RECOVERING";this.successes=1;this.recoveringSince=this.now();return this.state}if(this.state==="CLOUD_RECOVERING"&&++this.successes>=3&&this.now()-this.recoveringSince>=10_000)this.state="CLOUD_HEALTHY";return this.state
  }
  this.successes=0;if(this.state==="CLOUD_RECOVERING"){this.state=bleReady?"BLE_ACTIVE":"CLOUD_UNAVAILABLE";return this.state}if(++this.failures===1){this.state="CLOUD_SUSPECT"}else if(this.failures>=3){this.state=bleReady?"BLE_ACTIVE":"CLOUD_UNAVAILABLE"}return this.state
 }
 useBle(){return this.state==="BLE_ACTIVE"||this.state==="CLOUD_RECOVERING"}
}

export async function pingFiscal(origin:string,timeoutMs=1500,fetcher:typeof fetch=fetch){const c=new AbortController(),timer=setTimeout(()=>c.abort(),timeoutMs);try{const r=await fetcher(origin.replace(/\/public\/v1\/?$/,"")+"/connectivity/ping",{method:"HEAD",cache:"no-store",signal:c.signal});return r.status===204&&r.headers.get("X-BeeFiscal-Ping")==="1"}catch{return false}finally{clearTimeout(timer)}}
