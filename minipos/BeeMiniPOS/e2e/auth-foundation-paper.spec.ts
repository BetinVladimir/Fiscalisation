// Authored only; runtime execution is deferred by the global forms/tests gate.
import {expect,test} from '@playwright/test';
import {lightColors,darkColors} from '@beeloy/design-tokens';
import AxeBuilder from '@axe-core/playwright';
import {doubleText} from './text-scale';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const){
 for(const theme of ['light','dark'] as const){
  test(`Paper email foundation CSS200 ${size}/${theme}`,async({page})=>{
   await page.setViewportSize({width,height});
   await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
   await page.goto('/');
   const field=page.getByTestId('operator-login-email');
   await expect(field).toBeVisible();
   const paper=page.getByTestId('app-paper-surface');await expect(paper).toBeVisible();
   const expected=await page.evaluate(color=>{const probe=document.createElement('span');probe.style.color=color;document.body.append(probe);const value=getComputedStyle(probe).color;probe.remove();return value;},theme==='dark'?darkColors.background:lightColors.background);
   await expect.poll(()=>paper.evaluate(node=>getComputedStyle(node).backgroundColor)).toBe(expected);
   await expect.poll(()=>page.getByTestId('app-root').evaluate(node=>getComputedStyle(node).backgroundColor)).toBe('rgba(0, 0, 0, 0)');
   await doubleText(page);
   const label=page.getByTestId('operator-login-email.label');
   const id=await label.getAttribute('id');
   expect(id).toBeTruthy();
   await expect(field).toHaveAttribute('aria-labelledby',id!);
   await expect(field).toHaveAttribute('autocomplete','email');
   const action=page.getByTestId('operator-login-start');
   await expect(action).toBeDisabled();
   await field.fill('paper-fixture@example.org');
   await expect(field).toHaveValue('paper-fixture@example.org');
   await expect(action).toBeEnabled();
   await action.scrollIntoViewIfNeeded();
   expect((await action.boundingBox())!.height).toBeGreaterThanOrEqual(48);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
   const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']).analyze();
   expect(result.violations).toEqual([]);
   expect(result.incomplete).toEqual([]);
  });
 }
}
