// Authored only; global runtime execution gate remains closed.
import {describe,it,expect} from 'vitest';
import {parseSaleNumber} from '../src/saleNumber';
describe('sale decimal drafts',()=>{
 for(const text of ['', ' ', '-1', '1e2', 'Infinity', 'NaN', '1.001', '1,234', '01', '.5'])it(`rejects money ${JSON.stringify(text)}`,()=>expect(parseSaleNumber(text,2)).toBeNull());
 for(const [text,value] of [['0',0],[' 2,50 ',2.5],['1.00',1],['12.34',12.34]] as const)it(`accepts money ${text}`,()=>expect(parseSaleNumber(text,2)).toBe(value));
 it('quantity supports three decimals without rounding',()=>{expect(parseSaleNumber('1.125',3)).toBe(1.125);expect(parseSaleNumber('1.1251',3)).toBeNull();});
 it('rejects values exceeding safe scaled integer range',()=>expect(parseSaleNumber('9999999999999999',2)).toBeNull());
});
