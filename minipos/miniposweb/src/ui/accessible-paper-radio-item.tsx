import React,{useLayoutEffect,useRef,useState} from 'react';
import {Platform,View} from 'react-native';
import {RadioButton} from 'react-native-paper';
import {synchronizePaperRadioTabStops} from './paper-radio-navigation';

/** Genuine Paper Item; bridge RN-web checked semantics and the decorative inner control.
 * Web labels receive hit testing and bubble activation to the Paper radio; no extra focus stop. */
export function AccessiblePaperRadioItem({style,labelStyle,...props}:React.ComponentProps<typeof RadioButton.Item>){
 const [narrow,setNarrow]=useState(false);
 const mountedRoot=useRef<HTMLElement|null>(null),mounted=useRef(false),snapshot={props},latest=useRef(snapshot);useLayoutEffect(()=>{latest.current=snapshot;});
 const owns=()=>mounted.current&&latest.current===snapshot&&!props.disabled;
 useLayoutEffect(()=>{mounted.current=true;return()=>{mounted.current=false;cleanup.current?.();cleanup.current=null;mountedRoot.current=null}},[]);
 useLayoutEffect(()=>{if(Platform.OS==='web'){const group=mountedRoot.current?.closest<HTMLElement>('[role="radiogroup"]');if(group)synchronizePaperRadioTabStops(group);}});
 const cleanup=useRef<(()=>void)|null>(null);
 return <View onLayout={event=>{const width=event.nativeEvent.layout.width;if(width>0)setNarrow(width<160);}} ref={node=>{
  cleanup.current?.();cleanup.current=null;mountedRoot.current=null;
  if(Platform.OS!=='web'||!node)return;
  const root=node as unknown as HTMLElement;mountedRoot.current=root;const radio=root.querySelector<HTMLElement>('[role="radio"]');if(!radio)return;
  radio.setAttribute('aria-checked',String(props.status==='checked'));
  radio.setAttribute('aria-disabled',String(!!props.disabled));
  radio.tabIndex=props.disabled?-1:0;
  const navigationKeys=[' ','Enter','ArrowDown','ArrowRight','ArrowUp','ArrowLeft','Home','End'];
  const keydown=(event:KeyboardEvent)=>{
   if(!navigationKeys.includes(event.key))return;
   event.preventDefault();event.stopPropagation();if(event.repeat||!owns()||radio.getAttribute('aria-disabled')==='true')return;
   if(event.key===' '||event.key==='Enter'){radio.click();return;}
   const group=radio.closest('[role="radiogroup"]');if(!group)return;
   const items=Array.from(group.querySelectorAll<HTMLElement>('[role="radio"]')).filter(item=>item.closest('[role="radiogroup"]')===group&&!item.closest('[aria-hidden="true"],[inert]')&&item.getAttribute('aria-disabled')!=='true');
   if(!items.length)return;const index=items.indexOf(radio),next=event.key==='Home'?0:event.key==='End'?items.length-1:(index+(event.key==='ArrowUp'||event.key==='ArrowLeft'?-1:1)+items.length)%items.length;
   items[next].focus();items[next].click();items[next].scrollIntoView({block:'nearest'});
  };
  const keyup=(event:KeyboardEvent)=>{if(navigationKeys.includes(event.key)){event.preventDefault();event.stopPropagation();}};
  radio.addEventListener('keydown',keydown,true);radio.addEventListener('keyup',keyup,true);
  cleanup.current=()=>{radio.removeEventListener('keydown',keydown,true);radio.removeEventListener('keyup',keyup,true);};
  for(const indicator of Array.from(radio.querySelectorAll<HTMLElement>('[role="radio"]'))){indicator.setAttribute('aria-hidden','true');indicator.tabIndex=-1;indicator.inert=true;}
  const group=radio.closest<HTMLElement>('[role="radiogroup"]');if(group)synchronizePaperRadioTabStops(group);
 }}><RadioButton.Item {...props} mode={props.mode??'android'} onPress={event=>{if(owns())props.onPress?.(event)}} labelStyle={[{flexWrap:'wrap',flexShrink:1,minWidth:0},labelStyle,Platform.OS==='web'?{pointerEvents:'auto'}:undefined,narrow?{flex:0,width:'100%',alignSelf:'stretch'}:undefined]} style={[style,{minHeight:48,minWidth:48},narrow?{flexDirection:'column',alignItems:'flex-start',paddingHorizontal:0}:undefined]}/></View>;
}
