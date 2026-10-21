import type { NativeBleBootstrapContract } from './nativeBle.ts';
import type { BleSessionPackage } from './webBle.ts';
import { BleResultWaiters } from './bleResult.ts';
import { encodeCanonical } from './cbor.ts';
import { base64urlDecode,base64urlEncode,randomBytes } from './portableCrypto.ts';
import { authorizedAdvertisingIdentity,matchesAdvertisingIdentity } from './bleAdvertising.ts';
import { FrameAssembler,frameMessage,uuidString } from './bleFrames.ts';
export class NativeBleTransport implements NativeBleBootstrapContract {
  private manager: any;
  private android: boolean;
  private generation=0;
  private connecting=false;
  private active?:string;
  private mtu=23;
  private monitor?:{remove():void};
  private disconnect?:{remove():void};
  private cancelScan?:()=>void;
  constructor(manager:any,android=false){this.manager=manager;this.android=android;}
  private deviceId?: string;
  private session?: BleSessionPackage;
  private ready = false;
  private results = new BleResultWaiters();
  private frames = new FrameAssembler();
  get canSendComplianceIntent() {
    return this.ready;
  }
  prepareSessionPublicKey() {
    return base64urlEncode(randomBytes(32));
  }
  resetSession() {
    this.generation++;
    this.ready=false;
    this.cancelScan?.();this.cancelScan=undefined;
    this.monitor?.remove();this.monitor=undefined;this.disconnect?.remove();this.disconnect=undefined;
    this.frames=new FrameAssembler();this.active=undefined;
    this.results.cancelAll();
    if (this.deviceId)
      void this.manager.cancelDeviceConnection(this.deviceId).catch(() => {});
    this.deviceId = undefined;
    this.session = undefined;
    this.ready = false;
    this.results = new BleResultWaiters();
  }
  async connectFromUserGesture(
    session: BleSessionPackage,
    onEvent: (raw: Uint8Array) => void,
  ) {
    if(this.connecting)throw Error('BLE_CONNECT_IN_PROGRESS');
    this.resetSession();this.connecting=true;const generation=this.generation;
    const live=()=>{if(generation!==this.generation)throw Error('BLE_SESSION_CHANGED');};
    try {
    if (session.security_mode !== "OPEN_MVP")
      throw new Error("BLE_SECURITY_MODE_UNSUPPORTED");
    if (new Date(session.expires_at).getTime() <= Date.now())
      throw new Error("BLE_ROUTE_PACKAGE_EXPIRED");
    const advertisingIdentity = authorizedAdvertisingIdentity(
      session.advertising_identity,
      session.edge_id,
    );
    const device = await new Promise<any>((resolve, reject) => {
      let done = false;
      const timer = setTimeout(() => {
        if (done) return;
        done = true;
        this.manager.stopDeviceScan();
        reject(new Error("BLE_SCAN_TIMEOUT"));
      }, 10_000);
      this.cancelScan=()=>{if(done)return;done=true;clearTimeout(timer);this.manager.stopDeviceScan();reject(Error('BLE_SESSION_CHANGED'));};
      this.manager.startDeviceScan(
        [session.service_uuid],
        null,
        (error: Error | null, found: any) => {
          if (done) return;
          if (error) {
            done = true;
            clearTimeout(timer);
            this.manager.stopDeviceScan();
            reject(error);
            return;
          }
          if (found && matchesAdvertisingIdentity(found, advertisingIdentity)) {
            done = true;
            clearTimeout(timer);
            this.manager.stopDeviceScan();
            resolve(found);
          }
        },
      );
    });
    live();this.cancelScan=undefined;this.deviceId=device.id;
    const connected = await device.connect();live();
    await connected.discoverAllServicesAndCharacteristics();live();
    const negotiated=this.android?await connected.requestMTU(185):connected;live();
    this.mtu=Number(negotiated.mtu??connected.mtu??23);
    if(!Number.isInteger(this.mtu)||this.mtu<61||this.mtu>517)throw Error('BLE_MTU_TOO_SMALL');
    this.deviceId = connected.id;
    this.session = session;
    const disconnect=this.manager.onDeviceDisconnected(connected.id,()=>{if(generation===this.generation)this.resetSession();});
    if(generation!==this.generation){disconnect.remove();live();}
    this.disconnect=disconnect;
    const monitor=connected.monitorCharacteristicForService(
      session.service_uuid,
      session.event_characteristic_uuid,
      (error: Error | null, characteristic: {value?:string} | null) => {
        if(generation!==this.generation)return;
        if (error) {this.resetSession();return;}
        if (!characteristic?.value) return;
        let raw:Uint8Array;
        try{if(characteristic.value.length>1024)return;raw=fromStandardBase64(characteristic.value);}catch{return;}
        if(!this.active||raw.length<58||raw.length>this.mtu-3||uuidString(raw.slice(4,20))!==this.active)return;
        void this.frames
          .accept(raw)
          .then((done) => {
            if (!done||generation!==this.generation||done.id!==this.active) return;
            this.results.accept(done.id, done.payload);
            onEvent(done.payload);
          })
          .catch(() => {});
      },
    );
    if(generation!==this.generation){monitor.remove();live();}
    this.monitor=monitor;
    live();this.ready = true;
    return { state: "OPEN_READY" as const, ready: Promise.resolve() };
    } catch(error){if(generation===this.generation)this.resetSession();throw error;}finally{this.connecting=false;}
  }
  async sendComplianceIntentAndWait(
    intent: unknown,
    messageId: string,
    _attMtu = 185,
    timeoutMs = 30_000,
  ) {
    if (!this.ready || !this.session) throw new Error("BLE_NOT_READY");
    if(this.active)throw Error('BLE_OPERATION_IN_PROGRESS');
    if(new Date(this.session.expires_at).getTime()<=Date.now())throw Error('BLE_ROUTE_PACKAGE_EXPIRED');
    const command=intent as {command?:string;intent_id?:string;execution_authorization?:unknown;client_proof?:unknown};
    if(command.intent_id!==messageId||!command.execution_authorization||!command.client_proof)throw Error('EXECUTION_AUTHORIZATION_REQUIRED');
    const generation=this.generation;
    const frames=await frameMessage(encodeCanonical(intent),messageId,Math.min(this.mtu,_attMtu));
    if(generation!==this.generation||!this.ready)throw Error('BLE_SESSION_CHANGED');
    if(this.active)throw Error('BLE_OPERATION_IN_PROGRESS');
    const result=this.results.wait(messageId,timeoutMs);
    this.active=messageId;this.frames=new FrameAssembler();
    let attempted=false;
    // Observe result rejection immediately: a native write may never settle.
    // A rejected/timed-out operation invalidates the generation before a late
    // write can resume and transmit subsequent fragments.
    const writing=(async()=>{
      for(const frame of frames){
        if(generation!==this.generation||!this.ready)throw Error('BLE_SESSION_CHANGED');
        attempted=true;await this.write(this.session!.command_characteristic_uuid,frame);
      }
    })();
    let writeTimer:ReturnType<typeof setTimeout>|undefined;
    const boundedWriting=Promise.race([writing,new Promise<never>((_,reject)=>{writeTimer=setTimeout(()=>reject(Error('BLE_WRITE_TIMEOUT')),timeoutMs);})]);
    try {
      const [,outcome]=await Promise.all([boundedWriting,result]);
      return outcome;
    } catch(error) {
      if(generation===this.generation)this.resetSession();
      if(attempted && !(error instanceof Error && /BLE_DISCONNECTED|BLE_SESSION_CHANGED/.test(error.message)))
        throw Error('BLE_OUTCOME_UNKNOWN_AFTER_SEND');
      throw error;
    } finally {
      clearTimeout(writeTimer);
      if(generation===this.generation){this.active=undefined;this.frames=new FrameAssembler();}
    }
  }
  private async write(characteristic: string, raw: Uint8Array) {
    if (!this.deviceId || !this.session) throw new Error("BLE_NOT_CONNECTED");
    await this.manager.writeCharacteristicWithResponseForDevice(
      this.deviceId,
      this.session.service_uuid,
      characteristic,
      toStandardBase64(raw),
    );
  }
}
function toStandardBase64(v: Uint8Array) {
  const raw = base64urlEncode(v).replaceAll("-", "+").replaceAll("_", "/");
  return raw + "=".repeat((4 - (raw.length % 4)) % 4);
}
function fromStandardBase64(v: string) {
  return base64urlDecode(
    v.replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", ""),
  );
}
