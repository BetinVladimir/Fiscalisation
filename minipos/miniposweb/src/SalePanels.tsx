import { parseSaleNumber } from './saleNumber';
import { useEffect, useRef, useState } from 'react';
import { FormField as TextInput } from './ui/form-field';
import { AppButton as Button } from './ui/app-button';
import { Card, Divider, Text } from "react-native-paper";
import { Platform, View } from "react-native";
import type { Product } from "./api/client";
import { translations } from "./ui/workTranslations.generated";
import type { Language } from "./ui/PreferenceControls";

export type CartLine = { product: Product; quantity: number; discount: number };

export function ProductGrid({ products, onAdd, language, busy = false }: { busy?: boolean; products: Product[]; onAdd: (product: Product) => void; language: Language }) {
  const mounted=useRef(false),latest=useRef({products,onAdd,busy});latest.current={products,onAdd,busy};
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  const actionable=()=>mounted.current&&!latest.current.busy;
  const t = translations[language];
  return <Card mode="outlined" style={{ marginTop: 16 }} testID="sale-products">
    <Card.Content style={{ gap: 12 }}>
      <Text variant="titleLarge" accessibilityRole="header">{t.products}</Text>
      {products.length === 0 ? <Text variant="bodyMedium">{t.noProducts}</Text> : <div className="products">
        {products.map(product => <Card key={product.id} mode="contained" style={{ minWidth: 0 }}>
          <Card.Content style={{ gap: 8 }}>
            <Button disabled={busy} mode="contained-tonal" onPress={() => { if (actionable() && latest.current.products.includes(product)) latest.current.onAdd(product); }} testID={`sale-product-${product.id}`} style={{ alignSelf: "stretch" }}>{product.name}</Button>
            <Text variant="bodyMedium">{product.price.amount} EUR</Text>
          </Card.Content>
        </Card>)}
      </div>}
    </Card.Content>
  </Card>;
}

