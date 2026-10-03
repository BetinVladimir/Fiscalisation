import {test,expect} from '@playwright/test';
import {writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
test('Diagnostic Axe virtual stack versus browser stack',async({page},testInfo)=>{
 await page.setViewportSize({width:360,height:800});await page.goto('/');await expect(page.getByTestId('operator-login-email')).toBeVisible();
 await page.addScriptTag({path:resolve('node_modules/axe-core/axe.min.js')});
 const diagnostic=await page.evaluate(async()=>{
  const engine=(window as any).axe;const captured:any[]=[];
  engine.configure({checks:[{id:'diagnostic-stack-check',evaluate:(node:HTMLElement)=>{
   const stacks=engine.commons.dom.getTextElementStack(node).map((stack:HTMLElement[])=>stack.map(el=>({html:el.outerHTML.slice(0,400),same:el===node,inside:node.contains(el),ancestor:el.contains(node),zIndex:getComputedStyle(el).zIndex,position:getComputedStyle(el).position})));
   captured.push({target:node.outerHTML.slice(0,400),stacks});return true;
  }}],rules:[{id:'diagnostic-stack-rule',selector:'[data-testid="operator-login-email.label"],h1,[data-testid="theme-menu-open-text"]',any:['diagnostic-stack-check'],enabled:true,tags:['diagnostic']} ]});
  await engine.run(document,{runOnly:{type:'rule',values:['diagnostic-stack-rule']}});return captured;
 });
 await writeFile(testInfo.outputPath('virtual-stacks.json'),JSON.stringify(diagnostic,null,2));
 expect(diagnostic).toHaveLength(3);
 expect(diagnostic.every((item:any)=>item.stacks.length>0&&item.stacks.every((stack:any[])=>stack[0]?.same))).toBe(true);
});
