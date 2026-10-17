import { AppButton as Button } from './ui/app-button';
import { FormField as TextInput } from './ui/form-field';
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Card, HelperText, List, Text } from "react-native-paper";
import { View } from "react-native";
import { MiniPosClient, type Employee, type Product } from "./api/client";
import { translations } from "./ui/workTranslations.generated";
import type { Language } from "./ui/PreferenceControls";

export function Management({kind,products,employees,api,onProducts,onEmployees,language}:{kind:"products"|"employees";products:Product[];employees:Employee[];api:MiniPosClient;onProducts:(v:Product[])=>void;onEmployees:(v:Employee[])=>void;language:Language}) {
  const t = translations[language];
  const errorId = `miniposweb-management-${useId().replace(/:/g,'')}`;
  const pending = useRef<{owner:object;ticket:object}|null>(null), mounted = useRef(false);
  const owner = useMemo(()=>({api,kind}),[api,kind]), latestOwner = useRef(owner);latestOwner.current=owner;
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;pending.current=null;};},[]);
  const [adding,setAdding]=useState(false), [name,setName]=useState(""), [price,setPrice]=useState(""), [first,setFirst]=useState(""), [last,setLast]=useState(""), [code,setCode]=useState("");
  const [busy,setBusy]=useState(false), [error,setError]=useState("");
  useEffect(()=>{pending.current=null;setBusy(false);setError('');setAdding(false);},[owner]);
  const lists=useRef({products,employees,onProducts,onEmployees});lists.current={products,employees,onProducts,onEmployees};
  const current=()=>mounted.current&&latestOwner.current===owner;
  const editable=()=>current()&&!busy&&pending.current?.owner!==owner;
  const edit=(setter:(value:string)=>void)=>(value:string)=>{if(editable()&&adding)setter(value);};
  const validPrice=/^(?:0|[1-9]\d*)(?:[.,]\d{1,2})?$/.test(price.trim())&&Number(price.trim().replace(',','.'))>0&&Number.isSafeInteger(Math.round(Number(price.trim().replace(',','.'))*100));
  const valid=kind==='products'?Boolean(name.trim()&&validPrice):Boolean(first.trim()&&last.trim()&&code.trim());
  const openForm=()=>{if(!editable()||adding)return;setError("");setName("");setPrice("");setFirst("");setLast("");setCode("");setAdding(true);};
  const cancel=()=>{if(!editable()||!adding)return;setAdding(false);setError("");};
  async function save(){
    if(!editable()||!adding||!valid)return;const ticket={};pending.current={owner,ticket};
    setBusy(true);setError("");
    try {
      if(kind==="products"){
        const created=await api.createProduct({name:name.trim(),sku:crypto.randomUUID().slice(0,8),unit:"pcs",price:{amount:Number(price.trim().replace(',','.')).toFixed(2),currency:"EUR"},tax_group:"B",status:"ACTIVE"});
        if(!mounted.current||latestOwner.current!==owner)return;
        lists.current.onProducts([...lists.current.products.filter(item=>item.id!==created.id),created]);
      } else {
        const created=await api.createEmployee({first_name:first.trim(),last_name:last.trim(),operator_code:code.trim(),roles:["CASHIER"],status:"ACTIVE"});
        if(!mounted.current||latestOwner.current!==owner)return;
        lists.current.onEmployees([...lists.current.employees.filter(item=>item.id!==created.id),created]);
      }
      setAdding(false);
    } catch {if(mounted.current&&latestOwner.current===owner)setError('SAVE_FAILED');}
    finally {if(pending.current?.ticket===ticket){pending.current=null;if(current())setBusy(false);}}
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
          ? products.map(p=><List.Item titleNumberOfLines={0} descriptionNumberOfLines={0} key={p.id} title={p.name} description={`${p.price.amount} EUR`}/>)
          : employees.map(e=><List.Item titleNumberOfLines={0} descriptionNumberOfLines={0} key={e.id} title={`${e.first_name} ${e.last_name}`} description={e.operator_code}/>)}
    </View> : null}
    {adding ? <View style={{gap:12}}>
      <Text variant="titleMedium" accessibilityRole="header">{kind==="products"?t.newProduct:t.newEmployee}</Text>
      {kind==="products" ? <>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.name} accessibilityLabel={t.name} value={name} onChangeText={edit(setName)} testID="management-product-name"/>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.priceEur} accessibilityLabel={t.priceEur} keyboardType="decimal-pad" value={price} onChangeText={edit(setPrice)} onSubmitEditing={()=>void save()} testID="management-product-price"/>
      </> : <>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.firstName} accessibilityLabel={t.firstName} value={first} onChangeText={edit(setFirst)} testID="management-employee-first"/>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.lastName} accessibilityLabel={t.lastName} value={last} onChangeText={edit(setLast)} testID="management-employee-last"/>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.operatorCode} accessibilityLabel={t.operatorCode} maxLength={4} value={code} onChangeText={edit(setCode)} onSubmitEditing={()=>void save()} testID="management-employee-code"/>
      </>}
      <View style={{flexDirection:"row",flexWrap:"wrap",gap:8}}>
        <Button mode="contained" loading={busy} disabled={busy || !valid} onPress={()=>void save()} testID="management-save">{t.save}</Button>
        <Button mode="outlined" disabled={busy} onPress={cancel} testID="management-cancel">{t.cancel}</Button>
      </View>
      {error ? <HelperText nativeID={errorId} accessibilityRole="alert" accessibilityLiveRegion="polite" type="error" visible testID="management-error">{t.saveFailed}</HelperText> : null}
    </View> : null}
  </Card.Content></Card>;
}
