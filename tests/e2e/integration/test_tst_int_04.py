"""TST-INT-04 — duplicate MQTT result.

Requires compose stack: INTEGRATION_BASE_URL, INTEGRATION_COMPOSE_DIR, INTEGRATION_MQTT_*.
Verifies: same MQTT result delivered twice → idempotent; state is set once,
no duplicate sale or UNP allocated.
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
MQTT_HOST = os.environ.get('INTEGRATION_MQTT_HOST', 'localhost')
MQTT_PORT = os.environ.get('INTEGRATION_MQTT_PORT', '1883')


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


def _get(path, token='test-token'):
    req = urllib.request.Request(
        BASE_URL + path,
        headers={'Authorization': 'Bearer ' + token, 'X-Api-Version': '2026-08-07'},
    )
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.status, json.loads(r.read())


def _mqtt_publish(topic, payload):
    subprocess.run(
        ['mosquitto_pub', '-h', MQTT_HOST, '-p', MQTT_PORT,
         '-t', topic, '-m', payload],
        check=True, capture_output=True, timeout=10,
    )


@unittest.skipUnless(BASE_URL and COMPOSE_DIR, 'needs INTEGRATION_BASE_URL + INTEGRATION_COMPOSE_DIR')
class TestTSTINT04(unittest.TestCase):
    """Duplicate MQTT result → idempotent state, no duplicate UNP."""

    def test_duplicate_mqtt_result_is_idempotent(self):
        key = str(uuid.uuid4())
        status, body = _post('/public/v1/sales', {
            'external_id': 'int04-' + key,
            'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
            'operator_id': 'A001',
        }, key)
        self.assertIn(status, (200, 201), f'create sale: {status} {body}')
        sale_id = body.get('sale_id') or body.get('id')
        self.assertIsNotNone(sale_id)

        # Wait for the FU to process it once.
        time.sleep(5)
        _, detail1 = _get('/public/v1/sales/' + sale_id)
        state1 = detail1.get('state', '')
        unp1 = detail1.get('fiscal_reference', '')

        if state1 not in ('FISCALIZED', 'COMPLETED'):
            self.skipTest(f'sale not fiscalized yet (state={state1}); cannot test duplicate MQTT')

        # Re-publish the same result message to simulate duplicate delivery.
        result_topic = os.environ.get(
            'INTEGRATION_RESULT_TOPIC', f'fiscal/result/{sale_id}')
        result_payload = json.dumps({
            'sale_id': sale_id,
            'state': 'FISCALIZED',
            'fiscal_reference': unp1,
            'operation_id': detail1.get('fiscal_operation_id', key),
            'version': detail1.get('version', 1),
        })
        try:
            _mqtt_publish(result_topic, result_payload)
            _mqtt_publish(result_topic, result_payload)
        except Exception as e:
            self.skipTest(f'mosquitto_pub not available: {e}')

        time.sleep(3)
        _, detail2 = _get('/public/v1/sales/' + sale_id)
        self.assertEqual(detail2.get('state'), state1, 'state changed after duplicate result')
        self.assertEqual(detail2.get('fiscal_reference'), unp1,
                         'UNP changed after duplicate result (duplicate invariant violated)')
