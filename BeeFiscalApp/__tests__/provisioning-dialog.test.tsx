jest.mock('react-native',()=>({View:'View'}));
jest.mock('react-native-paper',()=>({Portal:'Portal',Modal:'Modal',Text:'Text',TextInput:'Input',Button:'Button',useTheme:()=>({colors:{surface:'#fff'}})}));
import React from 'react';import {act,create} from 'react-test-renderer';
import {ProvisioningDialog} from '../src/components/ProvisioningDialog';
test('passwords stay masked, clear before async submit and after contextual dismissal',async()=>{
 let tree:any;let submitted:any;const onSubmit=jest.fn(async(settings)=>{submitted=settings});const props={visible:true,busy:false,onCancel:jest.fn(),onSubmit};await act(async()=>{tree=create(<ProvisioningDialog {...props}/>)});
 const inputs=()=>tree.root.findAllByType('Input');expect(inputs()[1].props.secureTextEntry).toBe(true);expect(inputs()[2].props.secureTextEntry).toBe(true);
 await act(async()=>{inputs()[0].props.onChangeText('private-network');inputs()[1].props.onChangeText('wifi-secret');inputs()[2].props.onChangeText('12345678')});await act(async()=>tree.root.findAllByType('Button')[0].props.onPress());
 expect(submitted).toEqual({ssid:'private-network',wifiPassword:'wifi-secret',fiscalPassword:'12345678'});expect(inputs().every((input:any)=>input.props.value==='')).toBe(true);
 await act(async()=>inputs()[1].props.onChangeText('discarded-secret'));await act(async()=>tree.update(<ProvisioningDialog {...props} visible={false}/>));expect(inputs()[1].props.value).toBe('');await act(async()=>tree.unmount());
});
