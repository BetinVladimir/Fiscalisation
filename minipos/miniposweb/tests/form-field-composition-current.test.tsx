// @vitest-environment jsdom
// Authored only; execution deferred until global gate.
import React,{act} from 'react';import {createRoot,type Root} from 'react-dom/client';
import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest';
const capture=vi.hoisted(()=>({input:null as any,label:null as any,support:null as any}));
vi.mock('react-native',()=>({Platform:{OS:'web'},View:({children}:any)=><div>{children}</div>}));
vi.mock('react-native-paper',()=>({
 useTheme:()=>({colors:{onSurface:'#1c1b1f'}}),
 Text:(props:any)=>{capture.label=props;return <span>{props.children}</span>},
 TextInput:(props:any)=>{capture.input=props;return <input/>},
 HelperText:(props:any)=>{capture.support=props;return <span>{props.children}</span>},
}));
import {FormField} from '../src/ui/form-field';
let root:Root,host:HTMLDivElement;
beforeEach(()=>{(globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;capture.input=capture.label=capture.support=null;host=document.createElement('div');document.body.append(host);root=createRoot(host)});
afterEach(async()=>{await act(async()=>root.unmount());host.remove()});
describe('Paper field wrapper contract',()=>{
 it('persistent Paper label names the field and preserves value/handler',async()=>{const change=vi.fn();await act(async()=>root.render(<FormField testID="email" label="Email" value="a@example.org" onChangeText={change}/>));expect(capture.label.children).toBe('Email');expect(capture.label.testID).toBe('email.label');expect(capture.input['aria-labelledby']).toBe(capture.label.nativeID);expect(capture.input.label).toBeUndefined();expect(capture.input.accessibilityLabel).toBe('Email');expect(capture.input.mode).toBe('outlined');expect(capture.input.value).toBe('a@example.org');capture.input.onChangeText('b@example.org');expect(change).toHaveBeenCalledWith('b@example.org');expect(capture.input.maxFontSizeMultiplier).toBe(0)});
 it('error keeps external description, uses unique support id, clears after correction',async()=>{await act(async()=>root.render(<FormField label="Email" errorMessage="Invalid email" aria-describedby="external-help"/>));expect(capture.input.error).toBe(true);expect(capture.input['aria-invalid']).toBe(true);expect(capture.input['aria-describedby']).toBe(`external-help ${capture.support.nativeID}`);expect(capture.support.type).toBe('error');expect(capture.support.accessibilityRole).toBe('alert');expect(capture.support.children).toBe('Invalid email');await act(async()=>root.render(<FormField label="Email" aria-describedby="external-help"/>));expect(capture.input.error).toBe(false);expect(capture.input['aria-describedby']).toBe('external-help')});
 it('help is informative rather than an error alert',async()=>{await act(async()=>root.render(<FormField label="Email" helpText="Use your work address"/>));expect(capture.support.type).toBe('info');expect(capture.support.accessibilityRole).toBeUndefined();expect(capture.input['aria-describedby']).toBe(capture.support.nativeID);expect(capture.input['aria-invalid']).toBe(false)});
});
