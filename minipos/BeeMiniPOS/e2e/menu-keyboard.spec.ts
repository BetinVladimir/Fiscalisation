import {test,expect} from '@playwright/test';
for(const [size,width,height] of [['xs',360,800],['md',960,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper menu keyboard ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await page.goto('/');
  const trigger=page.getByTestId('language-menu-open');await trigger.focus();await page.keyboard.press('Enter');
  await expect(page.getByTestId('language-bg')).toBeFocused();
  await page.keyboard.press('ArrowDown');await expect(page.getByTestId('language-hr')).toBeFocused();
  await page.keyboard.press('End');await expect(page.getByTestId('language-ru')).toBeFocused();
  await page.keyboard.press('ArrowDown');await expect(page.getByTestId('language-bg')).toBeFocused();
  await page.keyboard.press('ArrowUp');await expect(page.getByTestId('language-ru')).toBeFocused();
  await page.keyboard.press('Home');await expect(page.getByTestId('language-bg')).toBeFocused();
  await page.keyboard.press('Escape');await expect(page.getByTestId('language-menu-scroll')).not.toBeVisible();await expect(trigger).toBeFocused();
  const themeTrigger=page.getByTestId('theme-menu-open');await themeTrigger.focus();await page.keyboard.press('Enter');
  await expect(page.getByTestId('theme-system')).toBeFocused();
  await page.keyboard.press('ArrowDown');await expect(page.getByTestId('theme-light')).toBeFocused();
  await page.keyboard.press('End');await expect(page.getByTestId('theme-dark')).toBeFocused();
  await page.keyboard.press('Home');await expect(page.getByTestId('theme-system')).toBeFocused();
  await page.keyboard.press('Escape');await expect(page.getByTestId('theme-system')).not.toBeVisible();await expect(themeTrigger).toBeFocused();
 });
}
