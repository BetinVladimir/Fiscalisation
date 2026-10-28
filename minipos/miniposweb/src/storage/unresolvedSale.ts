import {isProvenTerminal,type OutboxRow} from './outbox';
import {belongsToOutboxScope,type OutboxScope} from './outboxScope';
/** An attempted sale may already have charged/fiscalized: only status recovery is safe. */
export function unresolvedSale(rows:OutboxRow[],scope:OutboxScope){return rows.find(row=>belongsToOutboxScope(row,scope)&&row.intent.command==='SALE_FINALIZE'&&Boolean(row.attempt||row.result)&&(!row.result||!isProvenTerminal(row.result)));}
