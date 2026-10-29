import React,{useContext,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {Platform,ScrollView as NativeScrollView,View,type GestureResponderEvent} from 'react-native';
import {SafeAreaInsetsContext} from 'react-native-safe-area-context';
import {IconButton as PaperIconButton,List as PaperList,Menu as PaperMenu,useTheme} from 'react-native-paper';
import {decorativeHeaderIcon} from './header-icons';
import {useHeaderMetrics} from './PaperOperationalHeader';
import type {ThemePreference} from './provider';
import {useTranslation,type Translation,type Language} from '../languageStore';
import {supportedLanguages} from '../translations.generated';
const menuBackgroundLocks = new WeakMap<HTMLElement, { count: number; originallyInert: boolean }>();
// createPortal is only available on web (react-dom); null on native so it is never called there.
const createPortal: ((children: React.ReactNode, container: Element) => React.ReactPortal) | null =
  typeof document !== 'undefined' ? (require('react-dom') as typeof import('react-dom')).createPortal : null;

function usePaperMenuKeyboard(visible: boolean, prefix: "language" | "theme", selected: string) {
  useEffect(() => {
    if (Platform.OS !== "web") return;
    const trigger = document.querySelector<HTMLElement>(`[data-testid="${prefix}-menu-open"]`);
    trigger?.setAttribute("aria-expanded", String(visible));
    trigger?.setAttribute("aria-haspopup", "menu");
    if (!visible) return;
    // Menu portal is outside app-root. Match blocked pointer interaction with
    // blocked background keyboard and assistive-tech access.
    const background = document.querySelector<HTMLElement>('[data-testid="app-root"]');
    if (background) {
      const lock = menuBackgroundLocks.get(background) || { count: 0, originallyInert: background.hasAttribute("inert") };
      lock.count += 1;
      menuBackgroundLocks.set(background, lock);
      background.setAttribute("inert", "");
    }
    const configure = () => {
      const items=Array.from(document.querySelectorAll<HTMLElement>(`[data-testid^="${prefix}-"][role^="menuitem"]`));
      const active=items.find(item=>item===document.activeElement||item.contains(document.activeElement));
      items.forEach(item => {
        item.setAttribute("role", "menuitemradio");
        item.setAttribute("aria-checked", String(item.dataset.testid === `${prefix}-${selected}`));
        item.tabIndex = active ? item === active ? 0 : -1 : item.dataset.testid === `${prefix}-${selected}` ? 0 : -1;
      });
      // Focus the selected item once items appear in the portal
      if (!active && items.length > 0) {
        const chosen = items.find(item => item.tabIndex === 0) || items[0];
        chosen.focus();
        chosen.scrollIntoView?.({ block: 'nearest' });
      }
    };
    configure();
    const observer = new MutationObserver(configure);
    observer.observe(document.body, { childList: true, subtree: true });
    const onKeyDown = (event: KeyboardEvent) => {
      if (!["ArrowDown", "ArrowUp", "Home", "End", "Tab"].includes(event.key)) return;
      const items = Array.from(document.querySelectorAll<HTMLElement>(`[role^="menuitem"][data-testid^="${prefix}-"]`))
        .filter(item => item.getClientRects().length > 0);
      const current = items.findIndex(item => item === document.activeElement || item.contains(document.activeElement));
      if (!items.length) return;
      if (event.key === "Tab") {
        const close = items[0].closest('[data-testid$="-view"]')?.parentElement?.querySelector<HTMLElement>('button[aria-label]');
        if (!close || (current < 0 && document.activeElement !== close)) return;
        event.preventDefault();
        const destination = current >= 0 ? close : (items.find(item => item.tabIndex === 0) || items[0]);
        destination.focus();
        destination.scrollIntoView({ block: "nearest", inline: "nearest" });
        return;
      }
      if (current < 0) return;
      event.preventDefault();
      const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1
        : (current + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
      items.forEach((item, index) => { item.tabIndex = index === next ? 0 : -1; });
      items[next].focus();
      items[next].scrollIntoView({ block: "nearest", inline: "nearest" });
    };
    document.addEventListener("keydown", onKeyDown);
    return () => {
      observer.disconnect(); document.removeEventListener("keydown", onKeyDown);
      // Remove inert before focusing trigger — trigger is inside app-root which is inert
      if (background) {
        const lock = menuBackgroundLocks.get(background);
        if (lock && --lock.count === 0) {
          if (!lock.originallyInert) background.removeAttribute("inert");
          menuBackgroundLocks.delete(background);
        }
      }
      trigger?.focus();
    };
  }, [visible, prefix, selected]);
}

/** On web, Paper's Menu.show() hangs because measureMenuLayout() returns a
 * never-resolving Promise when menuRef is null at the time useEffect fires
 * (PortalManager setState is async in React 19). Bypass with createPortal. */
function WebPreferenceMenu({anchor, children, visible, onDismiss, overlayAccessibilityLabel, testID, contentStyle}: {
  anchor: React.ReactNode;
  children: React.ReactNode;
  visible: boolean;
  onDismiss?: (() => void) | undefined;
  overlayAccessibilityLabel?: string;
  testID: string;
  contentStyle?: React.ComponentProps<typeof PaperMenu>['contentStyle'];
}) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => { setMounted(visible); }, [visible]);

  useEffect(() => {
    if (!visible) return;
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') { e.preventDefault(); onDismiss?.(); } };
    document.addEventListener('keyup', h);
    return () => document.removeEventListener('keyup', h);
  }, [visible, onDismiss]);

  const cs = ((contentStyle ?? {}) as {width?: number; maxHeight?: number});
  const portal = mounted && createPortal ? createPortal(
    <div style={{position:'fixed',top:0,left:0,width:'100%',height:'100%',zIndex:1000}}>
      <button
        aria-label={overlayAccessibilityLabel}
        onClick={() => onDismiss?.()}
        style={{position:'absolute',top:0,left:0,width:'100%',height:'100%',background:'transparent',border:'none',cursor:'default',padding:0,margin:0}}
      />
      <div
        data-testid={`${testID}-view`}
        style={{position:'absolute',top:0,right:0,zIndex:1,overflow:'hidden',display:'flex',flexDirection:'column',...(cs.width != null ? {width:cs.width} : {}), ...(cs.maxHeight != null ? {maxHeight:cs.maxHeight} : {})}}
      >
        {children}
      </div>
    </div>,
    document.body
  ) : null;

  return <>{anchor}{portal}</>;
}

