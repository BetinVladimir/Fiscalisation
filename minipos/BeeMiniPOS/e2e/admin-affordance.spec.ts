import { test, expect } from '@playwright/test';
import { setupMiniposRoutes, setupFiscalRoutes, loginViaUI, waitForReady, EMPLOYEES } from './helpers';

for (const [size, width, height] of [['xs',360,800],['md',960,900],['lg',1440,1000]] as const) {
  for (const theme of ['light','dark'] as const) {
    test(`Paper admin action frames ${size}/${theme}`, async ({page}, testInfo) => {
      await page.setViewportSize({width,height});
      await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
      await setupMiniposRoutes(page); await setupFiscalRoutes(page);
      await page.route('**/operator-session', route => route.fulfill({contentType:'application/json',body:JSON.stringify({authenticated:true,employee:{...EMPLOYEES[0],roles:['ADMIN']},expires_at:'2099-12-31T23:59:59Z'})}));
      await page.goto('/'); await loginViaUI(page); await waitForReady(page);
      await page.getByTestId('admin-toggle').click();
      await page.getByTestId('admin-tax-groups').click();
      const back = page.getByTestId('admin-catalog-back');
      const row = page.getByRole('button',{name:'B • Стандартна • 20% • активна',exact:true});
      const measurements=[];
      for (const action of [back,row]) {
        await action.scrollIntoViewIfNeeded();
        const m=await action.evaluate(el=>{
          const candidates=[el,el.parentElement].filter(Boolean) as HTMLElement[];
          return candidates.map(node=>{const s=getComputedStyle(node);return {border:parseFloat(s.borderTopWidth),style:s.borderTopStyle,color:s.borderTopColor,height:node.getBoundingClientRect().height};});
        });
        measurements.push(m);
        expect(m.some(x=>x.border>=1&&x.style==='solid'&&x.color!=='rgba(0, 0, 0, 0)')).toBe(true);
        expect((await action.boundingBox())!.height).toBeGreaterThanOrEqual(48);
      }
      await row.focus(); await page.keyboard.press('Enter');
      await expect(page.getByTestId('tax-group-name')).toHaveValue('Стандартна');
      await expect(page.getByTestId('tax-group-code')).toHaveValue('B');
      await page.getByTestId('admin-catalog-back').focus(); await page.keyboard.press('Enter');
      await expect(page.getByTestId('admin-tax-groups')).toBeVisible();
      expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
      await testInfo.attach('action-frames',{body:JSON.stringify(measurements),contentType:'application/json'});
      await page.getByTestId('admin-tax-groups').click();
      await page.screenshot({path:testInfo.outputPath('admin-actions.png')});
    });
  }
}
