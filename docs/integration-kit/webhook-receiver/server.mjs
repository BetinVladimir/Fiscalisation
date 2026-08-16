import { createHmac, timingSafeEqual } from "node:crypto";
import { createServer } from "node:http";
const key=Buffer.from(process.env.BEEFISCAL_WEBHOOK_KEY_BASE64||"","base64"), seen=new Set();
if(key.length!==32)throw new Error("BEEFISCAL_WEBHOOK_KEY_BASE64 must decode to 32 bytes");
createServer((req,res)=>{const chunks=[];req.on("data",c=>chunks.push(c));req.on("end",()=>{const body=Buffer.concat(chunks),sig=req.headers["beefiscal-signature"]||"",fields=Object.fromEntries(sig.split(",").map(v=>v.split("=",2))),ts=Number(fields.t),expected=createHmac("sha256",key).update(`${fields.t}.`).update(body).digest(),provided=Buffer.from(fields.v1||"","hex"),event=req.headers["beefiscal-event-id"];
const valid=Number.isFinite(ts)&&Math.abs(Date.now()/1000-ts)<=300&&provided.length===expected.length&&timingSafeEqual(provided,expected);if(!valid){res.writeHead(401).end();return}if(!seen.has(event)){seen.add(event);console.log(JSON.parse(body))}res.writeHead(204).end()})}).listen(Number(process.env.PORT||9090));
