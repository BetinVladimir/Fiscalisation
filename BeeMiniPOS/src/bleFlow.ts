export type PlainFrame={messageId:string;chunkIndex:number;chunkCount:number;plaintext:Uint8Array};
export type FlowState={messageId:string;highestContiguous:number;missingBitmap:string;complete?:Uint8Array};
function equal(a:Uint8Array,b:Uint8Array){if(a.length!==b.length)return false;let d=0;for(let i=0;i<a.length;i++)d|=a[i]^b[i];return d===0}
export class BleMessageAssembler{
 private active?:{id:string;count:number;chunks:Map<number,Uint8Array>};
 accept(v:PlainFrame):FlowState{
  if(v.chunkCount<1||v.chunkIndex<0||v.chunkIndex>=v.chunkCount)throw new Error("BLE_BAD_CHUNK");if(!this.active)this.active={id:v.messageId,count:v.chunkCount,chunks:new Map()};const a=this.active;if(a.id!==v.messageId)throw new Error("BLE_BUSY");if(a.count!==v.chunkCount)throw new Error("BLE_BAD_CHUNK");const previous=a.chunks.get(v.chunkIndex);if(previous&&!equal(previous,v.plaintext))throw new Error("BLE_DUPLICATE_MISMATCH");if(!previous)a.chunks.set(v.chunkIndex,v.plaintext.slice());let highest=-1;while(a.chunks.has(highest+1))highest++;let bits="";for(let i=highest+1;i<a.count;i++)bits+=a.chunks.has(i)?"0":"1";const state:FlowState={messageId:a.id,highestContiguous:highest,missingBitmap:bits};if(a.chunks.size===a.count){const size=Array.from(a.chunks.values()).reduce((n,x)=>n+x.length,0),complete=new Uint8Array(size);let p=0;for(let i=0;i<a.count;i++){const chunk=a.chunks.get(i)!;complete.set(chunk,p);p+=chunk.length}state.complete=complete;this.active=undefined}return state
 }
 cancel(messageId:string){if(this.active?.id===messageId)this.active=undefined}
}

export function ackControl(sessionId:string,counter:bigint,v:FlowState){if(!sessionId||counter<1n||counter>BigInt(Number.MAX_SAFE_INTEGER))throw new Error("BLE_ACK_INVALID");return {type:"ACK" as const,protocol_version:"2026-08-07",session_id:sessionId,counter:Number(counter),message_id:v.messageId,payload:{highest_contiguous:v.highestContiguous,missing_bitmap:v.missingBitmap}}}
export function nackControl(sessionId:string,counter:bigint,messageId:string,reason:"BUSY"|"BAD_CHUNK"){if(!sessionId||!messageId||counter<1n||counter>BigInt(Number.MAX_SAFE_INTEGER))throw new Error("BLE_NACK_INVALID");return {type:"NACK" as const,protocol_version:"2026-08-07",session_id:sessionId,counter:Number(counter),message_id:messageId,payload:{reason}}}
