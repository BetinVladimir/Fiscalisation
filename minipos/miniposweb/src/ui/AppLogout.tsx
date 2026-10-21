import {useLayoutEffect,useRef,useState} from 'react';
import {Dialog,Portal,Text} from 'react-native-paper';
import {AppButton} from './app-button';
import {useShiftDialogAccessibility} from './useShiftDialogAccessibility';
/** Dirty sale confirmation is separate from the registered editor leave guard. */
export function AppLogout({owner,blocked,dirty,label,copy,onLogout}:{owner:object;blocked:boolean;dirty:boolean;label:string;copy:{cancelMessage:string;keepEditing:string};onLogout:()=>void}){
 const[prompt,setPrompt]=useState<object|null>(null),consumed=useRef(false),render={},committed=useRef<object|null>(null);
 useLayoutEffect(()=>{committed.current=render;consumed.current=false;return()=>{if(committed.current===render)committed.current=null}});
 const allowed=()=>committed.current===render&&!blocked,visible=prompt===owner;
 useShiftDialogAccessibility(visible,'minipos-logout-dialog',blocked,()=>setPrompt(null));
 const submit=()=>{if(!allowed()||consumed.current)return;consumed.current=true;setPrompt(null);onLogout()};
 return <><AppButton testID="app-logout" mode="outlined" disabled={blocked} onPress={()=>{if(!allowed())return;if(dirty)setPrompt(owner);else submit()}}>{label}</AppButton>
 {visible?<Portal><Dialog testID="minipos-logout-dialog" visible onDismiss={()=>{if(!blocked)setPrompt(null)}} style={{width:'90%',maxWidth:560,alignSelf:'center',marginHorizontal:0}}>
 <Dialog.Title testID="minipos-logout-dialog-title">{label}</Dialog.Title><Dialog.Content><Text testID="minipos-logout-dialog-description">{copy.cancelMessage}</Text></Dialog.Content><Dialog.Actions>
 <AppButton testID="app-logout-stay" mode="outlined" disabled={blocked} onPress={()=>{if(allowed())setPrompt(null)}}>{copy.keepEditing}</AppButton>
 <AppButton testID="app-logout-confirm" mode="contained" disabled={blocked} onPress={submit}>{label}</AppButton>
 </Dialog.Actions></Dialog></Portal>:null}</>;
}
