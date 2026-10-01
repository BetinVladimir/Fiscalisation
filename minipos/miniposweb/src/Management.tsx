import { useState } from "react";
import { Button, Card, HelperText, List, Text, TextInput } from "react-native-paper";
import { View } from "react-native";
import { MiniPosClient, type Employee, type Product } from "./api/client";
import { translations } from "./ui/workTranslations.generated";
import type { Language } from "./ui/PreferenceControls";

export function Management({kind,products,employees,api,onProducts,onEmployees,language}:{kind:"products"|"employees";products:Product[];employees:Employee[];api:MiniPosClient;onProducts:(v:Product[])=>void;onEmployees:(v:Employee[])=>void;language:Language}) {
  const t = translations[language];
  const [adding,setAdding]=useState(false), [name,setName]=useState(""), [price,setPrice]=useState(""), [first,setFirst]=useState(""), [last,setLast]=useState(""), [code,setCode]=useState("");
  const [busy,setBusy]=useState(false), [error,setError]=useState("");
  const openForm=()=>{setError("");setName("");setPrice("");setFirst("");setLast("");setCode("");setAdding(true);};
  const cancel=()=>{setAdding(false);setError("");};
  async function save(){
    setBusy(true);setError("");
    try {
      if(kind==="products"){
        const created=await api.createProduct({name,sku:crypto.randomUUID().slice(0,8),unit:"pcs",price:{amount:Number(price).toFixed(2),currency:"EUR"},tax_group:"B",status:"ACTIVE"});
        onProducts([...products,created]);
      } else {
        const created=await api.createEmployee({first_name:first,last_name:last,operator_code:code,roles:["CASHIER"],status:"ACTIVE"});
        onEmployees([...employees,created]);
      }
      setAdding(false);
    } catch (reason) {setError(reason instanceof Error ? reason.message || t.saveFailed : String(reason));}
    finally {setBusy(false);}
  }
  return <Card mode="outlined" style={{marginVertical:16}}><Card.Content style={{gap:16}}>
    <View style={{flexDirection:"row",flexWrap:"wrap",alignItems:"center",justifyContent:"space-between",gap:8}}>
      <Text variant="titleLarge" accessibilityRole="header">{kind==="products"?t.products:t.employees}</Text>
      {!adding ? <Button mode="contained" onPress={openForm} testID="management-add">{t.add}</Button> : null}
    </View>
    {!adding ? <View testID="management-list" style={{gap:4}}>
      {(kind==="products" ? products : employees).length === 0
        ? <Text variant="bodyMedium">{kind==="products" ? t.noProducts : t.noEmployees}</Text>
        : kind==="products"
          ? products.map(p=><List.Item key={p.id} title={p.name} description={`${p.price.amount} EUR`}/>)
          : employees.map(e=><List.Item key={e.id} title={`${e.first_name} ${e.last_name}`} description={e.operator_code}/>)}
    </View> : null}
    {adding ? <View style={{gap:12}}>
      <Text variant="titleMedium" accessibilityRole="header">{kind==="products"?t.newProduct:t.newEmployee}</Text>
      {kind==="products" ? <>
        <TextInput mode="outlined" label={t.name} accessibilityLabel={t.name} value={name} onChangeText={setName} testID="management-product-name"/>
        <TextInput mode="outlined" label={t.priceEur} accessibilityLabel={t.priceEur} keyboardType="decimal-pad" value={price} onChangeText={setPrice} testID="management-product-price"/>
      </> : <>
        <TextInput mode="outlined" label={t.firstName} accessibilityLabel={t.firstName} value={first} onChangeText={setFirst} testID="management-employee-first"/>
        <TextInput mode="outlined" label={t.lastName} accessibilityLabel={t.lastName} value={last} onChangeText={setLast} testID="management-employee-last"/>
        <TextInput mode="outlined" label={t.operatorCode} accessibilityLabel={t.operatorCode} maxLength={4} value={code} onChangeText={setCode} testID="management-employee-code"/>
      </>}
      <View style={{flexDirection:"row",flexWrap:"wrap",gap:8}}>
        <Button mode="contained" loading={busy} disabled={busy || (kind==="products" ? !name.trim() || !Number.isFinite(Number(price)) || Number(price)<=0 : !first.trim() || !last.trim() || !code.trim())} onPress={()=>void save()} testID="management-save">{t.save}</Button>
        <Button mode="outlined" disabled={busy} onPress={cancel} testID="management-cancel">{t.cancel}</Button>
      </View>
      {error ? <HelperText type="error" visible testID="management-error">{error}</HelperText> : null}
    </View> : null}
  </Card.Content></Card>;
}
