import test from"node:test";import assert from"node:assert/strict";
import{FrameAssembler,frameMessage}from"./bleFrames.ts";
const id="10000000-0000-4000-8000-000000000001";
test("BFF1 fragments and restores an 8 KiB message",async()=>{const raw=Uint8Array.from({length:8192},(_,i)=>i%251),frames=await frameMessage(raw,id,185),a=new FrameAssembler();assert.ok(frames.length>1);let done:any;for(const f of frames)done=await a.accept(f);assert.equal(done.id,id);assert.deepEqual(done.payload,raw)});
test("BFF1 rejects order and digest corruption",async()=>{const frames=await frameMessage(new Uint8Array(300).fill(7),id,100),a=new FrameAssembler();await assert.rejects(a.accept(frames[1]),/ORDER/);frames.at(-1)![57]^=1;const b=new FrameAssembler();for(const f of frames.slice(0,-1))await b.accept(f);await assert.rejects(b.accept(frames.at(-1)!),/DIGEST/)});
