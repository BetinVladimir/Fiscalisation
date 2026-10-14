import React,{useEffect,useId,useRef} from 'react';
import {Platform, View } from 'react-native';
import { HelperText, Text, TextInput, useTheme, type TextInputProps } from 'react-native-paper';

type Props = TextInputProps & {
  'aria-describedby'?:string;
  errorMessage?: string;
  helpText?: string;
};

export function FormField({ errorMessage, helpText, error, ...props }: Props) {
  const theme = useTheme(),labelId=`beefiscal-field-${useId().replace(/:/g,'')}`,supportId=`${labelId}-support`;
  const mounted=useRef(false),snapshot={},latest=useRef(snapshot);latest.current=snapshot;
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
  const editable=()=>mounted.current&&latest.current===snapshot&&props.editable!==false&&!props.disabled;
  const invalid = Boolean(error || errorMessage);
  const supportingText = errorMessage || helpText;
  const visibleLabel = props.label || props.accessibilityLabel || props.placeholder || 'Поле';
  return (
    <View style={{ gap: 4, minWidth: 0 }}>
      <Text nativeID={labelId} testID={props.testID ? `${props.testID}.label` : undefined} variant="bodyMedium" style={{ color: theme.colors.onSurface, flexShrink: 1 }}>{visibleLabel}</Text>
      <TextInput
        mode="outlined"

        {...props}
        onChangeText={value=>{if(editable())props.onChangeText?.(value)}}
        onSubmitEditing={event=>{if(editable())props.onSubmitEditing?.(event)}}
        label={undefined}
        aria-labelledby={Platform.OS==='web'?labelId:undefined}
        aria-describedby={Platform.OS==='web'?[props['aria-describedby'],supportingText?supportId:undefined].filter(Boolean).join(' ')||undefined:props['aria-describedby']}
        aria-invalid={Platform.OS==='web'?invalid:undefined}
        contentStyle={[{ minHeight: 48 }, props.contentStyle]}
        error={invalid}
        accessibilityLabel={props.accessibilityLabel || (typeof visibleLabel === 'string' ? visibleLabel : undefined)}
      />
      {supportingText ? (
        <HelperText nativeID={supportId} accessibilityRole={invalid?'alert':undefined} type={invalid ? 'error' : 'info'} visible accessibilityLiveRegion={invalid ? 'polite' : undefined}>
          {supportingText}
        </HelperText>
      ) : null}
    </View>
  );
}
