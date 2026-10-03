import React, { useEffect, useRef, useState } from 'react';
import { Platform, View } from 'react-native';
import { ActivityIndicator, Button, Card, HelperText, List, Text, TextInput, useTheme } from 'react-native-paper';

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
  const [expanded, setExpanded] = useState(false);
  const id = display(event.event_id);
  const theme = useTheme();
  const accordionRef = useRef<View>(null);
  useEffect(() => {
    // Paper owns the native expanded state. This installed RN Web maps only
    // aria-expanded, while Paper Accordion currently exposes accessibilityState.
    if (Platform.OS !== 'web') return;
    const root = accordionRef.current as unknown as HTMLElement | null;
    root?.querySelector('[role="button"]')?.setAttribute('aria-expanded', String(expanded));
  }, [expanded]);
  return <Card mode="outlined" testID={`audit-event-${id}`}>
    <Card.Content style={{ gap: 6, paddingVertical: 16 }}>
      <Text variant="titleMedium">{display(event.action)} • {display(event.object_type)}</Text>
      <Text variant="bodyMedium">{display(event.occurred_at)}</Text>
      <View ref={accordionRef}>
      <List.Accordion testID={`audit.details.${id}`} expanded={expanded}
        title={expanded ? 'Скрий подробности' : 'Покажи подробности'} titleNumberOfLines={0}
        right={({ isExpanded }) => <View aria-hidden accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
          <List.Icon icon={isExpanded ? 'chevron-up' : 'chevron-down'} color={theme.colors.onSurfaceVariant} />
        </View>}
        style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline }}
        onPress={() => setExpanded(value => !value)}>
      <View testID={`audit.details-content.${id}`} style={{ gap: 4 }}>
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

export function AuditScreen({ events, filters, loading, error, onChange, onApply }: Props) {
  return <View testID="audit-events" style={{ gap: 12, paddingBottom: 16 }}>
    <Card mode="outlined" testID="audit-filters"><Card.Content style={{ gap: 12, paddingVertical: 16 }}>
      <Text variant="titleLarge" accessibilityRole="header">Филтри на одитния журнал</Text>
      {fields.map(({ name, label }) => <TextInput
        key={name} testID={`audit.filter.${name}`} label={label} accessibilityLabel={label}
        mode="outlined" value={filters[name]} onChangeText={value => onChange(name, value)}
        autoCapitalize="none" contentStyle={{ minHeight: 48 }}
      />)}
      <Button testID="apply-audit-filters" mode="contained" contentStyle={{ minHeight: 48 }}
        disabled={loading} onPress={onApply}>Приложи филтрите</Button>
    </Card.Content></Card>
    {loading ? <ActivityIndicator testID="audit.loading" accessibilityLabel="Зареждане на одитния журнал" /> : null}
    {!loading && error ? <Card mode="outlined" testID="audit.error"><Card.Content style={{ gap: 8 }}>
      <HelperText type="error" visible accessibilityLiveRegion="polite">{error}</HelperText>
      <Button mode="outlined" contentStyle={{ minHeight: 48 }} onPress={onApply}>Опитай отново</Button>
    </Card.Content></Card> : null}
    {!loading && !error && events.length === 0 ? <Text testID="audit.empty" variant="bodyMedium">Няма записи за избраните филтри.</Text> : null}
    {!error && events.map(event => <AuditEventCard key={display(event.event_id)} event={event} />)}
  </View>;
}
