import React from 'react';
import {act,create,type ReactTestRenderer} from 'react-test-renderer';
jest.mock('react-native',()=>({Platform:{OS:'web'},View:'View',useWindowDimensions:()=>({width:320,height:480,scale:1,fontScale:2})}));
jest.mock('react-native-paper',()=>({Appbar:{Header:'Header'},IconButton:'IconButton',Text:'Text',useTheme:()=>({colors:{}})}));
jest.mock('../src/ui/header-icons',()=>({headerIcons:{}}));
import {Platform} from 'react-native';
import {useHeaderMetrics,parseHeaderZoom} from '../src/ui/PaperOperationalHeader';
let value!:ReturnType<typeof useHeaderMetrics>;
function Harness(){value=useHeaderMetrics();return null;}
test.each([['200%',2],[' 125% ',1.25],['2',2],['normal',1],['0',1],['-2',1],['Infinity',1],['2oops',1],['',1]])('zoom %s parses to %s',(raw,expected)=>expect(parseHeaderZoom(raw as string)).toBe(expected));
test('actual hook observes style/class and resize, cleans both subscriptions; native font scaling stays untouched',()=>{
 const saved=Object.fromEntries(['window','document','getComputedStyle','MutationObserver'].map(key=>[key,Object.getOwnPropertyDescriptor(globalThis,key)]));
 let zoom='200%',observe!:()=>void,resize!:()=>void;const disconnect=jest.fn(),add=jest.fn((name:string,callback:()=>void)=>{resize=callback}),remove=jest.fn();
 const replacements={window:{addEventListener:add,removeEventListener:remove},document:{documentElement:{}},getComputedStyle:()=>({zoom}),MutationObserver:class{constructor(callback:()=>void){observe=callback}observe=jest.fn();disconnect=disconnect;}};for(const [key,replacement]of Object.entries(replacements))Object.defineProperty(globalThis,key,{value:replacement,configurable:true,writable:true});
 let tree!:ReactTestRenderer;
 try{act(()=>{tree=create(<Harness/>)});expect(value).toMatchObject({zoom:2,width:160,height:240});zoom='125%';act(()=>observe());expect(value.width).toBe(256);zoom='50%';act(()=>resize());expect(value).toMatchObject({zoom:.5,width:640,height:960});act(()=>tree.unmount());expect(disconnect).toHaveBeenCalledTimes(1);expect(remove).toHaveBeenCalledWith('resize',resize);
 (Platform as {OS:string}).OS='ios';act(()=>{tree=create(<Harness/>)});expect(value).toMatchObject({zoom:1,width:320,height:480,target:50});expect(add).toHaveBeenCalledTimes(1);act(()=>tree.unmount());
 }finally{(Platform as {OS:string}).OS='web';for(const [key,descriptor]of Object.entries(saved)){if(descriptor)Object.defineProperty(globalThis,key,descriptor);else Reflect.deleteProperty(globalThis,key);}}
});
