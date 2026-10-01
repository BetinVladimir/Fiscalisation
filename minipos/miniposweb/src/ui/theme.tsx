import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { darkColors, lightColors } from "@beeloy/design-tokens";
import { MD3DarkTheme, MD3LightTheme, PaperProvider, type MD3Theme } from "react-native-paper";
import { SafeAreaProvider } from "react-native-safe-area-context";

type ThemePreference = "system" | "light" | "dark";
const preferenceKey = "minipos-web-theme";
const lightTheme: MD3Theme = { ...MD3LightTheme, colors: { ...MD3LightTheme.colors, ...lightColors } };
const darkTheme: MD3Theme = { ...MD3DarkTheme, colors: { ...MD3DarkTheme.colors, ...darkColors } };
const ThemePreferenceContext = createContext<{ preference: ThemePreference; setPreference: (value: ThemePreference) => void } | null>(null);

function readPreference(): ThemePreference {
  try {
    const value = localStorage.getItem(preferenceKey);
    return value === "light" || value === "dark" ? value : "system";
  } catch {
    return "system";
  }
}

export function BeeloyPaperProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState<ThemePreference>(readPreference);
  const [systemDark, setSystemDark] = useState(() => window.matchMedia("(prefers-color-scheme: dark)").matches);
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemDark(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    if (preference === "system") document.documentElement.removeAttribute("data-theme");
    else document.documentElement.dataset.theme = preference;
    document.documentElement.style.colorScheme = preference === "system" ? "light dark" : preference;
    try { localStorage.setItem(preferenceKey, preference); } catch { /* private browsing */ }
  }, [preference]);
  const context = useMemo(() => ({ preference, setPreference }), [preference]);
  const theme = preference === "dark" || (preference === "system" && systemDark) ? darkTheme : lightTheme;
  return <ThemePreferenceContext.Provider value={context}>
    <SafeAreaProvider><PaperProvider theme={theme}>{children}</PaperProvider></SafeAreaProvider>
  </ThemePreferenceContext.Provider>;
}

export function useThemePreference() {
  const value = useContext(ThemePreferenceContext);
  if (!value) throw new Error("useThemePreference requires BeeloyPaperProvider");
  return value;
}
