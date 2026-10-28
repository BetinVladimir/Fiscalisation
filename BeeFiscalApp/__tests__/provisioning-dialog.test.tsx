let mockLanguage='en';
jest.mock('react-native',()=>({View:'View',ScrollView:'Scroll',Platform:{OS:'android'},useWindowDimensions:()=>({width:160,height:500})}));
jest.mock('../src/ui/preferences',()=>({useUiPreferences:()=>({language:mockLanguage})}));
jest.mock('../src/ui/app-button',()=>({AppButton:'Button'}));
jest.mock('../../../BeeloyBackend/mobile/shared-ui/src/components/PaperDialogHost',()=>({PaperDialogHost:'Host'}));
jest.mock('react-native-paper',()=>({Dialog:'Dialog',Text:'Text',TextInput:'Input',useTheme:()=>({colors:{elevation:{level3:'#eee'}}})}));
import React from 'react';import {act,create} from 'react-test-renderer';
import {ProvisioningDialog} from '../src/components/ProvisioningDialog';
test('passwords stay masked, clear before async submit and after contextual dismissal',async()=>{
 let tree:any;let submitted:any;const onSubmit=jest.fn(async(settings)=>{submitted={...settings}});const props={visible:true,busy:false,onCancel:jest.fn(),onSubmit};await act(async()=>{tree=create(<ProvisioningDialog {...props}/>)});
 const inputs=()=>tree.root.findAllByType('Input');expect(inputs()[1].props.secureTextEntry).toBe(true);expect(inputs()[2].props.secureTextEntry).toBe(true);
 await act(async()=>{inputs()[0].props.onChangeText('private-network');inputs()[1].props.onChangeText('wifi-secret');inputs()[2].props.onChangeText('12345678')});await act(async()=>tree.root.findAllByType('Button')[0].props.onPress());
 expect(submitted).toEqual({ssid:'private-network',wifiPassword:'wifi-secret',fiscalPassword:'12345678'});expect(inputs().every((input:any)=>input.props.value==='')).toBe(true);
 await act(async()=>inputs()[1].props.onChangeText('discarded-secret'));await act(async()=>tree.update(<ProvisioningDialog {...props} visible={false}/>));expect(inputs()[1].props.value).toBe('');await act(async()=>tree.unmount());
});

test('same-event double submit and pending cancel do not duplicate a command',async()=>{
 let tree:any,release!:()=>void,secret:any;const gate=new Promise<void>(r=>release=r),onSubmit=jest.fn(async(settings)=>{secret=settings;await gate}),onCancel=jest.fn();
 try{await act(async()=>{tree=create(<ProvisioningDialog visible busy={false} onSubmit={onSubmit} onCancel={onCancel}/>)});await act(async()=>tree.root.findAllByType('Input')[1].props.onChangeText('SYNTHETIC_SECRET'));
 const buttons=tree.root.findAllByType('Button');await act(async()=>{buttons[0].props.onPress();buttons[0].props.onPress();buttons[1].props.onPress()});expect(onSubmit).toHaveBeenCalledTimes(1);expect(onCancel).not.toHaveBeenCalled();expect(secret.wifiPassword).toBe('SYNTHETIC_SECRET');release();await act(async()=>gate);expect(secret).toEqual({ssid:'',wifiPassword:'',fiscalPassword:''});
 }finally{release?.();await act(async()=>tree?.unmount())}
});
test('retired callbacks cannot edit, cancel or submit after hide and unmount',async()=>{
 let tree:any;const onSubmit=jest.fn(async()=>{}),onCancel=jest.fn(),props={visible:true,busy:false,onSubmit,onCancel};await act(async()=>{tree=create(<ProvisioningDialog {...props}/>)});const input=tree.root.findAllByType('Input')[1].props.onChangeText,submit=tree.root.findAllByType('Button')[0].props.onPress,cancel=tree.root.findAllByType('Button')[1].props.onPress;
 await act(async()=>tree.update(<ProvisioningDialog {...props} visible={false}/>));await act(async()=>{input('RETIRED');submit();cancel()});expect(tree.root.findAllByType('Input')[1].props.value).toBe('');expect(onSubmit).not.toHaveBeenCalled();expect(onCancel).not.toHaveBeenCalled();await act(async()=>tree.unmount());await act(async()=>{input('RETIRED');submit();cancel()});expect(onSubmit).not.toHaveBeenCalled();expect(onCancel).not.toHaveBeenCalled();
});
test('context remount discards draft and old pending completion does not touch replacement',async()=>{
 let tree:any,release!:()=>void;const gate=new Promise<void>(r=>release=r),onSubmit=jest.fn(async()=>gate),props={visible:true,busy:false,onSubmit,onCancel:jest.fn()};try{await act(async()=>{tree=create(<ProvisioningDialog key="A" {...props}/>)});await act(async()=>tree.root.findAllByType('Input')[0].props.onChangeText('A_NETWORK'));await act(async()=>tree.root.findAllByType('Button')[0].props.onPress());await act(async()=>tree.update(<ProvisioningDialog key="B" {...props}/>));expect(tree.root.findAllByType('Input').every((x:any)=>x.props.value==='')).toBe(true);await act(async()=>tree.root.findAllByType('Input')[0].props.onChangeText('B_NETWORK'));release();await act(async()=>gate);expect(tree.root.findAllByType('Input')[0].props.value).toBe('B_NETWORK');}finally{release?.();await act(async()=>tree?.unmount())}
});

