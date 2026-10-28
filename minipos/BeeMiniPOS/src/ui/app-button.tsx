import React,{useLayoutEffect,useRef} from 'react';
import type {ComponentProps} from 'react';
import {Button} from 'react-native-paper';
import {touchTarget} from './tokens';
type Props=ComponentProps<typeof Button>;
/** Genuine Paper action; visual shape and scalable label are present before interaction. */
export function AppButton({contentStyle,labelStyle,style,mode,accessibilityRole='button',maxFontSizeMultiplier,labelNumberOfLines=0,onPress,...props}:Props){
 const snapshot={onPress,disabled:props.disabled,loading:props.loading},committed=useRef<typeof snapshot|null>(null);
 useLayoutEffect(()=>{committed.current=snapshot;return()=>{if(committed.current===snapshot)committed.current=null;};});
 return <Button {...props} mode={!mode||mode==='text'?'outlined':mode} accessibilityRole={accessibilityRole}
  disabled={props.disabled||props.loading} maxFontSizeMultiplier={maxFontSizeMultiplier === 0 ? undefined : maxFontSizeMultiplier} labelNumberOfLines={labelNumberOfLines}
  style={[{maxWidth:'100%',flexShrink:1},style]}
  contentStyle={[{minHeight:touchTarget.min,minWidth:touchTarget.min},contentStyle]}
  labelStyle={[{flexWrap:'wrap',flexShrink:1,minWidth:0},labelStyle]}
  onPress={event=>{if(committed.current===snapshot&&!snapshot.disabled&&!snapshot.loading)snapshot.onPress?.(event)}}/>;
}
