import { createContext, useContext, useEffect, useMemo, useState, useRef, type ReactNode } from "react";
import {createBeeloyPaperThemes,withSystemFonts} from "../../../../../BeeloyBackend/mobile/shared-ui/src/theme/paper";
import {foundationCopy} from "../../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated";
import {View} from "react-native";
import {loadRobotoFonts} from "./fonts";
import { ActivityIndicator, Button, HelperText, PaperProvider } from "react-native-paper";
import { SafeAreaProvider } from "react-native-safe-area-context";

type ThemePreference = "system" | "light" | "dark";
const preferenceKey = "minipos-web-theme";
const {light:lightTheme,dark:darkTheme}=createBeeloyPaperThemes({regular:"Roboto",medium:"Roboto",bold:"Roboto"});
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
  const [fontStatus,setFontStatus]=useState<'loading'|'ready'|'error'>('loading');
  const [fontAttempt,setFontAttempt]=useState(0);
  const fontRetryLock=useRef(false);
  useEffect(()=>{
    let active=true;setFontStatus('loading');fontRetryLock.current=true;
    void loadRobotoFonts().then(()=>{if(active)setFontStatus('ready');},()=>{if(active)setFontStatus('error');}).finally(()=>{if(active)fontRetryLock.current=false;});
    return()=>{active=false;};
  },[fontAttempt]);
  let language:keyof typeof foundationCopy='bg';
  try{const stored=localStorage.getItem('minipos-language');if(stored&&Object.hasOwn(foundationCopy,stored))language=stored as keyof typeof foundationCopy;}catch{}
  const copy=foundationCopy[language];

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
    <SafeAreaProvider><PaperProvider theme={fontStatus==='ready'?theme:withSystemFonts(theme)}>{fontStatus==='ready'?children:<View testID="font-startup" style={{flex:1,padding:16,gap:12,backgroundColor:theme.colors.background}}>{fontStatus==='loading'?<ActivityIndicator accessibilityLabel={copy.loading}/>:<><HelperText type="error" visible accessibilityLiveRegion="polite">{copy.error}</HelperText><Button testID="font-retry" mode="outlined" onPress={()=>{if(fontRetryLock.current)return;fontRetryLock.current=true;setFontStatus('loading');setFontAttempt(value=>value+1);}}>{copy.retry}</Button></>}</View>}</PaperProvider></SafeAreaProvider>
  </ThemePreferenceContext.Provider>;
}

export function useThemePreference() {
  const value = useContext(ThemePreferenceContext);
  if (!value) throw new Error("useThemePreference requires BeeloyPaperProvider");
  return value;
}
