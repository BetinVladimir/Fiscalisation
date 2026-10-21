import React from 'react';
import {act,create,type ReactTestRenderer} from 'react-test-renderer';
import {useOwnedDeviceValue} from '../src/useOwnedDeviceValue';
let current:any;let tree:ReactTestRenderer;
function Host({owner,initial=null}:{owner:string;initial?:any}){const [value,set]=useOwnedDeviceValue(owner,initial);current={value,set};return <output>{JSON.stringify(value)}</output>}
async function mount(initial:any=null){await act(async()=>{tree=create(<Host owner="A" initial={initial}/>)});}
async function owner(key:string){await act(async()=>tree.update(<Host owner={key}/>));}
afterEach(async()=>{await act(async()=>tree?.unmount())});
test('completed result is hidden immediately on owner change',async()=>{await mount();await act(async()=>current.set({ticket:'A_ONLY'}));expect(current.value).toEqual({ticket:'A_ONLY'});await owner('B');expect(current.value).toBeNull();});
test('held old-owner completion cannot overwrite new result',async()=>{await mount();const old=current.set;await owner('B');await act(async()=>current.set({ticket:'B_ONLY'}));await act(async()=>old({ticket:'LATE_A'}));expect(current.value).toEqual({ticket:'B_ONLY'});});
test('A to B to A does not resurrect data or revive old callback',async()=>{await mount();const old=current.set;await act(async()=>old('FIRST_A'));await owner('B');await owner('A');expect(current.value).toBeNull();await act(async()=>old('LATE_FIRST_A'));expect(current.value).toBeNull();await act(async()=>current.set('NEW_A'));expect(current.value).toBe('NEW_A');});
test('same-owner render and poll retain result and updater semantics',async()=>{await mount([]);await act(async()=>current.set(['one']));await owner('A');expect(current.value).toEqual(['one']);await act(async()=>current.set((rows:string[])=>[...rows,'two']));expect(current.value).toEqual(['one','two']);});
test('new-owner updater starts from empty value, never old owner data',async()=>{await mount([]);await act(async()=>current.set(['A']));await owner('B');await act(async()=>current.set((rows:string[])=>[...rows,'B']));expect(current.value).toEqual(['B']);});
test('retired updater is never invoked after owner change or unmount',async()=>{await mount();const stale=current.set,updater=jest.fn(()=> 'BAD');await owner('B');await act(async()=>stale(updater));expect(updater).not.toHaveBeenCalled();const last=current.set;await act(async()=>tree.unmount());await act(async()=>last(updater));expect(updater).not.toHaveBeenCalled();});
test.each(['auth','register','device','tab'])('%s owner invalidation clears completed data',async kind=>{await mount();await owner(JSON.stringify({auth:'A',register:'R1',device:'D1',tab:'devices'}));await act(async()=>current.set({private:'old'}));await owner(JSON.stringify({auth:'A',register:'R1',device:'D1',tab:'devices',[kind]:'changed'}));expect(current.value).toBeNull();});
