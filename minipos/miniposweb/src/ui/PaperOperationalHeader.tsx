import {headerTargetSize} from './header-layout';
import {headerIcons} from './header-icons';
import React, {useLayoutEffect, useRef, useState, type ReactNode} from 'react';
import {Platform, View, useWindowDimensions} from 'react-native';
import {Appbar, IconButton, Text, useTheme} from 'react-native-paper';

export type HeaderAction = {label:string; onPress:()=>void; disabled?:boolean; testID?:string};
export type OperationalHeaderProps = {
 title:string; testID?:string; back?:HeaderAction; menu?:HeaderAction;
 preferences:(target:number)=>ReactNode; preferenceCount?:number;
};
export function parseHeaderZoom(value:unknown){
 if(typeof value!=='string')return 1;
 const raw=value.trim();
 if(!/^(?:\d+(?:\.\d*)?|\.\d+)%?$/.test(raw))return 1;
 const n=Number(raw.endsWith('%')?raw.slice(0,-1):raw)/(raw.endsWith('%')?100:1);
 return Number.isFinite(n)&&n>0?n:1;
}
/** Measured WEB CSS zoom changes layout coordinates; native scaling remains enabled. */
export function useHeaderMetrics(){
 const {width,height}=useWindowDimensions(),[zoom,setZoom]=useState(1);
 useLayoutEffect(()=>{
  if(Platform.OS!=='web'||typeof document==='undefined')return;
  const update=()=>{setZoom(parseHeaderZoom(getComputedStyle(document.documentElement).zoom));};
  update();const observer=new MutationObserver(update);observer.observe(document.documentElement,{attributes:true,attributeFilter:['style','class']});window.addEventListener('resize',update);return()=>{observer.disconnect();window.removeEventListener('resize',update);};
 },[]);
 return {zoom,width:width/zoom,height:height/zoom,target:Platform.OS==='web'&&width/zoom<240?32:48};
}
/** Local presentation only. The application retains navigation, sale and owner guards. */
export function PaperOperationalHeader({title,testID='operational.header',back,menu,preferences,preferenceCount=2}:OperationalHeaderProps){
 const theme=useTheme(),metrics=useHeaderMetrics(),[rowWidth,setRowWidth]=useState<number|null>(null),row=useRef<View>(null);
 const count=preferenceCount+Number(Boolean(back))+Number(Boolean(menu));
 const available=rowWidth??Math.max(0,metrics.width-8);
 const target=headerTargetSize(Platform.OS,available,count);
 const measure=()=>{if(Platform.OS!=='web')return;const node=row.current as unknown as {getBoundingClientRect?:()=>{width:number}}|null;const width=node?.getBoundingClientRect?.().width;if(width&&Number.isFinite(width))setRowWidth(width/metrics.zoom);};
 useLayoutEffect(measure,[metrics.width,metrics.zoom,count]);
 const snapshot={back,menu},committed=useRef<object|null>(null);
 useLayoutEffect(()=>{committed.current=snapshot;return()=>{if(committed.current===snapshot)committed.current=null;};});
 const action=(value:HeaderAction,icon:keyof typeof headerIcons,id:string)=><IconButton testID={value.testID??id} mode="outlined" icon={headerIcons[icon]} size={20} accessibilityLabel={value.label} disabled={value.disabled} style={{width:target,height:target,margin:0,flexShrink:0}} onPress={()=>{if(committed.current===snapshot&&!value.disabled)value.onPress();}}/>;
 return <Appbar.Header testID={testID} statusBarHeight={0} style={{height:'auto',minHeight:56,backgroundColor:theme.colors.surface,paddingHorizontal:4}}>
  <View ref={row} onLayout={measure} style={{width:'100%',minWidth:0,flexDirection:'row',flexWrap:'nowrap',alignItems:'center',gap:2}}>
   {back?action(back,'arrow-left',`${testID}.back`):null}
   <Text testID={`${testID}.title`} variant="titleMedium" accessibilityRole="header" accessibilityLabel={title} numberOfLines={1} ellipsizeMode="tail" style={{flex:1,minWidth:0,flexShrink:1,color:theme.colors.onSurface}}>{title}</Text>
   {preferences(target)}
   {menu?action(menu,'menu',`${testID}.menu`):null}
  </View>
 </Appbar.Header>;
}
