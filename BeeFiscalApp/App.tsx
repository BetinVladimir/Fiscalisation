import {RequestAuthFence} from './src/requestAuthFence';
import {reportCopy} from './src/ui/report-copy';
import {requireFiscalAuthorization} from './src/fiscalAuthorization';
import {useUiPreferences} from './src/ui/preferences';
import {shellCopy,shellTabLabel} from './src/ui/shell-copy';
import {useOwnedDeviceValue} from './src/useOwnedDeviceValue';
import {parseApiJson} from './src/parseApiJson';
import * as ExpoCrypto from 'expo-crypto';
import {ProvisioningDialog} from './src/components/ProvisioningDialog';
import {connectProvisioningDevice} from './src/provisioningBle';
import {provisionDevice} from './src/provisioningProtocol';
import {Surface as SafePaperSurface} from 'react-native-paper';
import {Surface as PaintedPaperSurface} from 'react-native-paper';
import { compositeSelectionValid } from './src/compositeSelection';
import { auditFilterDatesValid } from './src/auditFilters';
import { AppButton } from './src/ui/app-button';
import { colors as beeloyColors } from './src/ui/tokens';
import { SafeAreaView } from 'react-native-safe-area-context';
import { ResponsiveScrollView as ScrollView } from './src/ui/responsive-scroll-view';
import { AuthScreen } from './src/screens/AuthScreen';
import { OperationsScreen } from './src/screens/OperationsScreen';
import { ReportsScreen } from './src/screens/ReportsScreen';
import { DevicesScreen } from './src/screens/DevicesScreen';
import { AdministrationScreen, type AdminRequest } from './src/screens/AdministrationScreen';
import { AuditScreen, type AuditFilters } from './src/screens/AuditScreen';
import { SettingsScreen } from './src/screens/SettingsScreen';
import { LanguageThemeControls } from './src/ui/LanguageThemeControls';
import { ReferencePicker } from './src/components/FiscalControls';
import { StatusBar } from "expo-status-bar";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Platform, StyleSheet, View } from "react-native";
import { useWindowDimensions } from 'react-native';
import { Badge, Drawer, List, Surface, Modal, Portal, Text as PaperText, useTheme } from 'react-native-paper';
import { MVP1_DEVICE_PROFILES } from "./src/deviceProfiles";
import { fetchTextWithTimeout } from "./src/http";
import { collectCursorPages } from "./src/pagination";
import {
  devicesBoundToRegister,
  isRegisterId,
  registerCollectionPath,
  registerResourcePath,
} from "./src/registerFilter";
import { useEmailOtpAuth } from "./src/emailOtpAuth";
import Constants from "expo-constants";
import { BeeloyAppProvider } from "./src/ui/provider";

const base = (
  process.env.EXPO_PUBLIC_FISCAL_API_URL || "http://localhost:8080/public/v1"
).replace(/\/$/, "");
const version = "2026-08-07";
const appEnv = (process.env.EXPO_PUBLIC_APP_ENV || "dev").toLowerCase();
const configuredRegisterId = process.env.EXPO_PUBLIC_REGISTER_ID || "";
const prodMode = appEnv === "prod";
const devAuthToken = prodMode
  ? ""
  : process.env.EXPO_PUBLIC_FISCAL_AUTH_TOKEN || "";
let runtimeAuthToken = devAuthToken;
const requestAuthFence=new RequestAuthFence();
type Item = Record<string, unknown>;
type AdminLists = {
  locations: Item[];
  registers: Item[];
  operators: Item[];
  devices: Item[];
};

async function call(path: string, init: RequestInit = {}, requestToken = runtimeAuthToken) {
  requireFiscalAuthorization(prodMode,requestToken);
  const requestOwner=requestAuthFence.capture(requestToken,init.headers);
  // Every mutation is independently idempotent. A PROD 401 clears local
  // authority immediately instead of leaving privileged controls usable.
  const mutation = !!init.method && init.method !== "GET";
  const {response: r, text} = await fetchTextWithTimeout(base + path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-Api-Version": version,
      ...(mutation
        ? {
            "Idempotency-Key": `${Date.now()}-${Math.random().toString(16).slice(2)}-admin`,
          }
        : {}),
      ...(requestToken
        ? { Authorization: `Bearer ${requestToken}` }
        : {}),
      ...(init.headers || {}),
    },
  });
  if (r.status === 401 && prodMode) requestOwner.unauthorized();
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  return text ? parseApiJson(text) : {};
}
const collect = (path: string, token: string, live: () => boolean) =>
  collectCursorPages<Item>(path, async next => {
    if (!live()) throw new Error('STALE_READ');
    const result = await call(next, {}, token);
    if (!live()) throw new Error('STALE_READ');
    return result;
  });
const label = (v: unknown) =>
  typeof v === "string" ? v : typeof v === "number" ? String(v) : "—";

