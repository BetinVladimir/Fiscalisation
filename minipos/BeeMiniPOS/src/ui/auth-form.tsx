import React from 'react';
import { useWindowDimensions, View, type ViewProps } from 'react-native';

/** Login/onboarding width: full on phones, half on md, one third on lg. */
export function AuthForm({ style, ...props }: ViewProps) {
  const { width } = useWindowDimensions();
  const availableWidth = Math.max(0, width - 32);
  const targetWidth = width >= 1024 ? width / 3 : width >= 600 ? width / 2 : availableWidth;
  const responsiveWidth = Math.min(availableWidth, Math.max(Math.min(320, availableWidth), Math.min(480, targetWidth)));
  return <View {...props} style={[style, { width: responsiveWidth, maxWidth: responsiveWidth, alignSelf: 'center' }]} />;
}
