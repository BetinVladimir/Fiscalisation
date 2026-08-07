import {gcm} from "@noble/ciphers/aes";
import {x25519} from "@noble/curves/ed25519";
import {hkdf} from "@noble/hashes/hkdf";
import {hmac} from "@noble/hashes/hmac";
import {sha256} from "@noble/hashes/sha256";

export const hash256=(v:Uint8Array)=>sha256(v);
export const hmac256=(key:Uint8Array,v:Uint8Array)=>hmac(sha256,key,v);
export const hkdf256=(ikm:Uint8Array,salt:Uint8Array,info:Uint8Array,length=32)=>hkdf(sha256,ikm,salt,info,length);
let randomSource:(n:number)=>Uint8Array<ArrayBufferLike>=(n:number)=>{const out=new Uint8Array(n);if(!globalThis.crypto?.getRandomValues)throw new Error("SECURE_RANDOM_UNAVAILABLE");return globalThis.crypto.getRandomValues(out)};
export function configureRandomSource(source:(n:number)=>Uint8Array<ArrayBufferLike>){const probe=source(32);if(!(probe instanceof Uint8Array)||probe.length!==32)throw new Error("SECURE_RANDOM_SOURCE_INVALID");randomSource=source}
export const randomBytes=(n:number)=>randomSource(n);
export function x25519Pair(){const privateKey=randomBytes(32);return{privateKey,publicKey:x25519.getPublicKey(privateKey)}}
export const x25519Shared=(privateKey:Uint8Array,publicKey:Uint8Array)=>x25519.getSharedSecret(privateKey,publicKey);
export const aesGcmEncrypt=(key:Uint8Array,nonce:Uint8Array,plain:Uint8Array,aad?:Uint8Array)=>gcm(key,nonce,aad).encrypt(plain);
export const aesGcmDecrypt=(key:Uint8Array,nonce:Uint8Array,sealed:Uint8Array,aad?:Uint8Array)=>gcm(key,nonce,aad).decrypt(sealed);

const alphabet="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
export function base64urlEncode(v:Uint8Array){let out="";for(let i=0;i<v.length;i+=3){const n=(v[i]<<16)|((v[i+1]||0)<<8)|(v[i+2]||0);out+=alphabet[(n>>>18)&63]+alphabet[(n>>>12)&63]+(i+1<v.length?alphabet[(n>>>6)&63]:"=")+(i+2<v.length?alphabet[n&63]:"=")}return out.replaceAll("+","-").replaceAll("/","_").replaceAll("=","")}
export function base64urlDecode(raw:string){const v=raw.replaceAll("-","+").replaceAll("_","/");if(!/^[A-Za-z0-9+/]*$/.test(v)||v.length%4===1)throw new Error("BASE64URL_INVALID");const padded=v+"=".repeat((4-v.length%4)%4),out:number[]=[];for(let i=0;i<padded.length;i+=4){const a=alphabet.indexOf(padded[i]),b=alphabet.indexOf(padded[i+1]),c=padded[i+2]==="="?-1:alphabet.indexOf(padded[i+2]),d=padded[i+3]==="="?-1:alphabet.indexOf(padded[i+3]);if(a<0||b<0||(c<0&&padded[i+2]!=="=")||(d<0&&padded[i+3]!=="="))throw new Error("BASE64URL_INVALID");const n=(a<<18)|(b<<12)|((c<0?0:c)<<6)|(d<0?0:d);out.push((n>>>16)&255);if(c>=0)out.push((n>>>8)&255);if(d>=0)out.push(n&255)}return new Uint8Array(out)}