/** Native Paper menu mounts open with a stable coordinate anchor. This avoids
 * measuring an Appbar child and prevents an old hide callback racing a new open. */
function PreferenceMenu({anchor,children,visible,onDismiss,overlayAccessibilityLabel,testID='menu',contentStyle,...rest}:React.ComponentProps<typeof PaperMenu>){
 const [point,setPoint]=useState({x:0,y:0});
 if(Platform.OS==='web') return <WebPreferenceMenu anchor={anchor} visible={visible} onDismiss={onDismiss} overlayAccessibilityLabel={overlayAccessibilityLabel} testID={testID} contentStyle={contentStyle}>{children}</WebPreferenceMenu>;
 const element=anchor as React.ReactElement<{onPress?:(event:GestureResponderEvent)=>void}>;
 const trigger=React.cloneElement(element,{onPress:(event:GestureResponderEvent)=>{
  const x=event?.nativeEvent?.pageX,y=event?.nativeEvent?.pageY;
  if(Number.isFinite(x)&&Number.isFinite(y))setPoint({x:Math.max(0,x-24),y:y+24});
  element.props.onPress?.(event);
 }});
 return <>{trigger}{visible?<PaperMenu {...rest} visible={visible} onDismiss={onDismiss} overlayAccessibilityLabel={overlayAccessibilityLabel} testID={testID} contentStyle={contentStyle} anchor={point}>{children}</PaperMenu>:null}</>;
}

function PaperMenuChoice({ testID, title, onPress }: { testID: string; title: string; onPress: () => void }) {
  const theme = useTheme(),mounted=useRef(false),snapshot={},latest=useRef<object|null>(null);
  useLayoutEffect(()=>{latest.current=snapshot;return()=>{if(latest.current===snapshot)latest.current=null;};});
  useLayoutEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  return <PaperList.Item testID={testID} title={title} titleNumberOfLines={0} titleMaxFontSizeMultiplier={0}
    accessibilityRole="menuitem" accessibilityLabel={title}
    style={{ minHeight: 48, minWidth: 0, margin: 4, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: theme.roundness }}
    onPress={()=>{if(mounted.current&&latest.current===snapshot)onPress();}} />;
}

