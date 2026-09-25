import React from 'react';
import { Pressable, type PressableProps, type PressableStateCallbackType, type StyleProp, type ViewStyle } from 'react-native';
import { touchTarget } from './tokens';

type Props = PressableProps & { activeOpacity?: number };

export function AppPressable({ style, accessibilityRole, activeOpacity: _activeOpacity, ...props }: Props) {
  const resolveStyle = (state: PressableStateCallbackType): StyleProp<ViewStyle> => [
    { minWidth: touchTarget.min, minHeight: touchTarget.min, justifyContent: 'center' },
    typeof style === 'function' ? style(state) : style,
  ];
  return <Pressable {...props} accessibilityRole={accessibilityRole || 'button'} style={resolveStyle} />;
}
