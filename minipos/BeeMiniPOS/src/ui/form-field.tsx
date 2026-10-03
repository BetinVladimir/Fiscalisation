import React from 'react';
import { View } from 'react-native';
import { HelperText, Text, TextInput, useTheme, type TextInputProps } from 'react-native-paper';

type Props = TextInputProps & {
  errorMessage?: string;
  helpText?: string;
};

export function FormField({ errorMessage, helpText, error, ...props }: Props) {
  const theme = useTheme();
  const invalid = Boolean(error || errorMessage);
  const supportingText = errorMessage || helpText;
  const visibleLabel = props.label || props.accessibilityLabel || props.placeholder || 'Поле';
  return (
    <View style={{ minWidth: 0, gap: 4 }}>
      <Text testID={props.testID ? `${props.testID}.label` : undefined} variant="bodyMedium" style={{ color: theme.colors.onSurface, flexShrink: 1 }}>{visibleLabel}</Text>
      <TextInput
        mode="outlined"
        {...props}
        label={undefined}
        contentStyle={[{ minHeight: 48 }, props.contentStyle]}
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
