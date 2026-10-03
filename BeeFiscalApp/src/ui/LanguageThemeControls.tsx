import React, { useEffect, useRef, useState } from 'react';
import { Platform, ScrollView, View } from 'react-native';
import { Button, Card, List, useTheme } from 'react-native-paper';
import { authCopy } from './auth-copy.generated';
import { languageOptions, useUiPreferences, type ThemeMode } from './preferences';

export function LanguageThemeControls({ testPrefix = 'auth' }: { testPrefix?: string }) {
  const { language, themeMode, setLanguage, setThemeMode } = useUiPreferences();
  const theme = useTheme();
  const controlsRef = useRef<View>(null);
  const menuRef = useRef<View>(null);
  const [languageOpen, setLanguageOpen] = useState(false);
  const copy = authCopy[language];
  const modes: ThemeMode[] = ['system', 'light', 'dark'];
  const nextMode = modes[(modes.indexOf(themeMode) + 1) % modes.length];
  const modeLabel = themeMode === 'system' ? copy.themeSystem : themeMode === 'light' ? copy.themeLight : copy.themeDark;

  const triggerNode = () => Platform.OS === 'web'
    ? (controlsRef.current as unknown as HTMLElement | null)?.querySelector<HTMLElement>('[role="button"]')
    : undefined;
  const closeLanguage = (restoreFocus: boolean) => {
    setLanguageOpen(false);
    if (restoreFocus) triggerNode()?.focus();
  };

  useEffect(() => {
    if (Platform.OS !== 'web') return;
    // This Paper Button overrides accessibilityState with disabled; installed
    // RN Web requires explicit ARIA on the actual own trigger, not its wrapper.
    const trigger = triggerNode();
    trigger?.setAttribute('aria-expanded', String(languageOpen));
    trigger?.setAttribute('aria-haspopup', 'menu');
    if (languageOpen) trigger?.setAttribute('aria-controls', `${testPrefix}-language-menu`);
    else trigger?.removeAttribute('aria-controls');
  }, [languageOpen, testPrefix]);

  useEffect(() => {
    if (!languageOpen || Platform.OS !== 'web') return;
    const root = menuRef.current as unknown as HTMLElement | null;
    if (!root) return;
    const items = Array.from(root.querySelectorAll<HTMLElement>('[role="menuitem"]'));
    let index = Math.max(0, languageOptions.findIndex(([code]) => code === language));
    items.forEach((item, i) => {
      item.setAttribute('role', 'menuitemradio');
      item.setAttribute('aria-checked', String(languageOptions[i][0] === language));
      item.tabIndex = i === index ? 0 : -1;
    });
    const move = (next: number) => {
      if (!items.length) return;
      index = (next + items.length) % items.length;
      items.forEach((item, i) => { item.tabIndex = i === index ? 0 : -1; });
      items[index].focus();
      items[index].scrollIntoView({ block: 'nearest' });
    };
    move(index);
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); closeLanguage(true); }
      else if (event.key === 'ArrowDown') { event.preventDefault(); move(index + 1); }
      else if (event.key === 'ArrowUp') { event.preventDefault(); move(index - 1); }
      else if (event.key === 'Home') { event.preventDefault(); move(0); }
      else if (event.key === 'End') { event.preventDefault(); move(items.length - 1); }
      else if (event.key === 'Tab') {
        event.preventDefault(); closeLanguage(false);
        const controls = (controlsRef.current as unknown as HTMLElement | null)?.querySelectorAll<HTMLElement>('[role="button"]');
        controls?.[event.shiftKey ? 0 : 1]?.focus();
      }
    };
    root.addEventListener('keydown', onKey);
    const escapeOutside = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !root.contains(event.target as Node)) closeLanguage(true);
    };
    document.addEventListener('keydown', escapeOutside);
    return () => { root.removeEventListener('keydown', onKey); document.removeEventListener('keydown', escapeOutside); };
  }, [languageOpen, language, testPrefix]);

  return <View style={{ paddingHorizontal: 16, paddingTop: 8, gap: 8 }}>
    <View ref={controlsRef} style={{ flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'flex-end', gap: 8 }}>
      <Button testID={`${testPrefix}.language.open`} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.language}: ${language.toUpperCase()}`} accessibilityState={{ expanded: languageOpen }} onPress={() => setLanguageOpen(open => !open)}>{language.toUpperCase()}</Button>
      <Button testID={`${testPrefix}.theme.toggle`} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.theme}: ${modeLabel}`} onPress={() => setThemeMode(nextMode)}>{modeLabel}</Button>
    </View>
    {languageOpen ? <Card mode="outlined" style={{ width: '100%', maxWidth: 320, alignSelf: 'flex-end' }}>
      <View ref={menuRef}><ScrollView nativeID={`${testPrefix}-language-menu`} testID={`${testPrefix}.language.list`} accessibilityRole="menu" style={{ height: 224 }} keyboardShouldPersistTaps="handled">
        {languageOptions.map(([code, name]) => <List.Item
          key={code}
          testID={`${testPrefix}.language.${code}`}
          style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: 8, marginVertical: 2 }}
          titleNumberOfLines={0}
          title={code === language ? `✓ ${name}` : name}
          accessibilityRole="menuitem"
          accessibilityLabel={name}
          accessibilityState={{ selected: code === language }}
          onPress={() => { setLanguage(code); closeLanguage(true); }}
        />)}
      </ScrollView></View>
    </Card> : null}
  </View>;
}
