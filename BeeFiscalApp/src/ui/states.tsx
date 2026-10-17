import React,{useId} from 'react';
import {Platform,StyleSheet,View} from 'react-native';
import {ActivityIndicator,HelperText,Icon,Text,useTheme} from 'react-native-paper';
import {AppButton} from './app-button';
import {spacing} from './tokens';
import {useUiPreferences} from './preferences';
import {foundationCopy} from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
function useCopy(){return foundationCopy[useUiPreferences().language]}
function DecorativeIcon({source,color}:{source:string;color:string}){
 return <View aria-hidden accessibilityElementsHidden importantForAccessibility="no-hide-descendants"><Icon source={source} size={32} color={color}/></View>;
}
export function LoadingState({label}:{label?:string}){
 const copy=useCopy(),text=label??copy.loading;
 return <View accessibilityLiveRegion="polite" style={styles.root}><ActivityIndicator animating accessibilityLabel={text}/><Text variant="bodyLarge">{text}</Text></View>;
}
export function EmptyState({title,message,icon='inbox-outline'}:{title:string;message?:string;icon?:string}){
 const theme=useTheme();
 return <View style={styles.root}><DecorativeIcon source={icon} color={theme.colors.onSurfaceVariant}/><Text variant="titleMedium" accessibilityRole="header">{title}</Text>{message?<Text variant="bodyMedium">{message}</Text>:null}</View>;
}
export function ErrorState({message,onRetry,retryDisabled=false}:{message:string;onRetry?:()=>void;retryDisabled?:boolean}){
 const theme=useTheme(),copy=useCopy(),errorId=`beefiscal-error-state-${useId().replace(/:/g,'')}`;
 return <View style={styles.root}><DecorativeIcon source="alert-circle-outline" color={theme.colors.error}/><Text variant="titleMedium" accessibilityRole="header">{copy.error}</Text><HelperText nativeID={errorId} type="error" visible accessibilityRole="alert" accessibilityLiveRegion="polite">{message}</HelperText>{onRetry?<AppButton testID="beefiscal.state.retry" mode="outlined" disabled={retryDisabled} aria-describedby={Platform.OS==='web'?errorId:undefined} onPress={onRetry}>{copy.retry}</AppButton>:null}</View>;
}
const styles=StyleSheet.create({root:{flex:1,minHeight:160,padding:spacing.lg,gap:spacing.sm,alignItems:'center',justifyContent:'center'}});
