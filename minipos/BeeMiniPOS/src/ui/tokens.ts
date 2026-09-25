export const lightColors = {
  primary: '#17613A', onPrimary: '#FFFFFF', primaryContainer: '#D4E8D9', onPrimaryContainer: '#092113',
  secondary: '#4F6354', onSecondary: '#FFFFFF', secondaryContainer: '#D2E8D5', onSecondaryContainer: '#0D1F13',
  tertiary: '#3A656F', onTertiary: '#FFFFFF', tertiaryContainer: '#BEEAF5', onTertiaryContainer: '#001F26',
  success: '#146C2E', onSuccess: '#FFFFFF', successContainer: '#B7F5C4', onSuccessContainer: '#002108',
  warning: '#8C4A00', onWarning: '#FFFFFF', warningContainer: '#FFDCC2', onWarningContainer: '#2D1600',
  error: '#BA1A1A', onError: '#FFFFFF', errorContainer: '#FFDAD6', onErrorContainer: '#410002',
  background: '#F8FAF8', onBackground: '#191C1A', surface: '#F8FAF8', onSurface: '#191C1A',
  surfaceVariant: '#DEE5DF', onSurfaceVariant: '#424943', outline: '#727972', outlineVariant: '#C2C9C2',
  inverseSurface: '#2E312F', inverseOnSurface: '#EFF1EE', inversePrimary: '#A9D1B3',
  shadow: '#000000', scrim: '#000000', backdrop: 'rgba(25,28,26,0.40)',
  elevation: { level0: 'transparent', level1: '#F2F5F2', level2: '#EDF1ED', level3: '#E7EDE8', level4: '#E5EBE6', level5: '#E0E7E1' },
} as const;

export const darkColors = {
  primary: '#9ED5AF', onPrimary: '#003920', primaryContainer: '#0B4F2D', onPrimaryContainer: '#BCECC9',
  secondary: '#B7CCBB', onSecondary: '#23352A', secondaryContainer: '#394B40', onSecondaryContainer: '#D3E8D7',
  tertiary: '#A2CED9', onTertiary: '#00363F', tertiaryContainer: '#204D56', onTertiaryContainer: '#BDEAF5',
  success: '#8ADB9E', onSuccess: '#003914', successContainer: '#0B5224', onSuccessContainer: '#A5F7B7',
  warning: '#FFB870', onWarning: '#4A2800', warningContainer: '#683C00', onWarningContainer: '#FFDCC2',
  error: '#FFB4AB', onError: '#690005', errorContainer: '#93000A', onErrorContainer: '#FFDAD6',
  background: '#101512', onBackground: '#E0E4E0', surface: '#101512', onSurface: '#E0E4E0',
  surfaceVariant: '#414944', onSurfaceVariant: '#C1C9C2', outline: '#8B938C', outlineVariant: '#414944',
  inverseSurface: '#E0E4E0', inverseOnSurface: '#2D322E', inversePrimary: '#17613A',
  shadow: '#000000', scrim: '#000000', backdrop: 'rgba(0,0,0,0.55)',
  elevation: { level0: 'transparent', level1: '#171D19', level2: '#1B211D', level3: '#1F2621', level4: '#202923', level5: '#242D27' },
} as const;

/** Compatibility alias for static styles. New theme-aware code must use useTheme().colors. */
export const colors = lightColors;

export const spacing = { xs: 4, sm: 8, md: 16, lg: 24, xl: 32, xxl: 48 } as const;
export const radius = { sm: 6, md: 10, lg: 14, xl: 20, pill: 999 } as const;
export const touchTarget = { min: 44, comfortable: 48 } as const;
export const typography = { body: 16, label: 14, title: 20, headline: 28 } as const;
export const breakpoints = { xs: 0, sm: 480, md: 768, lg: 1024 } as const;
export const layout = { contentMaxWidth: 1200, formMaxWidth: 720, gutterXs: 12, gutterSm: 16, gutterMd: 24, gutterLg: 32 } as const;
