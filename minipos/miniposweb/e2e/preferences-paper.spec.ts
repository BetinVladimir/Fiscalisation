// Authored only; execution deferred by global forms/test readiness gate.
import {test,expect} from '@playwright/test';
import {setupMiniposRoutes} from './helpers';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper preference radio scroll ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
  await setupMiniposRoutes(page);await page.goto('/');await expect(page.getByTestId('auth-email')).toBeVisible();
  await page.getByTestId('auth-language-open').click();
  await expect(page.getByRole('menuitemradio')).toHaveCount(25);await expect(page.getByRole('menuitemradio',{checked:true})).toHaveCount(1);
  await expect(page.getByTestId('auth-language-bg')).toBeFocused();await page.keyboard.press('End');
  const last=page.getByTestId('auth-language-ru');await expect(last).toBeFocused();
  const item=await last.boundingBox(),menu=await page.getByTestId('auth-language-scroll').boundingBox();expect(item).not.toBeNull();expect(menu).not.toBeNull();
  expect(item!.y).toBeGreaterThanOrEqual(menu!.y-1);expect(item!.y+item!.height).toBeLessThanOrEqual(menu!.y+menu!.height+1);expect(item!.height).toBeGreaterThanOrEqual(48);expect(menu!.x).toBeGreaterThanOrEqual(0);expect(menu!.x+menu!.width).toBeLessThanOrEqual(width+1);
  // Resizing while the menu is open must retain the roving keyboard position.
  await page.setViewportSize({width:Math.max(320,width-16),height:height-80});
  await expect(last).toBeFocused();
  await page.keyboard.press('ArrowUp');await expect(page.getByTestId('auth-language-sv')).toBeFocused();
  await page.keyboard.press('End');await expect(last).toBeFocused();
  await last.click();await expect(page.getByTestId('auth-language-scroll')).toHaveCount(0);
  await expect.poll(()=>page.evaluate(()=>localStorage.getItem('minipos-language'))).toBe('ru');
  await page.getByTestId('auth-theme-open').click();await expect(page.getByRole('menuitemradio')).toHaveCount(3);await expect(page.getByRole('menuitemradio',{checked:true})).toHaveCount(1);
  await page.getByTestId('auth-theme-dark').click();
  await expect(page.getByTestId('auth-theme-scroll')).toHaveCount(0);
  await expect.poll(()=>page.evaluate(()=>localStorage.getItem('minipos-web-theme'))).toBe('dark');
  await page.getByTestId('auth-language-open').click();
  await expect(page.getByTestId('auth-language-ru')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('auth-language-scroll')).toHaveCount(0);
  await expect(page.getByTestId('auth-language-open')).toBeFocused();
 });
}
