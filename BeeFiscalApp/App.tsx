import { StatusBar } from "expo-status-bar";
import { useCallback, useEffect, useState } from "react";
import {
  Pressable,
  SafeAreaView,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { fetchWithTimeout } from "./src/http";
import { useAdminOidc } from "./src/adminOidc";

const base = (
  process.env.EXPO_PUBLIC_FISCAL_API_URL || "http://localhost:8080/public/v1"
).replace(/\/$/, "");
const version = "2026-08-07";
const appEnv = (process.env.EXPO_PUBLIC_APP_ENV || "dev").toLowerCase();
const prodMode = appEnv === "prod";
const devAuthToken = prodMode ? "" : process.env.EXPO_PUBLIC_FISCAL_AUTH_TOKEN || "";
let runtimeAuthToken = devAuthToken;
let runtimeUnauthorized: (() => void) | undefined;
type Item = Record<string, unknown>;
type Page = { items?: Item[] };
type AdminLists = {
  locations: Item[];
  registers: Item[];
  operators: Item[];
  devices: Item[];
};

async function call(path: string, init: RequestInit = {}) {
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
      ...(runtimeAuthToken ? { Authorization: `Bearer ${runtimeAuthToken}` } : {}),
      ...(init.headers || {}),
    },
  });
  const text = await r.text();
  if (r.status === 401 && prodMode) runtimeUnauthorized?.();
  if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
  return text ? JSON.parse(text) : {};
}
const label = (v: unknown) =>
  typeof v === "string" ? v : typeof v === "number" ? String(v) : "—";

