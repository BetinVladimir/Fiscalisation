import React from 'react';
import type { PropsWithChildren, ReactNode } from 'react';
import { ScrollView, StyleSheet, View, type ViewStyle } from 'react-native';
import { Appbar, useTheme } from 'react-native-paper';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { spacing } from './tokens';
import { useResponsiveLayout } from './responsive';

type ScreenProps = PropsWithChildren<{
  title?: string;
  actions?: ReactNode;
  scroll?: boolean;
  contentStyle?: ViewStyle;
}>;

export function Screen({ title, actions, scroll = true, contentStyle, children }: ScreenProps) {
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  const { gutter } = useResponsiveLayout();
  const body = (
    <View style={[styles.content, { paddingHorizontal: gutter, paddingBottom: Math.max(insets.bottom, spacing.md) }, contentStyle]}>
      {children}
    </View>
  );

  return (
    <View style={[styles.root, { backgroundColor: theme.colors.background }]}>
      {title ? <Appbar.Header elevated><Appbar.Content title={title} />{actions}</Appbar.Header> : null}
      {scroll ? <ScrollView contentContainerStyle={styles.grow} keyboardShouldPersistTaps="handled">{body}</ScrollView> : body}
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  grow: { flexGrow: 1 },
  content: { width: '100%', maxWidth: 1440, alignSelf: 'center', paddingVertical: spacing.md, gap: spacing.md },
});
