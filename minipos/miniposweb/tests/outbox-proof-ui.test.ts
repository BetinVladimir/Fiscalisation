// @vitest-environment jsdom
import React,{act} from 'react';
import {createRoot} from 'react-dom/client';
import {expect,it,vi} from 'vitest';
vi.mock('react-native-paper',()=>({ActivityIndicator:()=>null,HelperText:({children}:any)=>React.createElement('span',null,children),Text:({children,testID}:any)=>React.createElement('span',{'data-testid':testID},children),Card:Object.assign(({children}:any)=>React.createElement('div',null,children),{Content:({children}:any)=>React.createElement('div',null,children)})}));
vi.mock('../src/ui/app-button',()=>({AppButton:({children}:any)=>React.createElement('button',null,children)}));
import {OutboxPanel} from '../src/OutboxPanel';
import {translations} from '../src/ui/workTranslations.generated';
it.each([['FAILED','UNKNOWN','outboxUncertain'],['FAILED','','outboxUncertain'],['REJECTED','UNKNOWN','outboxUncertain'],['FAILED','PROVEN_FAILURE','outboxFailed'],['FISCALIZED','UNKNOWN','outboxUncertain'],['FISCALIZED','PROVEN_SUCCESS','outboxFiscalized']])('renders %s/%s with proof-aware label',async(state,certainty,label)=>{(globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;const host=document.createElement('div'),root=createRoot(host);try{await act(async()=>root.render(React.createElement(OutboxPanel,{rows:[{intent:{client_operation_id:'op'} as any,result:{operation_id:'op',state,certainty,updated_at:'now'},createdAt:'now',updatedAt:'now'}],loading:false,error:false,language:'en',onRefresh:()=>{}})));expect(host.querySelector('[data-testid="offline-outbox-status-op"]')?.textContent).toBe(translations.en[label as keyof typeof translations.en]);}finally{await act(async()=>root.unmount());}});
