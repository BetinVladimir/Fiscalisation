import React from 'react';
import type { PropsWithChildren } from 'react';
import { AccessibilityInfo, Platform, useColorScheme, View } from 'react-native';
import { ActivityIndicator, PaperProvider } from 'react-native-paper';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { Roboto_400Regular, Roboto_500Medium, Roboto_700Bold, useFonts } from '@expo-google-fonts/roboto';
import { BeeloyDarkTheme, BeeloyLightTheme } from './theme';
import { FeedbackProvider } from './feedback';
import { UiPreferencesProvider, useUiPreferences } from './preferences';

function useReducedMotion() {
  const [reduced, setReduced] = React.useState(() => Platform.OS === 'web' && typeof window !== 'undefined' ? window.matchMedia('(prefers-reduced-motion: reduce)').matches : false);
  React.useEffect(() => {
    if (Platform.OS === 'web') {
      const query = window.matchMedia('(prefers-reduced-motion: reduce)');
      const update = () => setReduced(query.matches);
      update(); query.addEventListener('change', update);
      return () => query.removeEventListener('change', update);
    }
    let active = true;
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduced);
    void AccessibilityInfo.isReduceMotionEnabled().then(value => { if (active) setReduced(value); }).catch(() => undefined);
    return () => { active = false; subscription.remove(); };
  }, []);
  return reduced;
}

function ThemedContent({ children }: PropsWithChildren) {
  const scheme = useColorScheme();
  const { themeMode, hydrated } = useUiPreferences();
  const dark = themeMode === 'dark' || (themeMode === 'system' && scheme === 'dark');
  const baseTheme = dark ? BeeloyDarkTheme : BeeloyLightTheme;
  const reducedMotion = useReducedMotion();
  const theme = React.useMemo(() => ({ ...baseTheme, animation: { ...baseTheme.animation, scale: reducedMotion ? 0 : baseTheme.animation.scale } }), [baseTheme, reducedMotion]);
  const settings = React.useMemo(() => ({ rippleEffectEnabled: !reducedMotion }), [reducedMotion]);
  const [fontsLoaded] = useFonts({ Roboto_400Regular, Roboto_500Medium, Roboto_700Bold });
  return (
    <SafeAreaProvider>
      <PaperProvider theme={theme} settings={settings}>
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
