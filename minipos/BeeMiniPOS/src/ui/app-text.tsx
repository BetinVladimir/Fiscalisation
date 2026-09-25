import React from 'react';
import type { ComponentProps } from 'react';
import { Text } from 'react-native-paper';

type Props = ComponentProps<typeof Text>;

export function AppText({ allowFontScaling = true, maxFontSizeMultiplier = 2, ...props }: Props) {
  return <Text {...props} allowFontScaling={allowFontScaling} maxFontSizeMultiplier={maxFontSizeMultiplier} />;
}
