import { useState } from "react";
import { Button, Menu } from "react-native-paper";
import { ScrollView, View } from "react-native";
import { supportedLanguages, translations } from "./authTranslations.generated";
import { useThemePreference } from "./theme";

export type Language = typeof supportedLanguages[number];
const languageLabels: Record<Language, string> = {
  bg: "Български", hr: "Hrvatski", cs: "Čeština", da: "Dansk", nl: "Nederlands",
  en: "English", et: "Eesti", fi: "Suomi", fr: "Français", de: "Deutsch",
  el: "Ελληνικά", hu: "Magyar", ga: "Gaeilge", it: "Italiano", lv: "Latviešu",
  lt: "Lietuvių", mt: "Malti", pl: "Polski", pt: "Português", ro: "Română",
  sk: "Slovenčina", sl: "Slovenščina", es: "Español", sv: "Svenska", ru: "Русский",
};

export function readLanguage(): Language {
  try { return supportedLanguages.find(value => value === localStorage.getItem("minipos-language")) || "bg"; }
  catch { return "bg"; }
}

export function PreferenceControls({ language, onLanguageChange, testIdPrefix }: {
  language: Language;
  onLanguageChange: (value: Language) => void;
  testIdPrefix: string;
}) {
  const [languageOpen, setLanguageOpen] = useState(false);
  const [themeOpen, setThemeOpen] = useState(false);
  const { preference, setPreference } = useThemePreference();
  const t = translations[language];
  const chooseLanguage = (value: Language) => {
    onLanguageChange(value);
    setLanguageOpen(false);
    try { localStorage.setItem("minipos-language", value); } catch { /* private browsing */ }
  };
  return <View style={{ width: "100%", minWidth: 0, flexDirection: "row", flexWrap: "wrap", justifyContent: "flex-end", gap: 8 }}>
    <Menu visible={themeOpen} onDismiss={() => setThemeOpen(false)} theme={{ animation: { scale: 0 } }} contentStyle={{ maxHeight: 224 }} anchor={<Button mode="outlined" onPress={() => setThemeOpen(true)} testID={`${testIdPrefix}-theme-open`}>{t.theme}: {preference === "system" ? t.systemTheme : preference === "dark" ? t.darkTheme : t.lightTheme}</Button>}>
      <View accessibilityRole="menu">
        {(["system", "light", "dark"] as const).map(value => <Menu.Item key={value} testID={`${testIdPrefix}-theme-${value}`} title={`${value === "system" ? t.systemTheme : value === "dark" ? t.darkTheme : t.lightTheme}${preference === value ? " ✓" : ""}`} onPress={() => { setPreference(value); setThemeOpen(false); }}/>) }
      </View>
    </Menu>
    <Menu visible={languageOpen} onDismiss={() => setLanguageOpen(false)} theme={{ animation: { scale: 0 } }} contentStyle={{ maxHeight: 224, width: 240 }} anchor={<Button mode="outlined" onPress={() => setLanguageOpen(true)} testID={`${testIdPrefix}-language-open`}>{t.language}: {languageLabels[language]}</Button>}>
      <ScrollView style={{ maxHeight: 224 }} nestedScrollEnabled>
        <View accessibilityRole="menu">
          {supportedLanguages.map(value => <Menu.Item key={value} testID={`${testIdPrefix}-language-${value}`} title={`${languageLabels[value]}${language === value ? " ✓" : ""}`} onPress={() => chooseLanguage(value)} />)}
        </View>
      </ScrollView>
    </Menu>
  </View>;
}
