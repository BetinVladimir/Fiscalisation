// Authored only; execution deferred by the global forms/test gate.
import {test,expect} from '@playwright/test';
import {setup,waitForPOS} from './helpers';
for(const [size,width,height] of [['xs',320,800],['md',768,900],['lg',1440,1000]] as const)for(const theme of ['light','dark'] as const){
 test(`Paper sale invalid payment stays blocked ${size}/${theme}`,async({page})=>{
  await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await setup(page);await page.goto('/');await waitForPOS(page);
  await page.getByRole('button',{name:'Кафе',exact:true}).click();
  const checkout=page.getByTestId('sale-checkout'),card=page.getByTestId('sale-card-amount');await expect(checkout).toBeEnabled();
  for(const invalid of ['','abc','1e2','1.001']){await card.fill(invalid);await expect(card).toHaveValue(invalid);await expect(card).toHaveAttribute('aria-invalid','true');await expect(checkout).toBeDisabled();}
  await card.fill('100');await expect(checkout).toBeDisabled();
  await card.fill('-1');await expect(checkout).toBeDisabled();
  await card.fill('1,00');await expect(card).toHaveValue('1,00');await expect(checkout).toBeEnabled();
  await card.fill('0');await expect(checkout).toBeEnabled();
  const discount=page.locator('input[data-testid^="sale-discount-"]').first();await discount.fill('100');await expect(checkout).toBeDisabled();
  await discount.fill('0');await expect(checkout).toBeEnabled();
 });
}
