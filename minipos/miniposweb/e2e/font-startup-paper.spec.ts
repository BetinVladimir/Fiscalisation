// Authored only; execution deferred until the global gate.
import {test,expect} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
for(const width of [320,768,1440])for(const theme of ['light','dark'] as const){
 test(`font failure and retry ${width} ${theme}`,async({page},testInfo)=>{
  await page.setViewportSize({width,height:900});
  await page.addInitScript(mode=>localStorage.setItem('minipos-web-theme',mode),theme);
  const fontPattern=/\.ttf(?:\?.*)?$/;
  const failFont=(route:import('@playwright/test').Route)=>{
   const request=new URL(route.request().url());
   return request.searchParams.has('url')||request.searchParams.has('import')?route.continue():route.abort();
  };
  await page.route(fontPattern,failFont);
  await page.goto('/');
  const startup=page.getByTestId('font-startup'),retry=page.getByTestId('font-retry');
  await expect(startup).toBeVisible();await expect(startup.getByRole('alert')).toBeVisible();await expect(retry).toBeEnabled();
  const box=await startup.boundingBox();expect(box).not.toBeNull();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(width+1);
  const scan=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21aa','wcag22aa']).analyze();
  await testInfo.attach('font-error-wcag',{body:JSON.stringify({violations:scan.violations,incomplete:scan.incomplete},null,2),contentType:'application/json'});
  expect(scan.violations).toEqual([]);expect(scan.incomplete).toEqual([]);
  await retry.focus();await expect(retry).toBeFocused();
  await page.unroute(fontPattern,failFont);await page.keyboard.press('Enter');await expect(startup).toBeHidden();
 });
}
