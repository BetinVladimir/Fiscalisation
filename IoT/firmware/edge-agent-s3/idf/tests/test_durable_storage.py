import re
import sqlite3
import tempfile
import unittest
from pathlib import Path

SOURCE = Path(__file__).parents[1] / "main" / "durable_storage.cpp"


def schema() -> str:
    match = re.search(r'kSchema=R"sql\((.*?)\)sql";', SOURCE.read_text(), re.S)
    if not match:
        raise AssertionError("durable schema not found")
    return match.group(1)


class DurableStorageContract(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.path = Path(self.temp.name) / "edge.db"
        self.db = sqlite3.connect(self.path)
        self.db.executescript(schema())

    def tearDown(self):
        self.db.close()
        self.temp.cleanup()

    def reserve(self, operation, digest, transport="BLE"):
        try:
            self.db.execute("BEGIN IMMEDIATE")
            existing = self.db.execute(
                "SELECT payload_digest FROM operations WHERE operation_id=?", (operation,)
            ).fetchone()
            if existing:
                self.db.commit()
                return "DUPLICATE" if existing[0] == digest else "PAYLOAD_CONFLICT"
            self.db.execute(
                "INSERT INTO operations(operation_id,payload_digest,first_transport,route_snapshot,created_at,updated_at) VALUES(?,?,?,?,?,?)",
                (operation, digest, transport, "binding-generation-7", 1_000, 1_000),
            )
            self.db.commit()
            return "NEW"
        except Exception:
            self.db.rollback()
            raise

    def test_ble_mqtt_share_operation_idempotency(self):
        digest = "a" * 64
        self.assertEqual(self.reserve("operation-1", digest, "BLE"), "NEW")
        self.assertEqual(self.reserve("operation-1", digest, "MQTT"), "DUPLICATE")
        self.assertEqual(self.reserve("operation-1", "b" * 64, "MQTT"), "PAYLOAD_CONFLICT")
        self.assertEqual(self.db.execute("SELECT count(*) FROM operations").fetchone()[0], 1)

    def test_restart_recovers_nonterminal_receipt_and_payment(self):
        self.reserve("operation-2", "c" * 64)
        self.db.execute(
            "INSERT INTO receipts VALUES(?,?,?,?,?,?)",
            ("receipt-2", "operation-2", "FISCAL_OPEN", "{}", 2_000, 2_000),
        )
        self.db.execute(
            "INSERT INTO payments VALUES(?,?,?,?,?,?,?,?,?,?)",
            ("payment-2", "operation-2", "receipt-2", "APPROVED", 1234, "terminal", "rrn", "auth", 2_000, 2_000),
        )
        self.db.commit()
        self.db.close()
        self.db = sqlite3.connect(self.path)
        recovered = self.db.execute(
            "SELECT o.operation_id,r.receipt_id,p.state FROM operations o JOIN receipts r USING(operation_id) JOIN payments p USING(operation_id) WHERE o.state NOT IN('COMMITTED','COMPENSATED','REJECTED')"
        ).fetchone()
        self.assertEqual(recovered, ("operation-2", "receipt-2", "APPROVED"))

    def test_runtime_recovery_query_restores_card_approval_evidence_once(self):
        text = SOURCE.read_text()
        self.assertIn("'CARD_APPROVED'", text)
        self.assertIn("json_extract(r.canonical_payload,'$.unp')", text)
        self.assertIn("p.payment_id=(SELECT p1.payment_id", text)
        self.assertIn("LIMIT 1", text)

    def test_ack_cursor_and_retention_never_delete_unacknowledged(self):
        self.reserve("operation-3", "d" * 64)
        old = 100
        for event in ("acknowledged", "pending"):
            self.db.execute(
                "INSERT INTO event_outbox(event_id,operation_id,kind,payload,payload_digest,created_at) VALUES(?,?,?,?,?,?)",
                (event, "operation-3", "RESULT", "{}", "e" * 64, old),
            )
        sequence = self.db.execute(
            "SELECT sequence FROM event_outbox WHERE event_id='acknowledged'"
        ).fetchone()[0]
        self.db.execute(
            "UPDATE event_outbox SET acknowledged_at=?,ack_id=? WHERE sequence<=?",
            (10_000, "ack-1", sequence),
        )
        self.db.execute(
            "UPDATE sync_cursor SET acknowledged_sequence=?,ack_id=?,updated_at=? WHERE singleton=1",
            (sequence, "ack-1", 10_000),
        )
        self.db.execute(
            "DELETE FROM event_outbox WHERE acknowledged_at IS NOT NULL AND created_at<?",
            (20_000,),
        )
        self.db.commit()
        self.assertEqual(
            self.db.execute("SELECT event_id FROM event_outbox").fetchall(), [("pending",)]
        )
        self.assertEqual(
            self.db.execute("SELECT acknowledged_sequence FROM sync_cursor").fetchone()[0],
            sequence,
        )

    def test_retention_floor_is_enforced_by_public_api_source(self):
        text = SOURCE.read_text()
        self.assertIn("days<90", text)
        self.assertIn("acknowledged_at IS NOT NULL", text)
        config = (Path(__file__).parents[1] / "components/sqlite3/config_ext.h").read_text()
        self.assertNotIn("SQLITE_NO_SYNC", config)


if __name__ == "__main__":
    unittest.main()
