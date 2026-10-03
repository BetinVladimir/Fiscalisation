import React from 'react';
import { View } from 'react-native';
import { ActivityIndicator, Button, Card, HelperText, Text, useTheme } from 'react-native-paper';

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
  const theme = useTheme();
  return <View testID="reports.screen" style={{ gap: 12, paddingBottom: 16 }}>
    <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }}>
      <Card.Content style={{ gap: 12, paddingVertical: 16 }}>
        <Text variant="titleLarge" accessibilityRole="header">Команда към фискалното устройство</Text>
        <Text variant="bodyMedium">Показват се данните от фискалното устройство.</Text>
        {canRunReports ? <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
          {types.map(type => <Button
            key={type}
            testID={`reports.create.${type}`}
            mode="contained-tonal"
            contentStyle={{ minHeight: 48 }}
            accessibilityLabel={`Създай ${type} отчет`}
            disabled={loading || !registerValid}
            onPress={() => onReport(type)}
          >{type}</Button>)}
        </View> : <Text variant="bodyMedium">Преглед само за одитор; създаването на отчет изисква SUPERVISOR или ADMIN.</Text>}
        {commandError ? <HelperText testID="reports.command-error" type="error" visible accessibilityLiveRegion="polite">{commandError}</HelperText> : null}
      </Card.Content>
    </Card>
    {loading ? <ActivityIndicator testID="reports.loading" accessibilityLabel="Зареждане на отчети" /> : null}
    {!loading && loadError ? <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="reports.error"><Card.Content style={{ gap: 8 }}>
      <HelperText type="error" visible accessibilityLiveRegion="polite">{loadError}</HelperText>
      <Button mode="outlined" contentStyle={{ minHeight: 48 }} onPress={onRetry}>Опитай отново</Button>
    </Card.Content></Card> : null}
    {!loading && !loadError && reports.length === 0 ? <Text testID="reports.empty" variant="bodyMedium">Няма отчети за избрания филтър.</Text> : null}
    {!loadError && reports.map(report => <Card key={display(report.id)} mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID={`reports.item.${display(report.id)}`}>
      <Card.Content style={{ gap: 6, paddingVertical: 16 }}>
        <Text variant="titleMedium">{display(report.type)} • {display(report.state)}</Text>
        <Text variant="bodyMedium">{display(report.fiscal_reference || report.id)} • {display(report.created_at)}</Text>
      </Card.Content>
    </Card>)}
  </View>;
}
