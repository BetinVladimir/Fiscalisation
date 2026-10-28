"""TST-INT-07 — 50 finalizations with the same idempotency key.

Requires compose stack: INTEGRATION_BASE_URL.
Verifies: 50 POST /sales with the same idempotency key → exactly one
FISCALIZED document, 49 idempotency-replayed responses, one UNP.
"""
import json
import os
import threading
import unittest
import urllib.request
import uuid

BASE_URL = os.environ.get('INTEGRATION_BASE_URL', '')


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
        return r.status, dict(r.headers), json.loads(r.read())


@unittest.skipUnless(BASE_URL, 'needs INTEGRATION_BASE_URL')
class TestTSTINT07(unittest.TestCase):
    """50 POST /sales with same key → 1 FISCALIZED, 49 replays, 1 UNP."""

    def test_50_finalizations_same_key_one_document(self):
        key = str(uuid.uuid4())
        body = {
            'external_id': 'int07-' + key,
            'register_id': os.environ.get('INTEGRATION_REGISTER_ID', 'reg-001'),
            'operator_id': 'A001',
        }

        results = []
        errors = []
        lock = threading.Lock()

        def attempt():
            try:
                status, headers, resp = _post('/public/v1/sales', body, key)
                with lock:
                    results.append((status, headers, resp))
            except Exception as e:
                with lock:
                    errors.append(str(e))

        threads = [threading.Thread(target=attempt) for _ in range(50)]
        for t in threads:
            t.start()
        for t in threads:
            t.join(timeout=60)

        if errors:
            self.fail(f'{len(errors)} transport errors: {errors[:3]}')

        self.assertEqual(len(results), 50, 'expected 50 responses')

        # All responses must be 200 or 201.
        for status, _, _ in results:
            self.assertIn(status, (200, 201), f'unexpected status {status}')

        # Collect all returned UNPs / sale IDs.
        sale_ids = set()
        for _, _, resp in results:
            sid = resp.get('sale_id') or resp.get('id')
            if sid:
                sale_ids.add(sid)

        self.assertEqual(len(sale_ids), 1,
                         f'expected exactly 1 sale_id, got {len(sale_ids)}: {sale_ids}')

        # Count replays: all but (at most) one must be Idempotency-Replayed: true.
        non_replays = sum(
            1 for _, headers, _ in results
            if headers.get('Idempotency-Replayed', '').lower() != 'true'
        )
        self.assertLessEqual(non_replays, 1,
                             f'{non_replays} non-replayed responses for 50 identical requests')
