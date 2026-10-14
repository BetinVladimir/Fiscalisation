// Authored only; do not execute before global gate.
import AsyncStorage from '@react-native-async-storage/async-storage';
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{getItem:jest.fn(async()=>null),setItem:jest.fn(async()=>undefined),removeItem:jest.fn(async()=>undefined)}}));
import {useLanguageStore,isSupportedLanguage} from '../src/languageStore';import {supportedLanguages} from '../src/translations.generated';
test.each([null,[],{}, {language:'constructor'}, {language:'__proto__'}, {language:'zz'}, {language:12}, {language:null}])('malformed persisted value cannot replace supported locale: %j',async(value)=>{
 useLanguageStore.setState({language:'en'});(AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(JSON.stringify({state:value,version:0}));await useLanguageStore.persist.rehydrate();expect(useLanguageStore.getState().language).toBe('en');expect(typeof useLanguageStore.getState().setLanguage).toBe('function');
});
test.each(supportedLanguages)('restores supported language %s without replacing actions',async(language)=>{useLanguageStore.setState({language:'en'});const action=useLanguageStore.getState().setLanguage;(AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(JSON.stringify({state:{language,setLanguage:'invalid'},version:0}));await useLanguageStore.persist.rehydrate();expect(useLanguageStore.getState().language).toBe(language);expect(useLanguageStore.getState().setLanguage).toBe(action);});
test('runtime invalid selection is ignored',()=>{useLanguageStore.setState({language:'en'});useLanguageStore.getState().setLanguage('constructor' as any);expect(useLanguageStore.getState().language).toBe('en');expect(isSupportedLanguage('__proto__')).toBe(false);});
