import http from 'node:http';
import crypto from 'node:crypto';

const profile = process.env.DEVICE_PROFILE || 'edge-agent-s3';
const port = Number(process.env.HTTP_PORT || 8080);
const operations = new Map(), idempotency = new Map();
const json = (res, status, body) => { res.writeHead(status, {'content-type':'application/json'}); res.end(JSON.stringify(body)); };
const read = req => new Promise((resolve, reject) => { let s=''; req.on('data', c => s += c); req.on('end', () => { try { resolve(s ? JSON.parse(s) : {}); } catch(e) { reject(e); } }); });
const authorized = req => /^Bearer\s+\S+/.test(req.headers.authorization || '');

http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://localhost');
  const root = '/beeloy/local/v1';
  if (url.pathname === `${root}/healthz`) return json(res, 200, {status:'ok', profile});
  if (url.pathname === '/__e2e/operations') return json(res, 200, {items:[...operations.values()]});
  if (!url.pathname.startsWith(root)) return json(res, 404, {code:'NOT_FOUND'});
  if (!authorized(req)) return json(res, 401, {code:'UNAUTHORIZED'});
  if (url.pathname === `${root}/readyz`) return json(res, 200, {state:'READY', profile});
  if (url.pathname === `${root}/device`) return json(res, 200, {device_id:`mock-${profile}`, model:profile, firmware_version:'e2e-1', capabilities: profile === 'bluecash-app' ? ['CARD','FISCAL'] : ['CASH','FISCAL']});
  if (url.pathname === `${root}/intents` && req.method === 'POST') {
    let body; try { body = await read(req); } catch { return json(res, 400, {code:'INVALID_JSON'}); }
    const key = req.headers['idempotency-key'] || body.client_operation_id || body.intent_id;
    if (!key) return json(res, 400, {code:'IDEMPOTENCY_REQUIRED'});
    if (idempotency.has(key)) return json(res, 202, idempotency.get(key));
    const paymentType = body.payment_type || body.payment?.type || (profile === 'bluecash-app' ? 'CARD' : 'CASH');
    if (profile === 'bluecash-app' && paymentType !== 'CARD') return json(res, 422, {code:'CARD_REQUIRED'});
    const operation = {
      operation_id: crypto.randomUUID(), intent_id: body.intent_id || key, client_operation_id: body.client_operation_id || key,
      state:'FISCALIZED', profile, payment_type:paymentType, fiscal_reference:`E2E-${profile}-${Date.now()}`,
      binding_generation: body.binding_generation ?? 1, completed_at:new Date().toISOString()
    };
    if (profile === 'bluecash-app') Object.assign(operation, {payment_reference:`PAY-${Date.now()}`, rrn:'123456789012', authorization_code:'E2E001'});
    operations.set(operation.operation_id, operation); idempotency.set(key, operation);
    return json(res, 202, operation);
  }
  const match = url.pathname.match(new RegExp(`^${root}/operations/([^/:]+)(:reconcile)?$`));
  if (match) {
    const operation = operations.get(match[1]);
    if (!operation) return json(res, 404, {code:'OPERATION_NOT_FOUND'});
    return json(res, 200, {...operation, reconciled: Boolean(match[2])});
  }
  json(res, 404, {code:'NOT_FOUND'});
}).listen(port, '0.0.0.0');
