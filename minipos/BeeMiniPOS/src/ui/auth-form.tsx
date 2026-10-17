import React,{useState} from 'react';
import {View,useWindowDimensions,type ViewProps} from 'react-native';
import {Surface} from 'react-native-paper';
import {breakpoints,layout} from './tokens';
/** Measure available parent space; technical View containers do not replace Paper controls. */
export function AuthForm({style,children,...props}:ViewProps){
 const [available,setAvailable]=useState(0);
 const {width:viewportWidth,fontScale}=useWindowDimensions();
 const effective=viewportWidth/Math.max(1,fontScale||1);
 const fraction=effective>=breakpoints.lg?1/3:effective>=breakpoints.md?1/2:1;
 const width=effective<breakpoints.md?available:Math.min(available,Math.max(layout.formMinWidth,Math.min(layout.formMaxWidth,viewportWidth*fraction)));
 return <View style={{width:'100%'}} onLayout={event=>setAvailable(event.nativeEvent.layout.width)}>
  <Surface mode="flat" elevation={0} {...props} style={[style,{width:available?width:'100%',maxWidth:'100%',alignSelf:'center'}]}>{children}</Surface>
 </View>;
}
