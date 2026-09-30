import React, { useState } from 'react';
import { View } from 'react-native';
import { Button, Card, HelperText, List, Text, TextInput, useTheme } from 'react-native-paper';
import { ResponsiveScrollView as ScrollView } from '../ui/responsive-scroll-view';

export type Item = Record<string, unknown>;
export const label = (value: unknown) => typeof value === 'string' ? value : typeof value === 'number' ? String(value) : '—';

export function Field({ label: title, value, onChangeText, testID }: {
  label: string; value: string; onChangeText: (value: string) => void; testID?: string;
}) {
  return <TextInput testID={testID} mode="outlined" label={title} accessibilityLabel={title}
    value={value} onChangeText={onChangeText} style={{ width: '100%', minWidth: 0 }} />;
}

export function Action({ label: title, onPress, disabled, testID }: {
  label: string; onPress: () => void; disabled: boolean; testID: string;
}) {
  return <Button testID={testID} mode="contained-tonal" contentStyle={{ minHeight: 48 }}
    accessibilityLabel={title} disabled={disabled} onPress={onPress}>{title}</Button>;
}

export function AdminCard({ title, count, children }: { title: string; count: number; children: React.ReactNode }) {
  return <Card mode="outlined" style={{ flexBasis: 300, flexGrow: 1, minWidth: 0 }}><Card.Content style={{ gap: 12, paddingVertical: 16 }}>
    <Text variant="titleLarge" accessibilityRole="header">{title}</Text>
    <Text variant="bodyMedium">Регистрирани: {count}</Text>
    {children}
  </Card.Content></Card>;
}

export function ReferencePicker({ label: title, items, value, labelFor, onSelect, onEmpty, required = true }: {
  label: string; items: Item[]; value: string; labelFor: (item: Item) => string;
  onSelect: (id: string) => void; onEmpty?: () => void; required?: boolean;
}) {
  const [query, setQuery] = useState('');
  const theme = useTheme();
  const visible = items.filter(item => labelFor(item).toLowerCase().includes(query.trim().toLowerCase()));
  return <View style={{ gap: 8, minWidth: 0, width: '100%', maxWidth: 320 }}>
    {items.length ? <>
      <TextInput mode="outlined" label={`Търсене: ${title}`} accessibilityLabel={`Търсене: ${title}`} value={query} onChangeText={setQuery} />
      <View style={{ gap: 4 }}>
        {visible.map(item => <List.Item key={label(item.id)}
          title={item.id === value ? `✓ ${labelFor(item)}` : labelFor(item)} titleNumberOfLines={3}
          accessibilityRole="button" accessibilityLabel={item.id === value ? `Избрано: ${labelFor(item)}` : labelFor(item)} accessibilityState={{ selected: item.id === value }}
          style={{ borderRadius: 8, backgroundColor: item.id === value ? theme.colors.secondaryContainer : theme.colors.surface }}
          onPress={() => onSelect(label(item.id))} />)}
      </View>
      {!visible.length ? <HelperText type="info" visible>Няма съвпадения.</HelperText> : null}
    </> : <>
      <HelperText type="info" visible>{required ? 'Справочникът е празен.' : 'Няма налични записи.'}</HelperText>
      {required && onEmpty ? <Button mode="outlined" contentStyle={{ minHeight: 48 }} onPress={onEmpty}>Попълни справочника</Button> : null}
    </>}
  </View>;
}

export function Choices({ items, selectedId, labelFor, onSelect }: {
  items: Item[]; selectedId: string; labelFor: (item: Item) => string; onSelect: (item: Item) => void;
}) {
  if (items.length === 0) return <Text variant="bodyMedium">Все още няма записи.</Text>;
  return <ScrollView horizontal accessibilityRole="toolbar" accessibilityLabel="Съществуващи записи"
    contentContainerStyle={{ gap: 8, paddingVertical: 2 }}>
    {items.map(item => <List.Item key={label(item.id)}
      title={item.id === selectedId ? `✓ ${labelFor(item)}` : labelFor(item)} titleNumberOfLines={3}
      accessibilityRole="button" accessibilityLabel={item.id === selectedId ? `Избрано: ${labelFor(item)}` : labelFor(item)} accessibilityState={{ selected: item.id === selectedId }}
      style={{ minWidth: 140, maxWidth: 250 }} onPress={() => onSelect(item)} />)}
  </ScrollView>;
}
