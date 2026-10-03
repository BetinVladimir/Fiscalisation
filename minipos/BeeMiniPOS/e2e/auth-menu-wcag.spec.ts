import {test,expect} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import {mkdir,writeFile} from 'node:fs/promises';
for(const [size,width,height] of [['xs',360,800],['md',960,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Strict auth/menu WCAG ${size}/${theme}`,async({page},testInfo)=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await page.goto('/');await expect(page.getByTestId('operator-login-email')).toBeVisible();
  const scans=[];
  for(const state of ['login','language-menu','theme-menu']){
   if(state!=='login')await page.getByTestId(`${state}-open`).click();
   const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa']).analyze();
   const layout=await page.getByTestId('app-root').evaluate(el=>{
     const ancestors=[];let node:HTMLElement|null=el as HTMLElement;
     while(node){const r=node.getBoundingClientRect(),s=getComputedStyle(node);ancestors.push({tag:node.tagName,testid:node.dataset.testid,width:r.width,height:r.height,position:s.position,zIndex:s.zIndex,background:s.backgroundColor,flex:s.flex});node=node.parentElement;}
     const text=document.querySelector<HTMLElement>('[data-testid="operator-login-email.label"]')!,r=text.getBoundingClientRect();
     return {ancestors,stack:document.elementsFromPoint(r.x+r.width/2,r.y+r.height/2).map(n=>({tag:n.tagName,testid:n.getAttribute('data-testid'),role:n.getAttribute('role'),html:n.outerHTML.slice(0,300)}))};
   });
   const textStacks=await page.evaluate(targets=>targets.map(selector=>{
    const element=document.querySelector<HTMLElement>(selector);if(!element)return {selector,missing:true};
    const range=document.createRange();range.selectNodeContents(element);
    const rects=element.tagName==='INPUT'?[element.getBoundingClientRect()]:Array.from(range.getClientRects());
    return {selector,rects:rects.filter(r=>r.width>0&&r.height>0).map(r=>({x:r.x,y:r.y,width:r.width,height:r.height,stack:document.elementsFromPoint(r.x+r.width/2,r.y+r.height/2).slice(0,8).map(n=>{const s=getComputedStyle(n);return {html:n.outerHTML.slice(0,350),isTarget:n===element,insideTarget:element.contains(n),containsTarget:n.contains(element),color:s.color,background:s.backgroundColor,opacity:s.opacity,position:s.position,zIndex:s.zIndex};})}))};
   }),result.incomplete.flatMap(v=>v.nodes.flatMap(n=>n.target.filter((t):t is string=>typeof t==='string'))));
   scans.push({state,violations:result.violations,incomplete:result.incomplete,layout,textStacks});
   if(state!=='login'){await page.keyboard.press('Escape');await expect(page.getByTestId(state==='language-menu'?'language-menu-scroll':'theme-system')).not.toBeVisible();}
  }
  await writeFile(testInfo.outputPath('axe-report.json'),JSON.stringify({size,theme,scans},null,2));
  await mkdir('docs/ui-optimization-evidence/auth-menu-strict-aa/current',{recursive:true});
  await writeFile(`docs/ui-optimization-evidence/auth-menu-strict-aa/current/${size}-${theme}.json`,JSON.stringify({size,theme,scans},null,2));
  expect(scans.flatMap(s=>s.violations).map(v=>`${v.id}:${v.nodes.length}`),'Automatic WCAG violations').toEqual([]);
  expect(scans.flatMap(s=>s.incomplete).map(v=>`${v.id}:${v.nodes.length}`),'INCOMPLETE requires manual resolution before AA PASS').toEqual([]);
 });
}