export function PaymentPanel({ cart, total, cardAmount, busy, blocked=false, lastOrderId, language, onUpdate, onCardAmount, onCheckout, onReverse }: {
  cart: CartLine[];
  total: number;
  cardAmount: number;
  busy: boolean;
  blocked?: boolean;
  lastOrderId: string;
  language: Language;
  onUpdate: (id: string, key: "quantity" | "discount", value: number) => void;
  onCardAmount: (value: number) => void;
  onCheckout: () => void;
  onReverse: () => void;
}) {
  const mounted=useRef(false),latestBusy=useRef(busy),latestBlocked=useRef(blocked);latestBusy.current=busy;latestBlocked.current=blocked;
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  const actionable=()=>mounted.current&&!latestBusy.current&&!latestBlocked.current;
  const t = translations[language];
  const [drafts,setDrafts]=useState<Record<string,{text:string;value:number}>>({});
  const values:Record<string,number>={card:cardAmount};
  for(const line of cart){values[`quantity:${line.product.id}`]=line.quantity;values[`discount:${line.product.id}`]=line.discount;}
  const draft=(key:string)=>drafts[key]?.value===values[key]?drafts[key].text:String(values[key]);
  useEffect(()=>{setDrafts(previous=>{
    const entries=Object.entries(previous).filter(([key,value])=>Object.hasOwn(values,key)&&value.value===values[key]);
    return entries.length===Object.keys(previous).length?previous:Object.fromEntries(entries);
  });},[cart,cardAmount]);
  const quantityValid=(line:CartLine)=>{const n=parseSaleNumber(draft(`quantity:${line.product.id}`),3);return n!==null&&n>=1;};
  const discountValid=(line:CartLine)=>{const n=parseSaleNumber(draft(`discount:${line.product.id}`),2);return n!==null&&n<=Number(line.product.price.amount)*line.quantity;};
  const cardValue=parseSaleNumber(draft('card'),2),cardValid=cardValue!==null&&cardValue<=total;
  const changeNumber=(key:string,text:string,decimals:number,admit:(value:number)=>boolean,commit:(value:number)=>void)=>{
    if(!actionable())return;
    const parsed=parseSaleNumber(text,decimals),valid=parsed!==null&&admit(parsed);
    setDrafts(previous=>({...previous,[key]:{text,value:valid?parsed:values[key]}}));
    if(valid)commit(parsed);
  };
  const validPayment=cardValid&&cart.every(line=>quantityValid(line)&&discountValid(line))&&cart.length>0&&Number.isFinite(total)&&total>0&&Number.isFinite(cardAmount)&&cardAmount>=0&&cardAmount<=total&&cart.every(line=>Number.isFinite(line.quantity)&&line.quantity>0&&Number.isFinite(line.discount)&&line.discount>=0&&Number.isFinite(Number(line.product.price.amount))&&Number(line.product.price.amount)>=0&&line.discount<=Number(line.product.price.amount)*line.quantity);
  return <Card mode="outlined" style={{ marginVertical: 16 }} testID="sale-payment">
    <Card.Content style={{ gap: 16 }}>
      <Text variant="titleLarge" accessibilityRole="header">{t.currentSale}</Text>
      {cart.length === 0 ? <Text variant="bodyMedium">{t.chooseProduct}</Text> : cart.map(line => <View key={line.product.id} style={{ gap: 10, paddingVertical: 12 }}>
        <Text variant="titleMedium">{line.product.name}</Text>
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 12 }}>
          <TextInput editable={!busy&&!blocked} mode="outlined" label={t.quantity} accessibilityLabel={t.quantity} keyboardType="number-pad" value={draft(`quantity:${line.product.id}`)} errorMessage={!quantityValid(line)?`${t.quantity}: ≥ 1 (1.000)`:undefined} onChangeText={value => { if(cart.includes(line))changeNumber(`quantity:${line.product.id}`,value,3,n=>n>=1,n=>onUpdate(line.product.id,'quantity',n)); }} containerStyle={{ flexGrow: 1, flexBasis: 150, minWidth: 0 }} testID={`sale-quantity-${line.product.id}`} />
          <TextInput editable={!busy&&!blocked} mode="outlined" label={`${t.discountEur} ${line.product.name}`} accessibilityLabel={`${t.discountEur} ${line.product.name}`} keyboardType="decimal-pad" value={draft(`discount:${line.product.id}`)} errorMessage={!discountValid(line)?`${t.discountEur}: 0–${(Number(line.product.price.amount)*line.quantity).toFixed(2)} EUR`:undefined} onChangeText={value => { if(cart.includes(line))changeNumber(`discount:${line.product.id}`,value,2,n=>n<=Number(line.product.price.amount)*line.quantity,n=>onUpdate(line.product.id,'discount',n)); }} containerStyle={{ flexGrow: 1, flexBasis: 150, minWidth: 0 }} testID={`sale-discount-${line.product.id}`} />
        </View>
        <Divider testID={`sale-line-divider-${line.product.id}`}/>
      </View>)}
      <View testID="sale-total-container" style={{marginVertical:8,minWidth:0,maxWidth:"100%"}}><Text variant="titleLarge" testID="sale-total" style={{display:"flex",minWidth:0,maxWidth:"100%",flexShrink:1,flexWrap:"wrap",...(Platform.OS==="web"?{overflowWrap:"anywhere"} as any:{})}}>{t.total}: {total.toFixed(2)} EUR</Text></View>
      <TextInput editable={!busy&&!blocked} mode="outlined" label={t.cardRemainder} accessibilityLabel={t.cardRemainder} keyboardType="decimal-pad" value={draft('card')} errorMessage={!cardValid?`${t.cardRemainder}: 0–${Number.isFinite(total)?total.toFixed(2):'—'} EUR`:undefined} onChangeText={value => changeNumber('card',value,2,n=>n<=total,onCardAmount)} testID="sale-card-amount" />
      <Button mode="contained" contentStyle={{ minHeight: 56 }} loading={busy} disabled={busy || blocked || !validPayment} onPress={() => { if(actionable()&&validPayment)onCheckout(); }} testID="sale-checkout">{t.checkoutFiscal}</Button>
      {lastOrderId ? <Button mode="outlined" disabled={busy||blocked} onPress={() => { if(actionable()&&lastOrderId)onReverse(); }} testID="sale-reverse">{t.reverseLastSale}</Button> : null}
    </Card.Content>
  </Card>;
}
