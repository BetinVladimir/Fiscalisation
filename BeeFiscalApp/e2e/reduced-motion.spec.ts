import {test,expect} from '@playwright/test';
for(const [size,width,height] of [['xs',360,800],['md',960,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper live reduced motion ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'no-preference'});await page.goto('/');await expect(page.getByTestId('admin-login-email')).toBeVisible();
  const button=page.getByTestId('auth.theme.toggle');
  for(const [motion,expected] of [['no-preference',1],['reduce',0],['no-preference',1]] as const){
   await page.emulateMedia({reducedMotion:motion});
   const box=(await button.boundingBox())!;await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.mouse.down();
   await expect(page.locator('[data-paper-ripple]')).toHaveCount(expected);
   await page.mouse.up();await expect(page.locator('[data-paper-ripple]')).toHaveCount(0);
   await expect(page.getByTestId('admin-login-email')).toBeVisible();
  }
 });
}
