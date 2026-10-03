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
   scans.push({state,violations:result.violations,incomplete:result.incomplete});
   if(state!=='login'){await page.keyboard.press('Escape');await expect(page.getByTestId(state==='language-menu'?'language-menu-scroll':'theme-system')).not.toBeVisible();}
  }
  await writeFile(testInfo.outputPath('axe-report.json'),JSON.stringify({size,theme,scans},null,2));
  await mkdir('docs/ui-optimization-evidence/auth-menu-strict-aa/current',{recursive:true});
  await writeFile(`docs/ui-optimization-evidence/auth-menu-strict-aa/current/${size}-${theme}.json`,JSON.stringify({size,theme,scans},null,2));
  expect(scans.flatMap(s=>s.violations).map(v=>`${v.id}:${v.nodes.length}`),'Automatic WCAG violations').toEqual([]);
  expect(scans.flatMap(s=>s.incomplete).map(v=>`${v.id}:${v.nodes.length}`),'INCOMPLETE requires manual resolution before AA PASS').toEqual([]);
 });
}
