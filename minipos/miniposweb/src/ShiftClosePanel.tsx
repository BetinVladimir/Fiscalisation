import {useShiftDialogAccessibility} from './ui/useShiftDialogAccessibility';
import {useLayoutEffect,useRef,useState} from 'react';
import {Card,Dialog,Portal,Text} from 'react-native-paper';
import {AppButton} from './ui/app-button';
import {shiftCloseAction} from './ui/shift-close-state';
import copy from './ui/shift-close-copy.json';
import {operationOutcomeCopyFor} from './ui/operation-outcome-copy';
import type {Shift} from './api/client';
export function ShiftClosePanel({shift,language,busy,blocked,owner,onSubmit,onRefresh}:{shift:Shift;language:string;busy:boolean;blocked:boolean;owner:unknown;onSubmit:()=>void;onRefresh:()=>void}){
 const t=copy[language as keyof typeof copy]??copy.en,action=shiftCloseAction(shift);
 const [prompt,setPrompt]=useState<unknown>(null),render={},committed=useRef<object|null>(null),consumed=useRef(false);
 useLayoutEffect(()=>{committed.current=render;return()=>{if(committed.current===render)committed.current=null}});
 const allowed=()=>committed.current===render&&!busy&&!blocked&&Boolean(action);
 const visible=prompt===owner;
 useShiftDialogAccessibility(visible,'shift-close-dialog',busy,()=>setPrompt(null));
 const label=action==='reconcile'?t.reconcileZ:t.closeShift;
 const outcomeCopy=operationOutcomeCopyFor(language),description=action==='reconcile'?outcomeCopy.reconcileShiftBody:outcomeCopy.closeShiftBody;
 return <Card mode="outlined" style={{marginTop:16}} testID="shift-close-panel"><Card.Content style={{gap:12}}>
 <Text variant="titleMedium">{action?label:t.resultChecking}</Text>
 <AppButton mode="outlined" testID="shift-close-open" disabled={busy||blocked||!action} onPress={()=>{if(allowed()){consumed.current=false;setPrompt(owner)}}}>{label}</AppButton>
 {shift.state!=='OPEN'?<AppButton mode="outlined" testID="shift-close-refresh" disabled={busy||blocked} onPress={()=>{if(committed.current===render&&!busy&&!blocked)onRefresh()}}>{t.checkResult}</AppButton>:null}
 {visible?<Portal><Dialog visible style={{width:'90%',maxWidth:560,alignSelf:'center',marginHorizontal:0}} onDismiss={()=>{if(!busy)setPrompt(null)}} testID="shift-close-dialog"><Dialog.Title testID="shift-close-dialog-title">{label}</Dialog.Title><Dialog.Content><Text testID="shift-close-dialog-description">{description}</Text></Dialog.Content><Dialog.Actions>
 <AppButton mode="outlined" disabled={busy} onPress={()=>setPrompt(null)}>{t.back}</AppButton>
 <AppButton mode="contained" testID="shift-close-confirm" disabled={busy||blocked||!action} onPress={()=>{if(allowed()&&!consumed.current){consumed.current=true;setPrompt(null);onSubmit()}}}>{label}</AppButton>
 </Dialog.Actions></Dialog></Portal>:null}
 </Card.Content></Card>;
}
