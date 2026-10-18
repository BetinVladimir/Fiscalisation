import React,{useEffect,useState} from 'react';
import {View} from 'react-native';
import {Portal,Modal,Text,TextInput,Button,useTheme} from 'react-native-paper';
export function ProvisioningDialog({visible,busy,onCancel,onSubmit}:{visible:boolean;busy:boolean;onCancel:()=>void;onSubmit:(settings:{ssid:string;wifiPassword:string;fiscalPassword:string})=>Promise<void>}){
 const theme=useTheme();
 const [ssid,setSSID]=useState(''),[wifiPassword,setWifiPassword]=useState(''),[fiscalPassword,setFiscalPassword]=useState('');
 const clear=()=>{setSSID('');setWifiPassword('');setFiscalPassword('')};
 useEffect(()=>{if(!visible)clear()},[visible]);
 const submit=async()=>{const settings={ssid,wifiPassword,fiscalPassword};clear();await onSubmit(settings)};
 return <Portal><Modal visible={visible} onDismiss={()=>{if(!busy){clear();onCancel()}}} contentContainerStyle={{backgroundColor:theme.colors.surface,margin:24,padding:24,borderRadius:16,maxWidth:520,alignSelf:'center',width:'90%'}}><View style={{gap:12}}><Text variant="titleLarge">Настройка на адаптера</Text><Text>Изберете адаптера в режим за настройка. При вече завършена инсталация потвърждението се възстановява без повторно изпращане.</Text><TextInput label="Wi-Fi мрежа" value={ssid} onChangeText={setSSID} disabled={busy} autoCapitalize="none"/><TextInput label="Wi-Fi парола" value={wifiPassword} onChangeText={setWifiPassword} disabled={busy} secureTextEntry autoCapitalize="none"/><TextInput label="Парола на касовия оператор" value={fiscalPassword} onChangeText={setFiscalPassword} disabled={busy} secureTextEntry keyboardType="number-pad"/><Button mode="contained" loading={busy} disabled={busy} onPress={()=>void submit()}>Свързване и потвърждаване</Button><Button disabled={busy} onPress={()=>{clear();onCancel()}}>Отказ</Button></View></Modal></Portal>
}
