import {expect,type Page} from '@playwright/test';
import {TEST_JWT,TEST_REFRESH,CONFIG} from './helpers';
export async function ordinaryAuthFixture(page:Page){
 const ledger:{method:string;path:string;body:unknown;authorization?:string;version?:string;appInstance?:string}[]=[],errors:string[]=[];
 const counts={request:0,verify:0,session:0,configuration:0,products:0,shifts:0};let instance='';
 page.on('pageerror',e=>errors.push(e.message));
 await page.route('**/*',async route=>{const request=route.request(),url=new URL(request.url()),headers=request.headers();
  if(!['fetch','xhr'].includes(request.resourceType()))return route.continue();
  const body=request.postData()?request.postDataJSON():null;
  ledger.push({method:request.method(),path:url.pathname+url.search,body,authorization:headers.authorization,version:headers['x-api-version'],appInstance:headers['x-app-instance-id']});
  try{
   expect(url.origin).toBe(new URL(page.url()).origin);expect(headers['x-api-version']).toBe('2026-08-07');expect(headers.accept).toBe('application/json');
   const prefix='/public/v1/minipos',path=url.pathname;expect(path.startsWith(prefix+'/')).toBe(true);
   if(path===prefix+'/auth/request-code'){
    expect(request.method()).toBe('POST');expect(url.search).toBe('');expect(headers.authorization).toBeUndefined();expect(headers['x-app-instance-id']).toBeUndefined();expect(headers['content-type']).toBe('application/json');expect(body).toEqual({email:'qa-minipos@example.invalid',language:'en'});counts.request++;
    return route.fulfill(counts.request===1?{status:503,json:{message:'PRIVATE_REQUEST_SECRET'}}:{status:202,json:{sent:true,expires_in:600}});
   }
   if(path===prefix+'/auth/verify-code'){
    expect(request.method()).toBe('POST');expect(url.search).toBe('');expect(headers.authorization).toBeUndefined();expect(headers['content-type']).toBe('application/json');expect(body).toEqual({email:'qa-minipos@example.invalid',code:'123456'});counts.verify++;
    return route.fulfill(counts.verify===1?{status:503,json:{message:'PRIVATE_VERIFY_SECRET'}}:{json:{access_token:TEST_JWT,refresh_token:TEST_REFRESH,expires_in:3600,onboarding_required:false}});
   }
   expect(request.method()).toBe('GET');expect(body).toBeNull();expect(headers.authorization).toBe('Bearer '+TEST_JWT);expect(headers['x-app-instance-id']).toMatch(/^[0-9a-f-]{36}$/i);if(!instance)instance=headers['x-app-instance-id'];expect(headers['x-app-instance-id']).toBe(instance);
   if(path===prefix+'/operator-session'){expect(url.search).toBe('');counts.session++;return route.fulfill(counts.session===1?{status:503,json:{message:'PRIVATE_SESSION_SECRET'}}:{json:{authenticated:true,session_id:'ordinary-email-session',employee:{id:'emp-001',first_name:'MVP',last_name:'Operator',operator_code:'001',roles:['OPERATOR']},expires_at:'2099-01-01T00:00:00Z'}});}
   if(path===prefix+'/configuration'){expect(url.search).toBe('');counts.configuration++;return route.fulfill({json:CONFIG});}
   if(path===prefix+'/products'){expect(url.search).toBe('');counts.products++;return route.fulfill({json:{items:[{id:'auth-product',name:'MVP Coffee',price:{amount:'2.50',currency:'EUR'},tax_group:'B',status:'ACTIVE'}]}});}
   if(path===prefix+'/shifts'){expect(url.search).toBe('?employee_id=emp-001&state=');counts.shifts++;return route.fulfill({json:{items:[]}});}
   throw Error('Unexpected API '+request.method()+' '+url.href);
  }catch(error){errors.push(String(error));return route.fulfill({status:599,json:{error:'UNEXPECTED_FIXTURE_REQUEST'}});}
 });
 return {ledger,errors,counts,verify(){expect(errors).toEqual([]);expect(counts).toEqual({request:2,verify:2,session:3,configuration:2,products:2,shifts:2});expect(ledger.filter(x=>x.method!=='GET')).toHaveLength(4);}};
}
