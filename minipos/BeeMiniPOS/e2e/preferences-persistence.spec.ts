import {test,expect} from '@playwright/test';
import {supportedLanguages,translations} from '../src/translations.generated';
for(const [size,width,height] of [['xs',360,800],['md',960,900],['lg',1440,1000]] as const)for(const scheme of ['light','dark'] as const){
 test(`Real preference persistence ${size}/${scheme}`,async({page})=>{
  test.setTimeout(60000);await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:scheme,reducedMotion:'reduce'});await page.goto('/');await expect(page.getByTestId('operator-login-email')).toBeVisible();
  const chosen=scheme==='light'?'dark':'light';
  await page.getByTestId('theme-menu-open').click();await page.getByTestId(`theme-${chosen}`).click();
  const background=await page.getByTestId('app-root').evaluate(el=>getComputedStyle(el).backgroundColor);
  for(const language of supportedLanguages){
   await page.getByTestId('language-menu-open').click();const action=page.getByTestId(`language-${language}`);await action.scrollIntoViewIfNeeded();await action.click();
   await expect(page.getByTestId('operator-login-email')).toHaveAccessibleName(translations[language].email);
   await page.reload();await expect(page.getByTestId('operator-login-email')).toHaveAccessibleName(translations[language].email);
   await expect.poll(()=>page.getByTestId('app-root').evaluate(el=>getComputedStyle(el).backgroundColor)).toBe(background);
   await page.getByTestId('language-menu-open').click();
   const selected=page.getByTestId(`language-${language}`);await expect(selected).toHaveAttribute('aria-checked','true');await expect(selected).toBeFocused();
   await expect(page.locator('[role="menuitemradio"][aria-checked="true"]')).toHaveCount(1);
   await page.keyboard.press('Escape');await expect(page.getByTestId('app-root')).not.toHaveAttribute('inert');
   await page.getByTestId('theme-menu-open').click();await expect(page.getByTestId(`theme-${chosen}`)).toHaveAttribute('aria-checked','true');await expect(page.getByTestId(`theme-${chosen}`)).toBeFocused();await page.keyboard.press('Escape');
  }
 });
}
