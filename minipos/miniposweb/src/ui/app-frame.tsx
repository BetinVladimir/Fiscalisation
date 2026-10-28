import type {ReactNode} from 'react';
import {useWindowDimensions} from 'react-native';
import {Surface,useTheme} from 'react-native-paper';
export function AppFrame({children,testID}:{children:ReactNode;testID:string}){const theme=useTheme(),{height}=useWindowDimensions();return <Surface testID={testID} mode="flat" elevation={0} style={{minWidth:0,minHeight:height,backgroundColor:theme.colors.background}}>{children}</Surface>;}
