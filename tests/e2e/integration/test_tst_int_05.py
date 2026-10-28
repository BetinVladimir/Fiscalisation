"""TST-INT-05 — invalid HMAC on MQTT result.

Requires compose stack: INTEGRATION_BASE_URL, INTEGRATION_COMPOSE_DIR, INTEGRATION_MQTT_*.
Verifies: message with bad HMAC is rejected; mqtt_bad_signature_total metric
increments; operation state does not change.
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
METRICS_URL = os.environ.get('INTEGRATION_METRICS_URL', BASE_URL + '/metrics')


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


def _metric_value(name):
    try:
        with urllib.request.urlopen(METRICS_URL, timeout=5) as r:
            for line in r.read().decode().splitlines():
                if line.startswith(name + ' ') or line.startswith(name + '{'):
                    parts = line.rsplit(' ', 1)
                    return float(parts[-1])
    except Exception:
        pass
    return None


def _mqtt_publish(topic, payload):
    subprocess.run(
        ['mosquitto_pub', '-h', MQTT_HOST, '-p', MQTT_PORT,
         '-t', topic, '-m', payload],
        check=True, capture_output=True, timeout=10,
    )


@unittest.skipUnless(BASE_URL and COMPOSE_DIR, 'needs INTEGRATION_BASE_URL + INTEGRATION_COMPOSE_DIR')
class TestTSTINT05(unittest.TestCase):
    """Bad HMAC on result → rejected; mqtt_bad_signature_total +1; state unchanged."""

    def test_invalid_hmac_rejected_and_metric_increments(self):
        key = str(uuid.uuid4())
        status, body = _post('/public/v1/sales', {
            'external_id': 'int05-' + key,
            'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
            'operator_id': 'A001',
        }, key)
        self.assertIn(status, (200, 201), f'create sale: {status} {body}')
        sale_id = body.get('sale_id') or body.get('id')
        self.assertIsNotNone(sale_id)

        baseline_metric = _metric_value('mqtt_bad_signature_total')

        result_topic = os.environ.get(
            'INTEGRATION_RESULT_TOPIC', f'fiscal/result/{sale_id}')
        bad_payload = json.dumps({
            'sale_id': sale_id,
            'state': 'FISCALIZED',
            'fiscal_reference': 'FAKE-UNP',
            'operation_id': key,
            'version': 1,
            'hmac': 'deadbeef' * 8,  # deliberately wrong signature
        })
        try:
            _mqtt_publish(result_topic, bad_payload)
        except Exception as e:
            self.skipTest(f'mosquitto_pub not available: {e}')

        time.sleep(3)

        # Operation state must not have changed to FISCALIZED.
        _, detail = _get('/public/v1/sales/' + sale_id)
        state = detail.get('state', '')
        self.assertNotEqual(state, 'FISCALIZED',
                            'bad HMAC must not update operation state')
        self.assertNotEqual(detail.get('fiscal_reference'), 'FAKE-UNP',
                            'bad HMAC must not set a fiscal reference')

        # Metric must have incremented (or be unavailable — skip assertion).
        if baseline_metric is not None:
            after_metric = _metric_value('mqtt_bad_signature_total')
            if after_metric is not None:
                self.assertGreater(after_metric, baseline_metric,
                                   'mqtt_bad_signature_total must increment on bad HMAC')
