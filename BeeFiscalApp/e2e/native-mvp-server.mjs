import {createServer} from 'node:http';

// Synthetic loopback fixtures still fail closed on oversized or stalled input.
export async function startContractServer(port, contract, {maxBytes=65536, timeoutMs=2000}={}) {
 if(!Number.isInteger(port)||port<0||port>65535||!Number.isInteger(maxBytes)||maxBytes<1||!Number.isInteger(timeoutMs)||timeoutMs<1)throw Error('Invalid fixture server policy');
 const server=createServer({maxHeaderSize:16384},async(req,res)=>{
  const reject=(status,code)=>{contract.ledger.push({method:req.method,url:req.url,status,accepted:false,error:code});res.writeHead(status,{'content-type':'application/json','connection':'close'});res.end(JSON.stringify({code}));res.once('finish',()=>req.destroy());};
  try{
   const raw=await new Promise((resolve,rejectBody)=>{
    const chunks=[];let length=0;
    const cleanup=()=>{clearTimeout(timer);req.off('data',data);req.off('end',end);req.off('error',error);req.off('aborted',aborted);};
    const fail=(status,code)=>{cleanup();rejectBody(Object.assign(new Error(code),{status,code}));};
    const data=chunk=>{length+=chunk.length;if(length>maxBytes)fail(413,'FIXTURE_BODY_TOO_LARGE');else chunks.push(chunk);};
    const end=()=>{cleanup();try{resolve(new TextDecoder('utf-8',{fatal:true}).decode(Buffer.concat(chunks)));}catch{rejectBody(Object.assign(new Error('FIXTURE_INVALID_UTF8'),{status:400,code:'FIXTURE_INVALID_UTF8'}));}};
    const error=()=>fail(400,'FIXTURE_BODY_READ_FAILED'),aborted=()=>fail(400,'FIXTURE_BODY_ABORTED');
    const timer=setTimeout(()=>fail(408,'FIXTURE_BODY_TIMEOUT'),timeoutMs);
    req.on('data',data);req.once('end',end);req.once('error',error);req.once('aborted',aborted);
   });
   const before=contract.ledger.length;
   const answer=contract.respond(req.method,req.url,raw,req.headers.authorization??'',req.headers);
   if(answer.accepted!==true&&contract.ledger.length===before)contract.ledger.push({method:req.method,url:req.url,status:answer.status,accepted:false});
   res.writeHead(answer.status,{'content-type':'application/json'});res.end(JSON.stringify(answer.out));
  }catch(error){if(!res.destroyed&&!res.headersSent)reject(error.status??400,error.code??'FIXTURE_REQUEST_REJECTED');else contract.ledger.push({method:req.method,url:req.url,status:400,accepted:false,error:'FIXTURE_RESPONSE_FAILED'});}
 });
 await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(port,'127.0.0.1',resolve)});
 return server;
}
