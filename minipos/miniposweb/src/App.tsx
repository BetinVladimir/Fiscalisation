import {AppLogout} from './ui/AppLogout';
import {ShiftClosePanel} from './ShiftClosePanel';
import {provenClosedShift,shiftCloseAction} from './ui/shift-close-state';
import {useHeaderLeaveGuard,HeaderLeaveDialog} from './ui/useHeaderLeaveGuard';
import {headerLeaveCopy} from './ui/header-leave-copy';
import {PaperOperationalHeader} from './ui/PaperOperationalHeader';
import {reconciliationCopyFor} from './ui/reconciliation-copy';
import {PaymentReconciliationPanel} from './PaymentReconciliationPanel';
import {revokeReconciliationScope,type ReconciliationRoute,type ReconciliationScope} from './fiscal/paymentReconciliation';
import {OperationOutcome} from './ui/OperationOutcome';
import {FiscalOperationResult} from './ui/FiscalOperationResult';
import {AppFrame} from './ui/app-frame';
import {operationMessage} from './ui/operation-message';
import {unresolvedSale as findUnresolvedSale} from './storage/unresolvedSale';
import {isProvenTerminal} from './storage/outbox';
import { belongsToOutboxScope } from './storage/outboxScope';
import { referenceScope } from './storage/referenceScope';
import { persistAuthTokens } from './storage/authTokens';
import { AppButton as PaperButton } from './ui/app-button';
import { useEffect, useMemo, useRef, useState } from "react";
import { refreshCloudOutbox } from "./fiscal/cloudRecovery";
import {ExecutionAuthorizationManager} from "./fiscal/executionManager";
import type {AuthorizedContext} from "./fiscal/executionAuthorization";
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
import { getOutbox, listOutbox, putOutbox, type OutboxRow } from "./storage/outbox";
import type {
  FiscalIntent,
  FiscalOperation,
  SaleItem,
  SalePayment,
} from "./types";
import "./styles.css";
import { AuthFlow } from "./AuthFlow";
import { Management } from "./Management";
import { ActivityIndicator, Card, HelperText, Surface, Text as PaperText, useTheme } from "react-native-paper";
import { View } from "react-native";
import { PreferenceControls, readLanguage, type Language } from "./ui/PreferenceControls";
import { PaymentPanel, ProductGrid, type CartLine } from "./SalePanels";
import { OutboxPanel } from "./OutboxPanel";
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
  const [view, setView] = useState<"sale" | "products" | "employees" | "outbox" | "reconciliation">("sale");
  const [outboxRows, setOutboxRows] = useState<OutboxRow[]>([]);
  const [outboxLoading, setOutboxLoading] = useState(false);
  const [outboxError, setOutboxError] = useState(false);
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
  const [saleRecovery,setSaleRecovery]=useState<{scope:object;row:OutboxRow}|null>(null);
  const [executionContext,setExecutionContext]=useState<AuthorizedContext>();
  const [lastOrderId, setLastOrderId] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [settings, setSettings] = useState(false);
  const api = useMemo(
    () => new MiniPosClient(backend, () => accessToken, appInstance),
    [backend, accessToken],
  );
  const mounted=useRef(false),owner=useMemo(()=>({api}),[api]),latestOwner=useRef(owner);latestOwner.current=owner;
  const render={},latestRender=useRef(render);latestRender.current=render;
  const outboxRequest=useRef(0),loginPending=useRef<{owner:object;ticket:object}|null>(null);
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;managementRequest.current++;outboxRequest.current++;};},[]);
  const current=()=>mounted.current&&latestOwner.current===owner;
  const admitted=()=>current()&&latestRender.current===render;
  const operationScope=useMemo(()=>({owner,employee:session?.employee.id,configuration,shift:shift?.id}),[owner,session?.employee.id,configuration,shift?.id]);
  const latestOperationScope=useRef(operationScope);latestOperationScope.current=operationScope;
  const syncPending=useRef<{scope:object;ticket:object}|null>(null);
  const [syncActivity,setSyncActivity]=useState<{scope:object;ticket:object}|null>(null);
  const operationCurrent=()=>current()&&latestOperationScope.current===operationScope;
  const executionManager=useMemo(()=>session?.session_id&&shift ? new ExecutionAuthorizationManager(api,{tenant:tenantFrom(accessToken),employee:session.employee.id,sessionID:session.session_id,shiftID:shift.id}):undefined,[api,session?.session_id,session?.employee.id,shift?.id]);
  const previousExecutionManager=useRef<ExecutionAuthorizationManager|undefined>(undefined);
  useEffect(()=>{const previous=previousExecutionManager.current;previousExecutionManager.current=executionManager;setExecutionContext(undefined);if(previous&&previous!==executionManager)void previous.revoke();if(!executionManager)return;let active=true;void (async()=>{try{const cached=await executionManager.restore();if(active&&cached)setExecutionContext(cached);const fresh=await executionManager.issue();if(active)setExecutionContext(fresh.context);}catch(error){if(active&&!(error instanceof ApiError&&error.status>=500)&&navigator.onLine)setMessage(error instanceof Error?error.message:String(error));}})();return()=>{active=false;};},[executionManager]);
  const outboxScope={tenant:executionContext?.claims.tenant_id??'',location:executionContext?.claims.location_id??'',register:executionContext?.claims.register_id??'',operator:executionContext?.claims.operator_code??''};
  const saleLocked=Boolean(shift&&shift.state!=='OPEN')||saleRecovery?.scope===operationScope||!Object.values(outboxScope).every(Boolean);
  function acceptSaleRows(rows:OutboxRow[]){
    if(!operationCurrent())return;
    const pending=findUnresolvedSale(rows,outboxScope);
    if(pending){setSaleRecovery({scope:operationScope,row:pending});setResult(pending.result??{operation_id:pending.intent.client_operation_id,state:'UNKNOWN',certainty:'UNKNOWN',updated_at:pending.updatedAt});return;}
    if(saleRecovery?.scope===operationScope){const recovered=rows.find(row=>row.intent.client_operation_id===saleRecovery.row.intent.client_operation_id);if(recovered?.result&&isProvenTerminal(recovered.result)){setResult(recovered.result);if(['COMMITTED','FISCALIZED'].includes(recovered.result.state)&&recovered.result.certainty==='PROVEN_SUCCESS'){setCart([]);setCardAmount(0);}}}
    if(saleRecovery!==null)setSaleRecovery(null);
  }
  useEffect(()=>{let active=true;if(Object.values(outboxScope).every(Boolean))void listOutbox().then(rows=>{if(active&&operationCurrent())acceptSaleRows(rows);}).catch(()=>{});return()=>{active=false;};},[operationScope,outboxScope.tenant,outboxScope.location,outboxScope.register,outboxScope.operator]);
  const commandPending=useRef<object|null>(null);
  const beginCommand=()=>{
    if(!admitted()||!operationCurrent()||busy||commandPending.current)return null;
    const ticket={};commandPending.current=ticket;setResult(undefined);setBusy(true);
    return {live:()=>operationCurrent()&&commandPending.current===ticket,finish:()=>{if(commandPending.current===ticket){commandPending.current=null;if(current())setBusy(false);}}};
  };
  const cacheScope=referenceScope(backend,accessToken);
  function readReference<T>(key:string){return cacheScope?cacheGet<T>(`${cacheScope}:${key}`):Promise.resolve(undefined);}
  function writeReference<T>(key:string,value:T){return cacheScope?cachePut(`${cacheScope}:${key}`,value):Promise.resolve();}
  useEffect(()=>{setSession(undefined);setConfiguration(undefined);setShift(undefined);setProducts([]);setEmployees([]);setCart([]);setCardAmount(0);setResult(undefined);setLastOrderId('');setLocalToken('');sessionStorage.removeItem('minipos-local-token');sessionStorage.removeItem('minipos-local-token-expires');managementRequest.current++;outboxRequest.current++;},[cacheScope]);
  const total = cart.reduce(
    (sum, line) =>
      sum + Number(line.product.price.amount) * line.quantity - line.discount,
    0,
  );
  useEffect(() => {
    if (accessToken) void login();
  }, [api]);
  useEffect(() => {
    if (!accessToken) return;
    let timer: number, active=true, refreshing=false;
    const scheduleRefresh = (expiresIn?: number) => {
      window.clearInterval(timer);
      const delay = expiresIn ? Math.max((expiresIn - 60) * 1000, 30_000) : 12 * 60 * 1000;
      timer = window.setInterval(() => {
        if(!active||refreshing||!current()||commandPending.current)return;
        refreshing=true;
        void (async () => {
          const token = localStorage.getItem("minipos-refresh-token");
          if (!token) { refreshing=false; return; }
          try {
            const next = await new EmailAuthClient(backend).refresh(token);
            if(!active||!current()||localStorage.getItem("minipos-refresh-token")!==token)return;
            persistAuthTokens(localStorage,next);
            setAccessToken(next.access_token);
            scheduleRefresh(next.expires_in);
          } catch {
            if(!active||!current()||localStorage.getItem("minipos-refresh-token")!==token)return;
            localStorage.removeItem("minipos-access-token");
            localStorage.removeItem("minipos-refresh-token");
            setAccessToken("");
            setSession(undefined);
          } finally { refreshing=false; }
        })();
      }, delay);
    };
    scheduleRefresh();
    return () => {active=false;window.clearInterval(timer);};
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
  }, [session?.employee.id, accessToken, backend, configuration, shift?.id]);
  async function login() {
    if(!admitted()||commandPending.current||loginPending.current?.owner===owner)return;
    const ticket={};loginPending.current={owner,ticket};
    const live=()=>current()&&loginPending.current?.ticket===ticket;
    setBusy(true);
    setMessage("");
    try {
      const active = await api.session();
      if(!live())return;setSession(active);
      const [cfg, refs] = await Promise.all([
        api.configuration().catch(() => undefined),
        api.products(),
      ]);
      if(!live())return;
      if (cfg) setConfiguration(cfg);
      setProducts(refs.items.filter((item) => item.status === "ACTIVE"));
      await Promise.all([cfg ? writeReference("configuration", cfg) : Promise.resolve(), writeReference("products", refs.items)]);
      if(!live())return;
      const shifts = await api.shifts(active.employee.id, "");
      if(!live())return;
      const activeShift=shifts.items.find(item=>item.employee_id===active.employee.id&&item.register_id===cfg?.fiscal_register_id&&["OPEN","CLOSING","BLOCKED_RECONCILIATION"].includes(item.state));
      setShift(activeShift);
      await Promise.all([
        writeReference("operator-session", active),
        writeReference("active-shift", activeShift),
      ]);
      if(!live())return;
      localStorage.setItem("minipos-access-token", accessToken);
      localStorage.setItem("minipos-backend", backend);
    } catch (error) {
      if(!live())return;
      try {
      const [cfg, refs] = await Promise.all([
        readReference<PosConfiguration>("configuration"),
        readReference<Product[]>("products"),
      ]);
      if(!live())return;
      if (cfg && refs) {
        setConfiguration(cfg.value);
        setProducts(refs.value);
        const [cachedSession, cachedShift] = await Promise.all([
          readReference<OperatorSession>("operator-session"),
          readReference<Shift>("active-shift"),
        ]);
        if(!live())return;
        if (
          cachedSession &&
          new Date(cachedSession.value.expires_at) > new Date()
        )
          setSession(cachedSession.value);
        if (cachedShift?.value&&["OPEN","CLOSING","BLOCKED_RECONCILIATION"].includes(cachedShift.value.state)) setShift(cachedShift.value);
        setMessage(
          "OFFLINE_REFERENCE",
        );
      } else setMessage(error instanceof Error ? error.message : String(error));
      } catch { if(live())setMessage("SESSION_CACHE_UNAVAILABLE"); }
    } finally {
      if(loginPending.current?.ticket===ticket){loginPending.current=null;if(current())setBusy(false);}
    }
  }
  async function openManagement(kind: "products" | "employees") {
    if(!admitted())return;outboxRequest.current++;
    const request = ++managementRequest.current;
    setView(kind);
    setManagementError("");
    setManagementLoading(true);
    try {
      if (kind === "products") {
        const response = await api.products();
        if (current() && request === managementRequest.current) setProducts(response.items);
      } else {
        const response = await api.employees();
        if (current() && request === managementRequest.current) setEmployees(response.items);
      }
    } catch (error) {
      if (current() && request === managementRequest.current) setManagementError(error instanceof Error ? error.message : String(error));
    } finally {
      if (current() && request === managementRequest.current) setManagementLoading(false);
    }
  }
  async function openOutbox() {
    if(!admitted())return;
    const request=++outboxRequest.current;
    managementRequest.current++;
    setView('outbox');setOutboxLoading(true);setOutboxError(false);
    const live=()=>operationCurrent()&&request===outboxRequest.current;
    try { const rows=await refreshCloudOutbox(api,outboxScope,live);if(live()){setOutboxRows(rows);acceptSaleRows(rows);} }
    catch { if(live())setOutboxError(true); }
    finally { if(live())setOutboxLoading(false); }
  }
  async function openShift() {
    if (!session || !configuration) return;
    const command=beginCommand();if(!command)return;
    try {
      const opened = await api.openShift(
        configuration.fiscal_register_id,
        session.employee.id,
      );
      if(!command.live())return;
      setShift(opened);
      await writeReference("active-shift", opened);
    } catch (error) {
      if(!command.live())return;
      setMessage(String(error));
    } finally {
      command.finish();
    }
  }
  const shiftCloseKey=useRef<{scope:object;action:string;key:string}|null>(null);
  async function closeCurrentShift(){
    if(!shift||cart.length||saleRecovery?.scope===operationScope)return;
    const action=shiftCloseAction(shift);if(!action)return;
    const command=beginCommand();if(!command)return;
    const previous=shift;
    if(shiftCloseKey.current?.scope!==operationScope||shiftCloseKey.current.action!==action)shiftCloseKey.current={scope:operationScope,action,key:crypto.randomUUID()};
    try{
      const rows=await listOutbox();if(!command.live())return;
      if(findUnresolvedSale(rows,outboxScope)){acceptSaleRows(rows);return;}
      const updated=await api.closeShift(previous.id,shiftCloseKey.current.key,action==='reconcile');
      if(!command.live())return;
      if(updated.id!==previous.id||updated.register_id!==previous.register_id||updated.employee_id!==previous.employee_id)throw Error('SHIFT_IDENTITY_MISMATCH');
      if(provenClosedShift(previous,updated)){
        await writeReference('active-shift',updated);if(!command.live())return;
        setResult({operation_id:updated.z_operation_id!,state:'FISCALIZED',certainty:'PROVEN_SUCCESS',fiscal_reference:updated.z_fiscal_reference!,updated_at:new Date().toISOString()});setShift(undefined);setCart([]);setCardAmount(0);setLastOrderId('');setMessage('');
      }else{
        const pending={...updated,state:updated.state==='BLOCKED_RECONCILIATION'?'BLOCKED_RECONCILIATION':'CLOSING'};
        await writeReference('active-shift',pending);if(command.live()){setShift(pending);setMessage('SHIFT_CLOSE_PENDING');}
      }
    }catch{if(command.live()){const pending={...previous,state:'CLOSING'};setShift(pending);setMessage('SHIFT_CLOSE_PENDING');await writeReference('active-shift',pending).catch(()=>{});}}
    finally{command.finish();}
  }
  function add(product: Product) {
    if(!admitted()||busy||commandPending.current||!products.includes(product))return;
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
    if(!admitted()||busy||commandPending.current||!Number.isFinite(value))return;
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
    if(!operationCurrent())throw new Error("POS_CONTEXT_CHANGED");
    setLocalToken(issued.access_token);
    sessionStorage.setItem("minipos-local-token", issued.access_token);
    sessionStorage.setItem("minipos-local-token-expires", issued.expires_at);
    return issued.access_token;
  }
  async function syncOfflineOrders() {
    if (!navigator.onLine||!operationCurrent()||!configuration||!session||syncPending.current?.scope===operationScope) return;
    const ticket={};const activity={scope:operationScope,ticket};syncPending.current=activity;setSyncActivity(activity);
    const live=()=>operationCurrent()&&syncPending.current?.ticket===ticket;
    try {
    const pending = (await listOutbox()).filter(
      (row) => belongsToOutboxScope(row,outboxScope) && row.posSync && row.result && !row.posSync.synced_at,
    );
    if(!live())return;
    for (const row of pending) {
      if(!live())return;
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
      if(!live())return;
      if (typeof imported.id === "string") setLastOrderId(imported.id);
      await putOutbox({
        ...row,
        posSync: { ...sync, synced_at: new Date().toISOString() },
        updatedAt: new Date().toISOString(),
      });
    }
    } finally {if(syncPending.current?.ticket===ticket){syncPending.current=null;setSyncActivity(null);}}
  }
  async function prepareOrder(items: SaleItem[], live:()=>boolean) {
    if (!shift) throw new Error("SHIFT_REQUIRED");
    let order = await api.createOrder(shift.id),
      version = order.version;
    if(!live())throw new Error("POS_CONTEXT_CHANGED");
    for (let i = 0; i < items.length; i++) {
      if(!live())throw new Error("POS_CONTEXT_CHANGED");
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
      if(!live())throw new Error("POS_CONTEXT_CHANGED");
      version = updated.version;
    }
    return order;
  }
  async function checkout() {
    if (!configuration || !session || !shift || !cart.length || !Number.isFinite(total) || total <= 0 || !Number.isFinite(cardAmount) || cardAmount<0 || cardAmount>total)
      return;
    const command=beginCommand();if(!command)return;
    setMessage("");
    try {
      if(!Object.values(outboxScope).every(Boolean))throw new Error("POS_CONTEXT_INCOMPLETE");
      const durableRows=await listOutbox();if(!command.live())return;
      if(findUnresolvedSale(durableRows,outboxScope)){acceptSaleRows(durableRows);return;}
      const items: SaleItem[] = cart.map((line) => ({
        item_id: crypto.randomUUID(),
        name: line.product.name,
        quantity: line.quantity.toFixed(3),
        unit_price: line.product.price,
        ...(line.discount > 0 ? {discount: money(line.discount)} : {}),
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
        order = await prepareOrder(items,command.live);
        if(!command.live())return;
      } catch (error) {
      if(!command.live())return;
        if (error instanceof ApiError && error.status < 500) throw error;
        setMessage(
          "CLOUD_PREPARE_UNAVAILABLE",
        );
      }
      if(!executionManager)throw new Error('AUTHENTICATED_SESSION_REQUIRED');
      // Preparation failure may use a valid offline lease only before any fiscal attempt.
      // A server-prepared order must retain its exact authority, payload and operation.
      const prepared=order?await executionManager.issue({id:order.id,payments}):undefined;
      const context=prepared?.context??await executionManager.restore();
      if(!context)throw new Error('OFFLINE_EXECUTION_AUTHORIZATION_REQUIRED');
      if(!command.live())return;
      const claims=context.claims;
      const surrogateId = order?.external_id ?? crypto.randomUUID();
      const intent = await buildIntent({
        execution:context,serverPreparedPayload:prepared?.canonicalPayload,
        tenantId: claims.tenant_id,
        locationId: claims.location_id,
        registerId: claims.register_id,
        edgeDeviceId: claims.edge_device_id,
        bindingGeneration: claims.binding_generation,
        operatorCode: claims.operator_code,
        saleId: claims.unp_authority?.server_sale_id,
        surrogateId,
        command: "SALE_FINALIZE",
        items,
        payments,
        metadata: { order_id: order?.id, shift_id: shift.id },
      });
      if(!command.live())return;
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
      if(!command.live())return;
      const executionToken = context.transportToken??"";
      if(!command.live())return;
      const local = new LocalFiscalClient(
        configuration.adapter_base_url,
        () => executionToken,
      );
      const routes: FiscalRoute[] = [];
      if (order)
        routes.push({
          kind: "CLOUD",
          probe: (signal) => api.fiscalRouteHealth(signal).then(() => true),
          lookup: (id, signal) => api.operation(order!.id, id, signal),
          submit: (value, signal) =>
            api.dispatchExecution(order!.id, value, signal),
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
      if(!command.live())return;
      setResult(executed);
      if (order) setLastOrderId(order.id);
      if (!order) void syncOfflineOrders().catch(() => undefined);
      if (["COMMITTED", "FISCALIZED"].includes(executed.state) && executed.certainty === "PROVEN_SUCCESS") {
        setCart([]);
        setCardAmount(0);
      }
    } catch (error) {
      if(!command.live())return;
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      try{const rows=await listOutbox();if(command.live())acceptSaleRows(rows);}catch{if(command.live())setMessage('FISCAL_SUBMISSION_UNCERTAIN');}
      command.finish();
    }
  }
  async function reverseLastOrder() {
    if (!lastOrderId) return;
    const command=beginCommand();if(!command)return;
    try {
      const operationId = crypto.randomUUID();
      const reversed = await api.reverseOrder(
        lastOrderId,
        "CUSTOMER_RETURN",
        operationId,
      );
      if(!command.live())return;
      setResult({
        operation_id: String(reversed.operation_id ?? operationId),
        state: String(reversed.state ?? "UNKNOWN"),
        certainty: typeof reversed.certainty === "string" ? reversed.certainty : "UNKNOWN",
        fiscal_reference: reversed.fiscal_reference
          ? String(reversed.fiscal_reference)
          : undefined,
        updated_at: String(reversed.updated_at ?? new Date().toISOString()),
      });
    } catch (error) {
      if(!command.live())return;
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      command.finish();
    }
  }
  async function printer() {
    if (!configuration || !session || !shift) return;
    const command=beginCommand();if(!command)return;
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
      if(!command.live())return;
      const printed=await client.submit(intent);if(command.live())setResult(printed);
    } catch (error) {
      if(!command.live())return;
      setMessage(String(error));
    } finally {
      command.finish();
    }
  }
  const reconciliationScope:ReconciliationScope|undefined=executionContext?{tenant_id:executionContext.claims.tenant_id,actor_id:executionContext.claims.actor_subject,register_id:executionContext.claims.register_id,edge_device_id:executionContext.claims.edge_device_id,binding_sha256:executionContext.claims.binding_sha256,binding_generation:executionContext.claims.binding_generation,source_session_id:executionContext.scope.sessionID,app_instance_id:executionContext.scope.appInstance}:undefined;
  const reconciliationRoutes:ReconciliationRoute[]=[];
  if(configuration&&executionContext?.transportToken){const local=new LocalFiscalClient(configuration.adapter_base_url,()=>executionContext.transportToken);reconciliationRoutes.push({kind:'LOCAL_HTTP',probe:signal=>local.health(signal).then(()=>true),submit:(value,signal)=>local.submitReconciliation(value,signal),lookup:(id,signal)=>local.operation(id,signal)});
   if(configuration.ble_advertising_identity&&configuration.ble_service_uuid&&configuration.ble_command_uuid&&configuration.ble_event_uuid){const ble=new WebBleRoute({advertisingIdentity:configuration.ble_advertising_identity,serviceUUID:configuration.ble_service_uuid,commandUUID:configuration.ble_command_uuid,eventUUID:configuration.ble_event_uuid});reconciliationRoutes.push({kind:'BLE',probe:signal=>ble.probe(),submit:(value,signal)=>ble.submitCommand(value,signal),lookup:(id,signal)=>local.operation(id,signal)});}}
  const assertReconciliationAdministrator=async()=>{if(!operationCurrent()||!executionManager||!session?.employee.roles.some(role=>role.toUpperCase()==='ADMIN'))throw new Error('ADMIN_SESSION_REQUIRED');const active=await api.session();if(!operationCurrent()||active.session_id!==session.session_id||active.employee.id!==session.employee.id||!(active.employee.roles??[]).some(role=>role.toUpperCase()==='ADMIN')||!await executionManager.restore())throw new Error('ADMIN_SESSION_CHANGED');};
  const [headerChildPending,setHeaderChildPending]=useState<{scope:typeof operationScope;pending:boolean}|null>(null);
  const headerPending=headerChildPending?.scope===operationScope&&headerChildPending.pending;
  const headerLeave=useHeaderLeaveGuard(operationScope,()=>!admitted()||busy||Boolean(commandPending.current));
  const logoutBlocked=(Boolean(session)&&busy)||Boolean(commandPending.current)||Boolean(headerPending)||saleRecovery?.scope===operationScope||syncActivity?.scope===operationScope;
  function logout(){
    if(!admitted()||logoutBlocked||commandPending.current||syncPending.current?.scope===operationScope)return;
    // Revoke the render owner synchronously before any asynchronous work can publish state.
    latestOwner.current={api};
    void executionManager?.revoke().catch(()=>{});
    if(reconciliationScope)void revokeReconciliationScope(reconciliationScope).catch(()=>{});
    managementRequest.current++;outboxRequest.current++;loginPending.current=null;
    localStorage.removeItem('minipos-access-token');localStorage.removeItem('minipos-refresh-token');
    sessionStorage.removeItem('minipos-local-token');sessionStorage.removeItem('minipos-local-token-expires');
    setBusy(false);setAccessToken('');setSession(undefined);setConfiguration(undefined);setShift(undefined);setExecutionContext(undefined);setCart([]);setCardAmount(0);setResult(undefined);setSaleRecovery(null);setLocalToken('');setLastOrderId('');setMessage('');setView('sale');setSettings(false);
  }
  if (!accessToken) return <AuthFlow backend={backend} onAuthenticated={(tokens)=>setAccessToken(tokens.access_token)} />;
  if (!session) return <AppFrame testID="minipos.session.surface"><main><PaperOperationalHeader testID="session.header" title={busy ? t.loading : "Bee MiniPOS Web"} preferences={target=><PreferenceControls target={target} language={language} onLanguageChange={setLanguage} testIdPrefix="session"/>}/><Card mode="outlined" testID="session-recovery" style={{ width: "100%", maxWidth: 480, alignSelf: "center", marginTop: 40 }}><Card.Content style={{ gap: 12 }}>
    <PaperText variant="headlineSmall" accessibilityRole="header">Bee MiniPOS Web</PaperText>
    <PaperText variant="bodyMedium" accessibilityLiveRegion="polite" testID="session-status">{message === "OFFLINE_REFERENCE" ? t.offlineReference : message === "CLOUD_PREPARE_UNAVAILABLE" ? t.cloudPrepareUnavailable : message ? t.sessionUnavailable : t.loading}</PaperText>
    {busy ? <ActivityIndicator accessibilityLabel={t.loading} testID="session-loading-indicator" /> : null}
    <PaperButton mode="contained" disabled={busy} onPress={()=>void login()} testID="session-retry">{t.retry}</PaperButton>
    <PaperButton mode="outlined" disabled={logoutBlocked} onPress={logout} testID="session-logout">{t.logout}</PaperButton>
  </Card.Content></Card></main></AppFrame>;
  return (
    <AppFrame testID="minipos.work.surface"><main><HeaderLeaveDialog guard={headerLeave} copy={headerLeaveCopy[language]}/>
      <Surface elevation={1} style={{ position: 'relative', zIndex: 10, padding: 12, borderRadius: 12, backgroundColor: paperTheme.colors.surface }}>
        <PaperOperationalHeader testID="app.header" title={view==='sale'?t.sales:view==='products'?t.products:view==='employees'?t.employees:view==='reconciliation'?reconciliationCopyFor(language).navTitle:t.outboxTitle} back={view!=='sale'?{label:t.sales,disabled:busy||headerPending,onPress:()=>headerLeave.request(()=>{managementRequest.current++;outboxRequest.current++;setView('sale');})}:undefined} preferences={target=><PreferenceControls target={target} language={language} onLanguageChange={setLanguage} testIdPrefix="app"/>}/>
        <PaperText variant="bodySmall">{session.employee.first_name} {session.employee.last_name} · {navigator.onLine?t.online:t.offline}</PaperText>
        <View style={{minWidth:0,flexDirection:'row',flexWrap:'wrap',gap:8}}>
          <PaperButton disabled={busy||headerPending} mode={settings ? "contained-tonal" : "outlined"} onPress={() => {if(!headerLeave.isBlocked())setSettings(!settings);}} testID="app-settings">{t.settings}</PaperButton>
          <PaperButton disabled={busy||headerPending} mode={view === "products" ? "contained-tonal" : "outlined"} onPress={() => headerLeave.request(()=>void openManagement("products"))} testID="app-products">{t.products}</PaperButton>
          <PaperButton disabled={busy||headerPending} mode={view === "employees" ? "contained-tonal" : "outlined"} onPress={() => headerLeave.request(()=>void openManagement("employees"))} testID="app-employees">{t.employees}</PaperButton>
          {(session.employee.roles??[]).some(role=>role.toUpperCase()==='ADMIN')?<PaperButton disabled={busy||headerPending} mode={view==='reconciliation'?'contained-tonal':'outlined'} onPress={()=>headerLeave.request(()=>setView('reconciliation'))} testID="app-reconciliation">{reconciliationCopyFor(language).navTitle}</PaperButton>:null}
          <PaperButton disabled={busy||headerPending} mode={view === "outbox" ? "contained-tonal" : "outlined"} onPress={() => headerLeave.request(()=>void openOutbox())} testID="app-outbox">{t.outboxTitle}</PaperButton>
          <AppLogout owner={operationScope} blocked={logoutBlocked} dirty={cart.length>0} label={t.logout} copy={headerLeaveCopy[language]} onLogout={()=>headerLeave.request(logout)}/>
        </View>
      </Surface>
      {result&&!busy?<OperationOutcome value={result} language={language}/>:null}
      {settings && <Card mode="outlined" style={{ marginTop: 16 }} testID="sale-diagnostics"><Card.Content style={{ gap: 12 }}>
        <PaperText variant="titleLarge" accessibilityRole="header">{t.diagnostics}</PaperText>
        <PaperText variant="bodyMedium">{t.adapter}: {configuration?.fiscal_adapter_id}{"\n"}{t.register}: {configuration?.fiscal_register_id}{"\n"}{t.generation}: {configuration?.binding_generation}{"\n"}{t.localUrl}: {configuration?.adapter_base_url}</PaperText>
        <PaperButton mode="outlined" disabled={busy || headerPending || !shift} onPress={()=>{if(!headerLeave.isBlocked())void printer();}}>{t.printerTest}</PaperButton>
        {result ? <FiscalOperationResult value={result} language={language} ownerKey={operationScope} testID="sale-result"/> : null}
      </Card.Content></Card>}
      {shift?<ShiftClosePanel shift={shift} language={language} busy={busy} blocked={cart.length>0||saleRecovery?.scope===operationScope||headerPending===true} owner={operationScope} onSubmit={()=>void closeCurrentShift()} onRefresh={()=>void login()}/>:null}
      {view === "reconciliation" ? reconciliationScope&&shift ? <PaymentReconciliationPanel language={language} key={operationScope.employee+':'+shift.id} administrator={(session.employee.roles??[]).some(role=>role.toUpperCase()==='ADMIN')} scope={reconciliationScope} authorize={(decision,key)=>api.authorizePaymentReconciliation(shift.id,decision,key)} routes={reconciliationRoutes} assertActive={assertReconciliationAdministrator} canEditHeader={headerLeave.canEdit} registerHeaderDraft={headerLeave.register} reportHeaderPending={pending=>setHeaderChildPending({scope:operationScope,pending})}/> : <HelperText type="error" visible>{reconciliationCopyFor(language).contextRequired}</HelperText> : view === "outbox" ? <OutboxPanel rows={outboxRows} loading={outboxLoading} error={outboxError} language={language} onRefresh={() => void openOutbox()} /> : view !== "sale" ? managementLoading ? <Card mode="outlined" style={{ marginTop: 16 }} testID="management-loading"><Card.Content style={{ gap: 12 }}>
        <ActivityIndicator accessibilityLabel={t.loading} /><PaperText variant="bodyMedium">{t.loading}</PaperText>
      </Card.Content></Card> : managementError ? <Card mode="outlined" style={{ marginTop: 16 }} testID="management-load-error"><Card.Content style={{ gap: 12 }}>
        <HelperText accessibilityRole="alert" accessibilityLiveRegion="polite" type="error" visible>{t.loadFailed}</HelperText>
        <PaperButton mode="contained" onPress={() => void openManagement(view)} testID="management-retry">{t.retry}</PaperButton>
      </Card.Content></Card> : <Management canEditHeader={headerLeave.canEdit} reportHeaderPending={pending=>setHeaderChildPending({scope:operationScope,pending})} registerHeaderDraft={headerLeave.register} kind={view} products={products} employees={employees} api={api} onProducts={setProducts} onEmployees={setEmployees} language={language} /> : !shift ? (
        <Card mode="outlined" style={{ marginTop: 16 }} testID="shift-open"><Card.Content style={{ gap: 12 }}>
          <PaperText variant="titleLarge" accessibilityRole="header">{t.shiftTitle}</PaperText>
          <PaperButton mode="contained" loading={busy} disabled={busy || !configuration} onPress={openShift}>{t.openShift}</PaperButton>
        </Card.Content></Card>
      ) : (
        <>
          <ProductGrid busy={busy||saleLocked} products={products} onAdd={add} language={language} />
          <PaymentPanel cart={cart} total={total} cardAmount={cardAmount} busy={busy} blocked={saleLocked} lastOrderId={lastOrderId} language={language} onUpdate={update} onCardAmount={value=>{if(admitted()&&!busy&&!commandPending.current&&Number.isFinite(value))setCardAmount(value);}} onCheckout={checkout} onReverse={reverseLastOrder} />
        </>
      )}
      {message ? <div className="error"><HelperText accessibilityRole="alert" accessibilityLiveRegion="polite" type="error" visible testID="sale-message">{operationMessage(language,message)}</HelperText></div> : null}
    </main></AppFrame>
  );
}
