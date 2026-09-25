import { supportedLanguages, translations } from "./translations.generated";
import AsyncStorage from "@react-native-async-storage/async-storage";

// Metro Web currently emits Zustand's ESM build into a classic script and
// leaves import.meta intact. Requiring the package selects its CJS/React Native
// entrypoint and keeps the generated Expo bundle executable in browsers.
declare const require: (id: string) => unknown;
const { create } = require("zustand") as typeof import("zustand");
const { createJSONStorage, persist } = require("zustand/middleware") as typeof import("zustand/middleware");

export type Language = typeof supportedLanguages[number];

const copy = translations;

export type Translation = (typeof copy)[Language];

type LanguageState = {
  language: Language;
  setLanguage: (language: Language) => void;
};

export const useLanguageStore = create<LanguageState>()(
  persist(
    (set) => ({ language: "bg", setLanguage: (language) => set({ language }) }),
    { name: "beeminipos-language-store", storage: createJSONStorage(() => AsyncStorage) },
  ),
);

export const useTranslation = () => {
  const language = useLanguageStore((state) => state.language);
  return copy[language];
};
