import { colors as beeloyColors } from './src/ui/tokens';
import { SafeAreaView } from 'react-native-safe-area-context';
import { ResponsiveScrollView as ScrollView } from './src/ui/responsive-scroll-view';
import { FormField as TextInput } from './src/ui/form-field';
import { AuthForm } from './src/ui/auth-form';
import { StatusBar } from "expo-status-bar";
import { useEffect, useMemo, useRef, useState } from "react";
import { Image, Linking, Platform, ScrollView as NativeScrollView, StyleSheet, useWindowDimensions, View } from "react-native";
import {
  Button as PaperButton,
  Card as PaperCard,
  Checkbox as PaperCheckbox,
  HelperText as PaperHelperText,
  IconButton as PaperIconButton,
  List as PaperList,
  Menu as PaperMenu,
  SegmentedButtons as PaperSegmentedButtons,
  Surface as PaperSurface,
  Text as PaperText,
  useTheme,
} from "react-native-paper";
import { WebBleBootstrap, webBluetoothSupported } from "./src/webBle.ts";
import type { BleSessionPackage } from "./src/webBle.ts";
import { validateBleDeploymentAuthority } from "./src/bleDeployment.ts";
import { getRandomBytes } from "expo-crypto";
import { configureRandomSource } from "./src/portableCrypto.ts";
import { ConnectivityController, pingFiscal } from "./src/connectivity.ts";
import { bleFinalizeIntent, newCheckoutPlan } from "./src/checkoutPlan.ts";
import { CheckoutJournal } from "./src/checkoutJournal.ts";
import {
  createNativeBleBootstrap,
  requestNativeBlePermissions,
} from "./src/nativeBle";
import type { NativeBleBootstrapContract } from "./src/nativeBle";
import { useEmailAuth } from "./src/emailAuth";
import type { Language } from "./src/emailAuth";
import { supportedLanguages } from "./src/translations.generated";
import { legalTranslations } from "./src/legalTranslations.generated";
import { useTranslation } from "./src/languageStore";
import type { Translation } from "./src/languageStore";
import { fetchWithTimeout } from "./src/http";
import { collectCursorPages } from "./src/pagination";
import {
  isFiscalResourceId,
  requireFiscalResourceId,
} from "./src/fiscalIdentity";
import {
  FiscalRegisterResource,
  resolveFiscalDeviceId,
} from "./src/fiscalDeviceResolution";
import Constants from "expo-constants";
import { BeeloyAppProvider, useThemePreference, type ThemePreference } from "./src/ui/provider";
configureRandomSource(getRandomBytes);

// BeeMiniPOS is intentionally thin: it owns cart presentation and surrogate
// client UUIDs, while Fiscal owns UNP, validation, routing and receipt truth.

type Money = { amount: string; currency: "EUR" };
type Product = {
  id: string;
  sku: string;
  barcode?: string | null;
  name: string;
  price: Money;
  tax_group: string;
  active: boolean;
};
type TaxGroup = {
  id: string;
  code: string;
  name: string;
  rate: string;
  status: "ACTIVE" | "INACTIVE";
  version: number;
};
type Employee = {
  id: string;
  first_name: string;
  last_name: string;
  operator_code: string;
  roles?: string[];
};
type CartLine = { product: Product; quantity: number; discountAmount?: string };
type RegulatoryIdentifier = {
  type: string;
  scheme: string;
  value: string;
  country_code: string;
  profile_version: string;
};
type FiscalSaleLine = {
  line_id: string;
  product_code?: string;
  name: string;
  quantity: string;
  unit_price: Money;
  discount?: Money;
  tax_group: string;
};
type FiscalSale = {
  sale_id: string;
  external_id: string;
  register_id: string;
  operator_id: string;
  unp: string;
  regulatory_identifiers: RegulatoryIdentifier[];
  state: string;
  version: number;
  lines: FiscalSaleLine[];
  payments: unknown[];
  allowed_actions: string[];
  totals: { gross: Money };
  fiscal_operation_id?: string;
};
type WorkstationSession = {
  session_id: string;
  workstation_id: string;
  operator_code: string;
  expires_at: string;
};
type ActiveBleBinding = {
  tenant_id: string;
  location_id: string;
  register_id: string;
  edge_id: string;
  device_id: string;
  binding_version: number;
  expires_at: string;
};
type Shift = {
  id: string;
  register_id: string;
  employee_id: string;
  state: string;
  allowed_actions: string[];
  z_operation_id?: string;
  z_fiscal_reference?: string;
  close_error?: string;
};
type Order = {
  id: string;
  state: string;
  version: number;
  allowed_actions?: string[];
  fiscal_sale_id?: string;
  fiscal_operation_id?: string;
  receipt_reference?: string;
  reversal_operation_id?: string;
  reversal_fiscal_reference?: string;
  updated_at?: string;
};
type FiscalOperation = {
  operation_id: string;
  type: "FISCAL_SALE";
  state: "FISCALIZED" | "FAILED" | "UNKNOWN";
  fiscal_reference?: string | null;
};
type Configuration = {
  id: string;
  location_name: string;
  location_address: string;
  workstation_name: string;
  fiscal_register_id: string;
  version: number;
};
type SalesReport = {
  from: string;
  to: string;
  currency: "EUR";
  gross: Money;
  payments: { type: string; amount: Money }[];
  generated_at: string;
};
const apiBase = (
  process.env.EXPO_PUBLIC_MINIPOS_API_URL ||
  "http://localhost:8081/public/v1/minipos"
).replace(/\/$/, "");
const apiVersion = "2026-08-07";
const appEnv = (process.env.EXPO_PUBLIC_APP_ENV || "dev").toLowerCase();
const prodMode = appEnv === "prod";
const devAuthToken = prodMode
  ? ""
  : process.env.EXPO_PUBLIC_MINIPOS_AUTH_TOKEN || "";
let runtimeAuthToken = devAuthToken;
let runtimeUnauthorized: (() => void) | undefined;
const fiscalBase = (
  process.env.EXPO_PUBLIC_FISCAL_API_URL || "http://localhost:8080/public/v1"
).replace(/\/$/, "");
const devFiscalToken = prodMode
  ? ""
  : process.env.EXPO_PUBLIC_FISCAL_AUTH_TOKEN || "";
let runtimeFiscalToken = devFiscalToken;
const registerId = process.env.EXPO_PUBLIC_REGISTER_ID || "";
const fiscalDeviceId = process.env.EXPO_PUBLIC_FISCAL_DEVICE_ID || "";
const appInstanceId =
  process.env.EXPO_PUBLIC_APP_INSTANCE_ID ||
  "00000000-0000-4000-8000-000000000001";
const uuid = () =>
  "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = Math.floor(Math.random() * 16);
    return (c === "x" ? r : (r & 3) | 8).toString(16);
  });
// The HTTP idempotency key is the end-to-end client operation identity. It is
// generated before the first send and reused by fiscalCall across REST retries.
const key = () => uuid();
class MiniPosHttpError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "MiniPosHttpError";
  }
}
function responseErrorMessage(text: string, fallback: string) {
  if (!text) return fallback;
  try {
    const problem = JSON.parse(text) as { detail?: unknown; title?: unknown };
    if (typeof problem.detail === "string" && problem.detail.trim())
      return problem.detail;
    if (typeof problem.title === "string" && problem.title.trim())
      return problem.title;
  } catch {}
  return text;
}
async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  const r = await fetchWithTimeout(apiBase + path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-Api-Version": apiVersion,
      "X-App-Instance-Id": appInstanceId,
      "Idempotency-Key": key(),
      ...(runtimeAuthToken
        ? { Authorization: `Bearer ${runtimeAuthToken}` }
        : {}),
      ...(init.headers || {}),
    },
  });
  const text = await r.text();
  if (r.status === 401 && prodMode) runtimeUnauthorized?.();
  if (!r.ok)
    throw new MiniPosHttpError(
      r.status,
      responseErrorMessage(text, `HTTP ${r.status}`),
    );
  return text ? JSON.parse(text) : ({} as T);
}
const collect = <T,>(path: string) =>
  collectCursorPages<T>(path, (p) => call(p));
const collectFiscal = <T,>(path: string) =>
  collectCursorPages<T>(path, (p) => fiscalCall(p));
async function fiscalCall<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method || "GET").toUpperCase();
  const idempotencyKey =
    new Headers(init.headers).get("Idempotency-Key") || key();
  const request: RequestInit = {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-Api-Version": apiVersion,
      "Idempotency-Key": idempotencyKey,
      ...(runtimeFiscalToken
        ? { Authorization: `Bearer ${runtimeFiscalToken}` }
        : {}),
      ...(init.headers || {}),
    },
  };
  let r: Response;
  try {
    r = await fetchWithTimeout(fiscalBase + path, request);
  } catch (first) {
    if (method === "GET") throw first;
    r = await fetchWithTimeout(fiscalBase + path, request);
  }
  const text = await r.text();
  if (r.status === 401 && prodMode) runtimeUnauthorized?.();
  if (!r.ok)
    throw new FiscalHttpError(r.status, text || `Fiscal HTTP ${r.status}`);
  return text ? JSON.parse(text) : ({} as T);
}
class FiscalHttpError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "FiscalHttpError";
  }
}
class CheckoutFinalError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "CheckoutFinalError";
  }
}
async function fiscalCloudReachable(path: string): Promise<boolean> {
  void path;
  return pingFiscal(fiscalBase);
}

type FiscalRegisterReference = {id:string;code?:string;name?:string;status?:string};
export default function App() { return <BeeloyAppProvider><View style={{flex:1}}><AppContent/><DemoBadge/></View></BeeloyAppProvider>; }
function DemoBadge(){
  const theme = useTheme();
  if (!Constants.expoConfig?.extra?.isDemo) return null;
  return <PaperSurface pointerEvents="none" elevation={1} style={[demo.badge, { backgroundColor: theme.colors.errorContainer }]}>
    <PaperText variant="labelSmall" style={{ color: theme.colors.onErrorContainer, fontWeight: "700" }}>DEMO</PaperText>
  </PaperSurface>;
}
const demo=StyleSheet.create({badge:{position:"absolute",right:8,top:8,zIndex:9999,borderRadius:5,paddingHorizontal:9,paddingVertical:4}});
function AppContent() {
  const { preference: themePreference, dark, selectTheme } = useThemePreference();
  return <MiniPosApp dark={dark} themePreference={themePreference} selectTheme={selectTheme} />;
}

