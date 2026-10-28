import {expect,test} from '@playwright/test';
import {mockFiscalApi,openApp} from './helpers';

test('reference and existing-record radios expose one named checked row and one keyboard stop',async({page})=>{
 await mockFiscalApi(page,{secondRegister:true});await openApp(page);
 const reference=page.getByRole('radiogroup',{name:'Касово място',exact:true}).first();
 await expect(reference).toHaveCount(1);
 const verify=async(group:ReturnType<typeof page.getByRole>,selected:string)=>{
  const rows=group.getByRole('radio');await expect(rows).toHaveCount(2);
  for(const name of ['R01','R02']){const row=group.getByRole('radio',{name,exact:true});await expect(row).toHaveAttribute('aria-checked',String(name===selected));}
  await expect(group.locator('[role="radio"][tabindex="0"]')).toHaveCount(1);
  await expect(group.locator('[role="radio"] [role="radio"]')).toHaveCount(0);
 };
 await verify(reference,'R01');await reference.getByRole('radio',{name:'R01',exact:true}).focus();await page.keyboard.press('ArrowDown');await verify(reference,'R02');
 await expect(reference.getByRole('radio',{name:'R02',exact:true})).toBeFocused();
 await page.getByTestId('nav.open').click();await page.getByTestId('shell.navigation.modal').getByTestId('tab-Администриране').click();
 const choices=page.getByRole('radiogroup',{name:'Съществуващи записи',exact:true}).filter({has:page.getByRole('radio',{name:'R02',exact:true})});
 await verify(choices,'R02');await choices.getByRole('radio',{name:'R02',exact:true}).focus();await page.keyboard.press('Home');await verify(choices,'R01');
 await expect(choices.getByRole('radio',{name:'R01',exact:true})).toBeFocused();
});
