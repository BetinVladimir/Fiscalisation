import { useEffect, useMemo, useState } from "react";
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

type CartLine = { product: Product; quantity: number; discount: number };
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
          "Offline справочници: нова смяна не може да се отвори без backend",
        );
      } else setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
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
          "Cloud подготовката е недостъпна; изпълнява се локално и ще се съгласува след възстановяване",
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
  if (!session) return <main><section><h1>Bee MiniPOS Web</h1><p>{message || "Зареждане…"}</p><button onClick={()=>void login()} disabled={busy}>Повтори</button><button className="secondary" onClick={()=>{localStorage.removeItem("minipos-access-token");localStorage.removeItem("minipos-refresh-token");setAccessToken("");}}>Изход</button></section></main>;
  return (
    <main>
      <header>
        <div>
          <h1>Bee MiniPOS Web</h1>
          <small>
            {session.employee.first_name} {session.employee.last_name} ·{" "}
            {navigator.onLine ? "online" : "offline"}
          </small>
        </div>
        <button className="secondary" onClick={() => setSettings(!settings)}>
          Настройки
        </button>
        <button className="secondary" onClick={() => setView("sale")}>Продажби</button>
        <button className="secondary" onClick={async () => { setView("products"); const result=await api.products(); setProducts(result.items); }}>Товари</button>
        <button className="secondary" onClick={async () => { setView("employees"); const result=await api.employees(); setEmployees(result.items); }}>Сотрудники</button>
      </header>
      {settings && (
        <section>
          <h2>Диагностика</h2>
          <p>
            Adapter: {configuration?.fiscal_adapter_id}
            <br />
            Register: {configuration?.fiscal_register_id}
            <br />
            Generation: {configuration?.binding_generation}
            <br />
            Local URL: {configuration?.adapter_base_url}
          </p>
          <button disabled={busy || !shift} onClick={printer}>
            Тест на принтера
          </button>
          {result && <pre>{JSON.stringify(result, null, 2)}</pre>}
        </section>
      )}
      {view !== "sale" ? <Management kind={view} products={products} employees={employees} api={api} onProducts={setProducts} onEmployees={setEmployees} /> : !shift ? (
        <section>
          <h2>Касова смяна</h2>
          <button disabled={busy || !configuration} onClick={openShift}>
            Отвори смяна
          </button>
        </section>
      ) : (
        <>
          <section>
            <h2>Продукти</h2>
            <div className="products">
              {products.map((product) => (
                <button
                  className="product"
                  key={product.id}
                  onClick={() => add(product)}
                >
                  <strong>{product.name}</strong>
                  <span>{product.price.amount} EUR</span>
                </button>
              ))}
            </div>
          </section>
          <section>
            <h2>Текуща продажба</h2>
            {cart.length === 0 ? (
              <p>Изберете продукт.</p>
            ) : (
              cart.map((line) => (
                <div className="cart" key={line.product.id}>
                  <strong>{line.product.name}</strong>
                  <label>
                    Количество
                    <input
                      inputMode="numeric"
                      type="number"
                      min="1"
                      value={line.quantity}
                      onChange={(event) =>
                        update(
                          line.product.id,
                          "quantity",
                          Number(event.target.value),
                        )
                      }
                    />
                  </label>
                  <label>
                    Отстъпка EUR
                    <input
                      inputMode="decimal"
                      type="number"
                      min="0"
                      step="0.01"
                      value={line.discount}
                      onChange={(event) =>
                        update(
                          line.product.id,
                          "discount",
                          Number(event.target.value),
                        )
                      }
                    />
                  </label>
                </div>
              ))
            )}
            <div className="total">Общо: {total.toFixed(2)} EUR</div>
            <label>
              С карта (остатъкът е в брой)
              <input
                inputMode="decimal"
                type="number"
                min="0"
                max={total}
                step="0.01"
                value={cardAmount}
                onChange={(event) => setCardAmount(Number(event.target.value))}
              />
            </label>
            <button
              className="checkout"
              disabled={busy || !cart.length || total <= 0}
              onClick={checkout}
            >
              Плащане и фискализация
            </button>
            {lastOrderId && (
              <button
                className="secondary"
                disabled={busy}
                onClick={reverseLastOrder}
              >
                Сторно на последната продажба
              </button>
            )}
          </section>
        </>
      )}
      {message && <p className="error">{message}</p>}
    </main>
  );
}
