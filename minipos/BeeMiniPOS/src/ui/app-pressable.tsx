import React, { useEffect, useRef } from 'react';
import type { ComponentProps } from 'react';
import { View } from 'react-native';
import { TouchableRipple, useTheme } from 'react-native-paper';
import { touchTarget } from './tokens';

type RippleProps = ComponentProps<typeof TouchableRipple>;
type Props = Omit<RippleProps, 'children'> & { children?: React.ReactNode; activeOpacity?: number };

/** Paper interaction surface with the standard MD3 ripple and state layer. */
export function AppPressable({ style, children, accessibilityRole, activeOpacity: _activeOpacity, onPress, ...props }: Props) {
  const theme = useTheme();
  const mounted = useRef(false);
  const snapshot = { onPress, disabled: props.disabled };
  const latest = useRef(snapshot); latest.current = snapshot;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  return (
    <TouchableRipple
      {...props}
      onPress={event => { if (mounted.current && latest.current === snapshot && !props.disabled) onPress?.(event); }}
      accessibilityRole={accessibilityRole || 'button'}
      style={(state) => [
        { minWidth: touchTarget.min, minHeight: touchTarget.min, justifyContent: 'center', borderWidth: 1, borderColor: theme.colors.outline, borderRadius: theme.roundness },
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
