import {Encoder,decode} from "cbor-x";
const encoder=new Encoder({useRecords:false,mapsAsObjects:true,structuredClone:false,variableMapSize:true});
function keyBytes(v:string){return new TextEncoder().encode(v)}
function keyCompare(a:string,b:string){const aa=keyBytes(a),bb=keyBytes(b);if(aa.length!==bb.length)return aa.length-bb.length;for(let i=0;i<aa.length;i++){if(aa[i]!==bb[i])return aa[i]-bb[i]}return 0}
function canonical(v:unknown):unknown{if(Array.isArray(v))return v.map(canonical);if(v&&typeof v==="object"&&!(v instanceof Uint8Array)){const src=v as Record<string,unknown>,out:Record<string,unknown>={};for(const key of Object.keys(src).sort(keyCompare))out[key]=canonical(src[key]);return out}return v}
export function encodeCanonical(v:unknown):Uint8Array{return encoder.encode(canonical(v))}
export function decodeStrict<T>(v:Uint8Array):T{return decode(v) as T}
