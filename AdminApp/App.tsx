import { StatusBar } from "expo-status-bar";
import { useCallback, useEffect, useState } from "react";
import {
  SafeAreaView,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  Pressable,
  View,
} from "react-native";
import { usePlatformOidc } from "./src/usePlatformOidc";
type Device = {
  id: string;
  serial: string;
  state: string;
  tenant_id?: string;
  hardware_revision: string;
  firmware_version: string;
  manufacturing_batch: string;
  device_key_thumbprint: string;
  binding_version: number;
  version: number;
};
type ExternalSystem = {
  id: string;
  code: string;
  display_name: string;
  status: "ACTIVE" | "SUSPENDED" | "REVOKED";
  webhook_url: string;
  webhook_events: string[];
  version: number;
};
const base = (
  process.env.EXPO_PUBLIC_PLATFORM_API_URL || "http://localhost:8080"
).replace(/\/$/, "");
// Platform Admin owns the inventory before a device is assigned to a tenant.
// Transitions carry idempotency and optimistic-version guards so concurrent
// administrators cannot silently overwrite an activation or binding decision.
export default function App() {
  const oidc = usePlatformOidc();
  const [items, setItems] = useState<Device[]>([]),
    [selected, setSelected] = useState<Device | null>(null),
    [state, setState] = useState(""),
    [serial, setSerial] = useState(""),
    [tenant, setTenant] = useState(""),
    [reason, setReason] = useState(""),
    [message, setMessage] = useState("Authenticate with platform identity"),
    [busy, setBusy] = useState(false),
    [section, setSection] = useState<"devices" | "integrations">("devices"),
    [systems, setSystems] = useState<ExternalSystem[]>([]),
    [systemCode, setSystemCode] = useState(""),
    [systemName, setSystemName] = useState(""),
    [webhookUrl, setWebhookUrl] = useState(""),
    [revealedToken, setRevealedToken] = useState("");
  const [selectedSystem,setSelectedSystem]=useState<ExternalSystem|null>(null), [systemAudit,setSystemAudit]=useState<any[]>([]), [systemBindings,setSystemBindings]=useState<any[]>([]), [systemDeliveries,setSystemDeliveries]=useState<any[]>([]);
  const api = useCallback(
    async (path: string, init: RequestInit = {}) => {
      if (!oidc.accessToken) throw new Error("AUTH_REQUIRED");
      const r = await fetch(base + path, {
        ...init,
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${oidc.accessToken}`,
          ...(init.headers || {}),
        },
      });
      const text = await r.text();
      if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
      return text ? JSON.parse(text) : {};
    },
    [oidc.accessToken],
  );
  const load = useCallback(async () => {
    if (!oidc.accessToken) return;
    setBusy(true);
    try {
      const q = new URLSearchParams();
      if (state) q.set("state", state);
      if (serial) q.set("serial", serial);
      const result = await api(`/platform/v1/devices?${q}`);
      setItems(result.items || []);
      setSelected(
        (old) =>
          result.items?.find((x: Device) => x.id === old?.id) ||
          result.items?.[0] ||
          null,
      );
      setMessage(`${result.items?.length || 0} devices`);
    } catch (e) {
      setMessage(String(e));
    } finally {
      setBusy(false);
    }
  }, [api, state, serial, oidc.accessToken]);
  useEffect(() => {
    void load();
  }, [oidc.accessToken]);
  const loadSystems = useCallback(async () => {
    if (!oidc.accessToken) return;
    setBusy(true);
    try {
      const result = await api("/platform/v1/external-systems");
      setSystems(result.items || []);
      setMessage(`${result.items?.length || 0} external systems`);
    } catch (e) {
      setMessage(String(e));
    } finally {
      setBusy(false);
    }
  }, [api, oidc.accessToken]);
  useEffect(() => {
    if (section === "integrations") void loadSystems();
  }, [section, loadSystems]);
  const createSystem = async () => {
    setBusy(true);
    try {
      const result = await api("/platform/v1/external-systems", {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
        body: JSON.stringify({
          code: systemCode,
          display_name: systemName,
          webhook_url: webhookUrl,
          webhook_events: ["integration.command.updated"],
        }),
      });
      setRevealedToken(result.bootstrap_token || "");
      setSystemCode(""); setSystemName(""); setWebhookUrl("");
      await loadSystems();
    } catch (e) { setMessage(String(e)); } finally { setBusy(false); }
  };
  const systemAction = async (system: ExternalSystem, action: "rotate-key" | "suspend" | "resume") => {
    setBusy(true);
    try {
      const result = await api(`/platform/v1/external-systems/${system.id}:${action}`, { method: "POST", headers: { "Idempotency-Key": crypto.randomUUID() } });
      if (result.bootstrap_token) setRevealedToken(result.bootstrap_token);
      await loadSystems();
    } catch (e) { setMessage(String(e)); } finally { setBusy(false); }
  };
  const loadSystemJournal=async(system:ExternalSystem)=>{setSelectedSystem(system);setBusy(true);try{const [audit,bindings,deliveries]=await Promise.all([api(`/platform/v1/external-systems/${system.id}/audit-events`),api(`/platform/v1/external-systems/${system.id}/tenant-bindings`),api(`/platform/v1/external-systems/${system.id}/webhook-deliveries`)]);setSystemAudit(audit.items||[]);setSystemBindings(bindings.items||[]);setSystemDeliveries(deliveries.items||[])}catch(e){setMessage(String(e))}finally{setBusy(false)}};
  const editSystem=async()=>{if(!selectedSystem)return;setBusy(true);try{await api(`/platform/v1/external-systems/${selectedSystem.id}`,{method:"PATCH",headers:{"Idempotency-Key":crypto.randomUUID(),"If-Match":String(selectedSystem.version)},body:JSON.stringify({display_name:systemName||selectedSystem.display_name,webhook_url:webhookUrl||selectedSystem.webhook_url,webhook_events:selectedSystem.webhook_events,version:selectedSystem.version})});setSelectedSystem(null);setSystemName("");setWebhookUrl("");await loadSystems()}catch(e){setMessage(String(e))}finally{setBusy(false)}};
  const requeue=async(id:string)=>{setBusy(true);try{await api(`/platform/v1/webhook-deliveries/${id}:requeue`,{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()}});if(selectedSystem)await loadSystemJournal(selectedSystem)}catch(e){setMessage(String(e))}finally{setBusy(false)}};
  const transition = async (action: string) => {
    if (!selected) return;
    setBusy(true);
    try {
      const result = await api(
        `/platform/v1/devices/${selected.id}:${action}`,
        {
          method: "POST",
          headers: { "Idempotency-Key": crypto.randomUUID() },
          body: JSON.stringify({
            version: selected.version,
            ...(tenant ? { tenant_id: tenant } : {}),
            ...(reason ? { reason } : {}),
          }),
        },
      );
      setSelected(result);
      setMessage(`${result.serial}: ${result.state}`);
      await load();
    } catch (e) {
      setMessage(String(e));
    } finally {
      setBusy(false);
    }
  };
  if (!oidc.accessToken)
    return (
      <SafeAreaView style={s.root}>
        <View style={s.header}>
          <Text style={s.title}>BeeFiscal Platform Admin</Text>
          <Text>{oidc.error || message}</Text>
          <Button
            label="Sign in with OIDC + PKCE"
            onPress={() => void oidc.login()}
            disabled={!oidc.ready}
          />
        </View>
      </SafeAreaView>
    );
  return (
    <SafeAreaView style={s.root}>
      <StatusBar style="dark" />
      <View style={s.header}>
        <Text style={s.title}>BeeFiscal Platform Admin</Text>
        <Text>{message}</Text>
        <View style={s.actions}>
          <Button label="Devices" onPress={() => setSection("devices")} disabled={section === "devices"} />
          <Button label="External systems" onPress={() => setSection("integrations")} disabled={section === "integrations"} />
        </View>
        <Button label="Logout" onPress={oidc.logout} />
      </View>
      {section === "integrations" ? (
        <ScrollView contentContainerStyle={s.integrationBody}>
          <View style={s.detail}>
            <Text style={s.title}>Register external system</Text>
            <TextInput style={s.input} placeholder="System code" value={systemCode} onChangeText={setSystemCode} autoCapitalize="characters" />
            <TextInput style={s.input} placeholder="Display name" value={systemName} onChangeText={setSystemName} />
            <TextInput style={s.input} placeholder="HTTPS webhook URL" value={webhookUrl} onChangeText={setWebhookUrl} autoCapitalize="none" />
            <Button label="Create system" onPress={() => void createSystem()} disabled={busy || !systemCode || !systemName || !webhookUrl.startsWith("https://")} />
            {selectedSystem?<Button label="Save selected system" onPress={()=>void editSystem()} disabled={busy||(!(webhookUrl||selectedSystem.webhook_url).startsWith("https://"))}/>:null}
            {!!revealedToken && <View style={s.secret}><Text style={s.serial}>Copy now — shown only once</Text><Text selectable>{revealedToken}</Text><Button label="I have stored it" onPress={() => setRevealedToken("")} /></View>}
          </View>
          {systems.map((system) => <Pressable key={system.id} style={[s.card,selectedSystem?.id===system.id&&s.selected]} onPress={()=>{setSystemName(system.display_name);setWebhookUrl(system.webhook_url);void loadSystemJournal(system)}}>
            <Text style={s.serial}>{system.code} · {system.display_name}</Text>
            <Text>{system.status} · v{system.version}</Text><Text selectable>{system.webhook_url}</Text>
            <View style={s.actions}>
              <Button label="Rotate key" onPress={() => void systemAction(system,"rotate-key")} disabled={busy} />
              {system.status === "ACTIVE" ? <Button label="Suspend enrollment" onPress={() => void systemAction(system,"suspend")} disabled={busy} /> : <Button label="Resume enrollment" onPress={() => void systemAction(system,"resume")} disabled={busy} />}
            </View>
          </Pressable>)}
          {selectedSystem ? <View style={s.detail}><Text style={s.title}>Audit · {selectedSystem.code}</Text>{systemAudit.map(x=><Text key={x.id} selectable>{x.occurred_at} · {x.action} · {x.actor_subject}</Text>)}<Text style={s.title}>Tenant bindings</Text>{systemBindings.map(x=><Text key={x.id} selectable>{x.tenant_id} · {x.source_company_id} · {x.status}</Text>)}<Text style={s.title}>Webhook deliveries</Text>{systemDeliveries.map(x=><View key={x.id} style={s.card}><Text selectable>{x.event_id} · {x.status} · attempts {x.attempts}</Text>{x.last_error_detail?<Text>{x.last_error_detail}</Text>:null}{x.status==="DEAD"?<Button label="Requeue" onPress={()=>void requeue(x.id)} disabled={busy}/>:null}</View>)}</View>:null}
        </ScrollView>
      ) : <>
      <View style={s.filters}>
        <TextInput
          style={s.input}
          placeholder="Serial"
          value={serial}
          onChangeText={setSerial}
        />
        <TextInput
          style={s.input}
          placeholder="State"
          value={state}
          onChangeText={setState}
        />
        <Button label="Search" onPress={load} disabled={busy} />
      </View>
      <ScrollView horizontal contentContainerStyle={s.body}>
        <View style={s.list}>
          {items.map((d) => (
            <Pressable
              key={d.id}
              style={[s.card, selected?.id === d.id && s.selected]}
              onPress={() => setSelected(d)}
            >
              <Text style={s.serial}>{d.serial}</Text>
              <Text>
                {d.state} · {d.tenant_id || "unassigned"}
              </Text>
              <Text>
                {d.firmware_version} · {d.manufacturing_batch}
              </Text>
            </Pressable>
          ))}
        </View>
        {selected && (
          <View style={s.detail}>
            <Text style={s.title}>{selected.serial}</Text>
            <Text selectable>Device: {selected.id}</Text>
            <Text selectable>Key: {selected.device_key_thumbprint}</Text>
            <Text>Hardware: {selected.hardware_revision}</Text>
            <Text>Firmware: {selected.firmware_version}</Text>
            <Text>
              State: {selected.state} · binding v{selected.binding_version}
            </Text>
            <TextInput
              style={s.input}
              placeholder="Tenant ID"
              value={tenant}
              onChangeText={setTenant}
            />
            <TextInput
              style={s.input}
              placeholder="Mandatory reason"
              value={reason}
              onChangeText={setReason}
            />
            <View style={s.actions}>
              <Button
                label="Assign tenant"
                onPress={() => void transition("assign-tenant")}
                disabled={!tenant || busy}
              />
              <Button
                label="Unassign"
                onPress={() => void transition("unassign-tenant")}
                disabled={busy}
              />
              <Button
                label="Suspend"
                onPress={() => void transition("suspend")}
                disabled={busy}
              />
              <Button
                label="Resume"
                onPress={() => void transition("resume")}
                disabled={busy}
              />
              <Button
                label="Retire"
                onPress={() => void transition("retire")}
                disabled={!reason || busy}
              />
            </View>
          </View>
        )}
      </ScrollView>
      </>}
    </SafeAreaView>
  );
}
function Button({
  label,
  onPress,
  disabled,
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      disabled={disabled}
      onPress={onPress}
      style={[s.button, disabled && s.disabled]}
    >
      <Text style={s.buttonText}>{label}</Text>
    </Pressable>
  );
}
const s = StyleSheet.create({
  root: { flex: 1, backgroundColor: "#f4f7f8" },
  header: { padding: 20, gap: 8 },
  title: { fontSize: 22, fontWeight: "800", color: "#173843" },
  filters: {
    paddingHorizontal: 20,
    flexDirection: "row",
    gap: 10,
    flexWrap: "wrap",
  },
  body: { padding: 20, gap: 20 },
  list: { width: 340, gap: 10 },
  detail: {
    width: 560,
    backgroundColor: "white",
    padding: 20,
    borderRadius: 14,
    gap: 12,
  },
  card: {
    backgroundColor: "white",
    padding: 14,
    borderRadius: 12,
    borderWidth: 2,
    borderColor: "transparent",
  },
  selected: { borderColor: "#147d79" },
  serial: { fontSize: 17, fontWeight: "700" },
  input: {
    minWidth: 180,
    borderWidth: 1,
    borderColor: "#aac0c5",
    borderRadius: 8,
    padding: 11,
    backgroundColor: "white",
  },
  actions: { flexDirection: "row", gap: 8, flexWrap: "wrap" },
  integrationBody: { padding: 20, gap: 14 },
  secret: { padding: 12, borderRadius: 10, backgroundColor: "#fff4cc", gap: 8 },
  button: {
    backgroundColor: "#147d79",
    paddingHorizontal: 16,
    paddingVertical: 12,
    borderRadius: 8,
  },
  buttonText: { color: "white", fontWeight: "700" },
  disabled: { opacity: 0.4 },
});
