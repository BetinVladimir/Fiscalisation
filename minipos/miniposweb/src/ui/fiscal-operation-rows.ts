/** Presentation only: no proof decisions, DTO mutation, sorting of unknown fields or numeric coercion. */
export const operationKeys=['operation_id','state','certainty','fiscal_reference','terminal_reference','error_code','updated_at'] as const;
type Path={key:string;parent:Path|null};
export type OperationRow={path:string[];kind:'value'|'nullValue'|'emptyString'|'emptyObject'|'emptyArray'|'undefinedValue';value?:string;boolean?:boolean};
export function fiscalOperationRows(value:unknown):OperationRow[]{
 const output:OperationRow[]=[],stack:{value:unknown;path:Path|null}[]=[{value,path:null}];
 while(stack.length){const item=stack.pop()!,v=item.value;let kind:OperationRow['kind']='value',text:string|undefined;
 if(v===null)kind='nullValue';else if(v===undefined)kind='undefinedValue';else if(v==='')kind='emptyString';else if(typeof v==='object'){
 const keys=Object.keys(v);if(!keys.length)kind=Array.isArray(v)?'emptyArray':'emptyObject';else{const ordered=item.path===null&&!Array.isArray(v)?[...operationKeys.filter(key=>Object.prototype.hasOwnProperty.call(v,key)),...keys.filter(key=>!(operationKeys as readonly string[]).includes(key))]:keys;
 for(let i=ordered.length-1;i>=0;i--){const key=ordered[i];stack.push({value:(v as Record<string,unknown>)[key],path:{key:Array.isArray(v)?String(Number(key)+1):key,parent:item.path}})}continue;}
 }else text=typeof v==='boolean'?(v?'true':'false'):String(v);
 const path:string[]=[];for(let p=item.path;p;p=p.parent)path.push(p.key);path.reverse();output.push({path,kind,...(text===undefined?{}:{value:text}),...(typeof v==='boolean'?{boolean:v}:{})});}
 return output;
}
