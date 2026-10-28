import React,{Suspense,startTransition} from 'react';
import {act,create,type ReactTestRenderer} from 'react-test-renderer';
jest.mock('react-native',()=>({Platform:{OS:'ios'},View:'View'}));
jest.mock('react-native-paper',()=>({Card:Object.assign('Paper.Card',{Content:'Paper.Card.Content'}),HelperText:'Paper.HelperText',RadioButton:{Item:'Paper.Radio'},Text:'Paper.Text',useTheme:()=>({colors:{outline:'#333',surface:'#fff',secondaryContainer:'#ddd'}})}));
jest.mock('../src/ui/preferences',()=>({useUiPreferences:()=>({language:'en'})}));
jest.mock('../src/ui/app-button',()=>({AppButton:'Paper.Button'}));
jest.mock('../src/ui/form-field',()=>({FormField:'Paper.Field'}));
import {Choices,ReferencePicker} from '../src/components/FiscalControls';
const items=[{id:'A'},{id:'B'}];
const props={items,labelFor:(item:Record<string,unknown>)=>String(item.id)};

test.each(['reference','choices'])('%s old and unmounted callbacks cannot select after commit',kind=>{
 const select=jest.fn();let tree!:ReactTestRenderer;
 const render=(disabled=false)=>kind==='reference'?<ReferencePicker {...props} label="Pick" value="A" onSelect={select} disabled={disabled}/>:<Choices {...props} selectedId="A" onSelect={select} disabled={disabled}/>;
 act(()=>{tree=create(render())});const old=tree.root.findAllByType('Paper.Radio' as never)[1].props.onPress;
 act(()=>tree.update(render(true)));act(()=>old());expect(select).not.toHaveBeenCalled();
 act(()=>tree.update(render()));const current=tree.root.findAllByType('Paper.Radio' as never)[1].props.onPress;act(()=>current());expect(select).toHaveBeenCalledTimes(1);
 act(()=>tree.unmount());act(()=>{current();old()});expect(select).toHaveBeenCalledTimes(1);
});

test('interrupted render does not retire the still-committed visible control',async()=>{
 const select=jest.fn();let tree!:ReactTestRenderer,suspend=false;const pending=new Promise<void>(()=>{});
 function Gate(){if(suspend)throw pending;return null;}
 function Screen({value}:{value:string}){return <Suspense fallback={<span/>}><ReferencePicker {...props} label="Pick" value={value} onSelect={select}/><Gate/></Suspense>}
 await act(async()=>{tree=create(<Screen value="A"/>,{unstable_isConcurrent:true} as never)});
 const current=tree.root.findAllByType('Paper.Radio' as never)[1].props.onPress;
 suspend=true;await act(async()=>{startTransition(()=>tree.update(<Screen value="B"/>))});
 act(()=>current());expect(select).toHaveBeenCalledWith('B');
 suspend=false;await act(async()=>tree.update(<Screen value="B"/>));select.mockClear();act(()=>current());expect(select).not.toHaveBeenCalled();
 act(()=>tree.unmount());
});
