import { useState } from "react";
import { MiniPosClient, type Employee, type Product } from "./api/client";

export function Management({kind,products,employees,api,onProducts,onEmployees}:{kind:"products"|"employees";products:Product[];employees:Employee[];api:MiniPosClient;onProducts:(v:Product[])=>void;onEmployees:(v:Employee[])=>void}) {
  const [adding,setAdding]=useState(false), [name,setName]=useState(""), [price,setPrice]=useState(""), [first,setFirst]=useState(""), [last,setLast]=useState(""), [code,setCode]=useState("");
  async function addProduct(){const created=await api.createProduct({name,sku:crypto.randomUUID().slice(0,8),unit:"pcs",price:{amount:Number(price).toFixed(2),currency:"EUR"},tax_group:"B",status:"ACTIVE"});onProducts([...products,created]);setAdding(false);}
  async function addEmployee(){const created=await api.createEmployee({first_name:first,last_name:last,operator_code:code,roles:["CASHIER"],status:"ACTIVE"});onEmployees([...employees,created]);setAdding(false);}
  return <section><div className="list-heading"><h2>{kind==="products"?"Товары":"Сотрудники"}</h2><button className="fab" aria-label="Добавить" onClick={()=>setAdding(true)}>+</button></div>
    {!adding && <div className="entity-list">{kind==="products"?products.map(p=><article key={p.id}><strong>{p.name}</strong><span>{p.price.amount} EUR</span></article>):employees.map(e=><article key={e.id}><strong>{e.first_name} {e.last_name}</strong><span>{e.operator_code}</span></article>)}</div>}
    {adding && (kind==="products"?<div><h3>Новый товар</h3><label>Название<input value={name} onChange={e=>setName(e.target.value)}/></label><label>Цена EUR<input inputMode="decimal" value={price} onChange={e=>setPrice(e.target.value)}/></label><button onClick={()=>void addProduct()}>Сохранить</button><button className="secondary" onClick={()=>setAdding(false)}>Отмена</button></div>:<div><h3>Новый сотрудник</h3><label>Имя<input value={first} onChange={e=>setFirst(e.target.value)}/></label><label>Фамилия<input value={last} onChange={e=>setLast(e.target.value)}/></label><label>Код оператора<input maxLength={4} value={code} onChange={e=>setCode(e.target.value)}/></label><button onClick={()=>void addEmployee()}>Сохранить</button><button className="secondary" onClick={()=>setAdding(false)}>Отмена</button></div>)}
  </section>;
}
