import type {Shift} from '../api/client';
/** A final shift requires matching source identity and both Z-report references. */
export function provenClosedShift(previous:Shift, next:Shift):boolean {
 return next.id===previous.id&&next.register_id===previous.register_id&&next.employee_id===previous.employee_id&&next.state==='CLOSED'&&Boolean(next.z_operation_id?.trim())&&Boolean(next.z_fiscal_reference?.trim())&&Number.isInteger(next.version)&&next.version>=previous.version;
}
export function shiftCloseAction(shift:Shift):'close'|'reconcile'|null {
 if(shift.state==='OPEN'&&shift.allowed_actions?.includes('CLOSE'))return 'close';
 if(shift.state==='BLOCKED_RECONCILIATION'&&shift.allowed_actions?.includes('RECONCILE'))return 'reconcile';
 return null;
}