export default function App() { return <BeeloyAppProvider><View style={{flex:1}}><AppContent/><DemoBadge/></View></BeeloyAppProvider>; }
function DemoBadge(){return Constants.expoConfig?.extra?.isDemo?<View pointerEvents="none" style={{position:'absolute',right:8,top:8,zIndex:9999}}><Badge size={28}>DEMO</Badge></View>:null}
function AppContent() {
  const { width } = useWindowDimensions();
  const compact = width < 600;
  const {language}=useUiPreferences();
  const shell=shellCopy(language);
  const tabLabel=(value:string)=>shellTabLabel(language,value);
  const theme = useTheme();
  const [navOpen, setNavOpen] = useState(false);
  const commandPending = useRef<object | null>(null);
  // Token roles are reduced to explicit UI capabilities. The backend remains
  // authoritative; these guards prevent presenting accidental admin paths.
  const oidc = useEmailOtpAuth();
  const roles = prodMode ? oidc.roles : ["ADMIN", "SUPERVISOR", "AUDITOR"];
  const hasRole = (...allowed: string[]) =>
    roles.some((role) => allowed.includes(role));
  const canAdminister = hasRole("ADMIN");
  const canRunReports = hasRole("SUPERVISOR", "ADMIN");
  const canReconcile = hasRole("SUPERVISOR", "ADMIN");
  const canReadAudit = hasRole("ADMIN", "AUDITOR");
  const canDiagnose = hasRole("ADMIN", "SERVICE");
  const canIssueBleSession = hasRole("CASHIER", "SUPERVISOR", "ADMIN");
  const tabs = [
    "Устройства",
    "Операции",
    ...(hasRole("SUPERVISOR", "ADMIN", "AUDITOR") ? ["Отчети"] : []),
    ...(canReadAudit ? ["Одит"] : []),
    ...(canAdminister ? ["Администриране"] : []),
    "Настройки",
  ];
  const [tab, setTab] = useState("Устройства"),
    [message, setMessage] = useState("Зареждане на данните от публичния API…"),
    [core, setCore] = useState("CHECKING"),
    [items, setItems] = useState<Item[]>([]),
    [selected, setSelected] = useState<Item | null>(null),
    [register, setRegister] = useState<string>(configuredRegisterId),
    [policy, setPolicy] = useState<Item | null>(null),
    [adminLists, setAdminLists] = useState<AdminLists>({
      locations: [],
      registers: [],
      operators: [],
      devices: [],
    }),
    [locationCode, setLocationCode] = useState("SOF"),
    [locationName, setLocationName] = useState("Sofia"),
    [locationAddress, setLocationAddress] = useState(""),
    [registerCode, setRegisterCode] = useState("R01"),
    [locationId, setLocationId] = useState(""),
    [operatorCode, setOperatorCode] = useState("A001"),
    [operatorId, setOperatorId] = useState(""),
    [operatorFirstName, setOperatorFirstName] = useState(""),
    [operatorLastName, setOperatorLastName] = useState(""),
    [deviceVendor, setDeviceVendor] = useState("Datecs"),
    [deviceModel, setDeviceModel] = useState("DP-150 MX"),
    [deviceSerial, setDeviceSerial] = useState(""),
    [deviceKind, setDeviceKind] = useState("FISCAL_DEVICE"),
    [deviceDriverId, setDeviceDriverId] = useState("datecs"),
    [deviceProtocolId, setDeviceProtocolId] = useState("datecs-fp"),
    [deviceProtocolVersion, setDeviceProtocolVersion] = useState("2.11.4"),
    [deviceTransport, setDeviceTransport] = useState("RS232"),
    [deviceTransportParameters, setDeviceTransportParameters] = useState(
      '{"baud":115200,"data_bits":8,"parity":"N","stop_bits":1}',
    ),
    [bindingRole, setBindingRole] = useState("FISCAL_DEVICE"),
    [deviceId, setDeviceId] = useState(""),
    [compositeProfile, setCompositeProfile] = useState(
      MVP1_DEVICE_PROFILES[0].id,
    ),
    [adapterDeviceId, setAdapterDeviceId] = useState(""),
    [fiscalEndpointId, setFiscalEndpointId] = useState(""),
    [paymentEndpointId, setPaymentEndpointId] = useState(""),
    [expectedRegisterVersion, setExpectedRegisterVersion] = useState("1"),
    [compositeBinding, setCompositeBinding] = useState<Item | null>(null),
    [activationCode, setActivationCode] = useState(""),
    [activationLocationId, setActivationLocationId] = useState(""),
    [blePublicKey, setBlePublicKey] = useState(""),
    [bleAppInstance] = useState(() => `beefiscal-admin-${Date.now()}`),
    [auditActor, setAuditActor] = useState(""),
    [auditAction, setAuditAction] = useState(""),
    [auditObjectType, setAuditObjectType] = useState(""),
    [auditObjectId, setAuditObjectId] = useState(""),
    [auditUnp, setAuditUnp] = useState(""),
    [auditFrom, setAuditFrom] = useState(""),
    [auditTo, setAuditTo] = useState(""),
    [adminBusy, setAdminBusy] = useState(false),
    [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [reportError, setReportError] = useState('');
  const mounted = useRef(false), readEpoch = useRef(0);
  const authKey = JSON.stringify([oidc.accessToken, roles]);
  const authOwner = useRef({ key: authKey });
  if (authOwner.current.key !== authKey) authOwner.current = { key: authKey };
  const renderSnapshot = {}, latestRender = useRef(renderSnapshot);
  useLayoutEffect(() => { latestRender.current = renderSnapshot; });
  const currentRender = () => mounted.current && latestRender.current === renderSnapshot;
  const contextKey = JSON.stringify([authKey, tab, register, selected?.id]);
  const [provisioningOpen,setProvisioningOpen]=useOwnedDeviceValue(contextKey,false);
  const [provisioningBusy,setProvisioningBusy]=useOwnedDeviceValue(contextKey,false);
  const provisioningVisible=provisioningOpen && tab==='Устройства' && Boolean(selected) && canAdminister;
  const shellRef=useRef<View>(null);
  useLayoutEffect(()=>{
    if(Platform.OS!=='web')return;
    const node=shellRef.current as unknown as HTMLElement|null;
    if(!node)return;
    const previous=node.inert;node.inert=provisioningVisible;
    return()=>{node.inert=previous};
  },[provisioningVisible]);
  const [diagnostics, setDiagnostics] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [deviceHealth, setDeviceHealth] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [deviceReadiness, setDeviceReadiness] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [deviceActivity, setDeviceActivity] = useOwnedDeviceValue<Item[]>(contextKey, []);
  const [provisioning, setProvisioning] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [bleSession, setBleSession] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [blueCashActivation, setBlueCashActivation] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [activationRequest, setActivationRequest] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [transportMetrics, setTransportMetrics] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const [fiscalMetrics, setFiscalMetrics] = useOwnedDeviceValue<Item | null>(contextKey, null);
  const contextRef = useRef({ key: contextKey });
  if (contextRef.current.key !== contextKey) { contextRef.current = { key: contextKey }; commandPending.current = null; }
  const contextOwner = contextRef.current;
  const beginDeviceCommand = () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== 'Устройства') return null;
    const ticket = {}, owner = contextOwner, auth = authOwner.current, epoch = readEpoch.current;
    const token = oidc.accessToken || devAuthToken;
    commandPending.current = ticket; setLoading(true);
    const live = () => mounted.current && contextRef.current === owner && authOwner.current === auth && readEpoch.current === epoch && commandPending.current === ticket;
    return {
      live,
      call: async (path: string, init: RequestInit = {}) => {
        if (!live()) throw new Error('STALE_COMMAND');
        const result = await call(path, init, token);
        if (!live()) throw new Error('STALE_COMMAND');
        return result;
      },
      finish: () => {
        if (commandPending.current !== ticket) return;
        commandPending.current = null;
        if (mounted.current && contextRef.current === owner && authOwner.current === auth && readEpoch.current === epoch) setLoading(false);
      },
    };
  };


  const beginAdminCommand = () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== 'Администриране' || !canAdminister) return null;
    const ticket = {}, owner = contextOwner, auth = authOwner.current;
    const token = oidc.accessToken || devAuthToken;
    commandPending.current = ticket; setAdminBusy(true);
    const live = () => mounted.current && contextRef.current === owner && authOwner.current === auth && commandPending.current === ticket;
    return {
      live,
      call: async (path: string, init: RequestInit = {}) => {
        if (!live()) throw new Error('STALE_COMMAND');
        const result = await call(path, init, token);
        if (!live()) throw new Error('STALE_COMMAND');
        return result;
      },
      finish: () => {
        if (commandPending.current !== ticket) return;
        commandPending.current = null;
        if (mounted.current && contextRef.current === owner && authOwner.current === auth) setAdminBusy(false);
      },
    };
  };
  useEffect(() => { if (!commandPending.current) setAdminBusy(false); }, [contextKey]);

  const invalidateReads = () => { readEpoch.current++; authOwner.current = { key: authKey }; };
  useLayoutEffect(()=>{const token=oidc.accessToken||devAuthToken;runtimeAuthToken=token;return requestAuthFence.activate(token);},[oidc.accessToken]);
  useLayoutEffect(()=>{requestAuthFence.setUnauthorized(oidc.accessToken||devAuthToken,()=>{runtimeAuthToken='';invalidateReads();void oidc.logout();});});
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; readEpoch.current++; }; }, []);

  useEffect(() => {
    // Poll telemetry only while a selected device is visible. This avoids
    // background API load and late updates after navigation.
    if (tab !== "Устройства" || !selected || (prodMode && !oidc.accessToken.trim())) return;
    let active = true, pending = false;
    const owner = authOwner.current, epoch = readEpoch.current, token = oidc.accessToken || devAuthToken;
    const poll = async () => {
      if (
        typeof document !== "undefined" &&
        document.visibilityState === "hidden"
      )
        return;
      if (pending) return;
      pending = true;
      const devicePath = `/devices/${encodeURIComponent(label(selected.id))}`;
      const [readiness, health, activity] = await Promise.allSettled([
        call(`${devicePath}/readiness`, {}, token),
        call(`${devicePath}/health`, {}, token),
        call(`${devicePath}/activity`, {}, token),
      ]);
      pending = false;
      if (!active || !mounted.current || authOwner.current !== owner || readEpoch.current !== epoch) return;
      setDeviceReadiness(readiness.status === 'fulfilled' ? readiness.value : { ready: false, state: 'UNAVAILABLE' });
      setDeviceHealth(health.status === 'fulfilled' ? health.value : { effective_state: 'UNKNOWN' });
      setDeviceActivity(activity.status === 'fulfilled' && Array.isArray(activity.value.items)
        ? activity.value.items as Item[] : []);
    };
    void poll();
    const timer = setInterval(() => void poll(), 5000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [tab, selected?.id, register, oidc.accessToken, roles.join(",")]);
  const transportParameter = (name: string, fallback = "") => {
    try {
      const parsed = JSON.parse(deviceTransportParameters || "{}");
      return String(parsed[name] ?? fallback);
    } catch {
      return fallback;
    }
  };
  const setTransportParameter = (
    name: string,
    value: string,
    numeric = false,
  ) => {
    let parsed: Record<string, unknown> = {};
    try {
      parsed = JSON.parse(deviceTransportParameters || "{}");
    } catch {}
    parsed[name] = numeric ? Number(value) : value;
    setDeviceTransportParameters(JSON.stringify(parsed));
  };
  const refresh = async (next = tab, registerOverride = register, continuation = false) => {
      if ((!currentRender() && !(continuation && mounted.current && contextRef.current === contextOwner)) || !tabs.includes(next)) return false;
      if (next === "Одит" && !auditFilterDatesValid({ from: auditFrom, to: auditTo })) return false;
      const owner = authOwner.current, epoch = ++readEpoch.current, token = oidc.accessToken || devAuthToken;
      const requestCurrent = () => mounted.current && authOwner.current === owner && readEpoch.current === epoch;
      const read = (path: string) => collect(path, token, requestCurrent);
      setLoading(true);
      setLoadError('');
      if (next === 'Устройства' || next === 'Операции' || next === 'Отчети' || next === 'Одит') setItems([]);
      if (next === 'Устройства') {
        setSelected(null);
        setDeviceHealth(null);
        setDeviceReadiness(null);
        setDeviceActivity([]);
        setDiagnostics(null);
      }
      if (next === 'Настройки') setPolicy(null);
      try {
        if (next === "Администриране" && !canAdminister)
          throw new Error("ROLE_ADMIN_REQUIRED");
        if (next === "Отчети" && !hasRole("SUPERVISOR", "ADMIN", "AUDITOR"))
          throw new Error("ROLE_REPORT_READ_REQUIRED");
        if (next === "Одит" && !canReadAudit)
          throw new Error("ROLE_AUDIT_READ_REQUIRED");
        let registersForContext = adminLists.registers;
        if (next !== "Администриране" && registersForContext.length === 0) {
          const [locations, registers, operators] = await Promise.all([
            read("/locations"),
            read("/registers"),
            read("/operators"),
          ]);
          if (!requestCurrent()) return false;
          setAdminLists(current => ({ ...current, locations, registers, operators }));
          registersForContext = registers;
          setLocationId(current => current || label(locations[0]?.id));
          setActivationLocationId(current => current || label(locations[0]?.id));
          setRegister(current => current || label(registers[0]?.id));
          setOperatorId(current => current || label(operators[0]?.id));
        }
        if (next === "Устройства") {
          const companyDevices = await read("/devices");
          if (!requestCurrent()) return false;
          const registerId = registerOverride || label(registersForContext[0]?.id);
          const activeRegister = registersForContext.find(item => item.id === registerId);
          const scopedDevices = devicesBoundToRegister(companyDevices, activeRegister);
          setItems(scopedDevices);
          setSelected(scopedDevices[0] || null);
        }
        if (next === "Операции") {
          const v = await read(
            registerCollectionPath("/operations", registerOverride),
          );
          if (!requestCurrent()) return false;
          setItems(v);
        }
        if (next === "Отчети") {
          const v = await read(registerCollectionPath("/reports", registerOverride));
          if (!requestCurrent()) return false;
          setItems(v);
        }
        if (next === "Одит") {
          const params = new URLSearchParams();
          if (auditActor) params.set("actor_id", auditActor);
          if (auditAction) params.set("action", auditAction);
          if (auditObjectType) params.set("object_type", auditObjectType);
          if (auditObjectId) params.set("object_id", auditObjectId);
          if (auditUnp) params.set("unp", auditUnp);
          if (auditFrom) params.set("from", auditFrom.trim());
          if (auditTo) params.set("to", auditTo.trim());
          setItems([]);
          const events = await read(`/audit-events${params.size ? `?${params}` : ""}`);
          if (!requestCurrent()) return false;
          setItems(events);
        }
        if (next === "Настройки") {
          const [p, t] = await Promise.all([
            call("/country-policy", {}, token),
            call("/tax-groups", {}, token),
          ]);
          if (!requestCurrent()) return false;
          setPolicy({ ...p, tax_groups: t });
          setItems([]);
        }
        if (next === "Администриране") {
          const [locations, registers, operators, devices] = await Promise.all([
            read("/locations"),
            read("/registers"),
            read("/operators"),
            read("/devices"),
          ]);
          if (!requestCurrent()) return false;
          const lists = {
            locations,
            registers,
            operators,
            devices,
          };
          setAdminLists(lists);
          setLocationId((x) => x || label(lists.locations[0]?.id));
          setRegister((x) => x || label(lists.registers[0]?.id));
          setDeviceId((x) => x || label(lists.devices[0]?.id));
          setOperatorId((x) => x || label(lists.operators[0]?.id));
          setItems([]);
          const selectedRegister = registerOverride || label(lists.registers[0]?.id);
          if (isRegisterId(selectedRegister)) {
            const history = await call(
              registerResourcePath(selectedRegister, "/composite-bindings"), {}, token,
            );
            if (!requestCurrent()) return false;
            const bindings = Array.isArray(history.items)
              ? (history.items as Item[])
              : [];
            setCompositeBinding(
              bindings.find(
                (x) => x.status === "PENDING" || x.status === "ACTIVE",
              ) ||
                bindings[0] ||
                null,
            );
          }
        }
        if (!requestCurrent()) return false;
        setMessage(language==='en'?`${tabLabel(next)}: data refreshed from BeeFiscal API`:`${next}: данните са обновени от BeeFiscal API`);
        setCore("CORE READY");
        return true;
      } catch (e) {
        if (!requestCurrent()) return false;
        setCore("CORE OFFLINE");
        setLoadError(reportCopy(language).loadError);
        setMessage(reportCopy(language).loadError);
        return false;
      } finally {
        if (requestCurrent()) setLoading(false);
      }
  };
  useEffect(() => {
    runtimeAuthToken = oidc.accessToken || devAuthToken;
    if (prodMode && !oidc.accessToken) {
      setItems([]); setSelected(null); setAdminLists({ locations: [], registers: [], operators: [], devices: [] });
      setCore("AUTH REQUIRED");
      setMessage("Влезте с персоналния си административен профил");
      return;
    }
    setTab("Устройства");
    void refresh("Устройства");
  }, [oidc.accessToken]);
  useEffect(() => {
    if (!tabs.includes(tab)) {
      setTab("Устройства");
      setItems([]);
    }
  }, [roles.join(","), tab]);
  const changeTab = (next: string) => {
    if (!currentRender() || commandPending.current || adminBusy || !tabs.includes(next)) return;
    setTab(next);
    setItems([]);
    setLoadError('');
    setReportError('');
    void refresh(next);
  };
  const selectRegisterContext = (nextRegister: string) => {
    if (!currentRender() || nextRegister === register || !adminLists.registers.some(item => item.id === nextRegister)) return;
    setRegister(nextRegister);
    setItems([]);
    setSelected(null);
    setDeviceHealth(null);
    setDeviceReadiness(null);
    setDeviceActivity([]);
    setDiagnostics(null);
    setReportError('');
    void refresh(tab, nextRegister);
  };
  const probe = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!isRegisterId(register)) {
      setMessage("Изберете валиден Fiscal register UUID преди сквозния probe");
      return;
    }
    if (!selected) {
      setMessage("Няма регистрирано устройство за probe");
      return;
    }
    const command = beginDeviceCommand(); if (!command) return;
    setMessage("Проверка на cloud route, Edge, driver и крайното ФУ…");
    try {
      const id = label(selected.id);
      const route = await command.call(
        registerResourcePath(register, "/connectivity-probes"),
        { method: "POST", body: "{}" },
      );
      if (!command.live()) return;
      const device = await command.call(`/devices/${encodeURIComponent(id)}/readiness`);
      if (!command.live()) return;
      setTransportMetrics({
        state: route.state,
        hops: route.hops,
        recommended_transport: route.recommended_transport,
      });
      setFiscalMetrics({
        ready: device.ready,
        components: device.components,
        observed_at: device.observed_at,
      });
      setMessage(
        `Cloud ${route.hops?.cloud?.state || route.state} → Edge ${route.hops?.edge?.state || "?"} → Driver ${device.driver} → ФУ ${device.fiscal_device}`,
      );
    } catch (e) {
      if (!command.live()) return;
      setMessage(
        `Probe отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally { command.finish(); }
  };
  const loadDiagnostics = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!selected || !canDiagnose) {
      setMessage("Диагностиката изисква роля ADMIN или SERVICE");
      return;
    }
    const command = beginDeviceCommand(); if (!command) return;
    try {
      const id = encodeURIComponent(label(selected.id));
      const [capabilities, details] = await Promise.all([
        command.call(`/devices/${id}/capabilities`),
        command.call(`/devices/${id}/diagnostics`),
      ]);
      if (!command.live()) return;
      setDiagnostics({ capabilities, details });
      setMessage("Диагностиката е заредена с приложени server redactions");
    } catch (e) {
      if (!command.live()) return;
      setMessage(
        `Диагностиката е отказана: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      command.finish();
    }
  };
  const printerTest = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!selected || !canDiagnose) {
      setMessage("Тестът на принтера изисква ADMIN или SERVICE");
      return;
    }
    const command = beginDeviceCommand(); if (!command) return;
    try {
      const result = await command.call(
        `/devices/${encodeURIComponent(label(selected.id))}/tests/printer`,
        { method: "POST", body: "{}" },
      );
      if (!command.live()) return;
      setMessage(
        `Тест на принтера: ${label(result.state)} • ${label(result.id)}`,
      );
    } catch (e) {
      if (!command.live()) return;
      setMessage(
        `Тестът на принтера е отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      command.finish();
    }
  };
  const startProvisioning = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!selected || !canAdminister) {setMessage("Настройката изисква роля ADMIN");return;}
    setProvisioningOpen(true);
  };
  const installProvisioning = async (settings:{ssid:string;wifiPassword:string;fiscalPassword:string}) => {
    if (!currentRender() || !selected || !canAdminister || tab!=="Устройства" || !compositeBinding) return;
    const deviceID=label(selected.id),deviceKID=label(selected.transaction_signing_kid);
    if(compositeBinding.adapter_device_id!==deviceID||compositeBinding.register_id!==register||!deviceKID||!compositeBinding.payload_sha256){setMessage("Изберете адаптера и подписаната конфигурация за текущата каса");return;}
    const command=beginDeviceCommand();if(!command)return;setProvisioningBusy(true);
    try{
      const result=await provisionDevice({target:{deviceID,deviceKID,bindingID:label(compositeBinding.id),bindingSHA256:label(compositeBinding.payload_sha256),generation:Number(compositeBinding.generation)},settings,connect:connectProvisioningDevice,live:command.live,call:(path,body)=>command.call(path,{method:'POST',body:JSON.stringify(body)}),sha256:async(bytes)=>{const digest=await ExpoCrypto.digest(ExpoCrypto.CryptoDigestAlgorithm.SHA256,new Uint8Array(bytes));return Array.from(new Uint8Array(digest),b=>b.toString(16).padStart(2,'0')).join('')}});
      if(!command.live())return;setProvisioning(result);setProvisioningOpen(false);setMessage("Инсталацията е потвърдена от адаптера. Проверете свързаните устройства преди работа.");
    }catch(error){if(command.live())setMessage(`Настройката не е потвърдена: ${error instanceof Error?error.message:String(error)}`)}finally{settings.wifiPassword='';settings.fiscalPassword='';setProvisioningBusy(false);command.finish();}
  };
  const issueBleSession = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!canIssueBleSession) {
      setMessage("BLE сесията изисква оперативна роля");
      return;
    }
    if (!isRegisterId(register) || !adminLists.registers.some(item => item.id === register) || !adminLists.operators.some(item => item.id === operatorId) || !blePublicKey.trim()) {
      setMessage(
        "BLE сесията изисква валиден register UUID, operator и предварително подготвен client public key",
      );
      return;
    }
    const command = beginDeviceCommand(); if (!command) return;
    try {
      const result = await command.call(
        registerResourcePath(register, "/ble-sessions"),
        {
          method: "POST",
          body: JSON.stringify({
            operator_id: operatorId,
            app_instance_id: bleAppInstance,
            public_key: blePublicKey.trim(),
          }),
        },
      );
      if (!command.live()) return;
      setBleSession(result);
      setMessage(
        `BLE сесия ${label(result.ble_session_id)} • валидна до ${label(result.expires_at)}`,
      );
    } catch (e) {
      if (!command.live()) return;
      setMessage(
        `BLE сесията е отказана: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      command.finish();
    }
  };
  const activateBlueCash = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!activationRequest || !canAdminister || typeof activationRequest.activation_request_id !== "string" || !activationRequest.activation_request_id.trim() || !activationCode.trim()) {
      setMessage("Първо намерете чакащата заявка с QR/краткия код");
      return;
    }
    if (!isRegisterId(activationLocationId) || !adminLists.locations.some(item => item.id === activationLocationId) || !isRegisterId(register) || !adminLists.registers.some(item => item.id === register)) {
      setMessage("Изберете валидни търговска точка и касово място");
      return;
    }
    const command = beginDeviceCommand(); if (!command) return;
    try {
      const confirmed = await command.call(
        `/device-activation-requests/${encodeURIComponent(label(activationRequest.activation_request_id))}:confirm`,
        {
          method: "POST",
          body: JSON.stringify({
            user_code: activationCode.trim().toUpperCase(),
            location_id: activationLocationId,
            register_id: register,
            roles: activationRequest.requested_roles,
          }),
        },
      );
      if (!command.live()) return;
      setBlueCashActivation(confirmed);
      setActivationRequest(null);
      setMessage(
        `Заявката е потвърдена. Организацията е взета от вашия tenant token; устройството ще завърши key-bound activation.`,
      );
    } catch (e) {
      if (!command.live()) return;
      setMessage(
        `BlueCash activation е отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      command.finish();
    }
  };
  const lookupActivation = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!canAdminister || !activationCode.trim()) return;
    const command = beginDeviceCommand(); if (!command) return;
    try {
      const pending = await command.call(
        `/device-activation-requests:lookup?user_code=${encodeURIComponent(activationCode.trim().toUpperCase())}`,
      );
      if (!command.live()) return;
      if (typeof pending.activation_request_id !== "string" || !pending.activation_request_id.trim() || !Array.isArray(pending.requested_roles) || !pending.requested_roles.every((role: unknown) => typeof role === "string")) throw new Error("INVALID_ACTIVATION_REQUEST");
      setActivationRequest(pending);
      setMessage(
        "Проверете vendor, model, serial, FMIN и key thumbprint преди потвърждение",
      );
    } catch (e) {
      if (!command.live()) return;
      setActivationRequest(null);
      setMessage(
        `Заявката не е намерена: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      command.finish();
    }
  };
  const disconnectDevice = async () => {
    if (!currentRender() || loading || adminBusy || commandPending.current || tab !== "Устройства") return;
    if (!selected || !canAdminister) return;
    const command = beginDeviceCommand(); if (!command) return;
    try {
      await command.call(
        `/devices/${encodeURIComponent(label(selected.id))}:disconnect`,
        { method: "POST", body: "{}" },
      );
      if (!command.live()) return;
      setMessage(
        "Устройството, credential-ът, BLE сесиите и register bindings са оттеглени",
      );
      await refresh("Устройства", register, true);
      if (!command.live()) return;
    } catch (e) {
      if (!command.live()) return;
      setMessage(
        `Отключването е отказано: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      command.finish();
    }
  };
  const report = async (type: string) => {
    if (!currentRender() || loading || adminBusy || commandPending.current || !["X", "Z", "KLEN", "FISCAL_MEMORY"].includes(type)) return;
    if (!canRunReports) {
      setMessage(reportCopy(language).roleError);
      return;
    }
    if (!isRegisterId(register)) {
      setMessage(reportCopy(language).registerError);
      return;
    }
    const ticket = {}; commandPending.current = ticket;
    const commandLive = () => mounted.current && contextRef.current === contextOwner && commandPending.current === ticket;
    const token = oidc.accessToken || devAuthToken;
    setLoading(true);
    setReportError('');
    setMessage(reportCopy(language).pending.replace('{type}',type));
    try {
      const v = await call(registerResourcePath(register, "/reports"), {
        method: "POST",
        body: JSON.stringify({ type }),
      }, token);
      if (!commandLive()) return;
      setMessage(`${type}: ${v.state} • ${v.fiscal_reference || v.id}`);
      await refresh("Отчети", register, true);
    } catch (e) {
      if (!commandLive()) return;
      setReportError(reportCopy(language).commandError);
      setMessage(
        reportCopy(language).commandError,
      );
    } finally {
      if (commandPending.current === ticket) { commandPending.current = null; if (mounted.current && contextRef.current === contextOwner) setLoading(false); }
    }
  };
  const reconcileOperation = async (operation: Item) => {
    if (!currentRender() || loading || adminBusy || commandPending.current || !items.includes(operation) || typeof operation.id !== "string" || !operation.id.trim()) return;
    if (!canReconcile) {
      setMessage("Reconciliation изисква роля SUPERVISOR или ADMIN");
      return;
    }
    const actions = Array.isArray(operation.allowed_actions)
      ? operation.allowed_actions
      : [];
    if (!actions.includes("RECONCILE")) {
      setMessage("Reconciliation е блокирано: backend не го разрешава");
      return;
    }
    const ticket = {}; commandPending.current = ticket;
    const commandLive = () => mounted.current && contextRef.current === contextOwner && commandPending.current === ticket;
    const token = oidc.accessToken || devAuthToken;
    setLoading(true);
    setMessage(`Reconciliation ${label(operation.id)}…`);
    try {
      const result = await call(
        `/operations/${encodeURIComponent(label(operation.id))}/reconcile`,
        { method: "POST", body: "{}" }, token,
      );
      if (!commandLive()) return;
      setMessage(
        `Reconciliation: ${label(result.state)} • ${label(result.operation_id || result.id)}`,
      );
      await refresh("Операции", register, true);
    } catch (e) {
      if (!commandLive()) return;
      setMessage(
        `Reconciliation е отказано: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      if (commandPending.current === ticket) { commandPending.current = null; if (mounted.current && contextRef.current === contextOwner) setLoading(false); }
    }
  };
  const mutateAdmin = async (
    description: string,
    action: (request: AdminRequest) => Promise<unknown>,
  ) => {
    const command = beginAdminCommand(); if (!command) return;
    setMessage(`${description}…`);
    try {
      const result = await action(command.call);
      if (!command.live()) return;
      const commit = result && typeof result === 'object' ? result as { apply?: unknown; registerId?: unknown } : null;
      const targetRegister = typeof commit?.registerId === 'string' ? commit.registerId : register;
      const epoch = readEpoch.current;
      const refreshed = await refresh("Администриране", targetRegister, true);
      if (!command.live() || readEpoch.current !== epoch + 1) return;
      if (typeof commit?.apply === 'function') commit.apply();
      setMessage(refreshed ? `${description}: готово` : `${description}: записано, но списъкът не се обнови`);
    } catch (e) {
      if (!command.live()) return;
      setMessage(`${description}: ${e instanceof Error ? e.message : String(e)}`);
    } finally { command.finish(); }
  };
  const stageCompositeBinding = async () => {
    if (!isRegisterId(register) || !adminLists.registers.some(item => item.id === register)
      || !compositeSelectionValid(compositeProfile,adapterDeviceId,fiscalEndpointId,paymentEndpointId,adminLists.devices)
      || !adminLists.devices.some(item => item.id === adapterDeviceId)
      || !adminLists.devices.some(item => item.id === fiscalEndpointId)
      || (paymentEndpointId && !adminLists.devices.some(item => item.id === paymentEndpointId))
      || !Number.isSafeInteger(Number(expectedRegisterVersion)) || Number(expectedRegisterVersion) < 1) return;
    const command = beginAdminCommand(); if (!command) return;
    try {
      const v = await command.call(
        registerResourcePath(register, "/composite-bindings"),
        {
          method: "POST",
          body: JSON.stringify({
            profile: compositeProfile,
            adapter_device_id: adapterDeviceId,
            fiscal_device_id: fiscalEndpointId,
            ...(paymentEndpointId
              ? { payment_device_id: paymentEndpointId }
              : {}),
            expected_register_version: Number(expectedRegisterVersion),
          }),
        },
      );
      if (!command.live()) return;
      setCompositeBinding(v);
      setMessage(`Composite generation ${label(v.generation)} е подготвена`);
    } catch (e) {
      if (!command.live()) return;
      setMessage(
        `Composite binding: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      command.finish();
    }
  };
  const disableCompositeBinding = async () => {
    if (!compositeBinding || !isRegisterId(register) || !isRegisterId(String(compositeBinding.binding_id || ''))
      || compositeBinding.register_id !== register || compositeBinding.status === 'REVOKED'
      || !Number.isSafeInteger(compositeBinding.version) || Number(compositeBinding.version) < 1) return;
    const command = beginAdminCommand(); if (!command) return;
    try {
      const v = await command.call(
        registerResourcePath(register, "/composite-bindings/disable"),
        {
          method: "POST",
          body: JSON.stringify({
            binding_id: compositeBinding.binding_id,
            expected_version: compositeBinding.version,
          }),
        },
      );
      if (!command.live()) return;
      setCompositeBinding(v);
      // Disabling a pending binding does not increment the register version.
      // Read the authoritative version instead of guessing +1.
      const currentRegister = await command.call(registerResourcePath(register));
      if (!command.live()) return;
      if (currentRegister.id !== register || !Number.isSafeInteger(currentRegister.version) || Number(currentRegister.version) < 1) throw new Error('INVALID_REGISTER_RESPONSE');
      setExpectedRegisterVersion(String(currentRegister.version));
      setMessage(
        "Composite binding е изключен; може да бъде подготвена нова връзка",
      );
    } catch (e) {
      if (!command.live()) return;
      setMessage(`Disable: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      command.finish();
    }
  };
  if (prodMode && !oidc.accessToken) {
    return <AuthScreen auth={oidc} />;
  }
  return (
    <><SafePaperSurface ref={shellRef} aria-hidden={provisioningVisible} accessibilityElementsHidden={provisioningVisible} importantForAccessibility={provisioningVisible?"no-hide-descendants":"auto"} mode="flat" elevation={0} style={{flex:1,backgroundColor:theme.colors.background}}><SafeAreaView style={[s.root, { }]}>
      <StatusBar style={theme.dark ? 'light' : 'dark'} />
      <LanguageThemeControls testPrefix="shell" title={tabLabel(tab)} menu={{testID:"nav.open",disabled:adminBusy||Boolean(commandPending.current),onPress:()=>{if(currentRender()&&!commandPending.current&&!adminBusy)setNavOpen(true);}}} />
      {navOpen ? <Portal>
        <Modal testID="shell.navigation.modal" visible={navOpen} onDismiss={() => setNavOpen(false)} contentContainerStyle={{ margin: 16, maxHeight: '80%', borderRadius: 16, padding: 12, backgroundColor: theme.colors.surface }}>
          <ScrollView>
            <Drawer.Section title="BeeFiscal">
              {tabs.map(x => <List.Item key={x} testID={`tab-${x}`} disabled={adminBusy||Boolean(commandPending.current)} title={tabLabel(x)} titleNumberOfLines={0} titleMaxFontSizeMultiplier={0} titleStyle={{ color: tab === x ? theme.colors.onSecondaryContainer : theme.colors.onSurface }} style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: 12, marginVertical: 4, backgroundColor: tab === x ? theme.colors.secondaryContainer : theme.colors.surface }} accessibilityRole="button" accessibilityState={{ selected: tab === x }} accessibilityLabel={shell.open.replace('{name}',tabLabel(x))} onPress={() => { if(!currentRender())return;setNavOpen(false); changeTab(x); }} />)}
            </Drawer.Section>
          </ScrollView>
        </Modal>
      </Portal> : null}
      <PaintedPaperSurface mode="flat" elevation={0} style={[s.header, { backgroundColor: theme.colors.surface, borderColor: theme.colors.outlineVariant }, compact && { flexWrap: 'wrap', gap: 8, padding: 12 }]}>
        <Surface elevation={0}
          testID="status-fiscal-device"
          accessibilityLiveRegion="polite"
          style={[s.ready, { backgroundColor: core.includes("OFFLINE") ? theme.colors.errorContainer : theme.colors.secondaryContainer }]}
        >
          <PaperText style={{minWidth:0,flexShrink:1,flexWrap:"wrap",color: core.includes("OFFLINE") ? theme.colors.onErrorContainer : theme.colors.onSecondaryContainer}}>Fiscal Core: {core}</PaperText>
        </Surface>
        {prodMode && oidc.tenants.length > 1 ? <View style={s.actions}>{oidc.tenants.map(t=><AppButton key={t.tenant_id} mode={t.tenant_id===oidc.currentTenant?.tenant_id ? 'contained-tonal' : 'outlined'} contentStyle={{ minHeight: 48 }} disabled={!oidc.ready || loading || adminBusy || Boolean(commandPending.current) || t.tenant_id===oidc.currentTenant?.tenant_id} accessibilityLabel={shell.company.replace('{name}',t.display_name)} onPress={()=>{if(!currentRender()||!oidc.ready||loading||adminBusy||commandPending.current||t.tenant_id===oidc.currentTenant?.tenant_id)return;invalidateReads();setItems([]);setSelected(null);setRegister('');setAdminLists({locations:[],registers:[],operators:[],devices:[]});void oidc.switchTenant(t.tenant_id)}}>{t.display_name}</AppButton>)}</View>:null}
        {prodMode ? (
          <AppButton
            testID="admin-logout"
            disabled={loading||adminBusy||Boolean(commandPending.current)}
            mode="outlined"
            contentStyle={{ minHeight: 48 }}
            accessibilityLabel={shell.logoutLabel}
            onPress={() => {
              if(!currentRender()||loading||adminBusy||commandPending.current)return;
              invalidateReads();
              requestAuthFence.revoke();runtimeAuthToken = "";
              void oidc.logout();
            }}
          >{shell.logout}</AppButton>
        ) : null}
      </PaintedPaperSurface>
      <View style={s.body}>
        {!compact ? <PaintedPaperSurface testID="shell.navigation.surface" mode="flat" elevation={0} style={[s.nav,{padding:0,gap:0,backgroundColor:theme.colors.surface}]}><ScrollView contentContainerStyle={{ paddingHorizontal: 0 }} style={s.nav}>
          <Drawer.Section>
            {tabs.map(x => <List.Item key={x} testID={`tab-${x}`} disabled={adminBusy||Boolean(commandPending.current)} title={tabLabel(x)} titleNumberOfLines={0} titleMaxFontSizeMultiplier={0} titleStyle={{ color: tab === x ? theme.colors.onSecondaryContainer : theme.colors.onSurface }} style={{ minHeight: 48, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: 12, marginVertical: 4, backgroundColor: tab === x ? theme.colors.secondaryContainer : theme.colors.surface }} accessibilityRole="button" accessibilityState={{ selected: tab === x }} accessibilityLabel={shell.open.replace('{name}',tabLabel(x))} onPress={() => changeTab(x)} />)}
          </Drawer.Section>
        </ScrollView></PaintedPaperSurface> : null}
        <PaintedPaperSurface testID="shell.content.surface" mode="flat" elevation={0} style={[s.content,{backgroundColor:theme.colors.background}]}><ScrollView testID="operational.content" style={s.content}
          contentContainerStyle={{ paddingVertical: compact ? 16 : 20 }}>
          {loading ? (
            <PaperText
              testID="screen-loading"
              accessibilityLiveRegion="polite"
              style={[s.loadingText, { color: theme.colors.onSurfaceVariant, backgroundColor: theme.colors.surfaceVariant }]}
            >
              {language==='en'?'Loading from BeeFiscal API…':'Зареждане от BeeFiscal API…'}
            </PaperText>
          ) : null}
          <View style={[s.toolbar, compact && { flexWrap: 'wrap' }]}>
            <ReferencePicker disabled={adminBusy} label={shell.register} items={adminLists.registers} value={register} labelFor={(x)=>label(x.code || x.name)} onSelect={selectRegisterContext} onEmpty={()=>changeTab("Администриране")}/>
            <AppButton mode="contained-tonal" contentStyle={{ minHeight: 48 }} accessibilityLabel={language==='en'?`Refresh ${tabLabel(tab)}`:`Обнови ${tab}`} disabled={loading || adminBusy} onPress={() => refresh()}>{loading ? '…' : language==='en'?'Refresh':'Обнови'}</AppButton>
          </View>
          {tab === "Устройства" ? (
            <DevicesScreen
              dataOwnerKey={contextKey}
              devices={items} selected={selected} loading={loading} error={loadError}
              canDiagnose={canDiagnose} canAdminister={canAdminister} canIssueBleSession={canIssueBleSession}
              lists={adminLists}
              context={{ register, operatorId, blePublicKey, activationCode, activationRequest, activationLocationId }}
              telemetry={{ transportMetrics, deviceHealth, deviceReadiness, deviceActivity, fiscalMetrics, diagnostics, provisioning, bleSession, blueCashActivation }}
              actions={{
                retry: () => void refresh('Устройства'),
                selectDevice: device => { if (!currentRender() || loading || loadError || !items.includes(device)) return; setSelected(device); setDeviceHealth(null); setDeviceReadiness(null); setDeviceActivity([]); setDiagnostics(null); setMessage(language==='en'?`${label(device.name || device.id)} selected`:`${label(device.name || device.id)} е избрано`); },
                openAdministration: () => changeTab('Администриране'),
                diagnostics: () => void loadDiagnostics(),
                printerTest: () => void printerTest(),
                provisioning: () => void startProvisioning(),
                selectOperator: id => { if (currentRender() && !loading && !commandPending.current && adminLists.operators.some(item => item.id === id)) setOperatorId(id); },
                changeBlePublicKey: value => { if (currentRender() && !loading && !commandPending.current) setBlePublicKey(value); },
                issueBleSession: () => void issueBleSession(),
                changeActivationCode: value => { if (currentRender() && !loading && !commandPending.current) { setActivationCode(value); setActivationRequest(null); setBlueCashActivation(null); } },
                lookupActivation: () => void lookupActivation(),
                selectActivationLocation: id => { if (currentRender() && !loading && !commandPending.current && adminLists.locations.some(item => item.id === id)) setActivationLocationId(id); },
                selectRegister: selectRegisterContext,
                confirmActivation: () => void activateBlueCash(),
                disconnectDevice: () => void disconnectDevice(),
              }}
            />
          ) : tab === "Операции" ? (
            <OperationsScreen loadError={loadError} onRetry={() => void refresh('Операции')} operations={items} loading={loading} canReconcile={canReconcile} onReconcile={(operation) => void reconcileOperation(operation)} />
          ) : tab === "Отчети" ? (
            <ReportsScreen reports={items} loading={loading} canRunReports={canRunReports} registerValid={isRegisterId(register)} loadError={loadError} commandError={reportError} onReport={(type) => void report(type)} onRetry={() => void refresh('Отчети')} />
          ) : tab === "Одит" && canReadAudit ? (
            <AuditScreen events={items} loading={loading} error={loadError}
              filters={{ actor: auditActor, action: auditAction, objectType: auditObjectType, objectId: auditObjectId, unp: auditUnp, from: auditFrom, to: auditTo }}
              onChange={(name: keyof AuditFilters, value: string) => {
                if(!currentRender()||loading||commandPending.current||tab!=="Одит"||!canReadAudit)return;
                const setters = { actor: setAuditActor, action: setAuditAction, objectType: setAuditObjectType, objectId: setAuditObjectId, unp: setAuditUnp, from: setAuditFrom, to: setAuditTo };
                setters[name](value);
              }} onApply={() => void refresh('Одит')} />
          ) : tab === "Администриране" && canAdminister ? (
            <AdministrationScreen {...{
              adminLists,
              adminBusy: adminBusy || loading,
              compositeBinding,
              compositeProfile,
              mutateAdmin,
              stageCompositeBinding,
              disableCompositeBinding,
              transportParameter,
              setTransportParameter,
              locationCode,
              locationName,
              locationAddress,
              registerCode,
              locationId,
              operatorCode,
              operatorId,
              operatorFirstName,
              operatorLastName,
              deviceVendor,
              deviceModel,
              deviceSerial,
              deviceKind,
              deviceDriverId,
              deviceProtocolId,
              deviceProtocolVersion,
              deviceTransport,
              deviceTransportParameters,
              bindingRole,
              deviceId,
              adapterDeviceId,
              fiscalEndpointId,
              paymentEndpointId,
              expectedRegisterVersion,
              register,
              setLocationCode,
              setLocationName,
              setLocationAddress,
              setRegisterCode,
              setLocationId,
              setOperatorCode,
              setOperatorId,
              setOperatorFirstName,
              setOperatorLastName,
              setDeviceVendor,
              setDeviceModel,
              setDeviceSerial,
              setDeviceKind,
              setDeviceDriverId,
              setDeviceProtocolId,
              setDeviceProtocolVersion,
              setDeviceTransport,
              setDeviceTransportParameters,
              setBindingRole,
              setDeviceId,
              setAdapterDeviceId,
              setFiscalEndpointId,
              setPaymentEndpointId,
              setExpectedRegisterVersion,
              setRegister,
              setCompositeProfile,
              setMessage,
            }} />
          ) : (
            <SettingsScreen policy={policy} loading={loading} error={loadError} onRetry={() => void refresh('Настройки')} />
          )}
          {items.length === 0 &&
          tab !== "Настройки" &&
          tab !== "Устройства" &&
          tab !== "Одит" &&
          tab !== "Отчети" &&
          tab !== "Операции" &&
          tab !== "Администриране" ? (
            <PaperText variant="bodyMedium" style={{ paddingVertical: 32, textAlign: 'center' }}>Няма записи в API за избрания филтър.</PaperText>
          ) : null}
        </ScrollView></PaintedPaperSurface>
      </View>
      <PaintedPaperSurface mode="flat" elevation={0}
        testID="status-transport"
        accessibilityLiveRegion="polite"
        style={[s.footer, { backgroundColor: theme.colors.surface, borderColor: theme.colors.outlineVariant }, compact && { flexWrap: 'wrap', gap: 8 }]}
      >
        <PaperText style={[s.footerText, { color: theme.colors.onSurface }]}>{language==='en'&&message==='Зареждане на данните от публичния API…'?'Loading data from the public API…':message}</PaperText>
        <AppButton
          testID="readiness-probe"
          mode="contained"
          contentStyle={{ minHeight: 48 }}
          accessibilityLabel={language==='en'?'Check cloud, Edge, driver and fiscal device':'Провери cloud, Edge, driver и крайното фискално устройство'}
          disabled={loading || adminBusy || tab !== "Устройства" || !selected || !isRegisterId(register)}
          onPress={probe}
        >{language==='en'?'Connectivity check':'Сквозен probe'}</AppButton>
      </PaintedPaperSurface>
    </SafeAreaView></SafePaperSurface>
    <ProvisioningDialog key={contextKey} visible={provisioningVisible} busy={provisioningBusy} onCancel={()=>setProvisioningOpen(false)} onSubmit={installProvisioning}/></>
  );
}

