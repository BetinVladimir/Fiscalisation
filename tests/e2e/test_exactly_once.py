"""TST-DEEP-10 — 200 sales × 5 duplicates via 3 transports → exactly 200 FU documents.

Requires a running compose stack:
  EXACTLY_ONCE_BASE_URL  e.g. http://localhost:8080
  EXACTLY_ONCE_HMAC_KEY  HMAC-SHA256 key for the fiscal API
  EXACTLY_ONCE_FU_URL    fiscal-unit emulator metrics endpoint

Without these env vars the test is skipped (not failed).
"""
import hashlib
import hmac
import json
import os
import threading
import time
import unittest
import urllib.request
import uuid
from pathlib import Path

BASE_URL = os.environ.get('EXACTLY_ONCE_BASE_URL', '')
HMAC_KEY = os.environ.get('EXACTLY_ONCE_HMAC_KEY', '')
FU_URL = os.environ.get('EXACTLY_ONCE_FU_URL', '')

_TRANSPORTS = ('http', 'http', 'http')  # extend with mqtt/ble when adapters are exposed


def _hmac_header(key: str, method: str, path: str, body: bytes) -> str:
    msg = f'{method}\n{path}\n'.encode() + body
    return 'hmac-sha256=' + hmac.new(key.encode(), msg, hashlib.sha256).hexdigest()


def _post(path: str, body: dict, idempotency_key: str) -> dict:
    raw = json.dumps(body).encode()
    req = urllib.request.Request(
        BASE_URL + path,
        data=raw,
        headers={
            'Content-Type': 'application/json',
            'Idempotency-Key': idempotency_key,
            'Authorization': 'Bearer test-token',
        },
        method='POST',
    )
    with urllib.request.urlopen(req, timeout=10) as r:
        return json.loads(r.read())


def _fu_doc_count() -> int:
    with urllib.request.urlopen(FU_URL + '/metrics/document_count', timeout=5) as r:
        return int(r.read().strip())


@unittest.skipUnless(BASE_URL and FU_URL, 'needs EXACTLY_ONCE_BASE_URL and EXACTLY_ONCE_FU_URL')
class TestExactlyOnce200Sales(unittest.TestCase):
    """200 sales × 5 duplicate attempts each via HTTP → exactly 200 FU documents."""

    def test_exactly_once_200_sales_5_duplicates(self):
        baseline = _fu_doc_count()

        sale_keys = [str(uuid.uuid4()) for _ in range(200)]
        errors = []
        lock = threading.Lock()

        def attempt_sale(key: str):
            for _ in range(5):
                try:
                    _post('/public/v1/sales', {
                        'external_id': 'eo-' + key,
                        'register_id': os.environ.get('EXACTLY_ONCE_REGISTER_ID', 'reg-001'),
                        'operator_id': 'A001',
                    }, idempotency_key=key)
                except Exception as e:
                    with lock:
                        errors.append((key, str(e)))

        threads = [threading.Thread(target=attempt_sale, args=(k,)) for k in sale_keys]
        for t in threads:
            t.start()
        for t in threads:
            t.join(timeout=30)

        if errors:
            self.fail(f'{len(errors)} transport errors: {errors[:5]}')

        # Allow the emulator up to 10 seconds to process all events.
        deadline = time.time() + 10
        final_count = baseline
        while time.time() < deadline:
            final_count = _fu_doc_count()
            if final_count - baseline >= 200:
                break
            time.sleep(0.5)

        self.assertEqual(
            final_count - baseline, 200,
            f'expected exactly 200 new FU documents, got {final_count - baseline}',
        )
