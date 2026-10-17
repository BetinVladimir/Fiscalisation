import { AppButton } from '../ui/app-button';
import { auditFilterDatesValid } from '../auditFilters';
import { useCurrentAction } from '../ui/use-current-action';
import { useUiPreferences } from '../ui/preferences';
import { foundationCopy } from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import { FormField } from '../ui/form-field';
import React, { useEffect, useId, useRef, useState } from 'react';
import { Platform, View } from 'react-native';
import { ActivityIndicator, Card, HelperText, List, Text, useTheme } from 'react-native-paper';

type Event = Record<string, unknown>;
export type AuditFilters = {
  actor: string;
  action: string;
  objectType: string;
  objectId: string;
  unp: string;
  from: string;
  to: string;
};
type Props = {
  events: Event[];
  filters: AuditFilters;
  loading: boolean;
  error: string;
  onChange: (name: keyof AuditFilters, value: string) => void;
  onApply: () => void;
};
const fields: { name: keyof AuditFilters; label: string }[] = [
  { name: 'actor', label: 'Оператор/участник' },
  { name: 'action', label: 'Действие' },
  { name: 'objectType', label: 'Тип обект' },
  { name: 'objectId', label: 'Идентификатор на обект' },
  { name: 'unp', label: 'УНП' },
  { name: 'from', label: 'От (RFC 3339)' },
  { name: 'to', label: 'До (RFC 3339)' },
];
const display = (value: unknown) => typeof value === 'string' || typeof value === 'number' ? String(value) : '—';

function AuditEventCard({ event }: { event: Event }) {
  const admit = useCurrentAction();
  const detailsId = `beefiscal-audit-details-${useId().replace(/:/g, '')}`;
  const [expanded, setExpanded] = useState(false);
  const id = display(event.event_id);
  const theme = useTheme();
  const accordionRef = useRef<View>(null);
  useEffect(() => {
    // Paper owns the native expanded state. This installed RN Web maps only
    // aria-expanded, while Paper Accordion currently exposes accessibilityState.
    if (Platform.OS !== 'web') return;
    const root = accordionRef.current as unknown as HTMLElement | null;
    const trigger = root?.querySelector('[role="button"]');
    trigger?.setAttribute('aria-expanded', String(expanded));
    if (expanded) trigger?.setAttribute('aria-controls', detailsId); else trigger?.removeAttribute('aria-controls');
  }, [expanded, detailsId, theme.dark]);
  return <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID={`audit-event-${id}`}>
    <Card.Content style={{ gap: 6, paddingVertical: 16 }}>
      <Text variant="titleMedium">{display(event.action)} • {display(event.object_type)}</Text>
      <Text variant="bodyMedium">{display(event.occurred_at)}</Text>
      <View ref={accordionRef}>
      <List.Accordion titleMaxFontSizeMultiplier={0} testID={`audit.details.${id}`} expanded={expanded}
        title={expanded ? 'Скрий подробности' : 'Покажи подробности'} titleNumberOfLines={0}
        right={({ isExpanded }) => <View aria-hidden accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
          <List.Icon icon={isExpanded ? 'chevron-up' : 'chevron-down'} color={theme.colors.onSurfaceVariant} />
        </View>}
        style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline }}
        onPress={() => { if (admit()) setExpanded(value => !value); }}>
      <View nativeID={detailsId} testID={`audit.details-content.${id}`} style={{ gap: 4 }}>
        <Text variant="bodyMedium">Обект: {display(event.object_id)}</Text>
        <Text variant="bodyMedium">Участник: {display(event.actor_id)}</Text>
        <Text variant="bodyMedium">УНП: {display(event.unp)}</Text>
        <Text variant="bodySmall">Hash: {display(event.event_hash)}</Text>
      </View>
      </List.Accordion>
      </View>
    </Card.Content>
  </Card>;
}

export function AuditScreen({ events, filters, loading, error: loadError, onChange, onApply }: Props) {
  const theme = useTheme(), admit = useCurrentAction(), { language } = useUiPreferences();
  const validDates = auditFilterDatesValid(filters);
  const validEvents = Array.isArray(events) && events.every(event => event && typeof event.event_id === 'string' && !!event.event_id.trim())
    && new Set(events.map(event => event.event_id)).size === events.length;
  const error = loadError || (!validEvents ? foundationCopy[language].error : '');
  const errorId = `beefiscal-audit-error-${useId().replace(/:/g, '')}`;
  return <View testID="audit-events" style={{ gap: 12, paddingBottom: 16 }}>
    <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="audit-filters"><Card.Content style={{ gap: 12, paddingVertical: 16 }}>
      <Text variant="titleLarge" accessibilityRole="header">Филтри на одитния журнал</Text>
      {fields.map(({ name, label }) => <FormField
        key={name} testID={`audit.filter.${name}`} label={label} accessibilityLabel={label}
        editable={!loading} error={(name === 'from' || name === 'to') && !validDates}
        errorMessage={(name === 'from' || name === 'to') && !validDates ? foundationCopy[language].error : undefined}
        mode="outlined" value={filters[name]} onChangeText={value => onChange(name, value)}
        autoCapitalize="none" contentStyle={{ minHeight: 48 }}
      />)}
      <AppButton testID="apply-audit-filters" mode="contained" contentStyle={{ minHeight: 48 }}
        disabled={loading || !validDates} onPress={() => { if (!loading && validDates && admit()) onApply(); }}>Приложи филтрите</AppButton>
    </Card.Content></Card>
    {loading ? <ActivityIndicator testID="audit.loading" accessibilityLabel="Зареждане на одитния журнал" /> : null}
    {!loading && error ? <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }} testID="audit.error"><Card.Content style={{ gap: 8 }}>
      <HelperText nativeID={errorId} accessibilityRole="alert" type="error" visible accessibilityLiveRegion="polite">{error}</HelperText>
      <AppButton testID="audit.retry" aria-describedby={Platform.OS === 'web' ? errorId : undefined} disabled={loading} mode="outlined" contentStyle={{ minHeight: 48 }} onPress={onApply}>Опитай отново</AppButton>
    </Card.Content></Card> : null}
    {!loading && !error && events.length === 0 ? <Text testID="audit.empty" variant="bodyMedium">Няма записи за избраните филтри.</Text> : null}
    {!loading && !error && events.map(event => <AuditEventCard key={display(event.event_id)} event={event} />)}
  </View>;
}
