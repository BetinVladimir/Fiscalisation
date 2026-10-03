import { test, expect } from '@playwright/test';
import { writeFile } from 'node:fs/promises';
import { mockFiscalApi, openApp } from './helpers';
import { frameContrast } from './affordance-contrast';

for (const [size, width, height] of [['xs',360,800],['md',960,900],['lg',1440,1000]] as const) {
 for (const mode of ['light','dark'] as const) {
  test(`Paper audit decorative icon contrast and keyboard ${size}/${mode}`, async ({page},info)=>{
   await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:mode,reducedMotion:'reduce'});
   await mockFiscalApi(page);await openApp(page);if(size==='xs')await page.getByTestId('nav.open').click();await page.getByTestId('tab-Одит').click();
   const action=page.getByTestId('audit.details.audit-1');await action.scrollIntoViewIfNeeded();await action.focus();await expect(action).toHaveAttribute('aria-expanded','false');await action.press('Enter');await expect(action).toHaveAttribute('aria-expanded','true');
   const icon=action.locator('[role="img"]');await expect(icon).toHaveCount(1);await expect(icon).toBeVisible();
   expect(await icon.evaluate(node=>Boolean(node.closest('[aria-hidden="true"]')))).toBe(true);
   const contrast=await frameContrast(icon,'color');expect(contrast.insideRatio).toBeGreaterThanOrEqual(3);expect(contrast.outsideRatio).toBeGreaterThanOrEqual(3);
   await expect(action).toContainText('Скрий подробности');await expect(page.getByTestId('audit.details-content.audit-1')).toBeVisible();await action.press('Enter');await expect(action).toHaveAttribute('aria-expanded','false');await expect(action).toContainText('Покажи подробности');await expect(action).toBeFocused();
   await writeFile(info.outputPath('audit-icon.json'),JSON.stringify({size,mode,contrast,scope:'Computed graphic contrast and real keyboard/hidden decorative icon semantics; not complete WCAG adjudication.'},null,2));
  });
 }
}
