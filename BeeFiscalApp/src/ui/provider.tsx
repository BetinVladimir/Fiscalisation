import { withSystemFonts } from '../../../../BeeloyBackend/mobile/shared-ui/src/theme/paper';
import { foundationCopy } from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import React from 'react';
import type { PropsWithChildren } from 'react';
import { AccessibilityInfo, Platform, useColorScheme } from 'react-native';
import { ActivityIndicator, PaperProvider, Surface } from 'react-native-paper';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { Roboto_400Regular, Roboto_500Medium, Roboto_700Bold, useFonts } from '@expo-google-fonts/roboto';
import { BeeloyDarkTheme, BeeloyLightTheme } from './theme';
import { FeedbackProvider } from './feedback';
import { UiPreferencesProvider, useUiPreferences } from './preferences';

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

function ThemedContent({ children }: PropsWithChildren) {
  const scheme = useColorScheme();
  const { themeMode, hydrated, language } = useUiPreferences();
  const loadingLabel = foundationCopy[language].loading;
  const [fontsLoaded, fontError] = useFonts({ Roboto_400Regular, Roboto_500Medium, Roboto_700Bold });
  const dark = themeMode === 'dark' || (themeMode === 'system' && scheme === 'dark');
  const selectedTheme = dark ? BeeloyDarkTheme : BeeloyLightTheme;
  const baseTheme = React.useMemo(() => fontError ? withSystemFonts(selectedTheme) : selectedTheme, [selectedTheme, fontError]);
  const reducedMotion = useReducedMotion();
  const theme = React.useMemo(() => ({ ...baseTheme, animation: { ...baseTheme.animation, scale: reducedMotion ? 0 : baseTheme.animation.scale } }), [baseTheme, reducedMotion]);
  const settings = React.useMemo(() => ({ rippleEffectEnabled: !reducedMotion }), [reducedMotion]);
  return (
    <SafeAreaProvider>
      <PaperProvider theme={theme} settings={settings}>
        {(fontsLoaded || Boolean(fontError)) && hydrated ? <FeedbackProvider>{children}</FeedbackProvider> : (
          <Surface mode="flat" elevation={0} testID="beefiscal.startup.loading" accessibilityLabel={loadingLabel} accessibilityLiveRegion="polite" style={{ flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: theme.colors.background }}>
            <ActivityIndicator animating accessibilityLabel={loadingLabel} />
          </Surface>
        )}
      </PaperProvider>
    </SafeAreaProvider>
  );
}

export function BeeloyAppProvider({ children }: PropsWithChildren) {
  return <UiPreferencesProvider><ThemedContent>{children}</ThemedContent></UiPreferencesProvider>;
}
