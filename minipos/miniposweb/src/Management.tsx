import { useState } from "react";
import { Button, Card, HelperText, List, Text, TextInput } from "react-native-paper";
import { View } from "react-native";
import { MiniPosClient, type Employee, type Product } from "./api/client";

export function Management({kind,products,employees,api,onProducts,onEmployees}:{kind:"products"|"employees";products:Product[];employees:Employee[];api:MiniPosClient;onProducts:(v:Product[])=>void;onEmployees:(v:Employee[])=>void}) {
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
    } catch (reason) {setError(reason instanceof Error ? reason.message || "Ошибка сохранения" : String(reason));}
    finally {setBusy(false);}
  }
  return <Card mode="outlined" style={{marginVertical:16}}><Card.Content style={{gap:16}}>
    <View style={{flexDirection:"row",flexWrap:"wrap",alignItems:"center",justifyContent:"space-between",gap:8}}>
      <Text variant="titleLarge" accessibilityRole="header">{kind==="products"?"Товары":"Сотрудники"}</Text>
      {!adding ? <Button mode="contained" onPress={openForm} testID="management-add">Добавить</Button> : null}
    </View>
    {!adding ? <View testID="management-list" style={{gap:4}}>
      {(kind==="products" ? products : employees).length === 0
        ? <Text variant="bodyMedium">{kind==="products" ? "Товары пока не добавлены" : "Сотрудники пока не добавлены"}</Text>
        : kind==="products"
          ? products.map(p=><List.Item key={p.id} title={p.name} description={`${p.price.amount} EUR`}/>)
          : employees.map(e=><List.Item key={e.id} title={`${e.first_name} ${e.last_name}`} description={e.operator_code}/>)}
    </View> : null}
    {adding ? <View style={{gap:12}}>
      <Text variant="titleMedium" accessibilityRole="header">{kind==="products"?"Новый товар":"Новый сотрудник"}</Text>
      {kind==="products" ? <>
        <TextInput mode="outlined" label="Название" accessibilityLabel="Название" value={name} onChangeText={setName} testID="management-product-name"/>
        <TextInput mode="outlined" label="Цена EUR" accessibilityLabel="Цена EUR" keyboardType="decimal-pad" value={price} onChangeText={setPrice} testID="management-product-price"/>
      </> : <>
        <TextInput mode="outlined" label="Имя" accessibilityLabel="Имя" value={first} onChangeText={setFirst} testID="management-employee-first"/>
        <TextInput mode="outlined" label="Фамилия" accessibilityLabel="Фамилия" value={last} onChangeText={setLast} testID="management-employee-last"/>
        <TextInput mode="outlined" label="Код оператора" accessibilityLabel="Код оператора" maxLength={4} value={code} onChangeText={setCode} testID="management-employee-code"/>
      </>}
      <View style={{flexDirection:"row",flexWrap:"wrap",gap:8}}>
        <Button mode="contained" loading={busy} disabled={busy || (kind==="products" ? !name.trim() || !Number.isFinite(Number(price)) || Number(price)<=0 : !first.trim() || !last.trim() || !code.trim())} onPress={()=>void save()} testID="management-save">Сохранить</Button>
        <Button mode="outlined" disabled={busy} onPress={cancel} testID="management-cancel">Отмена</Button>
      </View>
      {error ? <HelperText type="error" visible testID="management-error">{error}</HelperText> : null}
    </View> : null}
  </Card.Content></Card>;
}
