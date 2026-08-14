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
    [busy, setBusy] = useState(false);
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
        <Text style={s.title}>Platform Devices</Text>
        <Text>{message}</Text>
        <Button label="Logout" onPress={oidc.logout} />
      </View>
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
  button: {
    backgroundColor: "#147d79",
    paddingHorizontal: 16,
    paddingVertical: 12,
    borderRadius: 8,
  },
  buttonText: { color: "white", fontWeight: "700" },
  disabled: { opacity: 0.4 },
});
