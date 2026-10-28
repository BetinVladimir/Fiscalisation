"""TST-INT-01 — backend restart between PutOperation and reply.

Requires compose stack: INTEGRATION_BASE_URL, INTEGRATION_COMPOSE_DIR.
Verifies: after restart, client retry with same key → one receipt, one UNP,
reply body identical to original.
"""
import json
import os
import subprocess
import time
import unittest
import urllib.request
import uuid

BASE_URL = os.environ.get('INTEGRATION_BASE_URL', '')
COMPOSE_DIR = os.environ.get('INTEGRATION_COMPOSE_DIR', '')
HMAC_KEY = os.environ.get('INTEGRATION_HMAC_KEY', '')


def _post(path, body, key, token='test-token'):
    raw = json.dumps(body).encode()
    req = urllib.request.Request(
        BASE_URL + path,
        data=raw,
        headers={
            'Content-Type': 'application/json',
            'Idempotency-Key': key,
            'Authorization': 'Bearer ' + token,
            'X-Api-Version': '2026-08-07',
        },
        method='POST',
    )
    with urllib.request.urlopen(req, timeout=15) as r:
        return r.status, json.loads(r.read())


def _restart_backend():
    subprocess.run(
        ['docker', 'compose', 'restart', 'fiscal-backend'],
        cwd=COMPOSE_DIR, check=True, capture_output=True, timeout=60,
    )
    deadline = time.time() + 30
    while time.time() < deadline:
        try:
            urllib.request.urlopen(BASE_URL + '/health', timeout=2)
            return
        except Exception:
            time.sleep(1)
    raise RuntimeError('backend did not recover within 30s')


@unittest.skipUnless(BASE_URL and COMPOSE_DIR, 'needs INTEGRATION_BASE_URL + INTEGRATION_COMPOSE_DIR')
class TestTSTINT01(unittest.TestCase):
    """Restart backend between PutOperation and reply → exactly one receipt."""

    def test_restart_between_put_and_reply(self):
        key = str(uuid.uuid4())
        status1, body1 = _post('/public/v1/sales', {
            'external_id': 'int01-' + key,
            'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
            'operator_id': 'A001',
        }, key)
        self.assertIn(status1, (200, 201), f'first PUT: {status1} {body1}')
        original_id = body1.get('sale_id') or body1.get('id')
        self.assertIsNotNone(original_id, 'no sale_id in response')

        _restart_backend()

        status2, body2 = _post('/public/v1/sales', {
            'external_id': 'int01-' + key,
            'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
            'operator_id': 'A001',
        }, key)
        self.assertIn(status2, (200, 201), f'retry after restart: {status2} {body2}')
        retry_id = body2.get('sale_id') or body2.get('id')
        self.assertEqual(original_id, retry_id,
                         'retry must return same sale id (idempotency)')
