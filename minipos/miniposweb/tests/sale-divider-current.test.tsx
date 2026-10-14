// @vitest-environment jsdom
// Authored only; execution deferred until global gate.
import React,{act} from 'react';import {createRoot} from 'react-dom/client';import {expect,it,vi} from 'vitest';
const mock=vi.hoisted(()=>({styles:[] as any[]}));
vi.mock('react-native',()=>({View:({children,style}:any)=>{mock.styles.push(style);return <div>{children}</div>}}));
vi.mock('react-native-paper',()=>({Divider:({testID}:any)=><hr data-testid={testID}/>,Text:({children}:any)=><span>{children}</span>,Card:Object.assign(({children}:any)=><div>{children}</div>,{Content:({children}:any)=><div>{children}</div>})}));
vi.mock('../src/ui/app-button',()=>({AppButton:({children}:any)=><button>{children}</button>}));vi.mock('../src/ui/form-field',()=>({FormField:()=>null}));
import {PaymentPanel} from '../src/SalePanels';
it('each cart row uses Paper Divider instead of a View border',async()=>{(globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;mock.styles=[];const host=document.createElement('div'),root=createRoot(host);try{await act(async()=>root.render(<PaymentPanel cart={[{product:{id:'one',name:'Item',price:{amount:'1.00',currency:'EUR'}} as any,quantity:1,discount:0}]} total={1} cardAmount={0} busy={false} lastOrderId="" language="en" onUpdate={()=>{}} onCardAmount={()=>{}} onCheckout={()=>{}} onReverse={()=>{}}/>));expect(host.querySelector('[data-testid="sale-line-divider-one"]')).not.toBeNull();expect(mock.styles.every(style=>style?.borderBottomWidth===undefined)).toBe(true);}finally{await act(async()=>root.unmount());}});
