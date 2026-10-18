import {afterEach,expect,it,vi} from 'vitest';
import {MiniPosClient} from '../src/api/client';
afterEach(()=>vi.unstubAllGlobals());
it('sends required version and mutation idempotency to the real source API contract',async()=>{
 const requests:RequestInit[]=[];vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{requests.push(init);return new Response('{}',{status:200,headers:{'Content-Type':'application/json'}})}));
 const api=new MiniPosClient('/public/v1/minipos',()=> 'jwt','app');
 await api.createOrder('shift');await api.openShift('register','employee');
 for(const req of requests){const h=new Headers(req.headers);expect(h.get('X-Api-Version')).toBe('2026-08-07');expect(h.get('Idempotency-Key')).toMatch(/^[0-9a-f-]{36}$/);expect(h.get('Authorization')).toBe('Bearer jwt');expect(h.get('X-App-Instance-Id')).toBe('app');}
});
