import type {FiscalOperation} from '../types';
export type OperationOutcomeKind='confirmed'|'failed'|'pending';
/** A terminal-looking state alone is never proof of success. */
export function operationOutcomeKind(value:Pick<FiscalOperation,'state'|'certainty'>):OperationOutcomeKind{
 if(['COMMITTED','FISCALIZED','REVERSED','PRINTER_TESTED','CANCELLED'].includes(value.state)&&value.certainty==='PROVEN_SUCCESS')return 'confirmed';
 if(['REJECTED','FAILED','COMPENSATED'].includes(value.state)&&['PROVEN_FAILURE','NOT_SENT'].includes(value.certainty))return 'failed';
 return 'pending';
}
