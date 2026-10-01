import { Button, Card, Text, TextInput, useTheme } from "react-native-paper";
import { View } from "react-native";
import type { Product } from "./api/client";
import { translations } from "./ui/workTranslations.generated";
import type { Language } from "./ui/PreferenceControls";

export type CartLine = { product: Product; quantity: number; discount: number };

export function ProductGrid({ products, onAdd, language }: { products: Product[]; onAdd: (product: Product) => void; language: Language }) {
  const t = translations[language];
  return <Card mode="outlined" style={{ marginTop: 16 }} testID="sale-products">
    <Card.Content style={{ gap: 12 }}>
      <Text variant="titleLarge" accessibilityRole="header">{t.products}</Text>
      {products.length === 0 ? <Text variant="bodyMedium">{t.noProducts}</Text> : <div className="products">
        {products.map(product => <Card key={product.id} mode="contained" style={{ minWidth: 0 }}>
          <Card.Content style={{ gap: 8 }}>
            <Button mode="contained-tonal" onPress={() => onAdd(product)} testID={`sale-product-${product.id}`} style={{ alignSelf: "stretch" }}>{product.name}</Button>
            <Text variant="bodyMedium">{product.price.amount} EUR</Text>
          </Card.Content>
        </Card>)}
      </div>}
    </Card.Content>
  </Card>;
}

export function PaymentPanel({ cart, total, cardAmount, busy, lastOrderId, language, onUpdate, onCardAmount, onCheckout, onReverse }: {
  cart: CartLine[];
  total: number;
  cardAmount: number;
  busy: boolean;
  lastOrderId: string;
  language: Language;
  onUpdate: (id: string, key: "quantity" | "discount", value: number) => void;
  onCardAmount: (value: number) => void;
  onCheckout: () => void;
  onReverse: () => void;
}) {
  const theme = useTheme();
  const t = translations[language];
  return <Card mode="outlined" style={{ marginVertical: 16 }} testID="sale-payment">
    <Card.Content style={{ gap: 16 }}>
      <Text variant="titleLarge" accessibilityRole="header">{t.currentSale}</Text>
      {cart.length === 0 ? <Text variant="bodyMedium">{t.chooseProduct}</Text> : cart.map(line => <View key={line.product.id} style={{ gap: 10, paddingVertical: 12, borderBottomWidth: 1, borderBottomColor: theme.colors.outlineVariant }}>
        <Text variant="titleMedium">{line.product.name}</Text>
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 12 }}>
          <TextInput mode="outlined" label={t.quantity} accessibilityLabel={t.quantity} keyboardType="number-pad" value={String(line.quantity)} onChangeText={value => { const number = Number(value); if (Number.isFinite(number)) onUpdate(line.product.id, "quantity", number); }} style={{ flexGrow: 1, flexBasis: 150 }} testID={`sale-quantity-${line.product.id}`} />
          <TextInput mode="outlined" label={t.discountEur} accessibilityLabel={t.discountEur} keyboardType="decimal-pad" value={String(line.discount)} onChangeText={value => { const number = Number(value); if (Number.isFinite(number)) onUpdate(line.product.id, "discount", number); }} style={{ flexGrow: 1, flexBasis: 150 }} testID={`sale-discount-${line.product.id}`} />
        </View>
      </View>)}
      <div className="total"><Text variant="titleLarge" testID="sale-total">{t.total}: {total.toFixed(2)} EUR</Text></div>
      <TextInput mode="outlined" label={t.cardRemainder} accessibilityLabel={t.cardRemainder} keyboardType="decimal-pad" value={String(cardAmount)} onChangeText={value => { const number = Number(value); if (Number.isFinite(number)) onCardAmount(number); }} testID="sale-card-amount" />
      <Button mode="contained" contentStyle={{ minHeight: 56 }} loading={busy} disabled={busy || !cart.length || total <= 0} onPress={onCheckout} testID="sale-checkout">{t.checkoutFiscal}</Button>
      {lastOrderId ? <Button mode="outlined" disabled={busy} onPress={onReverse} testID="sale-reverse">{t.reverseLastSale}</Button> : null}
    </Card.Content>
  </Card>;
}
