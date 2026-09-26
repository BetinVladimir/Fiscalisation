import { MD3DarkTheme, MD3LightTheme } from 'react-native-paper';

/**
 * Standard React Native Paper MD3 palettes.
 * Success and warning are compatibility aliases for semantic statuses that
 * are not part of the Paper MD3 color contract.
 */
export const lightColors = {
  ...MD3LightTheme.colors,
  success: MD3LightTheme.colors.tertiary,
  onSuccess: MD3LightTheme.colors.onTertiary,
  successContainer: MD3LightTheme.colors.tertiaryContainer,
  onSuccessContainer: MD3LightTheme.colors.onTertiaryContainer,
  warning: MD3LightTheme.colors.secondary,
  onWarning: MD3LightTheme.colors.onSecondary,
  warningContainer: MD3LightTheme.colors.secondaryContainer,
  onWarningContainer: MD3LightTheme.colors.onSecondaryContainer,
} as const;

export const darkColors = {
  ...MD3DarkTheme.colors,
  success: MD3DarkTheme.colors.tertiary,
  onSuccess: MD3DarkTheme.colors.onTertiary,
  successContainer: MD3DarkTheme.colors.tertiaryContainer,
  onSuccessContainer: MD3DarkTheme.colors.onTertiaryContainer,
  warning: MD3DarkTheme.colors.secondary,
  onWarning: MD3DarkTheme.colors.onSecondary,
  warningContainer: MD3DarkTheme.colors.secondaryContainer,
  onWarningContainer: MD3DarkTheme.colors.onSecondaryContainer,
} as const;

/** Compatibility alias for static styles. Theme-aware code uses useTheme().colors. */
export const colors = lightColors;

export const spacing = { xs: 4, sm: 8, md: 16, lg: 24, xl: 32, xxl: 48 } as const;
export const radius = { sm: 6, md: 10, lg: 14, xl: 20, pill: 999 } as const;
export const touchTarget = { min: 44, comfortable: 48 } as const;
export const typography = { body: 16, label: 14, title: 20, headline: 28 } as const;
export const breakpoints = { xs: 0, sm: 480, md: 768, lg: 1024 } as const;
export const layout = { contentMaxWidth: 1200, formMaxWidth: 720, gutterXs: 12, gutterSm: 16, gutterMd: 24, gutterLg: 32 } as const;
