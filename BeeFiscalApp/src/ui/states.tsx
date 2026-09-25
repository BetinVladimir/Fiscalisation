import React from 'react';
import { StyleSheet, View } from 'react-native';
import { ActivityIndicator, Button, Icon, Text, useTheme } from 'react-native-paper';
import { spacing } from './tokens';

export function LoadingState({ label = 'Загрузка' }: { label?: string }) {
  return <View accessibilityLabel={label} accessibilityLiveRegion="polite" style={styles.root}><ActivityIndicator animating /><Text>{label}</Text></View>;
}

export function EmptyState({ title, message, icon = 'inbox-outline' }: { title: string; message?: string; icon?: string }) {
  const theme = useTheme();
  return <View style={styles.root}><Icon source={icon} size={32} color={theme.colors.onSurfaceVariant} /><Text variant="titleMedium">{title}</Text>{message ? <Text variant="bodyMedium">{message}</Text> : null}</View>;
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  const theme = useTheme();
  return <View accessibilityLiveRegion="polite" style={styles.root}><Icon source="alert-circle-outline" size={32} color={theme.colors.error} /><Text variant="titleMedium">Не удалось загрузить данные</Text><Text variant="bodyMedium">{message}</Text>{onRetry ? <Button mode="contained" onPress={onRetry} contentStyle={styles.target}>Повторить</Button> : null}</View>;
}

const styles = StyleSheet.create({ root: { flex: 1, minHeight: 160, padding: spacing.lg, gap: spacing.sm, alignItems: 'center', justifyContent: 'center' }, target: { minHeight: 48 } });