export function ThemeMenu({
  target=48,
  preference,
  text,
  onSelect,
}: {
  target?:number;
  preference: ThemePreference;
  text: Translation;
  onSelect: (value: ThemePreference) => void;
}) {
  const [visible, setVisible] = useState(false);
  const consumed=useRef<object|null>(null),snapshot={onSelect},committed=useRef<object|null>(null);
  useLayoutEffect(()=>{committed.current=snapshot;return()=>{if(committed.current===snapshot)committed.current=null;};});
  const current=()=>committed.current===snapshot;
  usePaperMenuKeyboard(visible, "theme", preference);
  const {width,height}=useHeaderMetrics();
  const insets = useContext(SafeAreaInsetsContext) ?? { top: 0, bottom: 0, left: 0, right: 0 };
  const menuHeight = Math.max(0, Math.min(420, height - insets.top - insets.bottom - 64));
  const menuWidth = Math.max(0, Math.min(320, width - insets.left - insets.right - 32));
  const labels: Record<ThemePreference, string> = {
    system: text.systemTheme,
    light: text.lightTheme,
    dark: text.darkTheme,
  };
  const choose = (value: ThemePreference) => {
    if(!current()||!visible||consumed.current===snapshot)return;
    consumed.current=snapshot;
    setVisible(false);
    onSelect(value);
  };

  return (
    <PreferenceMenu
      statusBarHeight={0}
      contentStyle={{ maxHeight: menuHeight, width: menuWidth, overflow: "hidden" }}
      overlayAccessibilityLabel={text.back}
      visible={visible}
      onDismiss={() => {if(current())setVisible(false);}}
      anchor={
        <PaperIconButton testID="theme-menu-open" mode="outlined" icon={decorativeHeaderIcon(preference==='dark'?'weather-night':'weather-sunny')} size={20} style={{width:target,height:target,margin:0}} accessibilityLabel={`${text.theme}: ${labels[preference]}`} onPress={()=>{if(current())setVisible(true);}}/>
      }
    >
      <NativeScrollView accessibilityRole="menu" testID="theme-menu-scroll" style={{maxHeight: menuHeight}} nestedScrollEnabled showsVerticalScrollIndicator keyboardShouldPersistTaps="handled">
      <PaperMenuChoice
        testID="theme-system"
        title={`${text.systemTheme}${preference === "system" ? ` • ${text.selected}` : ""}`}
        onPress={() => choose("system")}
      />
      <PaperMenuChoice
        testID="theme-light"
        title={`${text.lightTheme}${preference === "light" ? ` • ${text.selected}` : ""}`}
        onPress={() => choose("light")}
      />
      <PaperMenuChoice
        testID="theme-dark"
        title={`${text.darkTheme}${preference === "dark" ? ` • ${text.selected}` : ""}`}
        onPress={() => choose("dark")}
      />
      </NativeScrollView>
    </PreferenceMenu>
  );
}

export function LanguageMenu({
  target=48,
  language,
  label,
  onSelect,
}: {
  target?:number;
  language: Language;
  label: string;
  onSelect: (value: Language) => void;
}) {
  const text = useTranslation();
  const [visible, setVisible] = useState(false);
  const consumed=useRef<object|null>(null),snapshot={onSelect},committed=useRef<object|null>(null);
  useLayoutEffect(()=>{committed.current=snapshot;return()=>{if(committed.current===snapshot)committed.current=null;};});
  const current=()=>committed.current===snapshot;
  usePaperMenuKeyboard(visible, "language", language);
  const {width,height}=useHeaderMetrics();
  const insets = useContext(SafeAreaInsetsContext) ?? { top: 0, bottom: 0, left: 0, right: 0 };
  const menuHeight = Math.max(0, Math.min(420, height - insets.top - insets.bottom - 64));
  const menuWidth = Math.max(0, Math.min(320, width - insets.left - insets.right - 32));
  const labels: Record<Language, string> = {
    bg: "Български", hr: "Hrvatski", cs: "Čeština", da: "Dansk", nl: "Nederlands",
    en: "English", et: "Eesti", fi: "Suomi", fr: "Français", de: "Deutsch",
    el: "Ελληνικά", hu: "Magyar", ga: "Gaeilge", it: "Italiano", lv: "Latviešu",
    lt: "Lietuvių", mt: "Malti", pl: "Polski", pt: "Português", ro: "Română",
    sk: "Slovenčina", sl: "Slovenščina", es: "Español", sv: "Svenska", ru: "Русский",
  };
  const choose = (value: Language) => {
    if(!current()||!visible||consumed.current===snapshot)return;
    consumed.current=snapshot;
    setVisible(false);
    onSelect(value);
  };

  return (
    <PreferenceMenu
      statusBarHeight={0}
      overlayAccessibilityLabel={text.back}
      visible={visible}
      onDismiss={() => {if(current())setVisible(false);}}
      contentStyle={{ maxHeight: menuHeight, width: menuWidth, overflow: "hidden" }}
      anchor={
        <PaperIconButton testID="language-menu-open" mode="outlined" icon={decorativeHeaderIcon('translate')} size={20} style={{width:target,height:target,margin:0}} accessibilityLabel={`${label}: ${labels[language]}`} onPress={()=>{if(current())setVisible(true);}}/>
      }
    >
      <NativeScrollView accessibilityRole="menu" testID="language-menu-scroll" style={{ maxHeight: menuHeight, ...(Platform.OS !== "web" ? { height: Math.max(0, menuHeight - 16) } : {}) }} nestedScrollEnabled showsVerticalScrollIndicator keyboardShouldPersistTaps="handled">
        {supportedLanguages.map((value) => (
          <PaperMenuChoice
            key={value}
            testID={`language-${value}`}
            title={`${labels[value]}${language === value ? ` • ${text.selected}` : ""}`}
            onPress={() => choose(value)}
          />
        ))}
      </NativeScrollView>
    </PreferenceMenu>
  );
}
