export type Reachability="READY"|"DEGRADED"|"UNAVAILABLE"|"UNKNOWN";
export type Transport="REST"|"BLE"|"BLOCK";
export type TransportSnapshot={cloudRoute:Reachability;bleAdapter:Reachability;edgeRuntime:Reachability;fiscalDevice:Reachability;bleSessionExpiresAt?:string;observedAt:string};
export type TransportDecision={transport:Transport;reason:string;transportMetrics:{cloudRoute:Reachability;bleAdapter:Reachability;edgeRuntime:Reachability};fiscalDeviceMetrics:{state:Reachability;observedAt:string}};

export function chooseTransport(v:TransportSnapshot,now=new Date()):TransportDecision{
 const metrics={transportMetrics:{cloudRoute:v.cloudRoute,bleAdapter:v.bleAdapter,edgeRuntime:v.edgeRuntime},fiscalDeviceMetrics:{state:v.fiscalDevice,observedAt:v.observedAt}};
 if(v.fiscalDevice!=="READY")return{transport:"BLOCK",reason:"FISCAL_DEVICE_UNREACHABLE",...metrics};
 if(v.cloudRoute==="READY")return{transport:"REST",reason:"CLOUD_ROUTE_READY",...metrics};
 const sessionValid=!!v.bleSessionExpiresAt&&new Date(v.bleSessionExpiresAt).getTime()>now.getTime();
 if(v.bleAdapter==="READY"&&v.edgeRuntime==="READY"&&sessionValid)return{transport:"BLE",reason:"CLOUD_ROUTE_LOST_LOCAL_DEVICE_READY",...metrics};
 return{transport:"BLOCK",reason:sessionValid?"LOCAL_ROUTE_UNAVAILABLE":"BLE_SESSION_EXPIRED",...metrics};
}

export class FiscalTransportController{
 private snapshot:TransportSnapshot;
 constructor(initial:TransportSnapshot){this.snapshot=initial}
 update(patch:Partial<TransportSnapshot>){this.snapshot={...this.snapshot,...patch,observedAt:patch.observedAt||new Date().toISOString()};return this.decision()}
 decision(now=new Date()){return chooseTransport(this.snapshot,now)}
}
