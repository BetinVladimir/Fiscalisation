import React from 'react';
import type { ComponentProps } from 'react';
import { View } from 'react-native';
import { TouchableRipple } from 'react-native-paper';
import { touchTarget } from './tokens';

type RippleProps = ComponentProps<typeof TouchableRipple>;
type Props = Omit<RippleProps, 'children'> & { children?: React.ReactNode; activeOpacity?: number };

/** Paper interaction surface with the standard MD3 ripple and state layer. */
export function AppPressable({ style, children, accessibilityRole, activeOpacity: _activeOpacity, ...props }: Props) {
  return (
    <TouchableRipple
      {...props}
      accessibilityRole={accessibilityRole || 'button'}
      style={(state) => [
        { minWidth: touchTarget.min, minHeight: touchTarget.min, justifyContent: 'center' },
        typeof style === 'function' ? style(state) : style,
      ]}
    >
      {React.isValidElement(children) && React.Children.count(children) === 1 ? (
        children
      ) : (
        <View pointerEvents="none">{children ?? null}</View>
      )}
    </TouchableRipple>
  );
}
