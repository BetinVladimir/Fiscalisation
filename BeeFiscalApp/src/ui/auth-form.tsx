import React from 'react';
import { useWindowDimensions, View, type ViewProps } from 'react-native';

/** Login/onboarding width: full on phones, half on md, one third on lg. */
export function AuthForm({ style, ...props }: ViewProps) {
  const { width } = useWindowDimensions();
  const responsiveWidth = width >= 1024 ? '33.3333%' : width >= 768 ? '50%' : '100%';
  return <View {...props} style={[style, { width: responsiveWidth, maxWidth: 640, alignSelf: 'center' }]} />;
}
