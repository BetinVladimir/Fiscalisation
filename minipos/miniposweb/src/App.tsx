import { useEffect, useMemo, useRef, useState } from "react";
import { buildIntent } from "./fiscal/intentBuilder";
import {
  ApiError,
  EmailAuthClient,
  LocalFiscalClient,
  MiniPosClient,
  type OperatorSession,
  type PosConfiguration,
  type Product,
  type Employee,
  type Shift,
} from "./api/client";
import { RouteController, type FiscalRoute } from "./fiscal/routeController";
import { WebBleRoute } from "./fiscal/webBleRoute";
import { cacheGet, cachePut } from "./storage/referenceCache";
import { getOutbox, listOutbox, putOutbox } from "./storage/outbox";
import type {
  FiscalIntent,
  FiscalOperation,
  SaleItem,
  SalePayment,
} from "./types";
import "./styles.css";
import { AuthFlow } from "./AuthFlow";
import { Management } from "./Management";
import { ActivityIndicator, Button as PaperButton, Card, HelperText, Surface, Text as PaperText, useTheme } from "react-native-paper";
import { PreferenceControls, readLanguage, type Language } from "./ui/PreferenceControls";
import { PaymentPanel, ProductGrid, type CartLine } from "./SalePanels";
import { translations as workTranslations } from "./ui/workTranslations.generated";

const appInstance =
  localStorage.getItem("app-instance-id") ?? crypto.randomUUID();
localStorage.setItem("app-instance-id", appInstance);
// This stable ID binds the operator session to the browser installation.
// Credentials stay session-scoped; reference data and pending fiscal intents
// are persisted through the IndexedDB cache and outbox modules.
const tenantFrom = (token: string) => {
  try {
    return JSON.parse(
      atob(token.split(".")[1].replaceAll("-", "+").replaceAll("_", "/")),
    ).tenant_id as string;
  } catch {
    return "";
  }
};
const money = (value: number) => ({
  amount: value.toFixed(2),
  currency: "EUR" as const,
});

