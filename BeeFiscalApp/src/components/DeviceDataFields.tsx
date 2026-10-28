import React,{useEffect,useLayoutEffect,useMemo,useRef,useState}from'react';
import{Platform,View}from'react-native';
import{Card,Divider,Text}from'react-native-paper';
import{AppButton}from'../ui/app-button';
import{deviceDataRows,type DeviceDataCopy}from'../deviceDataRows';
type Props={value:unknown;title:string;copy:DeviceDataCopy;testID:string;ownerKey:string};
/** Owner changes remount local expansion; polling the same owner retains the chosen page size. */
export function DeviceDataFields(props:Props){return <OwnedFields key={props.ownerKey} {...props}/>;}
function OwnedFields({value,title,copy,testID}:Props){
 const rows=useMemo(()=>deviceDataRows(value,copy),[value,copy]),[limit,setLimit]=useState(10);
 const mounted=useRef(false),snapshot={},latest=useRef(snapshot);useLayoutEffect(()=>{latest.current=snapshot});
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 const wrap={minWidth:0,maxWidth:'100%' as const,flexShrink:1,flexWrap:'wrap' as const,...(Platform.OS==='web'?{overflowWrap:'anywhere' as const}:{})};
 return <Card testID={testID} mode="outlined" style={{minWidth:0,maxWidth:'100%'}}><Card.Content style={{gap:12,paddingHorizontal:12}}>
  <Text variant="titleMedium" accessibilityRole="header" style={wrap}>{title}</Text>
  {rows.slice(0,limit).map((row,index)=><View key={index} testID={`${testID}.row.${index}`} style={{gap:4,minWidth:0}}>
   {index>0?<Divider/>:null}<Text testID={`${testID}.row.${index}.label`} variant="labelLarge" style={wrap}>{row.label}</Text>
   <Text testID={`${testID}.row.${index}.value`} variant="bodyMedium" selectable style={wrap}>{row.value}</Text>
  </View>)}
  {limit<rows.length?<AppButton testID={`${testID}.more`} mode="outlined" onPress={()=>{if(mounted.current&&latest.current===snapshot)setLimit(previous=>Math.min(previous+10,rows.length));}}>{copy.showMore}</AppButton>:null}
 </Card.Content></Card>;
}
