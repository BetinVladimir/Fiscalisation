// Authored only; execution deferred until all apps' forms and tests are complete.
import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mockFiscalApi, openApp } from './helpers';
import { doubleText } from './text-scale';
import { frameContrast } from './affordance-contrast';
for (const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const) {
 for (const theme of ['light','dark'] as const) {
  test(`Paper shell CSS200 ${size}/${theme}`,async({page})=>{
   await page.setViewportSize({width,height});
   await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
   const mutations=await mockFiscalApi(page);
   await openApp(page);
   await doubleText(page);
   const status=page.getByTestId('status-fiscal-device');
   await status.scrollIntoViewIfNeeded();
   await expect(status).toContainText('Fiscal Core:');
   const contrast=await frameContrast(status.locator('[dir="auto"]').first(),'color');
   expect(contrast.insideRatio).toBeGreaterThanOrEqual(4.5);
   const probe=page.getByTestId('readiness-probe');
   await probe.scrollIntoViewIfNeeded();
   expect((await probe.boundingBox())!.height).toBeGreaterThanOrEqual(48);
   if(size==='xs') {
    await page.getByTestId('nav.open').click();
    const settings=page.getByTestId('tab-Настройки');
    await settings.scrollIntoViewIfNeeded();
    await expect(settings).toBeVisible();
    expect((await settings.boundingBox())!.height).toBeGreaterThanOrEqual(48);
    await settings.click();
    await expect(page.getByTestId('settings.screen')).toBeVisible();
   }
   expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
   const result=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21a','wcag21aa','wcag22aa']).analyze();
   expect(result.violations).toEqual([]);
   expect(result.incomplete).toEqual([]);
   expect(mutations).toEqual([]);
  });
 }
}
