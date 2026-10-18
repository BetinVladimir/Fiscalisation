import React,{useEffect,useLayoutEffect,useMemo,useRef,useState} from 'react';
import {Platform,View} from 'react-native';
import {Text,Divider} from 'react-native-paper';
import {AppButton} from './app-button';
import {fiscalOperationCopyFor,type FiscalOperationCopy} from './fiscal-operation-copy';
import {fiscalOperationRows,operationKeys,type OperationRow} from './fiscal-operation-rows';
const wrap={minWidth:0,maxWidth:'100%' as const,flexShrink:1,flexWrap:'wrap' as const,...(Platform.OS==='web'?{overflowWrap:'anywhere' as const}:{})};
function rowValue(row:OperationRow,copy:FiscalOperationCopy){return row.kind!=='value'?copy[row.kind]:row.boolean===true?copy.yes:row.boolean===false?copy.no:row.value;}
export function FiscalOperationResult({value,language,ownerKey,testID='sale-result'}:{value:unknown;language:string;ownerKey:unknown;testID?:string}){
 const operation=value&&typeof value==='object'?(value as Record<string,unknown>).operation_id:undefined;
 const identity=useMemo(()=>({ownerKey,operation}),[ownerKey,operation]);
 // Remount the action wrapper on an owner/operation replacement. Its retained
 // Paper handler must not delegate a stale press into the new operation.
 const [revision,setRevision]=useState({identity,epoch:0});
 if(revision.identity!==identity)setRevision({identity,epoch:revision.epoch+1});
 return <ResultFields key={revision.epoch} value={value} language={language} ownerKey={identity} testID={testID}/>;
}
function ResultFields({value,language,ownerKey,testID}:{value:unknown;language:string;ownerKey:unknown;testID:string}){
 const copy=fiscalOperationCopyFor(language),rows=useMemo(()=>fiscalOperationRows(value),[value]);
 const [expansion,setExpansion]=useState<{owner:unknown;count:number}>({owner:ownerKey,count:10}),limit=expansion.owner===ownerKey?expansion.count:10;
 const mounted=useRef(false),lease=useRef<{owner:unknown;value:unknown}|null>(null),rendered={owner:ownerKey,value};
 useLayoutEffect(()=>{lease.current=rendered;return()=>{if(lease.current===rendered)lease.current=null}});useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 const label=(path:string[])=>path.map((key,index)=>index===0&&(operationKeys as readonly string[]).includes(key)?copy[key as keyof FiscalOperationCopy]:key).join(' › ');
 return <View testID={testID} style={{gap:8,minWidth:0,maxWidth:'100%'}}><Text testID={testID+'.title'} variant="titleMedium" accessibilityRole="header" style={wrap}>{copy.title}</Text>{rows.slice(0,limit).map((row,index)=><View key={index} testID={testID+'.row.'+index} style={{gap:4,minWidth:0,maxWidth:'100%'}}><Text selectable testID={testID+'.row.'+index+'.label'} variant="labelLarge" style={wrap}>{label(row.path)}</Text><Text selectable testID={testID+'.row.'+index+'.value'} variant="bodyMedium" style={wrap}>{rowValue(row,copy)}</Text><Divider/></View>)}{limit<rows.length?<AppButton testID={testID+'.more'} mode="outlined" labelNumberOfLines={0} contentStyle={{minHeight:48,minWidth:48}} labelStyle={wrap} onPress={()=>{if(mounted.current&&lease.current===rendered)setExpansion({owner:ownerKey,count:limit+10})}}>{copy.showMore}</AppButton>:null}</View>;
}
