import React, { useEffect, useState } from 'react';
import { Platform, ScrollView, View } from 'react-native';
import { Button, Card, List } from 'react-native-paper';
import { authCopy } from './auth-copy.generated';
import { languageOptions, useUiPreferences, type ThemeMode } from './preferences';

export function LanguageThemeControls({ testPrefix = 'auth' }: { testPrefix?: string }) {
  const { language, themeMode, setLanguage, setThemeMode } = useUiPreferences();
  const [languageOpen, setLanguageOpen] = useState(false);
  const copy = authCopy[language];
  const modes: ThemeMode[] = ['system', 'light', 'dark'];
  const nextMode = modes[(modes.indexOf(themeMode) + 1) % modes.length];
  const modeLabel = themeMode === 'system' ? copy.themeSystem : themeMode === 'light' ? copy.themeLight : copy.themeDark;

  useEffect(() => {
    if (!languageOpen || Platform.OS !== 'web') return;
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') setLanguageOpen(false); };
    document.addEventListener('keydown', closeOnEscape);
    return () => document.removeEventListener('keydown', closeOnEscape);
  }, [languageOpen]);

  return <View style={{ paddingHorizontal: 16, paddingTop: 8, gap: 8 }}>
    <View style={{ flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'flex-end', gap: 8 }}>
      <Button testID={`${testPrefix}.language.open`} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.language}: ${language.toUpperCase()}`} accessibilityState={{ expanded: languageOpen }} onPress={() => setLanguageOpen(open => !open)}>{language.toUpperCase()}</Button>
      <Button testID={`${testPrefix}.theme.toggle`} mode="outlined" contentStyle={{ minHeight: 48 }} accessibilityLabel={`${copy.theme}: ${modeLabel}`} onPress={() => setThemeMode(nextMode)}>{modeLabel}</Button>
    </View>
    {languageOpen ? <Card mode="outlined" style={{ width: '100%', maxWidth: 320, alignSelf: 'flex-end' }}>
      <ScrollView testID={`${testPrefix}.language.list`} accessibilityRole="menu" style={{ height: 224 }} keyboardShouldPersistTaps="handled">
        {languageOptions.map(([code, name]) => <List.Item
          key={code}
          testID={`${testPrefix}.language.${code}`}
          title={code === language ? `✓ ${name}` : name}
          accessibilityRole="menuitem"
          accessibilityLabel={name}
          accessibilityState={{ selected: code === language }}
          onPress={() => { setLanguage(code); setLanguageOpen(false); }}
        />)}
      </ScrollView>
    </Card> : null}
  </View>;
}
