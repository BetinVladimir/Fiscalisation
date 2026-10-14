// @vitest-environment jsdom
// Authored only; execution deferred until global gate.
import React,{act} from 'react';import {createRoot,type Root} from 'react-dom/client';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
const mock=vi.hoisted(()=>({background:'#fffbff',surfaces:[] as any[]}));
vi.mock('react-native-paper',()=>({Surface:(props:any)=>{mock.surfaces.push(props);return <div data-testid={props.testID}>{props.children}</div>},Text:({children}:any)=><span>{children}</span>,HelperText:({children}:any)=><span>{children}</span>,Card:Object.assign(({children}:any)=><div>{children}</div>,{Content:({children}:any)=><div>{children}</div>}),useTheme:()=>({colors:{background:mock.background,surface:'#fff'}})}));
vi.mock('../src/ui/PreferenceControls',()=>({readLanguage:()=> 'en',PreferenceControls:()=>null}));
vi.mock('../src/ui/app-button',()=>({AppButton:({children}:any)=><button>{children}</button>}));
vi.mock('../src/ui/form-field',()=>({FormField:({accessibilityLabel}:any)=><input aria-label={accessibilityLabel}/>}));
vi.mock('../src/api/client',()=>({EmailAuthClient:class{}}));
import {AuthFlow} from '../src/AuthFlow';
let root:Root,host:HTMLDivElement;
beforeEach(()=>{(globalThis as any).IS_REACT_ACT_ENVIRONMENT=true;mock.surfaces=[];host=document.createElement('div');document.body.append(host);root=createRoot(host)});
afterEach(async()=>{await act(async()=>root.unmount());host.remove()});
it.each(['#fffbff','#141218'])('auth page owns genuine Paper surface %s',async background=>{mock.background=background;await act(async()=>root.render(<AuthFlow backend="https://example.test" onAuthenticated={()=>{}}/>));const surface=mock.surfaces.find(item=>item.testID==='minipos.auth.surface');expect(surface.mode).toBe('flat');expect(surface.elevation).toBe(0);expect(surface.style.backgroundColor).toBe(background);expect(surface.style.minWidth).toBe(0);expect(host.querySelector('main.auth-shell')).not.toBeNull();});

import {AppFrame} from '../src/ui/app-frame';
it.each(['#fffbff','#141218'])('working and recovery frame use Paper background %s',async background=>{mock.background=background;await act(async()=>root.render(<AppFrame testID="frame"><main>Content</main></AppFrame>));const surface=mock.surfaces.find(item=>item.testID==='frame');expect(surface.mode).toBe('flat');expect(surface.elevation).toBe(0);expect(surface.style.backgroundColor).toBe(background);expect(surface.style.minWidth).toBe(0);expect(host.querySelector('main')?.textContent).toBe('Content');});
