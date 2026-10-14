// Authored only; execution deferred until the global gate.
import React from 'react';import {act,create} from 'react-test-renderer';
let mockHydrated=false,mockFonts=false,mockFontError:Error|null=null;
let mockMotionListener:(value:boolean)=>void=()=>undefined;
const mockMotionRead=jest.fn(()=>Promise.resolve(false));
const mockRemove=jest.fn();
jest.mock('react-native',()=>({Platform:{OS:'ios'},useColorScheme:()=> 'dark',AccessibilityInfo:{addEventListener:(_event:string,listener:(value:boolean)=>void)=>{mockMotionListener=listener;return {remove:mockRemove}},isReduceMotionEnabled:()=>mockMotionRead()}}));
jest.mock('react-native-paper',()=>({PaperProvider:(props:any)=>require('react').createElement('Paper.Provider',props,props.children),Surface:'Paper.Surface',ActivityIndicator:'Paper.Indicator'}));
jest.mock('react-native-safe-area-context',()=>({SafeAreaProvider:({children}:any)=>children}));
jest.mock('@expo-google-fonts/roboto',()=>({useFonts:()=>[mockFonts,mockFontError]}));
jest.mock('../src/ui/theme',()=>({BeeloyDarkTheme:{colors:{background:'#141218'},animation:{scale:1}},BeeloyLightTheme:{colors:{background:'#fff'},animation:{scale:1}}}));
jest.mock('../src/ui/feedback',()=>({FeedbackProvider:({children}:any)=>children}));
jest.mock('../src/ui/preferences',()=>({UiPreferencesProvider:({children}:any)=>children,useUiPreferences:()=>({themeMode:'dark',hydrated:mockHydrated,language:'ru'})}));
jest.mock('../../../BeeloyBackend/mobile/shared-ui/src/theme/paper',()=>({withSystemFonts:(theme:any)=>theme}));
import {BeeloyAppProvider} from '../src/ui/provider';
import {foundationCopy} from '../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
test.each([[false,false,false],[false,true,false],[true,false,false],[true,true,true]] as const)('hydrated=%s fonts=%s content=%s',async(hydrated,fonts,content)=>{mockHydrated=hydrated;mockFonts=fonts;mockFontError=null;let tree:any;try{await act(async()=>{tree=create(<BeeloyAppProvider>{React.createElement('Child')}</BeeloyAppProvider>)});expect(tree.root.findAllByType('Child')).toHaveLength(content?1:0);if(!content){expect(tree.root.findByType('Paper.Surface').props.style.backgroundColor).toBe('#141218');expect(tree.root.findByType('Paper.Indicator').props.accessibilityLabel).toBe(foundationCopy.ru.loading);}}finally{if(tree)await act(async()=>tree.unmount());mockHydrated=false;mockFonts=false;}});
test.each([false,true])('font error still respects preferences hydration=%s',async(hydrated)=>{mockHydrated=hydrated;mockFonts=false;mockFontError=new Error('font unavailable');let tree:any;try{await act(async()=>{tree=create(<BeeloyAppProvider>{React.createElement('Child')}</BeeloyAppProvider>)});expect(tree.root.findAllByType('Child')).toHaveLength(hydrated?1:0);}finally{if(tree)await act(async()=>tree.unmount());mockHydrated=false;mockFontError=null;}});

test('newer native reduced-motion event wins over delayed initial read',async()=>{
 let resolve!:(value:boolean)=>void;mockMotionRead.mockReturnValueOnce(new Promise<boolean>(r=>{resolve=r}));let tree:any;
 try{await act(async()=>{tree=create(<BeeloyAppProvider/>)});await act(async()=>mockMotionListener(true));expect(tree.root.findByType('Paper.Provider').props.theme.animation.scale).toBe(0);expect(tree.root.findByType('Paper.Provider').props.settings.rippleEffectEnabled).toBe(false);await act(async()=>resolve(false));expect(tree.root.findByType('Paper.Provider').props.theme.animation.scale).toBe(0);
 await act(async()=>mockMotionListener(false));expect(tree.root.findByType('Paper.Provider').props.theme.animation.scale).toBe(1);
 }finally{if(tree)await act(async()=>tree.unmount());}
});
test('native motion subscription is removed and retired callback is ignored',async()=>{let tree:any;await act(async()=>{tree=create(<BeeloyAppProvider/>)});const retired=mockMotionListener;await act(async()=>tree.unmount());expect(mockRemove).toHaveBeenCalled();await act(async()=>retired(true));});

test.each(['no-window','no-matchMedia'])('web startup tolerates %s',async(mode)=>{const native=jest.requireMock('react-native'),original=Object.getOwnPropertyDescriptor(globalThis,'window');native.Platform.OS='web';if(mode==='no-window')Reflect.deleteProperty(globalThis,'window');else Object.defineProperty(globalThis,'window',{configurable:true,value:{}});let tree:any;try{await act(async()=>{tree=create(<BeeloyAppProvider/>)});expect(tree.root.findByType('Paper.Provider').props.theme.animation.scale).toBe(1);}finally{if(tree)await act(async()=>tree.unmount());native.Platform.OS='ios';if(original)Object.defineProperty(globalThis,'window',original);else Reflect.deleteProperty(globalThis,'window');}});

test('legacy web motion listener updates and is released',async()=>{const native=jest.requireMock('react-native'),original=Object.getOwnPropertyDescriptor(globalThis,'window');native.Platform.OS='web';let listener!:()=>void;const remove=jest.fn(),query={matches:false,addListener:jest.fn((callback:()=>void)=>{listener=callback}),removeListener:remove};Object.defineProperty(globalThis,'window',{configurable:true,value:{matchMedia:()=>query}});let tree:any;try{await act(async()=>{tree=create(<BeeloyAppProvider/>)});expect(query.addListener).toHaveBeenCalledTimes(1);query.matches=true;await act(async()=>listener());expect(tree.root.findByType('Paper.Provider').props.theme.animation.scale).toBe(0);await act(async()=>tree.unmount());tree=null;expect(remove).toHaveBeenCalledWith(listener);await act(async()=>listener());}finally{if(tree)await act(async()=>tree.unmount());native.Platform.OS='ios';if(original)Object.defineProperty(globalThis,'window',original);else Reflect.deleteProperty(globalThis,'window');}});
