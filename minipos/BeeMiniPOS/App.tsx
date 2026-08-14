import { StatusBar } from "expo-status-bar";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  Platform,
  Pressable,
  SafeAreaView,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
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
import { useOperatorOidc } from "./src/operatorOidc";
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
  if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
  return text ? JSON.parse(text) : ({} as T);
}
const collect = <T,>(path: string) =>
  collectCursorPages<T>(path, (p) => call(p));
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

export default function App() {
  const connectivity = useRef(new ConnectivityController());
  const checkoutJournal = useRef(new CheckoutJournal());
  const oidc = useOperatorOidc();
  runtimeUnauthorized = () => oidc.logout();
  const [products, setProducts] = useState<Product[]>([]),
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
    [pendingOrder, setPendingOrder] = useState<Order | null>(null);
  const [pName, setPName] = useState(""),
    [pPrice, setPPrice] = useState(""),
    [pBarcode, setPBarcode] = useState(""),
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
      const [p, e] = await Promise.all([
        collect<Product>("/products"),
        employeeRequest,
      ]);
      setProducts(p.filter((x) => x.active));
      setEmployees(e);
      const employee =
        session?.employee || e.find((x) => x.id === selectedEmployeeId) || e[0];
      if (session) {
        setOperatorEmployee(session.employee);
        setSelectedEmployeeId(session.employee.id);
      } else setSelectedEmployeeId((v) => v || employee?.id || "");
      const cfg = await call<Configuration>("/configuration").catch(() => null);
      if (cfg) {
        setConfiguration(cfg);
        setLocationName(cfg.location_name);
        setLocationAddress(cfg.location_address);
        setWorkstationName(cfg.workstation_name);
        setConfiguredRegister(cfg.fiscal_register_id);
      }
      const recoveryRegister =
        cfg?.fiscal_register_id || configuredRegister || registerId;
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
    if (prodMode && !oidc.accessToken) {
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
    runtimeAuthToken = oidc.accessToken || devAuthToken;
    runtimeFiscalToken = oidc.accessToken || devFiscalToken;
    void refresh();
  }, [oidc.accessToken]);
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
        if (!selectedEmployee) throw new Error("Създайте и изберете служител");
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
      setStatus(`Операцията е отказана: ${message(e)}`);
      await refresh();
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
          tax_group: "B",
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
      oidc.logout();
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
  if (prodMode && !oidc.accessToken)
    return (
      <SafeAreaView style={s.root}>
        <StatusBar style="dark" />
        <View testID="operator-login" style={s.login}>
          <Text style={s.brand}>BeeMiniPOS</Text>
          <Text style={s.loginTitle}>Персонален вход на оператор</Text>
          <Text style={s.loginText}>
            Входът използва OIDC Authorization Code + PKCE. Споделен касиерски
            профил и избор на служител не са разрешени.
          </Text>
          {!oidc.configured ? (
            <Text style={s.loginError}>
              OIDC не е конфигуриран: задайте issuer и client ID.
            </Text>
          ) : null}
          {oidc.error ? <Text style={s.loginError}>{oidc.error}</Text> : null}
          <Pressable
            testID="operator-login-start"
            accessibilityRole="button"
            accessibilityLabel="Влез с персонален профил"
            accessibilityState={{ disabled: !oidc.ready }}
            disabled={!oidc.ready}
            style={s.cash}
            onPress={() => void oidc.login()}
          >
            <Text style={s.payText}>Влез</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  return (
    <SafeAreaView style={s.root}>
      <StatusBar style="dark" />
      <View style={s.top}>
        <View>
          <Text style={s.brand}>BeeMiniPOS</Text>
          <Text style={s.caption}>
            {configuration?.location_name || "1 точка"} •{" "}
            {configuration?.workstation_name || "каса 01"} • {activeRegisterId}{" "}
            •{" "}
            {selectedEmployee
              ? `${selectedEmployee.first_name} ${selectedEmployee.last_name}`
              : "DEV"}{" "}
            • EUR • BLE {bleReady ? "READY" : "OFF"}
          </Text>
        </View>
        <View style={s.topActions}>
          {prodMode ? (
            <Pressable
              testID="operator-logout"
              accessibilityRole="button"
              accessibilityLabel="Излез от операторската сесия"
              accessibilityState={{ disabled: !!shift }}
              disabled={!!shift}
              style={s.adminButton}
              onPress={() => void logoutOperator()}
            >
              <Text>Излез</Text>
            </Pressable>
          ) : null}
          <Pressable
            testID="ble-connect"
            accessibilityRole="button"
            accessibilityLabel={
              bleReady ? "BLE връзката е готова" : "Свържи BLE"
            }
            accessibilityState={{ disabled: busy || bleReady }}
            disabled={busy || bleReady}
            style={s.adminButton}
            onPress={connectBle}
          >
            <Text>{bleReady ? "BLE свързан" : "Свържи BLE"}</Text>
          </Pressable>
          {canManage ? (
            <Pressable
              testID="admin-toggle"
              accessibilityRole="button"
              accessibilityLabel={admin ? "Отвори продажби" : "Отвори обекти"}
              style={s.adminButton}
              onPress={() => setAdmin((v) => !v)}
            >
              <Text>{admin ? "Продажби" : "Обекти"}</Text>
            </Pressable>
          ) : null}
          <Pressable
            testID="shift-toggle"
            accessibilityRole="button"
            accessibilityLabel={
              shift?.allowed_actions?.includes("RECONCILE")
                ? "Свери Z отчет"
                : shift
                  ? "Затвори смяна"
                  : "Отвори смяна"
            }
            accessibilityState={{ disabled: busy || !selectedEmployee }}
            disabled={busy || !selectedEmployee}
            style={[s.shift, shift && s.shiftOpen]}
            onPress={toggleShift}
          >
            <Text style={s.shiftText}>
              {shift?.allowed_actions?.includes("RECONCILE")
                ? "Свери Z"
                : shift
                  ? "Затвори смяна"
                  : "Отвори смяна"}
            </Text>
          </Pressable>
        </View>
      </View>
      {pendingOrder ? (
        <View
          testID="operation-unknown"
          accessibilityLiveRegion="assertive"
          style={reconciliationStyles.panel}
        >
          <View>
            <Text style={reconciliationStyles.title}>
              Резултатът се проверява
            </Text>
            <Text style={reconciliationStyles.text}>
              Не повтаряйте плащането. Поръчка {pendingOrder.id} •{" "}
              {pendingOrder.state}
            </Text>
            <Text style={reconciliationStyles.text}>
              Разрешени действия:{" "}
              {pendingOrder.allowed_actions?.join(", ") || "няма"}
            </Text>
          </View>
          {pendingOrder.allowed_actions?.includes("READ") ? (
            <Pressable
              testID="reconcile-start"
              accessibilityRole="button"
              accessibilityLabel="Провери фискалния резултат без повторно плащане"
              accessibilityState={{ disabled: busy }}
              disabled={busy}
              style={reconciliationStyles.button}
              onPress={reconcile}
            >
              <Text style={s.payText}>Провери резултата</Text>
            </Pressable>
          ) : null}
        </View>
      ) : null}
      {admin ? (
        <Admin
          production={prodMode}
          products={products}
          employees={employees}
          selectedEmployeeId={selectedEmployee?.id || ""}
          salesReport={salesReport}
          values={{
            pName,
            pPrice,
            pBarcode,
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
          saveConfiguration={saveConfiguration}
          loadReport={loadReport}
        />
      ) : (
        <View style={s.body}>
          <View style={s.catalog}>
            <TextInput
              testID="product-search"
              accessibilityLabel="Търсене или баркод"
              value={search}
              onChangeText={setSearch}
              onSubmitEditing={submitProductLookup}
              returnKeyType="done"
              placeholder="Търсене или сканиране на баркод"
              style={s.search}
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
                  <Pressable
                    testID={`product-${p.id}`}
                    accessibilityRole="button"
                    accessibilityLabel={`Добави ${p.name}`}
                    key={p.id}
                    style={s.product}
                    onPress={() => add(p)}
                  >
                    <Text style={s.productName}>{p.name}</Text>
                    <Text style={s.price}>
                      € {Number(p.price.amount).toFixed(2)}
                    </Text>
                    <Text style={s.tax}>ДДС {p.tax_group}</Text>
                  </Pressable>
                ))}
            </ScrollView>
          </View>
          <View style={s.cart}>
            <Text style={s.cartTitle}>Текуща продажба</Text>
            {saleProjection ? (
              <View
                testID="sale-regulatory-identity"
                accessibilityLiveRegion="polite"
              >
                <Text>
                  УНП:{" "}
                  {saleProjection.regulatory_identifiers[0]?.value ||
                    saleProjection.unp}
                </Text>
                <Text>ФУ/каса: {saleProjection.register_id}</Text>
                <Text>Състояние: {saleProjection.state}</Text>
              </View>
            ) : null}
            <ScrollView style={s.lines}>
              {cart.length === 0 ? (
                <Text style={s.empty}>Докоснете продукт</Text>
              ) : (
                cart.map((x) => (
                  <View
                    testID={`cart-line-${x.product.id}`}
                    style={s.line}
                    key={x.product.id}
                  >
                    <View>
                      <Text style={s.lineName}>
                        {x.quantity} × {x.product.name}
                      </Text>
                      <View style={s.quantity}>
                        <Pressable
                          testID={`quantity-dec-${x.product.id}`}
                          accessibilityRole="button"
                          accessibilityLabel={`Намали ${x.product.name}`}
                          style={s.qtyButton}
                          onPress={() => changeQuantity(x.product.id, -1)}
                        >
                          <Text>−</Text>
                        </Pressable>
                        <Pressable
                          testID={`quantity-inc-${x.product.id}`}
                          accessibilityRole="button"
                          accessibilityLabel={`Увеличи ${x.product.name}`}
                          style={s.qtyButton}
                          onPress={() => changeQuantity(x.product.id, 1)}
                        >
                          <Text>+</Text>
                        </Pressable>
                      </View>
                      {canManage ? (
                        <View style={s.discountRow}>
                          <Pressable
                            testID={`discount-percent-${x.product.id}`}
                            accessibilityRole="button"
                            accessibilityLabel={`Отстъпка 10 процента за ${x.product.name}`}
                            style={s.discountButton}
                            onPress={() =>
                              setLineDiscount(
                                x.product.id,
                                Number(x.product.price.amount) *
                                  x.quantity *
                                  0.1,
                              )
                            }
                          >
                            <Text>−10%</Text>
                          </Pressable>
                          <TextInput
                            testID={`discount-amount-${x.product.id}`}
                            accessibilityLabel={`Отстъпка в евро за ${x.product.name}`}
                            style={s.discountInput}
                            keyboardType="decimal-pad"
                            value={x.discountAmount ?? "0.00"}
                            onChangeText={(value) =>
                              setLineDiscountText(x.product.id, value)
                            }
                          />
                        </View>
                      ) : null}
                    </View>
                    <Text style={s.linePrice}>
                      €{" "}
                      {(
                        Number(x.product.price.amount) * x.quantity -
                        Number(x.discountAmount || "0")
                      ).toFixed(2)}
                    </Text>
                  </View>
                ))
              )}
            </ScrollView>
            <View style={s.total}>
              <Text style={s.totalLabel}>ОБЩО</Text>
              <Text testID="sale-total" style={s.totalValue}>
                € {total.toFixed(2)}
              </Text>
            </View>
            <View style={s.splitRow}>
              <TextInput
                testID="split-cash-amount"
                accessibilityLabel="Сума в брой при разделено плащане"
                value={splitCash}
                onChangeText={setSplitCash}
                placeholder="В брой EUR"
                keyboardType="decimal-pad"
                style={s.splitInput}
              />
              <Pressable
                testID="payment-split"
                accessibilityRole="button"
                accessibilityLabel="Разделено плащане в брой и с карта"
                accessibilityState={{
                  disabled: busy || !cart.length || !!pendingOrder,
                }}
                disabled={busy || !cart.length || !!pendingOrder}
                style={s.split}
                onPress={() => checkout("SPLIT")}
              >
                <Text style={s.payText}>В брой + карта</Text>
              </Pressable>
            </View>
            <View style={s.payRow}>
              <Pressable
                testID="sale-pay"
                accessibilityRole="button"
                accessibilityLabel="Плащане в брой"
                accessibilityState={{
                  disabled: busy || !cart.length || !!pendingOrder,
                }}
                disabled={busy || !cart.length || !!pendingOrder}
                style={s.cash}
                onPress={() => checkout("CASH")}
              >
                <Text style={s.payText}>В брой</Text>
              </Pressable>
              <Pressable
                testID="payment-card"
                accessibilityRole="button"
                accessibilityLabel="Плащане с карта"
                accessibilityState={{
                  disabled: busy || !cart.length || !!pendingOrder,
                }}
                disabled={busy || !cart.length || !!pendingOrder}
                style={s.card}
                onPress={() => checkout("CARD")}
              >
                <Text style={s.payText}>Карта</Text>
              </Pressable>
            </View>
          </View>
        </View>
      )}
      <View
        testID="status-transport"
        accessibilityRole="text"
        accessibilityLiveRegion="polite"
        accessibilityLabel={`Състояние: ${status}`}
        style={s.footer}
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
        <Text style={s.status}>{status}</Text>
      </View>
    </SafeAreaView>
  );
}
function Admin({
  production,
  products,
  employees,
  selectedEmployeeId,
  salesReport,
  values,
  setters,
  createProduct,
  createEmployee,
  saveConfiguration,
  loadReport,
}: any) {
  const [orders, setOrders] = useState<Order[]>([]),
    [reversalStatus, setReversalStatus] = useState("");
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
    <ScrollView testID="admin-editors" contentContainerStyle={s.admin}>
      <Text style={s.adminTitle}>Минимални редактори и отчети</Text>
      <View testID="reversal-editor" style={s.editor}>
        <Text style={s.cartTitle}>Сторно на завършена продажба</Text>
        <Text>
          {reversalStatus || "Изберете продажба с разрешено действие REVERSE"}
        </Text>
        {orders
          .filter(
            (order) =>
              order.state === "COMPLETED" || order.state === "REVERSED",
          )
          .map((order) => (
            <View key={order.id} style={s.line}>
              <View>
                <Text>{order.id}</Text>
                <Text>
                  {order.state}
                  {order.receipt_reference
                    ? ` • бон ${order.receipt_reference}`
                    : ""}
                  {order.reversal_fiscal_reference
                    ? ` • сторно ${order.reversal_fiscal_reference}`
                    : ""}
                </Text>
              </View>
              {order.allowed_actions?.includes("REVERSE") ? (
                <Pressable
                  testID={`reversal-${order.id}`}
                  accessibilityRole="button"
                  accessibilityLabel={`Сторно на продажба ${order.id}`}
                  style={s.card}
                  onPress={() => void reverse(order)}
                >
                  <Text style={s.payText}>Сторно • връщане</Text>
                </Pressable>
              ) : null}
            </View>
          ))}
      </View>
      <View style={s.editor}>
        <Text style={s.cartTitle}>Точка и касово място</Text>
        <TextInput
          testID="config-location-name"
          accessibilityLabel="Име на точката"
          style={s.search}
          placeholder="Име на точката"
          value={values.locationName}
          onChangeText={setters.setLocationName}
        />
        <TextInput
          style={s.search}
          placeholder="Адрес"
          value={values.locationAddress}
          onChangeText={setters.setLocationAddress}
        />
        <TextInput
          style={s.search}
          placeholder="Име на касата"
          value={values.workstationName}
          onChangeText={setters.setWorkstationName}
        />
        <TextInput
          style={s.search}
          placeholder="Fiscal register ID"
          value={values.configuredRegister}
          onChangeText={setters.setConfiguredRegister}
        />
        <Pressable
          testID="config-save"
          accessibilityRole="button"
          accessibilityLabel="Запиши конфигурацията"
          style={s.cash}
          onPress={saveConfiguration}
        >
          <Text style={s.payText}>Запиши конфигурацията</Text>
        </Pressable>
      </View>
      <View style={s.editor}>
        <Text style={s.cartTitle}>Продукт ({products.length})</Text>
        <TextInput
          testID="product-name"
          accessibilityLabel="Име на продукт"
          style={s.search}
          placeholder="Име"
          value={values.pName}
          onChangeText={setters.setPName}
        />
        <TextInput
          testID="product-barcode"
          accessibilityLabel="Баркод на продукт"
          style={s.search}
          placeholder="Баркод (по избор)"
          value={values.pBarcode}
          onChangeText={setters.setPBarcode}
        />
        <TextInput
          testID="product-price"
          accessibilityLabel="Цена EUR"
          style={s.search}
          placeholder="Цена EUR"
          keyboardType="decimal-pad"
          value={values.pPrice}
          onChangeText={setters.setPPrice}
        />
        <Pressable
          testID="product-create"
          accessibilityRole="button"
          accessibilityLabel="Добави продукт"
          style={s.cash}
          onPress={createProduct}
        >
          <Text style={s.payText}>Добави продукт</Text>
        </Pressable>
      </View>
      <View style={s.editor}>
        <Text style={s.cartTitle}>Служител ({employees.length})</Text>
        {production ? (
          <Text style={s.loginText}>
            Активният оператор идва само от OIDC сесията; изборът на друг
            служител е забранен.
          </Text>
        ) : null}
        <View style={s.employeeList}>
          {employees.map((employee: Employee) => (
            <Pressable
              key={employee.id}
              disabled={production}
              accessibilityState={{ disabled: production }}
              style={[
                s.employee,
                selectedEmployeeId === employee.id && s.employeeSelected,
              ]}
              onPress={() => setters.setSelectedEmployeeId(employee.id)}
            >
              <Text>
                {employee.first_name} {employee.last_name} •{" "}
                {employee.operator_code}
              </Text>
            </Pressable>
          ))}
        </View>
        <TextInput
          testID="employee-first-name"
          accessibilityLabel="Име на служител"
          style={s.search}
          placeholder="Име"
          value={values.eFirst}
          onChangeText={setters.setEFirst}
        />
        <TextInput
          testID="employee-last-name"
          accessibilityLabel="Фамилия на служител"
          style={s.search}
          placeholder="Фамилия"
          value={values.eLast}
          onChangeText={setters.setELast}
        />
        <TextInput
          testID="employee-code"
          accessibilityLabel="Код на служител"
          style={s.search}
          maxLength={4}
          placeholder="Код (4)"
          value={values.eCode}
          onChangeText={setters.setECode}
        />
        <Pressable
          testID="employee-create"
          accessibilityRole="button"
          accessibilityLabel="Добави служител"
          style={s.card}
          onPress={createEmployee}
        >
          <Text style={s.payText}>Добави служител</Text>
        </Pressable>
      </View>
      <View style={s.editor}>
        <Text style={s.cartTitle}>Търговски отчет</Text>
        <Pressable
          testID="sales-report-load"
          accessibilityRole="button"
          accessibilityLabel="Обнови търговския отчет"
          style={s.card}
          onPress={loadReport}
        >
          <Text style={s.payText}>Обнови отчета</Text>
        </Pressable>
        {salesReport && (
          <View testID="sales-report-result" style={s.report}>
            <Text style={s.totalValue}>€ {salesReport.gross.amount}</Text>
            <Text>
              {salesReport.from} — {salesReport.to}
            </Text>
            {salesReport.payments.map((payment: any) => (
              <Text key={payment.type}>
                {payment.type}: € {payment.amount.amount}
              </Text>
            ))}
          </View>
        )}
      </View>
    </ScrollView>
  );
}
const reconciliationStyles = StyleSheet.create({
  panel: {
    backgroundColor: "#fff3cd",
    borderColor: "#9a6b00",
    borderWidth: 2,
    padding: 16,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 16,
  },
  title: { fontSize: 18, fontWeight: "900", color: "#5f4200" },
  text: { color: "#5f4200", marginTop: 4 },
  button: {
    backgroundColor: "#7a5500",
    paddingHorizontal: 18,
    minHeight: 56,
    borderRadius: 12,
    alignItems: "center",
    justifyContent: "center",
  },
});
function message(e: unknown) {
  return e instanceof Error ? e.message : String(e);
}
const s = StyleSheet.create({
  root: { flex: 1, backgroundColor: "#f3f1ea" },
  login: {
    alignSelf: "center",
    width: "90%",
    maxWidth: 520,
    marginTop: 80,
    padding: 28,
    backgroundColor: "white",
    borderRadius: 16,
    gap: 18,
  },
  loginTitle: { fontSize: 24, fontWeight: "800", color: "#17251d" },
  loginText: {
    fontSize: 16,
    color: "#526057",
    lineHeight: 23,
    marginBottom: 10,
  },
  loginError: { fontSize: 15, color: "#9a2f24", fontWeight: "700" },
  top: {
    padding: 18,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    borderBottomWidth: 1,
    borderColor: "#d8d4c7",
  },
  brand: { fontSize: 28, fontWeight: "800", color: "#17251d" },
  caption: { color: "#637067", marginTop: 2 },
  topActions: {
    flexDirection: "row",
    gap: 10,
    alignItems: "center",
    flexWrap: "wrap",
  },
  adminButton: {
    backgroundColor: "#e8e4d8",
    padding: 12,
    borderRadius: 10,
    minHeight: 48,
    justifyContent: "center",
  },
  shift: {
    backgroundColor: "#39453e",
    padding: 12,
    borderRadius: 10,
    minHeight: 48,
    justifyContent: "center",
  },
  shiftOpen: { backgroundColor: "#99573f" },
  shiftText: { color: "white", fontWeight: "700" },
  body: { flex: 1, flexDirection: "row" },
  catalog: { flex: 3, padding: 16 },
  search: {
    backgroundColor: "white",
    borderWidth: 1,
    borderColor: "#d8d4c7",
    borderRadius: 12,
    padding: 14,
    fontSize: 17,
    marginBottom: 14,
    minHeight: 48,
  },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 12 },
  product: {
    width: "47%",
    minHeight: 120,
    backgroundColor: "white",
    borderRadius: 14,
    padding: 16,
    borderWidth: 1,
    borderColor: "#ddd8ca",
    justifyContent: "space-between",
  },
  productName: { fontSize: 19, fontWeight: "700" },
  price: { fontSize: 22, fontWeight: "800", color: "#17613a" },
  tax: { color: "#777" },
  cart: {
    flex: 2,
    backgroundColor: "#fff",
    borderLeftWidth: 1,
    borderColor: "#d8d4c7",
    padding: 16,
  },
  cartTitle: { fontSize: 20, fontWeight: "800", marginBottom: 12 },
  lines: { marginVertical: 12 },
  empty: { color: "#888", textAlign: "center", marginTop: 40 },
  line: {
    flexDirection: "row",
    justifyContent: "space-between",
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderColor: "#eee",
  },
  lineName: { fontSize: 16 },
  linePrice: { fontSize: 16, fontWeight: "700" },
  quantity: { flexDirection: "row", gap: 8, marginTop: 8 },
  discountRow: { flexDirection: "row", gap: 8, marginTop: 8 },
  discountButton: {
    minWidth: 64,
    minHeight: 48,
    backgroundColor: "#f4dfb4",
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
  },
  discountInput: {
    minWidth: 88,
    minHeight: 48,
    borderWidth: 1,
    borderColor: "#d8d4c7",
    borderRadius: 8,
    paddingHorizontal: 10,
  },
  qtyButton: {
    minWidth: 48,
    minHeight: 48,
    backgroundColor: "#e8e4d8",
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
  },
  total: {
    flexDirection: "row",
    justifyContent: "space-between",
    paddingVertical: 16,
    borderTopWidth: 2,
    borderColor: "#222",
  },
  totalLabel: { fontSize: 18, fontWeight: "800" },
  totalValue: { fontSize: 26, fontWeight: "900" },
  splitRow: { flexDirection: "row", gap: 10, marginBottom: 10 },
  splitInput: {
    flex: 1,
    backgroundColor: "white",
    borderWidth: 1,
    borderColor: "#d8d4c7",
    borderRadius: 12,
    padding: 12,
    fontSize: 16,
    minHeight: 56,
  },
  split: {
    flex: 2,
    backgroundColor: "#6c4d9a",
    padding: 14,
    borderRadius: 12,
    alignItems: "center",
    minHeight: 56,
    justifyContent: "center",
  },
  payRow: { flexDirection: "row", gap: 10 },
  cash: {
    flex: 1,
    backgroundColor: "#17613a",
    padding: 18,
    borderRadius: 12,
    alignItems: "center",
    minHeight: 56,
    justifyContent: "center",
  },
  card: {
    flex: 1,
    backgroundColor: "#174a61",
    padding: 18,
    borderRadius: 12,
    alignItems: "center",
    minHeight: 56,
    justifyContent: "center",
  },
  payText: { color: "white", fontSize: 17, fontWeight: "800" },
  footer: {
    padding: 12,
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: "#17251d",
  },
  dot: {
    width: 10,
    height: 10,
    borderRadius: 5,
    backgroundColor: "#57dc8b",
    marginRight: 8,
  },
  dotBad: { backgroundColor: "#ff765f" },
  status: { color: "white", fontWeight: "600", flex: 1 },
  admin: { padding: 20, gap: 16 },
  adminTitle: { fontSize: 24, fontWeight: "800" },
  editor: {
    backgroundColor: "white",
    padding: 18,
    borderRadius: 14,
    maxWidth: 620,
  },
  employeeList: { gap: 8, marginBottom: 14 },
  employee: {
    padding: 12,
    minHeight: 48,
    justifyContent: "center",
    borderWidth: 1,
    borderColor: "#d8d4c7",
    borderRadius: 10,
  },
  employeeSelected: {
    borderColor: "#17613a",
    borderWidth: 2,
    backgroundColor: "#e7f4eb",
  },
  report: { paddingTop: 14, gap: 5 },
});
