export type OutboxScope={tenant:string;location:string;register:string;operator:string};
/** Fail closed for legacy/malformed rows; never rewrite ownership to current context. */
export function belongsToOutboxScope(row:unknown,scope:OutboxScope):boolean {
 if(!Object.values(scope).every(value=>typeof value==='string'&&value.trim()))return false;
 if(!row||typeof row!=='object'||!('intent' in row)||!row.intent||typeof row.intent!=='object')return false;
 const intent=row.intent as Record<string,unknown>;
 return intent.tenant_id===scope.tenant&&intent.location_id===scope.location&&intent.register_id===scope.register&&intent.operator_code===scope.operator;
}
