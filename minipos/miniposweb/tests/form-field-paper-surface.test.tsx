// Authored only; do not execute before the global Paper/test-writing gate.
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import {describe,it,expect,vi} from 'vitest';
vi.mock('react-native',()=>({Platform:{OS:'web'}}));
vi.mock('react-native-paper',()=>({
 Surface:({children,style,mode,elevation}:any)=><section data-paper="surface" data-mode={mode} data-elevation={elevation} style={Object.assign({},...style)}>{children}</section>,
 Text:({children,nativeID}:any)=><span id={nativeID}>{children}</span>,
 TextInput:(props:any)=><input aria-label={props.accessibilityLabel} aria-labelledby={props['aria-labelledby']} aria-describedby={props['aria-describedby']} aria-invalid={props['aria-invalid']}/>,
 HelperText:({children,nativeID,accessibilityRole}:any)=><span id={nativeID} role={accessibilityRole}>{children}</span>,
 useTheme:()=>({colors:{onSurface:'#111'}}),
}));
import {FormField} from '../src/ui/form-field';
describe('Paper FormField decoration',()=>{
 it('keeps caller borders/background on genuine flat Paper while preserving error semantics',()=>{
  const html=renderToStaticMarkup(<FormField label="Email" errorMessage="Invalid email" containerStyle={{backgroundColor:'#fff',borderWidth:1}}/>);
  expect(html).toContain('data-paper="surface"');expect(html).toContain('data-mode="flat"');expect(html).toContain('data-elevation="0"');expect(html).toContain('background-color:#fff');expect(html).toContain('border-width:1px');expect(html).toContain('aria-invalid="true"');expect(html).toContain('role="alert"');expect(html).toContain('aria-describedby=');expect(html).toContain('aria-labelledby=');
 });
});
