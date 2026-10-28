export type DeviceDataCopy = Readonly<Record<string,string>>;
export type DeviceDataRow = {label:string;value:string};
type Path = {key:string;parent:Path|null};
type Frame = {value:unknown;path:Path|null}|{exit:object};
/** Iterative traversal preserves every received field and array position without serializing JSON. */
export function deviceDataRows(value:unknown,copy:DeviceDataCopy):DeviceDataRow[]{
 const rows:DeviceDataRow[]=[],active=new WeakSet<object>(),stack:Frame[]=[{value,path:null}];
 const label=(path:Path|null)=>{const parts:string[]=[];for(let node=path;node;node=node.parent){const key='field.'+node.key;parts.push(Object.hasOwn(copy,key)?copy[key]:node.key);}return parts.reverse().join(' › ')||copy.value;};
 while(stack.length){const frame=stack.pop()!;if('exit'in frame){active.delete(frame.exit);continue;}const current=frame.value;
  if(current!==null&&typeof current==='object'){
   if(active.has(current))throw new TypeError('Cyclic device data');
   const entries=Array.isArray(current)?current.map((item,index)=>[String(index+1),item] as const):Object.entries(current);
   if(entries.length){active.add(current);stack.push({exit:current});for(let i=entries.length-1;i>=0;i--){const [key,child]=entries[i];stack.push({value:child,path:{key,parent:frame.path}});}continue;}
   rows.push({label:label(frame.path),value:Array.isArray(current)?copy.emptyArray:copy.emptyObject});continue;
  }
  const text=current===null||current===undefined?copy.noValue:current===true?copy.yes:current===false?copy.no:current===''?copy.emptyText:String(current);
  rows.push({label:label(frame.path),value:text});
 }
 return rows;
}
