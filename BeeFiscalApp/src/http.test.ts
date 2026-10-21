import assert from "node:assert/strict";
import test from "node:test";
import { fetchWithTimeout, fetchTextWithTimeout, type FetchLike } from "./http.ts";

const url = "https://api.example";
test("noncooperative fetch cannot retain the caller beyond deadline", {timeout:1500}, async () => {
 let signal: AbortSignal | undefined;
 await assert.rejects(fetchWithTimeout(url, {}, 100, async (_url, init) => {
   signal=init?.signal??undefined; return new Promise<Response>(()=>{});
 }), /HTTP_TIMEOUT/);
 assert.equal(signal?.aborted,true);
});
test("body stall is bounded even after headers arrive", {timeout:1500}, async () => {
 let signal: AbortSignal | undefined;
 const response={text:()=>new Promise<string>(()=>{})} as Response;
 await assert.rejects(fetchTextWithTimeout(url,{},100,async(_url,init)=>{signal=init?.signal??undefined;return response;}),/HTTP_TIMEOUT/);
 assert.equal(signal?.aborted,true);
});
test("body shares the original deadline rather than starting a second one", {timeout:1500}, async () => {
 let release!:()=>void;
 const body=new Promise<string>(resolve=>{release=()=>resolve("late");});
 const pending=fetchTextWithTimeout(url,{},100,async()=>{
   await new Promise(resolve=>setTimeout(resolve,60));
   return {text:()=>body} as Response;
 });
 const lateBody = setTimeout(release,130);
 try { await assert.rejects(pending,/HTTP_TIMEOUT/); } finally {clearTimeout(lateBody);release();}
});
test("upstream cancellation bounds a noncooperative body", {timeout:1500}, async () => {
 const controller=new AbortController();const reason=new Error("OPERATOR_CANCELLED");
 const pending=fetchTextWithTimeout(url,{signal:controller.signal},1000,async()=>({text:()=>new Promise<string>(()=>{})}) as Response);
 controller.abort(reason);await assert.rejects(pending,error=>error===reason);
});
test("preaborted request never calls transport", {timeout:1500}, async () => {
 const controller=new AbortController();controller.abort(new Error("CANCELLED"));let calls=0;
 await assert.rejects(fetchWithTimeout(url,{signal:controller.signal},100,async()=>{calls++;return new Response();}),/CANCELLED/);
 assert.equal(calls,0);
});
test("completed text preserves error HTTP metadata and unicode", {timeout:1500}, async () => {
 const response=new Response("Отказ",{status:409,headers:{"X-Request-Id":"request-1"}});
 const result=await fetchTextWithTimeout(url,{},100,async()=>response);
 assert.equal(result.response,response);assert.equal(result.text,"Отказ");assert.equal(result.response.status,409);assert.equal(result.response.headers.get("X-Request-Id"),"request-1");
});
test("body rejection and late rejection are observed without replacing their causes", {timeout:1500}, async () => {
 const cause=new Error("BODY_FAILURE");
 await assert.rejects(fetchTextWithTimeout(url,{},100,async()=>Object.assign(new Response(),{text:async()=>{throw cause;}})),error=>error===cause);
 let rejectLate!:(error:Error)=>void;
 const pending=fetchWithTimeout(url,{},100,()=>new Promise<Response>((_resolve,reject)=>{rejectLate=reject;}));
 await assert.rejects(pending,/HTTP_TIMEOUT/);rejectLate(cause);await new Promise(resolve=>setTimeout(resolve,0));
});
test("headers-only API preserves Response and validates timeout before I/O", {timeout:1500}, async () => {
 const response=new Response("ok");assert.equal(await fetchWithTimeout(url,{},100,async()=>response),response);
 for(const timeout of [0,NaN,Infinity,120001])await assert.rejects(fetchTextWithTimeout(url,{},timeout,async()=>{throw Error("MUST_NOT_RUN");}),/HTTP_TIMEOUT_INVALID/);
});
