import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import {expect,it,vi} from 'vitest';
vi.mock('react-native-paper',()=>({Card:Object.assign(({children}:any)=><section>{children}</section>,{Content:({children}:any)=><div>{children}</div>}),Text:({children}:any)=><span>{children}</span>}));
import {OperationOutcome} from '../src/ui/OperationOutcome';
import {operationOutcomeCopyFor} from '../src/ui/operation-outcome-copy';
import type {FiscalOperation} from '../src/types';
it.each(['en','bg','ru'])('renders actual selected %s outcome and never presents uncertain reference as confirmed',(language)=>{
 const copy=operationOutcomeCopyFor(language);
 for(const [certainty,kind] of [['PROVEN_SUCCESS','confirmed'],['UNKNOWN','pending']] as const){
 const value={state:'FISCALIZED',certainty,fiscal_reference:'TEST-REFERENCE'} as FiscalOperation;
 const html=renderToStaticMarkup(<OperationOutcome value={value} language={language}/>);
 expect(html).toContain(copy[`${kind}Title`]);expect(html).toContain(copy[`${kind}Body`]);
 expect(html.includes('TEST-REFERENCE')).toBe(kind==='confirmed');
 expect(html).not.toContain('"certainty"');expect(html).not.toContain('"state"');
 }
 const failed=renderToStaticMarkup(<OperationOutcome value={{state:'FAILED',certainty:'PROVEN_FAILURE'} as FiscalOperation} language={language}/>);
 expect(failed).toContain(copy.failedTitle);expect(failed).toContain(copy.failedBody);
});
