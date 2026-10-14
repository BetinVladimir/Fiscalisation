import { AppButton } from './app-button';
import { SafeAreaInsetsContext } from 'react-native-safe-area-context';
import React, { useContext, useEffect, useRef, useState } from 'react';
import { Platform, ScrollView, View, useWindowDimensions } from 'react-native';
import { Card, Menu, useTheme } from 'react-native-paper';
import { authCopy } from './auth-copy.generated';
import { languageOptions, useUiPreferences, type ThemeMode } from './preferences';

export function LanguageThemeControls({ testPrefix = 'auth' }: { testPrefix?: string }) {
  const { language, themeMode, setLanguage, setThemeMode } = useUiPreferences();
  const theme = useTheme();
  const { height } = useWindowDimensions();
  const insets = useContext(SafeAreaInsetsContext) ?? { top: 0, bottom: 0, left: 0, right: 0 };
  const [menuTop, setMenuTop] = useState(160);
  const menuHeight = Math.max(0, Math.min(224, height - menuTop - insets.bottom - 16));
  const mounted = useRef(false);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const controlsRef = useRef<View>(null);
  const menuRef = useRef<View>(null);
  const [languageOpen, setLanguageOpen] = useState(false);
  const snapshot = { language, themeMode, languageOpen };
  const consumed = useRef<object | null>(null);
  const current = () => mounted.current;
  const copy = authCopy[language];
  const modes: ThemeMode[] = ['system', 'light', 'dark'];
  const nextMode = modes[(modes.indexOf(themeMode) + 1) % modes.length];
  const modeLabel = themeMode === 'system' ? copy.themeSystem : themeMode === 'light' ? copy.themeLight : copy.themeDark;

  const triggerNode = () => Platform.OS === 'web'
    ? (controlsRef.current as unknown as HTMLElement | null)?.querySelector<HTMLElement>('[role="button"]')
    : undefined;
  const closeLanguage = (restoreFocus: boolean) => {
    if (!current()) return;
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
    trigger?.removeAttribute('aria-controls');
  });

  useEffect(() => {
    if (!languageOpen || Platform.OS !== 'web') return;
    const root = menuRef.current as unknown as HTMLElement | null;
    if (!root) return;
    const items = Array.from(root.querySelectorAll<HTMLElement>('[role="menuitem"],[role="menuitemradio"]'));
    // Layout/theme rerenders must retain the keyboard user's current item.
    const focusedIndex = items.findIndex(item => item === document.activeElement || item.contains(document.activeElement));
    let index = focusedIndex >= 0 ? focusedIndex : Math.max(0, languageOptions.findIndex(([code]) => code === language));
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
  });

  return <View style={{ paddingHorizontal: 16, paddingTop: 8, gap: 8 }}>
    <View ref={controlsRef} style={{ flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'flex-end', gap: 8 }}>
      <AppButton testID={`${testPrefix}.language.open`} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.language}: ${language.toUpperCase()}`} accessibilityState={{ expanded: languageOpen }} onPress={() => { if (current()) setLanguageOpen(open => !open); }}>{language.toUpperCase()}</AppButton>
      <AppButton testID={`${testPrefix}.theme.toggle`} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.theme}: ${modeLabel}`} onPress={() => { if (current() && consumed.current !== snapshot) { consumed.current = snapshot; setThemeMode(nextMode); } }}>{modeLabel}</AppButton>
    </View>
    {languageOpen ? <Card mode="contained" style={{ overflow: 'hidden', borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface, width: '100%', maxWidth: 320, alignSelf: 'flex-end' }}>
      <View ref={menuRef} {...(Platform.OS === 'web' ? { id: `${testPrefix}-language-menu` } : { nativeID: `${testPrefix}-language-menu` })} onLayout={() => { menuRef.current?.measureInWindow((_x, y) => { if (current() && languageOpen) setMenuTop(y); }); }}><ScrollView testID={`${testPrefix}.language.list`} accessibilityRole="menu" style={{ maxHeight: menuHeight, flexShrink: 1 }} nestedScrollEnabled showsVerticalScrollIndicator keyboardShouldPersistTaps="handled">
        {languageOptions.map(([code, name]) => <Menu.Item
          key={code}
          testID={`${testPrefix}.language.${code}`}
          style={{ minHeight: 48, height: 'auto', minWidth: 0, width: '100%', maxWidth: '100%', paddingVertical: 8, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: 8, marginVertical: 2 }}
          containerStyle={{ minWidth: 0, flexShrink: 1 }}
          contentStyle={{ minWidth: 0, maxWidth: '100%', flex: 1 }}
          titleNumberOfLines={0}
          titleStyle={{ flexShrink: 1, textDecorationLine: code === language ? 'underline' : 'none' }}
          title={code === language ? `✓ ${name}` : name}
          accessibilityLabel={name}
          accessibilityState={{ selected: code === language }}
          onPress={() => { if (!current() || !languageOpen || consumed.current === snapshot) return; consumed.current = snapshot; setLanguage(code); closeLanguage(true); }}
        />)}
      </ScrollView></View>
    </Card> : null}
  </View>;
}
