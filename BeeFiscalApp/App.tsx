import { colors as beeloyColors } from './src/ui/tokens';
import { SafeAreaView } from 'react-native-safe-area-context';
import { ResponsiveScrollView as ScrollView } from './src/ui/responsive-scroll-view';
import { AuthScreen } from './src/screens/AuthScreen';
import { OperationsScreen } from './src/screens/OperationsScreen';
import { ReportsScreen } from './src/screens/ReportsScreen';
import { DevicesScreen } from './src/screens/DevicesScreen';
import { AuditScreen, type AuditFilters } from './src/screens/AuditScreen';
import { SettingsScreen } from './src/screens/SettingsScreen';
import { LanguageThemeControls } from './src/ui/LanguageThemeControls';
import { Action, AdminCard, Choices, Field, ReferencePicker } from './src/components/FiscalControls';
import { StatusBar } from "expo-status-bar";
import { useCallback, useEffect, useRef, useState } from "react";
import { StyleSheet, View } from "react-native";
import { useWindowDimensions } from 'react-native';
import { Button, Drawer, Modal, Portal, Text as PaperText, useTheme } from 'react-native-paper';
import { MVP1_DEVICE_PROFILES } from "./src/deviceProfiles";
import { fetchWithTimeout } from "./src/http";
import { collectCursorPages } from "./src/pagination";
import {
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
let runtimeUnauthorized: (() => void) | undefined;
type Item = Record<string, unknown>;
type AdminLists = {
  locations: Item[];
  registers: Item[];
  operators: Item[];
  devices: Item[];
};

async function call(path: string, init: RequestInit = {}) {
  // Every mutation is independently idempotent. A PROD 401 clears local
  // authority immediately instead of leaving privileged controls usable.
  const mutation = !!init.method && init.method !== "GET";
  const r = await fetchWithTimeout(base + path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-Api-Version": version,
      ...(mutation
        ? {
            "Idempotency-Key": `${Date.now()}-${Math.random().toString(16).slice(2)}-admin`,
          }
        : {}),
      ...(runtimeAuthToken
        ? { Authorization: `Bearer ${runtimeAuthToken}` }
        : {}),
      ...(init.headers || {}),
    },
  });
  const text = await r.text();
  if (r.status === 401 && prodMode) runtimeUnauthorized?.();
  if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
  return text ? JSON.parse(text) : {};
}
const collect = (path: string) =>
  collectCursorPages<Item>(path, (next) => call(next));
const label = (v: unknown) =>
  typeof v === "string" ? v : typeof v === "number" ? String(v) : "—";

