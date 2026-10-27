let mockExtra:any={isDemo:false,appVariant:'production'};
jest.mock('expo-constants',()=>({__esModule:true,default:{get expoConfig(){return{extra:mockExtra}}}}));
// Authored only; runtime remains behind the global Paper/test gate.
import React from 'react';
import { act, create } from 'react-test-renderer';
import { Linking } from 'react-native';
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
let mockLanguage = 'en';
jest.mock('react-native', () => ({ Platform: { OS: 'ios' }, Linking: { openURL: jest.fn() } }));
jest.mock('react-native-safe-area-context', () => ({ SafeAreaView: 'SafeAreaView' }));
jest.mock('expo-status-bar', () => ({ StatusBar: 'StatusBar' }));
jest.mock('../preferences', () => ({ useUiPreferences: () => ({ language: mockLanguage }) }));
jest.mock('../app-button', () => ({ AppButton: 'Paper.Button' }));
jest.mock('../form-field', () => ({ FormField: 'Paper.Input' }));
jest.mock('../auth-form', () => ({ AuthForm: 'AuthLayout' }));
jest.mock('../responsive-scroll-view', () => ({ ResponsiveScrollView: 'ScrollLayout' }));
jest.mock('../LanguageThemeControls', () => ({ LanguageThemeControls: 'Paper.Preferences' }));
jest.mock('react-native-paper', () => ({ Surface:'Paper.Surface', Card: Object.assign((p: any) => require('react').createElement('Paper.Card', p, p.children), { Content: 'Paper.Content' }), HelperText: 'Paper.HelperText', Text: 'Paper.Text', useTheme: () => ({ dark: false, colors: { background: '#fff', outline: '#777', surface: '#fff' } }) }));
import { AuthScreen } from '../../screens/AuthScreen';
const auth = () => ({ ready: true, email: 'qa@example.org', code: '', temporary: '', tenants: [], error: '', setEmail: jest.fn(), setCode: jest.fn(), requestCode: jest.fn(), verifyCode: jest.fn(), selectTenant: jest.fn() } as any);

test('privacy failure is visible and permits retry', async () => {
  mockLanguage = 'en'; const open = Linking.openURL as jest.Mock;
  open.mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce(undefined);
  let tree: any; await act(async () => { tree = create(<AuthScreen auth={auth()} />); });
  await act(async () => { tree.root.findByProps({ testID: 'auth.privacy' }).props.onPress(); });
  expect(tree.root.findByProps({ testID: 'auth.privacy.error' }).props.children).toBeTruthy();
  expect(tree.root.findByProps({ testID: 'auth.privacy' }).props.disabled).toBe(false);
  await act(async () => { tree.root.findByProps({ testID: 'auth.privacy' }).props.onPress(); });
  expect(tree.root.findAllByProps({ testID: 'auth.privacy.error' })).toHaveLength(0);
  await act(async () => { tree.unmount(); });
});

test('privacy pending excludes duplicates and old locale failure', async () => {
  mockLanguage = 'en'; const open = Linking.openURL as jest.Mock;
  let reject!: (reason: Error) => void;
  open.mockImplementationOnce(() => new Promise((_resolve, fail) => { reject = fail; }));
  const state = auth(); let tree: any;
  await act(async () => { tree = create(<AuthScreen auth={state} />); });
  const old = tree.root.findByProps({ testID: 'auth.privacy' }).props.onPress;
  const before = open.mock.calls.length;
  await act(async () => { old(); old(); }); expect(open.mock.calls.length).toBe(before + 1);
  mockLanguage = 'bg'; await act(async () => { tree.update(<AuthScreen auth={state} />); });
  await act(async () => { reject(new Error('old locale')); old(); });
  expect(tree.root.findAllByProps({ testID: 'auth.privacy.error' })).toHaveLength(0);
  expect(tree.root.findByProps({ testID: 'auth.privacy' }).props.disabled).toBe(false);
  await act(async () => { tree.unmount(); });
});

test('email submit reaches auth command and unmounted privacy action does nothing', async () => {
  mockLanguage = 'en'; const state = auth(); let tree: any;
  await act(async () => { tree = create(<AuthScreen auth={state} />); });
  await act(async () => { tree.root.findByProps({ testID: 'admin-login-email' }).props.onSubmitEditing(); });
  expect(state.requestCode).toHaveBeenCalledTimes(1);
  const old = tree.root.findByProps({ testID: 'auth.privacy' }).props.onPress;
  const before = (Linking.openURL as jest.Mock).mock.calls.length;
  await act(async () => { tree.unmount(); }); await act(async () => { old(); });
  expect((Linking.openURL as jest.Mock).mock.calls.length).toBe(before);
});

test.each(['email','otp','company'])('auth %s stage keeps background on Paper and safe area technical',async stage=>{
 mockLanguage='en';const state=auth();if(stage!=='email')state.temporary='temporary';if(stage==='company')state.tenants=[{tenant_id:'company',name:'Company'}];let tree:any;
 try{await act(async()=>{tree=create(<AuthScreen auth={state}/>)});const surface=tree.root.findByType('Paper.Surface');expect(surface.props.mode).toBe('flat');expect(surface.props.elevation).toBe(0);expect(surface.props.style.backgroundColor).toBe('#fff');expect(tree.root.findByType('SafeAreaView').props.style).toEqual({flex:1});}
 finally{if(tree)await act(async()=>tree.unmount());}
});


test.each(['en','bg','ru'])('server JSON is never rendered in %s auth errors', async language => {
 mockLanguage=language;const state=auth();state.error='{"code":"TEMPORARY_FAILURE","secret":"PRIVATE_TOKEN","details":{"sql":"PRIVATE_SQL_TABLE"}}';let tree:any;
 try {
  await act(async()=>{tree=create(<AuthScreen auth={state}/>)});
  const error=tree.root.findByProps({testID:'auth.error'});
  const {foundationCopy}=require('../../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated');
  expect(error.props.children).toBe(foundationCopy[language].error);
  const rendered=JSON.stringify(tree.toJSON());
  expect(rendered).not.toMatch(/TEMPORARY_FAILURE|PRIVATE_TOKEN|PRIVATE_SQL_TABLE|"sql"/);
  expect(tree.root.findByProps({testID:'admin-login-email'}).props.value).toBe('qa@example.org');
  await act(async()=>tree.root.findByProps({testID:'admin-login-start'}).props.onPress());
  expect(state.requestCode).toHaveBeenCalledTimes(1);
 } finally {if(tree)await act(async()=>tree.unmount());}
});

test.each([{extra:{isDemo:true,appVariant:'demo'},allowed:true},{extra:{isDemo:false,appVariant:'production'},allowed:false},{extra:{isDemo:'true',appVariant:'demo'},allowed:false},{extra:{isDemo:true,appVariant:'production'},allowed:false}])('actual Paper OTP only admits demo for $extra',async({extra,allowed})=>{mockExtra=extra;const state={...auth(),temporary:'challenge',code:'demo'};let tree:any;await act(async()=>{tree=create(<AuthScreen auth={state}/>)});expect(tree.root.findByProps({testID:'admin-login-code'}).props.keyboardType).toBe(allowed?'default':'number-pad');expect(tree.root.findByProps({testID:'admin-login-verify'}).props.disabled).toBe(!allowed);await act(async()=>tree.unmount());mockExtra={isDemo:false,appVariant:'production'};});
