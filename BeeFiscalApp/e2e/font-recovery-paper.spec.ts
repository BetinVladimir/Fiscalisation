// Authored only; runtime deferred until the global forms/test gate opens.
import { expect, test } from '@playwright/test';
import { mockFiscalApi, openApp } from './helpers';
for (const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const) {
 for (const theme of ['light','dark'] as const) {
  test(`Paper startup recovers failed font ${size}/${theme}`,async({page})=>{
   await page.setViewportSize({width,height});
   await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
   let failedFonts=0;
   await page.route(/\.(ttf|otf|woff2?)(\?.*)?$/,route=>{failedFonts++;return route.abort('failed');});
   const mutations=await mockFiscalApi(page);
   await openApp(page);
   expect(failedFonts,'fixture must actually fail a font request').toBeGreaterThan(0);
   await expect(page.getByTestId('beefiscal.startup.loading')).toHaveCount(0);
   await expect(page.getByTestId('status-fiscal-device')).toBeVisible();
   await expect(page.getByTestId('shell.language.open')).toBeEnabled();
   expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
   expect(mutations).toEqual([]);
  });
 }
}