const s = StyleSheet.create({
  root: { flex: 1 },
  logout: {
    minHeight: 48,
    paddingHorizontal: 14,
    backgroundColor: beeloyColors.tertiary,
    borderRadius: 9,
    justifyContent: "center",
  },
  header: {
    flexWrap: 'wrap',
    gap: 8,
    padding: 20,
    backgroundColor: beeloyColors.surface,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    borderBottomWidth: 1,
    borderColor: beeloyColors.tertiaryContainer,
  },
  title: { flexShrink: 1 },
  sub: { color: beeloyColors.tertiary, marginTop: 3 },
  ready: {
    minWidth: 0,
    maxWidth: '100%',
    flexShrink: 1,
    backgroundColor: beeloyColors.successContainer,
    padding: 10,
    borderRadius: 20,
    minHeight: 48,
    justifyContent: "center",
  },
  offline: { backgroundColor: beeloyColors.errorContainer },
  readyText: { color: beeloyColors.success, fontWeight: "900" },
  body: { flex: 1, flexDirection: "row" },
  nav: { width: 190, flexBasis: 190, flexGrow: 0, flexShrink: 0, padding: 12, gap: 6 },
  content: { flex: 1 },
  toolbar: {
    flexWrap: 'wrap',
    flexDirection: "row",
    gap: 10,
    alignItems: "center",
    marginBottom: 16,
  },
  section: {
    flexBasis: '100%',
    maxWidth: '100%',
    flexShrink: 1, flex: 1 },
  card: {
    backgroundColor: beeloyColors.surface,
    padding: 18,
    borderRadius: 14,
    marginBottom: 12,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    borderWidth: 1,
    borderColor: beeloyColors.tertiaryContainer,
  },
  selected: { borderWidth: 2, borderColor: beeloyColors.success },
  device: { fontSize: 18, fontWeight: "800", color: beeloyColors.tertiary },
  meta: { color: beeloyColors.tertiary, marginTop: 5 },
  state: {
    paddingHorizontal: 11,
    paddingVertical: 8,
    borderRadius: 8,
    backgroundColor: beeloyColors.tertiary,
  },
  stateText: { color: beeloyColors.onPrimary, fontSize: 11, fontWeight: "900" },
  panel: {
    backgroundColor: beeloyColors.surface,
    borderRadius: 14,
    padding: 20,
    marginBottom: 14,
  },
  panelTitle: { fontSize: 18, fontWeight: "800" },
  empty: { paddingVertical: 60, textAlign: "center", color: beeloyColors.tertiary },
  loadingText: {
    padding: 10,
    marginBottom: 10,
    color: beeloyColors.tertiary,
    backgroundColor: beeloyColors.tertiaryContainer,
    borderRadius: 8,
  },
  actions: { flexDirection: "row", gap: 10, marginTop: 20, flexWrap: "wrap" },
  action: {
    backgroundColor: beeloyColors.tertiary,
    padding: 14,
    borderRadius: 9,
    minHeight: 48,
    minWidth: 48,
    justifyContent: "center",
  },
  disabled: { opacity: 0.55 },
  adminCard: {
    backgroundColor: beeloyColors.surface,
    borderRadius: 14,
    padding: 18,
    gap: 10,
    minWidth: 280,
    flexGrow: 1,
    flexBasis: 320,
  },
  adminInput: {
    backgroundColor: beeloyColors.background,
    borderWidth: 1,
    borderColor: beeloyColors.tertiaryContainer,
    borderRadius: 9,
    paddingHorizontal: 12,
    minHeight: 48,
  },
  choices: { gap: 8, paddingVertical: 2 },
  choice: {
    borderWidth: 1,
    borderColor: beeloyColors.tertiaryContainer,
    borderRadius: 8,
    paddingHorizontal: 10,
    minHeight: 48,
    justifyContent: "center",
  },
  json: { marginTop: 16, fontFamily: "monospace", color: beeloyColors.tertiary },
  footer: {
    padding: 12,
    borderTopWidth: 1,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  footerText: { color: beeloyColors.tertiary, flex: 1 },
  probeText: { color: beeloyColors.onPrimary, fontWeight: "800" },
});
