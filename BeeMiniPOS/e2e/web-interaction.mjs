import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile, readdir, stat } from "node:fs/promises";
import { existsSync } from "node:fs";
import { extname, join, normalize } from "node:path";
import { chromium } from "@playwright/test";

const root = new URL("../", import.meta.url).pathname;
const fiscalRoot = new URL("../../BeeFiscalApp/", import.meta.url).pathname;
const mime = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".png": "image/png", ".svg": "image/svg+xml" };

async function assertNoPublicStaticTokens(directory) {
  const files = await readdir(directory, { recursive: true });
  const text = (await Promise.all(files.filter(file => /\.(html|js|json)$/.test(file)).map(file => readFile(join(directory, file), "utf8")))).join("\n");
  assert.equal(text.includes("forbidden-static-token"), false, "PROD bundle contains MiniPOS static token");
  assert.equal(text.includes("forbidden-fiscal-static-token"), false, "PROD bundle contains Fiscal static token");
  assert.equal(text.includes("forbidden-fiscal-admin-static-token"), false, "PROD BeeFiscal bundle contains admin static token");
}

async function staticServer(directory) {
  const server = createServer(async (request, response) => {
    try {
      const pathname = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
      const relative = normalize(pathname).replace(/^(\.\.(\/|\\|$))+/, "").replace(/^[/\\]+/, "");
      let file = join(directory, relative || "index.html");
      if ((await stat(file)).isDirectory()) file = join(file, "index.html");
      response.writeHead(200, { "Content-Type": mime[extname(file)] || "application/octet-stream" });
      response.end(await readFile(file));
    } catch {
      response.writeHead(404).end("not found");
    }
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  return { server, url: `http://127.0.0.1:${server.address().port}` };
}

async function fiscalProductionLoginFailClosed(browser, appUrl) {
  const page = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  let apiCalls = 0;
  await page.route("http://fiscal-admin.test/**", route => { apiCalls += 1; return json(route, { code: "AUTH_BYPASS" }, 500); });
  await page.goto(appUrl);
  await page.getByTestId("admin-login").waitFor();
  await page.getByTestId("admin-login-start").waitFor();
  assert.equal(apiCalls, 0, "BeeFiscal PROD called API before personal OIDC login");
  await page.close();
}

const json = (route, body, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
const pathOf = (route) => new URL(route.request().url()).pathname;

async function miniPosJourney(browser, appUrl) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const products = [{ id: "coffee", sku: "COF", barcode: "380000000001", name: "Кафе", price: { amount: "2.50", currency: "EUR" }, tax_group: "B", active: true }];
  const employees = [{ id: "employee-1", first_name: "Иван", last_name: "Петров", operator_code: "0001" }];
  let checkoutMode = "success", checkoutCalls = 0, splitRequests = [], reportPeriodValid = false, shiftRecoveryCalls = 0, openShift = null, zCloseMode = "success", zCloseCalls = 0, zReconcileCalls = 0;
  let fiscalSale = null, fiscalSaleNumber = 0, fiscalOperationNumber = 0, paidCents = 0;
  let configuration = { id: "configuration", location_name: "Магазин 1", location_address: "София", workstation_name: "Каса 01", fiscal_register_id: "00000000-0000-4000-8000-000000000001", version: 1 };

  await page.route("http://minipos-api.test/**", async (route) => {
    const path = pathOf(route), method = route.request().method();
    if (path.endsWith("/products") && method === "GET") return json(route, { items: products });
    if (path.endsWith("/products") && method === "POST") { const input=route.request().postDataJSON(); products.push({ id: "product-2", sku: "NEW", ...input, active: true }); return json(route, products.at(-1), 201); }
    if (path.endsWith("/employees") && method === "GET") return json(route, { items: employees });
    if (path.endsWith("/employees") && method === "POST") { employees.push({ id: "employee-2", first_name: "Мария", last_name: "Иванова", operator_code: "0002" }); return json(route, employees.at(-1), 201); }
    if (path.endsWith("/configuration") && method === "GET") return json(route, configuration);
    if (path.endsWith("/configuration") && method === "PATCH") { configuration = { ...configuration, ...route.request().postDataJSON(), version: configuration.version + 1 }; return json(route, configuration); }
    if (path.endsWith("/reports/sales")) { const url=new URL(route.request().url()),from=Date.parse(url.searchParams.get("from")||""),to=Date.parse(url.searchParams.get("to")||""); reportPeriodValid=Number.isFinite(from)&&Number.isFinite(to)&&from<to; return json(route, { from: new Date(from).toISOString(), to: new Date(to).toISOString(), currency: "EUR", gross: { amount: "12.50", currency: "EUR" }, payments: [{ type: "CASH", amount: { amount: "12.50", currency: "EUR" } }], generated_at: "2026-08-09T12:00:00Z" }); }
    if (path.endsWith("/shifts") && method === "GET") { shiftRecoveryCalls += 1; return json(route, { items: openShift ? [openShift] : [], page: { next_cursor: null, has_more: false } }); }
    if (path.endsWith("/shifts") && method === "POST") { openShift = { id: "shift-1", register_id: "00000000-0000-4000-8000-000000000001", employee_id: "employee-1", state: "OPEN", allowed_actions: ["SALE", "CLOSE"] }; return json(route, openShift, 201); }
    if (path.endsWith("/shifts/shift-1/close") && method === "POST") { zCloseCalls += 1; if (zCloseMode === "ambiguous") { openShift = { ...openShift, state: "BLOCKED_RECONCILIATION", allowed_actions: ["READ", "RECONCILE"], z_operation_id: "z-operation-1", close_error: "UNKNOWN" }; return json(route, { code: "MINIPOS_ERROR", status: 409 }, 409); } openShift = null; return json(route, { id: "shift-1", register_id: "00000000-0000-4000-8000-000000000001", employee_id: "employee-1", state: "CLOSED", allowed_actions: [], z_operation_id: "z-operation-1", z_fiscal_reference: "Z-000001" }); }
    if (path.endsWith("/shifts/shift-1/reconcile") && method === "POST") { zReconcileCalls += 1; openShift = null; return json(route, { id: "shift-1", register_id: "00000000-0000-4000-8000-000000000001", employee_id: "employee-1", state: "CLOSED", allowed_actions: [], z_operation_id: "z-operation-1", z_fiscal_reference: "Z-000001" }); }
    if (path.endsWith("/orders") && method === "POST") { orderNumber += 1; return json(route, { id: `order-${orderNumber}`, external_id: `sale-${orderNumber}`, state: "DRAFT", version: 1 }, 201); }
    if (/\/orders\/order-\d+\/lines$/.test(path)) return json(route, { id: `order-${orderNumber}`, external_id: `sale-${orderNumber}`, state: "DRAFT", version: 2 });
    if (/\/orders\/order-\d+\/checkout$/.test(path)) { checkoutCalls += 1; return json(route, { operation_id: `fiscal-${orderNumber}`, type: "FISCAL_SALE", state: checkoutMode === "success" ? "FISCALIZED" : checkoutMode === "failed" ? "FAILED" : "UNKNOWN", fiscal_reference: checkoutMode === "success" ? `FD-${orderNumber}` : null, simulated: true, allowed_actions: [], created_at: "2026-08-09T12:00:00Z", updated_at: "2026-08-09T12:00:00Z" }, 202); }
    if (/\/orders\/order-\d+\/checkout-batch$/.test(path)) { checkoutCalls += 1; splitRequest = route.request().postDataJSON(); return json(route, { operation_id: `fiscal-${orderNumber}`, type: "FISCAL_SALE", state: "FISCALIZED", fiscal_reference: `FD-${orderNumber}`, simulated: true, allowed_actions: [], created_at: "2026-08-09T12:00:00Z", updated_at: "2026-08-09T12:00:00Z" }, 202); }
    if (/\/orders\/order-\d+$/.test(path) && method === "GET") { orderStatusReads += 1; return orderStatusReads === 1 ? json(route, { id: `order-${orderNumber}`, state: "UNKNOWN", version: 3, allowed_actions: ["READ"] }) : json(route, { id: `order-${orderNumber}`, state: "COMPLETED", version: 4, allowed_actions: ["RECEIPT"], fiscal_operation_id: `fiscal-${orderNumber}` }); }
    return json(route, { code: "UNMOCKED", path, method }, 500);
  });
  await page.route("http://fiscal-api.test/**", (route) => {
    const path = pathOf(route), method = route.request().method();
    if (path.endsWith("/clock-sync") && method === "POST") return json(route, { state: "VERIFIED" });
    if (path.endsWith("/readiness:refresh") && method === "POST") return json(route, { ready: true });
    if (path.endsWith("/sessions") && method === "POST") return json(route, { session_id: "00000000-0000-4000-8000-000000000020", workstation_id: "00000000-0000-4000-8000-000000000001", operator_code: "0001", expires_at: "2099-08-09T12:00:00Z" }, 201);
    if (path.endsWith("/sales") && method === "GET") return json(route, { items: fiscalSale?.state === "OPEN" ? [fiscalSale] : [], page: { has_more: false, next_cursor: null } });
    if (path.endsWith("/sales:open-with-line") && method === "POST") {
      fiscalSaleNumber += 1; paidCents = 0; const input = route.request().postDataJSON();
      fiscalSale = { sale_id: `sale-${fiscalSaleNumber}`, external_id: input.client_sale_surrogate_id, register_id: input.workstation_id, operator_id: "0001", unp: `AB123456-0001-${String(fiscalSaleNumber).padStart(7,"0")}`, regulatory_identifiers: [{ type: "SALE", scheme: "BG_UNP_V1", value: `AB123456-0001-${String(fiscalSaleNumber).padStart(7,"0")}`, country_code: "BG", profile_version: "2026.1" }], state: "OPEN", version: 1, lines: [{ ...input.line, unit_price: input.line.unit_price, discount: null }], payments: [], allowed_actions: ["ADD_LINE","CHANGE_LINE","CANCEL_LINE","PAY","CANCEL"], totals: { gross: input.line.unit_price } };
      return json(route, fiscalSale, 201);
    }
    if (/\/sales\/sale-\d+\/lines\/[^/:]+$/.test(path) && method === "PATCH") {
      const input=route.request().postDataJSON(), price=Number(input.unit_price.amount), quantity=Number(input.quantity), discount=Number(input.discount?.amount||0);
      fiscalSale={...fiscalSale,version:fiscalSale.version+1,lines:[input],totals:{gross:{amount:(price*quantity-discount).toFixed(2),currency:"EUR"}}}; return json(route,fiscalSale);
    }
    if (/\/sales\/sale-\d+\/payment-intents$/.test(path) && method === "POST") {
      checkoutCalls += 1; const payment=route.request().postDataJSON(); splitRequests.push(payment); fiscalOperationNumber += 1;
      const state=checkoutMode === "failed" ? "FAILED" : checkoutMode === "unknown" ? "UNKNOWN" : "FISCALIZED";
      if(state==="FISCALIZED"){paidCents+=Math.round(Number(payment.amount.amount)*100);const totalCents=Math.round(Number(fiscalSale.totals.gross.amount)*100);fiscalSale={...fiscalSale,version:fiscalSale.version+1,payments:[...fiscalSale.payments,payment],state:paidCents>=totalCents?"COMPLETED":"OPEN"}}
      return json(route,{operation_id:`fiscal-${fiscalOperationNumber}`,type:"FISCAL_SALE",state,fiscal_reference:state==="FISCALIZED"?`FD-${fiscalOperationNumber}`:null,allowed_actions:state==="UNKNOWN"?["RECONCILE"]:[]},202);
    }
    if (/\/sales\/sale-\d+$/.test(path) && method === "GET") return json(route,fiscalSale);
    if (/\/operations\/fiscal-\d+$/.test(path) && method === "GET") return json(route,{operation_id:`fiscal-${fiscalOperationNumber}`,state:checkoutMode==="unknown"?"UNKNOWN":"FISCALIZED"});
    if (/\/operations\/fiscal-\d+:reconcile$/.test(path) && method === "POST") {checkoutMode="success";fiscalSale={...fiscalSale,state:"COMPLETED",version:fiscalSale.version+1};return json(route,{operation_id:`fiscal-${fiscalOperationNumber}`,state:"FISCALIZED"},202)}
    return json(route, { code: "UNMOCKED", path, method }, 500);
  });

  await page.goto(appUrl);
  await page.getByTestId("status-transport").getByText(/Готово/).waitFor();
  await page.getByTestId("product-coffee").click();
  await page.getByTestId("status-transport").getByText(/няма .*отворена смяна/).waitFor();

  await page.getByTestId("admin-toggle").click();
  await page.getByTestId("config-location-name").fill("Магазин София");
  await page.getByTestId("config-save").click();
  await page.getByTestId("status-transport").getByText(/кас[оа]вото място са записани/).waitFor();
  await page.getByTestId("product-name").fill("Чай");
  await page.getByTestId("product-barcode").fill("380000000002");
  await page.getByTestId("product-price").fill("3.20");
  await page.getByTestId("product-create").click();
  await page.getByText("Продукт (2)").waitFor();
  await page.getByTestId("employee-first-name").fill("Мария");
  await page.getByTestId("employee-last-name").fill("Иванова");
  await page.getByTestId("employee-code").fill("0002");
  await page.getByTestId("employee-create").click();
  await page.getByText("Служител (2)").waitFor();
  await page.getByTestId("sales-report-load").click();
  await page.getByTestId("sales-report-result").getByText("€ 12.50", { exact: true }).waitFor();
  assert.equal(reportPeriodValid, true, "MiniPOS commercial report must send the required ordered RFC3339 period");

  await page.getByTestId("admin-toggle").click();
  await page.getByTestId("shift-toggle").click();
  await page.getByTestId("status-transport").getByText(/Смяната е отворена/).waitFor();
  await page.reload();
  await page.getByTestId("status-transport").getByText(/Отворената смяна е възстановена/).waitFor();
  await page.getByTestId("product-search").fill("380000000001");
  await page.getByTestId("product-search").press("Enter");
  await page.getByTestId("product-search").getAttribute("value").then((value)=>assert.equal(value,""));
  await page.getByTestId("quantity-inc-coffee").click();
  await page.getByTestId("sale-total").getByText("€ 5.00").waitFor();
  await page.getByTestId("quantity-dec-coffee").click();
  await page.getByTestId("sale-pay").click();
  await page.getByTestId("status-transport").getByText(/Успешен фискален бон/).waitFor();

  await page.getByTestId("product-coffee").click();
  await page.getByTestId("split-cash-amount").fill("1,00");
  await page.getByTestId("payment-split").click();
  await page.getByTestId("status-transport").getByText(/Успешен фискален бон/).waitFor();
  assert.deepEqual(splitRequests.slice(-2).map(({ type, amount, terminal_policy }) => ({ type, amount: amount.amount, terminal_policy })), [
    { type: "CASH", amount: "1.00", terminal_policy: "NONE" },
    { type: "CARD", amount: "1.50", terminal_policy: "AUTO_IF_AVAILABLE" },
  ], "touch split tender must preserve exact ordered EUR amounts and terminal policy");

  checkoutMode = "failed";
  await page.getByTestId("product-coffee").click();
  await page.getByTestId("payment-card").click();
  await page.getByTestId("status-transport").getByText(/PAYMENT_REJECTED/).waitFor();
  assert.equal(await page.getByTestId("operation-unknown").count(), 0, "known fiscal rejection must not be mislabeled as ambiguous");

  checkoutMode = "unknown";
  await page.getByTestId("product-coffee").click();
  await page.getByTestId("payment-card").click();
  await page.getByTestId("operation-unknown").getByText(/Не повтаряйте плащането/).waitFor();
  await page.getByTestId("operation-unknown").getByText(/Разрешени действия: READ/).waitFor();
  const callsBeforeReconcile = checkoutCalls;
  await page.getByTestId("reconcile-start").click();
  await page.getByTestId("status-transport").getByText(/резултат е потвърден/).waitFor();
  assert.equal(checkoutCalls, callsBeforeReconcile, "reconciliation must not repeat checkout/payment");
  assert.equal(checkoutCalls, 5, "exactly one payment intent per cash/card leg, including both split legs");
  zCloseMode = "ambiguous";
  await page.getByTestId("shift-toggle").click();
  await page.getByTestId("status-transport").getByText(/Z-отчетът изисква сверяване/).waitFor();
  assert.equal(await page.getByTestId("shift-toggle").getByText("Свери Z").count(), 1, "blocked Z must expose only server-authorized reconciliation");
  await page.getByTestId("shift-toggle").click();
  await page.getByTestId("status-transport").getByText(/Z-000001/).waitFor();
  assert.equal(zCloseCalls, 1, "Z reconciliation must not submit a second close command");
  assert.equal(zReconcileCalls, 1, "blocked Z must reconcile exactly once");
  const recoveryCallsBeforeMissingConfiguration = shiftRecoveryCalls;
  configuration = null;
  await page.reload();
  await page.getByTestId("status-transport").getByText(/Настройте валидна фискална каса/).waitFor();
  assert.equal(shiftRecoveryCalls, recoveryCallsBeforeMissingConfiguration, "missing register configuration must not recover a shift across every register");
  await page.close();
}

async function miniPosProductionLoginFailClosed(browser, appUrl) {
  const page = await browser.newPage({ viewport: { width: 900, height: 700 } });
  let apiCalls = 0;
  await page.route("http://minipos-api.test/**", route => { apiCalls += 1; return json(route, { code: "STATIC_TOKEN_MUST_NOT_BE_USED" }, 500); });
  await page.goto(appUrl);
  await page.getByTestId("operator-login").getByText(/Персонален вход/).waitFor();
  await page.getByText(/OIDC не е конфигуриран/).waitFor();
  assert.equal(await page.getByTestId("operator-login-start").isDisabled(), true, "PROD login must fail closed without OIDC discovery/client configuration");
  assert.equal(await page.getByTestId("shift-toggle").count(), 0, "PROD must not expose sales before an authenticated bound operator session");
  assert.equal(apiCalls, 0, "EXPO_PUBLIC static token must be ignored in PROD");
  await page.close();
}

async function fiscalJourney(browser, appUrl) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } });
  const device = { id: "device-1", name: "DP-150", vendor: "Datecs", model: "DP-150 MX", serial: "DT1", status: "ACTIVE", transport: "EDGE" };
  let location = { id: "location-1", code: "SOF", name: "Sofia", address: "Center", version: 1 };
  let reportCreated = false, reconcileCalls = 0, diagnosticsCalls = 0, provisioningCalls = 0, bleSessionCalls = 0;
  const registerFilters = [];
  let operations = [
    { id: "operation-1", type: "SALE", state: "UNKNOWN", unp: "UNP-1", allowed_actions: ["RECONCILE"], created_at: "2026-08-09T12:00:00Z" },
    { id: "operation-2", type: "SALE", state: "UNKNOWN", unp: "UNP-2", allowed_actions: [], created_at: "2026-08-09T12:01:00Z" },
  ];
  await page.route("http://fiscal-admin.test/**", async (route) => {
    const path = pathOf(route), method = route.request().method();
    if (path.endsWith("/devices")) return json(route, { items: [device] });
    if (path.endsWith("/operations")) { registerFilters.push(new URL(route.request().url()).searchParams.get("register_id")); return json(route, { items: operations }); }
    if (path.endsWith("/operations/operation-1/reconcile") && method === "POST") { reconcileCalls += 1; operations = operations.map((operation)=>operation.id === "operation-1" ? { ...operation, state: "RECONCILING", allowed_actions: [] } : operation); return json(route, operations[0], 202); }
    if (/\/registers\/[^/]+\/reports$/.test(path) && method === "POST") { reportCreated = true; return json(route, { id: "report-2", state: "COMPLETED", fiscal_reference: "X-2" }, 201); }
    if (path.endsWith("/reports")) { registerFilters.push(new URL(route.request().url()).searchParams.get("register_id")); return json(route, { items: [{ id: reportCreated ? "report-2" : "report-1", type: "X", state: "COMPLETED", fiscal_reference: reportCreated ? "X-2" : "X-1", created_at: "2026-08-09T12:00:00Z" }] }); }
    if (path.endsWith("/audit-events")) return json(route, { items: [{ event_id: "audit-1", actor_id: "admin-1", action: "DEVICE_ACTIVATED", object_type: "device", object_id: "device-1", occurred_at: "2026-08-09T12:00:00Z", event_hash: "a".repeat(64) }], page: { next_cursor: null, has_more: false } });
    if (/\/registers\/[^/]+\/connectivity-probes$/.test(path)) return json(route, { state: "READY", hops: { cloud: { state: "READY" }, edge: { state: "READY" } } }, 201);
    if (/\/devices\/[^/]+\/readiness$/.test(path)) return json(route, { ready: true, driver: "READY", fiscal_device: "READY", components: { driver: "READY", fiscal_device: "READY" }, observed_at: "2026-08-09T12:00:00Z" });
    if (/\/devices\/[^/]+\/capabilities$/.test(path)) return json(route, { device_id: "device-1", capabilities: { SALE: "SUPPORTED" }, evidence_state: "DOCUMENTED" });
    if (/\/devices\/[^/]+\/diagnostics$/.test(path)) { diagnosticsCalls += 1; return json(route, { device_id: "device-1", observed_at: "2026-08-09T12:00:00Z", metrics: { uart_errors: 0 }, redactions_applied: true }); }
    if (/\/devices\/[^/]+\/provisioning-sessions$/.test(path) && method === "POST") { provisioningCalls += 1; return json(route, { session_id: "provisioning-1", device_id: "device-1", expires_at: "2026-08-09T12:05:00Z", state: "CREATED" }, 201); }
    if (/\/registers\/[^/]+\/ble-sessions$/.test(path) && method === "POST") { bleSessionCalls += 1; const body = route.request().postDataJSON(); assert.equal(body.operator_id, "operator-1"); assert.equal(body.public_key, "prepared-x25519-public-key"); return json(route, { ble_session_id: "ble-1", tenant_id: "tenant-ui", location_id: "00000000-0000-4000-8000-000000000010", register_id: "00000000-0000-4000-8000-000000000001", edge_id: "edge-1", device_id: "00000000-0000-4000-8000-000000000002", service_uuid: "7b6f0000-7c6d-4c7a-9e4f-424545464953", command_characteristic_uuid: "7b6f0002-7c6d-4c7a-9e4f-424545464953", event_characteristic_uuid: "7b6f0003-7c6d-4c7a-9e4f-424545464953", advertising_identity: "edge-1", protocol_version: "2026-08-07", signed_session_ticket: "signed", expires_at: "2026-08-09T12:05:00Z" }, 201); }
    if (path.endsWith("/country-policy")) return json(route, { country: "BG", currency: "EUR" });
    if (path.endsWith("/tax-groups")) return json(route, { items: [{ code: "B", rate: "20.00" }] });
    if (path.endsWith("/locations")) return json(route, { items: [location] });
    if (path.endsWith("/registers")) return json(route, { items: [{ id: "00000000-0000-4000-8000-000000000001", code: "R01", location_id: "location-1", version: 1 }] });
    if (path.endsWith("/operators")) return json(route, { items: [{ id: "operator-1", code: "0001", first_name: "Ivan", last_name: "Petrov", roles: ["CASHIER"], version: 1 }] });
    if (path.endsWith("/locations/location-1") && method === "PATCH") { location = { ...location, ...route.request().postDataJSON(), version: location.version + 1 }; return json(route, location); }
    return json(route, { code: "UNMOCKED", path, method }, 500);
  });

  await page.goto(appUrl);
  await page.getByTestId("status-fiscal-device").getByText("CORE READY").waitFor();
  await page.getByTestId("readiness-probe").click();
  await page.getByTestId("status-transport").getByText(/Cloud READY.*Edge READY.*Driver READY.*ФУ READY/).waitFor();
  await page.getByTestId("transport-metrics").waitFor();
  await page.getByTestId("fiscal-device-metrics").waitFor();
  await page.getByTestId("device-diagnostics").click();
  await page.getByTestId("diagnostics-result").getByText(/redactions_applied/).waitFor();
  await page.getByTestId("device-provision").click();
  await page.getByTestId("provisioning-result").getByText(/provisioning-1/).waitFor();
  await page.getByTestId("ble-operator-id").fill("operator-1");
  await page.getByTestId("ble-client-public-key").fill("prepared-x25519-public-key");
  await page.getByTestId("ble-session-issue").click();
  await page.getByTestId("ble-session-result").getByText(/ble-1/).waitFor();
  assert.deepEqual({ diagnosticsCalls, provisioningCalls, bleSessionCalls }, { diagnosticsCalls: 1, provisioningCalls: 1, bleSessionCalls: 1 });
  await page.getByTestId("tab-Операции").click();
  await page.getByTestId("operation-unknown").waitFor();
  assert.equal(await page.getByTestId("operation-reconcile-operation-1").count(), 1, "RECONCILE action must be rendered when allowed by backend");
  assert.equal(await page.getByTestId("operation-reconcile-operation-2").count(), 0, "UI must not invent a missing server action");
  await page.getByTestId("operation-reconcile-operation-1").click();
  await page.getByText(/SALE • RECONCILING/).waitFor();
  assert.equal(reconcileCalls, 1, "allowed reconciliation must be submitted exactly once");
  await page.getByTestId("tab-Отчети").click();
  await page.getByRole("button", { name: "Създай X отчет" }).click();
  await page.getByText(/X-2/).waitFor();
  assert.equal(registerFilters.every(value => /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value || "")), true, "BeeFiscal collection filters must use an OpenAPI UUID");
  await page.getByTestId("tab-Одит").click();
  await page.getByTestId("audit-events").waitFor();
  await page.getByTestId("audit-event-audit-1").getByText(/DEVICE_ACTIVATED.*device/).waitFor();
  await page.getByText(/Hash: a{64}/).waitFor();
  await page.getByTestId("tab-Администриране").click();
  await page.getByTestId("admin-editors").waitFor();
  await page.getByTestId("admin-device-kind-terminal").click();
  await page.getByText(/Capability: PAYMENT_TERMINAL.*OPTIONAL_PAYMENT_TERMINAL/).waitFor();
  await page.getByTestId("admin-device-kind-fiscal").click();
  await page.getByTestId("admin-location-code").fill("VAR");
  await page.getByTestId("admin-location-save").click();
  await page.getByTestId("admin-location-code").waitFor();
  assert.equal(await page.getByTestId("admin-location-code").inputValue(), "VAR");
  await page.getByTestId("tab-Настройки").click();
  await page.getByText(/"country": "BG"/).waitFor();
  await page.close();
}

