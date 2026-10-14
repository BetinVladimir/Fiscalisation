import { AppButton as Button } from './app-button';
import { createContext, useContext, useEffect, useMemo, useState, useRef, type ReactNode } from "react";
import {createBeeloyPaperThemes,withSystemFonts} from "../../../../../BeeloyBackend/mobile/shared-ui/src/theme/paper";
import {foundationCopy} from "../../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated";
import {loadRobotoFonts} from "./fonts";
import { ActivityIndicator, HelperText, PaperProvider, Surface } from "react-native-paper";
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
    void Promise.resolve().then(()=>loadRobotoFonts()).then(()=>{if(active)setFontStatus('ready');},()=>{if(active)setFontStatus('error');}).finally(()=>{if(active)fontRetryLock.current=false;});
    return()=>{active=false;};
  },[fontAttempt]);
  let language:keyof typeof foundationCopy='bg';
  try{const stored=localStorage.getItem('minipos-language');if(stored&&Object.hasOwn(foundationCopy,stored))language=stored as keyof typeof foundationCopy;}catch{}
  const copy=foundationCopy[language];

  const reducedMotion=useSystemMedia('(prefers-reduced-motion: reduce)');
  const systemDark=useSystemMedia('(prefers-color-scheme: dark)');
  useEffect(() => {
    if(typeof document==='undefined')return;
    if (preference === "system") document.documentElement.removeAttribute("data-theme");
    else document.documentElement.dataset.theme = preference;
    document.documentElement.style.colorScheme = preference === "system" ? "light dark" : preference;
    try { localStorage.setItem(preferenceKey, preference); } catch { /* private browsing */ }
  }, [preference]);
  const context = useMemo(() => ({ preference, setPreference }), [preference]);
  const theme = preference === "dark" || (preference === "system" && systemDark) ? darkTheme : lightTheme;
  const resolvedTheme = useMemo(()=>{const base=fontStatus==='ready'?theme:withSystemFonts(theme);return {...base,animation:{...base.animation,scale:reducedMotion?0:base.animation.scale}};},[theme,fontStatus,reducedMotion]);
  const settings=useMemo(()=>({rippleEffectEnabled:!reducedMotion}),[reducedMotion]);
  return <ThemePreferenceContext.Provider value={context}>
    <SafeAreaProvider><PaperProvider theme={resolvedTheme} settings={settings}>{fontStatus==='ready'?children:<Surface mode="flat" elevation={0} testID="font-startup" style={{flex:1,padding:16,gap:12,backgroundColor:theme.colors.background}}>{fontStatus==='loading'?<ActivityIndicator accessibilityLabel={copy.loading}/>:<><HelperText nativeID="minipos-font-error" accessibilityRole="alert" type="error" visible accessibilityLiveRegion="polite">{copy.error}</HelperText><Button {...{'aria-describedby':'minipos-font-error'}} testID="font-retry" mode="outlined" onPress={()=>{if(fontRetryLock.current)return;fontRetryLock.current=true;setFontStatus('loading');setFontAttempt(value=>value+1);}}>{copy.retry}</Button></>}</Surface>}</PaperProvider></SafeAreaProvider>
  </ThemePreferenceContext.Provider>;
}

export function useThemePreference() {
  const value = useContext(ThemePreferenceContext);
  if (!value) throw new Error("useThemePreference requires BeeloyPaperProvider");
  return value;
}

/** Missing browser APIs use light/no-motion-preference defaults until available. */
export function useSystemMedia(query:string){
 const read=()=>typeof window!=='undefined'&&typeof window.matchMedia==='function'?window.matchMedia(query).matches:false;
 const [matches,setMatches]=useState(read);
 useEffect(()=>{
  if(typeof window==='undefined'||typeof window.matchMedia!=='function')return;
  const media=window.matchMedia(query);let active=true;
  const update=()=>{if(active)setMatches(media.matches);};
  update();
  if(typeof media.addEventListener==='function'){
   media.addEventListener('change',update);
   return()=>{active=false;media.removeEventListener('change',update);};
  }
  // Older embedded browser engines expose the original MediaQueryList API.
  if(typeof media.addListener==='function'){
   media.addListener(update);
   return()=>{active=false;media.removeListener(update);};
  }
  return()=>{active=false;};
 },[query]);
 return matches;
}