function MiniPosApp({
  dark,
  themePreference,
  selectTheme,
}: {
  dark: boolean;
  themePreference: ThemePreference;
  selectTheme: (value: ThemePreference) => void;
}) {
  const { width } = useWindowDimensions();
  const paperTheme = useTheme();
  const isPhone = width < 720;
  const isCompact = width < 1024;
  const connectivity = useRef(new ConnectivityController());
  const checkoutJournal = useRef(new CheckoutJournal());
  const emailAuth = useEmailAuth(apiBase);
  const legalText = legalTranslations[emailAuth.language];
  const uiText = useTranslation();
  runtimeUnauthorized = () => void emailAuth.logout();
  const [legalAccepted,setLegalAccepted]=useState(false);
  const [loginCode,setLoginCode]=useState(""), [companyName,setCompanyName]=useState(""), [companyAddress,setCompanyAddress]=useState(""), [taxIdentifier,setTaxIdentifier]=useState(""), [currentUserName,setCurrentUserName]=useState("");
  const [products, setProducts] = useState<Product[]>([]),
    [taxGroups, setTaxGroups] = useState<TaxGroup[]>([]),
    [employees, setEmployees] = useState<Employee[]>([]),
    [shift, setShift] = useState<Shift | null>(null),
    [saleProjection, setSaleProjection] = useState<FiscalSale | null>(null),
    [workstationSession, setWorkstationSession] =
      useState<WorkstationSession | null>(null);
  const [status, setStatus] = useState("Зареждане…"),
    [search, setSearch] = useState(""),
    [splitCash, setSplitCash] = useState(""),
    [busy, setBusy] = useState(false),
    [admin, setAdmin] = useState(false),
    [mobilePosView, setMobilePosView] = useState<"catalog" | "cart">("catalog"),
    [pendingOrder, setPendingOrder] = useState<Order | null>(null);
  useEffect(() => setStatus(uiText.ready), [uiText]);
  const [pName, setPName] = useState(""),
    [pPrice, setPPrice] = useState(""),
    [pBarcode, setPBarcode] = useState(""),
    [pTaxGroup, setPTaxGroup] = useState(""),
    [eFirst, setEFirst] = useState(""),
    [eLast, setELast] = useState(""),
    [eCode, setECode] = useState("");
  const [configuration, setConfiguration] = useState<Configuration | null>(
      null,
    ),
    [locationName, setLocationName] = useState("Магазин 1"),
    [locationAddress, setLocationAddress] = useState(""),
    [workstationName, setWorkstationName] = useState("Каса 01"),
    [configuredRegister, setConfiguredRegister] = useState(registerId),
    [fiscalRegisters, setFiscalRegisters] = useState<FiscalRegisterReference[]>([]),
    [selectedEmployeeId, setSelectedEmployeeId] = useState(""),
    [operatorEmployee, setOperatorEmployee] = useState<Employee | null>(null),
    [salesReport, setSalesReport] = useState<SalesReport | null>(null);
  const ble = useRef<WebBleBootstrap | NativeBleBootstrapContract>(
      Platform.OS === "web"
        ? new WebBleBootstrap()
        : createNativeBleBootstrap(),
    ),
    [bleBinding, setBleBinding] = useState<
      (ActiveBleBinding & BleSessionPackage & { device_id: string }) | null
    >(null),
    [bleReady, setBleReady] = useState(false);

  const cart = useMemo<CartLine[]>(
    () =>
      saleProjection?.lines.map((line) => ({
        product: products.find((p) => p.id === line.product_code) || {
          id: line.product_code || line.line_id,
          sku: line.product_code || line.line_id,
          name: line.name,
          price: line.unit_price,
          tax_group: line.tax_group,
          active: true,
        },
        quantity: Number(line.quantity),
        discountAmount: line.discount?.amount,
      })) || [],
    [saleProjection, products],
  );
  const total = useMemo(
    () =>
      cart.reduce(
        (n, x) =>
          n +
          Number(x.product.price.amount) * x.quantity -
          Number(x.discountAmount || "0"),
        0,
      ),
    [cart],
  );
  const activeRegisterId =
    configuration?.fiscal_register_id || configuredRegister || registerId;
  const selectedEmployee =
    operatorEmployee ||
    (prodMode
      ? undefined
      : employees.find((x) => x.id === selectedEmployeeId) || employees[0]);
  const canManage =
    !prodMode ||
    !!operatorEmployee?.roles?.some(
      (role) => role === "SUPERVISOR" || role === "ADMIN",
    );
  const clearBleLocal = () => {
    ble.current.resetSession();
    setBleBinding(null);
    setBleReady(false);
  };
  const mutateProjectedLine = async (
    productId: string,
    quantity: number,
    discountAmount?: string,
  ) => {
    if (!saleProjection) return;
    const line = saleProjection.lines.find((v) => v.product_code === productId);
    if (!line) return;
    if (quantity <= 0) {
      setSaleProjection(
        await fiscalCall<FiscalSale>(
          `/sales/${saleProjection.sale_id}/lines/${line.line_id}:cancel`,
          {
            method: "POST",
            headers: { "If-Match": String(saleProjection.version) },
            body: "{}",
          },
        ),
      );
      return;
    }
    setSaleProjection(
      await fiscalCall<FiscalSale>(
        `/sales/${saleProjection.sale_id}/lines/${line.line_id}`,
        {
          method: "PATCH",
          headers: { "If-Match": String(saleProjection.version) },
          body: JSON.stringify({
            ...line,
            quantity: quantity.toFixed(3),
            ...(discountAmount && Number(discountAmount) > 0
              ? {
                  discount: {
                    amount: Number(discountAmount).toFixed(2),
                    currency: "EUR",
                  },
                }
              : { discount: undefined }),
          }),
        },
      ),
    );
  };
  const setLineDiscount = (productId: string, amount: number) => {
    if (!canManage) {
      setStatus("Отстъпката изисква права на мениджър");
      return;
    }
    const line = cart.find((v) => v.product.id === productId);
    const gross = line ? Number(line.product.price.amount) * line.quantity : 0;
    if (!line || !Number.isFinite(amount) || amount < 0 || amount > gross) {
      setStatus("Отстъпката трябва да е между € 0.00 и сумата на реда");
      return;
    }
    void mutateProjectedLine(productId, line.quantity, amount.toFixed(2)).catch(
      (e) => setStatus(message(e)),
    );
  };
  const setLineDiscountText = (productId: string, value: string) => {
    if (!canManage) return;
    const normalized = value.replace(",", ".");
    if (!/^\d*(\.\d{0,2})?$/.test(normalized)) return;
    const line = cart.find((v) => v.product.id === productId);
    const amount = normalized === "" ? 0 : Number(normalized);
    const gross = line ? Number(line.product.price.amount) * line.quantity : 0;
    if (!line || amount > gross) {
      setStatus("Отстъпката не може да надвишава сумата на реда");
      return;
    }
    void mutateProjectedLine(productId, line.quantity, normalized).catch((e) =>
      setStatus(message(e)),
    );
  };
  const revokeActiveBle = async () => {
    const id = bleBinding?.ble_session_id;
    clearBleLocal();
    if (id)
      await fiscalCall(`/ble-sessions/${id}/revoke`, {
        method: "POST",
        body: "",
      }).catch(() => {});
  };
  const refresh = async () => {
    try {
      const session = prodMode
        ? await call<{
            authenticated: true;
            employee: Employee;
            expires_at: string;
          }>("/operator-session")
        : null;
      const privileged = !!session?.employee.roles?.some(
        (role) =>
          role === "SUPERVISOR" || role === "ADMIN" || role === "AUDITOR",
      );
      const employeeRequest =
        prodMode && !privileged
          ? Promise.resolve(session ? [session.employee] : [])
          : collect<Employee>("/employees");
      const [p, groups, e, registers] = await Promise.all([
        collect<Product>("/products"),
        collect<TaxGroup>("/tax-groups"),
        employeeRequest,
        collectFiscal<FiscalRegisterReference>("/registers"),
      ]);
      setFiscalRegisters(registers.filter((x) => !x.status || x.status === "ACTIVE"));
      setProducts(p.filter((x) => x.active));
      setTaxGroups(groups);
      setPTaxGroup((current) => current || groups.find((group) => group.status === "ACTIVE")?.code || "");
      setEmployees(e);
      const employee =
        session?.employee || e.find((x) => x.id === selectedEmployeeId) || e[0];
      if (session) {
        setOperatorEmployee(session.employee);
        setSelectedEmployeeId(session.employee.id);
      } else setSelectedEmployeeId((v) => v || employee?.id || "");
      let cfg: Configuration;
      try {
        cfg = await call<Configuration>("/configuration");
      } catch (error) {
        if (error instanceof MiniPosHttpError && error.status === 404) {
          setConfiguration(null);
          setAdmin(true);
          setStatus("Настройте POS преди първата продажба");
          return;
        }
        throw error;
      }
      setConfiguration(cfg);
      setLocationName(cfg.location_name);
      setLocationAddress(cfg.location_address);
      setWorkstationName(cfg.workstation_name);
      setConfiguredRegister(cfg.fiscal_register_id);
      const recoveryRegister =
        cfg.fiscal_register_id || configuredRegister || registerId;
      if (employee && isFiscalResourceId(recoveryRegister)) {
        // SUPTO startup orchestration: establish trusted daily time evidence,
        // probe the selected physical FU and bind the operator before any sale
        // intent can be accepted. A failed probe keeps the workstation blocked.
        await fiscalCall(`/workstations/${recoveryRegister}/clock-sync`, {
          method: "POST",
          body: "{}",
        });
        await fiscalCall(
          `/workstations/${recoveryRegister}/readiness:refresh`,
          { method: "POST", body: "{}" },
        );
        const startupSession = await fiscalCall<WorkstationSession>(
          `/workstations/${recoveryRegister}/sessions`,
          {
            method: "POST",
            body: JSON.stringify({
              operator_code: employee.operator_code,
              app_instance_id: appInstanceId,
            }),
          },
        );
        setWorkstationSession(startupSession);
        const shifts = await call<{ items: Shift[] }>(
          `/shifts?employee_id=${encodeURIComponent(employee.id)}&register_id=${encodeURIComponent(recoveryRegister)}`,
        );
        const recovered =
          shifts.items.find((x) => x.state !== "CLOSED") || null;
        setShift(recovered);
        if (recovered?.state === "OPEN") {
          const journals = await checkoutJournal.current.pending();
          if (journals.length) {
            const durable = journals[0];
            const sale = await fiscalCall<FiscalSale>(
              `/sales/${durable.sale_id}`,
            ).catch(() => null);
            if (sale) {
              setSaleProjection(sale);
              setPendingOrder({
                id: durable.sale_id,
                state: "UNKNOWN",
                version: durable.sale_version,
                allowed_actions: ["READ"],
                fiscal_operation_id: durable.plan.client_operation_id,
              });
            }
          } else {
            const sales = await fiscalCall<{ items: FiscalSale[] }>(
              `/sales?operator_id=${encodeURIComponent(employee.operator_code)}&register_id=${encodeURIComponent(recoveryRegister)}&state=OPEN`,
            ).catch(() => ({ items: [] }));
            setSaleProjection(sales.items[0] || null);
          }
        }
        if (recovered?.state !== "OPEN") {
          setSaleProjection(null);
          setSplitCash("");
        }
        setStatus(
          recovered?.state === "BLOCKED_RECONCILIATION"
            ? "Z-отчетът изисква сверяване • продажбите са блокирани"
            : recovered
              ? "Отворената смяна е възстановена • проверете готовността на ФУ"
              : "Готово • отворете смяна",
        );
      } else if (!employee) {
        setShift(null);
        setStatus("Създайте служител преди отваряне на смяна");
      } else {
        setShift(null);
        setSaleProjection(null);
        setSplitCash("");
        setStatus(
          "Настройте валидна фискална каса преди възстановяване на смяна",
        );
      }
    } catch (e) {
      setOperatorEmployee(null);
      setShift(null);
      setStatus(
        prodMode
          ? "Операторската идентичност не е привързана • продажбата е блокирана"
          : "MiniPOS API е недостъпен • продажбата е блокирана",
      );
    }
  };
  useEffect(() => {
    if (!emailAuth.accessToken) {
      if (runtimeAuthToken) void revokeActiveBle();
      runtimeAuthToken = "";
      runtimeFiscalToken = "";
      setOperatorEmployee(null);
      setShift(null);
      setSaleProjection(null);
      setSplitCash("");
      setAdmin(false);
      setStatus("Влезте с персоналния си профил");
      return;
    }
    runtimeAuthToken = emailAuth.accessToken;
    runtimeFiscalToken = emailAuth.accessToken;
    void refresh();
  }, [emailAuth.accessToken]);
  useEffect(() => {
    if (!bleBinding) return;
    const remaining = new Date(bleBinding.expires_at).getTime() - Date.now();
    if (remaining <= 0) {
      clearBleLocal();
      setStatus("BLE сесията изтече • активирайте я отново");
      return;
    }
    const timer = setTimeout(
      () => {
        const id = bleBinding.ble_session_id;
        clearBleLocal();
        setStatus("BLE сесията изтече • активирайте я отново");
        void fiscalCall(`/ble-sessions/${id}/revoke`, {
          method: "POST",
          body: "",
        }).catch(() => {});
      },
      Math.min(remaining, 2_147_000_000),
    );
    return () => clearTimeout(timer);
  }, [bleBinding?.ble_session_id, bleBinding?.expires_at]);
  const add = async (p: Product) => {
    if (
      !shift ||
      shift.state !== "OPEN" ||
      !shift.allowed_actions?.includes("SALE")
    ) {
      setStatus("Продажбата е блокирана: няма активна отворена смяна");
      return;
    }
    if (busy || !selectedEmployee) return;
    setBusy(true);
    try {
      const existing = cart.find((v) => v.product.id === p.id);
      if (existing) {
        await mutateProjectedLine(
          p.id,
          existing.quantity + 1,
          existing.discountAmount,
        );
        return;
      }
      let session = workstationSession;
      if (!saleProjection) {
        await fiscalCall(`/workstations/${activeRegisterId}/clock-sync`, {
          method: "POST",
          body: "{}",
        });
        await fiscalCall(
          `/workstations/${activeRegisterId}/readiness:refresh`,
          { method: "POST", body: "{}" },
        );
      }
      if (!session || new Date(session.expires_at).getTime() <= Date.now()) {
        session = await fiscalCall<WorkstationSession>(
          `/workstations/${activeRegisterId}/sessions`,
          {
            method: "POST",
            body: JSON.stringify({
              operator_code: selectedEmployee.operator_code,
              app_instance_id: appInstanceId,
            }),
          },
        );
        setWorkstationSession(session);
      }
      const line: FiscalSaleLine = {
        line_id: uuid(),
        product_code: p.id,
        name: p.name,
        quantity: "1.000",
        unit_price: p.price,
        tax_group: p.tax_group,
      };
      const next = saleProjection
        ? await fiscalCall<FiscalSale>(
            `/sales/${saleProjection.sale_id}/lines`,
            {
              method: "POST",
              headers: { "If-Match": String(saleProjection.version) },
              body: JSON.stringify(line),
            },
          )
        : await fiscalCall<FiscalSale>("/sales:open-with-line", {
            method: "POST",
            body: JSON.stringify({
              client_sale_surrogate_id: uuid(),
              workstation_id: activeRegisterId,
              operator_session_id: session.session_id,
              line,
            }),
          });
      setSaleProjection(next);
      setStatus(
        `Продажбата е отворена • УНП ${next.regulatory_identifiers[0]?.value || next.unp}`,
      );
    } catch (e) {
      setStatus(`Артикулът не е приет: ${message(e)}`);
    } finally {
      setBusy(false);
    }
  };
  const submitProductLookup = () => {
    const value = search.trim();
    if (!value) return;
    const exact = products.find(
      (p) => p.barcode === value || p.sku.toLowerCase() === value.toLowerCase(),
    );
    if (!exact) {
      setStatus(`Няма продукт за код ${value}`);
      return;
    }
    void add(exact);
    setSearch("");
  };
  const toggleShift = async () => {
    if (busy) return;
    const opening = !shift;
    setBusy(true);
    try {
      if (shift) {
        const action = shift.allowed_actions?.includes("RECONCILE")
          ? "reconcile"
          : "close";
        const closed = await call<Shift>(`/shifts/${shift.id}/${action}`, {
          method: "POST",
          body: "{}",
        });
        if (
          closed.state !== "CLOSED" ||
          !closed.z_operation_id ||
          !closed.z_fiscal_reference
        )
          throw new Error("липсва финален Z-report reference");
        setShift(null);
        setSaleProjection(null);
        setStatus(`Смяната е затворена • Z ${closed.z_fiscal_reference}`);
      } else {
        if (!configuration) throw new Error(uiText.configureBeforeShift);
        if (!selectedEmployee)
          throw new Error(uiText.selectEmployeeBeforeShift);
        if (!isFiscalResourceId(activeRegisterId))
          throw new Error(uiText.invalidRegister);
        const fiscalRegister = requireFiscalResourceId(
          activeRegisterId,
          "Фискалната каса",
        );
        const opened = await call<Shift>("/shifts", {
          method: "POST",
          body: JSON.stringify({
            register_id: fiscalRegister,
            employee_id: selectedEmployee.id,
          }),
        });
        setShift(opened);
        setStatus("Смяната е отворена • проверете готовността на ФУ");
      }
    } catch (e) {
      if (!opening && shift) {
        try {
          const recovered = (await collect<Shift>("/shifts")).find(
            (candidate) => candidate.id === shift.id && candidate.state !== "CLOSED",
          );
          if (recovered) {
            setShift(recovered);
            if (
              recovered.state === "BLOCKED_RECONCILIATION" &&
              recovered.allowed_actions?.includes("RECONCILE")
            ) {
              setStatus(
                "Z-отчетът изисква сверяване • продажбите са блокирани",
              );
              return;
            }
          }
        } catch {
          // Preserve the original close failure when authoritative recovery is
          // unavailable; never infer a successful close locally.
        }
      }
      setStatus(
        `${opening ? uiText.openShiftFailed : uiText.shiftOperationFailed}: ${message(e)}`,
      );
    } finally {
      setBusy(false);
    }
  };
  const connectBle = async () => {
    if (busy) return;
    setBusy(true);
    let issuedSessionId = "";
    try {
      if (Platform.OS === "web" && !webBluetoothSupported())
        throw new Error("Web Bluetooth изисква Chrome/secure context");
      if (Platform.OS !== "web" && !(await requestNativeBlePermissions()))
        throw new Error("Bluetooth permission е отказано");
      if (!selectedEmployee) throw new Error("Създайте служител");
      const fiscalRegister = requireFiscalResourceId(
        activeRegisterId,
        "Фискалната каса",
      );
      const publicKey = ble.current.prepareSessionPublicKey();
      const expectedDevice = resolveFiscalDeviceId(
        await fiscalCall<FiscalRegisterResource>(
          `/registers/${fiscalRegister}`,
        ),
        fiscalRegister,
        prodMode ? "" : fiscalDeviceId,
      );
      const session = await fiscalCall<
        BleSessionPackage & {
          edge_id: string;
          device_id: string;
          location_id: string;
          binding_version: number;
        }
      >(`/registers/${fiscalRegister}/ble-sessions`, {
        method: "POST",
        body: JSON.stringify({
          operator_id: selectedEmployee.operator_code,
          app_instance_id: appInstanceId,
          public_key: publicKey,
        }),
      });
      issuedSessionId = session.ble_session_id;
      validateBleDeploymentAuthority(session, {
        register_id: fiscalRegister,
        device_id: expectedDevice,
      });
      const binding = { ...session };
      const connected = await ble.current.connectFromUserGesture(
        session,
        () => {},
      );
      await connected.ready;
      setBleBinding(binding);
      setBleReady(true);
      setStatus("Локалният BLE контур е READY");
    } catch (e) {
      ble.current.resetSession();
      setBleBinding(null);
      setBleReady(false);
      if (issuedSessionId)
        await fiscalCall(`/ble-sessions/${issuedSessionId}/revoke`, {
          method: "POST",
          body: "",
        }).catch(() => {});
      setStatus(`BLE не е готов: ${message(e)}`);
    } finally {
      setBusy(false);
    }
  };
  const checkout = async (type: "CASH" | "CARD" | "SPLIT") => {
    if (
      busy ||
      !shift ||
      shift.state !== "OPEN" ||
      !shift.allowed_actions?.includes("SALE") ||
      !cart.length ||
      !selectedEmployee ||
      pendingOrder
    )
      return;
    const totalCents = Math.round(total * 100),
      cashCents = Math.round(Number(splitCash.replace(",", ".")) * 100);
    if (
      type === "SPLIT" &&
      (!Number.isFinite(cashCents) || cashCents <= 0 || cashCents >= totalCents)
    ) {
      setStatus(
        "Разделено плащане: въведете сума в брой над € 0.00 и под общата сума",
      );
      return;
    }
    setBusy(true);
    setStatus(
      type === "CARD"
        ? "Картово плащане и фискализация…"
        : type === "SPLIT"
          ? "Разделено плащане: в брой, карта и фискализация…"
          : "Фискализация…",
    );
    let recoveryOperationId = "";
    try {
      const payments =
        type === "SPLIT"
          ? [
              {
                payment_id: uuid(),
                type: "CASH" as const,
                amount: {
                  amount: (cashCents / 100).toFixed(2),
                  currency: "EUR" as const,
                },
                terminal_policy: "NONE",
              },
              {
                payment_id: uuid(),
                type: "CARD" as const,
                amount: {
                  amount: ((totalCents - cashCents) / 100).toFixed(2),
                  currency: "EUR" as const,
                },
                terminal_policy: "AUTO_IF_AVAILABLE",
              },
            ]
          : [
              {
                payment_id: uuid(),
                type,
                amount: { amount: total.toFixed(2), currency: "EUR" as const },
                terminal_policy: type === "CARD" ? "AUTO_IF_AVAILABLE" : "NONE",
              },
            ];
      if (!saleProjection) throw new Error("Липсва приета продажба");
      let sale = saleProjection;
      let last: FiscalOperation | undefined;
      const stored = await checkoutJournal.current.load(sale.sale_id);
      const checkoutPlan =
        stored?.plan || newCheckoutPlan(uuid, payments, total);
      recoveryOperationId = checkoutPlan.client_operation_id;
      if (stored && stored.sale_version !== sale.version)
        throw new Error("CHECKOUT_RECONCILIATION_REQUIRED");
      await checkoutJournal.current.save({
        sale_id: sale.sale_id,
        sale_version: sale.version,
        plan: checkoutPlan,
        created_at: new Date().toISOString(),
      });
      const cloud = await fiscalCloudReachable(`/sales/${sale.sale_id}`);
      connectivity.current.observe(cloud, Boolean(bleReady && bleBinding));
      if (!cloud) {
        await new Promise((r) => setTimeout(r, 1000));
        connectivity.current.observe(
          await fiscalCloudReachable(""),
          Boolean(bleReady && bleBinding),
        );
        await new Promise((r) => setTimeout(r, 1000));
        connectivity.current.observe(
          await fiscalCloudReachable(""),
          Boolean(bleReady && bleBinding),
        );
      }
      const useBle =
        Boolean(bleReady && bleBinding) && connectivity.current.useBle();
      if (useBle) {
        const intent = bleFinalizeIntent(checkoutPlan, bleBinding!, sale, {
          operator_code: selectedEmployee.operator_code,
          app_instance_id: appInstanceId,
          expected_version: sale.version,
        });
        const local = await ble.current.sendComplianceIntentAndWait(
          intent,
          checkoutPlan.client_operation_id,
        );
        last = {
          operation_id: local.operation_id,
          type: "FISCAL_SALE",
          state:
            local.state === "FISCALIZED"
              ? "FISCALIZED"
              : local.state === "FISCAL_RESULT_UNKNOWN"
                ? "UNKNOWN"
                : "FAILED",
          fiscal_reference: local.fiscal_reference,
        };
        if (local.state === "FISCAL_RESULT_UNKNOWN") {
          setPendingOrder({
            id: sale.sale_id,
            state: "UNKNOWN",
            version: sale.version,
            allowed_actions: ["READ"],
            fiscal_operation_id: local.operation_id,
          });
          throw new Error("UNKNOWN: RECONCILE");
        }
        if (local.state !== "FISCALIZED") {
          await checkoutJournal.current.remove(sale.sale_id);
          throw new CheckoutFinalError(local.error_code || "PAYMENT_REJECTED");
        }
        sale = { ...sale, state: "COMPLETED" };
        setSaleProjection(sale);
      } else {
        try {
          last = await fiscalCall<FiscalOperation>(
            `/sales/${sale.sale_id}:finalize`,
            {
              method: "POST",
              headers: {
                "If-Match": String(sale.version),
                "Idempotency-Key": checkoutPlan.client_operation_id,
              },
              body: JSON.stringify(checkoutPlan),
            },
          );
        } catch (e) {
          if (!(bleReady && bleBinding) || e instanceof FiscalHttpError)
            throw e;
          const intent = bleFinalizeIntent(checkoutPlan, bleBinding, sale, {
            operator_code: selectedEmployee.operator_code,
            app_instance_id: appInstanceId,
            expected_version: sale.version,
          });
          const local = await ble.current.sendComplianceIntentAndWait(
            intent,
            checkoutPlan.client_operation_id,
          );
          last = {
            operation_id: local.operation_id,
            type: "FISCAL_SALE",
            state:
              local.state === "FISCALIZED"
                ? "FISCALIZED"
                : local.state === "FISCAL_RESULT_UNKNOWN"
                  ? "UNKNOWN"
                  : "FAILED",
            fiscal_reference: local.fiscal_reference,
          };
        }
        if (last.state === "UNKNOWN") {
          setPendingOrder({
            id: sale.sale_id,
            state: "UNKNOWN",
            version: sale.version,
            allowed_actions: ["READ"],
            fiscal_operation_id: last.operation_id,
          });
          throw new Error("UNKNOWN: RECONCILE");
        }
        if (last.state === "FAILED") {
          await checkoutJournal.current.remove(sale.sale_id);
          throw new CheckoutFinalError("PAYMENT_REJECTED");
        }
        sale = await fiscalCall<FiscalSale>(`/sales/${sale.sale_id}`);
        setSaleProjection(sale);
      }
      if (sale.state !== "COMPLETED")
        throw new Error("Плащането не е завършено");
      setPendingOrder(null);
      await checkoutJournal.current.remove(sale.sale_id);
      setSaleProjection(null);
      setSplitCash("");
      setStatus(
        `Успешен фискален бон • ${last?.fiscal_reference || last?.operation_id} • EUR ${total.toFixed(2)}`,
      );
    } catch (e) {
      if (
        saleProjection &&
        message(e) !== "UNKNOWN: RECONCILE" &&
        !(e instanceof CheckoutFinalError) &&
        !(e instanceof FiscalHttpError)
      )
        setPendingOrder({
          id: saleProjection.sale_id,
          state: "UNKNOWN",
          version: saleProjection.version,
          allowed_actions: ["READ"],
          fiscal_operation_id:
            recoveryOperationId || saleProjection.fiscal_operation_id,
        });
      setStatus(
        `Няма потвърден фискален успех; не повтаряйте плащането: ${message(e)}`,
      );
    } finally {
      setBusy(false);
    }
  };
  const reconcile = async () => {
    if (!pendingOrder || busy) return;
    if (!pendingOrder.allowed_actions?.includes("READ")) {
      setStatus("Проверката е блокирана: backend не я разрешава");
      return;
    }
    setBusy(true);
    try {
      if (!pendingOrder.fiscal_operation_id)
        throw new Error("Липсва operation ID");
      const operation = await fiscalCall<FiscalOperation>(
        `/operations/${pendingOrder.fiscal_operation_id}`,
      );
      if (operation.state === "UNKNOWN")
        await fiscalCall(`/operations/${operation.operation_id}:reconcile`, {
          method: "POST",
          body: "{}",
        });
      const sale = await fiscalCall<FiscalSale>(`/sales/${pendingOrder.id}`);
      setSaleProjection(sale);
      if (sale.state === "COMPLETED") {
        await checkoutJournal.current.remove(pendingOrder.id);
        setPendingOrder(null);
        setSaleProjection(null);
        setStatus(
          `Фискалният резултат е потвърден • ${operation.operation_id}`,
        );
      } else if (sale.state === "FAILED" || sale.state === "CANCELLED") {
        await checkoutJournal.current.remove(pendingOrder.id);
        setPendingOrder(null);
        setStatus(
          `Продажбата е ${sale.state}; нов опит е разрешен само като нова продажба`,
        );
      } else
        setStatus(
          `Резултатът още се проверява • ${sale.state} • ${operation.operation_id}`,
        );
    } catch (e) {
      setStatus(
        `Проверката е недостъпна; плащането не е повторено: ${message(e)}`,
      );
    } finally {
      setBusy(false);
    }
  };
  const createProduct = async () => {
    try {
      await call("/products", {
        method: "POST",
        body: JSON.stringify({
          sku: `SKU-${Date.now()}`,
          barcode: pBarcode.trim() || null,
          name: pName,
          price: { amount: Number(pPrice).toFixed(2), currency: "EUR" },
          tax_group: pTaxGroup,
        }),
      });
      setPName("");
      setPPrice("");
      setPBarcode("");
      await refresh();
    } catch (e) {
      setStatus(message(e));
    }
  };
  const createEmployee = async () => {
    try {
      await call("/employees", {
        method: "POST",
        body: JSON.stringify({
          first_name: eFirst,
          last_name: eLast,
          operator_code: eCode,
        }),
      });
      setEFirst("");
      setELast("");
      setECode("");
      await refresh();
    } catch (e) {
      setStatus(message(e));
    }
  };
  const saveTaxGroup = async (group: Partial<TaxGroup> & { code: string; name: string; rate: string }) => {
    try {
      const body = JSON.stringify({ code: group.code, name: group.name, rate: group.rate, status: group.status || "ACTIVE" });
      if (group.id) {
        await call(`/tax-groups/${group.id}`, {
          method: "PATCH",
          headers: { "If-Match": String(group.version) },
          body,
        });
      } else {
        await call("/tax-groups", { method: "POST", body });
      }
      await refresh();
      return true;
    } catch (e) {
      setStatus(message(e));
      return false;
    }
  };
  const saveConfiguration = async () => {
    if (shift) {
      setStatus("Затворете смяната преди промяна на касовото място");
      return;
    }
    try {
      const fiscalRegister = requireFiscalResourceId(
        configuredRegister,
        "Фискалната каса",
      );
      const cfg = await call<Configuration>("/configuration", {
        method: "PATCH",
        headers: configuration
          ? { "If-Match": String(configuration.version) }
          : {},
        body: JSON.stringify({
          location_name: locationName,
          location_address: locationAddress,
          workstation_name: workstationName,
          fiscal_register_id: fiscalRegister,
        }),
      });
      setConfiguration(cfg);
      await revokeActiveBle();
      setStatus(
        "Точката и касовото място са записани; BLE сесията трябва да се активира отново",
      );
    } catch (e) {
      setStatus(message(e));
    }
  };
  const loadReport = async () => {
    try {
      const to = new Date(),
        from = new Date(to.getTime() - 30 * 24 * 60 * 60 * 1000);
      const report = await call<SalesReport>(
        `/reports/sales?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`,
      );
      setSalesReport(report);
      setStatus(`Търговски отчет • EUR ${report.gross.amount}`);
    } catch (e) {
      setStatus(message(e));
    }
  };
  const logoutOperator = async () => {
    if (shift) {
      setStatus("Затворете смяната преди изход");
      return;
    }
    try {
      await revokeActiveBle();
      await call("/operator-session", { method: "POST", body: "{}" });
    } finally {
      clearBleLocal();
      await emailAuth.logout();
      runtimeAuthToken = "";
      runtimeFiscalToken = "";
      setOperatorEmployee(null);
      setShift(null);
      setSaleProjection(null);
    }
  };
  const changeQuantity = (productId: string, delta: number) => {
    const line = cart.find((v) => v.product.id === productId);
    if (!line) return;
    const quantity = line.quantity + delta,
      discount = Math.min(
        Number(line.discountAmount || "0"),
        Number(line.product.price.amount) * Math.max(0, quantity),
      ).toFixed(2);
    void mutateProjectedLine(productId, quantity, discount).catch((e) =>
      setStatus(message(e)),
    );
  };
  if (!emailAuth.accessToken)
    return (
      <SafeAreaView testID="app-root" style={[s.root, s.authRoot, { backgroundColor: paperTheme.colors.background }]}>
        <StatusBar style={dark ? "light" : "dark"} />
        <Image
          accessibilityIgnoresInvertColors
          source={require("../imgs/background.png")}
          resizeMode="cover"
          style={s.authBackground}
        />
        <View style={s.authHeader}>
          <ThemeMenu preference={themePreference} text={uiText} onSelect={selectTheme} />
          <LanguageMenu
            language={emailAuth.language}
            label={uiText.language}
            onSelect={(value) => void emailAuth.setLanguage(value)}
          />
        </View>
        <NativeScrollView
          style={s.authScroll}
          contentContainerStyle={s.authScrollContent}
          keyboardShouldPersistTaps="handled"
        >
        <AuthForm testID="operator-login" style={s.authForm}>
          <PaperCard mode="elevated" style={s.loginCard}>
          <PaperCard.Content style={s.loginContent}>
          <PaperText variant="titleLarge">BeeMiniPOS</PaperText>
          <PaperText variant="headlineSmall" accessibilityRole="header">{uiText.signIn}</PaperText>
          {emailAuth.stage === "email" ? <><TextInput testID="operator-login-email" label={uiText.email} accessibilityLabel={uiText.email} keyboardType="email-address" autoCapitalize="none" autoComplete="email" placeholder="email@example.com" value={emailAuth.email} onChangeText={emailAuth.setEmail}/><PaperButton testID="operator-login-start" mode="contained" contentStyle={s.authButtonContent} loading={emailAuth.busy} disabled={emailAuth.busy||!emailAuth.email} onPress={()=>void emailAuth.requestCode()}>{uiText.sendCode}</PaperButton></> : null}
          {emailAuth.stage === "code" ? <><TextInput testID="operator-login-code" label={uiText.oneTimeCode} accessibilityLabel={uiText.oneTimeCode} textContentType="oneTimeCode" keyboardType="number-pad" maxLength={6} placeholder="000000" value={loginCode} onChangeText={setLoginCode}/><PaperButton testID="operator-login-verify" mode="contained" contentStyle={s.authButtonContent} loading={emailAuth.busy} disabled={emailAuth.busy||loginCode.length!==6} onPress={()=>void emailAuth.verifyCode(loginCode)}>{uiText.continue}</PaperButton></> : null}
          {emailAuth.stage === "onboarding" ? <>
            <PaperText variant="titleLarge" accessibilityRole="header">{uiText.companyDetails}</PaperText>
            <TextInput testID="onboarding-company-name" placeholder={uiText.companyName} value={companyName} onChangeText={setCompanyName}/>
            <TextInput testID="onboarding-address" placeholder={uiText.address} value={companyAddress} onChangeText={setCompanyAddress}/>
            <TextInput testID="onboarding-tax-id" placeholder={uiText.taxIdentifier} value={taxIdentifier} onChangeText={setTaxIdentifier}/>
            <TextInput testID="onboarding-full-name" placeholder={uiText.fullName} value={currentUserName} onChangeText={setCurrentUserName}/>
            <PaperCheckbox.Item
              testID="onboarding-legal-consent"
              position="leading"
              status={legalAccepted ? "checked" : "unchecked"}
              onPress={() => setLegalAccepted(value => !value)}
              label={legalText.consent}
            />
            <PaperLegalLink testID="operator-onboarding-terms" label={legalText.terms} url={`https://legal.beeloy.org/${emailAuth.language}/terms`} />
            <PaperLegalLink testID="operator-onboarding-privacy" label={legalText.privacy} url={`https://legal.beeloy.org/${emailAuth.language}/privacy`} />
            <PaperButton testID="operator-onboarding-create" mode="contained" contentStyle={s.authButtonContent} loading={emailAuth.busy} disabled={emailAuth.busy||!companyName||!companyAddress||!taxIdentifier||!currentUserName||!legalAccepted} onPress={()=>void emailAuth.onboard({company_name:companyName,address:companyAddress,tax_identifier:taxIdentifier,full_name:currentUserName,accepted_terms_version:"b2b-2026-09-28"})}>{uiText.createCompany}</PaperButton>
          </> : null}
          {emailAuth.error ? <PaperHelperText testID="operator-login-error" type="error" visible accessibilityLiveRegion="polite">{emailAuth.error}</PaperHelperText> : null}
          </PaperCard.Content>
          </PaperCard>
        </AuthForm>
        </NativeScrollView>
      </SafeAreaView>
    );
  return (
    <SafeAreaView testID="app-root" style={[s.root, { backgroundColor: paperTheme.colors.background }]}>
      <StatusBar style={dark ? "light" : "dark"} />
      <View testID="app-header" style={[s.top, isCompact && s.topCompact, { backgroundColor: paperTheme.colors.surface, borderColor: paperTheme.colors.outlineVariant }]}>
        <View style={s.topIdentity}>
          <PaperText variant="headlineSmall" style={{ color: paperTheme.colors.onSurface }}>BeeMiniPOS</PaperText>
          <PaperText variant="bodySmall" style={{ color: paperTheme.colors.onSurfaceVariant }}>
            {configuration?.location_name || "1 точка"} •{" "}
            {configuration?.workstation_name || "каса 01"} • {activeRegisterId}{" "}
            •{" "}
            {selectedEmployee
              ? `${selectedEmployee.first_name} ${selectedEmployee.last_name}`
              : "DEV"}{" "}
            • EUR • BLE {bleReady ? "READY" : "OFF"}
          </PaperText>
        </View>
        <View style={[s.topActions, isCompact && s.topActionsCompact]}>
          {prodMode ? (
            <PaperButton
              testID="operator-logout"
              mode="outlined"
              accessibilityLabel={uiText.logout}
              disabled={!!shift}
              onPress={() => void logoutOperator()}
            >{uiText.logout}</PaperButton>
          ) : null}
          <PaperButton
            testID="ble-connect"
            mode="outlined"
            accessibilityLabel={
              bleReady ? uiText.bleReady : uiText.connectBle
            }
            disabled={busy || bleReady}
            onPress={connectBle}
          >{bleReady ? uiText.bleConnected : uiText.connectBle}</PaperButton>
          {canManage ? (
            <PaperButton
              testID="admin-toggle"
              mode="outlined"
              accessibilityLabel={admin ? uiText.openSales : uiText.openConfiguration}
              onPress={() => setAdmin((v) => !v)}
            >{admin ? uiText.sales : uiText.configuration}</PaperButton>
          ) : null}
          <ThemeMenu
            preference={themePreference}
            text={uiText}
            onSelect={selectTheme}
          />
          <LanguageMenu
            language={emailAuth.language}
            label={uiText.language}
            onSelect={(value) => void emailAuth.setLanguage(value)}
          />
          <PaperButton
            testID="shift-toggle"
            mode={shift?.allowed_actions?.includes("RECONCILE") ? "contained-tonal" : shift ? "outlined" : "contained"}
            accessibilityLabel={
              shift?.allowed_actions?.includes("RECONCILE")
                ? uiText.reconcileZ
                : shift
                  ? uiText.closeShift
                  : uiText.openShift
            }
            disabled={busy}
            contentStyle={{ minHeight: 48 }}
            onPress={toggleShift}
          >{shift?.allowed_actions?.includes("RECONCILE")
            ? uiText.reconcileZ
            : shift
              ? uiText.closeShift
              : uiText.openShift}</PaperButton>
        </View>
      </View>
      {pendingOrder ? (
        <PaperCard
          testID="operation-unknown"
          mode="outlined"
          accessibilityLiveRegion="assertive"
          style={{ backgroundColor: paperTheme.colors.tertiaryContainer, borderColor: paperTheme.colors.tertiary }}
        >
          <PaperCard.Content style={[reconciliationStyles.panel, isPhone && reconciliationStyles.panelPhone]}>
          <View>
            <PaperText variant="titleMedium" style={{ color: paperTheme.colors.onTertiaryContainer }}>
              {uiText.resultChecking}
            </PaperText>
            <PaperText variant="bodyMedium" style={{ color: paperTheme.colors.onTertiaryContainer }}>
              {uiText.doNotRepeatPayment}. {pendingOrder.id} •{" "}
              {pendingOrder.state}
            </PaperText>
            <PaperText variant="bodyMedium" style={{ color: paperTheme.colors.onTertiaryContainer }}>
              {uiText.allowedActions}:{" "}
              {pendingOrder.allowed_actions?.join(", ") || uiText.noActions}
            </PaperText>
          </View>
          {pendingOrder.allowed_actions?.includes("READ") ? (
            <PaperButton
              testID="reconcile-start"
              mode="contained"
              accessibilityLabel={uiText.checkFiscalResult}
              disabled={busy}
              contentStyle={{ minHeight: 48 }}
              onPress={reconcile}
            >{uiText.checkResult}</PaperButton>
          ) : null}
          </PaperCard.Content>
        </PaperCard>
      ) : null}
      {admin ? (
        <Admin
          compact={isPhone}
          production={prodMode}
          products={products}
          taxGroups={taxGroups}
          employees={employees}
          selectedEmployeeId={selectedEmployee?.id || ""}
          salesReport={salesReport}
          fiscalRegisters={fiscalRegisters}
          values={{
            pName,
            pPrice,
            pBarcode,
            pTaxGroup,
            eFirst,
            eLast,
            eCode,
            locationName,
            locationAddress,
            workstationName,
            configuredRegister,
          }}
          setters={{
            setPName,
            setPPrice,
            setPBarcode,
            setPTaxGroup,
            setEFirst,
            setELast,
            setECode,
            setLocationName,
            setLocationAddress,
            setWorkstationName,
            setConfiguredRegister,
            setSelectedEmployeeId,
          }}
          createProduct={createProduct}
          createEmployee={createEmployee}
          saveTaxGroup={saveTaxGroup}
          saveConfiguration={saveConfiguration}
          loadReport={loadReport}
        />
      ) : (
        <View style={s.posShell}>
          {isPhone ? (
            <View style={[s.mobileTabs, { backgroundColor: paperTheme.colors.background }]}>
              <PaperSegmentedButtons
                value={mobilePosView}
                onValueChange={setMobilePosView}
                buttons={[
                  { value: "catalog", label: uiText.products, accessibilityLabel: mobilePosView === "catalog" ? `${uiText.products} • ${uiText.selected}` : uiText.products, testID: "mobile-view-catalog" },
                  { value: "cart", label: uiText.cart, accessibilityLabel: `${uiText.currentSale} (${cart.length}) • € ${total.toFixed(2)}${mobilePosView === "cart" ? ` • ${uiText.selected}` : ""}`, testID: "mobile-view-cart" },
                ]}
              />
            </View>
          ) : null}
          <View style={[s.body, isPhone && s.bodyPhone]}>
          {(!isPhone || mobilePosView === "catalog") ? <View style={[s.catalog, isPhone && s.panelPhone]}>
            <TextInput
              testID="product-search"
              accessibilityLabel={uiText.searchBarcode}
              value={search}
              onChangeText={setSearch}
              onSubmitEditing={submitProductLookup}
              returnKeyType="done"
              placeholder={uiText.searchBarcode}
              style={{ marginBottom: 14, backgroundColor: paperTheme.colors.surface }}
            />
            <ScrollView contentContainerStyle={s.grid}>
              {products
                .filter((p) => {
                  const q = search.toLowerCase();
                  return (
                    p.name.toLowerCase().includes(q) ||
                    p.sku.toLowerCase().includes(q) ||
                    (p.barcode || "").includes(search)
                  );
                })
                .map((p) => (
                  <PaperCard
                    testID={`product-${p.id}`}
                    mode="outlined"
                    key={p.id}
                    style={[s.product, isPhone && s.productPhone]}
                    onPress={() => add(p)}
                  >
                    <PaperCard.Content style={s.productContent}>
                    <PaperText variant="titleMedium">{p.name}</PaperText>
                    <PaperText variant="titleLarge" style={{ color: paperTheme.colors.primary }}>
                      € {Number(p.price.amount).toFixed(2)}
                    </PaperText>
                    <PaperText variant="bodySmall">{uiText.vat} {p.tax_group}</PaperText>
                    </PaperCard.Content>
                  </PaperCard>
                ))}
            </ScrollView>
          </View> : null}
          {(!isPhone || mobilePosView === "cart") ? <View
            style={[s.cart, isPhone && s.cartPhone, { backgroundColor: paperTheme.colors.surface, borderColor: paperTheme.colors.outlineVariant }]}
          >
            <NativeScrollView
              testID="cart-scroll"
              style={{ flex: 1 }}
              contentContainerStyle={[s.cartContent, isPhone && s.cartContentPhone]}
              keyboardShouldPersistTaps="handled"
            >
            <PaperText variant="titleLarge" style={{ marginBottom: 12 }}>{uiText.currentSale}</PaperText>
            {saleProjection ? (
              <View
                testID="sale-regulatory-identity"
                accessibilityLiveRegion="polite"
              >
                <PaperText variant="bodyMedium">
                  УНП:{" "}
                  {saleProjection.regulatory_identifiers[0]?.value ||
                    saleProjection.unp}
                </PaperText>
                {!isPhone ? <PaperText variant="bodyMedium">{uiText.registerState}: {saleProjection.register_id}</PaperText> : null}
                <PaperText variant="bodyMedium">{uiText.state}: {saleProjection.state}</PaperText>
              </View>
            ) : null}
            <View style={s.lines}>
              {cart.length === 0 ? (
                <PaperText variant="bodyMedium" style={[s.empty, { color: paperTheme.colors.onSurfaceVariant }]}>{uiText.touchProduct}</PaperText>
              ) : (
                cart.map((x) => (
                  <View
                    testID={`cart-line-${x.product.id}`}
                    style={[s.line, { borderColor: paperTheme.colors.outlineVariant }]}
                    key={x.product.id}
                  >
                    <View style={s.lineContent}>
                      <PaperText variant="bodyLarge">
                        {x.quantity} × {x.product.name}
                      </PaperText>
                      <View style={s.quantity}>
                        <PaperIconButton
                          testID={`quantity-dec-${x.product.id}`}
                          accessibilityLabel={`${uiText.decrease} ${x.product.name}`}
                          icon={() => <PaperText variant="titleLarge">−</PaperText>}
                          mode="outlined"
                          size={24}
                          style={{ width: 52, height: 52, margin: 0 }}
                          onPress={() => changeQuantity(x.product.id, -1)}
                        />
                        <PaperIconButton
                          testID={`quantity-inc-${x.product.id}`}
                          accessibilityLabel={`${uiText.increase} ${x.product.name}`}
                          icon={() => <PaperText variant="titleLarge">+</PaperText>}
                          mode="outlined"
                          size={24}
                          style={{ width: 52, height: 52, margin: 0 }}
                          onPress={() => changeQuantity(x.product.id, 1)}
                        />
                      </View>
                      {canManage ? (
                        <View style={s.discountRow}>
                          <PaperButton
                            testID={`discount-percent-${x.product.id}`}
                            mode="outlined"
                            accessibilityLabel={`${uiText.discount10} ${x.product.name}`}
                            contentStyle={{ minHeight: 48 }}
                            onPress={() =>
                              setLineDiscount(
                                x.product.id,
                                Number(x.product.price.amount) *
                                  x.quantity *
                                  0.1,
                              )
                            }
                          >−10%</PaperButton>
                          <TextInput
                            testID={`discount-amount-${x.product.id}`}
                            accessibilityLabel={`${uiText.discountEur} ${x.product.name}`}
                            style={{ minWidth: 88, flex: 1, backgroundColor: paperTheme.colors.surface }}
                            keyboardType="decimal-pad"
                            value={x.discountAmount ?? "0.00"}
                            onChangeText={(value) =>
                              setLineDiscountText(x.product.id, value)
                            }
                          />
                        </View>
                      ) : null}
                    </View>
                    <PaperText variant="titleMedium">
                      €{" "}
                      {(
                        Number(x.product.price.amount) * x.quantity -
                        Number(x.discountAmount || "0")
                      ).toFixed(2)}
                    </PaperText>
                  </View>
                ))
              )}
            </View>
            <View style={[s.splitRow, isPhone && s.splitRowPhone]}>
              <TextInput
                testID="split-cash-amount"
                label={uiText.cashEur}
                accessibilityLabel={uiText.splitCashAmount}
                value={splitCash}
                onChangeText={setSplitCash}
                placeholder={uiText.cashEur}
                keyboardType="decimal-pad"
                style={[isPhone ? { width: "100%" } : { flex: 1 }, { backgroundColor: paperTheme.colors.surface }]}
              />
              <PaperButton
                testID="payment-split"
                mode="contained-tonal"
                accessibilityLabel={uiText.splitPayment}
                disabled={busy || !cart.length || !!pendingOrder}
                style={isPhone ? { width: "100%" } : { flex: 1 }}
                contentStyle={{ minHeight: 56 }}
                onPress={() => checkout("SPLIT")}
              >{uiText.cashCard}</PaperButton>
            </View>
            <View style={s.payRow}>
              <PaperButton
                testID="sale-pay"
                mode="contained"
                accessibilityLabel={uiText.cashPayment}
                disabled={busy || !cart.length || !!pendingOrder}
                style={{ flex: 1 }}
                contentStyle={{ minHeight: 56 }}
                onPress={() => checkout("CASH")}
              >{uiText.cash}</PaperButton>
              <PaperButton
                testID="payment-card"
                mode="outlined"
                accessibilityLabel={uiText.cardPayment}
                disabled={busy || !cart.length || !!pendingOrder}
                style={{ flex: 1 }}
                contentStyle={{ minHeight: 56 }}
                onPress={() => checkout("CARD")}
              >{uiText.card}</PaperButton>
            </View>
            </NativeScrollView>
            <View testID="cart-sticky-total" style={[s.total, { borderColor: paperTheme.colors.outlineVariant, backgroundColor: paperTheme.colors.surface }]}>
              <PaperText variant="titleMedium">{uiText.total}</PaperText>
              <PaperText testID="sale-total" variant="headlineSmall">€ {total.toFixed(2)}</PaperText>
            </View>
          </View> : null}
          </View>
        </View>
      )}
      <View
        testID="status-transport"
        accessibilityLiveRegion="polite"
        style={[s.footer, { backgroundColor: paperTheme.colors.surfaceVariant }]}
      >
        <View
          style={[
            s.dot,
            (status.includes("блокирана") ||
              status.includes("Няма") ||
              status.includes("отказана")) &&
              s.dotBad,
          ]}
        />
        <PaperText variant="bodyMedium" style={[s.status, { color: paperTheme.colors.onSurfaceVariant }]}>{uiText.statusLabel}: {status}</PaperText>
      </View>
    </SafeAreaView>
  );
}

