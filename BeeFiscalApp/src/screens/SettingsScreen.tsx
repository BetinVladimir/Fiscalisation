import { AppButton } from '../ui/app-button';
import { useCurrentAction } from '../ui/use-current-action';
import { useUiPreferences } from '../ui/preferences';
import { foundationCopy } from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import React, { useId } from 'react';
import { Platform, View } from 'react-native';
import { ActivityIndicator, Card, HelperText, Text, useTheme } from 'react-native-paper';

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

export function SettingsScreen({ policy, loading, error: loadError, onRetry }: Props) {
  const theme = useTheme(), admit = useCurrentAction(), { language } = useUiPreferences();
  const valid = !policy || (typeof policy === 'object' && !Array.isArray(policy)
    && (policy.tax_groups === undefined || (Array.isArray(policy.tax_groups)
      && policy.tax_groups.every(group => group && typeof group === 'object' && typeof group.code === 'string' && !!group.code.trim())
      && new Set(policy.tax_groups.map(group => group.code)).size === policy.tax_groups.length)));
  const error = loadError || (!valid ? foundationCopy[language].error : '');
  const errorId = `beefiscal-policy-error-${useId().replace(/:/g, '')}`;
  const taxGroups = Array.isArray(policy?.tax_groups) ? policy.tax_groups as Policy[] : [];
  return <View testID="settings.screen" style={{ gap: 12, paddingBottom: 16 }}>
    {loading ? <ActivityIndicator testID="settings.loading" accessibilityLabel="Зареждане на регулаторна политика" /> : null}
    {!loading && error ? <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="settings.error"><Card.Content style={{ gap: 8 }}>
      <HelperText nativeID={errorId} accessibilityRole="alert" type="error" visible accessibilityLiveRegion="polite">{error}</HelperText>
      <AppButton testID="settings.retry" disabled={loading} aria-describedby={Platform.OS === 'web' ? errorId : undefined} mode="outlined" contentStyle={{ minHeight: 48 }} onPress={() => { if (!loading && admit()) onRetry(); }}>Опитай отново</AppButton>
    </Card.Content></Card> : null}
    {!loading && !error && !policy ? <Text testID="settings.empty" variant="bodyMedium">Политиката не е заредена.</Text> : null}
    {!loading && !error && policy ? <>
      <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="settings.policy"><Card.Content style={{ gap: 8, paddingVertical: 16 }}>
        <Text variant="titleLarge" accessibilityRole="header">Ефективна регулаторна политика</Text>
        {rows.map(([key, label]) => <Text key={key} variant="bodyMedium">{label}: {display(policy[key])}</Text>)}
        <Text variant="bodySmall">Профили: {Array.isArray(policy.profiles) ? policy.profiles.map(display).join(', ') || '—' : '—'}</Text>
      </Card.Content></Card>
      <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="settings.tax-groups"><Card.Content style={{ gap: 8, paddingVertical: 16 }}>
        <Text variant="titleMedium" accessibilityRole="header">Данъчни групи</Text>
        {taxGroups.length ? taxGroups.map((group, index) => <Text key={`${display(group.code)}-${index}`} variant="bodyMedium"
          testID={`settings.tax-group.${display(group.code)}`}>
          {display(group.code)} • {display(group.rate)}% • {display(group.policy_version)}
        </Text>) : <Text variant="bodyMedium">Няма данъчни групи в API.</Text>}
      </Card.Content></Card>
    </> : null}
  </View>;
}
