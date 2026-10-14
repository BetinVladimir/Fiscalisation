import React from 'react';
import type { PropsWithChildren, ReactNode } from 'react';
import { Platform, ScrollView, StyleSheet, View, type ViewStyle } from 'react-native';
import { Appbar, Surface, Text, useTheme } from 'react-native-paper';
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
    <Surface mode="flat" elevation={0} style={[styles.content, { paddingLeft:Math.max(gutter,insets.left),paddingRight:Math.max(gutter,insets.right), paddingBottom: Math.max(insets.bottom, spacing.md) }, contentStyle]}>
      {children}
    </Surface>
  );

  return (
    <Surface mode="flat" elevation={0} style={[styles.root, { backgroundColor: theme.colors.background }]}>
      {title ? <Appbar.Header elevated style={{height:Platform.OS==='ios'?null:'auto',minHeight:64,paddingVertical:8}}><View style={{flex:1,minWidth:0,gap:8}}><Appbar.Content title={<Text variant="headlineSmall" accessibilityRole="header">{title}</Text>}/>{actions?<View style={{flexDirection:'row',flexWrap:'wrap',gap:8}}>{actions}</View>:null}</View></Appbar.Header> : actions?<View style={{flexDirection:'row',flexWrap:'wrap',gap:8}}>{actions}</View>:null}
      {scroll ? <ScrollView contentContainerStyle={styles.grow} keyboardShouldPersistTaps="handled">{body}</ScrollView> : body}
    </Surface>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  grow: { flexGrow: 1 },
  content: { width: '100%', maxWidth: 1440, alignSelf: 'center', paddingVertical: spacing.md, gap: spacing.md },
});
