// Authored only; execution deferred until the global gate.
import React from 'react';import {act,create} from 'react-test-renderer';
jest.mock('react-native',()=>({Platform:{OS:'ios'}}));
jest.mock('expo-secure-store',()=>({getItemAsync:jest.fn(),setItemAsync:jest.fn(async()=>undefined)}));
import * as SecureStore from 'expo-secure-store';import {UiPreferencesProvider,useUiPreferences} from '../src/ui/preferences';
let latest:ReturnType<typeof useUiPreferences>;
function Child(){latest=useUiPreferences();return React.createElement('Child');}
beforeEach(()=>jest.clearAllMocks());
test('waits for storage before persisting and validates runtime selections',async()=>{let resolve!:(v:string)=>void;(SecureStore.getItemAsync as jest.Mock).mockReturnValue(new Promise<string>(r=>{resolve=r}));let tree:any;try{await act(async()=>{tree=create(<UiPreferencesProvider><Child/></UiPreferencesProvider>)});expect(latest.hydrated).toBe(false);expect(SecureStore.setItemAsync).not.toHaveBeenCalled();await act(async()=>resolve(JSON.stringify({language:'ru',themeMode:'dark'})));expect(latest.hydrated).toBe(true);expect(latest.language).toBe('ru');expect(latest.themeMode).toBe('dark');await act(async()=>{latest.setLanguage('constructor' as any);latest.setThemeMode('invalid' as any)});expect(latest.language).toBe('ru');expect(latest.themeMode).toBe('dark');}finally{if(tree)await act(async()=>tree.unmount());}});
test('late hydration from an unmounted owner cannot persist settings',async()=>{let resolve!:(v:string)=>void;(SecureStore.getItemAsync as jest.Mock).mockReturnValue(new Promise<string>(r=>{resolve=r}));let tree:any;await act(async()=>{tree=create(<UiPreferencesProvider><Child/></UiPreferencesProvider>)});await act(async()=>tree.unmount());await act(async()=>resolve(JSON.stringify({language:'en',themeMode:'light'})));expect(SecureStore.setItemAsync).not.toHaveBeenCalled();});
