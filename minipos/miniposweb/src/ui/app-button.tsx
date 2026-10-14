import React,{useEffect,useRef} from 'react';
import type {ComponentProps} from 'react';
import {Button} from 'react-native-paper';
import {touchTarget} from '@beeloy/design-tokens';
type Props=ComponentProps<typeof Button>;
/** Genuine Paper action; visual shape and scalable label are present before interaction. */
export function AppButton({contentStyle,labelStyle,style,mode,accessibilityRole='button',maxFontSizeMultiplier,labelNumberOfLines=0,onPress,...props}:Props){
 const mounted=useRef(false),snapshot={onPress,disabled:props.disabled,loading:props.loading},latest=useRef(snapshot);latest.current=snapshot;
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 return <Button {...props} mode={!mode||mode==='text'?'outlined':mode} accessibilityRole={accessibilityRole}
  disabled={props.disabled||props.loading} maxFontSizeMultiplier={maxFontSizeMultiplier === 0 ? undefined : maxFontSizeMultiplier} labelNumberOfLines={labelNumberOfLines}
  style={[{maxWidth:'100%',flexShrink:1},style]}
  contentStyle={[{minHeight:touchTarget.min,minWidth:touchTarget.min},contentStyle]}
  labelStyle={[{flexWrap:'wrap',flexShrink:1,minWidth:0},labelStyle]}
  onPress={event=>{const current=latest.current;if(mounted.current&&!current.disabled&&!current.loading)current.onPress?.(event)}}/>;
}
