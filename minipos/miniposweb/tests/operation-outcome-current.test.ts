import {expect,it} from 'vitest';
import {operationOutcomeKind} from '../src/ui/operation-outcome';
it.each(['COMMITTED','FISCALIZED','REVERSED','PRINTER_TESTED','CANCELLED'])('requires proven success for %s',state=>{expect(operationOutcomeKind({state,certainty:'PROVEN_SUCCESS'})).toBe('confirmed');for(const certainty of ['UNKNOWN','PROVEN_FAILURE','NOT_SENT',''])expect(operationOutcomeKind({state,certainty})).toBe('pending');});
it.each(['FAILED','REJECTED','COMPENSATED'])('distinguishes proven failure from uncertain %s',state=>{for(const certainty of ['PROVEN_FAILURE','NOT_SENT'])expect(operationOutcomeKind({state,certainty})).toBe('failed');for(const certainty of ['UNKNOWN','PROVEN_SUCCESS'])expect(operationOutcomeKind({state,certainty})).toBe('pending');});
it('does not project success for an unknown or pending state',()=>{for(const state of ['UNKNOWN','PENDING','CARD_APPROVED','arbitrary'])expect(operationOutcomeKind({state,certainty:'PROVEN_SUCCESS'})).toBe('pending');});
