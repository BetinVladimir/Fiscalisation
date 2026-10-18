import {headerIcons} from './header-icons';
import {useHeaderMetrics} from './PaperOperationalHeader';
import {IconButton} from 'react-native-paper';
import { SafeAreaInsetsContext } from 'react-native-safe-area-context';
import { useContext, useEffect, useLayoutEffect, useRef, useState } from "react";
import { List, Surface, useTheme } from "react-native-paper";
import { Platform, ScrollView, View, useWindowDimensions } from "react-native";
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

export function PreferenceControls({ language, onLanguageChange, testIdPrefix, target=48 }: {
  target?:number;
  language: Language;
  onLanguageChange: (value: Language) => void;
  testIdPrefix: string;
}) {
  const theme = useTheme();
  const {width,height}=useHeaderMetrics();
  const insets = useContext(SafeAreaInsetsContext) ?? {top:0,bottom:0,left:0,right:0};
  const menuHeight = Math.max(0,Math.min(224,height-insets.top-insets.bottom-64));
  const menuWidth = Math.max(0,Math.min(320,width-insets.left-insets.right-32));
  const [languageOpen, setLanguageOpen] = useState(false);
  const [themeOpen, setThemeOpen] = useState(false);
  const { preference, setPreference } = useThemePreference();
  const t = translations[language];
  const mounted = useRef(false), snapshot = { language, preference, languageOpen, themeOpen, onLanguageChange, setPreference }, consumed = useRef<object | null>(null), committed = useRef<object | null>(null);
  useLayoutEffect(() => { mounted.current = true; committed.current = snapshot; return () => { mounted.current = false; if(committed.current===snapshot)committed.current=null; }; });
  const current = () => mounted.current && committed.current === snapshot;
  const openLanguage = () => { if (!current()) return; setThemeOpen(false); setLanguageOpen(true); };
  const openTheme = () => { if (!current()) return; setLanguageOpen(false); setThemeOpen(true); };
  const dismissLanguage = () => { if (current()) setLanguageOpen(false); };
  const dismissTheme = () => { if (current()) setThemeOpen(false); };
  const chooseTheme = (value: 'system' | 'light' | 'dark') => {
    if (!current() || !themeOpen || consumed.current === snapshot || !['system','light','dark'].includes(value)) return;
    consumed.current = snapshot; setPreference(value); setThemeOpen(false);
  };
  useEffect(()=>{
    const kind=languageOpen?'language':themeOpen?'theme':null;
    if(!kind||typeof document==='undefined')return;
    const prefix=`${testIdPrefix}-${kind}-`,selected=kind==='language'?language:preference;
    let focused=false;
    const choices=kind==='language'?supportedLanguages:['system','light','dark'];
    const items=()=>Array.from(document.querySelectorAll<HTMLElement>('[data-testid]')).filter(node=>choices.some(value=>node.dataset.testid===prefix+value));
    const configure=()=>{
      const nodes=items(),active=nodes.find(node=>node===document.activeElement||node.contains(document.activeElement));for(const node of nodes){node.setAttribute('role','menuitemradio');node.setAttribute('aria-checked',String(node.dataset.testid===prefix+selected));node.tabIndex=active?node===active?0:-1:node.dataset.testid===prefix+selected?0:-1;}
      if(!focused&&nodes.length){focused=true;(nodes.find(node=>node.tabIndex===0)??nodes[0]).focus();}
    };
    configure();const observer=typeof MutationObserver==='function'?new MutationObserver(configure):null;observer?.observe(document.body,{childList:true,subtree:true});
    const keydown=(event:KeyboardEvent)=>{const nodes=items(),index=nodes.indexOf(document.activeElement as HTMLElement);if(index<0)return;let next=index;if(event.key==='Home')next=0;else if(event.key==='End')next=nodes.length-1;else if(event.key==='ArrowDown')next=(index+1)%nodes.length;else if(event.key==='ArrowUp')next=(index-1+nodes.length)%nodes.length;else return;event.preventDefault();nodes.forEach((node,i)=>node.tabIndex=i===next?0:-1);nodes[next].focus();nodes[next].scrollIntoView?.({block:'nearest'});};
    document.addEventListener('keydown',keydown);return()=>{observer?.disconnect();document.removeEventListener('keydown',keydown);};
  },[languageOpen,themeOpen,language,preference,testIdPrefix]);
  useEffect(() => {
    if (!languageOpen && !themeOpen) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        const kind = languageOpen ? 'language' : 'theme';
        setLanguageOpen(false); setThemeOpen(false);
        requestAnimationFrame(() => document.querySelector<HTMLElement>(`[data-testid="${testIdPrefix}-${kind}-open"]`)?.focus());
      }
    };
    document.addEventListener('keydown', closeOnEscape);
    return () => document.removeEventListener('keydown', closeOnEscape);
  }, [languageOpen, themeOpen, testIdPrefix]);
  const chooseLanguage = (value: Language) => {
    if (!current() || !languageOpen || consumed.current === snapshot || !supportedLanguages.includes(value)) return;
    consumed.current = snapshot;
    onLanguageChange(value);
    setLanguageOpen(false);
    try { localStorage.setItem("minipos-language", value); } catch { /* private browsing */ }
  };
  return <View style={{ minWidth: 0, flexDirection: "row", flexWrap: "nowrap", gap: 2 }}>
    <View testID={`${testIdPrefix}-language-anchor`} style={{position:'relative',zIndex:languageOpen?2:0,minWidth:0,maxWidth:'100%',flexShrink:1}}>
      <IconButton mode="outlined" icon={headerIcons.translate} size={20} style={{width:target,height:target,margin:0}} accessibilityLabel={`${t.language}: ${languageLabels[language]}`} onPress={languageOpen?dismissLanguage:openLanguage} testID={`${testIdPrefix}-language-open`} accessibilityState={{expanded:languageOpen}}/>

      {languageOpen && <Surface elevation={3} style={{position:'absolute',top:'100%',right:0,zIndex:3,width:menuWidth,maxHeight:menuHeight,backgroundColor:theme.colors.elevation.level3}}>
      <ScrollView testID={`${testIdPrefix}-language-scroll`} style={{ maxHeight: menuHeight }} nestedScrollEnabled showsVerticalScrollIndicator keyboardShouldPersistTaps="handled">
        <View accessibilityRole="menu">
          {supportedLanguages.map(value => <List.Item titleNumberOfLines={0} accessibilityRole={'menuitemradio' as never} style={{minHeight:48,borderWidth:1,borderColor:theme.colors.outline,borderRadius:theme.roundness,margin:4}} key={value} accessibilityState={{checked:language===value}} testID={`${testIdPrefix}-language-${value}`} title={`${languageLabels[value]}${language === value ? " ✓" : ""}`} onPress={() => chooseLanguage(value)} />)}
        </View>
      </ScrollView>
      </Surface>}
    </View>    <View testID={`${testIdPrefix}-theme-anchor`} style={{position:'relative',zIndex:themeOpen?2:0,minWidth:0,maxWidth:'100%',flexShrink:1}}>
      <IconButton mode="outlined" icon={headerIcons[preference==='dark'?'weather-night':'weather-sunny']} size={20} style={{width:target,height:target,margin:0}} accessibilityLabel={`${t.theme}: ${preference === "system" ? t.systemTheme : preference === "dark" ? t.darkTheme : t.lightTheme}`} onPress={themeOpen?dismissTheme:openTheme} testID={`${testIdPrefix}-theme-open`} accessibilityState={{expanded:themeOpen}}/>

      {themeOpen && <Surface elevation={3} style={{position:'absolute',top:'100%',right:0,zIndex:3,width:menuWidth,maxHeight:menuHeight,backgroundColor:theme.colors.elevation.level3}}>
      <ScrollView accessibilityRole="menu" testID={`${testIdPrefix}-theme-scroll`} style={{maxHeight:menuHeight}} nestedScrollEnabled keyboardShouldPersistTaps="handled">
        {(["system", "light", "dark"] as const).map(value => <List.Item titleNumberOfLines={0} accessibilityRole={'menuitemradio' as never} style={{minHeight:48,borderWidth:1,borderColor:theme.colors.outline,borderRadius:theme.roundness,margin:4}} key={value} accessibilityState={{checked:preference===value}} testID={`${testIdPrefix}-theme-${value}`} title={`${value === "system" ? t.systemTheme : value === "dark" ? t.darkTheme : t.lightTheme}${preference === value ? " ✓" : ""}`} onPress={() => chooseTheme(value)}/>) }
      </ScrollView>
      </Surface>}
    </View>

  </View>;
}
