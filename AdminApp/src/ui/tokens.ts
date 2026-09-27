import { MD3DarkTheme, MD3LightTheme } from 'react-native-paper';

/** Calm, desaturated purple Paper MD3 palette with WCAG AA text pairs. */
export const lightColors = {
  ...MD3LightTheme.colors,
  primary: '#49317A', onPrimary: '#FFFFFF', primaryContainer: '#E8DDF8', onPrimaryContainer: '#24123F',
  secondary: '#4D435F', onSecondary: '#FFFFFF', secondaryContainer: '#E9E1F0', onSecondaryContainer: '#251E2D',
  tertiary: '#5A3F55', onTertiary: '#FFFFFF', tertiaryContainer: '#F1DDEA', onTertiaryContainer: '#2C1528',
  error: '#9F2D2D', onError: '#FFFFFF', errorContainer: '#F9DEDC', onErrorContainer: '#410E0B',
  background: '#F7F5F9', onBackground: '#1D1A20', surface: '#FFFBFF', onSurface: '#1D1A20',
  surfaceVariant: '#E8E2EC', onSurfaceVariant: '#48434D', surfaceDisabled: 'rgba(29,26,32,0.12)', onSurfaceDisabled: 'rgba(29,26,32,0.62)',
  outline: '#6F6874', outlineVariant: '#C9C2CD', inverseSurface: '#322F35', inverseOnSurface: '#F5EFF7', inversePrimary: '#D2C1F0',
  success: '#2F6247', onSuccess: '#FFFFFF', successContainer: '#CFE9D9', onSuccessContainer: '#102C1E',
  warning: '#765720', onWarning: '#FFFFFF', warningContainer: '#F7E0B2', onWarningContainer: '#2D2005',
  info: '#3B4F78', onInfo: '#FFFFFF', infoContainer: '#DCE5F8', onInfoContainer: '#14213C',
} as const;

export const darkColors = {
  ...MD3DarkTheme.colors,
  primary: '#D2C1F0', onPrimary: '#24153F', primaryContainer: '#3B2858', onPrimaryContainer: '#EEE4FF',
  secondary: '#CEC2D8', onSecondary: '#302936', secondaryContainer: '#443A4C', onSecondaryContainer: '#EBDFF3',
  tertiary: '#E5C2D8', onTertiary: '#3D2434', tertiaryContainer: '#58394C', onTertiaryContainer: '#FBDDEC',
  error: '#FFB4AB', onError: '#690005', errorContainer: '#8C1D18', onErrorContainer: '#FFDAD6',
  background: '#0D0A12', onBackground: '#EAE4EE', surface: '#15111B', onSurface: '#EAE4EE',
  surfaceVariant: '#292330', onSurfaceVariant: '#D4CDD9', surfaceDisabled: 'rgba(234,228,238,0.12)', onSurfaceDisabled: 'rgba(234,228,238,0.62)',
  outline: '#9B929F', outlineVariant: '#4A434F', inverseSurface: '#EAE4EE', inverseOnSurface: '#322F35', inversePrimary: '#49317A',
  success: '#A8D5B8', onSuccess: '#173724', successContainer: '#28513B', onSuccessContainer: '#C7F1D5',
  warning: '#E7C579', onWarning: '#3E2E08', warningContainer: '#5B4314', onWarningContainer: '#FFE4A6',
  info: '#B8C7EA', onInfo: '#203354', infoContainer: '#34486B', onInfoContainer: '#DCE5FF',
} as const;

export const colors = lightColors;


export const spacing = { xs: 4, sm: 8, md: 16, lg: 24, xl: 32, xxl: 48 } as const;
export const radius = { sm: 6, md: 10, lg: 14, xl: 20, pill: 999 } as const;
export const touchTarget = { min: 44, comfortable: 48 } as const;
export const typography = { body: 16, label: 14, title: 20, headline: 28 } as const;
export const breakpoints = { xs: 0, sm: 480, md: 768, lg: 1024 } as const;
export const layout = { contentMaxWidth: 1200, formMaxWidth: 720, gutterXs: 12, gutterSm: 16, gutterMd: 24, gutterLg: 32 } as const;