export default function App() {
  const oidc = useAdminOidc();
  const roles = prodMode ? oidc.roles : ["ADMIN", "SUPERVISOR", "AUDITOR"];
  const hasRole = (...allowed: string[]) => roles.some((role) => allowed.includes(role));
  const canAdminister = hasRole("ADMIN");
  const canRunReports = hasRole("SUPERVISOR", "ADMIN");
  const canReconcile = hasRole("SUPERVISOR", "ADMIN");
  const tabs = [
    "Устройства",
    "Операции",
    ...(hasRole("SUPERVISOR", "ADMIN", "AUDITOR") ? ["Отчети"] : []),
    ...(canAdminister ? ["Администриране"] : []),
    "Настройки",
  ];
  runtimeUnauthorized = () => oidc.logout();
  const [tab, setTab] = useState("Устройства"),
    [message, setMessage] = useState("Зареждане на данните от публичния API…"),
    [core, setCore] = useState("CHECKING"),
    [items, setItems] = useState<Item[]>([]),
    [selected, setSelected] = useState<Item | null>(null),
    [register, setRegister] = useState("FD000001"),
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
    [deviceId, setDeviceId] = useState(""),
    [adminBusy, setAdminBusy] = useState(false),
    [loading, setLoading] = useState(false);
  const refresh = useCallback(
    async (next = tab) => {
      setLoading(true);
      try {
        if (next === "Администриране" && !canAdminister) throw new Error("ROLE_ADMIN_REQUIRED");
        if (next === "Отчети" && !hasRole("SUPERVISOR", "ADMIN", "AUDITOR")) throw new Error("ROLE_REPORT_READ_REQUIRED");
        if (next === "Устройства") {
          const v: Page = await call("/devices?limit=100");
          setItems(v.items || []);
          setSelected((x) => x || v.items?.[0] || null);
        }
        if (next === "Операции") {
          const v: Page = await call(
            `/operations?limit=100${register ? `&register_id=${encodeURIComponent(register)}` : ""}`,
          );
          setItems(v.items || []);
        }
        if (next === "Отчети") {
          const v: Page = await call(
            `/reports?limit=100${register ? `&register_id=${encodeURIComponent(register)}` : ""}`,
          );
          setItems(v.items || []);
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
          const [locations, registers, operators, devices]: Page[] =
            await Promise.all([
              call("/locations?limit=100"),
              call("/registers?limit=100"),
              call("/operators?limit=100"),
              call("/devices?limit=100"),
            ]);
          const lists = {
            locations: locations.items || [],
            registers: registers.items || [],
            operators: operators.items || [],
            devices: devices.items || [],
          };
          setAdminLists(lists);
          setLocationId((x) => x || label(lists.locations[0]?.id));
          setRegister((x) =>
            x === "FD000001" ? label(lists.registers[0]?.id) : x,
          );
          setDeviceId((x) => x || label(lists.devices[0]?.id));
          setOperatorId((x) => x || label(lists.operators[0]?.id));
          setItems([]);
        }
        setMessage(`${next}: данните са обновени от BeeFiscal API`);
        setCore("CORE READY");
      } catch (e) {
        setCore("CORE OFFLINE");
        setMessage(`API отказ: ${e instanceof Error ? e.message : String(e)}`);
      } finally {
        setLoading(false);
      }
    },
    [register, tab, canAdminister, roles.join(",")],
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
    void refresh(next);
  };
  const probe = async () => {
    if (!selected) {
      setMessage("Няма регистрирано устройство за probe");
      return;
    }
    setMessage("Проверка на cloud route, Edge, driver и крайното ФУ…");
    try {
      const id = label(selected.id);
      const route = await call(
        `/registers/${encodeURIComponent(register)}/connectivity-probes`,
        { method: "POST", body: "{}" },
      );
      const device = await call(`/devices/${encodeURIComponent(id)}/readiness`);
      setMessage(
        `Cloud ${route.hops?.cloud?.state || route.state} → Edge ${route.hops?.edge?.state || "?"} → Driver ${device.driver} → ФУ ${device.fiscal_device}`,
      );
    } catch (e) {
      setMessage(
        `Probe отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
    }
  };
  const report = async (type: string) => {
    if (!canRunReports) {
      setMessage("Командата за отчет изисква роля SUPERVISOR или ADMIN");
      return;
    }
    setMessage(`${type} отчет…`);
    try {
      const v = await call(
        `/registers/${encodeURIComponent(register)}/reports`,
        { method: "POST", body: JSON.stringify({ type }) },
      );
      setMessage(`${type}: ${v.state} • ${v.fiscal_reference || v.id}`);
      await refresh("Отчети");
    } catch (e) {
      setMessage(
        `Отчетът е отказан: ${e instanceof Error ? e.message : String(e)}`,
      );
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
  if (prodMode && !oidc.accessToken) {
    return (
      <SafeAreaView style={s.root}>
        <StatusBar style="dark" />
        <View testID="admin-login" style={s.login}>
          <Text style={s.loginTitle}>BeeFiscal • административен вход</Text>
          <Text style={s.loginText}>OIDC Authorization Code + PKCE. Публични статични токени са забранени в production build.</Text>
          {!oidc.configured ? <Text style={s.loginError}>OIDC не е конфигуриран: задайте issuer и client ID.</Text> : null}
          {oidc.error ? <Text style={s.loginError}>{oidc.error}</Text> : null}
          <Pressable testID="admin-login-start" accessibilityRole="button" accessibilityLabel="Влез с административен профил" accessibilityState={{ disabled: !oidc.ready }} disabled={!oidc.ready} style={s.loginButton} onPress={() => void oidc.login()}>
            <Text style={s.loginButtonText}>Влез</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    );
  }
  return (
    <SafeAreaView style={s.root}>
      <StatusBar style="dark" />
      <View style={s.header}>
        <View>
          <Text style={s.title}>BeeFiscal</Text>
          <Text style={s.sub}>
            BG • EUR • публичен API • без локално симулирани записи
          </Text>
        </View>
        <View
          testID="status-fiscal-device"
          accessibilityRole="text"
          accessibilityLiveRegion="polite"
          accessibilityLabel={`Състояние на Fiscal Core: ${core}`}
          style={[s.ready, core.includes("OFFLINE") && s.offline]}
        >
          <Text style={s.readyText}>{core}</Text>
        </View>
        {prodMode ? <Pressable testID="admin-logout" accessibilityRole="button" accessibilityLabel="Излез от административния профил" style={s.logout} onPress={() => { runtimeAuthToken = ""; oidc.logout(); }}><Text style={s.readyText}>Излез</Text></Pressable> : null}
      </View>
      <View style={s.body}>
        <View style={s.nav}>
          {tabs.map((x) => (
            <Pressable
              key={x}
              testID={`tab-${x}`}
              accessibilityRole="tab"
              accessibilityState={{ selected: tab === x }}
              accessibilityLabel={`Отвори ${x}`}
              style={[s.navItem, tab === x && s.active]}
              onPress={() => changeTab(x)}
            >
              <Text style={[s.navText, tab === x && s.activeText]}>{x}</Text>
            </Pressable>
          ))}
        </View>
        <ScrollView style={s.content}>
          {loading ? (
            <Text
              testID="screen-loading"
              accessibilityLiveRegion="polite"
              style={s.loadingText}
            >
              Зареждане от BeeFiscal API…
            </Text>
          ) : null}
          <View style={s.toolbar}>
            <Text style={s.section}>{tab}</Text>
            <TextInput
              accessibilityLabel="Fiscal register ID"
              value={register}
              onChangeText={setRegister}
              style={s.input}
              placeholder="Register ID"
              autoCapitalize="characters"
            />
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={`Обнови ${tab}`}
              accessibilityState={{ disabled: loading }}
              disabled={loading}
              style={s.secondary}
              onPress={() => refresh()}
            >
              <Text style={s.probeText}>{loading ? "…" : "Обнови"}</Text>
            </Pressable>
          </View>
          {tab === "Устройства" ? (
            items.map((d) => (
              <Pressable
                key={label(d.id)}
                accessibilityRole="button"
                accessibilityState={{ selected: selected?.id === d.id }}
                accessibilityLabel={`${label(d.vendor)} ${label(d.model)}, състояние ${label(d.status || d.state)}`}
                style={[s.card, selected?.id === d.id && s.selected]}
                onPress={() => {
                  setSelected(d);
                  setMessage(`${label(d.name || d.id)} е избрано`);
                }}
              >
                <View>
                  <Text style={s.device}>
                    {label(d.name || d.model || d.id)}
                  </Text>
                  <Text style={s.meta}>
                    {label(d.vendor)} • {label(d.model)} •{" "}
                    {label(d.connection_type || d.transport)}
                  </Text>
                </View>
                <View style={s.state}>
                  <Text style={s.stateText}>{label(d.status || d.state)}</Text>
                </View>
              </Pressable>
            ))
          ) : tab === "Операции" ? (
            items.map((o) => (
              <View
                key={label(o.id)}
                testID={
                  String(o.state).includes("UNKNOWN") && Array.isArray(o.allowed_actions) && o.allowed_actions.includes("RECONCILE")
                    ? "operation-unknown"
                    : undefined
                }
                accessibilityLabel={`Операция ${label(o.type)}, състояние ${label(o.state)}, УНП ${label(o.unp)}`}
                style={s.card}
              >
                <View>
                  <Text style={s.device}>
                    {label(o.type)} • {label(o.state)}
                  </Text>
                  <Text style={s.meta}>
                    {label(o.id)} • УНП {label(o.unp)} • {label(o.created_at)}
                  </Text>
                  <Text style={s.meta}>
                    Разрешени действия: {Array.isArray(o.allowed_actions) && o.allowed_actions.length > 0 ? o.allowed_actions.join(", ") : "няма"}
                  </Text>
                </View>
                {canReconcile && Array.isArray(o.allowed_actions) && o.allowed_actions.includes("RECONCILE") && (
                  <Pressable
                    testID={`operation-reconcile-${label(o.id)}`}
                    accessibilityRole="button"
                    accessibilityLabel={`Reconcile операция ${label(o.id)}`}
                    accessibilityState={{ disabled: loading }}
                    disabled={loading}
                    style={s.action}
                    onPress={() => void reconcileOperation(o)}
                  >
                    <Text style={s.probeText}>Reconcile</Text>
                  </Pressable>
                )}
              </View>
            ))
          ) : tab === "Отчети" ? (
            <>
              <View style={s.panel}>
                <Text style={s.panelTitle}>
                  Команда към фискалното устройство
                </Text>
                <Text style={s.meta}>
                  UI показва само резултати и артефакти, получени от BeeFiscal
                  API.
                </Text>
                {canRunReports ? <View style={s.actions}>
                  {["X", "Z", "KLEN", "FISCAL_MEMORY"].map((x) => (
                    <Pressable
                      key={x}
                      accessibilityRole="button"
                      accessibilityLabel={`Създай ${x} отчет`}
                      style={s.action}
                      onPress={() => report(x)}
                    >
                      <Text style={s.probeText}>{x}</Text>
                    </Pressable>
                  ))}
                </View> : <Text style={s.meta}>Преглед само за одитор; създаването на отчет изисква SUPERVISOR или ADMIN.</Text>}
              </View>
              {items.map((r) => (
                <View key={label(r.id)} style={s.card}>
                  <View>
                    <Text style={s.device}>
                      {label(r.type)} • {label(r.state)}
                    </Text>
                    <Text style={s.meta}>
                      {label(r.fiscal_reference || r.id)} •{" "}
                      {label(r.created_at)}
                    </Text>
                  </View>
                </View>
              ))}
            </>
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
                <Field
                  label="ID на точката"
                  value={locationId}
                  onChangeText={setLocationId}
                  testID="admin-register-location"
                />
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
                        current
                          ? `/registers/${encodeURIComponent(register)}`
                          : "/registers",
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
                  }}
                />
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
                            kind: current?.kind || "FISCAL_DEVICE",
                            vendor: deviceVendor,
                            model: deviceModel,
                            serial: deviceSerial,
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
                <Field
                  label="Register ID"
                  value={register}
                  onChangeText={setRegister}
                  testID="admin-binding-register"
                />
                <Field
                  label="Device ID"
                  value={deviceId}
                  onChangeText={setDeviceId}
                  testID="admin-binding-device"
                />
                <Action
                  label="Привържи ФУ"
                  disabled={adminBusy}
                  testID="admin-binding-save"
                  onPress={() =>
                    mutateAdmin("Привързване на ФУ", () =>
                      call(
                        `/registers/${encodeURIComponent(register)}/bindings`,
                        {
                          method: "POST",
                          body: JSON.stringify({
                            device_id: deviceId,
                            role: "FISCAL_DEVICE",
                          }),
                        },
                      ),
                    )
                  }
                />
              </AdminCard>
            </View>
          ) : (
            <View style={s.panel}>
              <Text style={s.panelTitle}>Ефективна регулаторна политика</Text>
              <Text style={s.meta}>
                ID: {label(policy?.policy_id || policy?.id)} • валута{" "}
                {label(policy?.currency)} • държава{" "}
                {label(policy?.country_code)}
              </Text>
              <Text style={s.json}>
                {policy
                  ? JSON.stringify(policy, null, 2)
                  : "Политиката не е заредена"}
              </Text>
            </View>
          )}
          {items.length === 0 &&
          tab !== "Настройки" &&
          tab !== "Отчети" &&
          tab !== "Администриране" ? (
            <Text style={s.empty}>Няма записи в API за избрания филтър.</Text>
          ) : null}
        </ScrollView>
      </View>
      <View
        testID="status-transport"
        accessibilityRole="text"
        accessibilityLiveRegion="polite"
        accessibilityLabel={`Състояние: ${message}`}
        style={s.footer}
      >
        <Text style={s.footerText}>{message}</Text>
        <Pressable
          testID="readiness-probe"
          accessibilityRole="button"
          accessibilityLabel="Провери cloud, Edge, driver и крайното фискално устройство"
          style={s.probe}
          onPress={probe}
        >
          <Text style={s.probeText}>Сквозен probe</Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}

function Field({
  label: fieldLabel,
  value,
  onChangeText,
  testID,
}: {
  label: string;
  value: string;
  onChangeText: (value: string) => void;
  testID?: string;
}) {
  return (
    <TextInput
      testID={testID}
      accessibilityLabel={fieldLabel}
      value={value}
      onChangeText={onChangeText}
      style={s.adminInput}
      placeholder={fieldLabel}
    />
  );
}

function Action({
  label: actionLabel,
  onPress,
  disabled,
  testID,
}: {
  label: string;
  onPress: () => void;
  disabled: boolean;
  testID: string;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={actionLabel}
      accessibilityState={{ disabled }}
      disabled={disabled}
      style={[s.action, disabled && s.disabled]}
      onPress={onPress}
    >
      <Text style={s.probeText}>{actionLabel}</Text>
    </Pressable>
  );
}

function AdminCard({
  title,
  count,
  children,
}: {
  title: string;
  count: number;
  children: React.ReactNode;
}) {
  return (
    <View style={s.adminCard}>
      <Text style={s.panelTitle}>{title}</Text>
      {count > 0 ? <Text style={s.meta}>Регистрирани: {count}</Text> : null}
      {children}
    </View>
  );
}

function Choices({
  items,
  selectedId,
  labelFor,
  onSelect,
}: {
  items: Item[];
  selectedId: string;
  labelFor: (item: Item) => string;
  onSelect: (item: Item) => void;
}) {
  if (items.length === 0) {
    return <Text style={s.meta}>Все още няма записи.</Text>;
  }
  return (
    <ScrollView
      horizontal
      accessibilityLabel="Съществуващи записи"
      contentContainerStyle={s.choices}
    >
      {items.map((item) => (
        <Pressable
          key={label(item.id)}
          accessibilityRole="button"
          accessibilityState={{ selected: item.id === selectedId }}
          style={[s.choice, item.id === selectedId && s.selected]}
          onPress={() => onSelect(item)}
        >
          <Text style={s.meta}>{labelFor(item)}</Text>
        </Pressable>
      ))}
    </ScrollView>
  );
}

const s = StyleSheet.create({
  root: { flex: 1, backgroundColor: "#eef3f4" },
  login: { alignSelf: "center", width: "90%", maxWidth: 540, marginTop: 80, padding: 28, backgroundColor: "white", borderRadius: 16, gap: 18 },
  loginTitle: { fontSize: 24, fontWeight: "900", color: "#102b38" },
  loginText: { fontSize: 16, color: "#526b76", lineHeight: 23 },
  loginError: { fontSize: 15, color: "#9a2f24", fontWeight: "700" },
  loginButton: { minHeight: 56, backgroundColor: "#17613a", borderRadius: 10, alignItems: "center", justifyContent: "center" },
  loginButtonText: { color: "white", fontSize: 17, fontWeight: "800" },
  logout: { minHeight: 48, paddingHorizontal: 14, backgroundColor: "#285266", borderRadius: 9, justifyContent: "center" },
  header: {
    padding: 20,
    backgroundColor: "white",
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    borderBottomWidth: 1,
    borderColor: "#d7e0e3",
  },
  title: { fontSize: 29, fontWeight: "900", color: "#102b38" },
  sub: { color: "#58717c", marginTop: 3 },
  ready: {
    backgroundColor: "#dff3e7",
    padding: 10,
    borderRadius: 20,
    minHeight: 48,
    justifyContent: "center",
  },
  offline: { backgroundColor: "#f3d7d7" },
  readyText: { color: "#17613a", fontWeight: "900" },
  body: { flex: 1, flexDirection: "row" },
  nav: { width: 190, backgroundColor: "#102b38", padding: 12, gap: 6 },
  navItem: {
    padding: 14,
    borderRadius: 10,
    minHeight: 48,
    justifyContent: "center",
  },
  active: { backgroundColor: "#285266" },
  navText: { color: "#abc0ca", fontWeight: "700" },
  activeText: { color: "white" },
  content: { flex: 1, padding: 20 },
  toolbar: {
    flexDirection: "row",
    gap: 10,
    alignItems: "center",
    marginBottom: 16,
  },
  section: { fontSize: 25, fontWeight: "900", color: "#102b38", flex: 1 },
  input: {
    backgroundColor: "white",
    borderWidth: 1,
    borderColor: "#c8d5da",
    padding: 10,
    borderRadius: 9,
    minWidth: 150,
    minHeight: 48,
  },
  secondary: {
    backgroundColor: "#607884",
    padding: 11,
    borderRadius: 9,
    minHeight: 48,
    justifyContent: "center",
  },
  card: {
    backgroundColor: "white",
    padding: 18,
    borderRadius: 14,
    marginBottom: 12,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    borderWidth: 1,
    borderColor: "#d7e0e3",
  },
  selected: { borderWidth: 2, borderColor: "#17613a" },
  device: { fontSize: 18, fontWeight: "800", color: "#163645" },
  meta: { color: "#607884", marginTop: 5 },
  state: {
    paddingHorizontal: 11,
    paddingVertical: 8,
    borderRadius: 8,
    backgroundColor: "#285266",
  },
  stateText: { color: "white", fontSize: 11, fontWeight: "900" },
  panel: {
    backgroundColor: "white",
    borderRadius: 14,
    padding: 20,
    marginBottom: 14,
  },
  panelTitle: { fontSize: 18, fontWeight: "800" },
  empty: { paddingVertical: 60, textAlign: "center", color: "#78909a" },
  loadingText: {
    padding: 10,
    marginBottom: 10,
    color: "#425d69",
    backgroundColor: "#e2edf0",
    borderRadius: 8,
  },
  actions: { flexDirection: "row", gap: 10, marginTop: 20, flexWrap: "wrap" },
  action: {
    backgroundColor: "#285266",
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
    backgroundColor: "white",
    borderRadius: 14,
    padding: 18,
    gap: 10,
    minWidth: 280,
    flexGrow: 1,
    flexBasis: 320,
  },
  adminInput: {
    backgroundColor: "#f8fbfc",
    borderWidth: 1,
    borderColor: "#c8d5da",
    borderRadius: 9,
    paddingHorizontal: 12,
    minHeight: 48,
  },
  choices: { gap: 8, paddingVertical: 2 },
  choice: {
    borderWidth: 1,
    borderColor: "#c8d5da",
    borderRadius: 8,
    paddingHorizontal: 10,
    minHeight: 48,
    justifyContent: "center",
  },
  json: { marginTop: 16, fontFamily: "monospace", color: "#294955" },
  footer: {
    padding: 12,
    backgroundColor: "white",
    borderTopWidth: 1,
    borderColor: "#d7e0e3",
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  footerText: { color: "#425d69", flex: 1 },
  probe: {
    backgroundColor: "#17613a",
    padding: 11,
    borderRadius: 9,
    minHeight: 56,
    justifyContent: "center",
  },
  probeText: { color: "white", fontWeight: "800" },
});
