import type { MiniPosClient } from '../api/client';
import { listOutbox, updateOutboxResult, isProvenTerminal } from '../storage/outbox';
import { belongsToOutboxScope, type OutboxScope } from '../storage/outboxScope';

// Outbox refresh after reload performs only authenticated status GETs. It never
// reconstructs a sale or calls submit when the original response was lost.
export async function refreshCloudOutbox(api: Pick<MiniPosClient, 'operation'>, scope: OutboxScope, live: () => boolean) {
  const rows = (await listOutbox()).filter(row => belongsToOutboxScope(row, scope));
  for (const row of rows) {
    if (!live()) return [];
    const orderId = row.intent.metadata?.order_id;
    if (row.attempt?.route !== 'CLOUD' || typeof orderId !== 'string' || !orderId || (row.result && isProvenTerminal(row.result))) continue;
    const controller = new AbortController(), timer = setTimeout(() => controller.abort(), 15000);
    let result;
    try { result = await api.operation(orderId, row.intent.client_operation_id, controller.signal); }
    catch { continue; } // Transport/auth/not-found errors leave the durable attempt uncertain.
    finally { clearTimeout(timer); }
    if (!live()) return [];
    await updateOutboxResult(row.intent, result);
  }
  return live() ? (await listOutbox()).filter(row => belongsToOutboxScope(row, scope)) : [];
}
