// Authored only; do not execute before the global forms/test completeness gate.
import {expect,test} from '@playwright/test';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const){
 for(const theme of ['light','dark'] as const){
  test(`Paper font failure recovery ${size}/${theme}`,async({page})=>{
   await page.setViewportSize({width,height});
   await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
   let failed=0;
   await page.route(/\.(ttf|otf|woff2?)(\?.*)?$/,route=>{failed++;return route.abort('failed');});
   await page.goto('/');
   const field=page.getByTestId('operator-login-email');
   await expect(field).toBeVisible();
   expect(failed,'must exercise a real font request failure').toBeGreaterThan(0);
   await expect(page.getByTestId('beeminipos.startup.loading')).toHaveCount(0);
   await field.fill('font-fixture@example.org');
   await expect(field).toHaveValue('font-fixture@example.org');
   await expect(page.getByTestId('operator-login-start')).toBeEnabled();
   expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
  });
 }
}
