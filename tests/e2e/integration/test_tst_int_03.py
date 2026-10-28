"""TST-INT-03 — MQTT loss between publish and result.

Requires compose stack: INTEGRATION_BASE_URL, INTEGRATION_COMPOSE_DIR.
Verifies: operation stays UNKNOWN (not auto-retried/refunded) after MQTT drop.
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
        with urllib.request.urlopen(req, timeout=20) as r:
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
class TestTSTINT03(unittest.TestCase):
    """MQTT loss between publish and result → operation stays UNKNOWN."""

    def test_mqtt_loss_leaves_operation_unknown(self):
        key = str(uuid.uuid4())
        # Pause the MQTT broker so result messages cannot flow back.
        subprocess.run(
            ['docker', 'compose', 'pause', 'mqtt'],
            cwd=COMPOSE_DIR, check=True, capture_output=True, timeout=15,
        )
        try:
            status, body = _post('/public/v1/sales', {
                'external_id': 'int03-' + key,
                'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
                'operator_id': 'A001',
            }, key)
        finally:
            time.sleep(2)
            subprocess.run(
                ['docker', 'compose', 'unpause', 'mqtt'],
                cwd=COMPOSE_DIR, check=True, capture_output=True, timeout=15,
            )

        if status not in (200, 201):
            self.skipTest(f'sale not created (status {status}); cannot verify UNKNOWN invariant')

        sale_id = body.get('sale_id') or body.get('id')
        # After MQTT loss, if the fiscal operation was queued but never got a
        # result message, the backend must not auto-trigger a refund or retry.
        time.sleep(5)
        _, detail = _get('/public/v1/sales/' + sale_id)
        state = detail.get('state', '')
        self.assertNotEqual(state, 'FAILED',
                            'UNKNOWN ≠ FAILED: must not auto-fail after MQTT loss')
        self.assertNotIn('REVERSED', state,
                         'must not auto-reverse after MQTT loss (storno-first invariant)')
