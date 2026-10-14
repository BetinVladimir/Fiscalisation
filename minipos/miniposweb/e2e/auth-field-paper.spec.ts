// Authored only; execution deferred until all apps' forms and test matrices are ready.
import {test,expect} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import {setupMiniposRoutes} from './helpers';
import {doubleText} from './text-scale';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const){
 for(const theme of ['light','dark'] as const){
  test(`Paper persistent email label CSS200 ${size}/${theme}`,async({page})=>{
   await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
   await setupMiniposRoutes(page);await page.goto('/');
   const email=page.getByTestId('auth-email');await expect(email).toBeVisible();await doubleText(page);
   const label=page.getByTestId('auth-email.label');await label.scrollIntoViewIfNeeded();
   const id=await label.getAttribute('id');expect(id).toBeTruthy();await expect(email).toHaveAttribute('aria-labelledby',id!);
   await expect(email).toHaveAttribute('autocomplete','email');await email.fill('paper@example.org');await expect(email).toHaveValue('paper@example.org');
   expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
   const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']).analyze();expect(result.violations).toEqual([]);expect(result.incomplete).toEqual([]);
  });
 }
}
