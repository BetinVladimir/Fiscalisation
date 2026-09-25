import { useMemo } from 'react';
import { useWindowDimensions } from 'react-native';

export const BREAKPOINTS = { xs: 320, sm: 360, md: 600, lg: 840 } as const;
export type ScreenSize = keyof typeof BREAKPOINTS;

export function getScreenSize(width: number): ScreenSize {
  if (width >= BREAKPOINTS.lg) return 'lg';
  if (width >= BREAKPOINTS.md) return 'md';
  if (width >= BREAKPOINTS.sm) return 'sm';
  return 'xs';
}

export function useResponsiveLayout() {
  const readWindowDimensions = typeof useWindowDimensions === 'function'
    ? useWindowDimensions
    : () => ({ width: 390, height: 844, fontScale: 1, scale: 1 });
  const { width, height, fontScale } = readWindowDimensions();
  return useMemo(() => {
    const screenSize = getScreenSize(width);
    const compact = screenSize === 'xs' || screenSize === 'sm';
    return {
      width,
      height,
      fontScale,
      screenSize,
      compact,
      medium: screenSize === 'md',
      expanded: screenSize === 'lg',
      columns: compact ? 4 : screenSize === 'md' ? 8 : 12,
      gutter: compact ? 16 : screenSize === 'md' ? 24 : 32,
      gap: screenSize === 'xs' ? 8 : screenSize === 'lg' ? 24 : 16,
    };
  }, [width, height, fontScale]);
}