function PaperLegalLink({ testID, label, url }: { testID: string; label: string; url: string }) {
  const rootRef = useRef<View>(null);
  const labelStyle = Platform.OS === 'web'
    ? { whiteSpace: 'normal', overflowWrap: 'anywhere', flexShrink: 1 } as const
    : { flexShrink: 1 };
  useEffect(() => {
    if (Platform.OS !== 'web') return;
    // Installed RN Web treats role=link as native interactive but renders
    // a Paper-owned View. Give its actual leaf URL metadata and Enter behavior.
    const link = (rootRef.current as unknown as HTMLElement | null)?.querySelector<HTMLElement>('[role="link"]');
    link?.setAttribute('href', url);
    link?.setAttribute('target', '_blank');
    link?.setAttribute('rel', 'noopener noreferrer');
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Enter') return;
      event.preventDefault(); event.stopPropagation();
      if (!event.repeat) link?.click();
    };
    link?.addEventListener('keydown', onKeyDown);
    return () => link?.removeEventListener('keydown', onKeyDown);
  }, [url]);
  return <View ref={rootRef} style={{ minWidth: 0 }}>
    <PaperButton testID={testID} mode="outlined" accessibilityRole="link"
      contentStyle={s.authButtonContent} labelStyle={labelStyle} icon="open-in-new"
      onPress={event => { event.preventDefault(); void Linking.openURL(url); }}>{label}</PaperButton>
  </View>;
}

