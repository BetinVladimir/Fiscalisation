// @vitest-environment jsdom
// Authored only; execution deferred until global gate.
import React,{act} from 'react';import {createRoot,type Root} from 'react-dom/client';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
vi.mock('react-native-paper',()=>({ActivityIndicator:()=>null,HelperText:()=>null,PaperProvider:()=>null,Surface:()=>null}));
vi.mock('react-native-safe-area-context',()=>({SafeAreaProvider:()=>null}));
vi.mock('../src/ui/app-button',()=>({AppButton:()=>null}));
vi.mock('../src/ui/fonts',()=>({loadRobotoFonts:()=>Promise.resolve()}));
vi.mock('../../../../BeeloyBackend/mobile/shared-ui/src/theme/paper',()=>({createBeeloyPaperThemes:()=>({light:{},dark:{}}),withSystemFonts:(value:any)=>value}));
import {useSystemMedia} from '../src/ui/theme';
let root:Root,host:HTMLDivElement,unmounted=false,original:PropertyDescriptor|undefined;
function Probe({query}:{query:string}){return <output>{String(useSystemMedia(query))}</output>}
beforeEach(()=>{(globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;unmounted=false;original=Object.getOwnPropertyDescriptor(window,'matchMedia');host=document.createElement('div');document.body.append(host);root=createRoot(host)});
afterEach(async()=>{if(!unmounted)await act(async()=>root.unmount());host.remove();if(original)Object.defineProperty(window,'matchMedia',original);else Reflect.deleteProperty(window,'matchMedia')});
it('missing matchMedia safely defaults to false',async()=>{Reflect.deleteProperty(window,'matchMedia');await act(async()=>root.render(<Probe query="(prefers-color-scheme: dark)"/>));expect(host.textContent).toBe('false')});
it.each(['modern','legacy'] as const)('%s subscription follows changes and retires old query callbacks',async api=>{
 const entries=new Map<string,any>();
 Object.defineProperty(window,'matchMedia',{configurable:true,value:(query:string)=>{if(!entries.has(query)){const entry:any={matches:true,listener:()=>{},remove:vi.fn()};if(api==='modern'){entry.addEventListener=(_event:string,fn:any)=>entry.listener=fn;entry.removeEventListener=entry.remove;}else{entry.addListener=(fn:any)=>entry.listener=fn;entry.removeListener=entry.remove;}entries.set(query,entry)}return entries.get(query)}});
 await act(async()=>root.render(<Probe query="motion"/>));expect(host.textContent).toBe('true');const first=entries.get('motion');await act(async()=>{first.matches=false;first.listener()});expect(host.textContent).toBe('false');
 await act(async()=>root.render(<Probe query="theme"/>));expect(first.remove).toHaveBeenCalled();expect(host.textContent).toBe('true');await act(async()=>{first.matches=false;first.listener()});expect(host.textContent).toBe('true');
 const latest=entries.get('theme');await act(async()=>root.unmount());unmounted=true;expect(latest.remove).toHaveBeenCalled();await act(async()=>latest.listener());
});
