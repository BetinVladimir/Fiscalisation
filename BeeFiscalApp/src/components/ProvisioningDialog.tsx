import React,{useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {Platform,ScrollView,View,useWindowDimensions} from 'react-native';
import {Dialog,Text,TextInput,useTheme} from 'react-native-paper';
import {AppButton} from '../ui/app-button';
import {useUiPreferences} from '../ui/preferences';
import {provisioningCopy} from '../ui/provisioning-copy';
import {PaperDialogHost} from '../../../../BeeloyBackend/mobile/shared-ui/src/components/PaperDialogHost';
export function ProvisioningDialog({visible,busy,onCancel,onSubmit}:{visible:boolean;busy:boolean;onCancel:()=>void;onSubmit:(settings:{ssid:string;wifiPassword:string;fiscalPassword:string})=>Promise<void>}){
 const theme=useTheme(),{language}=useUiPreferences(),copy=provisioningCopy(language),{width,height}=useWindowDimensions();
 const [node,setNode]=useState<HTMLElement|null>(null);
 const dialogRef=useCallback((value:View|null)=>{if(Platform.OS==='web')setNode(value as unknown as HTMLElement|null)},[]);
 const cancelRef=useRef<()=>void>(()=>{});
 const [ssid,setSSID]=useState(''),[wifiPassword,setWifiPassword]=useState(''),[fiscalPassword,setFiscalPassword]=useState('');
 const committed=useRef<object|null>(null),snapshot={},pending=useRef(false);
 useLayoutEffect(()=>{committed.current=snapshot;return()=>{if(committed.current===snapshot)committed.current=null}});
 const current=()=>committed.current===snapshot&&visible&&!busy&&!pending.current;
 const clear=()=>{setSSID('');setWifiPassword('');setFiscalPassword('')};
 useEffect(()=>{if(!visible)clear()},[visible]);
 const cancel=()=>{if(!current())return;clear();onCancel()};
 const submit=async()=>{
  if(!current())return;
  pending.current=true;
  const settings={ssid,wifiPassword,fiscalPassword};clear();
  try{await onSubmit(settings)}finally{settings.ssid='';settings.wifiPassword='';settings.fiscalPassword='';pending.current=false;}
 };
 useLayoutEffect(()=>{cancelRef.current=cancel});
 useEffect(()=>{
  if(Platform.OS!=='web'||!visible||!node)return;
  const opener=document.activeElement as HTMLElement|null;
  const targets=()=>Array.from(node.querySelectorAll<HTMLElement>('input:not(:disabled),button:not(:disabled),[tabindex]:not([tabindex="-1"])')).filter(x=>x.getClientRects().length&&x.getAttribute('aria-disabled')!=='true');
  const focus=()=>{(targets()[0]??node).focus()};focus();
  const key=(event:KeyboardEvent)=>{
   if(event.key==='Escape'){event.preventDefault();event.stopPropagation();cancelRef.current();return}
   if(event.key!=='Tab')return;
   const list=targets(),index=list.indexOf(document.activeElement as HTMLElement);
   if(!list.length){event.preventDefault();node.focus();return}
   if(event.shiftKey?index<=0:index<0||index===list.length-1){event.preventDefault();list[event.shiftKey?list.length-1:0].focus()}
  };
  const focusin=(event:FocusEvent)=>{if(!node.contains(event.target as Node))focus()};
  document.addEventListener('keydown',key,true);document.addEventListener('focusin',focusin,true);
  return()=>{document.removeEventListener('keydown',key,true);document.removeEventListener('focusin',focusin,true);if(opener?.isConnected)opener.focus()};
 },[visible,node]);
 const maxHeight=Math.max(96,height-32),dialogWidth=Math.max(0,Math.min(520,width-32));
 const wrap={paddingRight:4,backgroundColor:theme.colors.elevation.level3,minWidth:0,flexShrink:1,flexWrap:'wrap' as const,...(Platform.OS==='web'?{overflowWrap:'anywhere' as const}:{})};
 const backgroundColor=theme.colors.elevation.level3;
 return <PaperDialogHost visible={visible} onDismiss={cancel}><Dialog visible={visible} onDismiss={cancel} dismissable={!busy} dismissableBackButton={!busy} style={{alignSelf:'center',width:dialogWidth,maxWidth:520,maxHeight,marginHorizontal:16,backgroundColor}}>
  <View ref={dialogRef} role="dialog" aria-modal={visible} aria-label={copy.title} accessibilityViewIsModal tabIndex={-1} testID="provisioning.dialog" style={{flexShrink:1,minWidth:0,backgroundColor}}>
   <ScrollView testID="provisioning.scroll" style={{maxHeight,flexShrink:1}} contentContainerStyle={{padding:12,gap:12}} keyboardShouldPersistTaps="handled" nestedScrollEnabled>
    <Text testID="provisioning.title" variant="titleLarge" accessibilityRole="header" style={wrap}>{copy.title}</Text><Text testID="provisioning.help" variant="bodyMedium" style={wrap}>{copy.help}</Text>
    <Text testID="provisioning.ssid.label" variant="labelLarge" style={wrap}>{copy.ssid}</Text><TextInput testID="provisioning.ssid" mode="outlined" accessibilityLabel={copy.ssid} value={ssid} onChangeText={value=>{if(current())setSSID(value)}} disabled={busy} autoCapitalize="none"/>
    <Text testID="provisioning.wifi-password.label" variant="labelLarge" style={wrap}>{copy.wifiPassword}</Text><TextInput testID="provisioning.wifi-password" mode="outlined" accessibilityLabel={copy.wifiPassword} value={wifiPassword} onChangeText={value=>{if(current())setWifiPassword(value)}} disabled={busy} secureTextEntry autoCapitalize="none"/>
    <Text testID="provisioning.fiscal-password.label" variant="labelLarge" style={wrap}>{copy.fiscalPassword}</Text><TextInput testID="provisioning.fiscal-password" mode="outlined" accessibilityLabel={copy.fiscalPassword} value={fiscalPassword} onChangeText={value=>{if(current())setFiscalPassword(value)}} disabled={busy} secureTextEntry keyboardType="number-pad"/>
    <AppButton testID="provisioning.submit" mode="contained" loading={busy} disabled={busy} onPress={()=>void submit()}>{copy.submit}</AppButton>
    <AppButton testID="provisioning.cancel" disabled={busy} mode="outlined" onPress={cancel}>{copy.cancel}</AppButton>
   </ScrollView>
  </View>
 </Dialog></PaperDialogHost>
}