test.each(['en','bg','ru'])('localized scrollable Paper dialog keeps all three fields and actions in %s',async language=>{
 mockLanguage=language;let tree:any;try{await act(async()=>{tree=create(<ProvisioningDialog visible busy={false} onSubmit={async()=>{}} onCancel={()=>{}}/>)});const {provisioningCopy}=await import('../src/ui/provisioning-copy');const copy=provisioningCopy(language);expect(tree.root.findByProps({testID:'provisioning.dialog'}).props['aria-label']).toBe(copy.title);expect(tree.root.findAllByType('Input').map((n:any)=>n.props.accessibilityLabel)).toEqual([copy.ssid,copy.wifiPassword,copy.fiscalPassword]);expect(tree.root.findAllByType('Button').map((n:any)=>n.props.children)).toEqual([copy.submit,copy.cancel]);expect(tree.root.findAllByType('Text').every((n:any)=>n.props.style.backgroundColor==='#eee'&&n.props.style.paddingRight===4)).toBe(true);expect(tree.root.findByType('Dialog').props.style.width).toBe(128);expect(tree.root.findByProps({testID:'provisioning.scroll'}).props.style.maxHeight).toBe(468);}finally{await act(async()=>tree?.unmount());mockLanguage='en'}
});
test('runtime copy matches actual model responses plus explicit reviewed overrides',async()=>{
 const fs=await import('node:fs'),path=await import('node:path'),crypto=await import('node:crypto'),{provisioningCopies}=await import('../src/ui/provisioning-copy');const dir=path.join(process.cwd(),'src/ui/provisioning-copy');const read=(n:string)=>JSON.parse(fs.readFileSync(path.join(dir,n),'utf8'));const en=read('en.json'),overrides=read('editorial-overrides.json');expect(provisioningCopies.en).toEqual(en);expect(Object.keys(en)).toHaveLength(7);for(const locale of ['bg','ru'] as const){expect(read(locale+'-request.json').model).toBe('translategemma:12b');const raw=JSON.parse(read(locale+'-response.json').response);expect(Object.keys(raw).sort()).toEqual(Object.keys(en).sort());expect(provisioningCopies[locale]).toEqual({...raw,...overrides[locale]});}expect(read('provenance.json').sourceSha256).toBe(crypto.createHash('sha256').update(fs.readFileSync(path.join(dir,'en.json'))).digest('hex'));
});
