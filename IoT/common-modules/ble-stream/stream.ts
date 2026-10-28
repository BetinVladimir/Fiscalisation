// MP026 Stage B only. Opaque bytes, no TLS/authentication/financial admission.
export const CAP=2048, FRAME=182, FLIGHT=16, MAX=0xffff0000;
export const INIT=0,OPENING=1,OPEN=2,CLOSING=3,CLOSED=4;
const zeros=new Uint8Array(8);
const eq=(a:Uint8Array,b:Uint8Array)=>a.length===b.length&&a.every((v,i)=>v===b[i]);
const r16=(b:Uint8Array,p:number)=>b[p]*256+b[p+1];
const r32=(b:Uint8Array,p:number)=>b[p]*16777216+b[p+1]*65536+b[p+2]*256+b[p+3];
const w16=(b:Uint8Array,p:number,v:number)=>{b[p]=Math.floor(v/256);b[p+1]=v%256;};
const w32=(b:Uint8Array,p:number,v:number)=>{b[p]=Math.floor(v/16777216);b[p+1]=Math.floor(v/65536)%256;b[p+2]=Math.floor(v/256)%256;b[p+3]=v%256;};
const validCap=(c:number)=>Number.isInteger(c)&&c>=20&&c<=FRAME;
const validWindow=(c:number)=>Number.isInteger(c)&&c>=256&&c<=CAP;
export class Stream {
 tx=new Uint8Array(CAP);rx=new Uint8Array(CAP);gen:Uint8Array;
 phase=INIT;error=0;acked=0;dispatched=0;end=0;readOffset=0;next=0;limit=0;peerLimit=0;
 flight:{off:number,len:number}[]=[];selected=0;peerCapacity=0;offer=0;reason=0;
 lastTick=0;local=false;localToken=0;localType=0;localAt=0;due=0;progress=0;blocked=0;ackDue=0;closeAt=0;
 pendingOpen=false;pendingOpenAck=false;pendingAck=false;pendingClose=false;pendingCloseAck=false;
 replay=false;replayAt=0;retries=0;openRetries=0;
 readonly server:boolean;readonly cap:number;readonly capacity:number;readonly start:number;
 constructor(server:boolean,generation:Uint8Array,cap=20,capacity=2048,start=0){this.server=server;this.cap=cap;this.capacity=capacity;this.start=start;this.lastTick=start;this.gen=generation.slice();if(!Number.isSafeInteger(start)||start<0||start>Number.MAX_SAFE_INTEGER-120000){this.fail(-7);return;}if(generation.length!==8||!validCap(cap)||!validWindow(capacity)||(server&&eq(generation,zeros)))this.fail(-1);}
 fail(error:number){this.tx.fill(0);this.rx.fill(0);this.flight=[];this.local=false;this.phase=CLOSED;this.error=error;this.pendingAck=this.pendingOpen=this.pendingOpenAck=this.pendingClose=this.pendingCloseAck=this.replay=false;return error;}
 time(now:number){if(!Number.isSafeInteger(now)||now<this.lastTick||now>Number.MAX_SAFE_INTEGER-120000){this.fail(-7);return false;}this.lastTick=now;return true;}
 header(type:number,offset=0,len=20){const b=new Uint8Array(len);b.set([0x42,0x53,1,type]);b.set(this.gen,4);w32(b,12,offset);return b;}
 info(){if(!this.server||this.phase===CLOSED)return new Uint8Array();const b=this.header(0);w16(b,16,this.cap);w16(b,18,this.capacity);return b;}
 ack(now:number,immediate=false){if(!this.pendingAck)this.ackDue=now+(immediate?0:25);if(immediate)this.ackDue=now;this.pendingAck=true;}
 feed(f:Uint8Array,now:number){
  if(this.phase===CLOSED)return -9;if(!this.time(now))return -7;
  if(f.length>=12&&!(this.phase===INIT&&!this.server)&&!eq(f.subarray(4,12),this.gen))return 0;
  if(f.length<16||f.length>FRAME||f[0]!==0x42||f[1]!==0x53||f[2]!==1||f[3]>6)return this.fail(-2);
  const type=f[3],off=r32(f,12);
  if(type!==3&&f.length!==(type>=5?18:20))return this.fail(-2);
  if(type!==3&&type!==4&&off!==0)return this.fail(-2);
  if(type===0){if(this.server||this.phase!==INIT||eq(f.subarray(4,12),zeros)||!validCap(r16(f,16))||!validWindow(r16(f,18)))return this.fail(-2);this.gen=f.slice(4,12);this.offer=this.cap;this.selected=Math.min(this.cap,r16(f,16));this.peerCapacity=r16(f,18);this.pendingOpen=true;this.phase=OPENING;return 0;}
  if(type===1){const offer=r16(f,16),capacity=r16(f,18);if(!this.server||!validCap(offer)||!validWindow(capacity))return this.fail(-2);if(this.phase===OPEN||this.phase===OPENING){if(this.offer!==offer||this.peerCapacity!==capacity)return this.fail(-2);this.pendingOpenAck=true;return 0;}if(this.phase!==INIT)return this.fail(-2);this.offer=offer;this.selected=Math.min(offer,this.cap);this.peerCapacity=capacity;this.peerLimit=capacity;this.limit=this.capacity;this.phase=OPENING;this.pendingOpenAck=true;return 0;}
  if(type===2){if(this.server||(this.phase!==OPENING&&this.phase!==OPEN)||r16(f,16)!==this.selected||r16(f,18)!==this.peerCapacity)return this.fail(-2);if(this.phase===OPENING){this.phase=OPEN;this.peerLimit=this.peerCapacity;this.limit=this.capacity;this.pendingOpen=false;this.due=0;}return 0;}
  if(type===5){if(r16(f,16)>7)return this.fail(-2);this.reason=r16(f,16);this.phase=CLOSING;this.pendingClose=false;this.pendingCloseAck=true;this.closeAt=now;return 0;}
  if(type===6){if(this.phase!==CLOSING||r16(f,16)!==this.reason)return this.fail(-2);return this.fail(0);}
  if(this.phase!==OPEN)return this.fail(-2);
  if(type===3){const len=f.length-16,end=off+len;if(!len||f.length>this.selected||end>MAX)return this.fail(-3);if(end<=this.next){this.ack(now,true);return 0;}if(off<this.next||end>this.limit)return this.fail(-3);if(off>this.next){this.ack(now,true);return 0;}for(let i=0;i<len;i++)this.rx[(off+i)%CAP]=f[16+i];this.next=end;this.ack(now);return 0;}
  if(type===4){const limit=r32(f,16);if(off<this.acked)return 0;if(off>this.dispatched||limit<off||limit>off+this.peerCapacity||limit>MAX)return this.fail(-4);if(off>this.acked){if(!this.flight.some(d=>d.off+d.len===off))return this.fail(-4);this.flight=this.flight.filter(d=>d.off+d.len>off);for(let p=this.acked;p<off;p++)this.tx[p%CAP]=0;this.acked=off;this.progress=now;this.retries=0;this.due=this.flight.length?now+750:0;}if(limit>this.peerLimit){this.peerLimit=limit;this.blocked=0;}return 0;}
  return this.fail(-2);
 }
 write(p:Uint8Array,now:number){if(this.phase!==OPEN)return 0;if(!Number.isSafeInteger(p.length)||p.length<0){this.fail(-5);return 0;}if(!p.length)return 0;if(!this.time(now))return 0;if(!Number.isSafeInteger(this.end)||this.end<0||this.end>MAX||p.length>MAX-this.end){this.fail(-5);return 0;}const n=Math.min(p.length,CAP-(this.end-this.acked));for(let i=0;i<n;i++)this.tx[(this.end+i)%CAP]=p[i];this.end+=n;return n;}
 read(n:number,now:number){if(this.phase!==OPEN)return new Uint8Array();if(!this.time(now))return new Uint8Array();if(!Number.isSafeInteger(n)||n<0){this.fail(-2);return new Uint8Array();}n=Math.max(0,Math.min(Math.floor(n),this.next-this.readOffset));const b=new Uint8Array(n);for(let i=0;i<n;i++){b[i]=this.rx[(this.readOffset+i)%CAP];this.rx[(this.readOffset+i)%CAP]=0;}this.readOffset+=n;this.limit=Math.min(MAX,this.readOffset+this.capacity);if(n)this.ack(now);return b;}
 take(now:number){
  if(this.phase===CLOSED)return new Uint8Array();if(!this.time(now)||this.local)return new Uint8Array();let out=new Uint8Array(),type=0;
  if(this.pendingClose){type=5;this.pendingClose=false;out=this.header(type,0,18);w16(out,16,this.reason);}
  else if(this.pendingCloseAck){type=6;this.pendingCloseAck=false;out=this.header(type,0,18);w16(out,16,this.reason);}
  else if(this.pendingOpenAck){type=2;this.pendingOpenAck=false;out=this.header(type);w16(out,16,this.selected);w16(out,18,this.capacity);this.phase=OPEN;}
  else if(this.pendingOpen){type=1;this.pendingOpen=false;out=this.header(type);w16(out,16,this.offer);w16(out,18,this.capacity);this.due=now+(this.openRetries===0?750:this.openRetries===1?1500:3000);}
  else if(this.phase===OPEN&&this.pendingAck&&now>=this.ackDue){type=4;this.pendingAck=false;out=this.header(type,this.next);w32(out,16,this.limit);}
  else if(this.phase===OPEN){let d:{off:number,len:number}|undefined;if(this.replay){d=this.flight.find(d=>d.off>=this.replayAt);if(d)this.replayAt=d.off+d.len;else this.replay=false;}
   if(!d&&this.dispatched<this.end&&this.flight.length<FLIGHT&&this.dispatched<this.peerLimit){d={off:this.dispatched,len:Math.min(this.end-this.dispatched,this.selected-16,this.peerLimit-this.dispatched)};this.flight.push(d);this.dispatched+=d.len;if(this.flight.length===1){this.progress=now;this.due=now+750;this.retries=0;}}
   if(d){type=3;out=this.header(type,d.off,16+d.len);for(let i=0;i<d.len;i++)out[16+i]=this.tx[(d.off+i)%CAP];}
  }
  if(out.length){if(this.localToken===0xffffffff){this.fail(-5);return new Uint8Array();}this.localToken++;this.local=true;this.localType=type;this.localAt=now;}return out;
 }
 settle(generation:Uint8Array,token:number,error:number,now:number){if(this.phase===CLOSED||!eq(generation,this.gen)||token!==this.localToken||!this.local)return;if(!this.time(now))return;this.local=false;if(error){this.fail(-6);return;}if(this.localType===6)this.fail(0);}
 tick(now:number){if(this.phase===CLOSED)return;if(!this.time(now))return;if(this.local&&now-this.localAt>=5000){this.fail(-7);return;}if(this.phase===CLOSING){if(now-this.closeAt>=300)this.fail(0);return;}if((this.phase===INIT||this.phase===OPENING)&&now-this.start>=30000){this.fail(-7);return;}if(!this.server&&this.phase===OPENING&&!this.pendingOpen&&this.due&&now>=this.due){if(this.openRetries++>=6){this.fail(-7);return;}this.pendingOpen=true;this.due=now+3000;}
  if(this.phase===OPEN&&this.flight.length){if(now-this.progress>=15000){this.fail(-7);return;}if(this.due&&now>=this.due){if(this.retries++>=6){this.fail(-7);return;}this.replay=true;this.replayAt=this.acked;this.due=now+(this.retries===1?1500:3000);}}
  if(this.phase===OPEN&&this.end>this.dispatched&&this.peerLimit<=this.dispatched){if(!this.blocked)this.blocked=now||1;else if(now>=this.blocked&&now-this.blocked>=15000)this.fail(-7);}else this.blocked=0;
 }
 close(reason:number,now:number){if(this.phase===CLOSED)return;if(!this.time(now))return;if(!Number.isInteger(reason)||reason<0||reason>7){this.fail(-2);return;}this.phase=CLOSING;this.reason=reason;this.pendingClose=true;this.closeAt=now;}
}
