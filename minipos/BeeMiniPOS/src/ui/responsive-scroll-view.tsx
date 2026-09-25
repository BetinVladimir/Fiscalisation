import React from 'react';
import { forwardRef } from 'react';
import { ScrollView, type ScrollViewProps } from 'react-native';
import { useResponsiveLayout } from './responsive';

export const ResponsiveScrollView = forwardRef<ScrollView, ScrollViewProps>(function ResponsiveScrollView(
  { horizontal, contentContainerStyle, keyboardShouldPersistTaps = 'handled', ...props },
  ref,
) {
  const { gutter } = useResponsiveLayout();
  return (
    <ScrollView
      ref={ref}
      {...props}
      horizontal={horizontal}
      keyboardShouldPersistTaps={keyboardShouldPersistTaps}
      contentContainerStyle={[
        horizontal ? undefined : { width: '100%', maxWidth: 1440, alignSelf: 'center', paddingHorizontal: gutter },
        contentContainerStyle,
      ]}
    />
  );
});
