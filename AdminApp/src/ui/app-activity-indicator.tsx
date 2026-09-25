import React from 'react';
import type { ComponentProps } from 'react';
import { ActivityIndicator } from 'react-native-paper';

type Props = ComponentProps<typeof ActivityIndicator> & { accessibilityLabel?: string };

export function AppActivityIndicator({ accessibilityLabel = 'Загрузка', ...props }: Props) {
  return <ActivityIndicator {...props} accessibilityLabel={accessibilityLabel} accessibilityLiveRegion="polite" />;
}
