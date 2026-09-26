import { configureFonts, MD3DarkTheme, MD3LightTheme, type MD3Theme } from 'react-native-paper';

const fonts = configureFonts({ config: { fontFamily: 'Roboto_400Regular' } });

/** Standard React Native Paper MD3 palette and geometry. */
export const BeeloyLightTheme: MD3Theme = {
  ...MD3LightTheme,
  fonts,
};

export const BeeloyDarkTheme: MD3Theme = {
  ...MD3DarkTheme,
  fonts,
};

/** Compatibility export. Providers select the system theme at runtime. */
export const BeeloyTheme = BeeloyLightTheme;
