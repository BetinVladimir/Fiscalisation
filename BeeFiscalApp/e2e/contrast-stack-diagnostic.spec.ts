import {test,expect} from '@playwright/test';
import {writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
for(const stage of ['email','otp'] as const) test(`Diagnostic Axe virtual stack ${stage}`,async({page},testInfo)=>{
 await page.setViewportSize({width:360,height:800});await page.goto('/');await expect(page.getByTestId('admin-login-email')).toBeVisible();
 if(stage==='otp'){await page.route('http://fiscal-admin.test/public/v1/app-auth/**',route=>route.fulfill({contentType:'application/json',body:JSON.stringify({temporary_token:'diagnostic-temporary'})}));await page.getByTestId('admin-login-email').fill('diagnostic@example.test');await page.getByTestId('admin-login-start').click();await expect(page.getByTestId('admin-login-code')).toBeVisible();}
 await page.addScriptTag({path:resolve('node_modules/axe-core/axe.min.js')});
 const diagnostic=await page.evaluate(async(selector)=>{
  const engine=(window as any).axe;const captured:any[]=[];
  engine.configure({checks:[{id:'diagnostic-stack-check',evaluate:(node:HTMLElement)=>{
   const stacks=engine.commons.dom.getTextElementStack(node).map((stack:HTMLElement[])=>stack.map(el=>({html:el.outerHTML.slice(0,400),same:el===node,inside:node.contains(el),ancestor:el.contains(node),zIndex:getComputedStyle(el).zIndex,position:getComputedStyle(el).position})));
   captured.push({target:node.outerHTML.slice(0,400),stacks});return true;
  }}],rules:[{id:'diagnostic-stack-rule',selector,any:['diagnostic-stack-check'],enabled:true,tags:['diagnostic']} ]});
  await engine.run(document,{runOnly:{type:'rule',values:['diagnostic-stack-rule']}});return captured;
 },`[data-testid="admin-login-${stage==='otp'?'code':'email'}"],h1,[data-testid="auth.theme.toggle-text"]`);
 await writeFile(testInfo.outputPath('virtual-stacks.json'),JSON.stringify(diagnostic,null,2));
 expect(diagnostic).toHaveLength(3);

});
