import { configureFonts, MD3DarkTheme, MD3LightTheme, type MD3Theme } from 'react-native-paper';
import { darkColors, lightColors } from './tokens';

const fonts = configureFonts({ config: { fontFamily: 'Roboto_400Regular' } });

export const BeeloyLightTheme: MD3Theme = {
  ...MD3LightTheme, dark: false, roundness: 12, fonts,
  colors: { ...MD3LightTheme.colors, ...lightColors },
};
export const BeeloyDarkTheme: MD3Theme = {
  ...MD3DarkTheme, dark: true, roundness: 12, fonts,
  colors: { ...MD3DarkTheme.colors, ...darkColors },
};
/** Compatibility export. Providers select the system theme at runtime. */
export const BeeloyTheme = BeeloyLightTheme;
