import React,{useCallback,useLayoutEffect,useMemo,useRef,useState} from 'react';
import {Platform,ScrollView} from 'react-native';
import {useHeaderMetrics} from './PaperOperationalHeader';
import {Button,Dialog,Portal,Text} from 'react-native-paper';
export type HeaderDraft={dirty:()=>boolean;pending:()=>boolean;discard:()=>void};
export function useHeaderLeaveGuard(owner:unknown,blocked:()=>boolean){
 const epoch=useMemo(()=>({sources:new Map<object,HeaderDraft>()}),[owner]),active=useRef<object|null>(null),latestBlocked=useRef(blocked);
 const [prompt,setPrompt]=useState<{epoch:object;sources:Map<object,HeaderDraft>;go:()=>void}|null>(null);
 useLayoutEffect(()=>{active.current=epoch;latestBlocked.current=blocked;return()=>{if(active.current===epoch){active.current=null;epoch.sources.clear();}};},[epoch]);
 useLayoutEffect(()=>{latestBlocked.current=blocked;});
 const intent=useRef<typeof prompt>(null);
 useLayoutEffect(()=>{if(intent.current&&intent.current.epoch!==epoch)intent.current=null;},[epoch]);
 const register=useCallback((source:HeaderDraft)=>{const key={};epoch.sources.set(key,source);return()=>{epoch.sources.delete(key);};},[epoch]);
 const render={},committed=useRef<object|null>(null);useLayoutEffect(()=>{committed.current=render;return()=>{if(committed.current===render)committed.current=null;};});
 const allowed=()=>committed.current===render&&active.current===epoch&&!latestBlocked.current()&&[...epoch.sources.values()].every(s=>!s.pending());
 const discarding=useRef(false);
 const request=(go:()=>void)=>{if(discarding.current||!allowed())return;
 // First admitted dirty intent wins even before React commits the prompt.
 const previous=intent.current;
 if(previous&&previous.epoch===epoch&&previous.sources.size===epoch.sources.size&&[...previous.sources].every(([key,value])=>epoch.sources.get(key)===value))return;
 const current=new Map(epoch.sources);if([...current.values()].some(s=>s.dirty())){const next={epoch,sources:current,go};intent.current=next;setPrompt(next);}else {intent.current=null;setPrompt(null);go();}};
 const samePrompt=()=>Boolean(prompt&&prompt.epoch===epoch&&allowed()&&prompt.sources.size===epoch.sources.size&&[...prompt.sources].every(([key,value])=>epoch.sources.get(key)===value));
 const consumed=useRef<object|null>(null);
 const discard=()=>{if(!samePrompt()||!prompt||consumed.current===prompt)return;consumed.current=prompt;if(intent.current===prompt)intent.current=null;setPrompt(null);discarding.current=true;try{for(const source of prompt.sources.values()){source.discard();if(!samePrompt())return;}if(samePrompt())prompt.go();}finally{discarding.current=false;}};
 const stay=()=>{if(committed.current===render&&active.current===epoch&&prompt?.epoch===epoch){consumed.current=prompt;intent.current=null;setPrompt(null);}};
 const discardDisabled=blocked()||[...epoch.sources.values()].some(source=>source.pending());
 return {register,request,discard,stay,canEdit:()=>allowed()&&!discarding.current&&intent.current?.epoch!==epoch,isBlocked:()=>!allowed()||discarding.current||intent.current?.epoch===epoch,discardDisabled,visible:Boolean(prompt&&prompt.epoch===epoch)};
}
export function HeaderLeaveDialog({guard,copy}:{guard:ReturnType<typeof useHeaderLeaveGuard>;copy:{cancelTitle:string;cancelMessage:string;keepEditing:string;discard:string}}){
 const metrics=useHeaderMetrics();
 const insets={top:0,bottom:0,left:0,right:0};
 const maxHeight=Math.max(0,metrics.height-insets.top-insets.bottom-32);
 const width=Math.max(0,Math.min(560,metrics.width-insets.left-insets.right-24));
 const wrap={minWidth:0,flexShrink:1,...(Platform.OS==='web'?{whiteSpace:'normal' as const,overflowWrap:'anywhere' as const}:{})}; 
 // Mount the Paper portal only for an active confirmation; a closed dialog must not leave a full-page overlay.
 if(!guard.visible)return null;
 return <Portal><Dialog testID="header.leave.dialog" visible={guard.visible} onDismiss={guard.stay} style={{maxHeight,width,alignSelf:'center',marginHorizontal:0,marginVertical:16}}>
 <ScrollView testID="header.leave.scroll" style={{maxHeight,flexShrink:1}} contentContainerStyle={{paddingBottom:8}} keyboardShouldPersistTaps="handled" nestedScrollEnabled>
 <Dialog.Title style={wrap}>{copy.cancelTitle}</Dialog.Title><Dialog.Content><Text style={wrap}>{copy.cancelMessage}</Text></Dialog.Content>
 <Dialog.Actions style={{flexDirection:'column',alignItems:'stretch',flexWrap:'wrap',gap:8,paddingHorizontal:16}}>
 <Button testID="header.leave.stay" mode="outlined" labelNumberOfLines={0} labelStyle={{...wrap,flexShrink:1}} contentStyle={{minHeight:48}} style={{width:'100%',marginHorizontal:0}} onPress={guard.stay}>{copy.keepEditing}</Button>
 <Button testID="header.leave.discard" mode="contained" labelNumberOfLines={0} labelStyle={{...wrap,flexShrink:1}} contentStyle={{minHeight:48}} style={{width:'100%',marginHorizontal:0}} disabled={guard.discardDisabled} onPress={guard.discard}>{copy.discard}</Button>
 </Dialog.Actions></ScrollView></Dialog></Portal>;
}
