import React from 'react';
import { View } from 'react-native';
import { HelperText, TextInput, type TextInputProps } from 'react-native-paper';

type Props = TextInputProps & {
  errorMessage?: string;
  helpText?: string;
};

export function FormField({ errorMessage, helpText, error, ...props }: Props) {
  const invalid = Boolean(error || errorMessage);
  const supportingText = errorMessage || helpText;
  const visibleLabel = props.label || props.accessibilityLabel || props.placeholder || 'Поле';
  return (
    <View>
      <TextInput
        mode="outlined"
        {...props}
        label={visibleLabel}
        error={invalid}
        accessibilityLabel={props.accessibilityLabel || (typeof visibleLabel === 'string' ? visibleLabel : undefined)}
      />
      {supportingText ? (
        <HelperText type={invalid ? 'error' : 'info'} visible accessibilityLiveRegion={invalid ? 'polite' : undefined}>
          {supportingText}
        </HelperText>
      ) : null}
    </View>
  );
}
