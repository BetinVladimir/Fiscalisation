// @vitest-environment jsdom
// Authored only; execution deferred until global gate.
import React,{act} from 'react';import {createRoot,type Root} from 'react-dom/client';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
const capture=vi.hoisted(()=>({button:null as any}));
vi.mock('react-native-paper',()=>({Button:(props:any)=>{capture.button=props;return <button disabled={props.disabled}>{props.children}</button>}}));
import {AppButton} from '../src/ui/app-button';
let root:Root,host:HTMLDivElement,unmounted=false;
beforeEach(()=>{(globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;capture.button=null;unmounted=false;host=document.createElement('div');document.body.append(host);root=createRoot(host)});
afterEach(async()=>{if(!unmounted)await act(async()=>root.unmount());host.remove()});
it.each([undefined,'text','outlined','contained','contained-tonal','elevated'] as const)('mode %s keeps action visually identifiable',async(mode)=>{await act(async()=>root.render(<AppButton mode={mode}>Action</AppButton>));expect(capture.button.mode).toBe(!mode||mode==='text'?'outlined':mode);expect(capture.button.accessibilityRole).toBe('button');expect(capture.button.maxFontSizeMultiplier).toBe(0);expect(capture.button.labelNumberOfLines).toBe(0);expect(capture.button.contentStyle[0]).toEqual({minHeight:48,minWidth:48});expect(capture.button.labelStyle[0].flexWrap).toBe('wrap')});
it.each(['disabled','loading'] as const)('blocked %s refuses even retained direct handler',async(flag)=>{const press=vi.fn();await act(async()=>root.render(<AppButton {...{[flag]:true}} onPress={press}>Action</AppButton>));expect(capture.button.disabled).toBe(true);capture.button.onPress({});expect(press).not.toHaveBeenCalled()});
it('active action forwards event; retired callback cannot invoke replacement',async()=>{const first=vi.fn(),next=vi.fn(),event={};await act(async()=>root.render(<AppButton onPress={first}>Action</AppButton>));const old=capture.button.onPress;old(event);expect(first).toHaveBeenCalledWith(event);await act(async()=>root.render(<AppButton onPress={next}>Action</AppButton>));old(event);expect(first).toHaveBeenCalledTimes(1);expect(next).not.toHaveBeenCalled();capture.button.onPress(event);expect(next).toHaveBeenCalledWith(event);const retired=capture.button.onPress;await act(async()=>root.unmount());unmounted=true;retired(event);expect(next).toHaveBeenCalledTimes(1)});
