import {reportCopy} from '../ui/report-copy';
import { AppButton } from '../ui/app-button';
import React, { useId } from 'react';
import { useCurrentAction } from '../ui/use-current-action';
import { foundationCopy } from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import { useUiPreferences } from '../ui/preferences';
import { Platform, View } from 'react-native';
import { ActivityIndicator, Card, HelperText, Text, useTheme } from 'react-native-paper';

type Report = Record<string, unknown>;
type ReportType = 'X' | 'Z' | 'KLEN' | 'FISCAL_MEMORY';

type ReportsScreenProps = {
  reports: Report[];
  loading: boolean;
  canRunReports: boolean;
  registerValid: boolean;
  loadError: string;
  commandError: string;
  onReport: (type: ReportType) => void;
  onRetry: () => void;
};

const types: ReportType[] = ['X', 'Z', 'KLEN', 'FISCAL_MEMORY'];
const display = (value: unknown) => typeof value === 'string' || typeof value === 'number' ? String(value) : '—';

export function ReportsScreen({ reports, loading, canRunReports, registerValid, loadError, commandError, onReport, onRetry }: ReportsScreenProps) {
  const theme = useTheme(), admit = useCurrentAction(), { language } = useUiPreferences();
  const copy=reportCopy(language);
  const valid = reports.every(report => report && typeof report.id === 'string' && !!report.id.trim()
    && (report.type === undefined || typeof report.type === 'string') && (report.state === undefined || typeof report.state === 'string'))
    && new Set(reports.map(report => report.id)).size === reports.length;
  const error = loadError ? copy.loadError : (!valid ? foundationCopy[language].error : '');
  const errorId = `beefiscal-report-error-${useId().replace(/:/g, '')}`;
  const commandErrorId = `${errorId}-command`;
  return <View testID="reports.screen" style={{ gap: 12, paddingBottom: 16 }}>
    <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }}>
      <Card.Content style={{ gap: 12, paddingVertical: 16 }}>
        <Text variant="titleLarge" accessibilityRole="header">{copy.heading}</Text>
        <Text variant="bodyMedium">{copy.intro}</Text>
        {canRunReports ? <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
          {types.map(type => <AppButton
            key={type}
            testID={`reports.create.${type}`}
            mode="contained-tonal"
            contentStyle={{ minHeight: 48 }}
            aria-describedby={Platform.OS === 'web' && commandError ? commandErrorId : undefined}
            accessibilityLabel={copy.create.replace('{type}',type)}
            disabled={loading || !registerValid || !!error}
            onPress={() => { if (!loading && !error && canRunReports && registerValid && types.includes(type) && admit()) onReport(type); }}
          >{type}</AppButton>)}
        </View> : <Text variant="bodyMedium">{copy.auditor}</Text>}
        {commandError ? <HelperText nativeID={commandErrorId} accessibilityRole="alert" testID="reports.command-error" type="error" visible accessibilityLiveRegion="polite">{copy.commandError}</HelperText> : null}
      </Card.Content>
    </Card>
    {loading ? <ActivityIndicator testID="reports.loading" accessibilityLabel={copy.loading} /> : null}
    {!loading && error ? <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="reports.error"><Card.Content style={{ gap: 8 }}>
      <HelperText nativeID={errorId} accessibilityRole="alert" type="error" visible accessibilityLiveRegion="polite">{error}</HelperText>
      <AppButton testID="reports.retry" aria-describedby={Platform.OS === 'web' ? errorId : undefined} disabled={loading} mode="outlined" contentStyle={{ minHeight: 48 }} onPress={() => { if (!loading && admit()) onRetry(); }}>{copy.retry}</AppButton>
    </Card.Content></Card> : null}
    {!loading && !error && reports.length === 0 ? <Text testID="reports.empty" variant="bodyMedium">{copy.empty}</Text> : null}
    {!loading && !error && reports.map(report => <Card key={display(report.id)} mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID={`reports.item.${display(report.id)}`}>
      <Card.Content style={{ gap: 6, paddingVertical: 16 }}>
        <Text variant="titleMedium">{display(report.type)} • {display(report.state)}</Text>
        <Text variant="bodyMedium">{display(report.fiscal_reference || report.id)} • {display(report.created_at)}</Text>
      </Card.Content>
    </Card>)}
  </View>;
}
