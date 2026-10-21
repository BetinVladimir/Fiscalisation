import {useUiPreferences} from '../ui/preferences';
import {deviceScreenLabel} from '../ui/device-screen-copy';
import {AppButton as Button} from '../ui/app-button';
import React, { useEffect, useRef, useState } from 'react';
import { Platform, View } from 'react-native';
import { Card, HelperText, RadioButton, Text, useTheme } from 'react-native-paper';
import { FormField } from '../ui/form-field';

export type Item = Record<string, unknown>;
export const label = (value: unknown) => typeof value === 'string' ? value : typeof value === 'number' ? String(value) : '—';

function useCurrentControl(disabled: boolean) {
  const mounted = useRef(false), snapshot = {}, latest = useRef(snapshot); latest.current = snapshot;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  return () => mounted.current && latest.current === snapshot && !disabled;
}
/** Keyboard adapter only; visible rows and activation remain genuine Paper radio items. */
function useRadioKeyboard(disabled: boolean, selectedIndex: number) {
  const ref = useRef<View>(null);
  useEffect(() => {
    if (Platform.OS !== 'web') return;
    const root = ref.current as unknown as HTMLElement | null;
    if (!root) return;
    const allRadios = Array.from(root.querySelectorAll<HTMLElement>('[role="radio"]'));
    const radios = allRadios.filter(node => !node.parentElement?.closest('[role="radio"]'));
    // Paper exposes both the whole interactive row and its decorative glyph as
    // radios. Only the row participates in the group on web. Paper's ripple
    // drops aria-checked, so derive it from the explicit current selection.
    radios.forEach((node, index) => { node.setAttribute('aria-checked', String(index === selectedIndex)); });
    allRadios.filter(node => !radios.includes(node)).forEach(node => {
      node.setAttribute('aria-hidden', 'true'); node.setAttribute('role', 'presentation'); node.tabIndex = -1;
    });
    const enabled = radios.filter(node => node.getAttribute('aria-disabled') !== 'true');
    const selected = enabled.find(node => node.getAttribute('aria-checked') === 'true') ?? enabled[0];
    radios.forEach(node => { node.tabIndex = !disabled && node === selected ? 0 : -1; });
    const keydown = (event: KeyboardEvent) => {
      if (disabled || event.altKey || event.ctrlKey || event.metaKey || !enabled.length) return;
      const index = enabled.findIndex(node => node === event.target || node.contains(event.target as Node));
      if (index < 0) return;
      let next: number;
      if (event.key === 'ArrowDown' || event.key === 'ArrowRight') next = (index + 1) % enabled.length;
      else if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') next = (index + enabled.length - 1) % enabled.length;
      else if (event.key === 'Home') next = 0;
      else if (event.key === 'End') next = enabled.length - 1;
      else return;
      event.preventDefault();
      enabled.forEach((node, i) => { node.tabIndex = i === next ? 0 : -1; });
      enabled[next].focus(); enabled[next].click();
    };
    root.addEventListener('keydown', keydown);
    return () => root.removeEventListener('keydown', keydown);
  });
  return ref;
}
const itemId = (item: Item) => typeof item.id === 'string' && item.id.trim() ? item.id : typeof item.id === 'number' && Number.isFinite(item.id) ? String(item.id) : '';
function uniqueItems(items: Item[]) {
  const counts = new Map<string, number>();
  items.forEach(item => { const id = itemId(item); counts.set(id, (counts.get(id) || 0) + 1); });
  return items.filter(item => itemId(item) && counts.get(itemId(item)) === 1);
}

export function Field({ label: title, value, onChangeText, testID, disabled = false }: {
  label: string; value: string; onChangeText: (value: string) => void; testID?: string; disabled?: boolean;
}) {
  const current = useCurrentControl(disabled);
  return <FormField editable={!disabled} testID={testID} mode="outlined" label={title} accessibilityLabel={title}
    value={value} onChangeText={next => { if (current()) onChangeText(next); }} style={{ width: '100%', minWidth: 0 }} />;
}

export function Action({ label: title, onPress, disabled, testID }: {
  label: string; onPress: () => void; disabled: boolean; testID: string;
}) {
  return <Button testID={testID} mode="contained-tonal" contentStyle={{ minHeight: 48 }}
    accessibilityLabel={title} disabled={disabled} onPress={onPress}>{title}</Button>;
}