export default function App() {
  const paperTheme = useTheme();
  const [language, setLanguage] = useState<Language>(readLanguage);
  const t = workTranslations[language];
  const [backend, setBackend] = useState(
    localStorage.getItem("minipos-backend") ?? "/public/v1/minipos",
  );
  const [accessToken, setAccessToken] = useState(
    localStorage.getItem("minipos-access-token") ?? "",
  );
  const [session, setSession] = useState<OperatorSession>();
  const [configuration, setConfiguration] = useState<PosConfiguration>();
  const [products, setProducts] = useState<Product[]>([]);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [view, setView] = useState<"sale" | "products" | "employees">("sale");
  const [managementLoading, setManagementLoading] = useState(false);
  const [managementError, setManagementError] = useState("");
  const managementRequest = useRef(0);
  const [shift, setShift] = useState<Shift>();
  const [cart, setCart] = useState<CartLine[]>([]);
  const [cardAmount, setCardAmount] = useState(0);
  const [localToken, setLocalToken] = useState(
    sessionStorage.getItem("minipos-local-token") ?? "",
  );
  const [result, setResult] = useState<FiscalOperation>();
  const [lastOrderId, setLastOrderId] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [settings, setSettings] = useState(false);
  const api = useMemo(
    () => new MiniPosClient(backend, () => accessToken, appInstance),
    [backend, accessToken],
  );
  const total = cart.reduce(
    (sum, line) =>
      sum + Number(line.product.price.amount) * line.quantity - line.discount,
    0,
  );
  useEffect(() => {
    if (accessToken && !session) void login();
  }, [accessToken]);
  useEffect(() => {
    if (!accessToken) return;
    let timer: number;
    const scheduleRefresh = (expiresIn?: number) => {
      window.clearInterval(timer);
      const delay = expiresIn ? Math.max((expiresIn - 60) * 1000, 30_000) : 12 * 60 * 1000;
      timer = window.setInterval(() => {
        void (async () => {
          const token = localStorage.getItem("minipos-refresh-token");
          if (!token) return;
          try {
            const next = await new EmailAuthClient(backend).refresh(token);
            localStorage.setItem("minipos-access-token", next.access_token);
            localStorage.setItem("minipos-refresh-token", next.refresh_token);
            setAccessToken(next.access_token);
            scheduleRefresh(next.expires_in);
          } catch {
            localStorage.removeItem("minipos-access-token");
            localStorage.removeItem("minipos-refresh-token");
            setAccessToken("");
            setSession(undefined);
          }
        })();
      }, delay);
    };
    scheduleRefresh();
    return () => window.clearInterval(timer);
  }, [accessToken, backend]);
  useEffect(() => {
    if (!session || !configuration || !shift) return;
    const expires = Date.parse(
      sessionStorage.getItem("minipos-local-token-expires") ?? "",
    );
    if (
      !localToken ||
      !Number.isFinite(expires) ||
      expires - Date.now() < 60_000
    )
      void adapterToken().catch(() => undefined);
  }, [session?.employee.id, configuration?.fiscal_adapter_id, shift?.id]);
  useEffect(() => {
    if (!session) return;
    void syncOfflineOrders().catch(() => undefined);
    const timer = window.setInterval(
      () => void syncOfflineOrders().catch(() => undefined),
      15_000,
    );
    return () => window.clearInterval(timer);
  }, [session?.employee.id, accessToken, backend]);
  async function login() {
    setBusy(true);
    setMessage("");
    try {
      const active = await api.session();
      setSession(active);
      const [cfg, refs] = await Promise.all([
        api.configuration().catch(() => undefined),
        api.products(),
      ]);
      if (cfg) setConfiguration(cfg);
      setProducts(refs.items.filter((item) => item.status === "ACTIVE"));
      await Promise.all([cfg ? cachePut("configuration", cfg) : Promise.resolve(), cachePut("products", refs.items)]);
      const shifts = await api.shifts(active.employee.id);
      if (!shifts.items[0]) throw new Error("NO_OPEN_SHIFT");
      setShift(shifts.items[0]);
      await Promise.all([
        cachePut("operator-session", active),
        cachePut("active-shift", shifts.items[0]),
      ]);
      localStorage.setItem("minipos-access-token", accessToken);
      localStorage.setItem("minipos-backend", backend);
    } catch (error) {
      const [cfg, refs] = await Promise.all([
        cacheGet<PosConfiguration>("configuration"),
        cacheGet<Product[]>("products"),
      ]);
      if (cfg && refs) {
        setConfiguration(cfg.value);
        setProducts(refs.value);
        const [cachedSession, cachedShift] = await Promise.all([
          cacheGet<OperatorSession>("operator-session"),
          cacheGet<Shift>("active-shift"),
        ]);
        if (
          cachedSession &&
          new Date(cachedSession.value.expires_at) > new Date()
        )
          setSession(cachedSession.value);
        if (cachedShift?.value.state === "OPEN") setShift(cachedShift.value);
        setMessage(
          "OFFLINE_REFERENCE",
        );
      } else setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }
  async function openManagement(kind: "products" | "employees") {
    const request = ++managementRequest.current;
    setView(kind);
    setManagementError("");
    setManagementLoading(true);
    try {
      if (kind === "products") {
        const response = await api.products();
        if (request === managementRequest.current) setProducts(response.items);
      } else {
        const response = await api.employees();
        if (request === managementRequest.current) setEmployees(response.items);
      }
    } catch (error) {
      if (request === managementRequest.current) setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      if (request === managementRequest.current) setManagementLoading(false);
    }
  }
  async function openShift() {
    if (!session || !configuration) return;
    setBusy(true);
    try {
      const opened = await api.openShift(
        configuration.fiscal_register_id,
        session.employee.id,
      );
      setShift(opened);
      await cachePut("active-shift", opened);
    } catch (error) {
      setMessage(String(error));
    } finally {
      setBusy(false);
    }
  }
  function add(product: Product) {
    setCart((value) => {
      const old = value.find((line) => line.product.id === product.id);
      return old
        ? value.map((line) =>
            line === old ? { ...line, quantity: line.quantity + 1 } : line,
          )
        : [...value, { product, quantity: 1, discount: 0 }];
    });
  }
  function update(id: string, key: "quantity" | "discount", value: number) {
    setCart((lines) =>
      lines.map((line) =>
        line.product.id === id
          ? { ...line, [key]: Math.max(key === "quantity" ? 1 : 0, value) }
          : line,
      ),
    );
  }
  async function adapterToken() {
    if (!configuration || !session || !shift)
      throw new Error("POS_CONTEXT_INCOMPLETE");
    const issued = await api.localToken(
      {
        adapter_device_id: configuration.fiscal_adapter_id,
        location_id: configuration.location_id,
        register_id: configuration.fiscal_register_id,
        operator_id: session.employee.id,
        shift_id: shift.id,
        binding_generation: configuration.binding_generation,
        adapter_base_url: configuration.adapter_base_url,
      },
      crypto.randomUUID(),
    );
    setLocalToken(issued.access_token);
    sessionStorage.setItem("minipos-local-token", issued.access_token);
    sessionStorage.setItem("minipos-local-token-expires", issued.expires_at);
    return issued.access_token;
  }
  async function syncOfflineOrders() {
    if (!navigator.onLine) return;
    const pending = (await listOutbox()).filter(
      (row) => row.posSync && row.result && !row.posSync.synced_at,
    );
    for (const row of pending) {
      const sync = row.posSync!;
      const imported = await api.importOfflineOrder(
        {
          external_id: sync.external_id,
          shift_id: sync.shift_id,
          client_operation_id: row.intent.client_operation_id,
          receipt_session_id: row.intent.receipt_session_id,
          fiscal_reference: row.result?.fiscal_reference,
          fiscal_state: ["COMMITTED", "FISCALIZED"].includes(row.result!.state)
            ? "FISCALIZED"
            : row.result!.state === "FAILED" || row.result!.state === "REJECTED"
              ? "FAILED"
              : "UNKNOWN",
          lines: sync.lines,
          payments: sync.payments,
        },
        sync.external_id,
      );
      if (typeof imported.id === "string") setLastOrderId(imported.id);
      await putOutbox({
        ...row,
        posSync: { ...sync, synced_at: new Date().toISOString() },
        updatedAt: new Date().toISOString(),
      });
    }
  }
  async function prepareOrder(items: SaleItem[]) {
    if (!shift) throw new Error("SHIFT_REQUIRED");
    let order = await api.createOrder(shift.id),
      version = order.version;
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      const updated = await api.addLine(
        order.id,
        version,
        {
          line_id: item.item_id,
          product_id: cart[i]?.product.id,
          name: item.name,
          quantity: item.quantity,
          unit_price: item.unit_price,
          discount: item.discount,
          tax_group: item.tax_group,
        },
        crypto.randomUUID(),
      );
      version = updated.version;
    }
    return order;
  }
  async function checkout() {
    if (!configuration || !session || !shift || !cart.length || total <= 0)
      return;
    setBusy(true);
    setMessage("");
    try {
      const items: SaleItem[] = cart.map((line) => ({
        item_id: crypto.randomUUID(),
        name: line.product.name,
        quantity: line.quantity.toFixed(3),
        unit_price: line.product.price,
        discount: line.discount > 0 ? money(line.discount) : undefined,
        tax_group: line.product.tax_group,
      }));
      const safeCard = Math.min(Math.max(cardAmount, 0), total),
        payments: SalePayment[] = [
          ...(safeCard > 0
            ? [
                {
                  payment_id: crypto.randomUUID(),
                  type: "CARD" as const,
                  amount: money(safeCard),
                },
              ]
            : []),
          ...(total - safeCard > 0
            ? [
                {
                  payment_id: crypto.randomUUID(),
                  type: "CASH" as const,
                  amount: money(total - safeCard),
                },
              ]
            : []),
        ];
      let order: { id: string; external_id: string } | undefined;
      try {
        order = await prepareOrder(items);
      } catch (error) {
        if (error instanceof ApiError && error.status < 500) throw error;
        setMessage(
          "CLOUD_PREPARE_UNAVAILABLE",
        );
      }
      const surrogateId = order?.external_id ?? crypto.randomUUID();
      const intent = await buildIntent({
        tenantId: tenantFrom(accessToken),
        locationId: configuration.location_id,
        registerId: configuration.fiscal_register_id,
        edgeDeviceId: configuration.fiscal_adapter_id,
        bindingGeneration: configuration.binding_generation,
        operatorCode: session.employee.operator_code,
        saleId: order?.id,
        surrogateId,
        command: "SALE_FINALIZE",
        items,
        payments,
        metadata: { order_id: order?.id, shift_id: shift.id },
      });
      if (!order) {
        await putOutbox({
          intent,
          posSync: {
            shift_id: shift.id,
            external_id: surrogateId,
            lines: items.map((item, index) => ({
              line_id: item.item_id,
              product_id: cart[index]?.product.id,
              name: item.name,
              quantity: item.quantity,
              unit_price: item.unit_price,
              discount: item.discount,
              tax_group: item.tax_group,
            })),
            payments,
          },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        });
      }
      const executionToken = localToken || (await adapterToken());
      const local = new LocalFiscalClient(
        configuration.adapter_base_url,
        () => executionToken,
      );
      const routes: FiscalRoute[] = [];
      if (order)
        routes.push({
          kind: "CLOUD",
          probe: (signal) => api.fiscalRouteHealth(signal).then(() => true),
          lookup: async () => {
            throw new Error("CLOUD_LOOKUP_NOT_FOUND");
          },
          submit: (value, signal) =>
            api.checkout(order!.id, payments, value, signal),
        });
      routes.push({
        kind: "LOCAL_HTTP",
        probe: (signal) => local.health(signal).then(() => true),
        lookup: (id, signal) => local.operation(id, signal),
        submit: (value, signal) => local.submit(value, signal),
      });
      if (
        configuration.ble_advertising_identity &&
        configuration.ble_service_uuid &&
        configuration.ble_command_uuid &&
        configuration.ble_event_uuid
      )
        routes.push(
          new WebBleRoute({
            advertisingIdentity: configuration.ble_advertising_identity,
            serviceUUID: configuration.ble_service_uuid,
            commandUUID: configuration.ble_command_uuid,
            eventUUID: configuration.ble_event_uuid,
          }),
        );
      const executed = await new RouteController(routes).execute(intent);
      setResult(executed);
      if (order) setLastOrderId(order.id);
      if (!order) void syncOfflineOrders().catch(() => undefined);
      if (["COMMITTED", "FISCALIZED"].includes(executed.state)) {
        setCart([]);
        setCardAmount(0);
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }
  async function reverseLastOrder() {
    if (!lastOrderId) return;
    setBusy(true);
    try {
      const operationId = crypto.randomUUID();
      const reversed = await api.reverseOrder(
        lastOrderId,
        "CUSTOMER_RETURN",
        operationId,
      );
      setResult({
        operation_id: String(reversed.operation_id ?? operationId),
        state: String(reversed.state ?? "UNKNOWN"),
        certainty:
          reversed.state === "FISCALIZED" ? "PROVEN_SUCCESS" : "UNKNOWN",
        fiscal_reference: reversed.fiscal_reference
          ? String(reversed.fiscal_reference)
          : undefined,
        updated_at: String(reversed.updated_at ?? new Date().toISOString()),
      });
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }
  async function printer() {
    if (!configuration || !session || !shift) return;
    setBusy(true);
    try {
      const token = localToken || (await adapterToken()),
        client = new LocalFiscalClient(
          configuration.adapter_base_url,
          () => token,
        ),
        intent = await buildIntent({
          tenantId: tenantFrom(accessToken),
          locationId: configuration.location_id,
          registerId: configuration.fiscal_register_id,
          edgeDeviceId: configuration.fiscal_adapter_id,
          bindingGeneration: configuration.binding_generation,
          operatorCode: session.employee.operator_code,
          command: "PRINTER_TEST",
        });
      setResult(await client.submit(intent));
    } catch (error) {
      setMessage(String(error));
    } finally {
      setBusy(false);
    }
  }
  if (!accessToken) return <AuthFlow backend={backend} onAuthenticated={(tokens)=>setAccessToken(tokens.access_token)} />;
  if (!session) return <main><Card mode="outlined"><Card.Content style={{ gap: 12 }}>
    <PaperText variant="headlineSmall" accessibilityRole="header">Bee MiniPOS Web</PaperText>
    <PaperText variant="bodyMedium">{message === "OFFLINE_REFERENCE" ? t.offlineReference : message === "CLOUD_PREPARE_UNAVAILABLE" ? t.cloudPrepareUnavailable : message || t.loading}</PaperText>
    <PaperButton mode="contained" loading={busy} disabled={busy} onPress={()=>void login()}>{t.retry}</PaperButton>
    <PaperButton mode="outlined" onPress={()=>{localStorage.removeItem("minipos-access-token");localStorage.removeItem("minipos-refresh-token");setAccessToken("");}}>{t.logout}</PaperButton>
  </Card.Content></Card></main>;
  return (
    <main>
      <Surface elevation={1} style={{ padding: 12, borderRadius: 12, backgroundColor: paperTheme.colors.surface }}>
        <header>
          <div>
            <PaperText variant="headlineSmall" accessibilityRole="header">Bee MiniPOS Web</PaperText>
            <PaperText variant="bodySmall">
              {session.employee.first_name} {session.employee.last_name} ·{" "}
              {navigator.onLine ? t.online : t.offline}
            </PaperText>
          </div>
          <PaperButton mode={settings ? "contained-tonal" : "outlined"} onPress={() => setSettings(!settings)} testID="app-settings">{t.settings}</PaperButton>
          <PaperButton mode={view === "sale" ? "contained-tonal" : "outlined"} onPress={() => { managementRequest.current++; setView("sale"); }} testID="app-sales">{t.sales}</PaperButton>
          <PaperButton mode={view === "products" ? "contained-tonal" : "outlined"} onPress={() => void openManagement("products")} testID="app-products">{t.products}</PaperButton>
          <PaperButton mode={view === "employees" ? "contained-tonal" : "outlined"} onPress={() => void openManagement("employees")} testID="app-employees">{t.employees}</PaperButton>
          <PreferenceControls language={language} onLanguageChange={setLanguage} testIdPrefix="app" />
        </header>
      </Surface>
      {settings && <Card mode="outlined" style={{ marginTop: 16 }} testID="sale-diagnostics"><Card.Content style={{ gap: 12 }}>
        <PaperText variant="titleLarge" accessibilityRole="header">{t.diagnostics}</PaperText>
        <PaperText variant="bodyMedium">{t.adapter}: {configuration?.fiscal_adapter_id}{"\n"}{t.register}: {configuration?.fiscal_register_id}{"\n"}{t.generation}: {configuration?.binding_generation}{"\n"}{t.localUrl}: {configuration?.adapter_base_url}</PaperText>
        <PaperButton mode="outlined" disabled={busy || !shift} onPress={printer}>{t.printerTest}</PaperButton>
        {result ? <PaperText variant="bodySmall" selectable style={{ fontFamily: "monospace" }} testID="sale-result">{JSON.stringify(result, null, 2)}</PaperText> : null}
      </Card.Content></Card>}
      {view !== "sale" ? managementLoading ? <Card mode="outlined" style={{ marginTop: 16 }} testID="management-loading"><Card.Content style={{ gap: 12 }}>
        <ActivityIndicator accessibilityLabel={t.loading} /><PaperText variant="bodyMedium">{t.loading}</PaperText>
      </Card.Content></Card> : managementError ? <Card mode="outlined" style={{ marginTop: 16 }} testID="management-load-error"><Card.Content style={{ gap: 12 }}>
        <HelperText type="error" visible>{t.loadFailed}</HelperText>
        <PaperButton mode="contained" onPress={() => void openManagement(view)} testID="management-retry">{t.retry}</PaperButton>
      </Card.Content></Card> : <Management kind={view} products={products} employees={employees} api={api} onProducts={setProducts} onEmployees={setEmployees} language={language} /> : !shift ? (
        <Card mode="outlined" style={{ marginTop: 16 }} testID="shift-open"><Card.Content style={{ gap: 12 }}>
          <PaperText variant="titleLarge" accessibilityRole="header">{t.shiftTitle}</PaperText>
          <PaperButton mode="contained" loading={busy} disabled={busy || !configuration} onPress={openShift}>{t.openShift}</PaperButton>
        </Card.Content></Card>
      ) : (
        <>
          <ProductGrid products={products} onAdd={add} language={language} />
          <PaymentPanel cart={cart} total={total} cardAmount={cardAmount} busy={busy} lastOrderId={lastOrderId} language={language} onUpdate={update} onCardAmount={setCardAmount} onCheckout={checkout} onReverse={reverseLastOrder} />
        </>
      )}
      {message ? <div className="error"><HelperText type="error" visible testID="sale-message">{message === "OFFLINE_REFERENCE" ? t.offlineReference : message === "CLOUD_PREPARE_UNAVAILABLE" ? t.cloudPrepareUnavailable : message}</HelperText></div> : null}
    </main>
  );
}