function usePaperMenuKeyboard(visible: boolean, prefix: "language" | "theme") {
  useEffect(() => {
    if (!visible || Platform.OS !== "web") return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
      const items = Array.from(document.querySelectorAll<HTMLElement>(`[role="menuitem"][data-testid^="${prefix}-"]`))
        .filter(item => item.getClientRects().length > 0);
      const current = items.findIndex(item => item === document.activeElement || item.contains(document.activeElement));
      if (current < 0 || !items.length) return;
      event.preventDefault();
      const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1
        : (current + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
      items[next].focus();
      items[next].scrollIntoView({ block: "nearest", inline: "nearest" });
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [visible, prefix]);
}

function PaperMenuChoice({ testID, title, onPress }: { testID: string; title: string; onPress: () => void }) {
  const theme = useTheme();
  return <PaperList.Item testID={testID} title={title} titleNumberOfLines={0}
    accessibilityRole="menuitem" accessibilityLabel={title}
    style={{ minHeight: 48, minWidth: 0, margin: 4, borderWidth: 1, borderColor: theme.colors.outline, borderRadius: theme.roundness }}
    onPress={onPress} />;
}

function ThemeMenu({
  preference,
  text,
  onSelect,
}: {
  preference: ThemePreference;
  text: Translation;
  onSelect: (value: ThemePreference) => void;
}) {
  const [visible, setVisible] = useState(false);
  usePaperMenuKeyboard(visible, "theme");
  const labels: Record<ThemePreference, string> = {
    system: text.systemTheme,
    light: text.lightTheme,
    dark: text.darkTheme,
  };
  const choose = (value: ThemePreference) => {
    setVisible(false);
    onSelect(value);
  };

  return (
    <PaperMenu
      theme={{ animation: { scale: 0 } }}
      visible={visible}
      onDismiss={() => setVisible(false)}
      anchor={
        <PaperButton
          testID="theme-menu-open"
          mode="contained-tonal"
          contentStyle={s.themeButtonContent}
          onPress={() => setVisible(true)}
        >
          {text.theme}: {labels[preference]}
        </PaperButton>
      }
    >
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
    </PaperMenu>
  );
}

function LanguageMenu({
  language,
  label,
  onSelect,
}: {
  language: Language;
  label: string;
  onSelect: (value: Language) => void;
}) {
  const text = useTranslation();
  const [visible, setVisible] = useState(false);
  usePaperMenuKeyboard(visible, "language");
  const { width, height } = useWindowDimensions();
  const menuHeight = Math.max(192, Math.min(420, height - 96));
  const menuWidth = Math.max(240, Math.min(320, width - 32));
  const labels: Record<Language, string> = {
    bg: "Български", hr: "Hrvatski", cs: "Čeština", da: "Dansk", nl: "Nederlands",
    en: "English", et: "Eesti", fi: "Suomi", fr: "Français", de: "Deutsch",
    el: "Ελληνικά", hu: "Magyar", ga: "Gaeilge", it: "Italiano", lv: "Latviešu",
    lt: "Lietuvių", mt: "Malti", pl: "Polski", pt: "Português", ro: "Română",
    sk: "Slovenčina", sl: "Slovenščina", es: "Español", sv: "Svenska", ru: "Русский",
  };
  const choose = (value: Language) => {
    setVisible(false);
    onSelect(value);
  };

  return (
    <PaperMenu
      theme={{ animation: { scale: 0 } }}
      visible={visible}
      onDismiss={() => setVisible(false)}
      contentStyle={{ maxHeight: menuHeight, width: menuWidth, overflow: "hidden" }}
      anchor={
        <PaperButton
          testID="language-menu-open"
          mode="contained-tonal"
          contentStyle={s.themeButtonContent}
          onPress={() => setVisible(true)}
        >
          {label}: {labels[language]}
        </PaperButton>
      }
    >
      <NativeScrollView testID="language-menu-scroll" style={{ maxHeight: menuHeight }} nestedScrollEnabled showsVerticalScrollIndicator keyboardShouldPersistTaps="handled">
        {supportedLanguages.map((value) => (
          <PaperMenuChoice
            key={value}
            testID={`language-${value}`}
            title={`${labels[value]}${language === value ? ` • ${text.selected}` : ""}`}
            onPress={() => choose(value)}
          />
        ))}
      </NativeScrollView>
    </PaperMenu>
  );
}

function ReferenceChoices({items,value,onChange,emptyLabel,onEmpty}:{items:FiscalRegisterReference[];value:string;onChange:(value:string)=>void;emptyLabel:string;onEmpty:()=>void}) {
  const text = useTranslation();
  const [query,setQuery]=useState("");
  const visible=items.filter((item)=>`${item.code || ""} ${item.name || ""}`.toLowerCase().includes(query.trim().toLowerCase()));
  if (!items.length) return <View style={s.stackCompact}><PaperText variant="bodyMedium">{emptyLabel}</PaperText><PaperButton mode="outlined" onPress={onEmpty}>Заполнить справочник кассовых мест</PaperButton></View>;
  return <View style={s.stackCompact}>
    <TextInput placeholder="Поиск кассового места" value={query} onChangeText={setQuery}/>
    {visible.map((item) => <PaperButton
      key={item.id}
      testID={`register-choice-${item.id}`}
      mode={item.id === value ? "contained-tonal" : "outlined"}
      accessibilityState={{ selected: item.id === value }}
      onPress={() => onChange(item.id)}
    >{`${item.code || item.name || "Кассовое место"}${item.id === value ? ` • ${text.selected}` : ""}`}</PaperButton>)}
    {!visible.length ? <PaperText variant="bodyMedium">Совпадений нет.</PaperText> : null}
  </View>;
}

function Admin({
  compact,
  production,
  products,
  taxGroups,
  employees,
  selectedEmployeeId,
  salesReport,
  fiscalRegisters,
  values,
  setters,
  createProduct,
  createEmployee,
  saveTaxGroup,
  saveConfiguration,
  loadReport,
}: any) {
  const text = useTranslation();
  const paperTheme = useTheme();
  const [orders, setOrders] = useState<Order[]>([]),
    [reversalStatus, setReversalStatus] = useState(""),
    [catalogScreen, setCatalogScreen] = useState<"home"|"products"|"product-new"|"tax-groups"|"tax-group-edit"|"employees"|"employee-new">("home"),
    [editingTaxGroup, setEditingTaxGroup] = useState<TaxGroup | null>(null),
    [taxCode, setTaxCode] = useState(""),
    [taxName, setTaxName] = useState(""),
    [taxRate, setTaxRate] = useState("");
  const editTaxGroup = (group?: TaxGroup) => {
    setEditingTaxGroup(group || null);
    setTaxCode(group?.code || "");
    setTaxName(group?.name || "");
    setTaxRate(group?.rate || "");
    setCatalogScreen("tax-group-edit");
  };
  const loadOrders = async () => {
    try {
      setOrders(await collect<Order>("/orders"));
    } catch (e) {
      setReversalStatus(message(e));
    }
  };
  useEffect(() => {
    void loadOrders();
  }, []);
  if (catalogScreen !== "home") return <ScrollView contentContainerStyle={[s.admin, compact && s.adminCompact]}>
    <PaperButton testID="admin-catalog-back" mode="outlined" contentStyle={{ minHeight: 48 }} onPress={() => setCatalogScreen("home")}>← {text.back}</PaperButton>
    {catalogScreen === "products" ? <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
      <View style={s.screenHeading}>
        <PaperText variant="titleLarge" accessibilityRole="header">{text.products}</PaperText>
        <PaperButton testID="product-add" mode="contained" onPress={() => setCatalogScreen("product-new")}>{text.addProduct}</PaperButton>
      </View>
      {products.map((p: Product) => <PaperList.Item key={p.id} title={p.name} description={`€ ${p.price.amount} • ${text.vat} ${p.tax_group}`} />)}
    </PaperCard.Content></PaperCard> : null}
    {catalogScreen === "product-new" ? <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
      <PaperText variant="titleLarge" accessibilityRole="header">{text.newProduct}</PaperText>
      <TextInput testID="product-name" style={s.search} placeholder={text.name} value={values.pName} onChangeText={setters.setPName}/>
      <TextInput testID="product-barcode" style={s.search} placeholder={text.barcode} value={values.pBarcode} onChangeText={setters.setPBarcode}/>
      <TextInput testID="product-price" style={s.search} placeholder={text.priceEur} keyboardType="decimal-pad" value={values.pPrice} onChangeText={setters.setPPrice}/>
      <PaperText variant="titleMedium">{text.vatGroup}</PaperText>
      {taxGroups.filter((group: TaxGroup) => group.status === "ACTIVE").map((group: TaxGroup) => <PaperButton
        key={group.id} mode={values.pTaxGroup === group.code ? "contained-tonal" : "outlined"}
        accessibilityState={{ selected: values.pTaxGroup === group.code }}
        onPress={() => setters.setPTaxGroup(group.code)}
      >{`${group.code} • ${group.name} • ${group.rate}%${values.pTaxGroup === group.code ? ` • ${text.selected}` : ""}`}</PaperButton>)}
      {!taxGroups.some((group: TaxGroup) => group.status === "ACTIVE") ? <PaperButton mode="outlined" onPress={() => setCatalogScreen("tax-groups")}>{text.configureTaxGroups}</PaperButton> : null}
      <PaperButton testID="product-create" mode="contained" disabled={!values.pTaxGroup} onPress={async () => { await createProduct(); setCatalogScreen("products"); }}>{text.save}</PaperButton>
    </PaperCard.Content></PaperCard> : null}
    {catalogScreen === "tax-groups" ? <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
      <View style={s.screenHeading}>
        <PaperText variant="titleLarge" accessibilityRole="header">{text.taxGroups}</PaperText>
        <PaperButton testID="tax-group-add" mode="contained" onPress={() => editTaxGroup()}>{text.newTaxGroup}</PaperButton>
      </View>
      {taxGroups.map((group: TaxGroup) => <PaperList.Item key={group.id} title={`${group.code} • ${group.name}`}
        description={`${group.rate}% • ${group.status === "ACTIVE" ? text.active : text.inactive}`}
        accessibilityRole="button" accessibilityLabel={`${group.code} • ${group.name} • ${group.rate}% • ${group.status === "ACTIVE" ? text.active : text.inactive}`}
        titleNumberOfLines={0} descriptionNumberOfLines={0}
        style={{ borderWidth: 1, borderColor: paperTheme.colors.outline, borderRadius: paperTheme.roundness, minHeight: 48 }}
        onPress={() => editTaxGroup(group)} />)}
    </PaperCard.Content></PaperCard> : null}
    {catalogScreen === "tax-group-edit" ? <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
      <PaperText variant="titleLarge" accessibilityRole="header">{editingTaxGroup ? text.editTaxGroup : text.newTaxGroup}</PaperText>
      <TextInput testID="tax-group-code" style={s.search} placeholder={text.taxCode} maxLength={1} autoCapitalize="characters" value={taxCode} onChangeText={setTaxCode}/>
      <TextInput testID="tax-group-name" style={s.search} placeholder={text.name} value={taxName} onChangeText={setTaxName}/>
      <TextInput testID="tax-group-rate" style={s.search} placeholder={text.taxRate} keyboardType="decimal-pad" value={taxRate} onChangeText={setTaxRate}/>
      <PaperButton testID="tax-group-save" mode="contained" onPress={async () => { const saved = await saveTaxGroup({ ...editingTaxGroup, code: taxCode, name: taxName, rate: taxRate, status: editingTaxGroup?.status || "ACTIVE" }); if (saved) setCatalogScreen("tax-groups"); }}>{text.save}</PaperButton>
    </PaperCard.Content></PaperCard> : null}
    {catalogScreen === "employees" ? <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
      <View style={s.screenHeading}>
        <PaperText variant="titleLarge" accessibilityRole="header">{text.employees}</PaperText>
        <PaperButton testID="employee-add" mode="contained" onPress={() => setCatalogScreen("employee-new")}>{text.addEmployee}</PaperButton>
      </View>
      {employees.map((e: Employee) => <PaperList.Item key={e.id} title={`${e.first_name} ${e.last_name}`} description={e.operator_code} />)}
    </PaperCard.Content></PaperCard> : null}
    {catalogScreen === "employee-new" ? <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
      <PaperText variant="titleLarge" accessibilityRole="header">{text.newEmployee}</PaperText>
      <TextInput testID="employee-first-name" style={s.search} placeholder={text.firstName} value={values.eFirst} onChangeText={setters.setEFirst}/>
      <TextInput testID="employee-last-name" style={s.search} placeholder={text.lastName} value={values.eLast} onChangeText={setters.setELast}/>
      <TextInput testID="employee-code" style={s.search} placeholder={text.operatorCode} maxLength={4} value={values.eCode} onChangeText={setters.setECode}/>
      <PaperButton testID="employee-create" mode="contained" onPress={async () => { await createEmployee(); setCatalogScreen("employees"); }}>{text.save}</PaperButton>
    </PaperCard.Content></PaperCard> : null}
  </ScrollView>;
  const reverse = async (order: Order) => {
    if (!order.allowed_actions?.includes("REVERSE")) {
      setReversalStatus("Сторното е блокирано: backend не го разрешава");
      return;
    }
    setReversalStatus(`Сторно на ${order.id}…`);
    try {
      const result = await call<{
        operation_id: string;
        state: string;
        fiscal_reference?: string;
      }>(`/orders/${order.id}/reversals`, {
        method: "POST",
        body: JSON.stringify({ reason_code: "CUSTOMER_RETURN" }),
      });
      if (result.state !== "FISCALIZED")
        throw new Error(`непотвърден резултат ${result.state}`);
      setReversalStatus(
        `Сторно бон • ${result.fiscal_reference || result.operation_id}`,
      );
      await loadOrders();
    } catch (e) {
      setReversalStatus(
        `Няма потвърден успех; не повтаряйте сторното: ${message(e)}`,
      );
    }
  };
  return (
    <ScrollView testID="admin-editors" contentContainerStyle={[s.admin, compact && s.adminCompact]}>
      <PaperText variant="headlineSmall" accessibilityRole="header">{text.configuration}</PaperText>
      <View style={[s.screenHeading, s.stackCompact]}>
        <PaperButton testID="admin-products" mode="outlined" onPress={() => setCatalogScreen("products")}>{text.products} ({products.length})</PaperButton>
        <PaperButton testID="admin-tax-groups" mode="outlined" onPress={() => setCatalogScreen("tax-groups")}>{text.taxGroups} ({taxGroups.length})</PaperButton>
        <PaperButton testID="admin-employees" mode="outlined" onPress={() => setCatalogScreen("employees")}>{text.employees} ({employees.length})</PaperButton>
      </View>
      <PaperCard testID="reversal-editor" mode="outlined"><PaperCard.Content style={s.managementContent}>
        <PaperText variant="titleLarge" accessibilityRole="header">{text.reversal}</PaperText>
        <PaperText variant="bodyMedium">{reversalStatus || text.selectReversal}</PaperText>
        {orders
          .filter(
            (order) =>
              order.state === "COMPLETED" || order.state === "REVERSED",
          )
          .map((order) => (
            <View key={order.id} style={[s.line, { borderColor: paperTheme.colors.outlineVariant }]}>
              <View>
                <PaperText variant="titleMedium">{order.id}</PaperText>
                <PaperText variant="bodyMedium">
                  {order.state}
                  {order.receipt_reference
                    ? ` • бон ${order.receipt_reference}`
                    : ""}
                  {order.reversal_fiscal_reference
                    ? ` • сторно ${order.reversal_fiscal_reference}`
                    : ""}
                </PaperText>
              </View>
              {order.allowed_actions?.includes("REVERSE") ? (
                <PaperButton
                  testID={`reversal-${order.id}`}
                  mode="outlined"
                  accessibilityLabel={`${text.reverseSale} ${order.id}`}
                  onPress={() => void reverse(order)}
                >{text.returnSale}</PaperButton>
              ) : null}
            </View>
          ))}
      </PaperCard.Content></PaperCard>
      <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
        <PaperText variant="titleLarge" accessibilityRole="header">{text.locationWorkstation}</PaperText>
        <TextInput
          testID="config-location-name"
          accessibilityLabel={text.locationName}
          style={s.search}
          placeholder={text.locationName}
          value={values.locationName}
          onChangeText={setters.setLocationName}
        />
        <TextInput
          style={s.search}
          placeholder={text.address}
          value={values.locationAddress}
          onChangeText={setters.setLocationAddress}
        />
        <TextInput
          style={s.search}
          placeholder={text.workstationName}
          value={values.workstationName}
          onChangeText={setters.setWorkstationName}
        />
        <ReferenceChoices
          items={fiscalRegisters}
          value={values.configuredRegister}
          onChange={setters.setConfiguredRegister}
          emptyLabel="Нет доступных кассовых мест."
          onEmpty={() => void Linking.openURL(process.env.EXPO_PUBLIC_BEEFISCAL_ADMIN_URL || "beefiscal://admin/registers")}
        />
        <PaperButton
          testID="config-save"
          mode="contained"
          accessibilityLabel={text.saveConfiguration}
          onPress={saveConfiguration}
        >{text.saveConfiguration}</PaperButton>
      </PaperCard.Content></PaperCard>
      <PaperCard mode="outlined"><PaperCard.Content style={s.managementContent}>
        <PaperText variant="titleLarge" accessibilityRole="header">{text.salesReport}</PaperText>
        <PaperButton
          testID="sales-report-load"
          mode="outlined"
          accessibilityLabel={text.refreshReport}
          onPress={loadReport}
        >{text.refreshReport}</PaperButton>
        {salesReport && (
          <View testID="sales-report-result" style={s.report}>
            <PaperText variant="headlineSmall">€ {salesReport.gross.amount}</PaperText>
            <PaperText variant="bodyMedium">
              {salesReport.from} — {salesReport.to}
            </PaperText>
            {salesReport.payments.map((payment: any) => (
              <PaperText key={payment.type} variant="bodyMedium">
                {payment.type}: € {payment.amount.amount}
              </PaperText>
            ))}
          </View>
        )}
      </PaperCard.Content></PaperCard>
    </ScrollView>
  );
}
const reconciliationStyles = StyleSheet.create({
  panel: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 16,
  },
  panelPhone: { flexDirection: "column", alignItems: "stretch", padding: 12 },
});
function message(e: unknown) {
  return e instanceof Error ? e.message : String(e);
}
const s = StyleSheet.create({
  root: { flex: 1 },
  authRoot: {
    width: "100%",
    minHeight: "100%",
    alignItems: "center",
  },
  authScroll: { flex: 1, width: "100%" },
  authScrollContent: { alignItems: "center", paddingBottom: 32 },
  authHeader: { width: "100%", flexDirection: "row", flexWrap: "wrap", justifyContent: "flex-end", gap: 8, padding: 16, zIndex: 2 },
  authBackground: {
    ...StyleSheet.absoluteFillObject,
    width: "100%",
    height: "100%",
    opacity: 0.7,
  },
  authForm: { marginTop: 48 },
  loginCard: { borderRadius: 16 },
  loginContent: { padding: 24, gap: 16 },
  authButtonContent: { minHeight: 56 },
  top: {
    padding: 18,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    borderBottomWidth: 1,
  },
  topCompact: { alignItems: "stretch", flexDirection: "column", gap: 12, padding: 12 },
  topIdentity: { flexShrink: 1 },
  topActions: {
    flexDirection: "row",
    gap: 10,
    alignItems: "center",
    flexWrap: "wrap",
  },
  topActionsCompact: { width: "100%" },
  themeButtonContent: { minHeight: 48 },
  posShell: { flex: 1 },
  body: { flex: 1, flexDirection: "row" },
  bodyPhone: { flexDirection: "column" },
  panelPhone: { flex: 1, width: "100%", padding: 12 },
  mobileTabs: {
    paddingHorizontal: 12,
    paddingTop: 10,
  },
  catalog: { flex: 3, padding: 16 },
  search: {
    marginBottom: 14,
    minHeight: 48,
  },
  managementContent: { padding: 16, gap: 12 },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 12 },
  product: {
    width: "47%",
    minHeight: 120,
  },
  productPhone: { width: "48%", minHeight: 108 },
  productContent: { flex: 1, justifyContent: "space-between", gap: 4 },
  cart: {
    flex: 2,
    borderLeftWidth: 1,
  },
  cartPhone: { flex: 1, width: "100%", borderLeftWidth: 0 },
  cartContent: { flexGrow: 1, padding: 16 },
  cartContentPhone: { padding: 12 },
  lines: { flexGrow: 1, marginVertical: 12 },
  empty: { textAlign: "center", marginTop: 40 },
  line: {
    flexDirection: "row",
    justifyContent: "space-between",
    paddingVertical: 12,
    borderBottomWidth: 1,
  },
  lineContent: { flex: 1, paddingRight: 10 },
  quantity: { flexDirection: "row", gap: 8, marginTop: 8 },
  discountRow: { flexDirection: "row", gap: 8, marginTop: 8 },
  total: {
    flexDirection: "row",
    justifyContent: "space-between",
    paddingVertical: 16,
    paddingHorizontal: 16,
    borderTopWidth: 2,
  },
  splitRow: { flexDirection: "row", gap: 10, marginBottom: 10 },
  splitRowPhone: { flexDirection: "column" },
  payRow: { flexDirection: "row", gap: 10 },
  footer: {
    padding: 12,
    flexDirection: "row",
    alignItems: "center",
  },
  dot: {
    width: 10,
    height: 10,
    borderRadius: 5,
    backgroundColor: beeloyColors.success,
    marginRight: 8,
  },
  dotBad: { backgroundColor: beeloyColors.error },
  status: { flex: 1 },
  admin: { padding: 20, gap: 16 },
  adminCompact: { padding: 12 },
  screenHeading: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 12 },
  stackCompact: { flexDirection: "column", alignItems: "stretch", width: "100%" },
  report: { paddingTop: 14, gap: 5 },
});
