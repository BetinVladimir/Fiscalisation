// @vitest-environment jsdom
import {test,expect,vi} from 'vitest';
import React,{act} from 'react';
import {createRoot} from 'react-dom/client';
vi.mock('react-native',()=>({Platform:{OS:'web'},View:'View',useWindowDimensions:()=>({width:320,height:480,scale:1,fontScale:2})}));
vi.mock('react-native-paper',()=>({Appbar:{Header:'Header'},IconButton:'IconButton',Text:'Text',useTheme:()=>({colors:{}})}));
vi.mock('../src/ui/header-icons',()=>({headerIcons:{}}));
import {Platform} from 'react-native';
import {useHeaderMetrics,parseHeaderZoom} from '../src/ui/PaperOperationalHeader';
let value!:ReturnType<typeof useHeaderMetrics>;
function Harness(){value=useHeaderMetrics();return null;}
test.each([['200%',2],[' 125% ',1.25],['2',2],['normal',1],['0',1],['-2',1],['Infinity',1],['2oops',1],['',1]])('zoom %s parses to %s',(raw,expected)=>expect(parseHeaderZoom(raw as string)).toBe(expected));
test('actual WEB hook updates from percentage style/class and resize and cleans up; native ignores fontScale',()=>{
 Object.assign(globalThis,{IS_REACT_ACT_ENVIRONMENT:true});
 const host=document.createElement('div');document.body.append(host);const root=createRoot(host);let zoom='200%';
 const computed=vi.spyOn(window,'getComputedStyle').mockImplementation(()=>({zoom}) as CSSStyleDeclaration);
 const add=vi.spyOn(window,'addEventListener'),remove=vi.spyOn(window,'removeEventListener');let observerCallback!:MutationCallback;
 const NativeObserver=globalThis.MutationObserver;const disconnect=vi.fn();
 globalThis.MutationObserver=class {constructor(callback:MutationCallback){observerCallback=callback}observe=vi.fn();disconnect=disconnect;takeRecords=()=>[];} as unknown as typeof MutationObserver;
 try{act(()=>root.render(<Harness/>));expect(value).toMatchObject({zoom:2,width:160,height:240});zoom='125%';act(()=>observerCallback([],{} as MutationObserver));expect(value.width).toBe(256);zoom='50%';act(()=>window.dispatchEvent(new Event('resize')));expect(value).toMatchObject({zoom:.5,width:640,height:960});act(()=>root.render(null));expect(disconnect).toHaveBeenCalledTimes(1);const resizeEntry=add.mock.calls.find(([name])=>name==='resize');expect(resizeEntry).toBeDefined();expect(remove).toHaveBeenCalledWith('resize',resizeEntry![1]);
 (Platform as {OS:string}).OS='ios';act(()=>root.render(<Harness/>));expect(value).toMatchObject({zoom:1,width:320,height:480,target:48});expect(add.mock.calls.filter(([name])=>name==='resize')).toHaveLength(1);
 }finally{act(()=>root.unmount());host.remove();(Platform as {OS:string}).OS='web';globalThis.MutationObserver=NativeObserver;computed.mockRestore();add.mockRestore();remove.mockRestore();}
});
