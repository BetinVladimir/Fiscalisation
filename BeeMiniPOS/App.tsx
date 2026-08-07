import { StatusBar } from "expo-status-bar";
import { SafeAreaView, StyleSheet, Text, View } from "react-native";

export default function App() {
  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.card}>
        <Text style={styles.title}>BeeMiniPOS</Text>
        <Text style={styles.subtitle}>Expo app initialized</Text>
      </View>
      <StatusBar style="auto" />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#f5f3ea",
    padding: 16,
  },
  card: {
    width: "100%",
    maxWidth: 420,
    backgroundColor: "#ffffff",
    borderRadius: 12,
    padding: 24,
    borderWidth: 1,
    borderColor: "#d9d5c4",
  },
  title: {
    fontSize: 28,
    fontWeight: "700",
    color: "#2d2a21",
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 16,
    color: "#5f5b4f",
  },
});