const mini = await staticServer(join(root, ".ui-e2e-web"));
const miniProd = await staticServer(join(root, ".ui-e2e-prod-web"));
await assertNoPublicStaticTokens(join(root, ".ui-e2e-prod-web"));
const fiscal = await staticServer(join(fiscalRoot, ".ui-e2e-web"));
const fiscalProd = await staticServer(join(fiscalRoot, ".ui-e2e-prod-web"));
await assertNoPublicStaticTokens(join(fiscalRoot, ".ui-e2e-prod-web"));
const bundledChromium = chromium.executablePath();
const systemChromium = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const executablePath = existsSync(bundledChromium) ? bundledChromium : systemChromium;
assert.ok(existsSync(executablePath), "install Playwright Chromium or set PLAYWRIGHT_CHROMIUM_EXECUTABLE");
const browser = await chromium.launch({ headless: true, executablePath });
try {
  await miniPosJourney(browser, mini.url);
  await miniPosProductionLoginFailClosed(browser, miniProd.url);
  await fiscalJourney(browser, fiscal.url);
  await fiscalProductionLoginFailClosed(browser, fiscalProd.url);
console.log("UI interaction E2E OK: MiniPOS sale/UNKNOWN/admin + both PROD OIDC fail-closed + BeeFiscal readiness/provisioning/BLE/diagnostics/audit/reports/admin");
} finally {
  await browser.close();
  mini.server.close();
  miniProd.server.close();
  fiscal.server.close();
  fiscalProd.server.close();
}
