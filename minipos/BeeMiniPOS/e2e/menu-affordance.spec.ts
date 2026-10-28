import { test, expect } from '@playwright/test';
import { writeFile } from 'node:fs/promises';
import { supportedLanguages, translations } from '../src/translations.generated';
import { doubleText } from './text-scale';
const languages=supportedLanguages;
for(const [size,width,height] of [['xs',360,800],['md',960,900],['lg',1440,1000]] as const) for(const theme of ['light','dark'] as const){
 test(`Paper menu frames and CSS200 ${size}/${theme}`,async({page},testInfo)=>{
  await page.setViewportSize({width,height}); await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
  await page.goto('/'); await expect(page.getByTestId('operator-login-email')).toBeVisible(); await doubleText(page);
  const measurements=[];
  for(const language of languages){
   await page.getByTestId('language-menu-open').click();
   const action=page.getByTestId(`language-${language}`);await action.scrollIntoViewIfNeeded();
   const metric=await action.evaluate(el=>{const s=getComputedStyle(el);const label=el.querySelector<HTMLElement>('[dir="auto"]')!;return {border:parseFloat(s.borderTopWidth),style:s.borderTopStyle,height:el.getBoundingClientRect().height,labelWidth:label.clientWidth,labelScroll:label.scrollWidth,labelHeight:label.clientHeight,labelScrollHeight:label.scrollHeight,size:parseFloat(getComputedStyle(label).fontSize),original:Number(label.dataset.originalFontSize)};});
   measurements.push({language,...metric});expect(metric.border).toBeGreaterThanOrEqual(1);expect(metric.style).toBe('solid');expect(metric.height).toBeGreaterThanOrEqual(48);expect(metric.size).toBeCloseTo(metric.original*2,1);expect(metric.labelScroll).toBeLessThanOrEqual(metric.labelWidth+1);expect(metric.labelScrollHeight).toBeLessThanOrEqual(metric.labelHeight+1);
   await action.focus();await page.keyboard.press('Enter');await expect(page.getByTestId('language-menu-scroll')).not.toBeVisible();
   await expect(page.getByTestId('operator-login-email')).toHaveAccessibleName(translations[language].email);
  }
  for(const choice of ['system','light','dark']){
   await page.getByTestId('theme-menu-open').click();const action=page.getByTestId(`theme-${choice}`);await action.scrollIntoViewIfNeeded();
   expect(await action.evaluate(el=>parseFloat(getComputedStyle(el).borderTopWidth))).toBeGreaterThanOrEqual(1);
   await action.focus();await page.keyboard.press('Enter');await expect(action).not.toBeVisible();
  }
  expect(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1)).toBe(false);
  await writeFile(testInfo.outputPath('menu-measurements.json'),JSON.stringify({size,theme,measurements},null,2));
  await page.getByTestId('language-menu-open').click();
  await page.getByTestId('language-ru').scrollIntoViewIfNeeded();
  await page.screenshot({path:testInfo.outputPath('menu-text200.png')});
  await testInfo.attach('menu-measurements',{body:JSON.stringify(measurements),contentType:'application/json'});
 });
}
