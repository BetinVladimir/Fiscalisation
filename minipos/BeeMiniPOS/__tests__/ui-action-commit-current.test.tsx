import React from 'react';
import {act,create,type ReactTestRenderer} from 'react-test-renderer';
(globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;
let mockButton:any,mockRipple:any;
jest.mock('react-native',()=>({View:'View'}));
jest.mock('react-native-paper',()=>({Button:(props:any)=>{mockButton=props;return require('react').createElement('Paper.Button',props)},TouchableRipple:(props:any)=>{mockRipple=props;return require('react').createElement('Paper.Ripple',props)},useTheme:()=>({colors:{outline:'#666'},roundness:8})}));
jest.mock('../src/ui/tokens',()=>({touchTarget:{min:48}}));
import {AppButton} from '../src/ui/app-button';
import {AppPressable} from '../src/ui/app-pressable';
const pending=new Promise(()=>{});
function Block({suspend}:{suspend:boolean}){if(suspend)throw pending;return null}
for(const [name,Component,read] of [['button',AppButton,()=>mockButton],['ripple',AppPressable,()=>mockRipple]] as const){
 function Harness({disabled=false,onPress,suspend=false}:{disabled?:boolean;onPress:()=>void;suspend?:boolean}){return <><Component disabled={disabled} onPress={onPress}>Action</Component><Block suspend={suspend}/></>}
 test(`${name}: abandoned enabled render cannot grant an action`,async()=>{
  const old=jest.fn(),next=jest.fn();let tree!:ReactTestRenderer;
  await act(async()=>{tree=create(<React.Suspense fallback={null}><Harness disabled onPress={old}/></React.Suspense>,{unstable_isConcurrent:true} as any)});
  const retained=read().onPress;
  await act(async()=>{React.startTransition(()=>tree.update(<React.Suspense fallback={null}><Harness onPress={next} suspend/></React.Suspense>))});
  const speculative=read().onPress;
  await act(async()=>{retained({});speculative({})});
  expect(old).not.toHaveBeenCalled();expect(next).not.toHaveBeenCalled();
  await act(async()=>tree.unmount());
 });
 test(`${name}: suspended disabled render preserves committed action`,async()=>{
  const old=jest.fn(),next=jest.fn();let tree!:ReactTestRenderer;
  await act(async()=>{tree=create(<React.Suspense fallback={null}><Harness onPress={old}/></React.Suspense>,{unstable_isConcurrent:true} as any)});
  const retained=read().onPress;
  await act(async()=>{React.startTransition(()=>tree.update(<React.Suspense fallback={null}><Harness disabled onPress={next} suspend/></React.Suspense>))});
  await act(async()=>retained({}));expect(old).toHaveBeenCalledTimes(1);expect(next).not.toHaveBeenCalled();
  await act(async()=>tree.unmount());
 });
 test(`${name}: retained callback cannot acquire a newly committed handler`,async()=>{
  const old=jest.fn(),next=jest.fn();let tree!:ReactTestRenderer;
  await act(async()=>{tree=create(<Harness onPress={old}/>)});const retained=read().onPress;
  await act(async()=>tree.update(<Harness onPress={next}/>));
  await act(async()=>retained({}));expect(old).not.toHaveBeenCalled();expect(next).not.toHaveBeenCalled();
  const current=read().onPress;await act(async()=>current({}));expect(next).toHaveBeenCalledTimes(1);
  await act(async()=>tree.unmount());await act(async()=>current({}));expect(next).toHaveBeenCalledTimes(1);
 });
}
