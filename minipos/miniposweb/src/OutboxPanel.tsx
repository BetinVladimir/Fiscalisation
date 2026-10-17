import { useId } from 'react';
import { AppButton as Button } from './ui/app-button';
import { ActivityIndicator, Card, HelperText, Text } from "react-native-paper";
import type { OutboxRow } from "./storage/outbox";
import { translations } from "./ui/workTranslations.generated";
import type { Language } from "./ui/PreferenceControls";

function statusKey(row: OutboxRow): keyof typeof translations.en {
  if (!row.result) return "outboxAwaiting";
  if (["UNKNOWN", "RECOVERY_REQUIRED", "EXECUTING", "RESERVED"].includes(row.result.state)) return "outboxUncertain";
  if (["FAILED", "REJECTED"].includes(row.result.state)) return "outboxFailed";
  if (row.posSync && !row.posSync.synced_at) return "outboxPendingSync";
  if (row.posSync?.synced_at) return "outboxSynced";
  if (["COMMITTED", "FISCALIZED"].includes(row.result.state)) return "outboxFiscalized";
  return "outboxUncertain";
}

export function OutboxPanel({ rows, loading, error, language, onRefresh }: {
  rows: OutboxRow[];
  loading: boolean;
  error: boolean;
  language: Language;
  onRefresh: () => void;
}) {
  const t = translations[language], errorId = `miniposweb-outbox-error-${useId().replace(/:/g, "")}`;
  return <Card mode="outlined" style={{ marginTop: 16 }} testID="offline-outbox">
    <Card.Content style={{ gap: 12 }}>
      <Text variant="titleLarge" accessibilityRole="header">{t.outboxTitle}</Text>
      <Button mode="outlined" loading={loading} disabled={loading} aria-describedby={error ? errorId : undefined} onPress={onRefresh} testID="offline-outbox-refresh">{t.outboxRefresh}</Button>
      {loading ? <ActivityIndicator accessibilityLabel={t.loading} testID="offline-outbox-loading" /> : null}
      {error ? <HelperText nativeID={errorId} accessibilityRole="alert" accessibilityLiveRegion="polite" type="error" visible testID="offline-outbox-error">{t.loadFailed}</HelperText> : null}
      {!loading && !error && rows.length === 0 ? <Text variant="bodyMedium" testID="offline-outbox-empty">{t.outboxEmpty}</Text> : null}
      {!loading && !error ? [...rows].sort((a, b) => b.createdAt.localeCompare(a.createdAt)).map(row =>
        <Card key={row.intent.client_operation_id} mode="outlined" testID={`offline-outbox-row-${row.intent.client_operation_id}`}>
          <Card.Content style={{ gap: 4 }}>
            <Text variant="titleMedium" testID={`offline-outbox-status-${row.intent.client_operation_id}`}>{t[statusKey(row)]}</Text>
            <Text variant="bodySmall" selectable style={{ flexShrink: 1 }}>{row.intent.client_operation_id}</Text>
          </Card.Content>
        </Card>) : null}
    </Card.Content>
  </Card>;
}
