import {headerTargetSize} from '../src/ui/header-layout';
test('outlined borders preserve actual48 target and narrowAA24 without overflowing row',()=>{
 const size=headerTargetSize('web',128,4);expect(size-2).toBeGreaterThanOrEqual(24);expect(size*4+8+16).toBeLessThanOrEqual(128);
 expect(headerTargetSize('web',288,4)-2).toBe(48);expect(headerTargetSize('ios',128,4)-2).toBe(48);expect(headerTargetSize('android',128,4)-2).toBe(48);expect(headerTargetSize('web',152,3)).toBe(34);
});
