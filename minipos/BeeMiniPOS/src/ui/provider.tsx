import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { PropsWithChildren } from 'react';
import { useColorScheme, View } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { ActivityIndicator, PaperProvider, Portal } from 'react-native-paper';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { Roboto_400Regular, Roboto_500Medium, Roboto_700Bold, useFonts } from '@expo-google-fonts/roboto';
import { BeeloyDarkTheme, BeeloyLightTheme } from './theme';
import { FeedbackProvider } from './feedback';

export type ThemePreference = 'system' | 'light' | 'dark';
type ThemePreferenceValue = { preference: ThemePreference; dark: boolean; selectTheme: (value: ThemePreference) => void };
const themePreferenceKey = 'beeminipos.theme.preference';
const ThemePreferenceContext = createContext<ThemePreferenceValue | null>(null);

export function useThemePreference(): ThemePreferenceValue {
  const value = useContext(ThemePreferenceContext);
  if (!value) throw new Error('useThemePreference requires BeeloyAppProvider');
  return value;
}

export function BeeloyAppProvider({ children }: PropsWithChildren) {
  const scheme = useColorScheme();
  const [preference, setPreference] = useState<ThemePreference>('system');
  const [hydrated, setHydrated] = useState(false);
  const dark = preference === 'dark' || (preference === 'system' && scheme === 'dark');
  const theme = dark ? BeeloyDarkTheme : BeeloyLightTheme;
  const [fontsLoaded] = useFonts({ Roboto_400Regular, Roboto_500Medium, Roboto_700Bold });
  useEffect(() => {
    let active = true;
    void AsyncStorage.getItem(themePreferenceKey)
      .then(value => {
        if (active && (value === 'system' || value === 'light' || value === 'dark')) setPreference(value);
      })
      .catch(() => undefined)
      .finally(() => { if (active) setHydrated(true); });
    return () => { active = false; };
  }, []);
  const selectTheme = useCallback((value: ThemePreference) => {
    setPreference(value);
    void AsyncStorage.setItem(themePreferenceKey, value).catch(() => undefined);
  }, []);
  const preferenceValue = useMemo(() => ({ preference, dark, selectTheme }), [preference, dark, selectTheme]);
  return (
    <SafeAreaProvider>
      <PaperProvider theme={theme}>
        <Portal.Host>
          {fontsLoaded && hydrated ? <ThemePreferenceContext.Provider value={preferenceValue}><FeedbackProvider>{children}</FeedbackProvider></ThemePreferenceContext.Provider> : (
            <View accessibilityLabel="Загрузка приложения" accessibilityLiveRegion="polite" style={{ flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: theme.colors.background }}>
              <ActivityIndicator animating />
            </View>
          )}
        </Portal.Host>
      </PaperProvider>
    </SafeAreaProvider>
  );
}
