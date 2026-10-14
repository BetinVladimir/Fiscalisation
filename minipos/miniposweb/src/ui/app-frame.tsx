import type {ReactNode} from 'react';
import {Surface,useTheme} from 'react-native-paper';
export function AppFrame({children,testID}:{children:ReactNode;testID:string}){const theme=useTheme();return <Surface testID={testID} mode="flat" elevation={0} style={{minWidth:0,minHeight:'100%',backgroundColor:theme.colors.background}}>{children}</Surface>;}