export function AdminCard({ title, count, children }: { title: string; count: number; children: React.ReactNode }) {
  const theme = useTheme();
  return <Card mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface, flexBasis: 300, flexGrow: 1, minWidth: 0 }}><Card.Content style={{ gap: 12, paddingVertical: 16 }}>
    <Text variant="titleLarge" accessibilityRole="header">{title}</Text>
    <Text variant="bodyMedium">Регистрирани: {count}</Text>
    {children}
  </Card.Content></Card>;
}

export function ReferencePicker({ label: title, items, value, labelFor, onSelect, onEmpty, required = true, disabled = false }: {
  label: string; items: Item[]; value: string; labelFor: (item: Item) => string;
  onSelect: (id: string) => void; onEmpty?: () => void; required?: boolean; disabled?: boolean;
}) {
  const [query, setQuery] = useState('');
  const {language}=useUiPreferences();
  const ui=(value:string)=>deviceScreenLabel(language,value);
  const theme = useTheme();
  const current = useCurrentControl(disabled);
  const selectable = uniqueItems(items);
  const visible = selectable.filter(item => labelFor(item).toLowerCase().includes(query.trim().toLowerCase()));
  const groupRef = useRadioKeyboard(disabled, visible.findIndex(item => itemId(item) === value));
  return <View style={{ gap: 8, minWidth: 0, width: '100%', maxWidth: 320 }}>
    {items.length ? <>
      <FormField editable={!disabled} testID={`reference.search.${title}`} mode="outlined" label={`${ui('Търсене')}: ${title}`} accessibilityLabel={`${ui('Търсене')}: ${title}`} value={query} onChangeText={next => { if (current()) setQuery(next); }} />
      <View ref={groupRef} accessibilityRole="radiogroup" accessibilityLabel={title} style={{ gap: 4 }}>
        {visible.map(item => <RadioButton.Item key={itemId(item)} value={itemId(item)}
          label={labelFor(item)} labelStyle={{ flexShrink: 1 }}
          status={itemId(item) === value ? 'checked' : 'unchecked'} disabled={disabled}
          accessibilityLabel={labelFor(item)}
          style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: 8, backgroundColor: itemId(item) === value ? theme.colors.secondaryContainer : theme.colors.surface }}
          onPress={() => { if (current() && selectable.some(candidate => itemId(candidate) === itemId(item))) onSelect(itemId(item)); }} />)}
      </View>
      {!visible.length ? <HelperText type="info" visible>{ui('Няма съвпадения.')}</HelperText> : null}
    </> : <>
      <HelperText type="info" visible>{ui(required ? 'Справочникът е празен.' : 'Няма налични записи.')}</HelperText>
      {required && onEmpty ? <Button mode="outlined" contentStyle={{ minHeight: 48 }} disabled={disabled} onPress={() => { if (current()) onEmpty(); }}>{ui('Попълни справочника')}</Button> : null}
    </>}
  </View>;
}

export function Choices({ items, selectedId, labelFor, onSelect, disabled = false }: {
  items: Item[]; selectedId: string; labelFor: (item: Item) => string; onSelect: (item: Item) => void; disabled?: boolean;
}) {
  const theme = useTheme(), current = useCurrentControl(disabled), selectable = uniqueItems(items);
  const groupRef = useRadioKeyboard(disabled, selectable.findIndex(item => itemId(item) === selectedId));
  if (selectable.length === 0) return <Text variant="bodyMedium">Все още няма записи.</Text>;
  return <View ref={groupRef} accessibilityRole="radiogroup" accessibilityLabel="Съществуващи записи"
    style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8, paddingVertical: 2, minWidth: 0 }}>
    {selectable.map(item => <RadioButton.Item key={itemId(item)} value={itemId(item)}
      label={labelFor(item)} labelStyle={{ flexShrink: 1 }}
      status={itemId(item) === selectedId ? 'checked' : 'unchecked'} disabled={disabled}
      accessibilityLabel={labelFor(item)}
      style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: 8, flexBasis: 220, flexGrow: 1, minWidth: 0, maxWidth: '100%' }}
      onPress={() => { if (current() && selectable.some(candidate => itemId(candidate) === itemId(item))) onSelect(item); }} />)}
  </View>;
}
