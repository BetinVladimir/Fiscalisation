import React from 'react';
import type { ComponentProps } from 'react';
import { Button } from 'react-native-paper';
import { touchTarget } from './tokens';

type Props = ComponentProps<typeof Button>;

export function AppButton({ contentStyle, labelStyle, ...props }: Props) {
  return (
    <Button
      {...props}
      accessibilityRole="button"
      contentStyle={[{ minHeight: touchTarget.min }, contentStyle]}
      labelStyle={[{ fontFamily: 'Roboto_500Medium' }, labelStyle]}
    />
  );
}
