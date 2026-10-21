import {requireFiscalAuthorization}from'../src/fiscalAuthorization';
test('logout authority blocks every production fiscal request before transport',()=>{
 const transport=jest.fn();const send=(token:string)=>{const authorized=requireFiscalAuthorization(true,token);transport({Authorization:`Bearer ${authorized}`});};
 send('current-owner');expect(transport).toHaveBeenCalledWith({Authorization:'Bearer current-owner'});
 transport.mockClear();for(const revoked of['',' ','\n'])expect(()=>send(revoked)).toThrow('AUTH_REQUIRED');expect(transport).not.toHaveBeenCalled();
});
test('explicit non-production local mode retains its existing optional-token contract',()=>{expect(requireFiscalAuthorization(false,'')).toBe('');});
