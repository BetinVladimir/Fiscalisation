import React from 'react';
import { View } from 'react-native';
import { ActivityIndicator, Button, Card, HelperText, Text } from 'react-native-paper';

type Policy = Record<string, unknown>;
type Props = { policy: Policy | null; loading: boolean; error: string; onRetry: () => void };
const display = (value: unknown) => typeof value === 'string' || typeof value === 'number' ? String(value) : '—';
const rows = [
  ['country', 'Държава'],
  ['official_currency', 'Официална валута'],
  ['version', 'Версия на политиката'],
  ['valid_from', 'В сила от'],
  ['valid_to', 'В сила до'],
] as const;

export function SettingsScreen({ policy, loading, error, onRetry }: Props) {
  const taxGroups = Array.isArray(policy?.tax_groups) ? policy.tax_groups as Policy[] : [];
  return <View testID="settings.screen" style={{ gap: 12, paddingBottom: 16 }}>
    {loading ? <ActivityIndicator testID="settings.loading" accessibilityLabel="Зареждане на регулаторна политика" /> : null}
    {!loading && error ? <Card mode="outlined" testID="settings.error"><Card.Content style={{ gap: 8 }}>
      <HelperText type="error" visible accessibilityLiveRegion="polite">{error}</HelperText>
      <Button mode="outlined" contentStyle={{ minHeight: 48 }} onPress={onRetry}>Опитай отново</Button>
    </Card.Content></Card> : null}
    {!loading && !error && !policy ? <Text testID="settings.empty" variant="bodyMedium">Политиката не е заредена.</Text> : null}
    {!error && policy ? <>
      <Card mode="outlined" testID="settings.policy"><Card.Content style={{ gap: 8, paddingVertical: 16 }}>
        <Text variant="titleLarge" accessibilityRole="header">Ефективна регулаторна политика</Text>
        {rows.map(([key, label]) => <Text key={key} variant="bodyMedium">{label}: {display(policy[key])}</Text>)}
        <Text variant="bodySmall">Профили: {Array.isArray(policy.profiles) ? policy.profiles.map(display).join(', ') || '—' : '—'}</Text>
      </Card.Content></Card>
      <Card mode="outlined" testID="settings.tax-groups"><Card.Content style={{ gap: 8, paddingVertical: 16 }}>
        <Text variant="titleMedium" accessibilityRole="header">Данъчни групи</Text>
        {taxGroups.length ? taxGroups.map((group, index) => <Text key={`${display(group.code)}-${index}`} variant="bodyMedium"
          testID={`settings.tax-group.${display(group.code)}`}>
          {display(group.code)} • {display(group.rate)}% • {display(group.policy_version)}
        </Text>) : <Text variant="bodyMedium">Няма данъчни групи в API.</Text>}
      </Card.Content></Card>
    </> : null}
  </View>;
}
