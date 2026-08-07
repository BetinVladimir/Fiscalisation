import { StatusBar } from "expo-status-bar";
import { SafeAreaView, StyleSheet, Text, View } from "react-native";

const tenant = {
  name: "Demo Tenant",
  id: "tenant-001",
  status: "active",
  devicesOnline: 24,
  alerts: 2,
};

export default function App() {
  return (
    <SafeAreaView style={styles.container}>
      <Text style={styles.header}>BeeFiscalApp</Text>
      <Text style={styles.subheader}>Monitoring and tenant administration</Text>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>{tenant.name}</Text>
        <Text style={styles.cardText}>Tenant ID: {tenant.id}</Text>
        <Text style={styles.cardText}>Status: {tenant.status}</Text>
        <Text style={styles.cardText}>Devices online: {tenant.devicesOnline}</Text>
        <Text style={styles.cardText}>Open alerts: {tenant.alerts}</Text>
      </View>

      <View style={styles.row}>
        <View style={styles.statBox}>
          <Text style={styles.statLabel}>Revenue Today</Text>
          <Text style={styles.statValue}>$12,940</Text>
        </View>
        <View style={styles.statBox}>
          <Text style={styles.statLabel}>Receipts</Text>
          <Text style={styles.statValue}>1,128</Text>
        </View>
      </View>

      <StatusBar style="dark" />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    paddingHorizontal: 16,
    paddingTop: 24,
    backgroundColor: "#f2f6f8",
  },
  header: {
    fontSize: 30,
    fontWeight: "800",
    color: "#0f2438",
  },
  subheader: {
    fontSize: 14,
    color: "#3e5a6f",
    marginTop: 6,
    marginBottom: 18,
  },
  card: {
    backgroundColor: "#ffffff",
    borderRadius: 14,
    borderWidth: 1,
    borderColor: "#d7e3ea",
    padding: 16,
    marginBottom: 14,
  },
  cardTitle: {
    fontSize: 20,
    fontWeight: "700",
    color: "#122a3f",
    marginBottom: 8,
  },
  cardText: {
    fontSize: 14,
    color: "#314e63",
    marginBottom: 4,
  },
  row: {
    flexDirection: "row",
    gap: 10,
  },
  statBox: {
    flex: 1,
    backgroundColor: "#122a3f",
    borderRadius: 12,
    padding: 14,
  },
  statLabel: {
    color: "#b5d0e2",
    fontSize: 12,
    marginBottom: 6,
  },
  statValue: {
    color: "#ffffff",
    fontSize: 22,
    fontWeight: "700",
  },
});
