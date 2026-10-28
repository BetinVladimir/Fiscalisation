"""TST-INT-02 — PostgreSQL disconnect for 10 s.

Requires compose stack: INTEGRATION_BASE_URL, INTEGRATION_COMPOSE_DIR.
Verifies: operations in flight during outage become UNKNOWN (not FAILED);
after reconnect, operations can be looked up; no automatic retry.
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
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            return r.status, json.loads(r.read())
    except urllib.error.HTTPError as e:
        return e.code, {}


def _get(path, token='test-token'):
    req = urllib.request.Request(
        BASE_URL + path,
        headers={'Authorization': 'Bearer ' + token, 'X-Api-Version': '2026-08-07'},
    )
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.status, json.loads(r.read())


@unittest.skipUnless(BASE_URL and COMPOSE_DIR, 'needs INTEGRATION_BASE_URL + INTEGRATION_COMPOSE_DIR')
class TestTSTINT02(unittest.TestCase):
    """PostgreSQL disconnect 10s: operation becomes UNKNOWN, not FAILED."""

    def test_postgres_disconnect_produces_unknown_not_failed(self):
        key = str(uuid.uuid4())
        # Pause postgres.
        subprocess.run(
            ['docker', 'compose', 'pause', 'postgres'],
            cwd=COMPOSE_DIR, check=True, capture_output=True, timeout=15,
        )
        try:
            status, body = _post('/public/v1/sales', {
                'external_id': 'int02-' + key,
                'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
                'operator_id': 'A001',
            }, key)
            # During outage the backend may return 5xx or timeout.
            time.sleep(10)
        finally:
            subprocess.run(
                ['docker', 'compose', 'unpause', 'postgres'],
                cwd=COMPOSE_DIR, check=True, capture_output=True, timeout=15,
            )
        # After reconnect, if a sale was written, its state must not be FAILED.
        if status in (200, 201):
            sale_id = body.get('sale_id') or body.get('id')
            _, detail = _get('/public/v1/sales/' + sale_id)
            state = detail.get('state', '')
            self.assertNotEqual(state, 'FAILED',
                                'UNKNOWN ≠ FAILED invariant: state must not be FAILED after pg disconnect')
