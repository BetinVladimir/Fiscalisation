import { MD3DarkTheme, MD3LightTheme, configureFonts, type MD3Theme, type MD3TypescaleKey } from 'react-native-paper';
import { darkColors, lightColors } from './tokens';

/** The loaded Expo Roboto faces back every Paper MD3 typography variant. */
const fontConfig = Object.fromEntries(
  Object.entries(MD3LightTheme.fonts).map(([variant, style]) => {
    const weight = Number(style.fontWeight);
    const fontFamily = weight >= 700 ? 'Roboto_700Bold' : weight >= 500 ? 'Roboto_500Medium' : 'Roboto_400Regular';
    return [variant, { fontFamily }];
  }),
) as Partial<Record<MD3TypescaleKey, { fontFamily: string }>>;
const fonts = configureFonts({ config: fontConfig });

export const BeeloyLightTheme: MD3Theme = {
  ...MD3LightTheme,
  fonts,
  colors: { ...MD3LightTheme.colors, ...lightColors },
};

export const BeeloyDarkTheme: MD3Theme = {
  ...MD3DarkTheme,
  fonts,
  colors: { ...MD3DarkTheme.colors, ...darkColors },
};

export const BeeloyTheme = BeeloyLightTheme;