export default function App() { return <BeeloyAppProvider><View style={{flex:1}}><AppContent/><DemoBadge/></View></BeeloyAppProvider>; }
function DemoBadge(){return Constants.expoConfig?.extra?.isDemo?<View pointerEvents="none" style={demo.badge}><PaperText style={demo.text}>DEMO</PaperText></View>:null}
const demo=StyleSheet.create({badge:{position:"absolute",right:8,top:8,zIndex:9999,backgroundColor:beeloyColors.error,borderRadius:5,paddingHorizontal:9,paddingVertical:4,elevation:8},text:{color:beeloyColors.onPrimary,fontWeight:"900",fontSize:12,letterSpacing:1}});
function AppContent() {
  const { width } = useWindowDimensions();
  const compact = width < 600;
  const theme = useTheme();
  const [navOpen, setNavOpen] = useState(false);
  const reportPending = useRef(false);
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
  runtimeUnauthorized = () => oidc.logout();
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
    [diagnostics, setDiagnostics] = useState<Item | null>(null),
    [deviceHealth, setDeviceHealth] = useState<Item | null>(null),
    [deviceActivity, setDeviceActivity] = useState<Item[]>([]),
    [provisioning, setProvisioning] = useState<Item | null>(null),
    [bleSession, setBleSession] = useState<Item | null>(null),
    [blueCashActivation, setBlueCashActivation] = useState<Item | null>(null),
    [activationCode, setActivationCode] = useState(""),
    [activationRequest, setActivationRequest] = useState<Item | null>(null),
    [activationLocationId, setActivationLocationId] = useState(""),
    [blePublicKey, setBlePublicKey] = useState(""),
    [bleAppInstance] = useState(() => `beefiscal-admin-${Date.now()}`),
    [transportMetrics, setTransportMetrics] = useState<Item | null>(null),
    [fiscalMetrics, setFiscalMetrics] = useState<Item | null>(null),
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
  useEffect(() => {
    // Poll telemetry only while a selected device is visible. This avoids
    // background API load and late updates after navigation.
    if (tab !== "Устройства" || !selected) return;
    let active = true;
    const poll = async () => {
      if (
        typeof document !== "undefined" &&
        document.visibilityState === "hidden"
      )
        return;
      try {
        const [health, activity] = await Promise.all([
          call(`/devices/${encodeURIComponent(label(selected.id))}/health`),
          call(`/devices/${encodeURIComponent(label(selected.id))}/activity`),
        ]);
        if (active) {
          setDeviceHealth(health);
          setDeviceActivity(
            Array.isArray(activity.items) ? (activity.items as Item[]) : [],
          );
        }
      } catch {
        if (active) setDeviceHealth({ effective_state: "STALE" });
      }
    };
    void poll();
    const timer = setInterval(() => void poll(), 5000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [tab, selected?.id]);
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
  const refresh = useCallback(
    async (next = tab, registerOverride = register) => {
      setLoading(true);
      setLoadError('');
      if (next === 'Устройства' || next === 'Операции' || next === 'Отчети' || next === 'Одит') setItems([]);
      if (next === 'Устройства') {
        setSelected(null);
        setDeviceHealth(null);
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
        if (next !== "Администриране" && adminLists.registers.length === 0) {
          const [locations, registers, operators] = await Promise.all([
            collect("/locations"),
            collect("/registers"),
            collect("/operators"),
          ]);
          setAdminLists(current => ({ ...current, locations, registers, operators }));
          setLocationId(current => current || label(locations[0]?.id));
          setActivationLocationId(current => current || label(locations[0]?.id));
          setRegister(current => current || label(registers[0]?.id));
          setOperatorId(current => current || label(operators[0]?.id));
        }
        if (next === "Устройства") {
          const v = await collect("/devices");
          setItems(v);
          setSelected((x) => x || v[0] || null);
        }
        if (next === "Операции") {
          const v = await collect(
            registerCollectionPath("/operations", registerOverride),
          );
          setItems(v);
        }
        if (next === "Отчети") {
          const v = await collect(registerCollectionPath("/reports", registerOverride));
          setItems(v);
        }
        if (next === "Одит") {
          const params = new URLSearchParams();
          if (auditActor) params.set("actor_id", auditActor);
          if (auditAction) params.set("action", auditAction);
          if (auditObjectType) params.set("object_type", auditObjectType);
          if (auditObjectId) params.set("object_id", auditObjectId);
          if (auditUnp) params.set("unp", auditUnp);
          if (auditFrom) params.set("from", auditFrom);
          if (auditTo) params.set("to", auditTo);
          setItems([]);
          setItems(
            await collect(`/audit-events${params.size ? `?${params}` : ""}`),
          );
        }
        if (next === "Настройки") {
          const [p, t] = await Promise.all([
            call("/country-policy"),
            call("/tax-groups"),
          ]);
          setPolicy({ ...p, tax_groups: t });
          setItems([]);
        }
        if (next === "Администриране") {
          const [locations, registers, operators, devices] = await Promise.all([
            collect("/locations"),
            collect("/registers"),
            collect("/operators"),
            collect("/devices"),
          ]);
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
              registerResourcePath(selectedRegister, "/composite-bindings"),
            );
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
        setMessage(`${next}: данните са обновени от BeeFiscal API`);
        setCore("CORE READY");
      } catch (e) {
        setCore("CORE OFFLINE");
        setLoadError('Данните не могат да се заредят.');
        setMessage(`API отказ: ${e instanceof Error ? e.message : String(e)}`);
      } finally {
        setLoading(false);
      }
    },
    [
      register,
      adminLists.registers.length,
      tab,
      canAdminister,
      canReadAudit,
      roles.join(","),
      auditActor,
      auditAction,
      auditObjectType,
      auditObjectId,
      auditUnp,
      auditFrom,
      auditTo,
    ],
  );
  useEffect(() => {
    runtimeAuthToken = oidc.accessToken || devAuthToken;
    if (prodMode && !oidc.accessToken) {
      setCore("AUTH REQUIRED");
      setMessage("Влезте с персоналния си административен профил");
      return;
    }
    void refresh("Устройства");
  }, [oidc.accessToken]);
  useEffect(() => {
    if (!tabs.includes(tab)) {
      setTab("Устройства");
      setItems([]);
    }
  }, [roles.join(","), tab]);
  const changeTab = (next: string) => {
    setTab(next);
    setItems([]);
    setLoadError('');
    setReportError('');
    void refresh(next);
  };
  const selectRegisterContext = (nextRegister: string) => {
    if (nextRegister === register) return;
    setRegister(nextRegister);
    setItems([]);
    setSelected(null);
    setDeviceHealth(null);
    setDeviceActivity([]);
    setDiagnostics(null);
    setReportError('');
    void refresh(tab, nextRegister);
  };
  const probe = async () => {
    if (!isRegisterId(register)) {
      setMessage("Изберете валиден Fiscal register UUID преди сквозния probe");
      return;
    }
    if (!selected) {
      setMessage("Няма регистрирано устройство за probe");
      return;
    }
    setMessage("Проверка на cloud route, Edge, driver и крайното ФУ…");
    try {
      const id = label(selected.id);
      const route = await call(
        registerResourcePath(register, "/connectivity-probes"),
        { method: "POST", body: "{}" },
      );
      const device = await call(`/devices/${encodeURIComponent(id)}/readiness`);
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
      setMessage(
        `Probe отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    }
  };
  const loadDiagnostics = async () => {
    if (!selected || !canDiagnose) {
      setMessage("Диагностиката изисква роля ADMIN или SERVICE");
      return;
    }
    setLoading(true);
    try {
      const id = encodeURIComponent(label(selected.id));
      const [capabilities, details] = await Promise.all([
        call(`/devices/${id}/capabilities`),
        call(`/devices/${id}/diagnostics`),
      ]);
      setDiagnostics({ capabilities, details });
      setMessage("Диагностиката е заредена с приложени server redactions");
    } catch (e) {
      setMessage(
        `Диагностиката е отказана: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const printerTest = async () => {
    if (!selected || !canDiagnose) {
      setMessage("Тестът на принтера изисква ADMIN или SERVICE");
      return;
    }
    setLoading(true);
    try {
      const result = await call(
        `/devices/${encodeURIComponent(label(selected.id))}/tests/printer`,
        { method: "POST", body: "{}" },
      );
      setMessage(
        `Тест на принтера: ${label(result.state)} • ${label(result.id)}`,
      );
    } catch (e) {
      setMessage(
        `Тестът на принтера е отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const startProvisioning = async () => {
    if (!selected || !canAdminister) {
      setMessage("Provisioning изисква роля ADMIN");
      return;
    }
    setLoading(true);
    try {
      const result = await call(
        `/devices/${encodeURIComponent(label(selected.id))}/provisioning-sessions`,
        { method: "POST", body: "" },
      );
      setProvisioning(result);
      setMessage(
        `Provisioning сесия ${label(result.session_id)} • ${label(result.state)}`,
      );
    } catch (e) {
      setMessage(
        `Provisioning е отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const issueBleSession = async () => {
    if (!canIssueBleSession) {
      setMessage("BLE сесията изисква оперативна роля");
      return;
    }
    if (!isRegisterId(register) || !operatorId || !blePublicKey.trim()) {
      setMessage(
        "BLE сесията изисква валиден register UUID, operator и предварително подготвен client public key",
      );
      return;
    }
    setLoading(true);
    try {
      const result = await call(
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
      setBleSession(result);
      setMessage(
        `BLE сесия ${label(result.ble_session_id)} • валидна до ${label(result.expires_at)}`,
      );
    } catch (e) {
      setMessage(
        `BLE сесията е отказана: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const activateBlueCash = async () => {
    if (!activationRequest || !canAdminister) {
      setMessage("Първо намерете чакащата заявка с QR/краткия код");
      return;
    }
    if (!isRegisterId(activationLocationId) || !isRegisterId(register)) {
      setMessage("Изберете валидни търговска точка и касово място");
      return;
    }
    setLoading(true);
    try {
      const confirmed = await call(
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
      setBlueCashActivation(confirmed);
      setActivationRequest(null);
      setMessage(
        `Заявката е потвърдена. Организацията е взета от вашия tenant token; устройството ще завърши key-bound activation.`,
      );
    } catch (e) {
      setMessage(
        `BlueCash activation е отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const lookupActivation = async () => {
    if (!canAdminister || !activationCode.trim()) return;
    setLoading(true);
    try {
      const pending = await call(
        `/device-activation-requests:lookup?user_code=${encodeURIComponent(activationCode.trim().toUpperCase())}`,
      );
      setActivationRequest(pending);
      setMessage(
        "Проверете vendor, model, serial, FMIN и key thumbprint преди потвърждение",
      );
    } catch (e) {
      setActivationRequest(null);
      setMessage(
        `Заявката не е намерена: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const disconnectDevice = async () => {
    if (!selected || !canAdminister) return;
    setLoading(true);
    try {
      await call(
        `/devices/${encodeURIComponent(label(selected.id))}:disconnect`,
        { method: "POST", body: "{}" },
      );
      setMessage(
        "Устройството, credential-ът, BLE сесиите и register bindings са оттеглени",
      );
      await refresh("Устройства");
    } catch (e) {
      setMessage(
        `Отключването е отказано: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const report = async (type: string) => {
    if (reportPending.current) return;
    if (!canRunReports) {
      setMessage("Командата за отчет изисква роля SUPERVISOR или ADMIN");
      return;
    }
    if (!isRegisterId(register)) {
      setMessage("Командата за отчет изисква валиден Fiscal register UUID");
      return;
    }
    reportPending.current = true;
    setLoading(true);
    setReportError('');
    setMessage(`${type} отчет…`);
    try {
      const v = await call(registerResourcePath(register, "/reports"), {
        method: "POST",
        body: JSON.stringify({ type }),
      });
      setMessage(`${type}: ${v.state} • ${v.fiscal_reference || v.id}`);
      await refresh("Отчети");
    } catch (e) {
      setReportError('Създаването на отчета не успя. Опитайте отново.');
      setMessage(
        `Отчетът е отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      reportPending.current = false;
      setLoading(false);
    }
  };
  const reconcileOperation = async (operation: Item) => {
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
    setLoading(true);
    setMessage(`Reconciliation ${label(operation.id)}…`);
    try {
      const result = await call(
        `/operations/${encodeURIComponent(label(operation.id))}/reconcile`,
        { method: "POST", body: "{}" },
      );
      setMessage(
        `Reconciliation: ${label(result.state)} • ${label(result.operation_id || result.id)}`,
      );
      await refresh("Операции");
    } catch (e) {
      setMessage(
        `Reconciliation е отказано: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setLoading(false);
    }
  };
  const mutateAdmin = async (
    description: string,
    action: () => Promise<unknown>,
  ) => {
    setAdminBusy(true);
    setMessage(`${description}…`);
    try {
      await action();
      setMessage(`${description}: готово`);
      await refresh("Администриране");
    } catch (e) {
      setMessage(
        `${description}: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setAdminBusy(false);
    }
  };
  const stageCompositeBinding = async () => {
    setAdminBusy(true);
    try {
      const v = await call(
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
      setCompositeBinding(v);
      setMessage(`Composite generation ${label(v.generation)} е подготвена`);
    } catch (e) {
      setMessage(
        `Composite binding: ${e instanceof Error ? e.message : String(e)}`,
      );
    } finally {
      setAdminBusy(false);
    }
  };
  const disableCompositeBinding = async () => {
    if (!compositeBinding) return;
    setAdminBusy(true);
    try {
      const v = await call(
        registerResourcePath(register, "/composite-bindings/disable"),
        {
          method: "POST",
          body: JSON.stringify({
            binding_id: compositeBinding.binding_id,
            expected_version: compositeBinding.version,
          }),
        },
      );
      setCompositeBinding(v);
      setExpectedRegisterVersion(String(Number(expectedRegisterVersion) + 1));
      setMessage(
        "Composite binding е изключен; може да бъде подготвена нова връзка",
      );
    } catch (e) {
      setMessage(`Disable: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setAdminBusy(false);
    }
  };
  if (prodMode && !oidc.accessToken) {
    return <AuthScreen auth={oidc} />;
  }
  return (
    <SafeAreaView style={[s.root, { backgroundColor: theme.colors.background }]}>
      <StatusBar style={theme.dark ? 'light' : 'dark'} />
      <LanguageThemeControls testPrefix="shell" />
      <Portal>
        <Modal visible={compact && navOpen} onDismiss={() => setNavOpen(false)} contentContainerStyle={{ margin: 16, maxHeight: '80%', borderRadius: 16, padding: 12, backgroundColor: theme.colors.surface }}>
          <ScrollView>
            <Drawer.Section title="BeeFiscal">
              {tabs.map(x => <Drawer.Item key={x} testID={`tab-${x}`} label={x} active={tab === x} accessibilityLabel={`Отвори ${x}`} onPress={() => { setNavOpen(false); changeTab(x); }} />)}
            </Drawer.Section>
          </ScrollView>
        </Modal>
      </Portal>
      <View style={[s.header, { backgroundColor: theme.colors.surface, borderColor: theme.colors.outlineVariant }, compact && { flexWrap: 'wrap', gap: 8, padding: 12 }]}>
        <View style={compact ? { minWidth: 0, width: '100%' } : undefined}>
          <PaperText style={[s.title, { color: theme.colors.onSurface }]}>BeeFiscal</PaperText>
          <PaperText style={[s.sub, { color: theme.colors.onSurfaceVariant }]}>
            BG • EUR • публичен API • без локално симулирани записи
          </PaperText>
        </View>
        {compact ? <Button testID="nav.open" mode="outlined" contentStyle={{ minHeight: 48 }} onPress={() => setNavOpen(true)}>Меню</Button> : null}
        <View
          testID="status-fiscal-device"
          accessibilityLiveRegion="polite"
          style={[s.ready, core.includes("OFFLINE") && s.offline]}
        >
          <PaperText style={s.readyText}>Fiscal Core: {core}</PaperText>
        </View>
        {prodMode && oidc.tenants.length > 1 ? <View style={s.actions}>{oidc.tenants.map(t=><Button key={t.tenant_id} mode={t.tenant_id===oidc.currentTenant?.tenant_id ? 'contained-tonal' : 'outlined'} contentStyle={{ minHeight: 48 }} disabled={t.tenant_id===oidc.currentTenant?.tenant_id} accessibilityLabel={`Компания ${t.display_name}`} onPress={()=>{setItems([]);setSelected(null);setAdminLists({locations:[],registers:[],operators:[],devices:[]});void oidc.switchTenant(t.tenant_id)}}>{t.display_name}</Button>)}</View>:null}
        {prodMode ? (
          <Button
            testID="admin-logout"
            mode="outlined"
            contentStyle={{ minHeight: 48 }}
            accessibilityLabel="Излез от административния профил"
            onPress={() => {
              runtimeAuthToken = "";
              void oidc.logout();
            }}
          >Излез</Button>
        ) : null}
      </View>
      <View style={s.body}>
        {!compact ? <View style={[s.nav, { backgroundColor: theme.colors.surface }]}>
          <Drawer.Section>
            {tabs.map(x => <Drawer.Item key={x} testID={`tab-${x}`} label={x} active={tab === x} accessibilityLabel={`Отвори ${x}`} onPress={() => changeTab(x)} />)}
          </Drawer.Section>
        </View> : null}
        <ScrollView style={[s.content, { backgroundColor: theme.colors.background }, compact && { padding: 16 }]}>
          {loading ? (
            <PaperText
              testID="screen-loading"
              accessibilityLiveRegion="polite"
              style={[s.loadingText, { color: theme.colors.onSurfaceVariant, backgroundColor: theme.colors.surfaceVariant }]}
            >
              Зареждане от BeeFiscal API…
            </PaperText>
          ) : null}
          <View style={[s.toolbar, compact && { flexWrap: 'wrap' }]}>
            <PaperText style={[s.section, { color: theme.colors.onBackground }, compact && { flexBasis: '100%' }]}>{tab}</PaperText>
            <ReferencePicker label="Касово място" items={adminLists.registers} value={register} labelFor={(x)=>label(x.code || x.name)} onSelect={selectRegisterContext} onEmpty={()=>changeTab("Администриране")}/>
            <Button mode="contained-tonal" contentStyle={{ minHeight: 48 }} accessibilityLabel={`Обнови ${tab}`} disabled={loading} onPress={() => refresh()}>{loading ? "…" : "Обнови"}</Button>
          </View>
          {tab === "Устройства" ? (
            <DevicesScreen
              devices={items} selected={selected} loading={loading} error={loadError}
              canDiagnose={canDiagnose} canAdminister={canAdminister} canIssueBleSession={canIssueBleSession}
              lists={adminLists}
              context={{ register, operatorId, blePublicKey, activationCode, activationRequest, activationLocationId }}
              telemetry={{ transportMetrics, deviceHealth, deviceActivity, fiscalMetrics, diagnostics, provisioning, bleSession, blueCashActivation }}
              actions={{
                retry: () => void refresh('Устройства'),
                selectDevice: device => { setSelected(device); setMessage(`${label(device.name || device.id)} е избрано`); },
                openAdministration: () => changeTab('Администриране'),
                diagnostics: () => void loadDiagnostics(),
                printerTest: () => void printerTest(),
                provisioning: () => void startProvisioning(),
                selectOperator: setOperatorId,
                changeBlePublicKey: setBlePublicKey,
                issueBleSession: () => void issueBleSession(),
                changeActivationCode: setActivationCode,
                lookupActivation: () => void lookupActivation(),
                selectActivationLocation: setActivationLocationId,
                selectRegister: selectRegisterContext,
                confirmActivation: () => void activateBlueCash(),
                disconnectDevice: () => void disconnectDevice(),
              }}
            />
          ) : tab === "Операции" ? (
            <OperationsScreen operations={items} loading={loading} canReconcile={canReconcile} onReconcile={(operation) => void reconcileOperation(operation)} />
          ) : tab === "Отчети" ? (
            <ReportsScreen reports={items} loading={loading} canRunReports={canRunReports} registerValid={isRegisterId(register)} loadError={loadError} commandError={reportError} onReport={(type) => void report(type)} onRetry={() => void refresh('Отчети')} />
          ) : tab === "Одит" && canReadAudit ? (
            <AuditScreen events={items} loading={loading} error={loadError}
              filters={{ actor: auditActor, action: auditAction, objectType: auditObjectType, objectId: auditObjectId, unp: auditUnp, from: auditFrom, to: auditTo }}
              onChange={(name: keyof AuditFilters, value: string) => {
                const setters = { actor: setAuditActor, action: setAuditAction, objectType: setAuditObjectType, objectId: setAuditObjectId, unp: setAuditUnp, from: setAuditFrom, to: setAuditTo };
                setters[name](value);
              }} onApply={() => void refresh('Одит')} />
          ) : tab === "Администриране" && canAdminister ? (
            <View testID="admin-editors" style={s.adminGrid}>
              <AdminCard
                title="Точка на продажба"
                count={adminLists.locations.length}
              >
                <Choices
                  items={adminLists.locations}
                  selectedId={locationId}
                  labelFor={(x) => `${label(x.code)} • ${label(x.name)}`}
                  onSelect={(x) => {
                    setLocationId(label(x.id));
                    setLocationCode(label(x.code));
                    setLocationName(label(x.name));
                    setLocationAddress(label(x.address));
                  }}
                />
                <Field
                  label="Код на точката"
                  value={locationCode}
                  onChangeText={setLocationCode}
                  testID="admin-location-code"
                />
                <Field
                  label="Име на точката"
                  value={locationName}
                  onChangeText={setLocationName}
                />
                <Field
                  label="Адрес"
                  value={locationAddress}
                  onChangeText={setLocationAddress}
                />
                <Action
                  label="Запази точка"
                  disabled={adminBusy}
                  testID="admin-location-save"
                  onPress={() =>
                    mutateAdmin("Запис на точка", async () => {
                      const current = adminLists.locations.find(
                        (x) => x.id === locationId,
                      );
                      const v = await call(
                        current
                          ? `/locations/${encodeURIComponent(locationId)}`
                          : "/locations",
                        {
                          method: current ? "PATCH" : "POST",
                          headers: current
                            ? { "If-Match": String(current.version) }
                            : {},
                          body: JSON.stringify({
                            code: locationCode,
                            name: locationName,
                            address: locationAddress,
                            status: "ACTIVE",
                          }),
                        },
                      );
                      setLocationId(label(v.id));
                    })
                  }
                />
              </AdminCard>
              <AdminCard
                title="Касово място"
                count={adminLists.registers.length}
              >
                <Choices
                  items={adminLists.registers}
                  selectedId={register}
                  labelFor={(x) => label(x.code)}
                  onSelect={(x) => {
                    setRegister(label(x.id));
                    setRegisterCode(label(x.code));
                    setLocationId(label(x.location_id));
                  }}
                />
                <ReferencePicker label="Търговска точка" items={adminLists.locations} value={locationId} labelFor={(x)=>label(x.name || x.code)} onSelect={setLocationId} onEmpty={()=>setMessage("Първо създайте търговска точка в картата по-горе.")}/>
                <Field
                  label="Код на касата"
                  value={registerCode}
                  onChangeText={setRegisterCode}
                />
                <Action
                  label="Запази каса"
                  disabled={adminBusy}
                  testID="admin-register-save"
                  onPress={() =>
                    mutateAdmin("Запис на каса", async () => {
                      const current = adminLists.registers.find(
                        (x) => x.id === register,
                      );
                      const v = await call(
                        current ? registerResourcePath(register) : "/registers",
                        {
                          method: current ? "PATCH" : "POST",
                          headers: current
                            ? { "If-Match": String(current.version) }
                            : {},
                          body: JSON.stringify({
                            location_id: locationId,
                            code: registerCode,
                            status: "ACTIVE",
                            fiscal_device_id: current?.fiscal_device_id,
                            payment_terminal_id: current?.payment_terminal_id,
                          }),
                        },
                      );
                      setRegister(label(v.id));
                    })
                  }
                />
              </AdminCard>
              <AdminCard
                title="Фискален оператор"
                count={adminLists.operators.length}
              >
                <Choices
                  items={adminLists.operators}
                  selectedId={operatorId}
                  labelFor={(x) =>
                    `${label(x.code)} • ${label(x.first_name)} ${label(x.last_name)}`
                  }
                  onSelect={(x) => {
                    setOperatorId(label(x.id));
                    setOperatorCode(label(x.code));
                    setOperatorFirstName(label(x.first_name));
                    setOperatorLastName(label(x.last_name));
                  }}
                />
                <Field
                  label="Код (4 символа)"
                  value={operatorCode}
                  onChangeText={setOperatorCode}
                  testID="admin-operator-code"
                />
                <Field
                  label="Име"
                  value={operatorFirstName}
                  onChangeText={setOperatorFirstName}
                />
                <Field
                  label="Фамилия"
                  value={operatorLastName}
                  onChangeText={setOperatorLastName}
                />
                <Action
                  label="Запази оператор"
                  disabled={adminBusy}
                  testID="admin-operator-save"
                  onPress={() =>
                    mutateAdmin("Запис на оператор", async () => {
                      const current = adminLists.operators.find(
                        (x) => x.id === operatorId,
                      );
                      const v = await call(
                        current
                          ? `/operators/${encodeURIComponent(operatorId)}`
                          : "/operators",
                        {
                          method: current ? "PATCH" : "POST",
                          headers: current
                            ? { "If-Match": String(current.version) }
                            : {},
                          body: JSON.stringify({
                            code: operatorCode,
                            first_name: operatorFirstName,
                            last_name: operatorLastName,
                            roles: current?.roles || ["CASHIER"],
                            active_from:
                              current?.active_from || new Date().toISOString(),
                            active_to: current?.active_to || null,
                          }),
                        },
                      );
                      setOperatorId(label(v.id));
                    })
                  }
                />
              </AdminCard>
              <AdminCard
                title="Фискално устройство"
                count={adminLists.devices.length}
              >
                <Choices
                  items={adminLists.devices}
                  selectedId={deviceId}
                  labelFor={(x) =>
                    `${label(x.vendor)} ${label(x.model)} • ${label(x.serial)}`
                  }
                  onSelect={(x) => {
                    setDeviceId(label(x.id));
                    setDeviceVendor(label(x.vendor));
                    setDeviceModel(label(x.model));
                    setDeviceSerial(label(x.serial));
                    setDeviceKind(label(x.kind));
                    setDeviceDriverId(label(x.driver_id || "datecs"));
                    setDeviceProtocolId(label(x.protocol_id || "datecs-fp"));
                    setDeviceProtocolVersion(
                      label(x.protocol_version || "2.11.4"),
                    );
                    setDeviceTransport(label(x.transport || "RS232"));
                    setDeviceTransportParameters(
                      JSON.stringify(x.transport_parameters || {}, null, 2),
                    );
                    setBindingRole(
                      label(x.kind) === "PAYMENT_TERMINAL"
                        ? "OPTIONAL_PAYMENT_TERMINAL"
                        : "FISCAL_DEVICE",
                    );
                  }}
                />
                <View style={s.actions}>
                  <Action
                    label="Фискално устройство"
                    disabled={adminBusy}
                    testID="admin-device-kind-fiscal"
                    onPress={() => {
                      setDeviceKind("FISCAL_DEVICE");
                      setBindingRole("FISCAL_DEVICE");
                    }}
                  />
                  <Action
                    label="Опционален POS терминал"
                    disabled={adminBusy}
                    testID="admin-device-kind-terminal"
                    onPress={() => {
                      setDeviceKind("PAYMENT_TERMINAL");
                      setBindingRole("OPTIONAL_PAYMENT_TERMINAL");
                    }}
                  />
                </View>
                <PaperText variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>
                  Capability: {deviceKind} • binding: {bindingRole}
                </PaperText>
                <Field
                  label="Производител"
                  value={deviceVendor}
                  onChangeText={setDeviceVendor}
                />
                <Field
                  label="Модел"
                  value={deviceModel}
                  onChangeText={setDeviceModel}
                />
                <Field
                  label="Сериен номер"
                  value={deviceSerial}
                  onChangeText={setDeviceSerial}
                  testID="admin-device-serial"
                />
                <Field
                  label="Driver ID"
                  value={deviceDriverId}
                  onChangeText={setDeviceDriverId}
                  testID="admin-device-driver"
                />
                <Field
                  label="Protocol ID"
                  value={deviceProtocolId}
                  onChangeText={setDeviceProtocolId}
                  testID="admin-device-protocol"
                />
                <Field
                  label="Protocol version"
                  value={deviceProtocolVersion}
                  onChangeText={setDeviceProtocolVersion}
                  testID="admin-device-protocol-version"
                />
                <View style={s.actions}>
                  <Action
                    testID="transport-rs232"
                    label="RS-232"
                    disabled={adminBusy}
                    onPress={() => {
                      setDeviceTransport("RS232");
                      setDeviceTransportParameters(
                        '{"baud":115200,"data_bits":8,"parity":"N","stop_bits":1,"tx_pin":17,"rx_pin":18}',
                      );
                    }}
                  />
                  <Action
                    testID="transport-usb"
                    label="USB CDC"
                    disabled={adminBusy}
                    onPress={() => {
                      setDeviceTransport("USB_SERIAL");
                      setDeviceTransportParameters(
                        '{"vid":0,"pid":0,"interface":0}',
                      );
                    }}
                  />
                  <Action
                    testID="transport-ble"
                    label="BLE GATT"
                    disabled={adminBusy}
                    onPress={() => {
                      setDeviceTransport("BLE_GATT");
                      setDeviceTransportParameters(
                        '{"address_policy":"ADVERTISING_IDENTITY"}',
                      );
                    }}
                  />
                </View>
                <PaperText variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>Избран transport: {deviceTransport}</PaperText>
                {deviceTransport === "RS232" ? (
                  <>
                    <Field
                      label="Baud"
                      value={transportParameter("baud", "115200")}
                      onChangeText={(v) =>
                        setTransportParameter("baud", v, true)
                      }
                    />
                    <Field
                      label="Data bits"
                      value={transportParameter("data_bits", "8")}
                      onChangeText={(v) =>
                        setTransportParameter("data_bits", v, true)
                      }
                    />
                    <Field
                      label="Parity (N/E/O)"
                      value={transportParameter("parity", "N")}
                      onChangeText={(v) => setTransportParameter("parity", v)}
                    />
                    <Field
                      label="Stop bits"
                      value={transportParameter("stop_bits", "1")}
                      onChangeText={(v) =>
                        setTransportParameter("stop_bits", v, true)
                      }
                    />
                    <Field
                      label="TX pin"
                      value={transportParameter("tx_pin", "17")}
                      onChangeText={(v) =>
                        setTransportParameter("tx_pin", v, true)
                      }
                    />
                    <Field
                      label="RX pin"
                      value={transportParameter("rx_pin", "18")}
                      onChangeText={(v) =>
                        setTransportParameter("rx_pin", v, true)
                      }
                    />
                  </>
                ) : null}
                {deviceTransport === "USB_SERIAL" ? (
                  <>
                    <Field
                      label="USB VID"
                      value={transportParameter("vid", "0")}
                      onChangeText={(v) =>
                        setTransportParameter("vid", v, true)
                      }
                    />
                    <Field
                      label="USB PID"
                      value={transportParameter("pid", "0")}
                      onChangeText={(v) =>
                        setTransportParameter("pid", v, true)
                      }
                    />
                    <Field
                      label="CDC interface"
                      value={transportParameter("interface", "0")}
                      onChangeText={(v) =>
                        setTransportParameter("interface", v, true)
                      }
                    />
                    <Field
                      label="Очакван USB serial"
                      value={transportParameter("serial", "")}
                      onChangeText={(v) => setTransportParameter("serial", v)}
                    />
                  </>
                ) : null}
                {deviceTransport === "BLE_GATT" ? (
                  <>
                    <Field
                      label="Advertising identity"
                      value={transportParameter("advertising_identity", "")}
                      onChangeText={(v) =>
                        setTransportParameter("advertising_identity", v)
                      }
                    />
                    <Field
                      label="Service UUID"
                      value={transportParameter("service_uuid", "")}
                      onChangeText={(v) =>
                        setTransportParameter("service_uuid", v)
                      }
                    />
                    <Field
                      label="TX characteristic UUID"
                      value={transportParameter("tx_characteristic_uuid", "")}
                      onChangeText={(v) =>
                        setTransportParameter("tx_characteristic_uuid", v)
                      }
                    />
                    <Field
                      label="RX characteristic UUID"
                      value={transportParameter("rx_characteristic_uuid", "")}
                      onChangeText={(v) =>
                        setTransportParameter("rx_characteristic_uuid", v)
                      }
                    />
                  </>
                ) : null}
                <Field
                  label="Advanced transport JSON (read-only contract preview)"
                  value={deviceTransportParameters}
                  onChangeText={setDeviceTransportParameters}
                  testID="admin-device-transport-parameters"
                />
                <Action
                  label="Запази DEV ФУ"
                  disabled={adminBusy}
                  testID="admin-device-save"
                  onPress={() =>
                    mutateAdmin("Запис на ФУ", async () => {
                      const current = adminLists.devices.find(
                        (x) => x.id === deviceId,
                      );
                      const v = await call(
                        current
                          ? `/devices/${encodeURIComponent(deviceId)}`
                          : "/devices",
                        {
                          method: current ? "PATCH" : "POST",
                          headers: current
                            ? { "If-Match": String(current.version) }
                            : {},
                          body: JSON.stringify({
                            kind: deviceKind,
                            vendor: deviceVendor,
                            model: deviceModel,
                            serial: deviceSerial,
                            driver_id: deviceDriverId,
                            protocol_id: deviceProtocolId,
                            protocol_version: deviceProtocolVersion,
                            transport: deviceTransport,
                            transport_parameters: JSON.parse(
                              deviceTransportParameters || "{}",
                            ),
                            status: current?.status || "DRAFT",
                            environment: current?.environment || "DEV",
                            simulated: current?.simulated ?? true,
                            approved_type_evidence_id:
                              current?.approved_type_evidence_id,
                            service_contract_evidence_id:
                              current?.service_contract_evidence_id,
                          }),
                        },
                      );
                      setDeviceId(label(v.id));
                    })
                  }
                />
                {(() => {
                  const current = adminLists.devices.find(
                    (x) => x.id === deviceId,
                  );
                  const target =
                    current?.status === "DRAFT"
                      ? "PENDING_SERVICE_ACTIVATION"
                      : current?.status === "PENDING_SERVICE_ACTIVATION"
                        ? "ACTIVE"
                        : "";
                  if (!current || !target) return null;
                  return (
                    <Action
                      label={
                        target === "ACTIVE"
                          ? "Активирай ФУ"
                          : "Предай към сервизна активация"
                      }
                      disabled={adminBusy}
                      testID="admin-device-activate"
                      onPress={() =>
                        mutateAdmin("Промяна на lifecycle", () =>
                          call(`/devices/${encodeURIComponent(deviceId)}`, {
                            method: "PATCH",
                            headers: { "If-Match": String(current.version) },
                            body: JSON.stringify({
                              ...current,
                              status: target,
                              id: undefined,
                              version: undefined,
                              created_at: undefined,
                              updated_at: undefined,
                            }),
                          }),
                        )
                      }
                    />
                  );
                })()}
              </AdminCard>
              <AdminCard title="Привързване към касата" count={0}>
                <ReferencePicker label="Касово място" items={adminLists.registers} value={register} labelFor={(x)=>label(x.code || x.name)} onSelect={setRegister} onEmpty={()=>setMessage("Първо създайте касово място.")}/>
                <View style={s.actions}>
                  <Action
                    label="Роля ФУ"
                    disabled={adminBusy}
                    testID="admin-binding-role-fiscal"
                    onPress={() => setBindingRole("FISCAL_DEVICE")}
                  />
                  <Action
                    label="Роля POS"
                    disabled={adminBusy}
                    testID="admin-binding-role-terminal"
                    onPress={() => setBindingRole("OPTIONAL_PAYMENT_TERMINAL")}
                  />
                </View>
                <ReferencePicker label="Устройство" items={adminLists.devices} value={deviceId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setDeviceId} onEmpty={()=>setMessage("Първо създайте устройство.")}/>
                <Action
                  label="Привържи ФУ"
                  disabled={adminBusy || !isRegisterId(register)}
                  testID="admin-binding-save"
                  onPress={() =>
                    mutateAdmin("Привързване на ФУ", () =>
                      call(registerResourcePath(register, "/bindings"), {
                        method: "POST",
                        body: JSON.stringify({
                          device_id: deviceId,
                          role: bindingRole,
                        }),
                      }),
                    )
                  }
                />
                <PaperText variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>
                  Composite profile — активира се само след apply ACK от
                  адаптера
                </PaperText>
                <View style={s.actions}>
                  {MVP1_DEVICE_PROFILES.map((profile) => (
                    <Action
                      key={profile.id}
                      label={profile.id}
                      disabled={adminBusy}
                      testID={`composite-profile-${profile.id}`}
                      onPress={() => setCompositeProfile(profile.id)}
                    />
                  ))}
                </View>
                <ReferencePicker label="Адаптер" items={adminLists.devices} value={adapterDeviceId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setAdapterDeviceId} onEmpty={()=>setMessage("Първо създайте устройство-адаптер.")}/>
                <ReferencePicker label="Фискално устройство" items={adminLists.devices.filter((x)=>x.kind === "FISCAL_DEVICE")} value={fiscalEndpointId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setFiscalEndpointId} onEmpty={()=>setMessage("Първо създайте фискално устройство.")}/>
                <ReferencePicker label="Платежен терминал (незадължително)" items={adminLists.devices.filter((x)=>x.kind === "PAYMENT_TERMINAL")} value={paymentEndpointId} labelFor={(x)=>`${label(x.vendor)} ${label(x.model)} · ${label(x.serial)}`} onSelect={setPaymentEndpointId} required={false}/>
                <Field
                  label="Expected register version"
                  value={expectedRegisterVersion}
                  onChangeText={setExpectedRegisterVersion}
                  testID="composite-register-version"
                />
                <Action
                  label="Подготви composite binding"
                  disabled={adminBusy || !isRegisterId(register)}
                  testID="composite-binding-save"
                  onPress={stageCompositeBinding}
                />
                {compositeBinding ? (
                  <>
                    <PaperText testID="composite-binding-state" variant="bodyMedium" style={{ color: theme.colors.onSurfaceVariant }}>
                      Статус: {label(compositeBinding.status)} • generation{" "}
                      {label(compositeBinding.generation)} • applied generation{" "}
                      {label(compositeBinding.applied_generation || "—")}
                    </PaperText>
                    {compositeBinding.status === "PENDING" ? (
                      <PaperText
                        testID="composite-binding-awaiting-adapter"
                        variant="bodyMedium"
                        style={{ color: theme.colors.onSurfaceVariant }}
                      >
                        Очаква се подписано apply потвърждение от адаптера
                      </PaperText>
                    ) : null}
                    <View style={s.actions}>
                      <Action
                        label="Изключи / rebind"
                        disabled={
                          adminBusy || compositeBinding.status === "REVOKED"
                        }
                        testID="composite-binding-disable"
                        onPress={disableCompositeBinding}
                      />
                    </View>
                  </>
                ) : null}
              </AdminCard>
            </View>
          ) : (
            <SettingsScreen policy={policy} loading={loading} error={loadError} onRetry={() => void refresh('Настройки')} />
          )}
          {items.length === 0 &&
          tab !== "Настройки" &&
          tab !== "Устройства" &&
          tab !== "Одит" &&
          tab !== "Отчети" &&
          tab !== "Администриране" ? (
            <PaperText variant="bodyMedium" style={{ paddingVertical: 32, textAlign: 'center' }}>Няма записи в API за избрания филтър.</PaperText>
          ) : null}
        </ScrollView>
      </View>
      <View
        testID="status-transport"
        accessibilityLiveRegion="polite"
        style={[s.footer, { backgroundColor: theme.colors.surface, borderColor: theme.colors.outlineVariant }, compact && { flexWrap: 'wrap', gap: 8 }]}
      >
        <PaperText style={[s.footerText, { color: theme.colors.onSurface }]}>{message}</PaperText>
        <Button
          testID="readiness-probe"
          mode="contained"
          contentStyle={{ minHeight: 48 }}
          accessibilityLabel="Провери cloud, Edge, driver и крайното фискално устройство"
          onPress={probe}
        >Сквозен probe</Button>
      </View>
    </SafeAreaView>
  );
}

const s = StyleSheet.create({
  root: { flex: 1, backgroundColor: beeloyColors.background },
  logout: {
    minHeight: 48,
    paddingHorizontal: 14,
    backgroundColor: beeloyColors.tertiary,
    borderRadius: 9,
    justifyContent: "center",
  },
  header: {
    padding: 20,
    backgroundColor: beeloyColors.surface,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    borderBottomWidth: 1,
    borderColor: beeloyColors.tertiaryContainer,
  },
  title: { fontSize: 29, fontWeight: "900", color: beeloyColors.tertiary },
  sub: { color: beeloyColors.tertiary, marginTop: 3 },
  ready: {
    backgroundColor: beeloyColors.successContainer,
    padding: 10,
    borderRadius: 20,
    minHeight: 48,
    justifyContent: "center",
  },
  offline: { backgroundColor: beeloyColors.errorContainer },
  readyText: { color: beeloyColors.success, fontWeight: "900" },
  body: { flex: 1, flexDirection: "row" },
  nav: { width: 190, padding: 12, gap: 6 },
  content: { flex: 1, padding: 20 },
  toolbar: {
    flexDirection: "row",
    gap: 10,
    alignItems: "center",
    marginBottom: 16,
  },
  section: { fontSize: 25, fontWeight: "900", flex: 1 },
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
  adminGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 14,
    paddingBottom: 24,
  },
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
