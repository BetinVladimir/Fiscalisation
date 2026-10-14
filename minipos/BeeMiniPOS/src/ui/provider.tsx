import { withSystemFonts } from '../../../../../BeeloyBackend/mobile/shared-ui/src/theme/paper';
import { foundationCopy } from '../../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import { useLanguageStore } from '../languageStore';
import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { PropsWithChildren } from 'react';
import { AccessibilityInfo, Platform, useColorScheme } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { ActivityIndicator, PaperProvider, Surface } from 'react-native-paper';
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

function useReducedMotion() {
  const [reduced, setReduced] = React.useState(() => Platform.OS === 'web' && typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia('(prefers-reduced-motion: reduce)').matches : false);
  React.useEffect(() => {
    if (Platform.OS === 'web') {
      if(typeof window==='undefined'||typeof window.matchMedia!=='function')return;
      const query = window.matchMedia('(prefers-reduced-motion: reduce)');
      let active=true;
      const update = () => {if(active)setReduced(query.matches);};
      update();
      if(typeof query.addEventListener==='function')query.addEventListener('change',update);
      else if(typeof query.addListener==='function')query.addListener(update);
      return () => {active=false;if(typeof query.removeEventListener==='function')query.removeEventListener('change',update);else if(typeof query.removeListener==='function')query.removeListener(update);};
    }
    let active = true,changed=false;
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', value=>{if(active){changed=true;setReduced(value);}});
    void AccessibilityInfo.isReduceMotionEnabled().then(value => { if (active&&!changed) setReduced(value); }).catch(() => undefined);
    return () => { active = false; subscription.remove(); };
  }, []);
  return reduced;
}

export function BeeloyAppProvider({ children }: PropsWithChildren) {
  const scheme = useColorScheme();
  const [preference, setPreference] = useState<ThemePreference>('system');
  const [hydrated, setHydrated] = useState(false);
  const dark = preference === 'dark' || (preference === 'system' && scheme === 'dark');
  const selectedTheme = dark ? BeeloyDarkTheme : BeeloyLightTheme;
  const [fontsLoaded, fontError] = useFonts({ Roboto_400Regular, Roboto_500Medium, Roboto_700Bold });
  const reducedMotion = useReducedMotion();
  const language = useLanguageStore(state => state.language);
  const loadingLabel = foundationCopy[language].loading;
  const theme = useMemo(() => {
    const base = fontError ? withSystemFonts(selectedTheme) : selectedTheme;
    return { ...base, animation: { ...base.animation, scale: reducedMotion ? 0 : base.animation.scale } };
  }, [selectedTheme, fontError, reducedMotion]);
  const settings = useMemo(() => ({ rippleEffectEnabled: !reducedMotion }), [reducedMotion]);
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
      <PaperProvider theme={theme} settings={settings}>
          {(fontsLoaded || Boolean(fontError)) && hydrated ? <ThemePreferenceContext.Provider value={preferenceValue}><FeedbackProvider>{children}</FeedbackProvider></ThemePreferenceContext.Provider> : (
            <Surface mode="flat" elevation={0} testID="beeminipos.startup.loading" accessibilityLabel={loadingLabel} accessibilityLiveRegion="polite" style={{ flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: theme.colors.background }}>
              <ActivityIndicator animating accessibilityLabel={loadingLabel} />
            </Surface>
          )}
      </PaperProvider>
    </SafeAreaProvider>
  );
}
