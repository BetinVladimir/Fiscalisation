import {test,expect} from 'vitest';
import {headerTargetSize} from '../src/ui/header-layout';
test('measured narrow row fits four actual controls, retains AA24 and native48',()=>{const size=headerTargetSize('web',128,4);expect(size).toBeGreaterThanOrEqual(24);expect(size*4+8+16).toBeLessThanOrEqual(128);expect(headerTargetSize('web',288,4)).toBe(48);expect(headerTargetSize('ios',128,4)).toBe(48);expect(headerTargetSize('android',128,4)).toBe(48);expect(headerTargetSize('web',152,3)).toBe(32);});
