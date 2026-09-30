import React from 'react';
import type { PropsWithChildren } from 'react';
import { useColorScheme, View } from 'react-native';
import { ActivityIndicator, PaperProvider } from 'react-native-paper';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { Roboto_400Regular, Roboto_500Medium, Roboto_700Bold, useFonts } from '@expo-google-fonts/roboto';
import { BeeloyDarkTheme, BeeloyLightTheme } from './theme';
import { FeedbackProvider } from './feedback';
import { UiPreferencesProvider, useUiPreferences } from './preferences';

function ThemedContent({ children }: PropsWithChildren) {
  const scheme = useColorScheme();
  const { themeMode, hydrated } = useUiPreferences();
  const dark = themeMode === 'dark' || (themeMode === 'system' && scheme === 'dark');
  const theme = dark ? BeeloyDarkTheme : BeeloyLightTheme;
  const [fontsLoaded] = useFonts({ Roboto_400Regular, Roboto_500Medium, Roboto_700Bold });
  return (
    <SafeAreaProvider>
      <PaperProvider theme={theme}>
        {fontsLoaded && hydrated ? <FeedbackProvider>{children}</FeedbackProvider> : (
          <View accessibilityLabel="Загрузка приложения" accessibilityLiveRegion="polite" style={{ flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: theme.colors.background }}>
            <ActivityIndicator animating />
          </View>
        )}
      </PaperProvider>
    </SafeAreaProvider>
  );
}

export function BeeloyAppProvider({ children }: PropsWithChildren) {
  return <UiPreferencesProvider><ThemedContent>{children}</ThemedContent></UiPreferencesProvider>;
}
