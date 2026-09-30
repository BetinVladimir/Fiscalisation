import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { Platform } from 'react-native';
import * as SecureStore from 'expo-secure-store';

export const languageOptions = [
  ['bg', 'Български'], ['hr', 'Hrvatski'], ['cs', 'Čeština'], ['da', 'Dansk'],
  ['nl', 'Nederlands'], ['en', 'English'], ['et', 'Eesti'], ['fi', 'Suomi'],
  ['fr', 'Français'], ['de', 'Deutsch'], ['el', 'Ελληνικά'], ['hu', 'Magyar'],
  ['ga', 'Gaeilge'], ['it', 'Italiano'], ['lv', 'Latviešu'], ['lt', 'Lietuvių'],
  ['mt', 'Malti'], ['pl', 'Polski'], ['pt', 'Português'], ['ro', 'Română'],
  ['sk', 'Slovenčina'], ['sl', 'Slovenščina'], ['es', 'Español'], ['sv', 'Svenska'],
  ['ru', 'Русский'],
] as const;

export type Language = (typeof languageOptions)[number][0];
export type ThemeMode = 'system' | 'light' | 'dark';
type Preferences = { language: Language; themeMode: ThemeMode };
type PreferencesContextValue = Preferences & {
  hydrated: boolean;
  setLanguage: (value: Language) => void;
  setThemeMode: (value: ThemeMode) => void;
};

const storageKey = 'beefiscal.ui.preferences.v1';
const defaults: Preferences = { language: 'bg', themeMode: 'system' };
const PreferencesContext = createContext<PreferencesContextValue | null>(null);
const isLanguage = (value: unknown): value is Language => languageOptions.some(([code]) => code === value);
const isThemeMode = (value: unknown): value is ThemeMode => value === 'system' || value === 'light' || value === 'dark';

async function readPreferences(): Promise<Preferences> {
  try {
    const raw = Platform.OS === 'web' ? globalThis.localStorage?.getItem(storageKey) : await SecureStore.getItemAsync(storageKey);
    if (!raw) return defaults;
    const value = JSON.parse(raw) as Partial<Preferences>;
    return {
      language: isLanguage(value.language) ? value.language : defaults.language,
      themeMode: isThemeMode(value.themeMode) ? value.themeMode : defaults.themeMode,
    };
  } catch {
    return defaults;
  }
}

async function writePreferences(value: Preferences) {
  try {
    const raw = JSON.stringify(value);
    if (Platform.OS === 'web') globalThis.localStorage?.setItem(storageKey, raw);
    else await SecureStore.setItemAsync(storageKey, raw);
  } catch {
    // Storage failure must not make theme or language controls unusable.
  }
}

export function UiPreferencesProvider({ children }: React.PropsWithChildren) {
  const [preferences, setPreferences] = useState<Preferences>(defaults);
  const [hydrated, setHydrated] = useState(false);
  useEffect(() => { void readPreferences().then(value => { setPreferences(value); setHydrated(true); }); }, []);
  useEffect(() => { if (hydrated) void writePreferences(preferences); }, [hydrated, preferences]);
  const value = useMemo<PreferencesContextValue>(() => ({
    ...preferences,
    hydrated,
    setLanguage: language => setPreferences(current => ({ ...current, language })),
    setThemeMode: themeMode => setPreferences(current => ({ ...current, themeMode })),
  }), [preferences, hydrated]);
  return <PreferencesContext.Provider value={value}>{children}</PreferencesContext.Provider>;
}

export function useUiPreferences(): PreferencesContextValue {
  const value = useContext(PreferencesContext);
  if (!value) throw new Error('UiPreferencesProvider is missing');
  return value;
}
