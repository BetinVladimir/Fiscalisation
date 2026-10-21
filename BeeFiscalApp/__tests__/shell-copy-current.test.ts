import {shellCopies,shellCopy,shellTabKeys,shellTabLabel} from '../src/ui/shell-copy';
test('EN shell presents all allowed tabs in English without changing internal keys',()=>{
 expect(Object.keys(shellTabKeys)).toEqual(['Устройства','Операции','Отчети','Одит','Администриране','Настройки']);
 expect(Object.keys(shellTabKeys).map(key=>shellTabLabel('en',key))).toEqual(['Devices','Operations','Reports','Audit','Administration','Settings']);
 expect(shellCopy('en').logout).toBe('Sign out');
 expect(shellCopy('en').open.replace('{name}',shellTabLabel('en','Устройства'))).toBe('Open Devices');
 expect(shellTabLabel('en','Future tab')).toBe('Future tab');
});
test('BG/RU operational copy retains keys and localized accessible actions',()=>{
 for(const locale of ['bg','ru'] as const){expect(Object.keys(shellCopies[locale]).sort()).toEqual(Object.keys(shellCopies.en).sort());expect(shellCopies[locale].open).toContain('{name}');expect(shellCopies[locale].company).toContain('{name}');}
 expect(shellTabLabel('bg','Администриране')).toBe('Администриране');expect(shellTabLabel('ru','Администриране')).toBe('Администрирование');expect(shellCopy('ru').register).toBe('Касса');
});
