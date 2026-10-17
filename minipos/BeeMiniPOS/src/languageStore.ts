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
export function isSupportedLanguage(value: unknown): value is Language {
  return typeof value === 'string' && supportedLanguages.some(language => language === value);
}


export type Translation = (typeof copy)[Language];

type LanguageState = {
  language: Language;
  setLanguage: (language: Language) => void;
};

export const useLanguageStore = create<LanguageState>()(
  persist(
    (set) => ({ language: "bg", setLanguage: (language) => { if (isSupportedLanguage(language)) set({ language }); } }),
    { name: "beeminipos-language-store", storage: createJSONStorage(() => AsyncStorage),
      partialize: state => ({ language: state.language }),
      merge: (persisted, current) => {
        const saved = persisted && typeof persisted === 'object' && !Array.isArray(persisted)
          && Object.prototype.hasOwnProperty.call(persisted, 'language')
          ? (persisted as {language?:unknown}).language : undefined;
        return { ...current, language: isSupportedLanguage(saved) ? saved : current.language };
      },
    },
  ),
);

export const useTranslation = () => {
  const language = useLanguageStore((state) => state.language);
  return copy[language];
};
