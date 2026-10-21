/** DOM semantics only. The rendered controls remain Paper RadioButton.Item. */
export function synchronizePaperRadioTabStops(group:HTMLElement):void{
 const radios=Array.from(group.querySelectorAll<HTMLElement>('[role="radio"]')).filter(radio=>radio.closest('[role="radiogroup"]')===group&&!radio.closest('[aria-hidden="true"],[inert]'));
 const enabled=radios.filter(radio=>radio.getAttribute('aria-disabled')!=='true');
 const entry=enabled.find(radio=>radio.getAttribute('aria-checked')==='true')??enabled[0];
 for(const radio of radios)radio.tabIndex=radio===entry?0:-1;
}
