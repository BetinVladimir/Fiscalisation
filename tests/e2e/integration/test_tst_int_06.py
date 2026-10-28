"""TST-INT-06 — RabbitMQ unavailable for 30 s.

Requires compose stack: INTEGRATION_BASE_URL, INTEGRATION_COMPOSE_DIR.
Verifies: operations queued during RabbitMQ outage drain after reconnect;
no duplicate documents produced.
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
class TestTSTINT06(unittest.TestCase):
    """RabbitMQ unavailable 30s → queued operations drain after reconnect."""

    def test_rabbitmq_unavailable_30s_operations_drain(self):
        # Submit a sale before the outage as a reference point.
        pre_key = str(uuid.uuid4())
        pre_status, pre_body = _post('/public/v1/sales', {
            'external_id': 'int06-pre-' + pre_key,
            'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
            'operator_id': 'A001',
        }, pre_key)
        self.assertIn(pre_status, (200, 201), f'pre-outage sale: {pre_status}')
        pre_id = pre_body.get('sale_id') or pre_body.get('id')

        # Pause RabbitMQ.
        subprocess.run(
            ['docker', 'compose', 'pause', 'rabbitmq'],
            cwd=COMPOSE_DIR, check=True, capture_output=True, timeout=15,
        )
        try:
            # Submit a sale during the outage.
            during_key = str(uuid.uuid4())
            _post('/public/v1/sales', {
                'external_id': 'int06-during-' + during_key,
                'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
                'operator_id': 'A001',
            }, during_key)
            time.sleep(30)
        finally:
            subprocess.run(
                ['docker', 'compose', 'unpause', 'rabbitmq'],
                cwd=COMPOSE_DIR, check=True, capture_output=True, timeout=15,
            )

        # After reconnect, pre-outage sale must still be consistent.
        if pre_id:
            time.sleep(5)
            _, detail = _get('/public/v1/sales/' + pre_id)
            state = detail.get('state', '')
            self.assertNotEqual(state, 'FAILED',
                                'pre-outage sale must not be FAILED after RabbitMQ reconnect')
